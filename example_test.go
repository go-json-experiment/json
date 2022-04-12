// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json_test

import (
	"fmt"
	"reflect"

	"github.com/go-json-experiment/json"
)

// In some applications, the exact precision of JSON numbers needs to be
// preserved when unmarshaling. This can be accomplished using a type-specific
// marshal function that intercepts all any types and pre-populates the
// interface value with a RawValue, which can represent a JSON number exactly.
func ExampleValue_unmarshalRawNumber() {
	opts := json.UnmarshalOptions{
		Unmarshalers: json.UnmarshalFuncV2(func(opts json.UnmarshalOptions, dec *json.Decoder, val *any) error {
			if dec.PeekKind() == '0' {
				*val = json.RawValue(nil) // provide concrete Go type to unmarshal a JSON number into
			}
			return json.SkipFunc // fall back to default unmarshal behavior
		}),
	}

	in := []byte(`[false, 1e-1000, 3.141592653589793238462643383279, 1e+1000, true]`)
	var val any
	if err := opts.Unmarshal(json.DecodeOptions{}, in, &val); err != nil {
		panic(err)
	}
	fmt.Println(val)

	// Sanity check.
	want := []any{false, json.RawValue("1e-1000"), json.RawValue("3.141592653589793238462643383279"), json.RawValue("1e+1000"), true}
	if !reflect.DeepEqual(val, want) {
		panic(fmt.Sprintf("value mismatch:\ngot  %v\nwant %v", val, want))
	}

	// Output:
	// [false 1e-1000 3.141592653589793238462643383279 1e+1000 true]
}
