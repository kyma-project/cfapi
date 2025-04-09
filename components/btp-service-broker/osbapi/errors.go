package osbapi

import (
	"net/http"
)

type ServiceBrokerErrorCode string

const (
	AsyncRequired           ServiceBrokerErrorCode = "AsyncRequired"
	ConcurrencyError        ServiceBrokerErrorCode = "ConcurrencyError"
	RequiresApp             ServiceBrokerErrorCode = "RequiresApp"
	MaintenanceInfoConflict ServiceBrokerErrorCode = "MaintenanceInfoConflict"
)

type ServiceBrokerError interface {
	HttpStatus() int
	ErrorCode() ServiceBrokerErrorCode
	Description() string
	InstanceUsable() *bool
	UpdateRepeatable() *bool
	Unwrap() error
	Error() string
}

type serviceBrokerError struct {
	cause            error
	httpStatus       int
	errorCode        ServiceBrokerErrorCode
	description      string
	instanceUsable   *bool
	updateRepeatable *bool
}

func (e serviceBrokerError) Error() string {
	if e.cause == nil {
		return "unknown"
	}

	return e.cause.Error()
}

func (e serviceBrokerError) Unwrap() error {
	return e.cause
}

func (e serviceBrokerError) HttpStatus() int {
	return e.httpStatus
}

func (e serviceBrokerError) ErrorCode() ServiceBrokerErrorCode {
	return e.errorCode
}

func (e serviceBrokerError) Description() string {
	return e.description
}

func (e serviceBrokerError) InstanceUsable() *bool {
	return e.instanceUsable
}

func (e serviceBrokerError) UpdateRepeatable() *bool {
	return e.updateRepeatable
}

type AsyncRequiredError struct {
	serviceBrokerError
}

func NewAsyncRequiredError(description string) AsyncRequiredError {
	return AsyncRequiredError{
		serviceBrokerError{
			httpStatus:  http.StatusUnprocessableEntity,
			errorCode:   AsyncRequired,
			description: description,
		},
	}
}

type UnknownError struct {
	serviceBrokerError
}

func NewUnknownError(cause error) UnknownError {
	return UnknownError{
		serviceBrokerError{
			cause:       cause,
			httpStatus:  http.StatusInternalServerError,
			description: cause.Error(),
		},
	}
}

type NotFoundError struct {
	serviceBrokerError
}

func NewNotFoundError(cause error, description string) NotFoundError {
	return NotFoundError{
		serviceBrokerError{
			cause:       cause,
			httpStatus:  http.StatusNotFound,
			description: description,
		},
	}
}
