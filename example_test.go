// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json_test

import (
	"fmt"
	"log"
	"math"
	"net/netip"
	"reflect"
	"time"

	"github.com/go-json-experiment/json"
)

// If a type implements encoding.TextMarshaler and/or encoding.TextUnmarshaler,
// then the MarshalText and UnmarshalText methods are used to encode/decode
// the value to/from a JSON string.
func Example_textMarshal() {
	// Round-trip marshal and unmarshal a hostname map where the netip.Addr type
	// implements both encoding.TextMarshaler and encoding.TextUnmarshaler.
	want := map[netip.Addr]string{
		netip.MustParseAddr("192.168.0.100"): "carbonite",
		netip.MustParseAddr("192.168.0.101"): "obsidian",
		netip.MustParseAddr("192.168.0.102"): "diamond",
	}
	b, err := json.Marshal(&want)
	if err != nil {
		log.Fatal(err)
	}
	var got map[netip.Addr]string
	err = json.Unmarshal(b, &got)
	if err != nil {
		log.Fatal(err)
	}

	// Sanity check.
	if !reflect.DeepEqual(got, want) {
		log.Fatalf("roundtrip mismatch: got %v, want %v", got, want)
	}

	// Print the serialized JSON object. Canonicalize the JSON first since
	// Go map entries are not serialized in a deterministic order.
	(*json.RawValue)(&b).Canonicalize()
	(*json.RawValue)(&b).Indent("", "\t") // indent for readability
	fmt.Println(string(b))

	// Output:
	// {
	// 	"192.168.0.100": "carbonite",
	// 	"192.168.0.101": "obsidian",
	// 	"192.168.0.102": "diamond"
	// }
}

// The "format" tag option can be used to alter the formatting of certain types.
func Example_formatFlags() {
	value := struct {
		BytesBase64    []byte         `json:",format:base64"`
		BytesHex       [8]byte        `json:",format:hex"`
		BytesArray     []byte         `json:",format:array"`
		FloatNonFinite float64        `json:",format:nonfinite"`
		MapEmitNull    map[string]any `json:",format:emitnull"`
		SliceEmitNull  []any          `json:",format:emitnull"`
		TimeDateOnly   time.Time      `json:",format:'2006-01-02'"`
		DurationNanos  time.Duration  `json:",format:nanos"`
	}{
		BytesBase64:    []byte{0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef},
		BytesHex:       [8]byte{0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef},
		BytesArray:     []byte{0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef},
		FloatNonFinite: math.NaN(),
		MapEmitNull:    nil,
		SliceEmitNull:  nil,
		TimeDateOnly:   time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
		DurationNanos:  time.Second + time.Millisecond + time.Microsecond + time.Nanosecond,
	}

	b, err := json.Marshal(&value)
	if err != nil {
		log.Fatal(err)
	}
	(*json.RawValue)(&b).Indent("", "\t") // indent for readability
	fmt.Println(string(b))

	// Output:
	// {
	// 	"BytesBase64": "ASNFZ4mrze8=",
	// 	"BytesHex": "0123456789abcdef",
	// 	"BytesArray": [
	// 		1,
	// 		35,
	// 		69,
	// 		103,
	// 		137,
	// 		171,
	// 		205,
	// 		239
	// 	],
	// 	"FloatNonFinite": "NaN",
	// 	"MapEmitNull": null,
	// 	"SliceEmitNull": null,
	// 	"TimeDateOnly": "2000-01-01",
	// 	"DurationNanos": 1001001001
	// }
}

// In some applications, the exact precision of JSON numbers needs to be
// preserved when unmarshaling. This can be accomplished using a type-specific
// unmarshal function that intercepts all any types and pre-populates the
// interface value with a RawValue, which can represent a JSON number exactly.
func ExampleUnmarshalers_rawNumber() {
	opts := json.UnmarshalOptions{
		// Intercept every attempt to unmarshal into the any type.
		Unmarshalers: json.UnmarshalFuncV2(func(opts json.UnmarshalOptions, dec *json.Decoder, val *any) error {
			// If the next value to be decoded is a JSON number,
			// then provide a concrete Go type to unmarshal into.
			if dec.PeekKind() == '0' {
				*val = json.RawValue(nil)
			}
			// Return SkipFunc to fallback on default unmarshal behavior.
			return json.SkipFunc
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
