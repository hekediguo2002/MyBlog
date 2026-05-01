package apperr

import (
	"fmt"
)

const (
	CodeInvalidParam  = 1001
	CodeUnauthorized  = 2001
	CodeDBError       = 3001
)

type AppErr struct {
	Code    int
	Msg     string
	HTTP    int
	Wrapped error
}

func (e *AppErr) Error() string {
	return fmt.Sprintf("[%d] %s", e.Code, e.Msg)
}

func (e *AppErr) Unwrap() error {
	return e.Wrapped
}

func New(code int, msg string) *AppErr {
	return &AppErr{
		Code: code,
		Msg:  msg,
		HTTP: httpFor(code),
	}
}

func Wrap(code int, msg string, cause error) *AppErr {
	return &AppErr{
		Code:    code,
		Msg:     msg,
		HTTP:    httpFor(code),
		Wrapped: cause,
	}
}

func httpFor(code int) int {
	switch code {
	case CodeInvalidParam:
		return 400
	case CodeUnauthorized:
		return 401
	case CodeDBError:
		return 500
	default:
		return 500
	}
}
