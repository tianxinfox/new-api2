package controller

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	alipay "github.com/smartwalle/alipay/v3"
)

func IsAgentWithdrawConfigured() bool {
	return setting.AgentWithdrawEnabled && hasAlipayCredentialConfig() && hasAgentWithdrawTransferSceneReportInfosConfig()
}

func parseAgentWithdrawTransferSceneReportInfos(raw string) ([]*alipay.TransferSceneReportInfo, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("支付宝提现场景上报信息不能为空")
	}

	var infos []*alipay.TransferSceneReportInfo
	if err := common.UnmarshalJsonStr(raw, &infos); err != nil {
		return nil, fmt.Errorf("支付宝提现场景上报信息格式错误: %w", err)
	}
	if len(infos) == 0 {
		return nil, errors.New("支付宝提现场景上报信息至少填写一项")
	}
	for i, info := range infos {
		if info == nil {
			return nil, fmt.Errorf("支付宝提现场景上报信息第 %d 项不能为空", i+1)
		}
		info.InfoType = strings.TrimSpace(info.InfoType)
		info.InfoContent = strings.TrimSpace(info.InfoContent)
		if info.InfoType == "" {
			return nil, fmt.Errorf("支付宝提现场景上报信息第 %d 项缺少 info_type", i+1)
		}
		if info.InfoContent == "" {
			return nil, fmt.Errorf("支付宝提现场景上报信息第 %d 项缺少 info_content", i+1)
		}
	}
	return infos, nil
}

func validateAgentWithdrawTransferSceneReportInfos(raw string) error {
	_, err := parseAgentWithdrawTransferSceneReportInfos(raw)
	return err
}

func hasAgentWithdrawTransferSceneReportInfosConfig() bool {
	return validateAgentWithdrawTransferSceneReportInfos(setting.AgentWithdrawTransferSceneReportInfos) == nil
}

func buildAgentWithdrawalTransferPayload(withdrawal *model.AgentWithdrawal) (alipay.FundTransUniTransfer, error) {
	orderTitle := strings.TrimSpace(setting.AgentWithdrawOrderTitle)
	if orderTitle == "" {
		orderTitle = "代理佣金提现"
	}
	sceneName := strings.TrimSpace(setting.AgentWithdrawSceneName)
	if sceneName == "" {
		sceneName = "佣金报酬"
	}
	reportInfos, err := parseAgentWithdrawTransferSceneReportInfos(setting.AgentWithdrawTransferSceneReportInfos)
	if err != nil {
		return alipay.FundTransUniTransfer{}, err
	}

	return alipay.FundTransUniTransfer{
		OutBizNo:                withdrawal.TradeNo,
		TransAmount:             fmt.Sprintf("%.2f", withdrawal.Amount),
		ProductCode:             "TRANS_ACCOUNT_NO_PWD",
		BizScene:                "DIRECT_TRANSFER",
		OrderTitle:              orderTitle,
		Remark:                  strings.TrimSpace(withdrawal.AdminRemark),
		TransferSceneName:       sceneName,
		TransferSceneReportInfo: reportInfos,
		PayeeInfo: &alipay.PayeeInfo{
			Identity:     withdrawal.PayeeAccount,
			IdentityType: "ALIPAY_LOGON_ID",
			Name:         withdrawal.PayeeName,
		},
	}, nil
}

func executeAgentWithdrawalTransfer(ctx context.Context, withdrawal *model.AgentWithdrawal) (*alipay.FundTransUniTransferRsp, string, error) {
	client, err := getAlipayClient()
	if err != nil {
		return nil, "", err
	}
	param, err := buildAgentWithdrawalTransferPayload(withdrawal)
	if err != nil {
		return nil, "", err
	}
	rsp, err := client.FundTransUniTransfer(ctx, param)
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}
	payload := common.GetJsonString(gin.H{
		"request":  param,
		"response": rsp,
		"error":    errMsg,
	})
	if err != nil {
		return rsp, payload, err
	}
	if rsp == nil {
		return nil, payload, fmt.Errorf("alipay fund transfer returned empty response")
	}
	if rsp.Code != "10000" {
		msg := rsp.SubMsg
		if msg == "" {
			msg = rsp.Msg
		}
		return rsp, payload, fmt.Errorf("alipay fund transfer failed: [%s] %s", rsp.SubCode, msg)
	}
	return rsp, payload, nil
}

func queryAgentWithdrawalTransfer(ctx context.Context, withdrawal *model.AgentWithdrawal) (*alipay.FundTransCommonQueryRsp, string, error) {
	client, err := getAlipayClient()
	if err != nil {
		return nil, "", err
	}
	rsp, err := client.FundTransCommonQuery(ctx, alipay.FundTransCommonQuery{
		ProductCode: "TRANS_ACCOUNT_NO_PWD",
		BizScene:    "DIRECT_TRANSFER",
		OutBizNo:    withdrawal.TradeNo,
		OrderId:     withdrawal.AlipayOrderId,
	})
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}
	payload := common.GetJsonString(gin.H{
		"request": gin.H{
			"product_code": "TRANS_ACCOUNT_NO_PWD",
			"biz_scene":    "DIRECT_TRANSFER",
			"out_biz_no":   withdrawal.TradeNo,
			"order_id":     withdrawal.AlipayOrderId,
		},
		"response": rsp,
		"error":    errMsg,
	})
	if err != nil {
		return rsp, payload, err
	}
	if rsp == nil {
		return nil, payload, fmt.Errorf("alipay fund transfer query returned empty response")
	}
	if rsp.Code != "10000" {
		msg := rsp.SubMsg
		if msg == "" {
			msg = rsp.Msg
		}
		return rsp, payload, fmt.Errorf("alipay fund transfer query failed: [%s] %s", rsp.SubCode, msg)
	}
	return rsp, payload, nil
}

func GetAgentWithdrawalStats(c *gin.Context) {
	agentId := c.GetInt("id")
	stats, err := model.GetAgentWithdrawalStats(agentId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"enabled":    setting.AgentWithdrawEnabled,
		"configured": IsAgentWithdrawConfigured(),
		"min_amount": setting.AgentWithdrawMinAmount,
		"stats":      stats,
	})
}

func GetAgentWithdrawals(c *gin.Context) {
	agentId := c.GetInt("id")
	pageInfo := common.GetPageQuery(c)
	status := strings.TrimSpace(c.Query("status"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)

	items, total, err := model.GetAgentWithdrawals(agentId, status, startTimestamp, endTimestamp, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func CreateAgentWithdrawal(c *gin.Context) {
	var req model.AgentWithdrawalCreateRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	if !IsAgentWithdrawConfigured() {
		common.ApiErrorMsg(c, "支付宝提现配置未完成")
		return
	}
	withdrawal, err := model.CreateAgentWithdrawal(c.GetInt("id"), &req)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	common.ApiSuccess(c, withdrawal)
}

type agentWithdrawalReviewRequest struct {
	Approve     bool   `json:"approve"`
	AdminRemark string `json:"admin_remark"`
}

type agentWithdrawalFailRequest struct {
	Reason string `json:"reason"`
}

func GetAdminAgentWithdrawals(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	keyword := strings.TrimSpace(c.Query("keyword"))
	status := strings.TrimSpace(c.Query("status"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)

	items, total, err := model.GetAdminAgentWithdrawalList(keyword, status, startTimestamp, endTimestamp, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func ReviewAdminAgentWithdrawal(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "无效的提现 ID")
		return
	}

	var req agentWithdrawalReviewRequest
	if err = common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	if req.Approve && !IsAgentWithdrawConfigured() {
		common.ApiErrorMsg(c, "支付宝提现配置未完成")
		return
	}

	result, err := model.ReviewAgentWithdrawal(c.Request.Context(), id, c.GetInt("id"), req.Approve, req.AdminRemark, executeAgentWithdrawalTransfer)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	common.ApiSuccess(c, result)
}

func SyncAdminAgentWithdrawal(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "无效的提现 ID")
		return
	}
	result, err := model.SyncAgentWithdrawalStatus(c.Request.Context(), id, queryAgentWithdrawalTransfer)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	common.ApiSuccess(c, result)
}

func FailAdminAgentWithdrawal(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "无效的提现 ID")
		return
	}

	var req agentWithdrawalFailRequest
	if err = common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}

	result, err := model.MarkAgentWithdrawalFailed(id, c.GetInt("id"), req.Reason)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	common.ApiSuccess(c, result)
}
