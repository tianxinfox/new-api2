package model

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

type AgentTopUpRecord struct {
	RecordId      int     `json:"record_id"`
	UserId        int     `json:"user_id"`
	Username      string  `json:"username"`
	Source        string  `json:"source"`
	TradeNo       string  `json:"trade_no"`
	PaymentMethod string  `json:"payment_method"`
	Quota         int64   `json:"quota"`
	Money         float64 `json:"money"`
	CreatedAt     int64   `json:"created_at"`
}

// GetAgentTopUpRecords returns merged top-up records (online + redemption) for users invited by the agent.
func GetAgentTopUpRecords(agentId int, keyword string, startIdx, num int) ([]AgentTopUpRecord, int64, error) {
	if startIdx < 0 {
		startIdx = 0
	}
	if num <= 0 {
		num = common.ItemsPerPage
	}

	kw := strings.TrimSpace(keyword)
	if kw != "" {
		// Use ! as ESCAPE char for cross-DB compatibility (MySQL/PostgreSQL/SQLite).
		kw = strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(kw)
	}

	onlineWhere := "u.inviter_id = ? AND t.status = ? AND u.deleted_at IS NULL"
	onlineArgs := []interface{}{agentId, common.TopUpStatusSuccess}
	redeemWhere := "u.inviter_id = ? AND r.status = ? AND u.deleted_at IS NULL AND r.deleted_at IS NULL"
	redeemArgs := []interface{}{agentId, common.RedemptionCodeStatusUsed}

	if kw != "" {
		like := "%" + kw + "%"
		onlineWhere += " AND (u.username LIKE ? ESCAPE '!' OR t.trade_no LIKE ? ESCAPE '!')"
		onlineArgs = append(onlineArgs, like, like)
		redeemWhere += " AND (u.username LIKE ? ESCAPE '!')"
		redeemArgs = append(redeemArgs, like)
	}

	unionSQL := fmt.Sprintf(`
SELECT
	t.id AS record_id,
	t.user_id AS user_id,
	u.username AS username,
	'online' AS source,
	t.trade_no AS trade_no,
	t.payment_method AS payment_method,
	t.amount AS quota,
	t.money AS money,
	t.complete_time AS created_at
FROM top_ups AS t
JOIN users AS u ON u.id = t.user_id
WHERE %s
UNION ALL
SELECT
	r.id AS record_id,
	r.used_user_id AS user_id,
	u.username AS username,
	'redemption' AS source,
	'' AS trade_no,
	'redemption' AS payment_method,
	r.quota AS quota,
	0 AS money,
	r.redeemed_time AS created_at
FROM redemptions AS r
JOIN users AS u ON u.id = r.used_user_id
WHERE %s
`, onlineWhere, redeemWhere)

	args := make([]interface{}, 0, len(onlineArgs)+len(redeemArgs))
	args = append(args, onlineArgs...)
	args = append(args, redeemArgs...)

	countSQL := "SELECT COUNT(1) AS total FROM (" + unionSQL + ") AS combined"
	var total int64
	if err := DB.Raw(countSQL, args...).Scan(&total).Error; err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []AgentTopUpRecord{}, 0, nil
	}

	dataSQL := "SELECT * FROM (" + unionSQL + ") AS combined ORDER BY created_at DESC, record_id DESC LIMIT ? OFFSET ?"
	dataArgs := append(append([]interface{}{}, args...), num, startIdx)
	var records []AgentTopUpRecord
	if err := DB.Raw(dataSQL, dataArgs...).Scan(&records).Error; err != nil {
		return nil, 0, err
	}

	for i := range records {
		if records[i].Source == "redemption" {
			records[i].Money = float64(records[i].Quota) / common.QuotaPerUnit
		}
	}

	return records, total, nil
}
