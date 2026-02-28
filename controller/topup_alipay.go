package controller

import (
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

const PaymentMethodAlipay = "alipay"

type AlipayPayRequest struct {
	Amount        int64  `json:"amount"`
	PaymentMethod string `json:"payment_method"`
}

func RequestAlipayPay(c *gin.Context) {
	var req AlipayPayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.SysError(fmt.Sprintf("alipay pay bind request failed: err=%v", err))
		common.ApiErrorMsgLegacy(c, "invalid parameters")
		return
	}
	if req.PaymentMethod != "" && req.PaymentMethod != PaymentMethodAlipay {
		common.SysError(fmt.Sprintf("alipay pay unsupported payment method: payment_method=%s", req.PaymentMethod))
		common.ApiErrorMsgLegacy(c, "unsupported payment method")
		return
	}
	if !IsAlipayConfigured() {
		common.SysError("alipay pay failed: alipay not configured")
		common.ApiErrorMsgLegacy(c, "alipay is not configured")
		return
	}
	if !operation_setting.ContainsPayMethod(PaymentMethodAlipay) {
		common.SysError("alipay pay failed: alipay method not enabled in PayMethods")
		common.ApiErrorMsgLegacy(c, "payment method not found")
		return
	}
	if req.Amount < getMinTopup() {
		common.SysError(fmt.Sprintf("alipay pay amount too low: amount=%d min=%d", req.Amount, getMinTopup()))
		common.ApiErrorMsgLegacy(c, fmt.Sprintf("top up amount cannot be less than %d", getMinTopup()))
		return
	}

	userID := c.GetInt("id")
	group, err := model.GetUserGroup(userID, true)
	if err != nil {
		common.SysError(fmt.Sprintf("alipay pay get user group failed: user_id=%d err=%v", userID, err))
		common.ApiErrorMsgLegacy(c, "failed to get user group")
		return
	}
	payMoney := getPayMoney(req.Amount, group)
	if payMoney < 0.01 {
		common.SysError(fmt.Sprintf("alipay pay computed money too low: user_id=%d amount=%d pay_money=%.6f", userID, req.Amount, payMoney))
		common.ApiErrorMsgLegacy(c, "top up amount is too low")
		return
	}

	client, err := getAlipayClient()
	if err != nil {
		common.SysError(fmt.Sprintf("alipay pay init client failed: user_id=%d err=%v", userID, err))
		common.ApiErrorMsgLegacy(c, "failed to initialize alipay client")
		return
	}
	tradeNo := fmt.Sprintf("ALIUSR%dNO%s%d", userID, common.GetRandomString(6), time.Now().Unix())
	callBackAddress := service.GetCallbackAddress()
	payMode := getAlipayPayMode()
	trade := alipay.Trade{
		NotifyURL:      callBackAddress + "/api/user/alipay/notify",
		ReturnURL:      system_setting.ServerAddress + "/console/topup",
		Subject:        fmt.Sprintf("TUC%d", req.Amount),
		OutTradeNo:     tradeNo,
		TotalAmount:    decimal.NewFromFloat(payMoney).StringFixed(2),
		ProductCode:    getAlipayProductCode(payMode),
		TimeoutExpress: "30m",
	}
	payLink := ""
	qrCode := ""
	if payMode == setting.AlipayPayModePreCreate {
		rsp, preErr := client.TradePreCreate(c.Request.Context(), alipay.TradePreCreate{Trade: trade})
		if preErr != nil || rsp == nil || rsp.IsFailure() || rsp.QRCode == "" {
			logAlipayPreCreateError("alipay trade precreate failed", userID, tradeNo, preErr, rsp)
			common.ApiErrorMsgLegacy(c, getAlipayPreCreateFailureMessage(preErr, rsp))
			return
		}
		qrCode = rsp.QRCode
	} else {
		payURL, pageErr := client.TradePagePay(alipay.TradePagePay{Trade: trade})
		if pageErr != nil || payURL == nil {
			common.SysError(fmt.Sprintf("alipay trade page pay failed: user_id=%d trade_no=%s err=%v pay_url_nil=%t", userID, tradeNo, pageErr, payURL == nil))
			common.ApiErrorMsgLegacy(c, "failed to initiate payment")
			return
		}
		payLink = payURL.String()
	}

	amount := req.Amount
	if operation_setting.GetQuotaDisplayType() == operation_setting.QuotaDisplayTypeTokens {
		dAmount := decimal.NewFromInt(amount)
		dQuotaPerUnit := decimal.NewFromFloat(common.QuotaPerUnit)
		amount = dAmount.Div(dQuotaPerUnit).IntPart()
	}
	topUp := &model.TopUp{
		UserId:        userID,
		Amount:        amount,
		Money:         payMoney,
		TradeNo:       tradeNo,
		PaymentMethod: PaymentMethodAlipay,
		CreateTime:    time.Now().Unix(),
		Status:        common.TopUpStatusPending,
	}
	if err = topUp.Insert(); err != nil {
		common.SysError(fmt.Sprintf("alipay pay insert topup failed: user_id=%d trade_no=%s err=%v", userID, tradeNo, err))
		common.ApiErrorMsgLegacy(c, "failed to create order")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"data": gin.H{
			"pay_mode": payMode,
			"pay_link": payLink,
			"qr_code":  qrCode,
			"trade_no": tradeNo,
		},
	})
}

func AlipayNotify(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		common.SysError(fmt.Sprintf("alipay notify parse form failed: err=%v", err))
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}
	notification, err := parseAlipayNotification(c.Request.Context(), c.Request.Form)
	if err != nil || notification == nil {
		common.SysError(fmt.Sprintf("alipay notify verify/decode failed: err=%v notification_nil=%t", err, notification == nil))
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}
	if notification.AppId != setting.AlipayAppID {
		common.SysError(fmt.Sprintf("alipay notify appid mismatch: got=%s expected=%s", notification.AppId, setting.AlipayAppID))
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}
	if notification.OutTradeNo == "" {
		common.SysError("alipay notify missing out_trade_no")
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

	topUp := model.GetTopUpByTradeNo(tradeNo)
	if topUp == nil || topUp.PaymentMethod != PaymentMethodAlipay {
		common.SysError(fmt.Sprintf("alipay notify order invalid: trade_no=%s topup_nil=%t payment_method=%v", tradeNo, topUp == nil, func() string {
			if topUp == nil {
				return ""
			}
			return topUp.PaymentMethod
		}()))
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}
	totalCents, err := alipayMoneyStrToCents(notification.TotalAmount)
	if err != nil || alipayMoneyToCents(topUp.Money) != totalCents {
		common.SysError(fmt.Sprintf("alipay notify amount mismatch: trade_no=%s topup_money=%.6f notify_total=%s err=%v", tradeNo, topUp.Money, notification.TotalAmount, err))
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}
	group, err := model.GetUserGroup(topUp.UserId, true)
	if err != nil {
		common.SysError(fmt.Sprintf("alipay notify get user group failed: trade_no=%s user_id=%d err=%v", tradeNo, topUp.UserId, err))
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}
	expectedAmount := calcTopUpRequestAmountForPay(topUp.Amount)
	expectedMoney := getPayMoney(expectedAmount, group)
	if alipayMoneyToCents(expectedMoney) != totalCents {
		common.SysError(fmt.Sprintf("alipay notify integrity check failed: trade_no=%s expected_money=%.6f notify_total=%s", tradeNo, expectedMoney, notification.TotalAmount))
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}
	if err := model.BindTopUpAlipayTradeNo(tradeNo, notification.TradeNo); err != nil {
		common.SysError(fmt.Sprintf("alipay notify bind trade no failed: trade_no=%s alipay_trade_no=%s err=%v", tradeNo, notification.TradeNo, err))
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}
	if err := model.ManualCompleteTopUp(tradeNo); err != nil {
		common.SysError(fmt.Sprintf("alipay notify complete topup failed: trade_no=%s err=%v", tradeNo, err))
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}
	_, _ = c.Writer.Write([]byte("success"))
}
