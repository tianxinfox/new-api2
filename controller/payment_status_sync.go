package controller

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	alipay "github.com/smartwalle/alipay/v3"
	"github.com/wechatpay-apiv3/wechatpay-go/core"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments/native"
)

func mapWeChatTradeStateToTopUpStatus(state string) string {
	switch strings.ToUpper(strings.TrimSpace(state)) {
	case "SUCCESS":
		return common.TopUpStatusSuccess
	case "CLOSED", "REVOKED":
		return common.TopUpStatusUnpaid
	case "PAYERROR":
		return common.TopUpStatusFailed
	default:
		return common.TopUpStatusPending
	}
}

func mapAlipayTradeStateToTopUpStatus(state alipay.TradeStatus) string {
	switch state {
	case alipay.TradeStatusSuccess, alipay.TradeStatusFinished:
		return common.TopUpStatusSuccess
	case alipay.TradeStatusClosed:
		return common.TopUpStatusUnpaid
	default:
		return common.TopUpStatusPending
	}
}

func syncTopUpStatusWithProvider(ctx context.Context, topUp *model.TopUp) error {
	if topUp == nil || topUp.Status != common.TopUpStatusPending {
		return nil
	}
	subscriptionOrder, err := model.GetSubscriptionOrderByTradeNo(topUp.TradeNo)
	if err != nil {
		return err
	}
	isSubscriptionTopUp := subscriptionOrder != nil

	switch topUp.PaymentMethod {
	case PaymentMethodWeChat:
		client, err := getWeChatPayClient(ctx)
		if err != nil {
			return err
		}
		svc := native.NativeApiService{Client: client}
		tx, _, err := svc.QueryOrderByOutTradeNo(ctx, native.QueryOrderByOutTradeNoRequest{
			OutTradeNo: core.String(topUp.TradeNo),
			Mchid:      core.String(setting.WeChatPayMchID),
		})
		if err != nil || tx == nil || tx.TradeState == nil {
			if err != nil {
				return err
			}
			return nil
		}
		status := mapWeChatTradeStateToTopUpStatus(*tx.TradeState)
		payload := common.GetJsonString(tx)
		// If provider still reports NOTPAY after local expiry, force-close then
		// converge to unpaid to avoid long-lived pending records.
		if status == common.TopUpStatusPending &&
			strings.EqualFold(strings.TrimSpace(*tx.TradeState), "NOTPAY") &&
			topUp.ProviderExpireTime > 0 &&
			time.Now().Unix() >= topUp.ProviderExpireTime+60 {
			if closeErr := closeWeChatOrderByTradeNo(ctx, topUp.TradeNo); closeErr != nil {
				common.SysError(fmt.Sprintf("wechat close overdue notpay order failed: trade_no=%s err=%v", topUp.TradeNo, closeErr))
			}
			status = common.TopUpStatusUnpaid
		}
		switch status {
		case common.TopUpStatusSuccess:
			if tx.TransactionId != nil && strings.TrimSpace(*tx.TransactionId) != "" {
				weChatTradeNo := strings.TrimSpace(*tx.TransactionId)
				if bindErr := model.BindTopUpWeChatTradeNo(topUp.TradeNo, weChatTradeNo); bindErr != nil {
					return bindErr
				}
				if isSubscriptionTopUp {
					if bindErr := model.BindSubscriptionWeChatTradeNo(topUp.TradeNo, weChatTradeNo); bindErr != nil {
						return bindErr
					}
				}
			}
			if isSubscriptionTopUp {
				if err = model.CompleteSubscriptionOrder(topUp.TradeNo, payload); err != nil {
					if errors.Is(err, model.ErrSubscriptionOrderStatusInvalid) {
						common.SysError(fmt.Sprintf("subscription order status conflict: trade_no=%s provider says SUCCESS but order status is not pending", topUp.TradeNo))
						if markErr := model.MarkTopUpSuccessIfPending(topUp.TradeNo); markErr != nil {
							return markErr
						}
					} else {
						return err
					}
				}
			} else {
				if err = model.ManualCompleteTopUp(topUp.TradeNo); err != nil {
					return err
				}
			}
		case common.TopUpStatusUnpaid, common.TopUpStatusFailed, common.TopUpStatusExpired:
			if isSubscriptionTopUp {
				if err = model.UpdateSubscriptionOrderStatusIfPending(topUp.TradeNo, status, payload); err != nil {
					return err
				}
			}
			if err = model.UpdateTopUpStatusIfPending(topUp.TradeNo, status); err != nil {
				return err
			}
		default:
			return nil
		}
		topUp.Status = status
		return nil
	case PaymentMethodAlipay:
		client, err := getAlipayClient()
		if err != nil {
			return err
		}
		rsp, err := client.TradeQuery(ctx, alipay.TradeQuery{
			OutTradeNo: topUp.TradeNo,
		})
		if err != nil || rsp == nil || rsp.IsFailure() {
			if err != nil {
				return err
			}
			if rsp != nil && rsp.IsFailure() {
				return fmt.Errorf("alipay trade query failed: trade_no=%s code=%s msg=%s sub_code=%s sub_msg=%s", topUp.TradeNo, rsp.Code, rsp.Msg, rsp.SubCode, rsp.SubMsg)
			}
			return nil
		}
		status := mapAlipayTradeStateToTopUpStatus(rsp.TradeStatus)
		payload := common.GetJsonString(rsp)
		switch status {
		case common.TopUpStatusSuccess:
			if strings.TrimSpace(rsp.TradeNo) != "" {
				alipayTradeNo := strings.TrimSpace(rsp.TradeNo)
				if bindErr := model.BindTopUpAlipayTradeNo(topUp.TradeNo, alipayTradeNo); bindErr != nil {
					return bindErr
				}
				if isSubscriptionTopUp {
					if bindErr := model.BindSubscriptionAlipayTradeNo(topUp.TradeNo, alipayTradeNo); bindErr != nil {
						return bindErr
					}
				}
			}
			if isSubscriptionTopUp {
				if err = model.CompleteSubscriptionOrder(topUp.TradeNo, payload); err != nil {
					if errors.Is(err, model.ErrSubscriptionOrderStatusInvalid) {
						common.SysError(fmt.Sprintf("subscription order status conflict: trade_no=%s provider says SUCCESS but order status is not pending", topUp.TradeNo))
						if markErr := model.MarkTopUpSuccessIfPending(topUp.TradeNo); markErr != nil {
							return markErr
						}
					} else {
						return err
					}
				}
			} else {
				if err = model.ManualCompleteTopUp(topUp.TradeNo); err != nil {
					return err
				}
			}
		case common.TopUpStatusUnpaid, common.TopUpStatusFailed, common.TopUpStatusExpired:
			if isSubscriptionTopUp {
				if err = model.UpdateSubscriptionOrderStatusIfPending(topUp.TradeNo, status, payload); err != nil {
					return err
				}
			}
			if err = model.UpdateTopUpStatusIfPending(topUp.TradeNo, status); err != nil {
				return err
			}
		default:
			return nil
		}
		topUp.Status = status
		return nil
	default:
		return nil
	}
}
