package errors

import (
	"fmt"
	"io"
	"testing"
)

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

func TestIs(t *testing.T) {
	regularErr := fmt.Errorf("just a regular error")

	custErr := customIsError{
		Key: "TestForFun",
		Err: io.EOF,
	}

	shouldMatch := customIsError{
		Key: "TestForFun",
	}

	shouldNotMatch := customIsError{Key: "notOk"}

	tests := []struct {
		name     string
		target   error
		original error
		want     bool
	}{
		{name: "custom error with same key", target: custErr, original: shouldMatch, want: true},
		{name: "custom error with different key", target: custErr, original: shouldNotMatch, want: false},
		{name: "custom error with same key, wrapped", target: Wrap(custErr, 0), original: shouldMatch, want: true},
		{name: "custom error with different key, wrapped", target: Wrap(custErr, 0), original: shouldNotMatch, want: false},
		{name: "wrapped custom error with same key", target: custErr, original: Wrap(shouldMatch, 0), want: true},
		{name: "wrapped custom error with different key", target: custErr, original: Wrap(shouldNotMatch, 0), want: false},
		{name: "wrapped custom error with same key, wrapped", target: Wrap(custErr, 0), original: Wrap(shouldMatch, 0), want: true},
		{name: "wrapped custom error with different key, wrapped", target: Wrap(custErr, 0), original: Wrap(shouldNotMatch, 0), want: false},

		{name: "regular error", target: regularErr, original: regularErr, want: true},
		{name: "regular error, wrapped", target: Wrap(regularErr, 0), original: regularErr, want: true},
		{name: "regular error, wrapped", target: regularErr, original: Wrap(regularErr, 0), want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Is(tt.target, tt.original); got != tt.want {
				t.Errorf("Is() = %v, want %v", got, tt.want)
			}
		})
	}
}
