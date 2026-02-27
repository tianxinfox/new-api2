package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/wechatpay-apiv3/wechatpay-go/core"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments/native"
)

type SubscriptionWeChatPayRequest struct {
	PlanId int `json:"plan_id"`
}

func SubscriptionRequestWeChatPay(c *gin.Context) {
	var req SubscriptionWeChatPayRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.PlanId <= 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	if !IsWeChatPayConfigured() {
		common.ApiErrorI18n(c, "payment.wechat_not_configured")
		return
	}

	plan, err := model.GetSubscriptionPlanById(req.PlanId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !plan.Enabled {
		common.ApiErrorI18n(c, "subscription.not_enabled")
		return
	}
	if plan.PriceAmount < 0.01 {
		common.ApiErrorMsg(c, "套餐金额过低")
		return
	}

	userId := c.GetInt("id")
	if plan.MaxPurchasePerUser > 0 {
		count, err := model.CountUserSubscriptionsByPlan(userId, plan.Id)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		if count >= int64(plan.MaxPurchasePerUser) {
			common.ApiErrorMsg(c, "已达到该套餐购买上限")
			return
		}
	}

	tradeNo := buildWeChatOutTradeNo("SWX", userId, time.Now())
	callBackAddress := service.GetCallbackAddress()
	notifyURL := callBackAddress + "/api/subscription/wechat/notify"
	totalFee := weChatMoneyToCents(plan.PriceAmount)
	if totalFee < 1 {
		common.ApiErrorMsg(c, "套餐金额过低")
		return
	}

	client, err := getWeChatPayClient(c.Request.Context())
	if err != nil {
		common.SysError(fmt.Sprintf(
			"subscription wechat pay init client failed: user_id=%d plan_id=%d mch_id=%s app_id=%s mch_serial=%s err=%v",
			userId, plan.Id, setting.WeChatPayMchID, setting.WeChatPayAppID, setting.WeChatPayMchSerial, err,
		))
		common.ApiErrorMsg(c, "微信支付配置无效")
		return
	}
	svc := native.NativeApiService{Client: client}
	prepayResp, _, err := svc.Prepay(c.Request.Context(), native.PrepayRequest{
		Appid:       core.String(setting.WeChatPayAppID),
		Mchid:       core.String(setting.WeChatPayMchID),
		Description: core.String(fmt.Sprintf("SUB:%s", plan.Title)),
		OutTradeNo:  core.String(tradeNo),
		NotifyUrl:   core.String(notifyURL),
		Amount: &native.Amount{
			Total:    core.Int64(totalFee),
			Currency: core.String("CNY"),
		},
	})
	if err != nil || prepayResp == nil || prepayResp.CodeUrl == nil || *prepayResp.CodeUrl == "" {
		common.SysError(fmt.Sprintf(
			"subscription wechat prepay failed: user_id=%d plan_id=%d out_trade_no=%s notify_url=%s total_fee=%d err=%v prepay_resp_nil=%t code_url_nil=%t",
			userId, plan.Id, tradeNo, notifyURL, totalFee, err, prepayResp == nil, prepayResp != nil && prepayResp.CodeUrl == nil,
		))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "拉起支付失败"})
		return
	}

	order := &model.SubscriptionOrder{
		UserId:        userId,
		PlanId:        plan.Id,
		Money:         plan.PriceAmount,
		TradeNo:       tradeNo,
		PaymentMethod: PaymentMethodWeChat,
		CreateTime:    time.Now().Unix(),
		Status:        common.TopUpStatusPending,
	}
	if err = order.Insert(); err != nil {
		common.SysError(fmt.Sprintf("subscription wechat pay insert order failed: user_id=%d plan_id=%d out_trade_no=%s err=%v", userId, plan.Id, tradeNo, err))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "创建订单失败"})
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

func SubscriptionWeChatNotify(c *gin.Context) {
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

	order := model.GetSubscriptionOrderByTradeNo(tradeNo)
	if order == nil {
		writeWeChatNotifyFail(c, "order not found")
		return
	}
	if order.PaymentMethod != PaymentMethodWeChat {
		writeWeChatNotifyFail(c, "invalid payment method")
		return
	}
	if weChatMoneyToCents(order.Money) != *transaction.Amount.Total {
		writeWeChatNotifyFail(c, "amount mismatch")
		return
	}
	plan, err := model.GetSubscriptionPlanById(order.PlanId)
	if err != nil {
		writeWeChatNotifyFail(c, "plan not found")
		return
	}
	// Integrity check: always enforce current plan price consistency to
	// avoid bypass windows around plan update timing.
	if weChatMoneyToCents(plan.PriceAmount) != weChatMoneyToCents(order.Money) {
		writeWeChatNotifyFail(c, "order integrity check failed")
		return
	}
	if err := model.BindSubscriptionWeChatTradeNo(tradeNo, *transaction.TransactionId); err != nil {
		writeWeChatNotifyFail(c, "bind transaction id failed")
		return
	}

	err = model.CompleteSubscriptionOrder(tradeNo, common.GetJsonString(transaction))
	if err != nil && !errors.Is(err, model.ErrSubscriptionOrderStatusInvalid) {
		writeWeChatNotifyFail(c, "complete order failed")
		return
	}
	writeWeChatNotifySuccess(c)
}
