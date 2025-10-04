package errs

import (
	"fmt"
	"net/http"
)

type AppError struct {
	Err  error
	Msg  string
	Type string
	Code int
}

func (e *AppError) Error() string {
	return fmt.Sprintf("type=%s, code=%d, msg=%s, err=%v\n", e.Type, e.Code, e.Msg, e.Err)
}

func NewAppError(err error, msg, typ string, code int) *AppError {
	return &AppError{
		Err:  err,
		Msg:  msg,
		Type: typ,
		Code: code,
	}
}

func NewInternalError(err error) *AppError {
	return &AppError{
		Err:  err,
		Msg:  "internal server error",
		Type: TypeInternal,
		Code: http.StatusInternalServerError,
	}
}
