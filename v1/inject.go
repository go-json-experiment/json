// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import (
	"bytes"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	jsonv2 "github.com/go-json-experiment/json"
	"github.com/go-json-experiment/json/internal"
	"github.com/go-json-experiment/json/jsontext"
)

// CheckMarshalResults checks the result of Marshal and Encoder.Encode
// against both the v1 and v2 implementations.
// The v1 implementation is not tested if this is nil.
var CheckMarshalResults = func(name string, b1, b2 []byte, err1, err2 error) {
	const checkExactErrors = false
	switch {
	case !bytes.Equal(b1, b2) && err1 == nil && err2 == nil:
		panic(fmt.Sprintf("%s output mismatch:\n\tv1: %s\n\tv2: %s", name, b1, b2))
	case (err1 == nil) != (err2 == nil):
		panic(fmt.Sprintf("%s error mismatch:\n\tv1: %v\n\tv2: %v", name, err1, err2))
	case checkExactErrors && !reflect.DeepEqual(err1, err2):
		panic(fmt.Sprintf("%s error mismatch:\n\tv1: %v\n\tv2: %v", name, err1, err2))
	}
}

// CheckUnmarshalResults checks the result of Unmarshal and Decoder.Decode
// against both the v1 and v2 implementation
// if the output Go value is a pointer to a zero value.
// The v1 implementation is not tested if this is nil.
var CheckUnmarshalResults = func(name string, in []byte, v1, v2 any, err1, err2 error) {
	const checkExactErrors = false
	switch {
	// TODO: We should verify that v1 and v2 are identical even under errors.
	case !reflect.DeepEqual(v1, v2) && err1 == nil && err2 == nil:
		panic(fmt.Sprintf("%s(`%s`, new(%v)) mismatch:\n\tv1: %v\n\tv2: %v", name, in, reflect.TypeOf(v1).Elem(), v1, v2))
	case (err1 == nil) != (err2 == nil):
		panic(fmt.Sprintf("%s error mismatch:\n\tv1: %v\n\tv2: %v", name, err1, err2))
	case checkExactErrors && !reflect.DeepEqual(err1, err2):
		panic(fmt.Sprintf("%s error mismatch:\n\tv1: %v\n\tv2: %v", name, err1, err2))
	}
}

// Inject functionality into v2 to properly handle v1 types.
func init() {
	internal.TransformMarshalError = transformMarshalError
	internal.TransformUnmarshalError = transformUnmarshalError
	internal.NewMarshalerError = func(val any, err error, funcName string) error {
		return &MarshalerError{reflect.TypeOf(val), err, funcName}
	}

	internal.NewRawNumber = func() any { return new(Number) }
	internal.RawNumberOf = func(b []byte) any { return Number(b) }
}

func transformMarshalError(root any, err error) error {
	// Historically, errors returned from Marshal methods were wrapped
	// in a [MarshalerError]. This is directly performed by the v2 package
	// via the injected [internal.NewMarshalerError] constructor
	// while operating under [ReportErrorsWithLegacySemantics].
	// Note that errors from a Marshal method were always wrapped,
	// even if wrapped for multiple layers.
	if err, ok := err.(*jsonv2.SemanticError); err != nil {
		if err.Err == nil {
			// Historically, this was only reported for unserializable types
			// like complex numbers, channels, functions, and unsafe.Pointers.
			return &UnsupportedTypeError{Type: err.GoType}
		} else {
			// Historically, this was only reported for NaN or ±Inf values
			// and cycles detected in the value.
			// The Val used to be populated with the reflect.Value,
			// but this is no longer supported.
			errStr := err.Err.Error()
			if err.Err == internal.ErrCycle && err.GoType != nil {
				errStr += " via " + err.GoType.String()
			}
			errStr = strings.TrimPrefix(errStr, "unsupported value: ")
			return &UnsupportedValueError{Str: errStr}
		}
	} else if ok {
		return (*UnsupportedValueError)(nil)
	}
	if err, _ := err.(*MarshalerError); err != nil {
		err.Err = transformSyntacticError(err.Err)
		return err
	}
	return transformSyntacticError(err)
}

func transformUnmarshalError(root any, err error) error {
	// Historically, errors from Unmarshal methods were never wrapped and
	// returned verbatim while operating under [ReportErrorsWithLegacySemantics].
	if err, ok := err.(*jsonv2.SemanticError); err != nil {
		if err.Err == internal.ErrNonNilReference {
			return &InvalidUnmarshalError{err.GoType}
		}
		if err.Err == jsonv2.ErrUnknownName {
			return fmt.Errorf("json: unknown field %q", err.JSONPointer.LastToken())
		}

		// Historically, UnmarshalTypeError has always been inconsistent
		// about how it reported position information.
		//
		// The Struct field now points to the root type,
		// rather than some intermediate struct in the path.
		// This better matches the original intent of the field based
		// on how the Error message was formatted.
		//
		// For a representation closer to the historical representation,
		// we switch the '/'-delimited representation of a JSON pointer
		// to use a '.'-delimited representation. This may be ambiguous,
		// but the prior representation was always ambiguous as well.
		// Users that care about precise positions should use v2 errors
		// by disabling [ReportErrorsWithLegacySemantics].
		//
		// The introduction of a Err field is new to the v1-to-v2 migration
		// and allows us to preserve stronger error information
		// that may be surfaced by the v2 package.
		//
		// See https://go.dev/issue/43126
		var value string
		switch err.JSONKind {
		case 'n', '"', '0':
			value = err.JSONKind.String()
		case 'f', 't':
			value = "bool"
		case '[', ']':
			value = "array"
		case '{', '}':
			value = "object"
		}
		if len(err.JSONValue) > 0 {
			isStrconvError := err.Err == strconv.ErrRange || err.Err == strconv.ErrSyntax
			isNumericKind := func(t reflect.Type) bool {
				if t == nil {
					return false
				}
				switch t.Kind() {
				case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
					reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
					reflect.Float32, reflect.Float64:
					return true
				}
				return false
			}
			if isStrconvError && isNumericKind(err.GoType) {
				value = "number"
				if err.JSONKind == '"' {
					err.JSONValue, _ = jsontext.AppendUnquote(nil, err.JSONValue)
				}
				err.Err = nil
			}
			value += " " + string(err.JSONValue)
		}
		var rootName string
		if t := reflect.TypeOf(root); t != nil && err.JSONPointer != "" {
			if t.Kind() == reflect.Pointer {
				t = t.Elem()
			}
			rootName = t.Name()
		}
		fieldPath := string(err.JSONPointer)
		fieldPath = strings.TrimPrefix(fieldPath, "/")
		fieldPath = strings.ReplaceAll(fieldPath, "/", ".")
		return &UnmarshalTypeError{
			Value:  value,
			Type:   err.GoType,
			Offset: err.ByteOffset,
			Struct: rootName,
			Field:  fieldPath,
			Err:    transformSyntacticError(err.Err),
		}
	} else if ok {
		return (*UnmarshalTypeError)(nil)
	}
	return transformSyntacticError(err)
}
