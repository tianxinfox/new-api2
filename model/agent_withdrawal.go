package model

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"
	"github.com/shopspring/decimal"
	alipay "github.com/smartwalle/alipay/v3"
	"gorm.io/gorm"
)

const (
	AgentWithdrawalStatusPending      = "pending"
	AgentWithdrawalStatusRejected     = "rejected"
	AgentWithdrawalStatusTransferring = "transferring"
	AgentWithdrawalStatusPaid         = "paid"
	AgentWithdrawalStatusFailed       = "failed"
)

type AgentWithdrawal struct {
	Id                   int     `json:"id"`
	AgentId              int     `json:"agent_id" gorm:"index;not null"`
	Amount               float64 `json:"amount" gorm:"type:decimal(12,2);not null;default:0"`
	Status               string  `json:"status" gorm:"type:varchar(32);not null;default:'pending';index"`
	TradeNo              string  `json:"trade_no" gorm:"uniqueIndex;type:varchar(64);not null"`
	PayeeAccount         string  `json:"payee_account" gorm:"type:varchar(128);not null"`
	PayeeName            string  `json:"payee_name" gorm:"type:varchar(64);not null"`
	AdminRemark          string  `json:"admin_remark" gorm:"type:text"`
	ApplicantRemark      string  `json:"applicant_remark" gorm:"type:text"`
	ReviewedBy           int     `json:"reviewed_by" gorm:"default:0"`
	ReviewedAt           int64   `json:"reviewed_at" gorm:"bigint;default:0"`
	TransferTime         int64   `json:"transfer_time" gorm:"bigint;default:0"`
	AlipayOrderId        string  `json:"alipay_order_id" gorm:"type:varchar(64);default:'';index"`
	AlipayPayFundOrderId string  `json:"alipay_pay_fund_order_id" gorm:"type:varchar(64);default:'';index"`
	TransferStatus       string  `json:"transfer_status" gorm:"type:varchar(32);default:''"`
	FailureReason        string  `json:"failure_reason" gorm:"type:text"`
	ProviderPayload      string  `json:"provider_payload" gorm:"type:text"`
	CreatedAt            int64   `json:"created_at" gorm:"bigint;autoCreateTime"`
	UpdatedAt            int64   `json:"updated_at" gorm:"bigint;autoUpdateTime"`
}

func (AgentWithdrawal) TableName() string {
	return "agent_withdrawals"
}

type AgentWithdrawalListItem struct {
	Id                   int     `json:"id"`
	Amount               float64 `json:"amount"`
	Status               string  `json:"status"`
	TradeNo              string  `json:"trade_no"`
	PayeeAccount         string  `json:"payee_account"`
	PayeeName            string  `json:"payee_name"`
	AdminRemark          string  `json:"admin_remark"`
	ApplicantRemark      string  `json:"applicant_remark"`
	ReviewedBy           int     `json:"reviewed_by"`
	ReviewedAt           int64   `json:"reviewed_at"`
	TransferTime         int64   `json:"transfer_time"`
	AlipayOrderId        string  `json:"alipay_order_id"`
	AlipayPayFundOrderId string  `json:"alipay_pay_fund_order_id"`
	TransferStatus       string  `json:"transfer_status"`
	FailureReason        string  `json:"failure_reason"`
	CreatedAt            int64   `json:"created_at"`
	UpdatedAt            int64   `json:"updated_at"`
}

type AdminAgentWithdrawalListItem struct {
	AgentWithdrawalListItem
	AgentName string `json:"agent_name"`
}

type AgentWithdrawalStats struct {
	TotalRebateAmount  float64 `json:"total_rebate_amount"`
	WithdrawableAmount float64 `json:"withdrawable_amount"`
	PendingAmount      float64 `json:"pending_amount"`
	TransferringAmount float64 `json:"transferring_amount"`
	PaidAmount         float64 `json:"paid_amount"`
	RejectedAmount     float64 `json:"rejected_amount"`
	FailedAmount       float64 `json:"failed_amount"`
}

type AgentWithdrawalCreateRequest struct {
	Amount          float64 `json:"amount"`
	ApplicantRemark string  `json:"applicant_remark"`
}

type AgentWithdrawalReviewResult struct {
	Withdrawal *AgentWithdrawal `json:"withdrawal"`
	Message    string           `json:"message"`
}

func sanitizeAgentWithdrawalForResponse(withdrawal *AgentWithdrawal) *AgentWithdrawal {
	if withdrawal == nil {
		return nil
	}
	safe := *withdrawal
	safe.ProviderPayload = ""
	return &safe
}

func roundWithdrawalAmount(amount float64) float64 {
	return decimal.NewFromFloat(amount).Round(2).InexactFloat64()
}

func isValidAgentWithdrawalStatus(status string) bool {
	switch status {
	case AgentWithdrawalStatusPending,
		AgentWithdrawalStatusRejected,
		AgentWithdrawalStatusTransferring,
		AgentWithdrawalStatusPaid,
		AgentWithdrawalStatusFailed:
		return true
	default:
		return false
	}
}

func getAgentTotalRebateAmountTx(tx *gorm.DB, agentId int) (float64, error) {
	var total float64
	if err := tx.Model(&AgentRebateRecord{}).
		Where("agent_id = ?", agentId).
		Select("COALESCE(sum(rebate_money), 0)").
		Scan(&total).Error; err != nil {
		return 0, err
	}
	return roundWithdrawalAmount(total), nil
}

func getAgentOccupiedWithdrawalAmountTx(tx *gorm.DB, agentId int) (float64, error) {
	var occupied float64
	if err := tx.Model(&AgentWithdrawal{}).
		Where("agent_id = ? AND status IN ?", agentId, []string{
			AgentWithdrawalStatusPending,
			AgentWithdrawalStatusTransferring,
			AgentWithdrawalStatusPaid,
		}).
		Select("COALESCE(sum(amount), 0)").
		Scan(&occupied).Error; err != nil {
		return 0, err
	}
	return roundWithdrawalAmount(occupied), nil
}

func getAgentWithdrawableAmountTx(tx *gorm.DB, agentId int) (float64, error) {
	total, err := getAgentTotalRebateAmountTx(tx, agentId)
	if err != nil {
		return 0, err
	}
	occupied, err := getAgentOccupiedWithdrawalAmountTx(tx, agentId)
	if err != nil {
		return 0, err
	}
	available := decimal.NewFromFloat(total).Sub(decimal.NewFromFloat(occupied))
	if available.IsNegative() {
		return 0, nil
	}
	return roundWithdrawalAmount(available.InexactFloat64()), nil
}

func GetAgentWithdrawalStats(agentId int) (*AgentWithdrawalStats, error) {
	stats := &AgentWithdrawalStats{}
	err := DB.Transaction(func(tx *gorm.DB) error {
		totalRebate, err := getAgentTotalRebateAmountTx(tx, agentId)
		if err != nil {
			return err
		}
		withdrawable, err := getAgentWithdrawableAmountTx(tx, agentId)
		if err != nil {
			return err
		}
		stats.TotalRebateAmount = totalRebate
		stats.WithdrawableAmount = withdrawable

		rows := []struct {
			Status string  `gorm:"column:status"`
			Amount float64 `gorm:"column:amount"`
		}{}
		if err := tx.Model(&AgentWithdrawal{}).
			Select("status, COALESCE(sum(amount), 0) AS amount").
			Where("agent_id = ?", agentId).
			Group("status").
			Find(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			switch row.Status {
			case AgentWithdrawalStatusPending:
				stats.PendingAmount = roundWithdrawalAmount(row.Amount)
			case AgentWithdrawalStatusTransferring:
				stats.TransferringAmount = roundWithdrawalAmount(row.Amount)
			case AgentWithdrawalStatusPaid:
				stats.PaidAmount = roundWithdrawalAmount(row.Amount)
			case AgentWithdrawalStatusRejected:
				stats.RejectedAmount = roundWithdrawalAmount(row.Amount)
			case AgentWithdrawalStatusFailed:
				stats.FailedAmount = roundWithdrawalAmount(row.Amount)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return stats, nil
}

func GetAgentWithdrawals(agentId int, status string, startTimestamp, endTimestamp int64, startIdx, num int) ([]AgentWithdrawalListItem, int64, error) {
	if status != "" && !isValidAgentWithdrawalStatus(status) {
		return nil, 0, errors.New("invalid withdrawal status")
	}
	query := DB.Model(&AgentWithdrawal{}).Where("agent_id = ?", agentId)
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if startTimestamp > 0 {
		query = query.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp > 0 {
		query = query.Where("created_at <= ?", endTimestamp)
	}

	var total int64
	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []AgentWithdrawalListItem{}, 0, nil
	}

	var items []AgentWithdrawalListItem
	err := query.Select("id, amount, status, trade_no, payee_account, payee_name, admin_remark, applicant_remark, reviewed_by, reviewed_at, transfer_time, alipay_order_id, alipay_pay_fund_order_id, transfer_status, failure_reason, created_at, updated_at").
		Order("id DESC").
		Offset(startIdx).
		Limit(num).
		Find(&items).Error
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func CreateAgentWithdrawal(agentId int, req *AgentWithdrawalCreateRequest) (*AgentWithdrawal, error) {
	if !setting.AgentWithdrawEnabled {
		return nil, errors.New("代理提现未启用")
	}

	applicantRemark := strings.TrimSpace(req.ApplicantRemark)
	amount := roundWithdrawalAmount(req.Amount)
	if amount <= 0 {
		return nil, errors.New("提现金额必须大于 0")
	}
	if amount < roundWithdrawalAmount(setting.AgentWithdrawMinAmount) {
		return nil, fmt.Errorf("提现金额不能低于 %.2f", setting.AgentWithdrawMinAmount)
	}

	withdrawal := &AgentWithdrawal{}
	err := DB.Transaction(func(tx *gorm.DB) error {
		agent := User{}
		agentQuery := tx.Select("id, withdraw_payee_account, withdraw_payee_name").Where("id = ?", agentId)
		if !common.UsingSQLite {
			agentQuery = agentQuery.Set("gorm:query_option", "FOR UPDATE")
		}
		if err := agentQuery.First(&agent).Error; err != nil {
			return err
		}
		payeeAccount := strings.TrimSpace(agent.WithdrawPayeeAccount)
		payeeName := strings.TrimSpace(agent.WithdrawPayeeName)
		if payeeAccount == "" {
			return errors.New("请先绑定支付宝账号")
		}
		if payeeName == "" {
			return errors.New("请先绑定收款人姓名")
		}

		available, err := getAgentWithdrawableAmountTx(tx, agentId)
		if err != nil {
			return err
		}
		if amount > available {
			return fmt.Errorf("可提现余额不足，当前最多可提现 %.2f", available)
		}

		tradeNo := fmt.Sprintf("AWDUSR%dNO%s%d", agentId, common.GetRandomString(12), time.Now().UnixNano())
		record := &AgentWithdrawal{
			AgentId:         agentId,
			Amount:          amount,
			Status:          AgentWithdrawalStatusPending,
			TradeNo:         tradeNo,
			PayeeAccount:    payeeAccount,
			PayeeName:       payeeName,
			ApplicantRemark: applicantRemark,
		}
		if err := tx.Create(record).Error; err != nil {
			return err
		}
		*withdrawal = *record
		return nil
	})
	if err != nil {
		return nil, err
	}
	return withdrawal, nil
}

func GetAdminAgentWithdrawalList(keyword, status string, startTimestamp, endTimestamp int64, startIdx, num int) ([]AdminAgentWithdrawalListItem, int64, error) {
	if status != "" && !isValidAgentWithdrawalStatus(status) {
		return nil, 0, errors.New("invalid withdrawal status")
	}
	query := DB.Table(AgentWithdrawal{}.TableName() + " AS w").
		Joins("LEFT JOIN users u ON u.id = w.agent_id").
		Select("w.id, w.amount, w.status, w.trade_no, w.payee_account, w.payee_name, w.admin_remark, w.applicant_remark, w.reviewed_by, w.reviewed_at, w.transfer_time, w.alipay_order_id, w.alipay_pay_fund_order_id, w.transfer_status, w.failure_reason, w.created_at, w.updated_at, COALESCE(NULLIF(TRIM(u.display_name), ''), u.username) AS agent_name")

	if keyword = strings.TrimSpace(keyword); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("u.username LIKE ? OR u.display_name LIKE ? OR w.trade_no LIKE ? OR w.payee_account LIKE ?", like, like, like, like)
	}
	if status != "" {
		query = query.Where("w.status = ?", status)
	}
	if startTimestamp > 0 {
		query = query.Where("w.created_at >= ?", startTimestamp)
	}
	if endTimestamp > 0 {
		query = query.Where("w.created_at <= ?", endTimestamp)
	}

	var total int64
	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []AdminAgentWithdrawalListItem{}, 0, nil
	}

	var items []AdminAgentWithdrawalListItem
	err := query.Order("w.id DESC").Offset(startIdx).Limit(num).Find(&items).Error
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func GetAgentWithdrawalByID(id int) (*AgentWithdrawal, error) {
	if id <= 0 {
		return nil, errors.New("invalid withdrawal id")
	}
	withdrawal := &AgentWithdrawal{}
	if err := DB.Where("id = ?", id).First(withdrawal).Error; err != nil {
		return nil, err
	}
	return withdrawal, nil
}

func ReviewAgentWithdrawal(ctx context.Context, id int, adminId int, approve bool, adminRemark string, transferFn func(context.Context, *AgentWithdrawal) (*alipay.FundTransUniTransferRsp, string, error)) (*AgentWithdrawalReviewResult, error) {
	adminRemark = strings.TrimSpace(adminRemark)

	if !approve {
		err := DB.Transaction(func(tx *gorm.DB) error {
			result := tx.Model(&AgentWithdrawal{}).
				Where("id = ? AND status = ?", id, AgentWithdrawalStatusPending).
				Updates(map[string]any{
					"status":       AgentWithdrawalStatusRejected,
					"reviewed_by":  adminId,
					"reviewed_at":  time.Now().Unix(),
					"admin_remark": adminRemark,
				})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return errors.New("提现申请不存在或已处理")
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		withdrawal, err := GetAgentWithdrawalByID(id)
		if err != nil {
			return nil, err
		}
		return &AgentWithdrawalReviewResult{
			Withdrawal: sanitizeAgentWithdrawalForResponse(withdrawal),
			Message:    "已拒绝提现申请",
		}, nil
	}

	var withdrawal AgentWithdrawal
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ?", id).First(&withdrawal).Error; err != nil {
			return err
		}
		if withdrawal.Status != AgentWithdrawalStatusPending && withdrawal.Status != AgentWithdrawalStatusFailed {
			return errors.New("当前状态不允许审核打款")
		}

		available, err := getAgentWithdrawableAmountTx(tx, withdrawal.AgentId)
		if err != nil {
			return err
		}
		if withdrawal.Status == AgentWithdrawalStatusPending {
			available = roundWithdrawalAmount(decimal.NewFromFloat(available).Add(decimal.NewFromFloat(withdrawal.Amount)).InexactFloat64())
		}
		if withdrawal.Amount > available {
			return fmt.Errorf("可提现余额不足，当前最多可提现 %.2f", available)
		}

		result := tx.Model(&AgentWithdrawal{}).
			Where("id = ? AND status IN ?", id, []string{AgentWithdrawalStatusPending, AgentWithdrawalStatusFailed}).
			Updates(map[string]any{
				"status":         AgentWithdrawalStatusTransferring,
				"reviewed_by":    adminId,
				"reviewed_at":    time.Now().Unix(),
				"admin_remark":   adminRemark,
				"failure_reason": "",
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return errors.New("提现申请不存在或已处理")
		}
		withdrawal.Status = AgentWithdrawalStatusTransferring
		withdrawal.ReviewedBy = adminId
		withdrawal.ReviewedAt = time.Now().Unix()
		withdrawal.AdminRemark = adminRemark
		withdrawal.FailureReason = ""
		return nil
	})
	if err != nil {
		return nil, err
	}

	rsp, payload, transferErr := transferFn(ctx, &withdrawal)
	if transferErr != nil {
		failReason := transferErr.Error()
		result := DB.Model(&AgentWithdrawal{}).Where("id = ? AND status = ?", withdrawal.Id, AgentWithdrawalStatusTransferring).Updates(map[string]any{
			"status":           AgentWithdrawalStatusFailed,
			"failure_reason":   failReason,
			"provider_payload": payload,
		})
		if result.Error != nil {
			return nil, result.Error
		}
		updated, getErr := GetAgentWithdrawalByID(withdrawal.Id)
		if getErr != nil {
			return nil, transferErr
		}
		if result.RowsAffected == 0 {
			return &AgentWithdrawalReviewResult{
				Withdrawal: sanitizeAgentWithdrawalForResponse(updated),
				Message:    "转账结果已返回，但提现单状态已被其他操作更新",
			}, nil
		}
		return &AgentWithdrawalReviewResult{
			Withdrawal: sanitizeAgentWithdrawalForResponse(updated),
			Message:    "审核通过，但支付宝转账失败",
		}, nil
	}

	nextStatus := AgentWithdrawalStatusTransferring
	message := "审核通过，转账处理中"
	transferTime := int64(0)
	if rsp != nil && strings.EqualFold(rsp.Status, "SUCCESS") {
		nextStatus = AgentWithdrawalStatusPaid
		message = "审核通过并转账成功"
		transferTime = time.Now().Unix()
	} else if rsp != nil && strings.EqualFold(rsp.Status, "FAIL") {
		nextStatus = AgentWithdrawalStatusFailed
		message = "审核通过，但支付宝返回转账失败"
	}

	updates := map[string]any{
		"status":                   nextStatus,
		"transfer_status":          "",
		"provider_payload":         payload,
		"alipay_order_id":          "",
		"alipay_pay_fund_order_id": "",
	}
	if rsp != nil {
		updates["transfer_status"] = rsp.Status
		updates["alipay_order_id"] = rsp.OrderId
		updates["alipay_pay_fund_order_id"] = rsp.PayFundOrderId
	}
	if transferTime > 0 {
		updates["transfer_time"] = transferTime
	}
	if nextStatus == AgentWithdrawalStatusFailed {
		failReason := "支付宝转账失败"
		if rsp != nil && rsp.SubMsg != "" {
			failReason = rsp.SubMsg
		} else if rsp != nil && rsp.Msg != "" {
			failReason = rsp.Msg
		}
		updates["failure_reason"] = failReason
	}
	result := DB.Model(&AgentWithdrawal{}).
		Where("id = ? AND status = ?", withdrawal.Id, AgentWithdrawalStatusTransferring).
		Updates(updates)
	if result.Error != nil {
		return nil, result.Error
	}

	updated, err := GetAgentWithdrawalByID(withdrawal.Id)
	if err != nil {
		return nil, err
	}
	if result.RowsAffected == 0 {
		return &AgentWithdrawalReviewResult{
			Withdrawal: sanitizeAgentWithdrawalForResponse(updated),
			Message:    "转账结果已返回，但提现单状态已被其他操作更新",
		}, nil
	}
	return &AgentWithdrawalReviewResult{
		Withdrawal: sanitizeAgentWithdrawalForResponse(updated),
		Message:    message,
	}, nil
}

func SyncAgentWithdrawalStatus(ctx context.Context, id int, queryFn func(context.Context, *AgentWithdrawal) (*alipay.FundTransCommonQueryRsp, string, error)) (*AgentWithdrawalReviewResult, error) {
	withdrawal, err := GetAgentWithdrawalByID(id)
	if err != nil {
		return nil, err
	}
	if withdrawal.Status != AgentWithdrawalStatusTransferring {
		return nil, errors.New("当前提现单不处于转账处理中")
	}

	rsp, payload, queryErr := queryFn(ctx, withdrawal)
	if queryErr != nil {
		return nil, queryErr
	}

	updates := map[string]any{
		"provider_payload": payload,
	}
	message := "提现状态已同步"
	if rsp != nil {
		updates["transfer_status"] = rsp.Status
		if rsp.OrderId != "" {
			updates["alipay_order_id"] = rsp.OrderId
		}
		if rsp.PayFundOrderId != "" {
			updates["alipay_pay_fund_order_id"] = rsp.PayFundOrderId
		}
		switch strings.ToUpper(strings.TrimSpace(rsp.Status)) {
		case "SUCCESS":
			updates["status"] = AgentWithdrawalStatusPaid
			updates["transfer_time"] = time.Now().Unix()
			message = "提现转账成功"
		case "FAIL", "REFUND":
			updates["status"] = AgentWithdrawalStatusFailed
			if strings.TrimSpace(rsp.FailReason) != "" {
				updates["failure_reason"] = rsp.FailReason
			} else if strings.TrimSpace(rsp.ErrorCode) != "" {
				updates["failure_reason"] = rsp.ErrorCode
			}
			message = "提现转账失败"
		default:
			message = "提现仍在处理中"
		}
	}
	result := DB.Model(&AgentWithdrawal{}).
		Where("id = ? AND status = ?", withdrawal.Id, AgentWithdrawalStatusTransferring).
		Updates(updates)
	if result.Error != nil {
		return nil, result.Error
	}

	updated, err := GetAgentWithdrawalByID(withdrawal.Id)
	if err != nil {
		return nil, err
	}
	if result.RowsAffected == 0 {
		return &AgentWithdrawalReviewResult{
			Withdrawal: sanitizeAgentWithdrawalForResponse(updated),
			Message:    "提现单状态已被其他操作更新，未覆盖当前状态",
		}, nil
	}
	return &AgentWithdrawalReviewResult{
		Withdrawal: sanitizeAgentWithdrawalForResponse(updated),
		Message:    message,
	}, nil
}

func MarkAgentWithdrawalFailed(id int, adminId int, reason string) (*AgentWithdrawalReviewResult, error) {
	_ = adminId
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "管理员手动标记失败"
	}

	result := DB.Model(&AgentWithdrawal{}).
		Where("id = ? AND status = ?", id, AgentWithdrawalStatusTransferring).
		Updates(map[string]any{
			"status":         AgentWithdrawalStatusFailed,
			"failure_reason": reason,
		})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, errors.New("当前提现单不处于转账处理中")
	}

	withdrawal, err := GetAgentWithdrawalByID(id)
	if err != nil {
		return nil, err
	}
	return &AgentWithdrawalReviewResult{
		Withdrawal: sanitizeAgentWithdrawalForResponse(withdrawal),
		Message:    "已标记为打款失败",
	}, nil
}
