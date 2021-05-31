// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import (
	"errors"
	"reflect"
	"strconv"
	"strings"
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

type wrapError struct {
	str string
	err error
}

func (e *wrapError) Error() string {
	return errorPrefix + e.str + ": " + e.err.Error()
}
func (e *wrapError) Unwrap() error {
	return e.err
}
func (e *wrapError) Is(target error) bool {
	return e == target || target == Error || errors.Is(e.err, target)
}

// TODO: Rename the exported error types?
//
// The words "semantic" and "syntactic" are adjectives.
// The words "semantics" and "syntax" are nouns.
// To be consistent, the error types should either be called
//	"SemanticError" and "SyntacticError", or
//	"SemanticsError" and "SyntaxError".
// Since "Error" is a noun and the word before it is usually an adjective,
// this suggests that "SemanticError" and "SyntacticError" are the right names.

// SemanticError describes an error determining the meaning
// of JSON data as Go data or vice-versa.
//
// The contents of this error as produced by this package may change over time.
type SemanticError struct {
	action string // either "marshal" or "unmarshal"

	// Offset indicates that an error occurred after processing Offset bytes.
	Offset int64
	// Pointer indicates that an error occurred within this specific JSON value
	// as indicated using the JSON Pointer notation (see RFC 6901).
	Pointer string
	// TODO: Rename Offset as ByteOffset and Pointer as JSONPointer?
	// If so, rename SyntaxError.Offset to be consistent.

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
	case e.Pointer != "":
		sb.WriteString(" within JSON value at ")
		sb.WriteString(strconv.Quote(e.Pointer))
	case e.Offset > 0:
		sb.WriteString(" after byte offset ")
		sb.WriteString(strconv.FormatInt(e.Offset, 10))
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

// SyntaxError is a description of a JSON syntax error.
//
// The contents of this error as produced by this package may change over time.
type SyntaxError struct {
	// Offset indicates that an error occurred after processing Offset bytes.
	Offset int64
	str    string
}

func (e *SyntaxError) Error() string {
	return errorPrefix + e.str
}
func (e *SyntaxError) Is(target error) bool {
	return e == target || target == Error
}
func (e *SyntaxError) withOffset(pos int64) error {
	return &SyntaxError{Offset: pos, str: e.str}
}

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
