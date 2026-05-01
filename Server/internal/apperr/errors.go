package apperr

import (
	"fmt"
)

const (
	CodeOK           = 0
	CodeInvalidParam = 1001
	CodeUnauthorized = 2001
	CodeDBError      = 3001
	CodeUsernameTaken = 1010
	CodeUnknown      = 5099
	CodeNotFound     = 4040
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
	case CodeOK:
		return 200
	case CodeInvalidParam:
		return 400
	case CodeUsernameTaken:
		return 409
	case CodeUnauthorized:
		return 401
	case CodeDBError, CodeUnknown:
		return 500
	case CodeNotFound:
		return 404
	default:
		return 500
	}
}
