package controller

import (
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func parseOptionalTimestamp(c *gin.Context, key string) (int64, bool) {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return 0, true
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		common.ApiErrorMsg(c, key+" is invalid")
		return 0, false
	}
	return value, true
}

func GetAdminTopUpOverview(c *gin.Context) {
	startTimestamp, ok := parseOptionalTimestamp(c, "start_timestamp")
	if !ok {
		return
	}
	endTimestamp, ok := parseOptionalTimestamp(c, "end_timestamp")
	if !ok {
		return
	}
	if (startTimestamp == 0) != (endTimestamp == 0) || (startTimestamp > 0 && endTimestamp < startTimestamp) {
		common.ApiErrorMsg(c, "invalid time range")
		return
	}

	data, err := model.GetAdminTopUpOverview(startTimestamp, endTimestamp)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, data)
}

func GetAdminTopUpRecords(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	startTimestamp, ok := parseOptionalTimestamp(c, "start_timestamp")
	if !ok {
		return
	}
	endTimestamp, ok := parseOptionalTimestamp(c, "end_timestamp")
	if !ok {
		return
	}
	if (startTimestamp == 0) != (endTimestamp == 0) || (startTimestamp > 0 && endTimestamp < startTimestamp) {
		common.ApiErrorMsg(c, "invalid time range")
		return
	}
	keyword := strings.TrimSpace(c.Query("keyword"))
	status := strings.TrimSpace(c.Query("status"))

	items, total, err := model.GetAdminTopUpRecords(pageInfo, keyword, status, startTimestamp, endTimestamp)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}
