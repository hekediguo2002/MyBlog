package idgen

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewUUID_Format(t *testing.T) {
	got := NewUUID()
	require.Regexp(t, regexp.MustCompile(`^[a-f0-9]{32}$`), got)
}

func TestNewUUID_Unique(t *testing.T) {
	require.NotEqual(t, NewUUID(), NewUUID())
}
