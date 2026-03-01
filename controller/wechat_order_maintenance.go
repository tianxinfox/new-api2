package controller

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/wechatpay-apiv3/wechatpay-go/core"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments/native"
)

const (
	weChatOrderSyncTimeout         = 5 * time.Second
	weChatOrderSweepInterval       = 1 * time.Hour
	weChatOrderSweepBatchSize      = 100
	weChatCloseFallbackGraceSecond = 60
	weChatDelayedCheckMaxScheduled = 20000
)

var weChatDelayedCheckScheduled sync.Map
var weChatDelayedCheckScheduledCount atomic.Int64

func scheduleWeChatOrderDelayedCheck(tradeNo string) {
	if tradeNo == "" {
		return
	}
	nowUnix := time.Now().Unix()
	if _, loaded := weChatDelayedCheckScheduled.LoadOrStore(tradeNo, nowUnix); loaded {
		return
	}
	if weChatDelayedCheckScheduledCount.Add(1) > weChatDelayedCheckMaxScheduled {
		removeWeChatDelayedScheduleEntry(tradeNo)
		common.SysError(fmt.Sprintf("wechat delayed check schedule overflow: trade_no=%s max=%d", tradeNo, weChatDelayedCheckMaxScheduled))
		return
	}
	delay := time.Duration(getWeChatDelayedCheckMinutes()) * time.Minute
	time.AfterFunc(delay, func() {
		defer removeWeChatDelayedScheduleEntry(tradeNo)
		processWeChatPendingOrder(tradeNo)
	})
}

func StartWeChatOrderMaintenanceTask() {
	go func() {
		// Run once on startup to recover pending orders after restart.
		cleanupStaleWeChatDelayedSchedules()
		sweepWeChatPendingOrders()

		ticker := time.NewTicker(weChatOrderSweepInterval)
		defer ticker.Stop()

		for range ticker.C {
			cleanupStaleWeChatDelayedSchedules()
			sweepWeChatPendingOrders()
		}
	}()
}

func sweepWeChatPendingOrders() {
	cleanupStaleWeChatDelayedSchedules()
	now := time.Now().Unix()
	before := now - int64(getWeChatDelayedCheckMinutes()*60)
	lastID := 0
	for {
		topUps, err := model.ListPendingWeChatTopUpsCreatedBefore(before, lastID, weChatOrderSweepBatchSize)
		if err != nil {
			common.SysError(fmt.Sprintf("wechat pending order sweep query failed: err=%v", err))
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
			processWeChatPendingOrder(topUp.TradeNo)
		}
		if len(topUps) < weChatOrderSweepBatchSize {
			return
		}
	}
}

func cleanupStaleWeChatDelayedSchedules() {
	now := time.Now().Unix()
	// A schedule entry should be consumed around delayed check time; anything
	// much older is considered stale and can be safely evicted.
	staleBefore := now - int64(getWeChatDelayedCheckMinutes()*3*60)
	weChatDelayedCheckScheduled.Range(func(key, value any) bool {
		tradeNo, ok := key.(string)
		if !ok || tradeNo == "" {
			return true
		}
		scheduledAt, ok := value.(int64)
		if !ok || scheduledAt <= 0 || scheduledAt <= staleBefore {
			removeWeChatDelayedScheduleEntry(tradeNo)
		}
		return true
	})
}

func removeWeChatDelayedScheduleEntry(tradeNo string) {
	if tradeNo == "" {
		return
	}
	if _, existed := weChatDelayedCheckScheduled.LoadAndDelete(tradeNo); existed {
		weChatDelayedCheckScheduledCount.Add(-1)
	}
}

func processWeChatPendingOrder(tradeNo string) {
	if tradeNo == "" {
		return
	}
	LockOrder(tradeNo)
	defer UnlockOrder(tradeNo)

	topUp := model.GetTopUpByTradeNo(tradeNo)
	if topUp == nil || topUp.Status != common.TopUpStatusPending || topUp.PaymentMethod != PaymentMethodWeChat {
		return
	}

	if err := withWeChatOrderTimeout(func(ctx context.Context) error {
		return syncTopUpStatusWithProvider(ctx, topUp)
	}); err != nil {
		common.SysError(fmt.Sprintf("wechat pending order sync failed: trade_no=%s err=%v", tradeNo, err))
		return
	}
	if topUp.Status != common.TopUpStatusPending {
		return
	}

	expireAt := topUp.ProviderExpireTime
	if expireAt <= 0 {
		expireAt = topUp.CreateTime + int64(getWeChatPendingSweepHours()*3600)
	}
	now := time.Now().Unix()
	if now < expireAt {
		return
	}

	if err := withWeChatOrderTimeout(func(ctx context.Context) error {
		return closeWeChatOrderByTradeNo(ctx, tradeNo)
	}); err != nil {
		common.SysError(fmt.Sprintf("wechat pending order close failed: trade_no=%s err=%v", tradeNo, err))
	}

	if err := withWeChatOrderTimeout(func(ctx context.Context) error {
		return syncTopUpStatusWithProvider(ctx, topUp)
	}); err != nil {
		common.SysError(fmt.Sprintf("wechat pending order sync after close failed: trade_no=%s err=%v", tradeNo, err))
		return
	}
	if topUp.Status != common.TopUpStatusPending {
		return
	}

	// Provider state may still be eventually consistent for a short time.
	if now < expireAt+weChatCloseFallbackGraceSecond {
		return
	}
	if err := model.UpdateTopUpStatusIfPending(tradeNo, common.TopUpStatusUnpaid); err != nil {
		common.SysError(fmt.Sprintf("wechat pending order local fallback update topup failed: trade_no=%s err=%v", tradeNo, err))
	}
	if err := model.UpdateSubscriptionOrderStatusIfPending(tradeNo, common.TopUpStatusUnpaid, common.GetJsonString(map[string]any{
		"provider": "wechat",
		"state":    "NOTPAY",
		"action":   "close_fallback",
	})); err != nil && !errors.Is(err, model.ErrSubscriptionOrderNotFound) {
		common.SysError(fmt.Sprintf("wechat pending order local fallback update subscription failed: trade_no=%s err=%v", tradeNo, err))
	}
}

func withWeChatOrderTimeout(fn func(ctx context.Context) error) error {
	ctx, cancel := context.WithTimeout(context.Background(), weChatOrderSyncTimeout)
	defer cancel()
	return fn(ctx)
}

func closeWeChatOrderByTradeNo(ctx context.Context, tradeNo string) error {
	client, err := getWeChatPayClient(ctx)
	if err != nil {
		return err
	}
	svc := native.NativeApiService{Client: client}
	_, err = svc.CloseOrder(ctx, native.CloseOrderRequest{
		OutTradeNo: core.String(tradeNo),
		Mchid:      core.String(setting.WeChatPayMchID),
	})
	return err
}
