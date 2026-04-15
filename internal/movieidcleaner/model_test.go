package movieidcleaner

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCleanErrorError(t *testing.T) {
	tests := []struct {
		name     string
		err      *CleanError
		expected string
	}{
		{
			name:     "nil_error",
			err:      nil,
			expected: "",
		},
		{
			name:     "message_only",
			err:      &CleanError{Message: "something broke"},
			expected: "something broke",
		},
		{
			name:     "cause_only",
			err:      &CleanError{Cause: errors.New("root cause")},
			expected: "root cause",
		},
		{
			name:     "message_and_cause",
			err:      &CleanError{Message: "wrapper", Cause: errors.New("root")},
			expected: "wrapper: root",
		},
		{
			name:     "no_message_no_cause",
			err:      &CleanError{},
			expected: "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.err.Error())
		})
	}
}

func TestCleanErrorUnwrap(t *testing.T) {
	tests := []struct {
		name      string
		err       *CleanError
		wantCause error
	}{
		{
			name:      "nil_error",
			err:       nil,
			wantCause: nil,
		},
		{
			name:      "with_cause",
			err:       &CleanError{Message: "msg", Cause: errors.New("cause")},
			wantCause: errors.New("cause"),
		},
		{
			name:      "without_cause",
			err:       &CleanError{Message: "msg"},
			wantCause: nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.err.Unwrap()
			if tc.wantCause == nil {
				assert.Nil(t, got)
			} else {
				assert.EqualError(t, got, tc.wantCause.Error())
			}
		})
	}
}
