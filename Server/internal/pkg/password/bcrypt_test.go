package password

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHashCompare_RoundTrip(t *testing.T) {
	h, err := Hash("Password123")
	require.NoError(t, err)
	require.Len(t, h, 60)
	require.True(t, Compare(h, "Password123"))
	require.False(t, Compare(h, "wrong"))
}
