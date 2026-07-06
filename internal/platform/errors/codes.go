package errors

import "net/http"

type ErrorCode string

const (
	CodeInternalServer     ErrorCode = "INTERNAL_SERVER_ERROR"
	CodeUnauthorized       ErrorCode = "UNAUTHORIZED"
	CodeForbidden          ErrorCode = "FORBIDDEN"
	CodeNotFound           ErrorCode = "NOT_FOUND"
	CodeValidationFailed   ErrorCode = "VALIDATION_FAILED"
	CodeReportAlreadyLocked ErrorCode = "REPORT_ALREADY_LOCKED"
	CodeTokenExpired       ErrorCode = "TOKEN_EXPIRED"
	CodeWeekOutOfRange     ErrorCode = "WEEK_OUT_OF_RANGE"
	CodeInvalidReportType  ErrorCode = "INVALID_REPORT_TYPE"
	CodeDuplicateReport    ErrorCode = "DUPLICATE_REPORT"
	CodeInvalidDomain      ErrorCode = "INVALID_DOMAIN"
	CodeEditWindowClosed   ErrorCode = "EDIT_WINDOW_CLOSED"
	CodeBadRequest         ErrorCode = "BAD_REQUEST"
)

var codeToStatus = map[ErrorCode]int{
	CodeInternalServer:     http.StatusInternalServerError,
	CodeUnauthorized:       http.StatusUnauthorized,
	CodeForbidden:          http.StatusForbidden,
	CodeNotFound:           http.StatusNotFound,
	CodeValidationFailed:   http.StatusUnprocessableEntity,
	CodeReportAlreadyLocked: http.StatusConflict,
	CodeTokenExpired:       http.StatusUnauthorized,
	CodeWeekOutOfRange:     http.StatusBadRequest,
	CodeInvalidReportType:  http.StatusBadRequest,
	CodeDuplicateReport:    http.StatusConflict,
	CodeInvalidDomain:      http.StatusForbidden,
	CodeEditWindowClosed:   http.StatusForbidden,
	CodeBadRequest:         http.StatusBadRequest,
}

func (c ErrorCode) Status() int {
	if status, ok := codeToStatus[c]; ok {
		return status
	}
	return http.StatusInternalServerError
}
