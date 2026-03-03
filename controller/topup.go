package controller

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/Calcium-Ion/go-epay/epay"
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/shopspring/decimal"
)

func GetTopUpInfo(c *gin.Context) {
	// Get payment method configuration.
	payMethods := operation_setting.PayMethods
	// If Stripe is enabled and missing from the list, append Stripe.
	if setting.StripeApiSecret != "" && setting.StripeWebhookSecret != "" && setting.StripePriceId != "" {
		hasStripe := false
		for _, method := range payMethods {
			if method["type"] == "stripe" {
				hasStripe = true
				break
			}
		}

		if !hasStripe {
			stripeMethod := map[string]string{
				"name":      "Stripe",
				"type":      "stripe",
				"color":     "rgba(var(--semi-purple-5), 1)",
				"min_topup": strconv.Itoa(setting.StripeMinTopUp),
			}
			payMethods = append(payMethods, stripeMethod)
		}
	}
	if IsWeChatPayConfigured() {
		hasWeChat := false
		for _, method := range payMethods {
			if method["type"] == PaymentMethodWeChat || method["type"] == "wxpay" {
				hasWeChat = true
				break
			}
		}
		if !hasWeChat {
			payMethods = append(payMethods, map[string]string{
				"name":      "WeChat Pay",
				"type":      PaymentMethodWeChat,
				"color":     "rgba(var(--semi-green-5), 1)",
				"min_topup": strconv.Itoa(operation_setting.MinTopUp),
			})
		}
	}
	if IsAlipayConfigured() {
		hasAlipay := false
		for _, method := range payMethods {
			if method["type"] == PaymentMethodAlipay {
				hasAlipay = true
				break
			}
		}
		if !hasAlipay {
			payMethods = append(payMethods, map[string]string{
				"name":      "Alipay",
				"type":      PaymentMethodAlipay,
				"color":     "rgba(var(--semi-blue-5), 1)",
				"min_topup": strconv.Itoa(operation_setting.MinTopUp),
			})
		}
	}

	data := gin.H{
		"enable_online_topup": operation_setting.PayAddress != "" && operation_setting.EpayId != "" && operation_setting.EpayKey != "",
		"enable_stripe_topup": setting.StripeApiSecret != "" && setting.StripeWebhookSecret != "" && setting.StripePriceId != "",
		"enable_wechat_topup": IsWeChatPayConfigured(),
		"enable_alipay_topup": IsAlipayConfigured(),
		"alipay_pay_mode":     getAlipayPayMode(),
		"enable_creem_topup":  setting.CreemApiKey != "" && setting.CreemProducts != "[]",
		"creem_products":      setting.CreemProducts,
		"pay_methods":         payMethods,
		"min_topup":           operation_setting.MinTopUp,
		"stripe_min_topup":    setting.StripeMinTopUp,
		"amount_options":      operation_setting.GetPaymentSetting().AmountOptions,
		"discount":            operation_setting.GetPaymentSetting().AmountDiscount,
	}
	common.ApiSuccess(c, data)
}

type EpayRequest struct {
	Amount        int64  `json:"amount"`
	PaymentMethod string `json:"payment_method"`
}

type AmountRequest struct {
	Amount int64 `json:"amount"`
}

type TopUpOrderStatusResponse struct {
	TradeNo string `json:"trade_no"`
	Status  string `json:"status"`
}

func GetEpayClient() *epay.Client {
	if operation_setting.PayAddress == "" || operation_setting.EpayId == "" || operation_setting.EpayKey == "" {
		return nil
	}
	withUrl, err := epay.NewClient(&epay.Config{
		PartnerID: operation_setting.EpayId,
		Key:       operation_setting.EpayKey,
	}, operation_setting.PayAddress)
	if err != nil {
		return nil
	}
	return withUrl
}

func getPayMoney(amount int64, group string) float64 {
	dAmount := decimal.NewFromInt(amount)
	// Amount follows the configured quota display type.
	if operation_setting.GetQuotaDisplayType() == operation_setting.QuotaDisplayTypeTokens {
		dQuotaPerUnit := decimal.NewFromFloat(common.QuotaPerUnit)
		dAmount = dAmount.Div(dQuotaPerUnit)
	}

	topupGroupRatio := common.GetTopupGroupRatio(group)
	if topupGroupRatio == 0 {
		topupGroupRatio = 1
	}

	dTopupGroupRatio := decimal.NewFromFloat(topupGroupRatio)
	dPrice := decimal.NewFromFloat(operation_setting.Price)
	// apply optional preset discount by the original request amount (if configured), default 1.0
	discount := 1.0
	if ds, ok := operation_setting.GetPaymentSetting().AmountDiscount[int(amount)]; ok {
		if ds > 0 {
			discount = ds
		}
	}
	dDiscount := decimal.NewFromFloat(discount)

	payMoney := dAmount.Mul(dPrice).Mul(dTopupGroupRatio).Mul(dDiscount)

	return payMoney.InexactFloat64()
}

func getMinTopup() int64 {
	minTopup := operation_setting.MinTopUp
	if operation_setting.GetQuotaDisplayType() == operation_setting.QuotaDisplayTypeTokens {
		dMinTopup := decimal.NewFromInt(int64(minTopup))
		dQuotaPerUnit := decimal.NewFromFloat(common.QuotaPerUnit)
		minTopup = int(dMinTopup.Mul(dQuotaPerUnit).IntPart())
	}
	return int64(minTopup)
}

func RequestEpay(c *gin.Context) {
	var req EpayRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		common.ApiErrorMsgLegacy(c, "invalid parameters")
		return
	}
	if req.Amount < getMinTopup() {
		common.ApiErrorMsgLegacy(c, fmt.Sprintf("top up amount cannot be less than %d", getMinTopup()))
		return
	}

	id := c.GetInt("id")
	group, err := model.GetUserGroup(id, true)
	if err != nil {
		common.ApiErrorMsgLegacy(c, "failed to get user group")
		return
	}
	payMoney := getPayMoney(req.Amount, group)
	if payMoney < 0.01 {
		common.ApiErrorMsgLegacy(c, "top up amount is too low")
		return
	}

	if !operation_setting.ContainsPayMethod(req.PaymentMethod) {
		common.ApiErrorMsgLegacy(c, "payment method not found")
		return
	}

	callBackAddress := service.GetCallbackAddress()
	returnUrl, _ := url.Parse(system_setting.ServerAddress + "/console/log")
	notifyUrl, _ := url.Parse(callBackAddress + "/api/user/epay/notify")
	tradeNo := fmt.Sprintf("%s%d", common.GetRandomString(6), time.Now().Unix())
	tradeNo = fmt.Sprintf("USR%dNO%s", id, tradeNo)
	client := GetEpayClient()
	if client == nil {
		common.ApiErrorMsgLegacy(c, "payment is not configured")
		return
	}
	uri, params, err := client.Purchase(&epay.PurchaseArgs{
		Type:           req.PaymentMethod,
		ServiceTradeNo: tradeNo,
		Name:           fmt.Sprintf("TUC%d", req.Amount),
		Money:          strconv.FormatFloat(payMoney, 'f', 2, 64),
		Device:         epay.PC,
		NotifyUrl:      notifyUrl,
		ReturnUrl:      returnUrl,
	})
	if err != nil {
		common.ApiErrorMsgLegacy(c, "failed to initiate payment")
		return
	}
	amount := req.Amount
	if operation_setting.GetQuotaDisplayType() == operation_setting.QuotaDisplayTypeTokens {
		dAmount := decimal.NewFromInt(int64(amount))
		dQuotaPerUnit := decimal.NewFromFloat(common.QuotaPerUnit)
		amount = dAmount.Div(dQuotaPerUnit).IntPart()
	}
	topUp := &model.TopUp{
		UserId:        id,
		Amount:        amount,
		Money:         payMoney,
		TradeNo:       tradeNo,
		PaymentMethod: req.PaymentMethod,
		CreateTime:    time.Now().Unix(),
		Status:        "pending",
	}
	err = topUp.Insert()
	if err != nil {
		common.ApiErrorMsgLegacy(c, "failed to create order")
		return
	}
	c.JSON(200, gin.H{"message": "success", "data": params, "url": uri})
}

// tradeNo lock
var orderLocks sync.Map

const orderLockCleanupDelay = time.Minute

type orderLock struct {
	mu   sync.Mutex
	refs atomic.Int64
}

// LockOrder tries to lock a specific trade number.
func LockOrder(tradeNo string) {
	for {
		lockAny, _ := orderLocks.LoadOrStore(tradeNo, &orderLock{})
		lock := lockAny.(*orderLock)
		refs := lock.refs.Load()
		// refs < 0 means this lock is being cleaned up, retry with a fresh lock.
		if refs < 0 {
			orderLocks.CompareAndDelete(tradeNo, lock)
			continue
		}
		if lock.refs.CompareAndSwap(refs, refs+1) {
			lock.mu.Lock()
			return
		}
	}
}

// UnlockOrder releases the lock for a trade number and schedules cleanup.
func UnlockOrder(tradeNo string) {
	lockAny, ok := orderLocks.Load(tradeNo)
	if !ok {
		return
	}

	lock := lockAny.(*orderLock)
	lock.mu.Unlock()

	if lock.refs.Add(-1) != 0 {
		return
	}

	time.AfterFunc(orderLockCleanupDelay, func() {
		current, ok := orderLocks.Load(tradeNo)
		if !ok {
			return
		}
		currentLock := current.(*orderLock)
		// Move refs 0 -> -1 atomically to prevent a new request from reviving
		// this lock between the zero check and map deletion.
		if !currentLock.refs.CompareAndSwap(0, -1) {
			return
		}
		orderLocks.CompareAndDelete(tradeNo, current)
	})
}

func EpayNotify(c *gin.Context) {
	var params map[string]string

	if c.Request.Method == "POST" {
		// POST request: parse parameters from POST body
		if err := c.Request.ParseForm(); err != nil {
			log.Println("epay notify POST parse failed:", err)
			_, _ = c.Writer.Write([]byte("fail"))
			return
		}
		params = lo.Reduce(lo.Keys(c.Request.PostForm), func(r map[string]string, t string, i int) map[string]string {
			r[t] = c.Request.PostForm.Get(t)
			return r
		}, map[string]string{})
	} else {
		// GET request: parse parameters from URL query
		params = lo.Reduce(lo.Keys(c.Request.URL.Query()), func(r map[string]string, t string, i int) map[string]string {
			r[t] = c.Request.URL.Query().Get(t)
			return r
		}, map[string]string{})
	}

	if len(params) == 0 {
		log.Println("epay notify params are empty")
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}
	client := GetEpayClient()
	if client == nil {
		log.Println("epay notify failed: client config not found")
		_, err := c.Writer.Write([]byte("fail"))
		if err != nil {
			log.Println("epay notify write response failed")
		}
		return
	}
	verifyInfo, err := client.Verify(params)
	if err == nil && verifyInfo.VerifyStatus {
		_, err := c.Writer.Write([]byte("success"))
		if err != nil {
			log.Println("epay notify write response failed")
		}
	} else {
		_, err := c.Writer.Write([]byte("fail"))
		if err != nil {
			log.Println("epay notify write response failed")
		}
		log.Println("epay notify signature verify failed")
		return
	}

	if verifyInfo.TradeStatus == epay.StatusTradeSuccess {
		log.Println(verifyInfo)
		LockOrder(verifyInfo.ServiceTradeNo)
		defer UnlockOrder(verifyInfo.ServiceTradeNo)
		topUp := model.GetTopUpByTradeNo(verifyInfo.ServiceTradeNo)
		if topUp == nil {
			log.Printf("epay notify: order not found, callback data: %v", verifyInfo)
			return
		}
		if topUp.Status == "pending" {
			topUp.Status = "success"
			err := topUp.Update()
			if err != nil {
				log.Printf("epay notify: update top-up status failed: %v", topUp)
				return
			}
			//user, _ := model.GetUserById(topUp.UserId, false)
			//user.Quota += topUp.Amount * 500000
			dAmount := decimal.NewFromInt(int64(topUp.Amount))
			dQuotaPerUnit := decimal.NewFromFloat(common.QuotaPerUnit)
			quotaToAdd := int(dAmount.Mul(dQuotaPerUnit).IntPart())
			err = model.IncreaseUserQuota(topUp.UserId, quotaToAdd, true)
			if err != nil {
				log.Printf("epay notify: increase user quota failed: %v", topUp)
				return
			}
			log.Printf("epay notify: top-up success: %v", topUp)
			model.RecordLog(topUp.UserId, model.LogTypeTopup, fmt.Sprintf("online top-up success, quota: %v, amount: %f", logger.LogQuota(quotaToAdd), topUp.Money))
		}
	} else {
		log.Printf("epay notify: trade status is not success: %v", verifyInfo)
	}
}

func RequestAmount(c *gin.Context) {
	var req AmountRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		common.ApiErrorMsgLegacy(c, "invalid parameters")
		return
	}

	if req.Amount < getMinTopup() {
		common.ApiErrorMsgLegacy(c, fmt.Sprintf("top up amount cannot be less than %d", getMinTopup()))
		return
	}
	id := c.GetInt("id")
	group, err := model.GetUserGroup(id, true)
	if err != nil {
		common.ApiErrorMsgLegacy(c, "failed to get user group")
		return
	}
	payMoney := getPayMoney(req.Amount, group)
	if payMoney <= 0.01 {
		common.ApiErrorMsgLegacy(c, "top up amount is too low")
		return
	}
	common.ApiSuccessLegacy(c, strconv.FormatFloat(payMoney, 'f', 2, 64))
}

func GetUserTopUps(c *gin.Context) {
	userId := c.GetInt("id")
	pageInfo := common.GetPageQuery(c)
	keyword := c.Query("keyword")

	var (
		topups []*model.TopUp
		total  int64
		err    error
	)
	if keyword != "" {
		topups, total, err = model.SearchUserTopUps(userId, keyword, pageInfo)
	} else {
		topups, total, err = model.GetUserTopUps(userId, pageInfo)
	}
	if err != nil {
		common.ApiError(c, err)
		return
	}
	refreshPendingTopUpStatuses(c.Request.Context(), topups)

	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(topups)
	common.ApiSuccess(c, pageInfo)
}

func GetTopUpOrderStatus(c *gin.Context) {
	userId := c.GetInt("id")
	tradeNo := strings.TrimSpace(c.Query("trade_no"))
	if tradeNo == "" {
		common.ApiErrorMsg(c, "订单号不能为空")
		return
	}

	topUp := model.GetTopUpByTradeNo(tradeNo)
	if topUp == nil || topUp.UserId != userId {
		common.ApiErrorMsg(c, "订单不存在")
		return
	}

	if topUp.Status == common.TopUpStatusPending &&
		(topUp.PaymentMethod == PaymentMethodWeChat || topUp.PaymentMethod == PaymentMethodAlipay) {
		LockOrder(tradeNo)
		func() {
			defer UnlockOrder(tradeNo)
			current := model.GetTopUpByTradeNo(tradeNo)
			if current == nil || current.UserId != userId || current.Status != common.TopUpStatusPending {
				return
			}
			syncCtx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
			defer cancel()
			if err := syncTopUpStatusWithProvider(syncCtx, current); err != nil {
				common.SysError(fmt.Sprintf("topup order status sync failed: trade_no=%s err=%v", tradeNo, err))
			}
		}()
		topUp = model.GetTopUpByTradeNo(tradeNo)
		if topUp == nil || topUp.UserId != userId {
			common.ApiErrorMsg(c, "订单不存在")
			return
		}
	}

	common.ApiSuccess(c, TopUpOrderStatusResponse{
		TradeNo: tradeNo,
		Status:  topUp.Status,
	})
}

// GetAllTopUps returns top-up records across all users for admin.
func GetAllTopUps(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	keyword := c.Query("keyword")

	var (
		topups []*model.TopUp
		total  int64
		err    error
	)
	if keyword != "" {
		topups, total, err = model.SearchAllTopUps(keyword, pageInfo)
	} else {
		topups, total, err = model.GetAllTopUps(pageInfo)
	}
	if err != nil {
		common.ApiError(c, err)
		return
	}
	refreshPendingTopUpStatuses(c.Request.Context(), topups)

	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(topups)
	common.ApiSuccess(c, pageInfo)
}

func refreshPendingTopUpStatuses(ctx context.Context, topups []*model.TopUp) {
	const (
		maxSyncItems   = 20
		maxParallelism = 5
		syncTimeout    = 5 * time.Second
	)

	candidates := make([]*model.TopUp, 0, maxSyncItems)
	for _, topUp := range topups {
		if len(candidates) >= maxSyncItems {
			break
		}
		if topUp == nil || topUp.Status != common.TopUpStatusPending {
			continue
		}
		if topUp.PaymentMethod != PaymentMethodWeChat && topUp.PaymentMethod != PaymentMethodAlipay {
			continue
		}
		candidates = append(candidates, topUp)
	}
	if len(candidates) == 0 {
		return
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, maxParallelism)
	for _, topUp := range candidates {
		wg.Add(1)
		go func(item *model.TopUp) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()

			syncCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), syncTimeout)
			defer cancel()

			LockOrder(item.TradeNo)
			if err := syncTopUpStatusWithProvider(syncCtx, item); err != nil {
				common.SysError(fmt.Sprintf("sync topup status with provider failed: trade_no=%s err=%v", item.TradeNo, err))
			}
			UnlockOrder(item.TradeNo)
		}(topUp)
	}
	wg.Wait()
}

type AdminCompleteTopupRequest struct {
	TradeNo string `json:"trade_no"`
}

// AdminCompleteTopUp allows admin to manually complete a top-up order.
func AdminCompleteTopUp(c *gin.Context) {
	var req AdminCompleteTopupRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.TradeNo == "" {
		common.ApiErrorMsg(c, "invalid parameters")
		return
	}

	// Lock the order to avoid duplicate concurrent processing.
	LockOrder(req.TradeNo)
	defer UnlockOrder(req.TradeNo)

	if err := model.ManualCompleteTopUp(req.TradeNo); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}
