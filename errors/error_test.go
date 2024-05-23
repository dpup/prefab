package errors

import (
	"io"
	"testing"
)

func TestIs(t *testing.T) {
	custErr := customIsError{
		Key: "TestForFun",
		Err: io.EOF,
	}

	shouldMatch := customIsError{
		Key: "TestForFun",
	}

	shouldNotMatch := customIsError{Key: "notOk"}

	if !Is(custErr, shouldMatch) {
		t.Errorf("custErr is not a TestForFun customError")
	}

	if Is(custErr, shouldNotMatch) {
		t.Errorf("custErr is a notOk customError")
	}

	if !Is(custErr, New(shouldMatch)) {
		t.Errorf("custErr is not a New(TestForFun customError)")
	}

	if Is(custErr, New(shouldNotMatch)) {
		t.Errorf("custErr is a New(notOk customError)")
	}

	if !Is(New(custErr), shouldMatch) {
		t.Errorf("New(custErr) is not a TestForFun customError")
	}

	if Is(New(custErr), shouldNotMatch) {
		t.Errorf("New(custErr) is a notOk customError")
	}

	if !Is(New(custErr), New(shouldMatch)) {
		t.Errorf("New(custErr) is not a New(TestForFun customError)")
	}

	if Is(New(custErr), New(shouldNotMatch)) {
		t.Errorf("New(custErr) is a New(notOk customError)")
	}
}

type customIsError struct {
	Key string
	Err error
}

func (ewci customIsError) Error() string {
	return "[" + ewci.Key + "]: " + ewci.Err.Error()
}

func (ewci customIsError) Is(target error) bool {
	matched, ok := target.(customIsError)
	return ok && matched.Key == ewci.Key
}
