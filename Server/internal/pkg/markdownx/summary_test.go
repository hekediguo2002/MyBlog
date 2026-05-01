package markdownx

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSummary_StripsCommonMarkup(t *testing.T) {
	in := "# Hello\n\nThis is **bold** and `code` and [link](https://x).\n\n```go\nfmt.Println(\"hi\")\n```\n\n后续中文内容也要保留。"
	got := Summary(in, 200)
	require.NotContains(t, got, "#")
	require.NotContains(t, got, "**")
	require.NotContains(t, got, "`")
	require.NotContains(t, got, "[")
	require.Contains(t, got, "Hello")
	require.Contains(t, got, "后续中文内容也要保留。")
}

func TestSummary_TruncatesByRune(t *testing.T) {
	in := strings.Repeat("中", 300)
	got := Summary(in, 200)
	require.Equal(t, 200, len([]rune(got)))
}
