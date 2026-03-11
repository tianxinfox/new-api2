package service

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

type RetryParam struct {
	Ctx          *gin.Context
	TokenGroup   string
	ModelName    string
	Retry        *int
	resetNextTry bool
}

func (p *RetryParam) GetRetry() int {
	if p.Retry == nil {
		return 0
	}
	return *p.Retry
}

func (p *RetryParam) SetRetry(retry int) {
	p.Retry = &retry
}

func (p *RetryParam) IncreaseRetry() {
	if p.resetNextTry {
		p.resetNextTry = false
		return
	}
	if p.Retry == nil {
		p.Retry = new(int)
	}
	*p.Retry++
}

func (p *RetryParam) ResetRetryNextTry() {
	p.resetNextTry = true
}

func prepareNextGroup(param *RetryParam, nextIndex int) {
	common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroupIndex, nextIndex)
	param.SetRetry(0)
	param.ResetRetryNextTry()
}

func calculateGroupSwitchThreshold(priorityCount, channelCountAtRetry int) int {
	if priorityCount <= 0 {
		return 0
	}
	// retry 在多优先级场景下表示“优先级偏移量”，不能直接映射为同优先级内的渠道重试次数。
	// 只有单优先级分组时，递增 retry 仍会命中同一优先级，此时才允许按渠道数量扩展重试窗口。
	if priorityCount == 1 && channelCountAtRetry > 1 {
		return channelCountAtRetry - 1
	}
	return priorityCount - 1
}

func CacheGetRandomSatisfiedChannel(param *RetryParam) (*model.Channel, string, error) {
	var channel *model.Channel
	var err error
	selectGroup := param.TokenGroup
	userGroup := common.GetContextKeyString(param.Ctx, constant.ContextKeyUserGroup)

	isAutoTokenGroup := IsAutoTokenGroup(param.TokenGroup)
	routeGroups := GetTokenRouteGroups(userGroup, param.TokenGroup)
	if len(routeGroups) == 0 {
		if isAutoTokenGroup {
			return nil, selectGroup, errors.New("auto groups is not enabled")
		}
		return nil, selectGroup, nil
	}

	if len(routeGroups) == 1 && !isAutoTokenGroup {
		selectGroup = routeGroups[0]
		common.SetContextKey(param.Ctx, constant.ContextKeyUsingGroup, selectGroup)
		channel, err = model.GetRandomSatisfiedChannel(selectGroup, param.ModelName, param.GetRetry())
		if err != nil {
			return nil, selectGroup, err
		}
		return channel, selectGroup, nil
	}

	startGroupIndex := 0
	if lastGroupIndex, exists := common.GetContextKeyType[int](param.Ctx, constant.ContextKeyAutoGroupIndex); exists {
		startGroupIndex = lastGroupIndex
	}
	if startGroupIndex < 0 {
		startGroupIndex = 0
	}
	if startGroupIndex >= len(routeGroups) {
		return nil, routeGroups[len(routeGroups)-1], nil
	}

	allowCrossGroupRetry := common.GetContextKeyBool(param.Ctx, constant.ContextKeyTokenCrossGroupRetry) || len(routeGroups) > 1
	selectGroup = routeGroups[startGroupIndex]

	for i := startGroupIndex; i < len(routeGroups); i++ {
		currentGroup := routeGroups[i]
		priorityRetry := param.GetRetry()
		if i > startGroupIndex {
			priorityRetry = 0
		}
		logger.LogDebug(param.Ctx, "Selecting group: %s, priorityRetry: %d", currentGroup, priorityRetry)

		channel, err = model.GetRandomSatisfiedChannel(currentGroup, param.ModelName, priorityRetry)
		if err != nil {
			return nil, currentGroup, err
		}
		priorityCount, err := model.GetSatisfiedChannelPriorityCount(currentGroup, param.ModelName)
		if err != nil {
			return nil, currentGroup, err
		}
		channelCountAtRetry, err := model.GetSatisfiedChannelCountForRetry(currentGroup, param.ModelName, priorityRetry)
		if err != nil {
			return nil, currentGroup, err
		}
		if channel == nil || priorityCount == 0 {
			logger.LogDebug(param.Ctx, "No available channel in group %s for model %s, trying next group", currentGroup, param.ModelName)
			common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroupIndex, i+1)
			param.SetRetry(0)
			continue
		}

		common.SetContextKey(param.Ctx, constant.ContextKeyUsingGroup, currentGroup)
		if isAutoTokenGroup {
			// ContextKeyAutoGroup is reserved for the literal "auto" token mode.
			// Explicit multi-group tokens rely on ContextKeyUsingGroup as the final billing/routing group.
			common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroup, currentGroup)
		}
		selectGroup = currentGroup

		switchRetryThreshold := calculateGroupSwitchThreshold(priorityCount, channelCountAtRetry)

		if allowCrossGroupRetry && priorityRetry >= switchRetryThreshold {
			logger.LogDebug(param.Ctx, "Current group %s exhausted retry window (priorityRetry=%d, priorityCount=%d, channelCountAtRetry=%d), preparing switch to next group", currentGroup, priorityRetry, priorityCount, channelCountAtRetry)
			prepareNextGroup(param, i+1)
		} else {
			common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroupIndex, i)
		}
		return channel, selectGroup, nil
	}
	return nil, selectGroup, nil
}
