// Package errors is a fork of `github.com/go-errors/errors` that adds support
// for gRPC status codes, public messages, as well as stack-traces.
//
// This is particularly useful when you want to understand the
// state of execution when an error was returned unexpectedly.
//
// It provides the type *Error which implements the standard
// golang error interface, so you can use this library interchangably
// with code that is expecting a normal error return.
//
// For example:
//
//	package crashy
//
//	import "github.com/go-errors/errors"
//
//	var Crashed = errors.Errorf("something really bad just happened")
//
//	func Crash() error {
//	    return errors.NewC(Crashed, codes.Internal).WithPublicMessage("An unknown error occurred")
//	}
//
// This can be called as follows:
//
//	package main
//
//	import (
//	    "crashy"
//	    "fmt"
//	    "github.com/go-errors/errors"
//	)
//
//	func main() {
//	    err := crashy.Crash()
//	    if err != nil {
//	        if errors.Is(err, crashy.Crashed) {
//	            fmt.Println(err.(*errors.Error).ErrorStack())
//	        } else {
//	            panic(err)
//	        }
//	    }
//	}
package errors

import (
	"bytes"
	"fmt"
	"net/http"
	"reflect"
	"runtime"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/runtime/protoiface"
)

// The maximum number of stackframes on any error.
var MaxStackDepth = 50

// Error is an error with an attached stacktrace. It can be used
// wherever the builtin error interface is expected.
type Error struct {
	Err    error
	stack  []uintptr
	frames []StackFrame
	prefix string

	// gRPC status code to associate with an error response.
	code codes.Code

	// Error details which gRPC returns the client
	details []protoiface.MessageV1

	// HTTP status code to associate with an error response.
	httpStatusCode int

	// Error message to return to client,
	publicMessage string
}

// New makes an Error from the given value. If that value is already an
// error then it will be used directly, if not, it will be passed to
// fmt.Errorf("%v"). The stacktrace will point to the line of code that
// called New.
func New(e interface{}) *Error {
	return NewC(e, codes.Unknown)
}

// NewC makes an Error with a status code defined.
func NewC(e interface{}, code codes.Code) *Error {
	var err error

	switch e := e.(type) {
	case error:
		err = e
	default:
		err = fmt.Errorf("%v", e)
	}

	stack := make([]uintptr, MaxStackDepth)
	length := runtime.Callers(2, stack[:])
	return &Error{
		Err:   err,
		stack: stack[:length],
		code:  code,
	}
}

// Wrap makes an Error from the given value. If that value is already an
// error then it will be used directly, if not, it will be passed to
// fmt.Errorf("%v"). The skip parameter indicates how far up the stack
// to start the stacktrace. 0 is from the current call, 1 from its caller, etc.
func Wrap(e interface{}, skip int) *Error {
	if e == nil {
		return nil
	}

	var err error

	switch e := e.(type) {
	case *Error:
		return e
	case error:
		err = e
	default:
		err = fmt.Errorf("%v", e)
	}

	stack := make([]uintptr, MaxStackDepth)
	length := runtime.Callers(2+skip, stack[:])
	return &Error{
		Err:   err,
		stack: stack[:length],
		code:  codes.Unknown,
	}
}

// WrapPrefix makes an Error from the given value. If that value is already an
// error then it will be used directly, if not, it will be passed to
// fmt.Errorf("%v"). The prefix parameter is used to add a prefix to the
// error message when calling Error(). The skip parameter indicates how far
// up the stack to start the stacktrace. 0 is from the current call,
// 1 from its caller, etc.
func WrapPrefix(e interface{}, prefix string, skip int) *Error {
	if e == nil {
		return nil
	}

	err := Wrap(e, 1+skip)

	if err.prefix != "" {
		prefix = fmt.Sprintf("%s: %s", prefix, err.prefix)
	}

	return &Error{
		Err:            err.Err,
		stack:          err.stack,
		code:           err.code,
		details:        err.details,
		httpStatusCode: err.httpStatusCode,
		publicMessage:  err.publicMessage,
		prefix:         prefix,
	}
}

// Mark takes an error and sets the stack trace from the point it was called,
// overriding any previous stack trace that may have been set. The skip parameter
// indicates how far up the stack to start the stacktrace. 0 is from the current
// call, 1 from its caller, etc.
func Mark(e interface{}, skip int) *Error {
	if e == nil {
		return nil
	}
	if err, ok := e.(*Error); ok {
		stack := make([]uintptr, MaxStackDepth)
		length := runtime.Callers(2+skip, stack[:])
		return &Error{
			Err:            err.Err,
			stack:          stack[:length],
			code:           err.code,
			details:        err.details,
			httpStatusCode: err.httpStatusCode,
			publicMessage:  err.publicMessage,
			prefix:         err.prefix,
		}
	}

	// If the error is not an `Error`, we can just use wrap.
	return Wrap(e, 1+skip)
}

// WithPublicMessage takes an error message and adds a public message to it. If
// the error is not already an `Error`, it will be wrapped in one.
func WithPublicMessage(err error, publicMessage string) *Error {
	if err == nil {
		return nil
	}
	return Wrap(err, 1).WithPublicMessage(publicMessage)
}

// WithCode takes an error and adds a gRPC status code to it. If the error is
// not already an `Error`, it will be wrapped in one.
func WithCode(err error, code codes.Code) *Error {
	if err == nil {
		return nil
	}
	return Wrap(err, 1).WithCode(code)
}

// WithHTTPStatusCode takes an error and adds an explicit HTTP status code to
// it, overriding the HTTP status mapped from the gRPC code.
func WithHTTPStatusCode(err error, code int) *Error {
	if err == nil {
		return nil
	}
	return Wrap(err, 1).WithHTTPStatusCode(code)
}

// WithDetails takes an error and adds gRPC details to it. If the error is
// not already an `Error`, it will be wrapped in one.
func WithDetails(err error, details ...protoiface.MessageV1) *Error {
	if err == nil {
		return nil
	}
	return Wrap(err, 1).WithDetails(details...)
}

// Errorf creates a new error with the given message. You can use it
// as a drop-in replacement for fmt.Errorf() to provide descriptive
// errors in return values.
func Errorf(format string, a ...interface{}) *Error {
	return Wrap(fmt.Errorf(format, a...), 1)
}

// Error returns the underlying error's message.
func (err *Error) Error() string {

	msg := err.Err.Error()
	if err.prefix != "" {
		msg = fmt.Sprintf("%s: %s", err.prefix, msg)
	}

	return msg
}

// Stack returns the callstack formatted the same way that go does
// in runtime/debug.Stack()
func (err *Error) Stack() []byte {
	buf := bytes.Buffer{}

	for _, frame := range err.StackFrames() {
		buf.WriteString(frame.String())
	}

	return buf.Bytes()
}

// Callers satisfies the bugsnag ErrorWithCallerS() interface
// so that the stack can be read out.
func (err *Error) Callers() []uintptr {
	return err.stack
}

// ErrorStack returns a string that contains both the
// error message and the callstack.
func (err *Error) ErrorStack() string {
	return err.TypeName() + " " + err.Error() + "\n" + string(err.Stack())
}

// StackFrames returns an array of frames containing information about the
// stack.
func (err *Error) StackFrames() []StackFrame {
	if err.frames == nil {
		err.frames = make([]StackFrame, len(err.stack))

		for i, pc := range err.stack {
			err.frames[i] = NewStackFrame(pc)
		}
	}

	return err.frames
}

// TypeName returns the type this error. e.g. *errors.stringError.
func (err *Error) TypeName() string {
	if _, ok := err.Err.(uncaughtPanic); ok {
		return "panic"
	}
	return reflect.TypeOf(err.Err).String()
}

// Unwrap the error (implements api for As function).
func (err *Error) Unwrap() error {
	return err.Err
}

// Code returns the gRPC status code associated with the error.
func (err *Error) Code() codes.Code {
	return err.code
}

// WithCode sets the gRPC status code associated with the error.
func (err *Error) WithCode(code codes.Code) *Error {
	err.code = code
	return err
}

// Details returns the gRPC details associated with the error.
func (err *Error) Details() []protoiface.MessageV1 {
	return err.details
}

// WithDetails sets the gRPC details associated with the error.
func (err *Error) WithDetails(details ...protoiface.MessageV1) *Error {
	err.details = append(err.details, details...)
	return err
}

// HTTPStatusCode returns the HTTP status code that should be returned to the
// client. If a code is set, it will be used, otherwise a default will be
// returned based on the gRPC code.
func (err *Error) HTTPStatusCode() int {
	if err.httpStatusCode != 0 {
		return err.httpStatusCode
	}
	switch err.code {
	case codes.OK:
		return http.StatusOK
	case codes.InvalidArgument, codes.OutOfRange:
		return http.StatusBadRequest
	case codes.DeadlineExceeded:
		return http.StatusGatewayTimeout
	case codes.NotFound:
		return http.StatusNotFound
	case codes.AlreadyExists:
		return http.StatusConflict
	case codes.PermissionDenied:
		return http.StatusForbidden
	case codes.Unauthenticated:
		return http.StatusUnauthorized
	case codes.ResourceExhausted:
		return http.StatusTooManyRequests
	case codes.FailedPrecondition:
		return http.StatusPreconditionFailed
	case codes.Unimplemented:
		return http.StatusNotImplemented
	case codes.Unavailable:
		return http.StatusServiceUnavailable

	case codes.Canceled, codes.Unknown, codes.Aborted, codes.Internal, codes.DataLoss:
		return http.StatusInternalServerError
	}

	return http.StatusInternalServerError
}

// WithHTTPStatusCode sets the HTTP status code that should be returned to the
// client.
func (err *Error) WithHTTPStatusCode(code int) *Error {
	err.httpStatusCode = code
	return err
}

// PublicMessage returns the error string that should be returned to the client.
func (err *Error) PublicMessage() string {
	if err.publicMessage != "" {
		return err.publicMessage
	}
	return err.Error()
}

// WithPublicMessage sets the error string that should be returned to the client.
func (err *Error) WithPublicMessage(publicMessage string) *Error {
	err.publicMessage = publicMessage
	return err
}

// GRPCStatus returns a gRPC status object for the error.
func (err *Error) GRPCStatus() *status.Status {
	st := status.New(err.Code(), err.PublicMessage())
	if len(err.details) > 0 {
		st, _ = st.WithDetails(err.details...)
	}
	return st
}

// Code returns a gRPC status code for an error. If the error is nil, it returns
// codes.OK. If error exposes a `Code()` method, it is returned. Otherwise
// codes.Internal is returned.
func Code(err error) codes.Code {
	if err == nil {
		return codes.OK
	}
	if e, ok := err.(codedError); ok {
		return e.Code()
	}
	return codes.Unknown
}

// HTTPStatusCode returns an HTTP status code for an error. If the error is nil,
// it returns http.StatusOK. If error exposes a `HTTPStatusCode()` method, it is
// returned. Otherwise http.StatusInternalServerError is returned.
func HTTPStatusCode(err error) int {
	if err == nil {
		return http.StatusOK
	}
	if e, ok := err.(httpError); ok {
		return e.HTTPStatusCode()
	}
	return http.StatusInternalServerError
}

type codedError interface {
	Code() codes.Code
}

type httpError interface {
	HTTPStatusCode() int
}
