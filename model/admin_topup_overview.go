package model

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

type AdminTopUpOverview struct {
	TotalTopupAmount  float64               `json:"total_topup_amount"`
	TotalTopupCount   int64                 `json:"total_topup_count"`
	TotalConsumeQuota int64                 `json:"total_consume_quota"`
	RangeTopupAmount  float64               `json:"range_topup_amount"`
	RangeTopupCount   int64                 `json:"range_topup_count"`
	RangeConsumeQuota int64                 `json:"range_consume_quota"`
	TodayTopupAmount  float64               `json:"today_topup_amount"`
	TodayConsumeQuota int64                 `json:"today_consume_quota"`
	DailyStats        []AdminTopUpDailyStat `json:"daily_stats"`
}

type AdminTopUpDailyStat struct {
	Date         string  `json:"date" gorm:"column:date"`
	TopupAmount  float64 `json:"topup_amount" gorm:"column:topup_amount"`
	TopupCount   int64   `json:"topup_count" gorm:"column:topup_count"`
	ConsumeQuota int64   `json:"consume_quota" gorm:"column:consume_quota"`
}

type AdminTopUpRecord struct {
	Id            int     `json:"id" gorm:"column:id"`
	UserId        int     `json:"user_id" gorm:"column:user_id"`
	Username      string  `json:"username" gorm:"column:username"`
	Source        string  `json:"source" gorm:"column:source"`
	Amount        int64   `json:"amount" gorm:"column:amount"`
	Money         float64 `json:"money" gorm:"column:money"`
	TradeNo       string  `json:"trade_no" gorm:"column:trade_no"`
	PaymentMethod string  `json:"payment_method" gorm:"column:payment_method"`
	CreateTime    int64   `json:"create_time" gorm:"column:create_time"`
	CompleteTime  int64   `json:"complete_time" gorm:"column:complete_time"`
	Status        string  `json:"status" gorm:"column:status"`
}

func buildUnixDateExpr(column string, dbType string) string {
	switch normalizeDBType(dbType) {
	case common.DatabaseTypePostgreSQL:
		return "TO_CHAR(TO_TIMESTAMP(" + column + "), 'YYYY-MM-DD')"
	case common.DatabaseTypeMySQL:
		return "DATE(FROM_UNIXTIME(" + column + "))"
	default:
		return "date(" + column + ", 'unixepoch')"
	}
}

func normalizeDBType(dbType string) string {
	switch dbType {
	case common.DatabaseTypePostgreSQL, common.DatabaseTypeMySQL, common.DatabaseTypeSQLite:
		return dbType
	default:
		return common.DatabaseTypeSQLite
	}
}

func currentDBType() string {
	switch {
	case common.UsingPostgreSQL:
		return common.DatabaseTypePostgreSQL
	case common.UsingMySQL:
		return common.DatabaseTypeMySQL
	default:
		return common.DatabaseTypeSQLite
	}
}

func currentLogDBType() string {
	if LOG_DB == DB {
		return currentDBType()
	}
	return normalizeDBType(common.LogSqlType)
}

func hasValidRange(startTimestamp, endTimestamp int64) bool {
	return startTimestamp > 0 && endTimestamp > 0 && endTimestamp >= startTimestamp
}

func normalizeDailyDateKey(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	layouts := []string{
		"2006-01-02",
		"2006-01-02 15:04:05",
		time.RFC3339,
		"2006-01-02T15:04:05Z07:00",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed.Format("2006-01-02")
		}
	}
	if len(raw) >= 10 {
		return raw[:10]
	}
	return raw
}

func buildIDTextCastExpr(column string) string {
	switch currentDBType() {
	case common.DatabaseTypeMySQL:
		return "CAST(" + column + " AS CHAR)"
	default:
		return "CAST(" + column + " AS TEXT)"
	}
}

func applyTopUpTimeRange(query *gorm.DB, startTimestamp, endTimestamp int64) *gorm.DB {
	if startTimestamp != 0 {
		query = query.Where("create_time >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		query = query.Where("create_time <= ?", endTimestamp)
	}
	return query
}

func applyLogTimeRange(query *gorm.DB, startTimestamp, endTimestamp int64) *gorm.DB {
	if startTimestamp != 0 {
		query = query.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		query = query.Where("created_at <= ?", endTimestamp)
	}
	return query
}

type adminTopupAggregate struct {
	TopupAmount float64 `gorm:"column:topup_amount"`
	TopupCount  int64   `gorm:"column:topup_count"`
}

func GetAdminTopUpOverview(startTimestamp, endTimestamp int64) (*AdminTopUpOverview, error) {
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()
	todayEnd := now.Unix()

	overview := &AdminTopUpOverview{
		DailyStats: []AdminTopUpDailyStat{},
	}

	if err := DB.Model(&TopUp{}).
		Select("COALESCE(sum(money), 0)").
		Where("status = ?", common.TopUpStatusSuccess).
		Scan(&overview.TotalTopupAmount).Error; err != nil {
		return nil, err
	}
	if err := DB.Model(&TopUp{}).
		Where("status = ?", common.TopUpStatusSuccess).
		Count(&overview.TotalTopupCount).Error; err != nil {
		return nil, err
	}
	var totalRedeemQuota int64
	if err := DB.Model(&Redemption{}).
		Select("COALESCE(sum(quota), 0)").
		Where("status = ? AND deleted_at IS NULL", common.RedemptionCodeStatusUsed).
		Scan(&totalRedeemQuota).Error; err != nil {
		return nil, err
	}
	var totalRedeemCount int64
	if err := DB.Model(&Redemption{}).
		Where("status = ? AND deleted_at IS NULL", common.RedemptionCodeStatusUsed).
		Count(&totalRedeemCount).Error; err != nil {
		return nil, err
	}
	overview.TotalTopupAmount += float64(totalRedeemQuota) / common.QuotaPerUnit
	overview.TotalTopupCount += totalRedeemCount

	if err := LOG_DB.Table("logs").
		Select("COALESCE(sum(quota), 0)").
		Where("type = ?", LogTypeConsume).
		Scan(&overview.TotalConsumeQuota).Error; err != nil {
		return nil, err
	}

	if hasValidRange(startTimestamp, endTimestamp) {
		var rangeAggregate adminTopupAggregate
		rangeTopUpQuery := DB.Model(&TopUp{}).
			Select("COALESCE(sum(money), 0) AS topup_amount, COUNT(1) AS topup_count").
			Where("status = ?", common.TopUpStatusSuccess)
		rangeTopUpQuery = applyTopUpTimeRange(rangeTopUpQuery, startTimestamp, endTimestamp)
		if err := rangeTopUpQuery.Scan(&rangeAggregate).Error; err != nil {
			return nil, err
		}
		overview.RangeTopupAmount = rangeAggregate.TopupAmount
		overview.RangeTopupCount = rangeAggregate.TopupCount
		var rangeRedeemQuota int64
		if err := DB.Model(&Redemption{}).
			Select("COALESCE(sum(quota), 0)").
			Where("status = ? AND deleted_at IS NULL AND redeemed_time >= ? AND redeemed_time <= ?", common.RedemptionCodeStatusUsed, startTimestamp, endTimestamp).
			Scan(&rangeRedeemQuota).Error; err != nil {
			return nil, err
		}
		var rangeRedeemCount int64
		if err := DB.Model(&Redemption{}).
			Where("status = ? AND deleted_at IS NULL AND redeemed_time >= ? AND redeemed_time <= ?", common.RedemptionCodeStatusUsed, startTimestamp, endTimestamp).
			Count(&rangeRedeemCount).Error; err != nil {
			return nil, err
		}
		overview.RangeTopupAmount += float64(rangeRedeemQuota) / common.QuotaPerUnit
		overview.RangeTopupCount += rangeRedeemCount

		rangeConsumeQuery := LOG_DB.Table("logs").
			Select("COALESCE(sum(quota), 0)").
			Where("type = ?", LogTypeConsume)
		rangeConsumeQuery = applyLogTimeRange(rangeConsumeQuery, startTimestamp, endTimestamp)
		if err := rangeConsumeQuery.Scan(&overview.RangeConsumeQuota).Error; err != nil {
			return nil, err
		}
	}

	if err := DB.Model(&TopUp{}).
		Select("COALESCE(sum(money), 0)").
		Where("status = ? AND create_time >= ? AND create_time <= ?", common.TopUpStatusSuccess, todayStart, todayEnd).
		Scan(&overview.TodayTopupAmount).Error; err != nil {
		return nil, err
	}
	var todayRedeemQuota int64
	if err := DB.Model(&Redemption{}).
		Select("COALESCE(sum(quota), 0)").
		Where("status = ? AND deleted_at IS NULL AND redeemed_time >= ? AND redeemed_time <= ?", common.RedemptionCodeStatusUsed, todayStart, todayEnd).
		Scan(&todayRedeemQuota).Error; err != nil {
		return nil, err
	}
	overview.TodayTopupAmount += float64(todayRedeemQuota) / common.QuotaPerUnit
	if err := LOG_DB.Table("logs").
		Select("COALESCE(sum(quota), 0)").
		Where("type = ? AND created_at >= ? AND created_at <= ?", LogTypeConsume, todayStart, todayEnd).
		Scan(&overview.TodayConsumeQuota).Error; err != nil {
		return nil, err
	}

	dailyStats, err := GetAdminTopUpDailyStats(startTimestamp, endTimestamp)
	if err != nil {
		return nil, err
	}
	overview.DailyStats = dailyStats

	return overview, nil
}

func GetAdminTopUpDailyStats(startTimestamp, endTimestamp int64) ([]AdminTopUpDailyStat, error) {
	if !hasValidRange(startTimestamp, endTimestamp) {
		return []AdminTopUpDailyStat{}, nil
	}

	topupDateExpr := buildUnixDateExpr("create_time", currentDBType())
	logDateExpr := buildUnixDateExpr("created_at", currentLogDBType())

	var topupRows []AdminTopUpDailyStat
	topupQuery := DB.Model(&TopUp{}).
		Select(topupDateExpr+" AS date, COALESCE(sum(money), 0) AS topup_amount, COUNT(1) AS topup_count").
		Where("status = ? AND create_time >= ? AND create_time <= ?", common.TopUpStatusSuccess, startTimestamp, endTimestamp).
		Group(topupDateExpr).
		Order("date asc")
	if err := topupQuery.Scan(&topupRows).Error; err != nil {
		return nil, err
	}
	var redeemRows []AdminTopUpDailyStat
	redeemQuery := DB.Model(&Redemption{}).
		Select(buildUnixDateExpr("redeemed_time", currentDBType())+" AS date, COALESCE(sum(quota), 0) / ? AS topup_amount, COUNT(1) AS topup_count", common.QuotaPerUnit).
		Where("status = ? AND deleted_at IS NULL AND redeemed_time >= ? AND redeemed_time <= ?", common.RedemptionCodeStatusUsed, startTimestamp, endTimestamp).
		Group(buildUnixDateExpr("redeemed_time", currentDBType())).
		Order("date asc")
	if err := redeemQuery.Scan(&redeemRows).Error; err != nil {
		return nil, err
	}

	var consumeRows []AdminTopUpDailyStat
	consumeQuery := LOG_DB.Table("logs").
		Select(logDateExpr+" AS date, COALESCE(sum(quota), 0) AS consume_quota").
		Where("type = ? AND created_at >= ? AND created_at <= ?", LogTypeConsume, startTimestamp, endTimestamp).
		Group(logDateExpr).
		Order("date asc")
	if err := consumeQuery.Scan(&consumeRows).Error; err != nil {
		return nil, err
	}

	statsMap := make(map[string]*AdminTopUpDailyStat)
	for _, row := range topupRows {
		dateKey := normalizeDailyDateKey(row.Date)
		if dateKey == "" {
			continue
		}
		item := row
		item.Date = dateKey
		statsMap[dateKey] = &item
	}
	for _, row := range redeemRows {
		dateKey := normalizeDailyDateKey(row.Date)
		if dateKey == "" {
			continue
		}
		if existing, ok := statsMap[dateKey]; ok {
			existing.TopupAmount += row.TopupAmount
			existing.TopupCount += row.TopupCount
			continue
		}
		item := row
		item.Date = dateKey
		statsMap[dateKey] = &item
	}
	for _, row := range consumeRows {
		dateKey := normalizeDailyDateKey(row.Date)
		if dateKey == "" {
			continue
		}
		if existing, ok := statsMap[dateKey]; ok {
			existing.ConsumeQuota = row.ConsumeQuota
			continue
		}
		item := row
		item.Date = dateKey
		statsMap[dateKey] = &item
	}

	result := make([]AdminTopUpDailyStat, 0)
	startDate := time.Unix(startTimestamp, 0)
	endDate := time.Unix(endTimestamp, 0)
	current := time.Date(endDate.Year(), endDate.Month(), endDate.Day(), 0, 0, 0, 0, endDate.Location())
	first := time.Date(startDate.Year(), startDate.Month(), startDate.Day(), 0, 0, 0, 0, startDate.Location())
	for !current.Before(first) {
		dateKey := current.Format("2006-01-02")
		if item, ok := statsMap[dateKey]; ok {
			result = append(result, *item)
		} else {
			result = append(result, AdminTopUpDailyStat{
				Date: dateKey,
			})
		}
		current = current.AddDate(0, 0, -1)
	}

	return result, nil
}

func GetAdminTopUpRecords(pageInfo *common.PageInfo, keyword, status string, startTimestamp, endTimestamp int64) (records []*AdminTopUpRecord, total int64, err error) {
	if status != "" && status != common.TopUpStatusSuccess && status != common.TopUpStatusPending && status != common.TopUpStatusUnpaid && status != common.TopUpStatusExpired && status != common.TopUpStatusFailed {
		return nil, 0, errors.New("invalid topup status")
	}
	kw := strings.TrimSpace(keyword)
	if kw != "" {
		kw = strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(kw)
	}

	onlineWhere := "u.deleted_at IS NULL"
	onlineArgs := make([]interface{}, 0)
	if hasValidRange(startTimestamp, endTimestamp) {
		onlineWhere += " AND t.create_time >= ? AND t.create_time <= ?"
		onlineArgs = append(onlineArgs, startTimestamp, endTimestamp)
	}
	if status != "" {
		onlineWhere += " AND t.status = ?"
		onlineArgs = append(onlineArgs, status)
	}
	if kw != "" {
		like := "%" + kw + "%"
		onlineWhere += " AND (u.username LIKE ? ESCAPE '!' OR t.trade_no LIKE ? ESCAPE '!')"
		onlineArgs = append(onlineArgs, like, like)
	}

	unionSQL := fmt.Sprintf(`
SELECT
	t.id AS id,
	t.user_id AS user_id,
	u.username AS username,
	'online' AS source,
	t.amount AS amount,
	t.money AS money,
	t.trade_no AS trade_no,
	t.payment_method AS payment_method,
	t.create_time AS create_time,
	t.complete_time AS complete_time,
	t.status AS status
FROM top_ups AS t
JOIN users AS u ON u.id = t.user_id
WHERE %s
`, onlineWhere)

	args := append([]interface{}{}, onlineArgs...)
	if status == "" || status == common.TopUpStatusSuccess {
		redeemWhere := "u.deleted_at IS NULL AND r.deleted_at IS NULL AND r.status = ?"
		redeemArgs := []interface{}{common.RedemptionCodeStatusUsed}
		if hasValidRange(startTimestamp, endTimestamp) {
			redeemWhere += " AND r.redeemed_time >= ? AND r.redeemed_time <= ?"
			redeemArgs = append(redeemArgs, startTimestamp, endTimestamp)
		}
		if kw != "" {
			like := "%" + kw + "%"
			redeemWhere += " AND (u.username LIKE ? ESCAPE '!' OR r.name LIKE ? ESCAPE '!' OR " + buildIDTextCastExpr("r.id") + " LIKE ? ESCAPE '!')"
			redeemArgs = append(redeemArgs, like, like, like)
		}
		unionSQL += fmt.Sprintf(`
UNION ALL
SELECT
	r.id AS id,
	r.used_user_id AS user_id,
	u.username AS username,
	'redemption' AS source,
	r.quota AS amount,
	0 AS money,
	'' AS trade_no,
	'redemption' AS payment_method,
	r.redeemed_time AS create_time,
	r.redeemed_time AS complete_time,
	'success' AS status
FROM redemptions AS r
JOIN users AS u ON u.id = r.used_user_id
WHERE %s
`, redeemWhere)
		args = append(args, redeemArgs...)
	}

	countSQL := "SELECT COUNT(1) AS total FROM (" + unionSQL + ") AS combined"
	if err = DB.Raw(countSQL, args...).Scan(&total).Error; err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []*AdminTopUpRecord{}, 0, nil
	}

	dataSQL := "SELECT * FROM (" + unionSQL + ") AS combined ORDER BY create_time DESC, id DESC LIMIT ? OFFSET ?"
	dataArgs := append(append([]interface{}{}, args...), pageInfo.GetPageSize(), pageInfo.GetStartIdx())
	if err = DB.Raw(dataSQL, dataArgs...).Scan(&records).Error; err != nil {
		return nil, 0, err
	}
	for i := range records {
		if records[i].Source == "redemption" {
			records[i].Money = float64(records[i].Amount) / common.QuotaPerUnit
		}
	}
	return records, total, nil
}
