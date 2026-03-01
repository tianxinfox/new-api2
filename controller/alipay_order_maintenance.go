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
)

const (
	alipayOrderSweepInterval  = 10 * time.Minute
	alipayOrderSweepBatchSize = 100
	alipayOrderSyncTimeout    = 5 * time.Second
	alipayOrderCloseGraceSec  = 60
)

func StartAlipayOrderMaintenanceTask() {
	go func() {
		sweepAlipayPendingOrders()
		ticker := time.NewTicker(alipayOrderSweepInterval)
		defer ticker.Stop()
		for range ticker.C {
			sweepAlipayPendingOrders()
		}
	}()
}

func sweepAlipayPendingOrders() {
	now := time.Now().Unix()
	before := now - int64(getAlipayPendingSweepDelayMinutes()*60)
	lastID := 0
	for {
		topUps, err := model.ListPendingAlipayTopUpsCreatedBefore(before, lastID, alipayOrderSweepBatchSize)
		if err != nil {
			common.SysError(fmt.Sprintf("alipay pending order sweep query failed: err=%v", err))
			return
		}
		if len(topUps) == 0 {
			return
		}
		for _, topUp := range topUps {
			if topUp == nil {
				continue
			}
			if topUp.Id > lastID {
				lastID = topUp.Id
			}
			processAlipayPendingOrder(topUp.TradeNo)
		}
		if len(topUps) < alipayOrderSweepBatchSize {
			return
		}
	}
}

func getAlipayPendingSweepDelayMinutes() int {
	// Default aligns with order expiry window to avoid premature mass queries.
	defaultDelay := getAlipayOrderExpireMinutes()
	if defaultDelay <= 0 {
		defaultDelay = 30
	}
	delay := setting.AlipayPendingSweepDelayMinutes
	if delay <= 0 {
		delay = common.GetEnvOrDefault("ALIPAY_PENDING_SWEEP_DELAY_MINUTES", defaultDelay)
	}
	if delay <= 0 {
		return defaultDelay
	}
	return delay
}

func processAlipayPendingOrder(tradeNo string) {
	if tradeNo == "" {
		return
	}
	LockOrder(tradeNo)
	defer UnlockOrder(tradeNo)

	topUp := model.GetTopUpByTradeNo(tradeNo)
	if topUp == nil || topUp.Status != common.TopUpStatusPending || topUp.PaymentMethod != PaymentMethodAlipay {
		return
	}

	if err := withAlipayOrderTimeout(func(ctx context.Context) error {
		return syncTopUpStatusWithProvider(ctx, topUp)
	}); err != nil {
		common.SysError(fmt.Sprintf("alipay pending order sync failed: trade_no=%s err=%v", tradeNo, err))
		return
	}
	if topUp.Status != common.TopUpStatusPending {
		return
	}

	expireAt := topUp.ProviderExpireTime
	if expireAt <= 0 {
		expireAt = topUp.CreateTime + int64(getAlipayOrderExpireMinutes()*60)
	}
	now := time.Now().Unix()
	if now < expireAt {
		return
	}

	if err := withAlipayOrderTimeout(func(ctx context.Context) error {
		return closeAlipayOrderByTradeNo(ctx, tradeNo)
	}); err != nil {
		common.SysError(fmt.Sprintf("alipay pending order close failed: trade_no=%s err=%v", tradeNo, err))
	}

	if err := withAlipayOrderTimeout(func(ctx context.Context) error {
		return syncTopUpStatusWithProvider(ctx, topUp)
	}); err != nil {
		common.SysError(fmt.Sprintf("alipay pending order sync after close failed: trade_no=%s err=%v", tradeNo, err))
		return
	}
	if topUp.Status != common.TopUpStatusPending {
		return
	}

	if now < expireAt+alipayOrderCloseGraceSec {
		return
	}
	if err := model.UpdateTopUpStatusIfPending(tradeNo, common.TopUpStatusUnpaid); err != nil {
		common.SysError(fmt.Sprintf("alipay pending order local fallback update topup failed: trade_no=%s err=%v", tradeNo, err))
	}
	if err := model.UpdateSubscriptionOrderStatusIfPending(tradeNo, common.TopUpStatusUnpaid, common.GetJsonString(map[string]any{
		"provider": "alipay",
		"state":    "WAIT_BUYER_PAY",
		"action":   "close_fallback",
	})); err != nil && !errors.Is(err, model.ErrSubscriptionOrderNotFound) {
		common.SysError(fmt.Sprintf("alipay pending order local fallback update subscription failed: trade_no=%s err=%v", tradeNo, err))
	}
}

func withAlipayOrderTimeout(fn func(ctx context.Context) error) error {
	ctx, cancel := context.WithTimeout(context.Background(), alipayOrderSyncTimeout)
	defer cancel()
	return fn(ctx)
}

func closeAlipayOrderByTradeNo(ctx context.Context, tradeNo string) error {
	client, err := getAlipayClient()
	if err != nil {
		return err
	}
	rsp, err := client.TradeClose(ctx, alipay.TradeClose{
		OutTradeNo: tradeNo,
	})
	if err != nil {
		return err
	}
	if rsp != nil && rsp.IsFailure() {
		subCode := strings.TrimSpace(rsp.SubCode)
		if subCode == "ACQ.TRADE_HAS_CLOSE" || subCode == "ACQ.TRADE_STATUS_ERROR" {
			return nil
		}
		return fmt.Errorf("alipay trade close failed: trade_no=%s code=%s msg=%s sub_code=%s sub_msg=%s", tradeNo, rsp.Code, rsp.Msg, rsp.SubCode, rsp.SubMsg)
	}
	return nil
}
