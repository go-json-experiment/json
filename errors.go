// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import (
	"errors"
	"reflect"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const errorPrefix = "json: "

// Error matches errors returned by this package according to errors.Is.
const Error = jsonError("json error")

type jsonError string

func (e jsonError) Error() string {
	return string(e)
}
func (e jsonError) Is(target error) bool {
	return e == target || target == Error
}

type ioError struct {
	action string // either "read" or "write"
	err    error
}

func (e *ioError) Error() string {
	return errorPrefix + e.action + " error: " + e.err.Error()
}
func (e *ioError) Unwrap() error {
	return e.err
}
func (e *ioError) Is(target error) bool {
	return e == target || target == Error || errors.Is(e.err, target)
}

// SemanticError describes an error determining the meaning
// of JSON data as Go data or vice-versa.
//
// The contents of this error as produced by this package may change over time.
type SemanticError struct {
	requireKeyedLiterals
	nonComparable

	action string // either "marshal" or "unmarshal"

	// ByteOffset indicates that an error occurred after this byte offset.
	ByteOffset int64
	// JSONPointer indicates that an error occurred within this JSON value
	// as indicated using the JSON Pointer notation (see RFC 6901).
	JSONPointer string

	// JSONKind is the JSON kind that could not be handled.
	JSONKind Kind // may be zero if unknown
	// GoType is the Go type that could not be handled.
	GoType reflect.Type // may be nil if unknown

	// Err is the underlying error.
	Err error // may be nil
}

func (e *SemanticError) Error() string {
	var sb strings.Builder
	sb.WriteString(errorPrefix)

	// Hyrum-proof the error message by deliberately switching between
	// two equivalent renderings of the same error message.
	// The randomization is tied to the Hyrum-proofing already applied
	// on map iteration in Go.
	for phrase := range map[string]struct{}{"cannot": {}, "unable to": {}} {
		sb.WriteString(phrase)
		break // use whichever phrase we get in the first iteration
	}

	// Format action.
	var preposition string
	switch e.action {
	case "marshal":
		sb.WriteString(" marshal")
		preposition = " from"
	case "unmarshal":
		sb.WriteString(" unmarshal")
		preposition = " into"
	default:
		sb.WriteString(" handle")
		preposition = " with"
	}

	// Format JSON kind.
	var omitPreposition bool
	switch e.JSONKind {
	case 'n':
		sb.WriteString(" JSON null")
	case 'f', 't':
		sb.WriteString(" JSON boolean")
	case '"':
		sb.WriteString(" JSON string")
	case '0':
		sb.WriteString(" JSON number")
	case '{', '}':
		sb.WriteString(" JSON object")
	case '[', ']':
		sb.WriteString(" JSON array")
	default:
		omitPreposition = true
	}

	// Format Go type.
	if e.GoType != nil {
		if !omitPreposition {
			sb.WriteString(preposition)
		}
		sb.WriteString(" Go value of type ")
		sb.WriteString(e.GoType.String())
	}

	// Format where.
	switch {
	case e.JSONPointer != "":
		sb.WriteString(" within JSON value at ")
		sb.WriteString(strconv.Quote(e.JSONPointer))
	case e.ByteOffset > 0:
		sb.WriteString(" after byte offset ")
		sb.WriteString(strconv.FormatInt(e.ByteOffset, 10))
	}

	// Format underlying error.
	if e.Err != nil {
		sb.WriteString(": ")
		sb.WriteString(e.Err.Error())
	}

	return sb.String()
}
func (e *SemanticError) Is(target error) bool {
	return e == target || target == Error || errors.Is(e.Err, target)
}
func (e *SemanticError) Unwrap() error {
	return e.Err
}

// SyntacticError is a description of a syntactic error that occurred when
// encoding or decoding JSON according to the grammar.
//
// The contents of this error as produced by this package may change over time.
type SyntacticError struct {
	requireKeyedLiterals
	nonComparable

	// ByteOffset indicates that an error occurred after this byte offset.
	ByteOffset int64
	str        string
}

func (e *SyntacticError) Error() string {
	return errorPrefix + e.str
}
func (e *SyntacticError) Is(target error) bool {
	return e == target || target == Error
}
func (e *SyntacticError) withOffset(pos int64) error {
	return &SyntacticError{ByteOffset: pos, str: e.str}
}

func newDuplicateNameError[Bytes ~[]byte | ~string](quoted Bytes) *SyntacticError {
	return &SyntacticError{str: "duplicate name " + string(quoted) + " in object"}
}

func newInvalidCharacterError[Bytes ~[]byte | ~string](prefix Bytes, where string) *SyntacticError {
	what := quoteRune(prefix)
	return &SyntacticError{str: "invalid character " + what + " " + where}
}

func newInvalidEscapeSequenceError[Bytes ~[]byte | ~string](what Bytes) *SyntacticError {
	label := "escape sequence"
	if len(what) > 6 {
		label = "surrogate pair"
	}
	needEscape := strings.IndexFunc(string(what), func(r rune) bool {
		return r == '`' || r == utf8.RuneError || unicode.IsSpace(r) || !unicode.IsPrint(r)
	}) >= 0
	if needEscape {
		return &SyntacticError{str: "invalid " + label + " " + strconv.Quote(string(what)) + " within string"}
	} else {
		return &SyntacticError{str: "invalid " + label + " `" + string(what) + "` within string"}
	}
}

func quoteRune[Bytes ~[]byte | ~string](b Bytes) string {
	r, n := utf8.DecodeRuneInString(string(truncateMaxUTF8(b)))
	if r == utf8.RuneError && n == 1 {
		return `'\x` + strconv.FormatUint(uint64(b[0]), 16) + `'`
	}
	return strconv.QuoteRune(r)
}

func firstError(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}
