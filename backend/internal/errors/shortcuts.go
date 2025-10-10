package errs

import "net/http"

func NewInternalError(err error) *AppError {
	return &AppError{
		Err:  err,
		Msg:  "internal server error",
		Type: TypeInternal,
		Code: http.StatusInternalServerError,
	}
}

func NewBadRequest(err error, msg string) *AppError {
	return &AppError{
		Err:  err,
		Msg:  msg,
		Type: TypeBadRequest,
		Code: http.StatusBadRequest,
	}
}

func NewNotFound(err error, msg string) *AppError {
	return &AppError{
		Err:  err,
		Msg:  msg,
		Type: TypeNotFound,
		Code: http.StatusNotFound,
	}
}

func NewConflict(err error, msg string) *AppError {
	return &AppError{
		Err:  err,
		Msg:  msg,
		Type: TypeConflict,
		Code: http.StatusConflict,
	}
}
