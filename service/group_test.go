package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsAutoTokenGroup(t *testing.T) {
	require.True(t, IsAutoTokenGroup("auto"))
	require.False(t, IsAutoTokenGroup("vip,auto"))
	require.False(t, IsAutoTokenGroup("vip,default"))
}

func TestGetTokenRouteGroupsFiltersUnavailableGroups(t *testing.T) {
	groups := GetTokenRouteGroups("default", "vip,default,backup,vip")
	require.Equal(t, []string{"vip", "default"}, groups)
}

func TestGetPrimaryTokenGroupWithExplicitGroups(t *testing.T) {
	require.Equal(t, "vip", GetPrimaryTokenGroup("default", "vip,default"))
	require.Equal(t, "default", GetPrimaryTokenGroup("default", ""))
}
