package apperr

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNew_BasicShape(t *testing.T) {
	e := New(CodeInvalidParam, "bad name")
	require.Equal(t, 1001, e.Code)
	require.Equal(t, "bad name", e.Msg)
	require.Equal(t, 400, e.HTTP)
	require.Equal(t, "[1001] bad name", e.Error())
}

func TestWrap_KeepsCause(t *testing.T) {
	cause := errors.New("db gone")
	e := Wrap(CodeDBError, "db", cause)
	require.True(t, errors.Is(e, cause))
}

func TestAs_FromGenericErr(t *testing.T) {
	var dst *AppErr
	require.True(t, errors.As(New(CodeUnauthorized, "x"), &dst))
	require.Equal(t, 2001, dst.Code)
}
