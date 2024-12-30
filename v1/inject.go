// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import (
	"fmt"
	"reflect"
	"strings"

	jsonv2 "github.com/go-json-experiment/json"
	"github.com/go-json-experiment/json/internal"
)

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
	// while operating under [ReportLegacyErrorValues].
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
	return transformSyntacticError(err)
}

func transformUnmarshalError(root any, err error) error {
	// Historically, errors from Unmarshal methods were never wrapped and
	// returned verbatim while operating under [ReportLegacyErrorValues].
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
		// by disabling [ReportLegacyErrorValues].
		//
		// The introduction of a Err field is new to the v1-to-v2 migration
		// and allows us to preserve stronger error information
		// that may be surfaced by the v2 package.
		//
		// See https://go.dev/issue/43126
		var value string
		switch err.JSONKind {
		case 'n', 'f', 't', '"', '0':
			value = err.JSONKind.String()
		case '[', ']':
			value = "array"
		case '{', '}':
			value = "object"
		}
		if len(err.JSONValue) > 0 {
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
			Err:    err.Err,
		}
	} else if ok {
		return (*UnmarshalTypeError)(nil)
	}
	return transformSyntacticError(err)
}
