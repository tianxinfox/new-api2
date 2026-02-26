package service

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"

	"github.com/bytedance/gopkg/util/gopool"
)

const (
	agentRebateRetryTickInterval = 1 * time.Minute
	agentRebateRetryBatchSize    = 50
)

var (
	agentRebateRetryOnce    sync.Once
	agentRebateRetryRunning atomic.Bool
)

func StartAgentRebateRetryTask() {
	agentRebateRetryOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			logger.LogInfo(context.Background(), fmt.Sprintf("agent rebate retry task started: tick=%s", agentRebateRetryTickInterval))
			ticker := time.NewTicker(agentRebateRetryTickInterval)
			defer ticker.Stop()

			runAgentRebateRetryOnce()
			for range ticker.C {
				runAgentRebateRetryOnce()
			}
		})
	})
}

func runAgentRebateRetryOnce() {
	if !agentRebateRetryRunning.CompareAndSwap(false, true) {
		return
	}
	defer agentRebateRetryRunning.Store(false)

	processed, succeeded, err := model.ProcessPendingAgentRebateRetryTasks(agentRebateRetryBatchSize)
	if err != nil {
		logger.LogWarn(context.Background(), fmt.Sprintf("agent rebate retry task failed: %v", err))
		return
	}
	if common.DebugEnabled && processed > 0 {
		logger.LogDebug(context.Background(), "agent rebate retry task: processed=%d, succeeded=%d", processed, succeeded)
	}
}
