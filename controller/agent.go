package controller

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

type AgentUserStatistics struct {
	Quota        int `json:"quota"`
	UsedQuota    int `json:"used_quota"`
	RequestCount int `json:"request_count"`
}

type AgentUserInviteInfo struct {
	AffCount        int `json:"aff_count"`
	AffHistoryQuota int `json:"aff_history_quota"`
	InviterId       int `json:"inviter_id"`
}

type AgentSubUserResponse struct {
	Id           int                 `json:"id"`
	Nickname     string              `json:"nickname"`
	Role         int                 `json:"role"`
	Group        string              `json:"group"`
	Statistics   AgentUserStatistics `json:"statistics"`
	InviteInfo   AgentUserInviteInfo `json:"invite_info"`
	RegisteredAt int64               `json:"registered_at"`
	IsActive     bool                `json:"is_active"`
}

func toAgentSubUserResponses(items []model.AgentSubUserItem) []AgentSubUserResponse {
	resp := make([]AgentSubUserResponse, 0, len(items))
	for _, item := range items {
		nickname := strings.TrimSpace(item.DisplayName)
		if nickname == "" {
			nickname = item.Username
		}
		resp = append(resp, AgentSubUserResponse{
			Id:       item.Id,
			Nickname: nickname,
			Role:     item.Role,
			Group:    item.Group,
			Statistics: AgentUserStatistics{
				Quota:        item.Quota,
				UsedQuota:    item.UsedQuota,
				RequestCount: item.RequestCount,
			},
			InviteInfo: AgentUserInviteInfo{
				AffCount:        item.AffCount,
				AffHistoryQuota: item.AffHistoryQuota,
				InviterId:       item.InviterId,
			},
			RegisteredAt: item.RegisteredAt,
			IsActive:     item.Status == common.UserStatusEnabled && !item.DeletedAt.Valid,
		})
	}
	return resp
}

func GetAgentDashboard(c *gin.Context) {
	agentId := c.GetInt("id")
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)

	stats, err := model.GetAgentDashboardStats(agentId, startTimestamp, endTimestamp)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    stats,
	})
}

func GetAgentSubUsers(c *gin.Context) {
	agentId := c.GetInt("id")
	pageInfo := common.GetPageQuery(c)

	users, total, err := model.GetAgentSubUsers(agentId, pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(toAgentSubUserResponses(users))

	common.ApiSuccess(c, pageInfo)
}

func SearchAgentSubUsers(c *gin.Context) {
	agentId := c.GetInt("id")
	keyword := c.Query("keyword")
	pageInfo := common.GetPageQuery(c)

	users, total, err := model.SearchAgentSubUsers(agentId, keyword, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}

	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(toAgentSubUserResponses(users))

	common.ApiSuccess(c, pageInfo)
}

func GetAgentTopUps(c *gin.Context) {
	agentId := c.GetInt("id")
	pageInfo := common.GetPageQuery(c)
	keyword := strings.TrimSpace(c.Query("keyword"))

	items, total, err := model.GetAgentTopUpRecords(agentId, keyword, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}

	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func GetAgentRebates(c *gin.Context) {
	agentId := c.GetInt("id")
	pageInfo := common.GetPageQuery(c)
	keyword := strings.TrimSpace(c.Query("keyword"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)

	items, total, err := model.GetAgentRebateRecords(agentId, keyword, startTimestamp, endTimestamp, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}

	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func GetAgentRebateStats(c *gin.Context) {
	agentId := c.GetInt("id")
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)

	stats, err := model.GetAgentRebateStats(agentId, startTimestamp, endTimestamp)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, stats)
}
