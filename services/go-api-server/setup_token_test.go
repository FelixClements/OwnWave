package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewSetupToken(t *testing.T) {
	configured := newSetupToken("  from-env-0123456789 ")
	require.Equal(t, "from-env-0123456789", configured.value)
	require.False(t, configured.generated)

	a, b := newSetupToken(""), newSetupToken("   ")
	require.True(t, a.generated)
	require.GreaterOrEqual(t, len(a.value), 26)
	require.NotEqual(t, a.value, b.value)
}
