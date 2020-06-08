package tracker

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestMetrics_String(t *testing.T) {
	m := getMetrics()
	require.Greater(t, m.GoRoutines, 0)
	s := m.String()
	require.True(t, len(s) > 100)
}
