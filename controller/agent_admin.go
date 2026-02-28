package controller

import (
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

func parseAdminAgentRange(c *gin.Context) (int64, int64) {
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)

	period := strings.TrimSpace(c.Query("period"))
	if period == "" {
		return startTimestamp, endTimestamp
	}

	now := time.Now()
	switch period {
	case "week":
		start := now.AddDate(0, 0, -7)
		return start.Unix(), now.Unix()
	case "month":
		start := now.AddDate(0, -1, 0)
		return start.Unix(), now.Unix()
	case "all":
		return 0, 0
	default:
		return startTimestamp, endTimestamp
	}
}

func GetAdminAgentSummary(c *gin.Context) {
	startTimestamp, endTimestamp := parseAdminAgentRange(c)
	stats, err := model.GetAdminAgentSummary(startTimestamp, endTimestamp)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, stats)
}

func GetAdminAgentList(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	keyword := strings.TrimSpace(c.Query("keyword"))
	sortBy := strings.TrimSpace(c.Query("sort_by"))
	sortOrder := strings.TrimSpace(c.Query("sort_order"))
	startTimestamp, endTimestamp := parseAdminAgentRange(c)

	statusFilter := -1
	if statusStr := strings.TrimSpace(c.Query("status")); statusStr != "" {
		if parsed, err := strconv.Atoi(statusStr); err == nil {
			statusFilter = parsed
		}
	}

	items, total, err := model.GetAdminAgentList(
		keyword,
		statusFilter,
		startTimestamp,
		endTimestamp,
		sortBy,
		sortOrder,
		pageInfo.GetStartIdx(),
		pageInfo.GetPageSize(),
	)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func GetAdminAgentRank(c *gin.Context) {
	metric := strings.TrimSpace(c.Query("metric"))
	if metric == "" {
		metric = "topup"
	}

	const maxRankLimit = 100
	limit := 10
	if limitStr := strings.TrimSpace(c.Query("limit")); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if limit > maxRankLimit {
		limit = maxRankLimit
	}

	startTimestamp, endTimestamp := parseAdminAgentRange(c)
	items, err := model.GetAdminAgentRank(metric, startTimestamp, endTimestamp, limit)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, items)
}
