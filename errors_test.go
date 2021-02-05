// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import (
	"errors"
	"io"
	"testing"
)

const (
	someGlobalError  = jsonError("some global error")
	otherGlobalError = jsonError("other global error alt")
)

var (
	someStringError  = &stringError{str: "some string error"}
	otherStringError = &stringError{str: "other string error"}
	someWrapError    = &wrapError{str: "some wrap error", err: io.ErrShortWrite}
	otherWrapError   = &wrapError{str: "other wrap error", err: io.ErrShortWrite}
	someSyntaxError  = &SyntaxError{str: "some syntax error"}
	otherSyntaxError = &SyntaxError{str: "other syntax error"}
)

func TestErrorsIs(t *testing.T) {
	tests := []struct {
		err    error
		target error
		want   bool
	}{
		// Top-level Error should match itself (identity).
		{Error, Error, true},

		// All sub-error values should match the top-level Error value.
		{someGlobalError, Error, true},
		{someStringError, Error, true},
		{someWrapError, Error, true},
		{someSyntaxError, Error, true},

		// Top-level Error should not match any other sub-error value.
		{Error, someGlobalError, false},
		{Error, someStringError, false},
		{Error, someWrapError, false},
		{Error, someSyntaxError, false},

		// Sub-error values should match itself (identity).
		{someGlobalError, someGlobalError, true},
		{someStringError, someStringError, true},
		{someWrapError, someWrapError, true},
		{someSyntaxError, someSyntaxError, true},

		// Sub-error values should not match each other.
		{someGlobalError, someStringError, false},
		{someStringError, someWrapError, false},
		{someWrapError, someSyntaxError, false},
		{someSyntaxError, someGlobalError, false},

		// Sub-error values should not match other error values of same type.
		{someGlobalError, otherGlobalError, false},
		{someStringError, otherStringError, false},
		{someWrapError, otherWrapError, false},
		{someSyntaxError, otherSyntaxError, false},

		// Error should not match any other random error.
		{Error, nil, false},
		{nil, Error, false},
		{io.ErrShortWrite, Error, false},
		{Error, io.ErrShortWrite, false},
	}

	for _, tt := range tests {
		got := errors.Is(tt.err, tt.target)
		if got != tt.want {
			t.Errorf("errors.Is(%#v, %#v) = %v, want %v", tt.err, tt.target, got, tt.want)
		}
		// If the type supports the Is method,
		// it should behave the same way if called directly.
		if iserr, ok := tt.err.(interface{ Is(error) bool }); ok {
			got := iserr.Is(tt.target)
			if got != tt.want {
				t.Errorf("%#v.Is(%#v) = %v, want %v", tt.err, tt.target, got, tt.want)
			}
		}
	}
}
