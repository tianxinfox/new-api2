package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCalculateGroupSwitchThreshold(t *testing.T) {
	require.Equal(t, 0, calculateGroupSwitchThreshold(1, 1))
	require.Equal(t, 4, calculateGroupSwitchThreshold(1, 5))
	require.Equal(t, 1, calculateGroupSwitchThreshold(2, 5))
	require.Equal(t, 2, calculateGroupSwitchThreshold(3, 1))
}
