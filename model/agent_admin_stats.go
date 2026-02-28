package model

import (
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

type AdminAgentSummary struct {
	TotalAgents           int64   `json:"total_agents"`
	TotalSubUsers         int64   `json:"total_sub_users"`
	TotalTopupAmount      float64 `json:"total_topup_amount"`
	TotalConsumptionQuota int64   `json:"total_consumption_quota"`
	TotalRequestCount     int64   `json:"total_request_count"`
	TotalRebateAmount     float64 `json:"total_rebate_amount"`
	NetContributionAmount float64 `json:"net_contribution_amount"`
}

type AdminAgentListItem struct {
	AgentId               int     `json:"agent_id"`
	AgentName             string  `json:"agent_name"`
	Status                int     `json:"status"`
	SubUserCount          int64   `json:"sub_user_count"`
	TotalTopupAmount      float64 `json:"total_topup_amount"`
	TotalConsumptionQuota int64   `json:"total_consumption_quota"`
	TotalRequestCount     int64   `json:"total_request_count"`
	TotalRebateAmount     float64 `json:"total_rebate_amount"`
	NetContributionAmount float64 `json:"net_contribution_amount"`
	LastActiveAt          int64   `json:"last_active_at"`
}

type adminAgentBasic struct {
	Id          int    `gorm:"column:id"`
	Username    string `gorm:"column:username"`
	DisplayName string `gorm:"column:display_name"`
	Status      int    `gorm:"column:status"`
}

type agentMetric struct {
	SubUserCount          int64
	TotalTopupAmount      float64
	TotalConsumptionQuota int64
	TotalRequestCount     int64
	TotalRebateAmount     float64
	LastActiveAt          int64
}

type subUserPair struct {
	Id        int `gorm:"column:id"`
	InviterId int `gorm:"column:inviter_id"`
}

type topupByUserRow struct {
	UserId int     `gorm:"column:user_id"`
	Amount float64 `gorm:"column:amount"`
}

type redemptionByUserRow struct {
	UserId int64 `gorm:"column:user_id"`
	Quota  int64 `gorm:"column:quota"`
}

type rebateByAgentRow struct {
	AgentId int     `gorm:"column:agent_id"`
	Amount  float64 `gorm:"column:amount"`
}

type logByUserRow struct {
	UserId     int   `gorm:"column:user_id"`
	Quota      int64 `gorm:"column:quota"`
	ReqCount   int64 `gorm:"column:req_count"`
	LastActive int64 `gorm:"column:last_active"`
}

func getAgentsSubQuery() *gorm.DB {
	return DB.Model(&User{}).Select("id").Where("role = ?", common.RoleAgentUser)
}

func getSubUsersSubQueryFromAgents(agentsSubQuery *gorm.DB) *gorm.DB {
	return DB.Model(&User{}).Select("id").Where("inviter_id IN (?)", agentsSubQuery)
}

func getAgentBasicsByIDs(agentIDs []int) (map[int]adminAgentBasic, error) {
	result := make(map[int]adminAgentBasic, len(agentIDs))
	if len(agentIDs) == 0 {
		return result, nil
	}
	var agents []adminAgentBasic
	if err := DB.Model(&User{}).
		Select("id, username, display_name, status").
		Where("id IN ?", agentIDs).
		Find(&agents).Error; err != nil {
		return nil, err
	}
	for _, agent := range agents {
		result[agent.Id] = agent
	}
	return result, nil
}

func buildRankItemsFromRows(rows []AdminAgentListItem, limit int) []AdminAgentListItem {
	if limit <= 0 {
		limit = 10
	}
	if limit > len(rows) {
		limit = len(rows)
	}
	return rows[:limit]
}

func normalizeAdminAgentRange(startTimestamp, endTimestamp int64) (bool, int64, int64) {
	if startTimestamp <= 0 && endTimestamp <= 0 {
		return false, 0, 0
	}
	if endTimestamp <= 0 {
		endTimestamp = time.Now().Unix()
	}
	return true, startTimestamp, endTimestamp
}

func getAgentBasics(keyword string, statusFilter int) ([]adminAgentBasic, error) {
	query := DB.Model(&User{}).
		Select("id, username, display_name, status").
		Where("role = ?", common.RoleAgentUser)

	if statusFilter >= 0 {
		query = query.Where("status = ?", statusFilter)
	}

	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("username LIKE ? OR display_name LIKE ?", like, like)
	}

	var agents []adminAgentBasic
	if err := query.Order("id DESC").Find(&agents).Error; err != nil {
		return nil, err
	}
	return agents, nil
}

func buildAgentMetrics(agentIDs []int, startTimestamp, endTimestamp int64) (map[int]*agentMetric, error) {
	metrics := make(map[int]*agentMetric, len(agentIDs))
	if len(agentIDs) == 0 {
		return metrics, nil
	}
	for _, agentId := range agentIDs {
		metrics[agentId] = &agentMetric{}
	}

	useRange, startTs, endTs := normalizeAdminAgentRange(startTimestamp, endTimestamp)

	var subUsers []subUserPair
	if err := DB.Model(&User{}).
		Select("id, inviter_id").
		Where("inviter_id IN ?", agentIDs).
		Find(&subUsers).Error; err != nil {
		return nil, err
	}

	userToAgent := make(map[int]int, len(subUsers))
	subUserIDs := make([]int, 0, len(subUsers))
	for _, item := range subUsers {
		userToAgent[item.Id] = item.InviterId
		subUserIDs = append(subUserIDs, item.Id)
		if m, ok := metrics[item.InviterId]; ok {
			m.SubUserCount++
		}
	}

	if len(subUserIDs) > 0 {
		var topupRows []topupByUserRow
		topupQuery := DB.Model(&TopUp{}).
			Select("user_id, COALESCE(sum(money), 0) AS amount").
			Where("status = ? AND user_id IN ?", common.TopUpStatusSuccess, subUserIDs)
		if useRange {
			topupQuery = topupQuery.Where("complete_time >= ? AND complete_time <= ?", startTs, endTs)
		}
		if err := topupQuery.Group("user_id").Find(&topupRows).Error; err != nil {
			return nil, err
		}
		for _, row := range topupRows {
			if agentId, ok := userToAgent[row.UserId]; ok {
				metrics[agentId].TotalTopupAmount += row.Amount
			}
		}

		var redemptionRows []redemptionByUserRow
		redemptionQuery := DB.Model(&Redemption{}).
			Select("used_user_id AS user_id, COALESCE(sum(quota), 0) AS quota").
			Where("status = ? AND used_user_id IN ?", common.RedemptionCodeStatusUsed, subUserIDs)
		if useRange {
			redemptionQuery = redemptionQuery.Where("redeemed_time >= ? AND redeemed_time <= ?", startTs, endTs)
		}
		if err := redemptionQuery.Group("used_user_id").Find(&redemptionRows).Error; err != nil {
			return nil, err
		}
		for _, row := range redemptionRows {
			if agentId, ok := userToAgent[int(row.UserId)]; ok {
				metrics[agentId].TotalTopupAmount += float64(row.Quota) / common.QuotaPerUnit
			}
		}

		var logRows []logByUserRow
		logQuery := LOG_DB.Table("logs").
			Select("user_id, COALESCE(sum(quota), 0) AS quota, count(*) AS req_count, COALESCE(max(created_at), 0) AS last_active").
			Where("type = ? AND user_id IN ?", LogTypeConsume, subUserIDs)
		if useRange {
			logQuery = logQuery.Where("created_at >= ? AND created_at <= ?", startTs, endTs)
		}
		if err := logQuery.Group("user_id").Find(&logRows).Error; err != nil {
			return nil, err
		}
		for _, row := range logRows {
			if agentId, ok := userToAgent[row.UserId]; ok {
				m := metrics[agentId]
				m.TotalConsumptionQuota += row.Quota
				m.TotalRequestCount += row.ReqCount
				if row.LastActive > m.LastActiveAt {
					m.LastActiveAt = row.LastActive
				}
			}
		}
	}

	var rebateRows []rebateByAgentRow
	rebateQuery := DB.Model(&AgentRebateRecord{}).
		Select("agent_id, COALESCE(sum(rebate_money), 0) AS amount").
		Where("agent_id IN ?", agentIDs)
	if useRange {
		rebateQuery = rebateQuery.Where("created_at >= ? AND created_at <= ?", startTs, endTs)
	}
	if err := rebateQuery.Group("agent_id").Find(&rebateRows).Error; err != nil {
		return nil, err
	}
	for _, row := range rebateRows {
		metrics[row.AgentId].TotalRebateAmount = row.Amount
	}

	return metrics, nil
}

func buildAdminAgentItems(agents []adminAgentBasic, metrics map[int]*agentMetric) []AdminAgentListItem {
	items := make([]AdminAgentListItem, 0, len(agents))
	for _, agent := range agents {
		m := metrics[agent.Id]
		name := strings.TrimSpace(agent.DisplayName)
		if name == "" {
			name = agent.Username
		}
		if m == nil {
			m = &agentMetric{}
		}
		items = append(items, AdminAgentListItem{
			AgentId:               agent.Id,
			AgentName:             name,
			Status:                agent.Status,
			SubUserCount:          m.SubUserCount,
			TotalTopupAmount:      m.TotalTopupAmount,
			TotalConsumptionQuota: m.TotalConsumptionQuota,
			TotalRequestCount:     m.TotalRequestCount,
			TotalRebateAmount:     m.TotalRebateAmount,
			NetContributionAmount: m.TotalTopupAmount - m.TotalRebateAmount,
			LastActiveAt:          m.LastActiveAt,
		})
	}
	return items
}

func sortAdminAgentItems(items []AdminAgentListItem, sortBy, sortOrder string) {
	desc := strings.ToLower(sortOrder) != "asc"
	lessFunc := func(i, j int) bool {
		if desc {
			return items[i].AgentId > items[j].AgentId
		}
		return items[i].AgentId < items[j].AgentId
	}

	switch sortBy {
	case "sub_user_count":
		lessFunc = func(i, j int) bool {
			if desc {
				return items[i].SubUserCount > items[j].SubUserCount
			}
			return items[i].SubUserCount < items[j].SubUserCount
		}
	case "total_topup_amount":
		lessFunc = func(i, j int) bool {
			if desc {
				return items[i].TotalTopupAmount > items[j].TotalTopupAmount
			}
			return items[i].TotalTopupAmount < items[j].TotalTopupAmount
		}
	case "total_consumption_quota":
		lessFunc = func(i, j int) bool {
			if desc {
				return items[i].TotalConsumptionQuota > items[j].TotalConsumptionQuota
			}
			return items[i].TotalConsumptionQuota < items[j].TotalConsumptionQuota
		}
	case "total_request_count":
		lessFunc = func(i, j int) bool {
			if desc {
				return items[i].TotalRequestCount > items[j].TotalRequestCount
			}
			return items[i].TotalRequestCount < items[j].TotalRequestCount
		}
	case "total_rebate_amount":
		lessFunc = func(i, j int) bool {
			if desc {
				return items[i].TotalRebateAmount > items[j].TotalRebateAmount
			}
			return items[i].TotalRebateAmount < items[j].TotalRebateAmount
		}
	case "net_contribution_amount":
		lessFunc = func(i, j int) bool {
			if desc {
				return items[i].NetContributionAmount > items[j].NetContributionAmount
			}
			return items[i].NetContributionAmount < items[j].NetContributionAmount
		}
	case "last_active_at":
		lessFunc = func(i, j int) bool {
			if desc {
				return items[i].LastActiveAt > items[j].LastActiveAt
			}
			return items[i].LastActiveAt < items[j].LastActiveAt
		}
	case "agent_id":
	}

	sort.SliceStable(items, lessFunc)
}

func paginateAdminAgentItems(items []AdminAgentListItem, startIdx, num int) []AdminAgentListItem {
	if startIdx < 0 {
		startIdx = 0
	}
	if num <= 0 {
		num = len(items)
	}
	if startIdx >= len(items) {
		return []AdminAgentListItem{}
	}
	end := startIdx + num
	if end > len(items) {
		end = len(items)
	}
	return items[startIdx:end]
}

func GetAdminAgentSummary(startTimestamp, endTimestamp int64) (*AdminAgentSummary, error) {
	summary := &AdminAgentSummary{}
	useRange, startTs, endTs := normalizeAdminAgentRange(startTimestamp, endTimestamp)

	agentsSubQuery := getAgentsSubQuery()
	subUsersSubQuery := getSubUsersSubQueryFromAgents(agentsSubQuery)

	if err := DB.Model(&User{}).Where("role = ?", common.RoleAgentUser).Count(&summary.TotalAgents).Error; err != nil {
		return nil, err
	}
	if err := DB.Model(&User{}).Where("inviter_id IN (?)", agentsSubQuery).Count(&summary.TotalSubUsers).Error; err != nil {
		return nil, err
	}

	var onlineTopup float64
	topupQuery := DB.Model(&TopUp{}).Select("COALESCE(sum(money), 0)").
		Where("status = ? AND user_id IN (?)", common.TopUpStatusSuccess, subUsersSubQuery)
	if useRange {
		topupQuery = topupQuery.Where("complete_time >= ? AND complete_time <= ?", startTs, endTs)
	}
	if err := topupQuery.Scan(&onlineTopup).Error; err != nil {
		return nil, err
	}

	var redeemQuota int64
	redeemQuery := DB.Model(&Redemption{}).Select("COALESCE(sum(quota), 0)").
		Where("status = ? AND used_user_id IN (?)", common.RedemptionCodeStatusUsed, subUsersSubQuery)
	if useRange {
		redeemQuery = redeemQuery.Where("redeemed_time >= ? AND redeemed_time <= ?", startTs, endTs)
	}
	if err := redeemQuery.Scan(&redeemQuota).Error; err != nil {
		return nil, err
	}
	summary.TotalTopupAmount = onlineTopup + float64(redeemQuota)/common.QuotaPerUnit

	rebateQuery := DB.Model(&AgentRebateRecord{}).Select("COALESCE(sum(rebate_money), 0)").
		Where("agent_id IN (?)", agentsSubQuery)
	if useRange {
		rebateQuery = rebateQuery.Where("created_at >= ? AND created_at <= ?", startTs, endTs)
	}
	if err := rebateQuery.Scan(&summary.TotalRebateAmount).Error; err != nil {
		return nil, err
	}

	type logAgg struct {
		Quota    int64 `gorm:"column:quota"`
		ReqCount int64 `gorm:"column:req_count"`
	}
	logTotal := logAgg{}
	if LOG_DB == DB {
		logQuery := LOG_DB.Table("logs").
			Select("COALESCE(sum(quota), 0) AS quota, count(*) AS req_count").
			Where("type = ? AND user_id IN (?)", LogTypeConsume, subUsersSubQuery)
		if useRange {
			logQuery = logQuery.Where("created_at >= ? AND created_at <= ?", startTs, endTs)
		}
		if err := logQuery.Scan(&logTotal).Error; err != nil {
			return nil, err
		}
	} else {
		var subUserIDs []int
		if err := DB.Model(&User{}).Where("inviter_id IN (?)", agentsSubQuery).Pluck("id", &subUserIDs).Error; err != nil {
			return nil, err
		}
		if len(subUserIDs) > 0 {
			logQuery := LOG_DB.Table("logs").
				Select("COALESCE(sum(quota), 0) AS quota, count(*) AS req_count").
				Where("type = ? AND user_id IN ?", LogTypeConsume, subUserIDs)
			if useRange {
				logQuery = logQuery.Where("created_at >= ? AND created_at <= ?", startTs, endTs)
			}
			if err := logQuery.Scan(&logTotal).Error; err != nil {
				return nil, err
			}
		}
	}

	summary.TotalConsumptionQuota = logTotal.Quota
	summary.TotalRequestCount = logTotal.ReqCount
	summary.NetContributionAmount = summary.TotalTopupAmount - summary.TotalRebateAmount
	return summary, nil
}

func GetAdminAgentList(keyword string, statusFilter int, startTimestamp, endTimestamp int64, sortBy, sortOrder string, startIdx, num int) ([]AdminAgentListItem, int64, error) {
	agents, err := getAgentBasics(keyword, statusFilter)
	if err != nil {
		return nil, 0, err
	}
	total := int64(len(agents))
	if len(agents) == 0 {
		return []AdminAgentListItem{}, 0, nil
	}

	agentIDs := make([]int, 0, len(agents))
	for _, agent := range agents {
		agentIDs = append(agentIDs, agent.Id)
	}
	metrics, err := buildAgentMetrics(agentIDs, startTimestamp, endTimestamp)
	if err != nil {
		return nil, 0, err
	}

	items := buildAdminAgentItems(agents, metrics)
	sortAdminAgentItems(items, sortBy, sortOrder)
	return paginateAdminAgentItems(items, startIdx, num), total, nil
}

func GetAdminAgentRank(metric string, startTimestamp, endTimestamp int64, limit int) ([]AdminAgentListItem, error) {
	useRange, startTs, endTs := normalizeAdminAgentRange(startTimestamp, endTimestamp)
	if limit <= 0 {
		limit = 10
	}

	// SQL aggregation fast path for metrics that can be sorted in SQL.
	switch metric {
	case "rebate":
		type rebateRankRow struct {
			AgentId int     `gorm:"column:agent_id"`
			Value   float64 `gorm:"column:value"`
		}
		var rows []rebateRankRow
		query := DB.Model(&AgentRebateRecord{}).
			Select("agent_id, COALESCE(sum(rebate_money), 0) AS value").
			Group("agent_id").
			Order("value DESC").
			Limit(limit)
		if useRange {
			query = query.Where("created_at >= ? AND created_at <= ?", startTs, endTs)
		}
		if err := query.Find(&rows).Error; err != nil {
			return nil, err
		}
		agentIDs := make([]int, 0, len(rows))
		for _, row := range rows {
			agentIDs = append(agentIDs, row.AgentId)
		}
		agentMap, err := getAgentBasicsByIDs(agentIDs)
		if err != nil {
			return nil, err
		}
		items := make([]AdminAgentListItem, 0, len(rows))
		for _, row := range rows {
			agent, ok := agentMap[row.AgentId]
			if !ok {
				continue
			}
			name := strings.TrimSpace(agent.DisplayName)
			if name == "" {
				name = agent.Username
			}
			items = append(items, AdminAgentListItem{
				AgentId:           agent.Id,
				AgentName:         name,
				Status:            agent.Status,
				TotalRebateAmount: row.Value,
			})
		}
		return buildRankItemsFromRows(items, limit), nil
	case "consumption", "requests":
		if LOG_DB == DB {
			type logRankRow struct {
				AgentId  int   `gorm:"column:agent_id"`
				Quota    int64 `gorm:"column:quota"`
				ReqCount int64 `gorm:"column:req_count"`
			}
			var rows []logRankRow
			query := LOG_DB.Table("logs AS l").
				Select("u.inviter_id AS agent_id, COALESCE(sum(l.quota), 0) AS quota, count(*) AS req_count").
				Joins("JOIN users u ON u.id = l.user_id").
				Joins("JOIN users a ON a.id = u.inviter_id AND a.role = ?", common.RoleAgentUser).
				Where("l.type = ?", LogTypeConsume).
				Group("u.inviter_id").
				Limit(limit)
			if useRange {
				query = query.Where("l.created_at >= ? AND l.created_at <= ?", startTs, endTs)
			}
			if metric == "requests" {
				query = query.Order("req_count DESC")
			} else {
				query = query.Order("quota DESC")
			}
			if err := query.Find(&rows).Error; err != nil {
				return nil, err
			}
			agentIDs := make([]int, 0, len(rows))
			for _, row := range rows {
				agentIDs = append(agentIDs, row.AgentId)
			}
			agentMap, err := getAgentBasicsByIDs(agentIDs)
			if err != nil {
				return nil, err
			}
			items := make([]AdminAgentListItem, 0, len(rows))
			for _, row := range rows {
				agent, ok := agentMap[row.AgentId]
				if !ok {
					continue
				}
				name := strings.TrimSpace(agent.DisplayName)
				if name == "" {
					name = agent.Username
				}
				items = append(items, AdminAgentListItem{
					AgentId:               agent.Id,
					AgentName:             name,
					Status:                agent.Status,
					TotalConsumptionQuota: row.Quota,
					TotalRequestCount:     row.ReqCount,
				})
			}
			return buildRankItemsFromRows(items, limit), nil
		}
	}

	// Fallback path (cross-DB or complex mixed metrics).
	agents, err := getAgentBasics("", -1)
	if err != nil {
		return nil, err
	}
	if len(agents) == 0 {
		return []AdminAgentListItem{}, nil
	}

	agentIDs := make([]int, 0, len(agents))
	for _, agent := range agents {
		agentIDs = append(agentIDs, agent.Id)
	}
	metrics, err := buildAgentMetrics(agentIDs, startTimestamp, endTimestamp)
	if err != nil {
		return nil, err
	}

	items := buildAdminAgentItems(agents, metrics)
	switch metric {
	case "consumption":
		sortAdminAgentItems(items, "total_consumption_quota", "desc")
	case "requests":
		sortAdminAgentItems(items, "total_request_count", "desc")
	case "rebate":
		sortAdminAgentItems(items, "total_rebate_amount", "desc")
	case "net":
		sortAdminAgentItems(items, "net_contribution_amount", "desc")
	default:
		sortAdminAgentItems(items, "total_topup_amount", "desc")
	}

	if limit <= 0 {
		limit = 10
	}
	if limit > len(items) {
		limit = len(items)
	}
	return items[:limit], nil
}
