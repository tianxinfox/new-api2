package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestNormalizeTokenGroup(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "empty", input: "", want: ""},
		{name: "single", input: "vip", want: "vip"},
		{name: "deduplicate_and_trim", input: " vip,default,vip , backup ", want: "vip,default,backup"},
		{name: "auto_only", input: "auto", want: "auto"},
		{name: "auto_with_other_group", input: "auto,vip", wantErr: true},
		{name: "too_many_groups", input: "g1,g2,g3,g4,g5,g6,g7,g8,g9,g10,g11", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeTokenGroup(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestTokenGetGroupListPreservesOrder(t *testing.T) {
	token := &Token{Group: "vip,default,backup,vip"}
	require.Equal(t, []string{"vip", "default", "backup"}, token.GetGroupList())
	require.Equal(t, "vip", token.GetPrimaryGroup())
}

func TestGetSatisfiedChannelPriorityCountFromCache(t *testing.T) {
	previousMemoryCacheEnabled := common.MemoryCacheEnabled
	previousGroupMap := group2model2channels
	previousChannels := channelsIDM
	t.Cleanup(func() {
		common.MemoryCacheEnabled = previousMemoryCacheEnabled
		group2model2channels = previousGroupMap
		channelsIDM = previousChannels
	})

	common.MemoryCacheEnabled = true

	priorityHigh := int64(10)
	priorityLow := int64(5)
	group2model2channels = map[string]map[string][]int{
		"vip": {
			"gpt-4o": {1, 2, 3},
		},
	}
	channelsIDM = map[int]*Channel{
		1: {Id: 1, Priority: &priorityHigh},
		2: {Id: 2, Priority: &priorityHigh},
		3: {Id: 3, Priority: &priorityLow},
	}

	count, err := GetSatisfiedChannelPriorityCount("vip", "gpt-4o")
	require.NoError(t, err)
	require.Equal(t, 2, count)
}

func TestGetSatisfiedChannelCountForRetryFromCache(t *testing.T) {
	previousMemoryCacheEnabled := common.MemoryCacheEnabled
	previousGroupMap := group2model2channels
	previousChannels := channelsIDM
	t.Cleanup(func() {
		common.MemoryCacheEnabled = previousMemoryCacheEnabled
		group2model2channels = previousGroupMap
		channelsIDM = previousChannels
	})

	common.MemoryCacheEnabled = true

	priorityHigh := int64(10)
	priorityLow := int64(5)
	group2model2channels = map[string]map[string][]int{
		"vip": {
			"gpt-4o": {1, 2, 3},
		},
	}
	channelsIDM = map[int]*Channel{
		1: {Id: 1, Priority: &priorityHigh},
		2: {Id: 2, Priority: &priorityHigh},
		3: {Id: 3, Priority: &priorityLow},
	}

	count, err := GetSatisfiedChannelCountForRetry("vip", "gpt-4o", 0)
	require.NoError(t, err)
	require.Equal(t, 2, count)

	count, err = GetSatisfiedChannelCountForRetry("vip", "gpt-4o", 1)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func TestGetSatisfiedChannelCountForRetryDB(t *testing.T) {
	previousMemoryCacheEnabled := common.MemoryCacheEnabled
	previousGroupCol := commonGroupCol
	t.Cleanup(func() {
		common.MemoryCacheEnabled = previousMemoryCacheEnabled
		commonGroupCol = previousGroupCol
		DB.Exec("DELETE FROM abilities")
	})

	require.NoError(t, DB.AutoMigrate(&Ability{}))

	common.MemoryCacheEnabled = false
	commonGroupCol = "`group`"

	priorityHigh := int64(10)
	priorityLow := int64(5)
	require.NoError(t, DB.Create(&[]Ability{
		{Group: "vip", Model: "gpt-4o", ChannelId: 1, Enabled: true, Priority: &priorityHigh, Weight: 100},
		{Group: "vip", Model: "gpt-4o", ChannelId: 2, Enabled: true, Priority: &priorityHigh, Weight: 100},
		{Group: "vip", Model: "gpt-4o", ChannelId: 3, Enabled: true, Priority: &priorityLow, Weight: 100},
	}).Error)

	count, err := GetSatisfiedChannelCountForRetry("vip", "gpt-4o", 0)
	require.NoError(t, err)
	require.Equal(t, 2, count)

	count, err = GetSatisfiedChannelCountForRetry("vip", "gpt-4o", 1)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}
