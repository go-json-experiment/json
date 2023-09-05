// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import (
	"fmt"
	"io"
	"reflect"

	jsonv2 "github.com/go-json-experiment/json"
	"github.com/go-json-experiment/json/internal"
)

// Inject functionality into v2 to properly handle v1 types.
func init() {
	internal.TransformMarshalError = transformMarshalError
	internal.TransformUnmarshalError = transformUnmarshalError
	internal.NewRawNumber = func() any { return new(Number) }
	internal.RawNumberOf = func(b []byte) any { return Number(b) }
}

func transformMarshalError(err error) error {
	if err, ok := err.(*jsonv2.SemanticError); err != nil {
		switch {
		case false: // for marshal methods only
			return &MarshalerError{
				Type:       err.GoType,
				Err:        err.Err,
				sourceFunc: "TODO", // use err.action?
			}
		case err.JSONKind == 0 && err.GoType != nil && err.Err == nil:
			return &UnsupportedTypeError{Type: err.GoType}
		default:
			return &UnsupportedValueError{
				Value: reflect.Value{}, // TODO
				Str:   "TODO",
			}
		}
	} else if ok {
		return (*UnsupportedValueError)(nil)
	}
	return transformSyntacticError(err)
}

func transformUnmarshalError(err error) error {
	if err, ok := err.(*jsonv2.SemanticError); err != nil {
		// TODO: Use a sentinel error for this.
		if fmt.Sprint(err.Err) == "value must be passed as a non-nil pointer reference" {
			return &InvalidUnmarshalError{err.GoType}
		}
		var valKind string
		switch err.JSONKind {
		case 'n', 'f', 't', '"', '0':
			valKind = err.JSONKind.String()
		case '[', ']':
			valKind = "array"
		case '{', '}':
			valKind = "object"
		}
		return &UnmarshalTypeError{
			Value:  valKind,
			Type:   err.GoType,
			Offset: err.ByteOffset,
			Struct: "", // TODO
			Field:  string(err.JSONPointer),
			Err:    err.Err,
		}
	} else if ok {
		return (*UnmarshalTypeError)(nil)
	}
	if err == io.ErrUnexpectedEOF {
		return &SyntaxError{msg: err.Error()}
	}
	return transformSyntacticError(err)
}
