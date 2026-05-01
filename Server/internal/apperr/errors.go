package apperr

import "fmt"

const (
	CodeOK            = 0
	CodeInvalidParam  = 1001
	CodeUsernameTaken = 1010
	CodeFileType      = 1020
	CodeFileTooLarge  = 1021
	CodeNotFound      = 1030
	CodeUnauthorized  = 2001
	CodeForbidden     = 2002
	CodeBadCredential = 2010
	CodeRateLimited   = 2020
	CodeCSRFInvalid   = 2030
	CodeDBError       = 5001
	CodeRedisError    = 5002
	CodeUnknown       = 5099
)

type AppErr struct {
	Code    int
	Msg     string
	HTTP    int
	Wrapped error
}

func (e *AppErr) Error() string { return fmt.Sprintf("[%d] %s", e.Code, e.Msg) }
func (e *AppErr) Unwrap() error { return e.Wrapped }

func httpFor(code int) int {
	switch code {
	case CodeOK:
		return 200
	case CodeInvalidParam, CodeFileType:
		return 400
	case CodeUnauthorized, CodeBadCredential:
		return 401
	case CodeForbidden, CodeCSRFInvalid:
		return 403
	case CodeNotFound:
		return 404
	case CodeUsernameTaken:
		return 409
	case CodeFileTooLarge:
		return 413
	case CodeRateLimited:
		return 429
	default:
		return 500
	}
}

func New(code int, msg string) *AppErr {
	return &AppErr{Code: code, Msg: msg, HTTP: httpFor(code)}
}
func Wrap(code int, msg string, cause error) *AppErr {
	return &AppErr{Code: code, Msg: msg, HTTP: httpFor(code), Wrapped: cause}
}
