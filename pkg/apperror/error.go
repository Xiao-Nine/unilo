package apperror

import (
	"errors"
	"fmt"
	"net/http"
)

type Error struct {
	Code    int
	Message string
	Err     error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *Error) Unwrap() error {
	return e.Err
}

func New(code int, message string) *Error {
	return &Error{Code: code, Message: message}
}

func Wrap(code int, message string, err error) *Error {
	return &Error{Code: code, Message: message, Err: err}
}

func BadRequest(message string) *Error   { return New(400, message) }
func Unauthorized(message string) *Error { return New(401, message) }
func Forbidden(message string) *Error    { return New(403, message) }
func NotFound(message string) *Error     { return New(404, message) }
func Conflict(message string) *Error     { return New(409, message) }
func Internal(err error) *Error          { return Wrap(500, "internal server error", err) }

func From(err error) *Error {
	if err == nil {
		return nil
	}
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr
	}
	return Internal(err)
}

func HTTPStatus(code int) int {
	switch code {
	case 400:
		return http.StatusBadRequest
	case 401:
		return http.StatusUnauthorized
	case 403:
		return http.StatusForbidden
	case 404:
		return http.StatusNotFound
	case 409:
		return http.StatusConflict
	case 500:
		return http.StatusInternalServerError
	default:
		return http.StatusOK
	}
}
