package controller

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	alipay "github.com/smartwalle/alipay/v3"
)

type SubscriptionAlipayPayRequest struct {
	PlanId        int    `json:"plan_id"`
	PaymentMethod string `json:"payment_method"`
}

func SubscriptionRequestAlipayPay(c *gin.Context) {
	var req SubscriptionAlipayPayRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.PlanId <= 0 {
		common.SysError(fmt.Sprintf("subscription alipay bind request failed: plan_id=%d err=%v", req.PlanId, err))
		common.ApiErrorMsg(c, "invalid parameters")
		return
	}
	if req.PaymentMethod != "" && req.PaymentMethod != PaymentMethodAlipay {
		common.SysError(fmt.Sprintf("subscription alipay unsupported payment method: payment_method=%s", req.PaymentMethod))
		common.ApiErrorMsg(c, "unsupported payment method")
		return
	}
	if !operation_setting.ContainsPayMethod(PaymentMethodAlipay) {
		common.SysError("subscription alipay method not enabled in PayMethods")
		common.ApiErrorMsg(c, "payment method not found")
		return
	}
	if !IsAlipayConfigured() {
		common.SysError("subscription alipay not configured")
		common.ApiErrorMsg(c, "alipay is not configured")
		return
	}

	plan, err := model.GetSubscriptionPlanById(req.PlanId)
	if err != nil {
		common.SysError(fmt.Sprintf("subscription alipay get plan failed: plan_id=%d err=%v", req.PlanId, err))
		common.ApiError(c, err)
		return
	}
	if !plan.Enabled {
		common.ApiErrorI18n(c, "subscription.not_enabled")
		return
	}
	if plan.PriceAmount < 0.01 {
		common.SysError(fmt.Sprintf("subscription alipay price too low: plan_id=%d price=%.6f", plan.Id, plan.PriceAmount))
		common.ApiErrorMsg(c, "plan price too low")
		return
	}

	userID := c.GetInt("id")
	if plan.MaxPurchasePerUser > 0 {
		count, err := model.CountUserSubscriptionsByPlan(userID, plan.Id)
		if err != nil {
			common.SysError(fmt.Sprintf("subscription alipay count user subscriptions failed: user_id=%d plan_id=%d err=%v", userID, plan.Id, err))
			common.ApiError(c, err)
			return
		}
		if count >= int64(plan.MaxPurchasePerUser) {
			common.SysError(fmt.Sprintf("subscription alipay max purchase reached: user_id=%d plan_id=%d count=%d max=%d", userID, plan.Id, count, plan.MaxPurchasePerUser))
			common.ApiErrorMsg(c, "max purchase limit reached")
			return
		}
	}

	client, err := getAlipayClient()
	if err != nil {
		common.SysError(fmt.Sprintf("subscription alipay init client failed: user_id=%d plan_id=%d err=%v", userID, plan.Id, err))
		common.ApiErrorMsg(c, "alipay config invalid")
		return
	}
	tradeNo := fmt.Sprintf("SUBALIUSR%dNO%s%d", userID, common.GetRandomString(6), time.Now().Unix())
	callBackAddress := service.GetCallbackAddress()
	payURL, err := client.TradePagePay(alipay.TradePagePay{
		Trade: alipay.Trade{
			NotifyURL:      callBackAddress + "/api/subscription/alipay/notify",
			ReturnURL:      system_setting.ServerAddress + "/console/subscription",
			Subject:        fmt.Sprintf("SUB:%s", plan.Title),
			OutTradeNo:     tradeNo,
			TotalAmount:    decimal.NewFromFloat(plan.PriceAmount).StringFixed(2),
			ProductCode:    "FAST_INSTANT_TRADE_PAY",
			TimeoutExpress: "30m",
		},
	})
	if err != nil || payURL == nil {
		common.SysError(fmt.Sprintf("subscription alipay trade page pay failed: user_id=%d plan_id=%d trade_no=%s err=%v pay_url_nil=%t", userID, plan.Id, tradeNo, err, payURL == nil))
		common.ApiErrorMsg(c, "failed to initiate payment")
		return
	}

	order := &model.SubscriptionOrder{
		UserId:        userID,
		PlanId:        plan.Id,
		Money:         plan.PriceAmount,
		TradeNo:       tradeNo,
		PaymentMethod: PaymentMethodAlipay,
		CreateTime:    time.Now().Unix(),
		Status:        common.TopUpStatusPending,
	}
	if err = model.CreateSubscriptionOrderWithTopUp(order); err != nil {
		common.SysError(fmt.Sprintf("subscription alipay create order with topup failed: user_id=%d plan_id=%d trade_no=%s err=%v", userID, plan.Id, tradeNo, err))
		common.ApiErrorMsg(c, "failed to create order")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"data": gin.H{
			"pay_link": payURL.String(),
			"trade_no": tradeNo,
		},
	})
}

func SubscriptionAlipayNotify(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		common.SysError(fmt.Sprintf("subscription alipay notify parse form failed: err=%v", err))
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}
	notification, err := parseAlipayNotification(c.Request.Context(), c.Request.Form)
	if err != nil || notification == nil {
		common.SysError(fmt.Sprintf("subscription alipay notify verify/decode failed: err=%v notification_nil=%t", err, notification == nil))
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}
	if notification.AppId != setting.AlipayAppID {
		common.SysError(fmt.Sprintf("subscription alipay notify appid mismatch: got=%s expected=%s", notification.AppId, setting.AlipayAppID))
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}
	if notification.OutTradeNo == "" {
		common.SysError("subscription alipay notify missing out_trade_no")
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}
	if notification.TradeStatus != alipay.TradeStatusSuccess && notification.TradeStatus != alipay.TradeStatusFinished {
		_, _ = c.Writer.Write([]byte("success"))
		return
	}

	tradeNo := notification.OutTradeNo
	LockOrder(tradeNo)
	defer UnlockOrder(tradeNo)

	order, err := model.GetSubscriptionOrderByTradeNo(tradeNo)
	if err != nil {
		common.SysError(fmt.Sprintf("subscription alipay notify query order failed: trade_no=%s err=%v", tradeNo, err))
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}
	if order == nil || order.PaymentMethod != PaymentMethodAlipay {
		common.SysError(fmt.Sprintf("subscription alipay notify order invalid: trade_no=%s order_nil=%t", tradeNo, order == nil))
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}
	totalCents, err := alipayMoneyStrToCents(notification.TotalAmount)
	if err != nil || alipayMoneyToCents(order.Money) != totalCents {
		common.SysError(fmt.Sprintf("subscription alipay notify amount mismatch: trade_no=%s order_money=%.6f notify_total=%s err=%v", tradeNo, order.Money, notification.TotalAmount, err))
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}
	plan, err := model.GetSubscriptionPlanById(order.PlanId)
	if err != nil {
		common.SysError(fmt.Sprintf("subscription alipay notify get plan failed: trade_no=%s plan_id=%d err=%v", tradeNo, order.PlanId, err))
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}
	if alipayMoneyToCents(plan.PriceAmount) != alipayMoneyToCents(order.Money) {
		common.SysError(fmt.Sprintf("subscription alipay notify integrity check failed: trade_no=%s plan_price=%.6f order_money=%.6f", tradeNo, plan.PriceAmount, order.Money))
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}
	err = model.CompleteSubscriptionOrder(tradeNo, common.GetJsonString(notification))
	if err != nil && !errors.Is(err, model.ErrSubscriptionOrderStatusInvalid) {
		common.SysError(fmt.Sprintf("subscription alipay notify complete order failed: trade_no=%s err=%v", tradeNo, err))
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}
	_, _ = c.Writer.Write([]byte("success"))
}
