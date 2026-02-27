package controller

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/wechatpay-apiv3/wechatpay-go/core"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments/native"
)

type WeChatPayRequest struct {
	Amount        int64  `json:"amount"`
	PaymentMethod string `json:"payment_method"`
}

func calcTopUpRequestAmountForPay(topUpAmount int64) int64 {
	if operation_setting.GetQuotaDisplayType() != operation_setting.QuotaDisplayTypeTokens {
		return topUpAmount
	}
	return decimal.NewFromInt(topUpAmount).
		Mul(decimal.NewFromFloat(common.QuotaPerUnit)).
		IntPart()
}

func writeWeChatPayError(c *gin.Context, message string) {
	c.JSON(http.StatusOK, gin.H{
		"message": "error",
		"data":    message,
	})
}

func RequestWeChatPay(c *gin.Context) {
	var req WeChatPayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeWeChatPayError(c, "参数错误")
		return
	}
	if req.PaymentMethod != "" && req.PaymentMethod != PaymentMethodWeChat {
		writeWeChatPayError(c, "不支持的支付渠道")
		return
	}
	if !IsWeChatPayConfigured() {
		writeWeChatPayError(c, "支付暂不可用")
		return
	}
	if req.Amount < getMinTopup() {
		writeWeChatPayError(c, fmt.Sprintf("充值数量不能小于 %d", getMinTopup()))
		return
	}

	userId := c.GetInt("id")
	group, err := model.GetUserGroup(userId, true)
	if err != nil {
		common.SysError(fmt.Sprintf("wechat pay get user group failed: user_id=%d err=%v", userId, err))
		writeWeChatPayError(c, "支付请求失败，请稍后重试")
		return
	}
	payMoney := getPayMoney(req.Amount, group)
	if payMoney < 0.01 {
		writeWeChatPayError(c, "top up amount is too low")
		return
	}
	totalFee := weChatMoneyToCents(payMoney)
	if totalFee < 1 {
		writeWeChatPayError(c, "top up amount is too low")
		return
	}

	now := time.Now()
	tradeNo := buildWeChatOutTradeNo("WX", userId, now)
	callBackAddress := service.GetCallbackAddress()
	notifyURL := callBackAddress + "/api/user/wechat/notify"

	client, err := getWeChatPayClient(c.Request.Context())
	if err != nil {
		common.SysError(fmt.Sprintf(
			"wechat pay init client failed: user_id=%d err=%v",
			userId, err,
		))
		writeWeChatPayError(c, "支付请求失败，请稍后重试")
		return
	}
	svc := native.NativeApiService{Client: client}
	prepayResp, _, err := svc.Prepay(c.Request.Context(), native.PrepayRequest{
		Appid:       core.String(setting.WeChatPayAppID),
		Mchid:       core.String(setting.WeChatPayMchID),
		Description: core.String(fmt.Sprintf("TUC%d", req.Amount)),
		OutTradeNo:  core.String(tradeNo),
		NotifyUrl:   core.String(notifyURL),
		Amount: &native.Amount{
			Total:    core.Int64(totalFee),
			Currency: core.String("CNY"),
		},
	})
	if err != nil || prepayResp == nil || prepayResp.CodeUrl == nil || *prepayResp.CodeUrl == "" {
		common.SysError(fmt.Sprintf(
			"wechat prepay failed: user_id=%d out_trade_no=%s notify_url=%s total_fee=%d err=%v prepay_resp_nil=%t code_url_nil=%t",
			userId, tradeNo, notifyURL, totalFee, err, prepayResp == nil, prepayResp != nil && prepayResp.CodeUrl == nil,
		))
		writeWeChatPayError(c, "支付请求失败，请稍后重试")
		return
	}

	amount := req.Amount
	if operation_setting.GetQuotaDisplayType() == operation_setting.QuotaDisplayTypeTokens {
		dAmount := decimal.NewFromInt(amount)
		dQuotaPerUnit := decimal.NewFromFloat(common.QuotaPerUnit)
		amount = dAmount.Div(dQuotaPerUnit).IntPart()
	}
	topUp := &model.TopUp{
		UserId:        userId,
		Amount:        amount,
		Money:         payMoney,
		TradeNo:       tradeNo,
		PaymentMethod: PaymentMethodWeChat,
		CreateTime:    time.Now().Unix(),
		Status:        common.TopUpStatusPending,
	}
	if err = topUp.Insert(); err != nil {
		common.SysError(fmt.Sprintf("wechat pay insert topup failed: user_id=%d out_trade_no=%s err=%v", userId, tradeNo, err))
		writeWeChatPayError(c, "支付请求失败，请稍后重试")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"data": gin.H{
			"code_url": *prepayResp.CodeUrl,
			"trade_no": tradeNo,
		},
	})
}

func WeChatNotify(c *gin.Context) {
	transaction, ok := parseWeChatSuccessTransaction(c)
	if !ok {
		return
	}
	if transaction.TransactionId == nil || strings.TrimSpace(*transaction.TransactionId) == "" {
		writeWeChatNotifyFail(c, "invalid transaction id")
		return
	}

	tradeNo := *transaction.OutTradeNo
	LockOrder(tradeNo)
	defer UnlockOrder(tradeNo)

	topUp := model.GetTopUpByTradeNo(tradeNo)
	if topUp == nil {
		writeWeChatNotifyFail(c, "order not found")
		return
	}
	if topUp.PaymentMethod != PaymentMethodWeChat {
		writeWeChatNotifyFail(c, "invalid payment method")
		return
	}
	if weChatMoneyToCents(topUp.Money) != *transaction.Amount.Total {
		writeWeChatNotifyFail(c, "amount mismatch")
		return
	}
	group, err := model.GetUserGroup(topUp.UserId, true)
	if err != nil {
		writeWeChatNotifyFail(c, "get user group failed")
		return
	}
	expectedAmount := calcTopUpRequestAmountForPay(topUp.Amount)
	expectedMoney := getPayMoney(expectedAmount, group)
	if weChatMoneyToCents(expectedMoney) != *transaction.Amount.Total {
		writeWeChatNotifyFail(c, "order integrity check failed")
		return
	}
	if err = model.BindTopUpWeChatTradeNo(tradeNo, *transaction.TransactionId); err != nil {
		writeWeChatNotifyFail(c, "bind transaction id failed")
		return
	}

	if err = model.ManualCompleteTopUp(tradeNo); err != nil {
		writeWeChatNotifyFail(c, "complete order failed")
		return
	}
	writeWeChatNotifySuccess(c)
}
