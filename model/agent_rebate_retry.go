package model

import (
	"errors"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

const (
	AgentRebateRetryStatusPending    = "pending"
	AgentRebateRetryStatusProcessing = "processing"
	AgentRebateRetryStatusDone       = "done"
	AgentRebateRetryStatusFailed     = "failed"
)

const agentRebateRetryMaxCount = 20
const agentRebateProcessingTimeout = 10 * time.Minute

type AgentRebateRetryTask struct {
	Id            int    `json:"id"`
	SourceType    string `json:"source_type" gorm:"type:varchar(32);not null;uniqueIndex:uk_agent_rebate_retry_source"`
	SourceId      int    `json:"source_id" gorm:"not null;uniqueIndex:uk_agent_rebate_retry_source"`
	CreditedQuota int64  `json:"credited_quota" gorm:"not null;default:0"`
	Status        string `json:"status" gorm:"type:varchar(16);index;not null"`
	RetryCount    int    `json:"retry_count" gorm:"not null;default:0"`
	LastError     string `json:"last_error" gorm:"type:text"`
	CreatedAt     int64  `json:"created_at" gorm:"bigint;autoCreateTime"`
	UpdatedAt     int64  `json:"updated_at" gorm:"bigint;autoUpdateTime"`
}

func (AgentRebateRetryTask) TableName() string {
	return "agent_rebate_retry_tasks"
}

func EnqueueAgentRebateRetryTask(sourceType string, sourceId int, creditedQuota int64, reason string) {
	if sourceType == "" || sourceId <= 0 {
		return
	}

	err := DB.Transaction(func(tx *gorm.DB) error {
		task := &AgentRebateRetryTask{}
		err := tx.Where("source_type = ? AND source_id = ?", sourceType, sourceId).First(task).Error
		if err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			return tx.Create(&AgentRebateRetryTask{
				SourceType:    sourceType,
				SourceId:      sourceId,
				CreditedQuota: creditedQuota,
				Status:        AgentRebateRetryStatusPending,
				RetryCount:    1,
				LastError:     reason,
			}).Error
		}

		return tx.Model(task).Updates(map[string]any{
			"status":         AgentRebateRetryStatusPending,
			"credited_quota": creditedQuota,
			"retry_count":    gorm.Expr("retry_count + ?", 1),
			"last_error":     reason,
		}).Error
	})
	if err != nil {
		common.SysError("enqueue rebate retry task failed: " + err.Error())
	}
}

func ProcessPendingAgentRebateRetryTasks(limit int) (processed int, succeeded int, err error) {
	if limit <= 0 {
		limit = 50
	}
	resetStuckProcessingRebateRetryTasks()

	var tasks []AgentRebateRetryTask
	if err = DB.Where("status = ?", AgentRebateRetryStatusPending).
		Order("id ASC").
		Limit(limit).
		Find(&tasks).Error; err != nil {
		return 0, 0, err
	}

	for i := range tasks {
		task := tasks[i]
		ok, procErr := processSingleRebateRetryTask(task.Id)
		if procErr != nil {
			common.SysError("process rebate retry task failed: " + procErr.Error())
		}
		processed++
		if ok {
			succeeded++
		}
	}
	return processed, succeeded, nil
}

func resetStuckProcessingRebateRetryTasks() {
	cutoff := time.Now().Add(-agentRebateProcessingTimeout).Unix()
	if err := DB.Model(&AgentRebateRetryTask{}).
		Where("status = ? AND updated_at < ?", AgentRebateRetryStatusProcessing, cutoff).
		Updates(map[string]any{
			"status":     AgentRebateRetryStatusPending,
			"last_error": "processing timeout, auto reset to pending",
		}).Error; err != nil {
		common.SysError("reset stuck rebate retry tasks failed: " + err.Error())
	}
}

func processSingleRebateRetryTask(taskId int) (bool, error) {
	task := &AgentRebateRetryTask{}
	res := DB.Model(&AgentRebateRetryTask{}).
		Where("id = ? AND status = ?", taskId, AgentRebateRetryStatusPending).
		Updates(map[string]any{"status": AgentRebateRetryStatusProcessing})
	if res.Error != nil {
		return false, res.Error
	}
	if res.RowsAffected == 0 {
		return false, nil
	}
	if err := DB.Where("id = ?", taskId).First(task).Error; err != nil {
		return false, err
	}

	settleErr := retrySettleBySource(task.SourceType, task.SourceId, task.CreditedQuota)
	if settleErr == nil {
		if err := DB.Model(&AgentRebateRetryTask{}).Where("id = ? AND status = ?", task.Id, AgentRebateRetryStatusProcessing).Updates(map[string]any{
			"status":     AgentRebateRetryStatusDone,
			"last_error": "",
		}).Error; err != nil {
			return false, err
		}
		return true, nil
	}

	nextStatus := AgentRebateRetryStatusPending
	if task.RetryCount >= agentRebateRetryMaxCount {
		nextStatus = AgentRebateRetryStatusFailed
	}
	if err := DB.Model(&AgentRebateRetryTask{}).Where("id = ? AND status = ?", task.Id, AgentRebateRetryStatusProcessing).Updates(map[string]any{
		"status":      nextStatus,
		"retry_count": gorm.Expr("retry_count + ?", 1),
		"last_error":  settleErr.Error(),
	}).Error; err != nil {
		return false, err
	}
	return false, nil
}

func retrySettleBySource(sourceType string, sourceId int, creditedQuota int64) error {
	switch sourceType {
	case AgentRebateSourceTopUp:
		topUp := GetTopUpById(sourceId)
		if topUp == nil {
			return fmt.Errorf("top up not found: %d", sourceId)
		}
		if topUp.Status != common.TopUpStatusSuccess {
			return fmt.Errorf("top up not completed: %d", sourceId)
		}
		if creditedQuota <= 0 {
			return fmt.Errorf("invalid credited quota for top up: %d", sourceId)
		}
		return SettleAgentRebateForTopUp(topUp, creditedQuota)
	case AgentRebateSourceRedemption:
		redemption, err := GetRedemptionById(sourceId)
		if err != nil || redemption == nil {
			if err != nil {
				return err
			}
			return fmt.Errorf("redemption not found: %d", sourceId)
		}
		return settleAgentRebateForRedemptionWithQuota(redemption, creditedQuota)
	default:
		return fmt.Errorf("unsupported rebate source type: %s", sourceType)
	}
}
