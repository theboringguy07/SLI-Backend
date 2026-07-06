package errors

import (
	"fmt"
)

type AppError struct {
	Code      ErrorCode `json:"code"`
	Message   string    `json:"message"`
	RequestID string    `json:"request_id,omitempty"`
	Err       error     `json:"-"`
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s (%s)", e.Code, e.Message, e.Err.Error())
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error {
	return e.Err
}

func New(code ErrorCode, message string) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
	}
}

func NewWithErr(code ErrorCode, message string, err error) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// ErrorResponse represents the JSON payload returned to the client
type ErrorResponse struct {
	Error struct {
		Code      string `json:"code"`
		Message   string `json:"message"`
		RequestID string `json:"request_id,omitempty"`
	} `json:"error"`
}

func ToErrorResponse(err error, requestID string) (*ErrorResponse, int) {
	appErr, ok := err.(*AppError)
	if !ok {
		appErr = NewWithErr(CodeInternalServer, "An unexpected error occurred", err)
	}

	resp := &ErrorResponse{}
	resp.Error.Code = string(appErr.Code)
	resp.Error.Message = appErr.Message
	resp.Error.RequestID = requestID

	return resp, appErr.Code.Status()
}
