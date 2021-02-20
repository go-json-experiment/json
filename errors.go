// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import (
	"strconv"
	"strings"
)

// TODO: Should we Hyrum-proof error messages so that it is harder
// for faulty code to depend on the exact error message?
// Perhaps we should randomly change the prefix at init?
const errorPrefix = "json: "

// Error matches errors returned by this package according to errors.Is.
const Error = jsonError("json error")

type jsonError string

func (e jsonError) Error() string        { return string(e) }
func (e jsonError) Is(target error) bool { return e == target || target == Error }

type stringError struct {
	str string
}

func (e *stringError) Error() string        { return errorPrefix + e.str }
func (e *stringError) Is(target error) bool { return e == target || target == Error }

type wrapError struct {
	str string
	err error
}

func (e *wrapError) Error() string        { return errorPrefix + e.str + ": " + e.err.Error() }
func (e *wrapError) Unwrap() error        { return e.err }
func (e *wrapError) Is(target error) bool { return e == target || target == Error }

// SyntaxError is a description of a JSON syntax error.
type SyntaxError struct {
	Offset int64 // error occurred after processing Offset bytes
	str    string
}

func (e *SyntaxError) Error() string              { return errorPrefix + e.str }
func (e *SyntaxError) Is(target error) bool       { return e == target || target == Error }
func (e *SyntaxError) withOffset(pos int64) error { return &SyntaxError{Offset: pos, str: e.str} }

func newInvalidCharacterError(c byte, where string) *SyntaxError {
	return &SyntaxError{str: "invalid character " + escapeCharacter(c) + " " + where}
}

func escapeCharacter(c byte) string {
	switch c {
	case '\'':
		return `'\''`
	case '"':
		return `'"'`
	default:
		return "'" + strings.TrimPrefix(strings.TrimSuffix(strconv.Quote(string([]byte{c})), `"`), `"`) + "'"
	}
}
