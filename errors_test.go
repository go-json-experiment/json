// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import (
	"archive/tar"
	"bytes"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
)

func TestSemanticError(t *testing.T) {
	tests := []struct {
		err  error
		want string
	}{{
		err:  &SemanticError{},
		want: "json: cannot handle",
	}, {
		err:  &SemanticError{JSONKind: 'n'},
		want: "json: cannot handle JSON null",
	}, {
		err:  &SemanticError{action: "unmarshal", JSONKind: 't'},
		want: "json: cannot unmarshal JSON boolean",
	}, {
		err:  &SemanticError{action: "unmarshal", JSONKind: 'x'},
		want: "json: cannot unmarshal", // invalid token kinds are ignored
	}, {
		err:  &SemanticError{action: "marshal", JSONKind: '"'},
		want: "json: cannot marshal JSON string",
	}, {
		err:  &SemanticError{GoType: reflect.TypeOf(bool(false))},
		want: "json: cannot handle Go value of type bool",
	}, {
		err:  &SemanticError{action: "marshal", GoType: reflect.TypeOf(int(0))},
		want: "json: cannot marshal Go value of type int",
	}, {
		err:  &SemanticError{action: "unmarshal", GoType: reflect.TypeOf(uint(0))},
		want: "json: cannot unmarshal Go value of type uint",
	}, {
		err:  &SemanticError{JSONKind: '0', GoType: reflect.TypeOf(tar.Header{})},
		want: "json: cannot handle JSON number with Go value of type tar.Header",
	}, {
		err:  &SemanticError{action: "marshal", JSONKind: '{', GoType: reflect.TypeOf(bytes.Buffer{})},
		want: "json: cannot marshal JSON object from Go value of type bytes.Buffer",
	}, {
		err:  &SemanticError{action: "unmarshal", JSONKind: ']', GoType: reflect.TypeOf(strings.Reader{})},
		want: "json: cannot unmarshal JSON array into Go value of type strings.Reader",
	}, {
		err:  &SemanticError{action: "unmarshal", JSONKind: '{', GoType: reflect.TypeOf(float64(0)), ByteOffset: 123},
		want: "json: cannot unmarshal JSON object into Go value of type float64 after byte offset 123",
	}, {
		err:  &SemanticError{action: "marshal", JSONKind: 'f', GoType: reflect.TypeOf(complex128(0)), ByteOffset: 123, JSONPointer: "/foo/2/bar/3"},
		want: "json: cannot marshal JSON boolean from Go value of type complex128 within JSON value at \"/foo/2/bar/3\"",
	}, {
		err:  &SemanticError{action: "unmarshal", JSONKind: '}', GoType: reflect.TypeOf((*io.Reader)(nil)).Elem(), ByteOffset: 123, JSONPointer: "/foo/2/bar/3", Err: errors.New("some underlying error")},
		want: "json: cannot unmarshal JSON object into Go value of type io.Reader within JSON value at \"/foo/2/bar/3\": some underlying error",
	}, {
		err:  &SemanticError{Err: errors.New("some underlying error")},
		want: "json: cannot handle: some underlying error",
	}, {
		err:  &SemanticError{ByteOffset: 123},
		want: "json: cannot handle after byte offset 123",
	}, {
		err:  &SemanticError{JSONPointer: "/foo/2/bar/3"},
		want: "json: cannot handle within JSON value at \"/foo/2/bar/3\"",
	}}

	for _, tt := range tests {
		got := tt.err.Error()
		// Cleanup the error of non-deterministic rendering effects.
		if strings.HasPrefix(got, errorPrefix+"unable to ") {
			got = errorPrefix + "cannot " + strings.TrimPrefix(got, errorPrefix+"unable to ")
		}
		if got != tt.want {
			t.Errorf("%#v.Error mismatch:\ngot  %v\nwant %v", tt.err, got, tt.want)
		}
	}
}
