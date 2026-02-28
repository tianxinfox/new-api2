package model

import (
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

const (
	AgentRebateSourceTopUp      = "topup"
	AgentRebateSourceRedemption = "redemption"
)

type AgentRebateRecord struct {
	Id          int     `json:"id"`
	AgentId     int     `json:"agent_id" gorm:"index;not null"`
	SubUserId   int     `json:"sub_user_id" gorm:"index;not null"`
	SourceType  string  `json:"source_type" gorm:"type:varchar(32);not null;uniqueIndex:uk_agent_rebate_source"`
	SourceId    int     `json:"source_id" gorm:"not null;uniqueIndex:uk_agent_rebate_source"`
	TradeNo     string  `json:"trade_no" gorm:"type:varchar(255);default:''"`
	SourceMoney float64 `json:"source_money"`
	SourceQuota int64   `json:"source_quota"`
	RebateRate  int     `json:"rebate_rate"` // bps, 10000 = 100%
	RebateMoney float64 `json:"rebate_money"`
	RebateQuota int64   `json:"rebate_quota"`
	CreatedAt   int64   `json:"created_at" gorm:"bigint;autoCreateTime"`
	SettledAt   int64   `json:"settled_at" gorm:"bigint"`
}

func (AgentRebateRecord) TableName() string {
	return "agent_rebate_records"
}

type AgentRebateListItem struct {
	Id          int     `json:"id"`
	SubUserId   int     `json:"sub_user_id"`
	Username    string  `json:"username"`
	SourceType  string  `json:"source_type"`
	TradeNo     string  `json:"trade_no"`
	SourceMoney float64 `json:"source_money"`
	SourceQuota int64   `json:"source_quota"`
	RebateRate  int     `json:"rebate_rate"`
	RebateMoney float64 `json:"rebate_money"`
	RebateQuota int64   `json:"rebate_quota"`
	CreatedAt   int64   `json:"created_at"`
	SettledAt   int64   `json:"settled_at"`
}

type AgentRebateStats struct {
	RangeRebateMoney float64 `json:"range_rebate_money"`
	RangeRebateQuota int64   `json:"range_rebate_quota"`
	RangeRebateCount int64   `json:"range_rebate_count"`
	TotalRebateMoney float64 `json:"total_rebate_money"`
	TotalRebateQuota int64   `json:"total_rebate_quota"`
	TotalRebateCount int64   `json:"total_rebate_count"`
}

func SettleAgentRebateForTopUp(topUp *TopUp, creditedQuota int64) error {
	if topUp == nil || topUp.Id == 0 || topUp.UserId == 0 {
		return nil
	}
	return settleAgentRebateBySource(
		AgentRebateSourceTopUp,
		topUp.Id,
		topUp.UserId,
		topUp.TradeNo,
		topUp.Money,
		creditedQuota,
	)
}

func SettleAgentRebateForRedemption(redemption *Redemption) error {
	return settleAgentRebateForRedemptionWithQuota(redemption, 0)
}

func settleAgentRebateForRedemptionWithQuota(redemption *Redemption, creditedQuota int64) error {
	if redemption == nil || redemption.Id == 0 || redemption.UsedUserId == 0 || redemption.Quota <= 0 {
		return nil
	}
	sourceQuota := creditedQuota
	if sourceQuota <= 0 {
		sourceQuota = int64(redemption.Quota)
	}
	if sourceQuota <= 0 {
		return nil
	}

	sourceMoney := decimal.NewFromInt(sourceQuota).
		Div(decimal.NewFromFloat(common.QuotaPerUnit)).
		InexactFloat64()
	return settleAgentRebateBySource(
		AgentRebateSourceRedemption,
		redemption.Id,
		redemption.UsedUserId,
		"",
		sourceMoney,
		sourceQuota,
	)
}

func settleAgentRebateBySource(sourceType string, sourceId int, subUserId int, tradeNo string, sourceMoney float64, sourceQuota int64) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var exists int64
		if err := tx.Model(&AgentRebateRecord{}).
			Where("source_type = ? AND source_id = ?", sourceType, sourceId).
			Count(&exists).Error; err != nil {
			return err
		}
		if exists > 0 {
			return nil
		}

		subUser := &User{}
		if err := tx.Select("id", "inviter_id").Where("id = ?", subUserId).First(subUser).Error; err != nil {
			return err
		}
		if subUser.InviterId == 0 {
			return nil
		}

		agent := &User{}
		if err := tx.Select("id", "role", "rebate_rate").Where("id = ?", subUser.InviterId).First(agent).Error; err != nil {
			return err
		}
		if agent.Role < common.RoleAgentUser || agent.RebateRate <= 0 {
			return nil
		}

		if agent.RebateRate > 10000 {
			return fmt.Errorf("invalid rebate rate for agent %d: %d", agent.Id, agent.RebateRate)
		}

		rebateMoneyDecimal := decimal.NewFromFloat(sourceMoney).
			Mul(decimal.NewFromInt(int64(agent.RebateRate))).
			Div(decimal.NewFromInt(10000))
		rebateQuota := rebateMoneyDecimal.
			Mul(decimal.NewFromFloat(common.QuotaPerUnit)).
			IntPart()
		if rebateQuota <= 0 {
			return nil
		}

		record := &AgentRebateRecord{
			AgentId:     agent.Id,
			SubUserId:   subUserId,
			SourceType:  sourceType,
			SourceId:    sourceId,
			TradeNo:     tradeNo,
			SourceMoney: sourceMoney,
			SourceQuota: sourceQuota,
			RebateRate:  agent.RebateRate,
			RebateMoney: rebateMoneyDecimal.InexactFloat64(),
			RebateQuota: rebateQuota,
			SettledAt:   common.GetTimestamp(),
		}
		if err := tx.Create(record).Error; err != nil {
			if common.IsDuplicateConstraintError(err) {
				return nil
			}
			return err
		}

		return tx.Model(&User{}).
			Where("id = ?", agent.Id).
			Updates(map[string]interface{}{
				"aff_quota":   gorm.Expr("aff_quota + ?", rebateQuota),
				"aff_history": gorm.Expr("aff_history + ?", rebateQuota),
			}).Error
	})
}

func GetAgentRebateRecords(agentId int, keyword string, startTimestamp, endTimestamp int64, startIdx, num int) ([]AgentRebateListItem, int64, error) {
	if startIdx < 0 {
		startIdx = 0
	}
	if num <= 0 {
		num = common.ItemsPerPage
	}
	kw := strings.TrimSpace(keyword)
	if kw != "" {
		kw = strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(kw)
	}

	query := DB.Table(AgentRebateRecord{}.TableName()+" AS r").
		Select("r.id, r.sub_user_id, u.username, r.source_type, r.trade_no, r.source_money, r.source_quota, r.rebate_rate, r.rebate_money, r.rebate_quota, r.created_at, r.settled_at").
		Joins("LEFT JOIN users AS u ON u.id = r.sub_user_id").
		Where("r.agent_id = ?", agentId)

	if startTimestamp > 0 {
		query = query.Where("r.created_at >= ?", startTimestamp)
	}
	if endTimestamp > 0 {
		query = query.Where("r.created_at <= ?", endTimestamp)
	}

	if kw != "" {
		like := "%" + kw + "%"
		query = query.Where("(u.username LIKE ? ESCAPE '!' OR r.trade_no LIKE ? ESCAPE '!')", like, like)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []AgentRebateListItem{}, 0, nil
	}

	var items []AgentRebateListItem
	if err := query.Order("r.created_at DESC, r.id DESC").Offset(startIdx).Limit(num).Scan(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func GetAgentRebateStats(agentId int, startTimestamp, endTimestamp int64) (*AgentRebateStats, error) {
	stats := &AgentRebateStats{}
	if startTimestamp == 0 {
		now := time.Now()
		startTimestamp = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()
	}
	if endTimestamp == 0 {
		endTimestamp = time.Now().Unix()
	}
	if endTimestamp < startTimestamp {
		endTimestamp = startTimestamp
	}

	type aggregateRow struct {
		RebateMoney float64 `gorm:"column:rebate_money"`
		RebateQuota int64   `gorm:"column:rebate_quota"`
		RebateCount int64   `gorm:"column:rebate_count"`
	}

	var totalRow aggregateRow
	if err := DB.Model(&AgentRebateRecord{}).
		Where("agent_id = ?", agentId).
		Select("COALESCE(sum(rebate_money), 0) AS rebate_money, COALESCE(sum(rebate_quota), 0) AS rebate_quota, COUNT(1) AS rebate_count").
		Scan(&totalRow).Error; err != nil {
		return nil, err
	}
	stats.TotalRebateMoney = totalRow.RebateMoney
	stats.TotalRebateQuota = totalRow.RebateQuota
	stats.TotalRebateCount = totalRow.RebateCount

	rangeQuery := DB.Model(&AgentRebateRecord{}).
		Where("agent_id = ? AND created_at >= ? AND created_at <= ?", agentId, startTimestamp, endTimestamp)
	var rangeRow aggregateRow
	if err := rangeQuery.
		Select("COALESCE(sum(rebate_money), 0) AS rebate_money, COALESCE(sum(rebate_quota), 0) AS rebate_quota, COUNT(1) AS rebate_count").
		Scan(&rangeRow).Error; err != nil {
		return nil, err
	}
	stats.RangeRebateMoney = rangeRow.RebateMoney
	stats.RangeRebateQuota = rangeRow.RebateQuota
	stats.RangeRebateCount = rangeRow.RebateCount

	return stats, nil
}
