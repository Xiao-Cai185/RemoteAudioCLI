package utils

import (
	"fmt"
)

// ErrorType represents different types of errors in the application
type ErrorType int

const (
	ErrUnknown ErrorType = iota
	ErrInvalidConfig
	ErrAudioDevice
	ErrAudioCapture
	ErrAudioPlayback
	ErrNetwork
	ErrConnection
	ErrProtocol
	ErrBuffer
	ErrTimeout
)

// String returns the string representation of the error type
func (e ErrorType) String() string {
	switch e {
	case ErrInvalidConfig:
		return "InvalidConfig"
	case ErrAudioDevice:
		return "AudioDevice"
	case ErrAudioCapture:
		return "AudioCapture"
	case ErrAudioPlayback:
		return "AudioPlayback"
	case ErrNetwork:
		return "Network"
	case ErrConnection:
		return "Connection"
	case ErrProtocol:
		return "Protocol"
	case ErrBuffer:
		return "Buffer"
	case ErrTimeout:
		return "Timeout"
	default:
		return "Unknown"
	}
}

// AppError represents an application-specific error
type AppError struct {
	Type    ErrorType
	Message string
	Cause   error
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Type.String(), e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Type.String(), e.Message)
}

// Unwrap returns the underlying error
func (e *AppError) Unwrap() error {
	return e.Cause
}

// NewAppError creates a new application error
func NewAppError(errType ErrorType, message string) *AppError {
	return &AppError{
		Type:    errType,
		Message: message,
		Cause:   nil,
	}
}

// NewAppErrorWithCause creates a new application error with an underlying cause
func NewAppErrorWithCause(errType ErrorType, message string, cause error) *AppError {
	return &AppError{
		Type:    errType,
		Message: message,
		Cause:   cause,
	}
}

// WrapError wraps an existing error with additional context
func WrapError(err error, errType ErrorType, message string) *AppError {
	if err == nil {
		return nil
	}
	
	// If it's already an AppError, preserve the original type if none specified
	if appErr, ok := err.(*AppError); ok && errType == ErrUnknown {
		return NewAppErrorWithCause(appErr.Type, message, appErr)
	}
	
	return NewAppErrorWithCause(errType, message, err)
}

// IsErrorType checks if an error is of a specific type
func IsErrorType(err error, errType ErrorType) bool {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Type == errType
	}
	return false
}

// GetErrorType returns the error type of an error
func GetErrorType(err error) ErrorType {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Type
	}
	return ErrUnknown
}

// Common error constructors for convenience

// ErrInvalidConfigf creates a formatted invalid configuration error
func ErrInvalidConfigf(format string, args ...interface{}) *AppError {
	return NewAppError(ErrInvalidConfig, fmt.Sprintf(format, args...))
}

// ErrAudioDevicef creates a formatted audio device error
func ErrAudioDevicef(format string, args ...interface{}) *AppError {
	return NewAppError(ErrAudioDevice, fmt.Sprintf(format, args...))
}

// ErrAudioCapturef creates a formatted audio capture error
func ErrAudioCapturef(format string, args ...interface{}) *AppError {
	return NewAppError(ErrAudioCapture, fmt.Sprintf(format, args...))
}

// ErrAudioPlaybackf creates a formatted audio playback error
func ErrAudioPlaybackf(format string, args ...interface{}) *AppError {
	return NewAppError(ErrAudioPlayback, fmt.Sprintf(format, args...))
}

// ErrNetworkf creates a formatted network error
func ErrNetworkf(format string, args ...interface{}) *AppError {
	return NewAppError(ErrNetwork, fmt.Sprintf(format, args...))
}

// ErrConnectionf creates a formatted connection error
func ErrConnectionf(format string, args ...interface{}) *AppError {
	return NewAppError(ErrConnection, fmt.Sprintf(format, args...))
}

// ErrProtocolf creates a formatted protocol error
func ErrProtocolf(format string, args ...interface{}) *AppError {
	return NewAppError(ErrProtocol, fmt.Sprintf(format, args...))
}

// ErrBufferf creates a formatted buffer error
func ErrBufferf(format string, args ...interface{}) *AppError {
	return NewAppError(ErrBuffer, fmt.Sprintf(format, args...))
}

// ErrTimeoutf creates a formatted timeout error
func ErrTimeoutf(format string, args ...interface{}) *AppError {
	return NewAppError(ErrTimeout, fmt.Sprintf(format, args...))
}