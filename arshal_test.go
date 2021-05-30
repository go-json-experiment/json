// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"strconv"
	"testing"
)

type (
	namedBool    bool
	namedString  string
	namedBytes   []byte
	namedInt64   int64
	namedUint64  uint64
	namedFloat64 float64
	namedByte    byte

	recursiveMap   map[string]recursiveMap
	recursiveSlice []recursiveSlice
)

func addr(v interface{}) interface{} {
	v1 := reflect.ValueOf(v)
	v2 := reflect.New(v1.Type())
	v2.Elem().Set(v1)
	return v2.Interface()
}

func TestMarshal(t *testing.T) {
	tests := []struct {
		name    string
		opts    MarshalOptions
		in      interface{}
		want    string
		wantErr error
	}{{
		name: "Nil",
		in:   nil,
		want: `null`,
	}, {
		name: "Bools",
		in:   []bool{false, true},
		want: `[false,true]`,
	}, {
		name: "Bools/Named",
		in:   []namedBool{false, true},
		want: `[false,true]`,
	}, {
		name: "Bools/NotStringified",
		opts: MarshalOptions{StringifyNumbers: true},
		in:   []bool{false, true},
		want: `[false,true]`,
	}, {
		name: "Strings",
		in:   []string{"", "hello", "世界"},
		want: `["","hello","世界"]`,
	}, {
		name: "Strings/Named",
		in:   []namedString{"", "hello", "世界"},
		want: `["","hello","世界"]`,
	}, {
		name: "Bytes",
		in:   [][]byte{nil, {}, {1}, {1, 2}, {1, 2, 3}},
		want: `["","","AQ==","AQI=","AQID"]`,
	}, {
		name: "Bytes/Large",
		in:   []byte("the quick brown fox jumped over the lazy dog and ate the homework that I spent so much time on."),
		want: `"dGhlIHF1aWNrIGJyb3duIGZveCBqdW1wZWQgb3ZlciB0aGUgbGF6eSBkb2cgYW5kIGF0ZSB0aGUgaG9tZXdvcmsgdGhhdCBJIHNwZW50IHNvIG11Y2ggdGltZSBvbi4="`,
	}, {
		name: "Bytes/Named",
		in:   []namedBytes{nil, {}, {1}, {1, 2}, {1, 2, 3}},
		want: `["","","AQ==","AQI=","AQID"]`,
	}, {
		name: "Bytes/NotStringified",
		opts: MarshalOptions{StringifyNumbers: true},
		in:   [][]byte{nil, {}, {1}, {1, 2}, {1, 2, 3}},
		want: `["","","AQ==","AQI=","AQID"]`,
	}, {
		// NOTE: []namedByte is not assignable to []byte,
		// so the following should be treated as a slice of uints.
		name: "Bytes/Invariant",
		in:   [][]namedByte{nil, {}, {1}, {1, 2}, {1, 2, 3}},
		want: `[[],[],[1],[1,2],[1,2,3]]`,
	}, {
		// NOTE: This differs in behavior from v1,
		// but keeps the representation of slices and arrays more consistent.
		name: "Bytes/ByteArray",
		in:   [5]byte{'h', 'e', 'l', 'l', 'o'},
		want: `"aGVsbG8="`,
	}, {
		// NOTE: []namedByte is not assignable to []byte,
		// so the following should be treated as an array of uints.
		name: "Bytes/NamedByteArray",
		in:   [5]namedByte{'h', 'e', 'l', 'l', 'o'},
		want: `[104,101,108,108,111]`,
	}, {
		name: "Ints",
		in: []interface{}{
			int(0), int8(math.MinInt8), int16(math.MinInt16), int32(math.MinInt32), int64(math.MinInt64), namedInt64(-6464),
		},
		want: `[0,-128,-32768,-2147483648,-9223372036854776000,-6464]`,
	}, {
		name: "Ints/Stringified",
		opts: MarshalOptions{StringifyNumbers: true},
		in: []interface{}{
			int(0), int8(math.MinInt8), int16(math.MinInt16), int32(math.MinInt32), int64(math.MinInt64), namedInt64(-6464),
		},
		want: `["0","-128","-32768","-2147483648","-9223372036854775808","-6464"]`,
	}, {
		name: "Uints",
		in: []interface{}{
			uint(0), uint8(math.MaxUint8), uint16(math.MaxUint16), uint32(math.MaxUint32), uint64(math.MaxUint64), namedUint64(6464),
		},
		want: `[0,255,65535,4294967295,18446744073709552000,6464]`,
	}, {
		name: "Uints/Stringified",
		opts: MarshalOptions{StringifyNumbers: true},
		in: []interface{}{
			uint(0), uint8(math.MaxUint8), uint16(math.MaxUint16), uint32(math.MaxUint32), uint64(math.MaxUint64), namedUint64(6464),
		},
		want: `["0","255","65535","4294967295","18446744073709551615","6464"]`,
	}, {
		name: "Floats",
		in: []interface{}{
			float32(math.MaxFloat32), float64(math.MaxFloat64), namedFloat64(64.64),
		},
		want: `[3.4028235e+38,1.7976931348623157e+308,64.64]`,
	}, {
		name: "Floats/Stringified",
		opts: MarshalOptions{StringifyNumbers: true},
		in: []interface{}{
			float32(math.MaxFloat32), float64(math.MaxFloat64), namedFloat64(64.64),
		},
		want: `["3.4028235e+38","1.7976931348623157e+308","64.64"]`,
	}, {
		name:    "Maps/InvalidKey/Bool",
		in:      map[bool]string{false: "value"},
		want:    `{`,
		wantErr: errMissingName,
	}, {
		name:    "Maps/InvalidKey/NamedBool",
		in:      map[namedBool]string{false: "value"},
		want:    `{`,
		wantErr: errMissingName,
	}, {
		name:    "Maps/InvalidKey/Array",
		in:      map[[1]string]string{[1]string{"key"}: "value"},
		want:    `{`,
		wantErr: errMissingName,
	}, {
		name:    "Maps/InvalidKey/Channel",
		in:      map[chan string]string{make(chan string): "value"},
		want:    `{`,
		wantErr: newMarshalError(0, reflect.TypeOf((chan string)(nil)), nil),
	}, {
		name: "Maps/ValidKey/Int",
		in:   map[int64]string{math.MinInt64: "MinInt64", 0: "Zero", math.MaxInt64: "MaxInt64"},
		want: `{"-9223372036854775808":"MinInt64","0":"Zero","9223372036854775807":"MaxInt64"}`,
	}, {
		name: "Maps/ValidKey/NamedInt",
		in:   map[namedInt64]string{math.MinInt64: "MinInt64", 0: "Zero", math.MaxInt64: "MaxInt64"},
		want: `{"-9223372036854775808":"MinInt64","0":"Zero","9223372036854775807":"MaxInt64"}`,
	}, {
		name: "Maps/ValidKey/Uint",
		in:   map[uint64]string{0: "Zero", math.MaxUint64: "MaxUint64"},
		want: `{"0":"Zero","18446744073709551615":"MaxUint64"}`,
	}, {
		name: "Maps/ValidKey/NamedUint",
		in:   map[namedUint64]string{0: "Zero", math.MaxUint64: "MaxUint64"},
		want: `{"0":"Zero","18446744073709551615":"MaxUint64"}`,
	}, {
		// TODO: In v1 json, floating point keys were rejected since the
		// initial release could not canonically serialize floats.
		// Even if it could, there's no guarantee that we receive canonically
		// serialized floats when decoding. Should we allow this?
		// @mvdan feels probably not.
		name: "Maps/ValidKey/Float",
		in:   map[float64]string{3.14159: "value"},
		want: `{"3.14159":"value"}`,
	}, {
		name: "Maps/ValidKey/Interface",
		in: map[interface{}]interface{}{
			"key":               "key",
			namedInt64(-64):     int32(-32),
			namedUint64(+64):    uint32(+32),
			namedFloat64(64.64): float32(32.32),
		},
		want: `{"-64":-32,"64":32,"64.64":32.32,"key":"key"}`,
	}, {
		name: "Maps/InvalidValue/Channel",
		in: map[string]chan string{
			"key": nil,
		},
		want:    `{"key"`,
		wantErr: newMarshalError(0, reflect.TypeOf((chan string)(nil)), nil),
	}, {
		name: "Maps/RecursiveMap",
		in: recursiveMap{
			"fizz": {
				"foo": {},
				"bar": nil,
			},
			"buzz": nil,
		},
		want: `{"buzz":{},"fizz":{"bar":{},"foo":{}}}`,
	}, {
		name: "Slices/Interface",
		in: []interface{}{
			false, true,
			"hello", []byte("world"),
			int32(-32), namedInt64(-64),
			uint32(+32), namedUint64(+64),
			float32(32.32), namedFloat64(64.64),
		},
		want: `[false,true,"hello","d29ybGQ=",-32,-64,32,64,32.32,64.64]`,
	}, {
		name:    "Slices/Invalid/Channel",
		in:      [](chan string){nil},
		want:    `[`,
		wantErr: newMarshalError(0, reflect.TypeOf((chan string)(nil)), nil),
	}, {
		name: "Slices/RecursiveSlice",
		in: recursiveSlice{
			nil,
			{},
			{nil},
			{nil, {}},
		},
		want: `[[],[],[[]],[[],[]]]`,
	}, {
		name: "Arrays/Empty",
		in:   [0]struct{}{},
		want: `[]`,
	}, {
		name: "Arrays/Bool",
		in:   [2]bool{false, true},
		want: `[false,true]`,
	}, {
		name: "Arrays/String",
		in:   [2]string{"hello", "goodbye"},
		want: `["hello","goodbye"]`,
	}, {
		name: "Arrays/Bytes",
		in:   [2][]byte{[]byte("hello"), []byte("goodbye")},
		want: `["aGVsbG8=","Z29vZGJ5ZQ=="]`,
	}, {
		name: "Arrays/Int",
		in:   [2]int64{math.MinInt64, math.MaxInt64},
		want: `[-9223372036854776000,9223372036854776000]`,
	}, {
		name: "Arrays/Uint",
		in:   [2]uint64{0, math.MaxUint64},
		want: `[0,18446744073709552000]`,
	}, {
		name: "Arrays/Float",
		in:   [2]float64{-math.MaxFloat64, +math.MaxFloat64},
		want: `[-1.7976931348623157e+308,1.7976931348623157e+308]`,
	}, {
		name:    "Arrays/Invalid/Channel",
		in:      new([1]chan string),
		want:    `[`,
		wantErr: newMarshalError(0, reflect.TypeOf((chan string)(nil)), nil),
	}, {
		name: "Pointers/NilL0",
		in:   (*int)(nil),
		want: `null`,
	}, {
		name: "Pointers/NilL1",
		in:   (**int)(new(*int)),
		want: `null`,
	}, {
		name: "Pointers/Bool",
		in:   addr(addr(bool(true))),
		want: `true`,
	}, {
		name: "Pointers/String",
		in:   addr(addr(string("string"))),
		want: `"string"`,
	}, {
		name: "Pointers/Bytes",
		in:   addr(addr([]byte("bytes"))),
		want: `"Ynl0ZXM="`,
	}, {
		name: "Pointers/Int",
		in:   addr(addr(int(-100))),
		want: `-100`,
	}, {
		name: "Pointers/Uint",
		in:   addr(addr(uint(100))),
		want: `100`,
	}, {
		name: "Pointers/Float",
		in:   addr(addr(float64(3.14159))),
		want: `3.14159`,
	}, {
		name: "Interfaces/Nil/Empty",
		in:   [1]interface{}{nil},
		want: `[null]`,
	}, {
		name: "Interfaces/Nil/NonEmpty",
		in:   [1]io.Reader{nil},
		want: `[null]`,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotErr := tt.opts.Marshal(EncodeOptions{}, tt.in)
			(*RawValue)(&got).Canonicalize()
			if string(got) != tt.want {
				t.Errorf("Marshal output mismatch:\ngot  %s\nwant %s", got, tt.want)
			}
			if !reflect.DeepEqual(gotErr, tt.wantErr) {
				t.Errorf("Marshal error mismatch:\ngot  %v\nwant %v", gotErr, tt.wantErr)
			}
		})
	}
}

func TestUnmarshal(t *testing.T) {
	tests := []struct {
		name    string
		opts    UnmarshalOptions
		inBuf   string
		inVal   interface{}
		want    interface{}
		wantErr error
	}{{
		name:    "Nil",
		inBuf:   `null`,
		wantErr: &SemanticError{str: "unable to mutate input; must be a non-nil pointer"},
	}, {
		name:  "Bools/Null",
		inBuf: `null`,
		inVal: addr(true),
		want:  addr(false),
	}, {
		name:  "Bools",
		inBuf: `[null,false,true]`,
		inVal: new([]bool),
		want:  addr([]bool{false, false, true}),
	}, {
		name:  "Bools/Named",
		inBuf: `[null,false,true]`,
		inVal: new([]namedBool),
		want:  addr([]namedBool{false, false, true}),
	}, {
		name:    "Bools/Invalid/StringifiedFalse",
		opts:    UnmarshalOptions{StringifyNumbers: true},
		inBuf:   `"false"`,
		inVal:   addr(true),
		want:    addr(true),
		wantErr: newUnmarshalError('"', reflect.TypeOf(bool(false)), nil),
	}, {
		name:    "Bools/Invalid/StringifiedTrue",
		opts:    UnmarshalOptions{StringifyNumbers: true},
		inBuf:   `"true"`,
		inVal:   addr(true),
		want:    addr(true),
		wantErr: newUnmarshalError('"', reflect.TypeOf(bool(false)), nil),
	}, {
		name:    "Bools/Invalid/Number",
		inBuf:   `0`,
		inVal:   addr(true),
		want:    addr(true),
		wantErr: newUnmarshalError('0', reflect.TypeOf(bool(false)), nil),
	}, {
		name:    "Bools/Invalid/String",
		inBuf:   `""`,
		inVal:   addr(true),
		want:    addr(true),
		wantErr: newUnmarshalError('"', reflect.TypeOf(bool(false)), nil),
	}, {
		name:    "Bools/Invalid/Object",
		inBuf:   `{}`,
		inVal:   addr(true),
		want:    addr(true),
		wantErr: newUnmarshalError('{', reflect.TypeOf(bool(false)), nil),
	}, {
		name:    "Bools/Invalid/Array",
		inBuf:   `[]`,
		inVal:   addr(true),
		want:    addr(true),
		wantErr: newUnmarshalError('[', reflect.TypeOf(bool(false)), nil),
	}, {
		name:  "Strings/Null",
		inBuf: `null`,
		inVal: addr("something"),
		want:  addr(""),
	}, {
		name:  "Strings",
		inBuf: `[null,"","hello","世界"]`,
		inVal: new([]string),
		want:  addr([]string{"", "", "hello", "世界"}),
	}, {
		name:  "Strings/Escaped",
		inBuf: `[null,"","\u0068\u0065\u006c\u006c\u006f","\u4e16\u754c"]`,
		inVal: new([]string),
		want:  addr([]string{"", "", "hello", "世界"}),
	}, {
		name:  "Strings/Named",
		inBuf: `[null,"","hello","世界"]`,
		inVal: new([]namedString),
		want:  addr([]namedString{"", "", "hello", "世界"}),
	}, {
		name:    "Strings/Invalid/False",
		inBuf:   `false`,
		inVal:   addr("nochange"),
		want:    addr("nochange"),
		wantErr: newUnmarshalError('f', reflect.TypeOf(string("")), nil),
	}, {
		name:    "Strings/Invalid/True",
		inBuf:   `true`,
		inVal:   addr("nochange"),
		want:    addr("nochange"),
		wantErr: newUnmarshalError('t', reflect.TypeOf(string("")), nil),
	}, {
		name:    "Strings/Invalid/Object",
		inBuf:   `{}`,
		inVal:   addr("nochange"),
		want:    addr("nochange"),
		wantErr: newUnmarshalError('{', reflect.TypeOf(string("")), nil),
	}, {
		name:    "Strings/Invalid/Array",
		inBuf:   `[]`,
		inVal:   addr("nochange"),
		want:    addr("nochange"),
		wantErr: newUnmarshalError('[', reflect.TypeOf(string("")), nil),
	}, {
		name:  "Bytes/Null",
		inBuf: `null`,
		inVal: addr([]byte("something")),
		want:  addr([]byte(nil)),
	}, {
		name:  "Bytes",
		inBuf: `[null,"","AQ==","AQI=","AQID"]`,
		inVal: new([][]byte),
		want:  addr([][]byte{nil, {}, {1}, {1, 2}, {1, 2, 3}}),
	}, {
		name:  "Bytes/Large",
		inBuf: `"dGhlIHF1aWNrIGJyb3duIGZveCBqdW1wZWQgb3ZlciB0aGUgbGF6eSBkb2cgYW5kIGF0ZSB0aGUgaG9tZXdvcmsgdGhhdCBJIHNwZW50IHNvIG11Y2ggdGltZSBvbi4="`,
		inVal: new([]byte),
		want:  addr([]byte("the quick brown fox jumped over the lazy dog and ate the homework that I spent so much time on.")),
	}, {
		name:  "Bytes/Reuse",
		inBuf: `"AQID"`,
		inVal: addr([]byte("changed")),
		want:  addr([]byte{1, 2, 3}),
	}, {
		name:  "Bytes/Escaped",
		inBuf: `[null,"","\u0041\u0051\u003d\u003d","\u0041\u0051\u0049\u003d","\u0041\u0051\u0049\u0044"]`,
		inVal: new([][]byte),
		want:  addr([][]byte{nil, {}, {1}, {1, 2}, {1, 2, 3}}),
	}, {
		name:  "Bytes/Named",
		inBuf: `[null,"","AQ==","AQI=","AQID"]`,
		inVal: new([]namedBytes),
		want:  addr([]namedBytes{nil, {}, {1}, {1, 2}, {1, 2, 3}}),
	}, {
		name:  "Bytes/NotStringified",
		opts:  UnmarshalOptions{StringifyNumbers: true},
		inBuf: `[null,"","AQ==","AQI=","AQID"]`,
		inVal: new([][]byte),
		want:  addr([][]byte{nil, {}, {1}, {1, 2}, {1, 2, 3}}),
	}, {
		// NOTE: []namedByte is not assignable to []byte,
		// so the following should be treated as a slice of uints.
		name:  "Bytes/Invariant",
		inBuf: `[null,[],[1],[1,2],[1,2,3]]`,
		inVal: new([][]namedByte),
		want:  addr([][]namedByte{nil, {}, {1}, {1, 2}, {1, 2, 3}}),
	}, {
		// NOTE: This differs in behavior from v1,
		// but keeps the representation of slices and arrays more consistent.
		name:  "Bytes/ByteArray",
		inBuf: `"aGVsbG8="`,
		inVal: new([5]byte),
		want:  addr([5]byte{'h', 'e', 'l', 'l', 'o'}),
	}, {
		name:  "Bytes/ByteArray0/Valid",
		inBuf: `""`,
		inVal: new([0]byte),
		want:  addr([0]byte{}),
	}, {
		name:  "Bytes/ByteArray0/Invalid",
		inBuf: `"A"`,
		inVal: new([0]byte),
		want:  addr([0]byte{}),
		wantErr: newUnmarshalError('"', reflect.TypeOf([0]byte{}), func() error {
			_, err := base64.StdEncoding.Decode(make([]byte, 0), []byte("A"))
			return err
		}()),
	}, {
		name:    "Bytes/ByteArray0/Overflow",
		inBuf:   `"AA=="`,
		inVal:   new([0]byte),
		want:    addr([0]byte{}),
		wantErr: newUnmarshalError('"', reflect.TypeOf([0]byte{}), errors.New("decoded base64 length of 1 mismatches array length of 0")),
	}, {
		name:  "Bytes/ByteArray1/Valid",
		inBuf: `"AQ=="`,
		inVal: new([1]byte),
		want:  addr([1]byte{1}),
	}, {
		name:  "Bytes/ByteArray1/Invalid",
		inBuf: `"$$=="`,
		inVal: new([1]byte),
		want:  addr([1]byte{}),
		wantErr: newUnmarshalError('"', reflect.TypeOf([1]byte{}), func() error {
			_, err := base64.StdEncoding.Decode(make([]byte, 1), []byte("$$=="))
			return err
		}()),
	}, {
		name:    "Bytes/ByteArray1/Underflow",
		inBuf:   `""`,
		inVal:   new([1]byte),
		want:    addr([1]byte{}),
		wantErr: newUnmarshalError('"', reflect.TypeOf([1]byte{}), errors.New("decoded base64 length of 0 mismatches array length of 1")),
	}, {
		name:    "Bytes/ByteArray1/Overflow",
		inBuf:   `"AQI="`,
		inVal:   new([1]byte),
		want:    addr([1]byte{}),
		wantErr: newUnmarshalError('"', reflect.TypeOf([1]byte{}), errors.New("decoded base64 length of 2 mismatches array length of 1")),
	}, {
		name:  "Bytes/ByteArray2/Valid",
		inBuf: `"AQI="`,
		inVal: new([2]byte),
		want:  addr([2]byte{1, 2}),
	}, {
		name:  "Bytes/ByteArray2/Invalid",
		inBuf: `"$$$="`,
		inVal: new([2]byte),
		want:  addr([2]byte{}),
		wantErr: newUnmarshalError('"', reflect.TypeOf([2]byte{}), func() error {
			_, err := base64.StdEncoding.Decode(make([]byte, 2), []byte("$$$="))
			return err
		}()),
	}, {
		name:    "Bytes/ByteArray2/Underflow",
		inBuf:   `"AQ=="`,
		inVal:   new([2]byte),
		want:    addr([2]byte{}),
		wantErr: newUnmarshalError('"', reflect.TypeOf([2]byte{}), errors.New("decoded base64 length of 1 mismatches array length of 2")),
	}, {
		name:    "Bytes/ByteArray2/Overflow",
		inBuf:   `"AQID"`,
		inVal:   new([2]byte),
		want:    addr([2]byte{}),
		wantErr: newUnmarshalError('"', reflect.TypeOf([2]byte{}), errors.New("decoded base64 length of 3 mismatches array length of 2")),
	}, {
		name:  "Bytes/ByteArray3/Valid",
		inBuf: `"AQID"`,
		inVal: new([3]byte),
		want:  addr([3]byte{1, 2, 3}),
	}, {
		name:  "Bytes/ByteArray3/Invalid",
		inBuf: `"$$$$"`,
		inVal: new([3]byte),
		want:  addr([3]byte{}),
		wantErr: newUnmarshalError('"', reflect.TypeOf([3]byte{}), func() error {
			_, err := base64.StdEncoding.Decode(make([]byte, 3), []byte("$$$$"))
			return err
		}()),
	}, {
		name:    "Bytes/ByteArray3/Underflow",
		inBuf:   `"AQI="`,
		inVal:   new([3]byte),
		want:    addr([3]byte{}),
		wantErr: newUnmarshalError('"', reflect.TypeOf([3]byte{}), errors.New("decoded base64 length of 2 mismatches array length of 3")),
	}, {
		name:    "Bytes/ByteArray3/Overflow",
		inBuf:   `"AQIDAQ=="`,
		inVal:   new([3]byte),
		want:    addr([3]byte{}),
		wantErr: newUnmarshalError('"', reflect.TypeOf([3]byte{}), errors.New("decoded base64 length of 4 mismatches array length of 3")),
	}, {
		name:  "Bytes/ByteArray4/Valid",
		inBuf: `"AQIDBA=="`,
		inVal: new([4]byte),
		want:  addr([4]byte{1, 2, 3, 4}),
	}, {
		name:  "Bytes/ByteArray4/Invalid",
		inBuf: `"$$$$$$=="`,
		inVal: new([4]byte),
		want:  addr([4]byte{}),
		wantErr: newUnmarshalError('"', reflect.TypeOf([4]byte{}), func() error {
			_, err := base64.StdEncoding.Decode(make([]byte, 4), []byte("$$$$$$=="))
			return err
		}()),
	}, {
		name:    "Bytes/ByteArray4/Underflow",
		inBuf:   `"AQID"`,
		inVal:   new([4]byte),
		want:    addr([4]byte{}),
		wantErr: newUnmarshalError('"', reflect.TypeOf([4]byte{}), errors.New("decoded base64 length of 3 mismatches array length of 4")),
	}, {
		name:    "Bytes/ByteArray4/Overflow",
		inBuf:   `"AQIDBAU="`,
		inVal:   new([4]byte),
		want:    addr([4]byte{}),
		wantErr: newUnmarshalError('"', reflect.TypeOf([4]byte{}), errors.New("decoded base64 length of 5 mismatches array length of 4")),
	}, {
		// NOTE: []namedByte is not assignable to []byte,
		// so the following should be treated as a array of uints.
		name:  "Bytes/NamedByteArray",
		inBuf: `[104,101,108,108,111]`,
		inVal: new([5]namedByte),
		want:  addr([5]namedByte{'h', 'e', 'l', 'l', 'o'}),
	}, {
		name:  "Bytes/Valid/Denormalized",
		inBuf: `"AR=="`,
		inVal: new([]byte),
		want:  addr([]byte{1}),
	}, {
		name:  "Bytes/Invalid/Unpadded1",
		inBuf: `"AQ="`,
		inVal: addr([]byte("nochange")),
		want:  addr([]byte("nochange")),
		wantErr: newUnmarshalError('"', reflect.TypeOf([]byte(nil)), func() error {
			_, err := base64.StdEncoding.Decode(make([]byte, 0), []byte("AQ="))
			return err
		}()),
	}, {
		name:  "Bytes/Invalid/Unpadded2",
		inBuf: `"AQ"`,
		inVal: addr([]byte("nochange")),
		want:  addr([]byte("nochange")),
		wantErr: newUnmarshalError('"', reflect.TypeOf([]byte(nil)), func() error {
			_, err := base64.StdEncoding.Decode(make([]byte, 0), []byte("AQ"))
			return err
		}()),
	}, {
		name:  "Bytes/Invalid/Character",
		inBuf: `"@@@@"`,
		inVal: addr([]byte("nochange")),
		want:  addr([]byte("nochange")),
		wantErr: newUnmarshalError('"', reflect.TypeOf([]byte(nil)), func() error {
			_, err := base64.StdEncoding.Decode(make([]byte, 3), []byte("@@@@"))
			return err
		}()),
	}, {
		name:    "Bytes/Invalid/Bool",
		inBuf:   `true`,
		inVal:   addr([]byte("nochange")),
		want:    addr([]byte("nochange")),
		wantErr: newUnmarshalError('t', reflect.TypeOf([]byte(nil)), nil),
	}, {
		name:    "Bytes/Invalid/Number",
		inBuf:   `0`,
		inVal:   addr([]byte("nochange")),
		want:    addr([]byte("nochange")),
		wantErr: newUnmarshalError('0', reflect.TypeOf([]byte(nil)), nil),
	}, {
		name:    "Bytes/Invalid/Object",
		inBuf:   `{}`,
		inVal:   addr([]byte("nochange")),
		want:    addr([]byte("nochange")),
		wantErr: newUnmarshalError('{', reflect.TypeOf([]byte(nil)), nil),
	}, {
		name:    "Bytes/Invalid/Array",
		inBuf:   `[]`,
		inVal:   addr([]byte("nochange")),
		want:    addr([]byte("nochange")),
		wantErr: newUnmarshalError('[', reflect.TypeOf([]byte(nil)), nil),
	}, {
		name:  "Ints/Null",
		inBuf: `null`,
		inVal: addr(int(1)),
		want:  addr(int(0)),
	}, {
		name:  "Ints/Int",
		inBuf: `1`,
		inVal: addr(int(0)),
		want:  addr(int(1)),
	}, {
		name:    "Ints/Int8/MinOverflow",
		inBuf:   `-129`,
		inVal:   addr(int8(-1)),
		want:    addr(int8(-1)),
		wantErr: newUnmarshalError('0', reflect.TypeOf(int8(0)), fmt.Errorf(`cannot parse "-129" as signed integer: %w`, strconv.ErrRange)),
	}, {
		name:  "Ints/Int8/Min",
		inBuf: `-128`,
		inVal: addr(int8(0)),
		want:  addr(int8(-128)),
	}, {
		name:  "Ints/Int8/Max",
		inBuf: `127`,
		inVal: addr(int8(0)),
		want:  addr(int8(127)),
	}, {
		name:    "Ints/Int8/MaxOverflow",
		inBuf:   `128`,
		inVal:   addr(int8(-1)),
		want:    addr(int8(-1)),
		wantErr: newUnmarshalError('0', reflect.TypeOf(int8(0)), fmt.Errorf(`cannot parse "128" as signed integer: %w`, strconv.ErrRange)),
	}, {
		name:    "Ints/Int16/MinOverflow",
		inBuf:   `-32769`,
		inVal:   addr(int16(-1)),
		want:    addr(int16(-1)),
		wantErr: newUnmarshalError('0', reflect.TypeOf(int16(0)), fmt.Errorf(`cannot parse "-32769" as signed integer: %w`, strconv.ErrRange)),
	}, {
		name:  "Ints/Int16/Min",
		inBuf: `-32768`,
		inVal: addr(int16(0)),
		want:  addr(int16(-32768)),
	}, {
		name:  "Ints/Int16/Max",
		inBuf: `32767`,
		inVal: addr(int16(0)),
		want:  addr(int16(32767)),
	}, {
		name:    "Ints/Int16/MaxOverflow",
		inBuf:   `32768`,
		inVal:   addr(int16(-1)),
		want:    addr(int16(-1)),
		wantErr: newUnmarshalError('0', reflect.TypeOf(int16(0)), fmt.Errorf(`cannot parse "32768" as signed integer: %w`, strconv.ErrRange)),
	}, {
		name:    "Ints/Int32/MinOverflow",
		inBuf:   `-2147483649`,
		inVal:   addr(int32(-1)),
		want:    addr(int32(-1)),
		wantErr: newUnmarshalError('0', reflect.TypeOf(int32(0)), fmt.Errorf(`cannot parse "-2147483649" as signed integer: %w`, strconv.ErrRange)),
	}, {
		name:  "Ints/Int32/Min",
		inBuf: `-2147483648`,
		inVal: addr(int32(0)),
		want:  addr(int32(-2147483648)),
	}, {
		name:  "Ints/Int32/Max",
		inBuf: `2147483647`,
		inVal: addr(int32(0)),
		want:  addr(int32(2147483647)),
	}, {
		name:    "Ints/Int32/MaxOverflow",
		inBuf:   `2147483648`,
		inVal:   addr(int32(-1)),
		want:    addr(int32(-1)),
		wantErr: newUnmarshalError('0', reflect.TypeOf(int32(0)), fmt.Errorf(`cannot parse "2147483648" as signed integer: %w`, strconv.ErrRange)),
	}, {
		name:    "Ints/Int64/MinOverflow",
		inBuf:   `-9223372036854775809`,
		inVal:   addr(int64(-1)),
		want:    addr(int64(-1)),
		wantErr: newUnmarshalError('0', reflect.TypeOf(int64(0)), fmt.Errorf(`cannot parse "-9223372036854775809" as signed integer: %w`, strconv.ErrRange)),
	}, {
		name:  "Ints/Int64/Min",
		inBuf: `-9223372036854775808`,
		inVal: addr(int64(0)),
		want:  addr(int64(-9223372036854775808)),
	}, {
		name:  "Ints/Int64/Max",
		inBuf: `9223372036854775807`,
		inVal: addr(int64(0)),
		want:  addr(int64(9223372036854775807)),
	}, {
		name:    "Ints/Int64/MaxOverflow",
		inBuf:   `9223372036854775808`,
		inVal:   addr(int64(-1)),
		want:    addr(int64(-1)),
		wantErr: newUnmarshalError('0', reflect.TypeOf(int64(0)), fmt.Errorf(`cannot parse "9223372036854775808" as signed integer: %w`, strconv.ErrRange)),
	}, {
		name:  "Ints/Named",
		inBuf: `-6464`,
		inVal: addr(namedInt64(0)),
		want:  addr(namedInt64(-6464)),
	}, {
		name:  "Ints/Stringified",
		opts:  UnmarshalOptions{StringifyNumbers: true},
		inBuf: `"-6464"`,
		inVal: new(int),
		want:  addr(int(-6464)),
	}, {
		name:  "Ints/Escaped",
		opts:  UnmarshalOptions{StringifyNumbers: true},
		inBuf: `"\u002d\u0036\u0034\u0036\u0034"`,
		inVal: new(int),
		want:  addr(int(-6464)),
	}, {
		name:  "Ints/Valid/NegativeZero",
		inBuf: `-0`,
		inVal: addr(int(1)),
		want:  addr(int(0)),
	}, {
		name:    "Ints/Invalid/Fraction",
		inBuf:   `1.0`,
		inVal:   addr(int(-1)),
		want:    addr(int(-1)),
		wantErr: newUnmarshalError('0', reflect.TypeOf(int(0)), fmt.Errorf(`cannot parse "1.0" as signed integer: %w`, strconv.ErrSyntax)),
	}, {
		name:    "Ints/Invalid/Exponent",
		inBuf:   `1e0`,
		inVal:   addr(int(-1)),
		want:    addr(int(-1)),
		wantErr: newUnmarshalError('0', reflect.TypeOf(int(0)), fmt.Errorf(`cannot parse "1e0" as signed integer: %w`, strconv.ErrSyntax)),
	}, {
		name:    "Ints/Invalid/StringifiedFraction",
		opts:    UnmarshalOptions{StringifyNumbers: true},
		inBuf:   `"1.0"`,
		inVal:   addr(int(-1)),
		want:    addr(int(-1)),
		wantErr: newUnmarshalError('"', reflect.TypeOf(int(0)), fmt.Errorf(`cannot parse "1.0" as signed integer: %w`, strconv.ErrSyntax)),
	}, {
		name:    "Ints/Invalid/StringifiedExponent",
		opts:    UnmarshalOptions{StringifyNumbers: true},
		inBuf:   `"1e0"`,
		inVal:   addr(int(-1)),
		want:    addr(int(-1)),
		wantErr: newUnmarshalError('"', reflect.TypeOf(int(0)), fmt.Errorf(`cannot parse "1e0" as signed integer: %w`, strconv.ErrSyntax)),
	}, {
		name:    "Ints/Invalid/Overflow",
		inBuf:   `100000000000000000000000000000`,
		inVal:   addr(int(-1)),
		want:    addr(int(-1)),
		wantErr: newUnmarshalError('0', reflect.TypeOf(int(0)), fmt.Errorf(`cannot parse "100000000000000000000000000000" as signed integer: %w`, strconv.ErrRange)),
	}, {
		name:    "Ints/Invalid/OverflowSyntax",
		opts:    UnmarshalOptions{StringifyNumbers: true},
		inBuf:   `"100000000000000000000000000000x"`,
		inVal:   addr(int(-1)),
		want:    addr(int(-1)),
		wantErr: newUnmarshalError('"', reflect.TypeOf(int(0)), fmt.Errorf(`cannot parse "100000000000000000000000000000x" as signed integer: %w`, strconv.ErrSyntax)),
	}, {
		name:    "Ints/Invalid/Whitespace",
		opts:    UnmarshalOptions{StringifyNumbers: true},
		inBuf:   `"0 "`,
		inVal:   addr(int(-1)),
		want:    addr(int(-1)),
		wantErr: newUnmarshalError('"', reflect.TypeOf(int(0)), fmt.Errorf(`cannot parse "0 " as signed integer: %w`, strconv.ErrSyntax)),
	}, {
		name:    "Ints/Invalid/Bool",
		inBuf:   `true`,
		inVal:   addr(int(-1)),
		want:    addr(int(-1)),
		wantErr: newUnmarshalError('t', reflect.TypeOf(int(0)), nil),
	}, {
		name:    "Ints/Invalid/String",
		inBuf:   `"0"`,
		inVal:   addr(int(-1)),
		want:    addr(int(-1)),
		wantErr: newUnmarshalError('"', reflect.TypeOf(int(0)), nil),
	}, {
		name:    "Ints/Invalid/Object",
		inBuf:   `{}`,
		inVal:   addr(int(-1)),
		want:    addr(int(-1)),
		wantErr: newUnmarshalError('{', reflect.TypeOf(int(0)), nil),
	}, {
		name:    "Ints/Invalid/Array",
		inBuf:   `[]`,
		inVal:   addr(int(-1)),
		want:    addr(int(-1)),
		wantErr: newUnmarshalError('[', reflect.TypeOf(int(0)), nil),
	}, {
		name:  "Uints/Null",
		inBuf: `null`,
		inVal: addr(uint(1)),
		want:  addr(uint(0)),
	}, {
		name:  "Uints/Uint",
		inBuf: `1`,
		inVal: addr(uint(0)),
		want:  addr(uint(1)),
	}, {
		name:  "Uints/Uint8/Min",
		inBuf: `0`,
		inVal: addr(uint8(1)),
		want:  addr(uint8(0)),
	}, {
		name:  "Uints/Uint8/Max",
		inBuf: `255`,
		inVal: addr(uint8(0)),
		want:  addr(uint8(255)),
	}, {
		name:    "Uints/Uint8/MaxOverflow",
		inBuf:   `256`,
		inVal:   addr(uint8(1)),
		want:    addr(uint8(1)),
		wantErr: newUnmarshalError('0', reflect.TypeOf(uint8(0)), fmt.Errorf(`cannot parse "256" as unsigned integer: %w`, strconv.ErrRange)),
	}, {
		name:  "Uints/Uint16/Min",
		inBuf: `0`,
		inVal: addr(uint16(1)),
		want:  addr(uint16(0)),
	}, {
		name:  "Uints/Uint16/Max",
		inBuf: `65535`,
		inVal: addr(uint16(0)),
		want:  addr(uint16(65535)),
	}, {
		name:    "Uints/Uint16/MaxOverflow",
		inBuf:   `65536`,
		inVal:   addr(uint16(1)),
		want:    addr(uint16(1)),
		wantErr: newUnmarshalError('0', reflect.TypeOf(uint16(0)), fmt.Errorf(`cannot parse "65536" as unsigned integer: %w`, strconv.ErrRange)),
	}, {
		name:  "Uints/Uint32/Min",
		inBuf: `0`,
		inVal: addr(uint32(1)),
		want:  addr(uint32(0)),
	}, {
		name:  "Uints/Uint32/Max",
		inBuf: `4294967295`,
		inVal: addr(uint32(0)),
		want:  addr(uint32(4294967295)),
	}, {
		name:    "Uints/Uint32/MaxOverflow",
		inBuf:   `4294967296`,
		inVal:   addr(uint32(1)),
		want:    addr(uint32(1)),
		wantErr: newUnmarshalError('0', reflect.TypeOf(uint32(0)), fmt.Errorf(`cannot parse "4294967296" as unsigned integer: %w`, strconv.ErrRange)),
	}, {
		name:  "Uints/Uint64/Min",
		inBuf: `0`,
		inVal: addr(uint64(1)),
		want:  addr(uint64(0)),
	}, {
		name:  "Uints/Uint64/Max",
		inBuf: `18446744073709551615`,
		inVal: addr(uint64(0)),
		want:  addr(uint64(18446744073709551615)),
	}, {
		name:    "Uints/Uint64/MaxOverflow",
		inBuf:   `18446744073709551616`,
		inVal:   addr(uint64(1)),
		want:    addr(uint64(1)),
		wantErr: newUnmarshalError('0', reflect.TypeOf(uint64(0)), fmt.Errorf(`cannot parse "18446744073709551616" as unsigned integer: %w`, strconv.ErrRange)),
	}, {
		name:  "Uints/Named",
		inBuf: `6464`,
		inVal: addr(namedUint64(0)),
		want:  addr(namedUint64(6464)),
	}, {
		name:  "Uints/Stringified",
		opts:  UnmarshalOptions{StringifyNumbers: true},
		inBuf: `"6464"`,
		inVal: new(uint),
		want:  addr(uint(6464)),
	}, {
		name:  "Uints/Escaped",
		opts:  UnmarshalOptions{StringifyNumbers: true},
		inBuf: `"\u0036\u0034\u0036\u0034"`,
		inVal: new(uint),
		want:  addr(uint(6464)),
	}, {
		name:    "Uints/Invalid/NegativeOne",
		inBuf:   `-1`,
		inVal:   addr(uint(1)),
		want:    addr(uint(1)),
		wantErr: newUnmarshalError('0', reflect.TypeOf(uint(0)), fmt.Errorf(`cannot parse "-1" as unsigned integer: %w`, strconv.ErrSyntax)),
	}, {
		name:    "Uints/Invalid/NegativeZero",
		inBuf:   `-0`,
		inVal:   addr(uint(1)),
		want:    addr(uint(1)),
		wantErr: newUnmarshalError('0', reflect.TypeOf(uint(0)), fmt.Errorf(`cannot parse "-0" as unsigned integer: %w`, strconv.ErrSyntax)),
	}, {
		name:    "Uints/Invalid/Fraction",
		inBuf:   `1.0`,
		inVal:   addr(uint(10)),
		want:    addr(uint(10)),
		wantErr: newUnmarshalError('0', reflect.TypeOf(uint(0)), fmt.Errorf(`cannot parse "1.0" as unsigned integer: %w`, strconv.ErrSyntax)),
	}, {
		name:    "Uints/Invalid/Exponent",
		inBuf:   `1e0`,
		inVal:   addr(uint(10)),
		want:    addr(uint(10)),
		wantErr: newUnmarshalError('0', reflect.TypeOf(uint(0)), fmt.Errorf(`cannot parse "1e0" as unsigned integer: %w`, strconv.ErrSyntax)),
	}, {
		name:    "Uints/Invalid/StringifiedFraction",
		opts:    UnmarshalOptions{StringifyNumbers: true},
		inBuf:   `"1.0"`,
		inVal:   addr(uint(10)),
		want:    addr(uint(10)),
		wantErr: newUnmarshalError('"', reflect.TypeOf(uint(0)), fmt.Errorf(`cannot parse "1.0" as unsigned integer: %w`, strconv.ErrSyntax)),
	}, {
		name:    "Uints/Invalid/StringifiedExponent",
		opts:    UnmarshalOptions{StringifyNumbers: true},
		inBuf:   `"1e0"`,
		inVal:   addr(uint(10)),
		want:    addr(uint(10)),
		wantErr: newUnmarshalError('"', reflect.TypeOf(uint(0)), fmt.Errorf(`cannot parse "1e0" as unsigned integer: %w`, strconv.ErrSyntax)),
	}, {
		name:    "Uints/Invalid/Overflow",
		inBuf:   `100000000000000000000000000000`,
		inVal:   addr(uint(1)),
		want:    addr(uint(1)),
		wantErr: newUnmarshalError('0', reflect.TypeOf(uint(0)), fmt.Errorf(`cannot parse "100000000000000000000000000000" as unsigned integer: %w`, strconv.ErrRange)),
	}, {
		name:    "Uints/Invalid/OverflowSyntax",
		opts:    UnmarshalOptions{StringifyNumbers: true},
		inBuf:   `"100000000000000000000000000000x"`,
		inVal:   addr(uint(1)),
		want:    addr(uint(1)),
		wantErr: newUnmarshalError('"', reflect.TypeOf(uint(0)), fmt.Errorf(`cannot parse "100000000000000000000000000000x" as unsigned integer: %w`, strconv.ErrSyntax)),
	}, {
		name:    "Uints/Invalid/Whitespace",
		opts:    UnmarshalOptions{StringifyNumbers: true},
		inBuf:   `"0 "`,
		inVal:   addr(uint(1)),
		want:    addr(uint(1)),
		wantErr: newUnmarshalError('"', reflect.TypeOf(uint(0)), fmt.Errorf(`cannot parse "0 " as unsigned integer: %w`, strconv.ErrSyntax)),
	}, {
		name:    "Uints/Invalid/Bool",
		inBuf:   `true`,
		inVal:   addr(uint(1)),
		want:    addr(uint(1)),
		wantErr: newUnmarshalError('t', reflect.TypeOf(uint(0)), nil),
	}, {
		name:    "Uints/Invalid/String",
		inBuf:   `"0"`,
		inVal:   addr(uint(1)),
		want:    addr(uint(1)),
		wantErr: newUnmarshalError('"', reflect.TypeOf(uint(0)), nil),
	}, {
		name:    "Uints/Invalid/Object",
		inBuf:   `{}`,
		inVal:   addr(uint(1)),
		want:    addr(uint(1)),
		wantErr: newUnmarshalError('{', reflect.TypeOf(uint(0)), nil),
	}, {
		name:    "Uints/Invalid/Array",
		inBuf:   `[]`,
		inVal:   addr(uint(1)),
		want:    addr(uint(1)),
		wantErr: newUnmarshalError('[', reflect.TypeOf(uint(0)), nil),
	}, {
		name:  "Floats/Null",
		inBuf: `null`,
		inVal: addr(float64(64.64)),
		want:  addr(float64(0)),
	}, {
		name:  "Floats/Float32/Pi",
		inBuf: `3.14159265358979323846264338327950288419716939937510582097494459`,
		inVal: addr(float32(32.32)),
		want:  addr(float32(math.Pi)),
	}, {
		name:  "Floats/Float32/Underflow",
		inBuf: `-1e1000`,
		inVal: addr(float32(32.32)),
		want:  addr(float32(-math.MaxFloat32)),
	}, {
		name:  "Floats/Float32/Overflow",
		inBuf: `-1e1000`,
		inVal: addr(float32(32.32)),
		want:  addr(float32(-math.MaxFloat32)),
	}, {
		name:  "Floats/Float64/Pi",
		inBuf: `3.14159265358979323846264338327950288419716939937510582097494459`,
		inVal: addr(float64(64.64)),
		want:  addr(float64(math.Pi)),
	}, {
		name:  "Floats/Float64/Underflow",
		inBuf: `-1e1000`,
		inVal: addr(float64(64.64)),
		want:  addr(float64(-math.MaxFloat64)),
	}, {
		name:  "Floats/Float64/Overflow",
		inBuf: `-1e1000`,
		inVal: addr(float64(64.64)),
		want:  addr(float64(-math.MaxFloat64)),
	}, {
		name:  "Floats/Named",
		inBuf: `64.64`,
		inVal: addr(namedFloat64(0)),
		want:  addr(namedFloat64(64.64)),
	}, {
		name:  "Floats/Stringified",
		opts:  UnmarshalOptions{StringifyNumbers: true},
		inBuf: `"64.64"`,
		inVal: new(float64),
		want:  addr(float64(64.64)),
	}, {
		name:  "Floats/Escaped",
		opts:  UnmarshalOptions{StringifyNumbers: true},
		inBuf: `"\u0036\u0034\u002e\u0036\u0034"`,
		inVal: new(float64),
		want:  addr(float64(64.64)),
	}, {
		name:    "Floats/Invalid/NaN",
		opts:    UnmarshalOptions{StringifyNumbers: true},
		inBuf:   `"NaN"`,
		inVal:   addr(float64(64.64)),
		want:    addr(float64(64.64)),
		wantErr: newUnmarshalError('"', reflect.TypeOf(float64(0)), fmt.Errorf(`cannot parse "NaN" as JSON number: %w`, strconv.ErrSyntax)),
	}, {
		name:    "Floats/Invalid/Infinity",
		opts:    UnmarshalOptions{StringifyNumbers: true},
		inBuf:   `"Infinity"`,
		inVal:   addr(float64(64.64)),
		want:    addr(float64(64.64)),
		wantErr: newUnmarshalError('"', reflect.TypeOf(float64(0)), fmt.Errorf(`cannot parse "Infinity" as JSON number: %w`, strconv.ErrSyntax)),
	}, {
		name:    "Floats/Invalid/Whitespace",
		opts:    UnmarshalOptions{StringifyNumbers: true},
		inBuf:   `"1 "`,
		inVal:   addr(float64(64.64)),
		want:    addr(float64(64.64)),
		wantErr: newUnmarshalError('"', reflect.TypeOf(float64(0)), fmt.Errorf(`cannot parse "1 " as JSON number: %w`, strconv.ErrSyntax)),
	}, {
		name:    "Floats/Invalid/GoSyntax",
		opts:    UnmarshalOptions{StringifyNumbers: true},
		inBuf:   `"1p-2"`,
		inVal:   addr(float64(64.64)),
		want:    addr(float64(64.64)),
		wantErr: newUnmarshalError('"', reflect.TypeOf(float64(0)), fmt.Errorf(`cannot parse "1p-2" as JSON number: %w`, strconv.ErrSyntax)),
	}, {
		name:    "Floats/Invalid/Bool",
		inBuf:   `true`,
		inVal:   addr(float64(64.64)),
		want:    addr(float64(64.64)),
		wantErr: newUnmarshalError('t', reflect.TypeOf(float64(0)), nil),
	}, {
		name:    "Floats/Invalid/String",
		inBuf:   `"0"`,
		inVal:   addr(float64(64.64)),
		want:    addr(float64(64.64)),
		wantErr: newUnmarshalError('"', reflect.TypeOf(float64(0)), nil),
	}, {
		name:    "Floats/Invalid/Object",
		inBuf:   `{}`,
		inVal:   addr(float64(64.64)),
		want:    addr(float64(64.64)),
		wantErr: newUnmarshalError('{', reflect.TypeOf(float64(0)), nil),
	}, {
		name:    "Floats/Invalid/Array",
		inBuf:   `[]`,
		inVal:   addr(float64(64.64)),
		want:    addr(float64(64.64)),
		wantErr: newUnmarshalError('[', reflect.TypeOf(float64(0)), nil),
	}, {
		name:  "Maps/Null",
		inBuf: `null`,
		inVal: addr(map[string]string{"key": "value"}),
		want:  new(map[string]string),
	}, {
		name:    "Maps/InvalidKey/Bool",
		inBuf:   `{"true":"false"}`,
		inVal:   new(map[bool]bool),
		want:    addr(make(map[bool]bool)),
		wantErr: newUnmarshalError('"', reflect.TypeOf(bool(false)), nil),
	}, {
		name:    "Maps/InvalidKey/NamedBool",
		inBuf:   `{"true":"false"}`,
		inVal:   new(map[namedBool]bool),
		want:    addr(make(map[namedBool]bool)),
		wantErr: newUnmarshalError('"', reflect.TypeOf(namedBool(false)), nil),
	}, {
		name:    "Maps/InvalidKey/Array",
		inBuf:   `{"key":"value"}`,
		inVal:   new(map[[1]string]string),
		want:    addr(make(map[[1]string]string)),
		wantErr: newUnmarshalError('"', reflect.TypeOf([1]string{}), nil),
	}, {
		name:    "Maps/InvalidKey/Channel",
		inBuf:   `{"key":"value"}`,
		inVal:   new(map[chan string]string),
		want:    addr(make(map[chan string]string)),
		wantErr: newUnmarshalError(0, reflect.TypeOf((chan string)(nil)), nil),
	}, {
		name:  "Maps/ValidKey/Int",
		inBuf: `{"0":0,"-1":1,"2":2,"-3":3}`,
		inVal: new(map[int]int),
		want:  addr(map[int]int{0: 0, -1: 1, 2: 2, -3: 3}),
	}, {
		// NOTE: For signed integers, the only possible way for duplicate keys
		// with different representations is negative zero and zero.
		name:  "Maps/ValidKey/Int/Duplicates",
		inBuf: `{"0":1,"-0":-1}`,
		inVal: new(map[int]int),
		want:  addr(map[int]int{0: -1}), // latter takes precedence
	}, {
		name:  "Maps/ValidKey/NamedInt",
		inBuf: `{"0":0,"-1":1,"2":2,"-3":3}`,
		inVal: new(map[namedInt64]int),
		want:  addr(map[namedInt64]int{0: 0, -1: 1, 2: 2, -3: 3}),
	}, {
		name:  "Maps/ValidKey/Uint",
		inBuf: `{"0":0,"1":1,"2":2,"3":3}`,
		inVal: new(map[uint]uint),
		want:  addr(map[uint]uint{0: 0, 1: 1, 2: 2, 3: 3}),
	}, {
		name:  "Maps/ValidKey/NamedUint",
		inBuf: `{"0":0,"1":1,"2":2,"3":3}`,
		inVal: new(map[namedUint64]uint),
		want:  addr(map[namedUint64]uint{0: 0, 1: 1, 2: 2, 3: 3}),
	}, {
		// TODO: Should we forbid floating point keys?
		name:  "Maps/ValidKey/Float",
		inBuf: `{"1.234":1.234,"12.34":12.34,"123.4":123.4}`,
		inVal: new(map[float64]float64),
		want:  addr(map[float64]float64{1.234: 1.234, 12.34: 12.34, 123.4: 123.4}),
	}, {
		name:  "Maps/ValidKey/Float/Duplicates",
		inBuf: `{"1.0":"1.0","1":"1","1e0":"1e0"}`,
		inVal: new(map[float64]string),
		want:  addr(map[float64]string{1: "1e0"}), // latter takes precedence
	}, {
		name:  "Maps/ValidKey/Interface",
		inBuf: `{"false":"false","true":"true","string":"string","0":"0","[]":"[]","{}":"{}"}`,
		inVal: new(map[interface{}]string),
		want: addr(map[interface{}]string{
			"false":  "false",
			"true":   "true",
			"string": "string",
			"0":      "0",
			"[]":     "[]",
			"{}":     "{}",
		}),
	}, {
		name:  "Maps/InvalidValue/Channel",
		inBuf: `{"key":"value"}`,
		inVal: new(map[string]chan string),
		want: addr(map[string]chan string{
			"key": nil,
		}),
		wantErr: newUnmarshalError(0, reflect.TypeOf((chan string)(nil)), nil),
	}, {
		name:  "Maps/RecursiveMap",
		inBuf: `{"buzz":{},"fizz":{"bar":{},"foo":{}}}`,
		inVal: new(recursiveMap),
		want: addr(recursiveMap{
			"fizz": {
				"foo": {},
				"bar": {},
			},
			"buzz": {},
		}),
	}, {
		// NOTE: The semantics differs from v1,
		// where existing map entries were not merged into.
		// See https://golang.org/issue/31924.
		name:  "Maps/Merge",
		inBuf: `{"k1":{"k2":"v2"},"k2":{"k1":"v1"},"k2":{"k2":"v2"}}`,
		inVal: addr(map[string]map[string]string{
			"k1": {"k1": "v1"},
		}),
		want: addr(map[string]map[string]string{
			"k1": {"k1": "v1", "k2": "v2"},
			"k2": {"k1": "v1", "k2": "v2"},
		}),
	}, {
		name:    "Maps/Invalid/Bool",
		inBuf:   `true`,
		inVal:   addr(map[string]string{"key": "value"}),
		want:    addr(map[string]string{"key": "value"}),
		wantErr: newUnmarshalError('t', reflect.TypeOf(map[string]string(nil)), nil),
	}, {
		name:    "Maps/Invalid/String",
		inBuf:   `""`,
		inVal:   addr(map[string]string{"key": "value"}),
		want:    addr(map[string]string{"key": "value"}),
		wantErr: newUnmarshalError('"', reflect.TypeOf(map[string]string(nil)), nil),
	}, {
		name:    "Maps/Invalid/Number",
		inBuf:   `0`,
		inVal:   addr(map[string]string{"key": "value"}),
		want:    addr(map[string]string{"key": "value"}),
		wantErr: newUnmarshalError('0', reflect.TypeOf(map[string]string(nil)), nil),
	}, {
		name:    "Maps/Invalid/Array",
		inBuf:   `[]`,
		inVal:   addr(map[string]string{"key": "value"}),
		want:    addr(map[string]string{"key": "value"}),
		wantErr: newUnmarshalError('[', reflect.TypeOf(map[string]string(nil)), nil),
	}, {
		name:  "Slices/Null",
		inBuf: `null`,
		inVal: new([]string),
		want:  addr([]string(nil)),
	}, {
		name:  "Slices/Bool",
		inBuf: `[true,false]`,
		inVal: new([]bool),
		want:  addr([]bool{true, false}),
	}, {
		name:  "Slices/String",
		inBuf: `["hello","goodbye"]`,
		inVal: new([]string),
		want:  addr([]string{"hello", "goodbye"}),
	}, {
		name:  "Slices/Bytes",
		inBuf: `["aGVsbG8=","Z29vZGJ5ZQ=="]`,
		inVal: new([][]byte),
		want:  addr([][]byte{[]byte("hello"), []byte("goodbye")}),
	}, {
		name:  "Slices/Int",
		inBuf: `[-2,-1,0,1,2]`,
		inVal: new([]int),
		want:  addr([]int{-2, -1, 0, 1, 2}),
	}, {
		name:  "Slices/Uint",
		inBuf: `[0,1,2,3,4]`,
		inVal: new([]uint),
		want:  addr([]uint{0, 1, 2, 3, 4}),
	}, {
		name:  "Slices/Float",
		inBuf: `[3.14159,12.34]`,
		inVal: new([]float64),
		want:  addr([]float64{3.14159, 12.34}),
	}, {
		// NOTE: The semantics differs from v1, where the slice length is reset
		// and new elements are appended to the end.
		// See https://golang.org/issue/21092.
		name:  "Slices/Merge",
		inBuf: `[{"k3":"v3"},{"k4":"v4"}]`,
		inVal: addr([]map[string]string{{"k1": "v1"}, {"k2": "v2"}}[:1]),
		want:  addr([]map[string]string{{"k3": "v3"}, {"k4": "v4"}}),
	}, {
		name:    "Slices/Invalid/Channel",
		inBuf:   `["hello"]`,
		inVal:   new([]chan string),
		want:    addr([]chan string{nil}),
		wantErr: newUnmarshalError(0, reflect.TypeOf((chan string)(nil)), nil),
	}, {
		name:  "Slices/RecursiveSlice",
		inBuf: `[[],[],[[]],[[],[]]]`,
		inVal: new(recursiveSlice),
		want: addr(recursiveSlice{
			{},
			{},
			{{}},
			{{}, {}},
		}),
	}, {
		name:    "Slices/Invalid/Bool",
		inBuf:   `true`,
		inVal:   addr([]string{"nochange"}),
		want:    addr([]string{"nochange"}),
		wantErr: newUnmarshalError('t', reflect.TypeOf([]string(nil)), nil),
	}, {
		name:    "Slices/Invalid/String",
		inBuf:   `""`,
		inVal:   addr([]string{"nochange"}),
		want:    addr([]string{"nochange"}),
		wantErr: newUnmarshalError('"', reflect.TypeOf([]string(nil)), nil),
	}, {
		name:    "Slices/Invalid/Number",
		inBuf:   `0`,
		inVal:   addr([]string{"nochange"}),
		want:    addr([]string{"nochange"}),
		wantErr: newUnmarshalError('0', reflect.TypeOf([]string(nil)), nil),
	}, {
		name:    "Slices/Invalid/Object",
		inBuf:   `{}`,
		inVal:   addr([]string{"nochange"}),
		want:    addr([]string{"nochange"}),
		wantErr: newUnmarshalError('{', reflect.TypeOf([]string(nil)), nil),
	}, {
		name:  "Arrays/Null",
		inBuf: `null`,
		inVal: new([1]string),
		want:  addr([1]string{}),
	}, {
		name:  "Arrays/Bool",
		inBuf: `[true,false]`,
		inVal: new([2]bool),
		want:  addr([2]bool{true, false}),
	}, {
		name:  "Arrays/String",
		inBuf: `["hello","goodbye"]`,
		inVal: new([2]string),
		want:  addr([2]string{"hello", "goodbye"}),
	}, {
		name:  "Arrays/Bytes",
		inBuf: `["aGVsbG8=","Z29vZGJ5ZQ=="]`,
		inVal: new([2][]byte),
		want:  addr([2][]byte{[]byte("hello"), []byte("goodbye")}),
	}, {
		name:  "Arrays/Int",
		inBuf: `[-2,-1,0,1,2]`,
		inVal: new([5]int),
		want:  addr([5]int{-2, -1, 0, 1, 2}),
	}, {
		name:  "Arrays/Uint",
		inBuf: `[0,1,2,3,4]`,
		inVal: new([5]uint),
		want:  addr([5]uint{0, 1, 2, 3, 4}),
	}, {
		name:  "Arrays/Float",
		inBuf: `[3.14159,12.34]`,
		inVal: new([2]float64),
		want:  addr([2]float64{3.14159, 12.34}),
	}, {
		// NOTE: The semantics differs from v1, where elements are not merged.
		// This is to maintain consistent merge semantics with slices.
		name:  "Arrays/Merge",
		inBuf: `[{"k3":"v3"},{"k4":"v4"}]`,
		inVal: addr([2]map[string]string{{"k1": "v1"}, {"k2": "v2"}}),
		want:  addr([2]map[string]string{{"k3": "v3"}, {"k4": "v4"}}),
	}, {
		name:    "Arrays/Invalid/Channel",
		inBuf:   `["hello"]`,
		inVal:   new([1]chan string),
		want:    new([1]chan string),
		wantErr: newUnmarshalError(0, reflect.TypeOf((chan string)(nil)), nil),
	}, {
		name:    "Arrays/Invalid/Underflow",
		inBuf:   `[]`,
		inVal:   new([1]string),
		want:    addr([1]string{}),
		wantErr: newUnmarshalError(0, reflect.TypeOf([1]string{}), errors.New("too few array elements")),
	}, {
		name:    "Arrays/Invalid/Overflow",
		inBuf:   `["1","2"]`,
		inVal:   new([1]string),
		want:    addr([1]string{"1"}),
		wantErr: newUnmarshalError(0, reflect.TypeOf([1]string{}), errors.New("too many array elements")),
	}, {
		name:    "Arrays/Invalid/Bool",
		inBuf:   `true`,
		inVal:   addr([1]string{"nochange"}),
		want:    addr([1]string{"nochange"}),
		wantErr: newUnmarshalError('t', reflect.TypeOf([1]string{}), nil),
	}, {
		name:    "Arrays/Invalid/String",
		inBuf:   `""`,
		inVal:   addr([1]string{"nochange"}),
		want:    addr([1]string{"nochange"}),
		wantErr: newUnmarshalError('"', reflect.TypeOf([1]string{}), nil),
	}, {
		name:    "Arrays/Invalid/Number",
		inBuf:   `0`,
		inVal:   addr([1]string{"nochange"}),
		want:    addr([1]string{"nochange"}),
		wantErr: newUnmarshalError('0', reflect.TypeOf([1]string{}), nil),
	}, {
		name:    "Arrays/Invalid/Object",
		inBuf:   `{}`,
		inVal:   addr([1]string{"nochange"}),
		want:    addr([1]string{"nochange"}),
		wantErr: newUnmarshalError('{', reflect.TypeOf([1]string{}), nil),
	}, {
		name:  "Pointers/NullL0",
		inBuf: `null`,
		inVal: new(*string),
		want:  addr((*string)(nil)),
	}, {
		name:  "Pointers/NullL1",
		inBuf: `null`,
		inVal: addr((**string)(new(*string))),
		want:  addr((**string)(nil)),
	}, {
		name:  "Pointers/Bool",
		inBuf: `true`,
		inVal: addr(new(bool)),
		want:  addr(addr(true)),
	}, {
		name:  "Pointers/String",
		inBuf: `"hello"`,
		inVal: addr(new(string)),
		want:  addr(addr("hello")),
	}, {
		name:  "Pointers/Bytes",
		inBuf: `"aGVsbG8="`,
		inVal: addr(new([]byte)),
		want:  addr(addr([]byte("hello"))),
	}, {
		name:  "Pointers/Int",
		inBuf: `-123`,
		inVal: addr(new(int)),
		want:  addr(addr(int(-123))),
	}, {
		name:  "Pointers/Uint",
		inBuf: `123`,
		inVal: addr(new(int)),
		want:  addr(addr(int(123))),
	}, {
		name:  "Pointers/Float",
		inBuf: `123.456`,
		inVal: addr(new(float64)),
		want:  addr(addr(float64(123.456))),
	}, {
		name:  "Pointers/Allocate",
		inBuf: `"hello"`,
		inVal: addr((*string)(nil)),
		want:  addr(addr("hello")),
	}, {
		name:  "Interfaces/Empty/Null",
		inBuf: `null`,
		inVal: new(interface{}),
		want:  new(interface{}),
	}, {
		name:  "Interfaces/NonEmpty/Null",
		inBuf: `null`,
		inVal: new(io.Reader),
		want:  new(io.Reader),
	}, {
		name:    "Interfaces/NonEmpty/Invalid",
		inBuf:   `"hello"`,
		inVal:   new(io.Reader),
		want:    new(io.Reader),
		wantErr: newUnmarshalError('"', reflect.TypeOf((*io.Reader)(nil)).Elem(), errors.New("cannot derive concrete type for non-empty interface")),
	}, {
		name:  "Interfaces/Empty/False",
		inBuf: `false`,
		inVal: new(interface{}),
		want: func() interface{} {
			var vi interface{} = false
			return &vi
		}(),
	}, {
		name:  "Interfaces/Empty/True",
		inBuf: `true`,
		inVal: new(interface{}),
		want: func() interface{} {
			var vi interface{} = true
			return &vi
		}(),
	}, {
		name:  "Interfaces/Empty/String",
		inBuf: `"string"`,
		inVal: new(interface{}),
		want: func() interface{} {
			var vi interface{} = "string"
			return &vi
		}(),
	}, {
		name:  "Interfaces/Empty/Number",
		inBuf: `3.14159`,
		inVal: new(interface{}),
		want: func() interface{} {
			var vi interface{} = 3.14159
			return &vi
		}(),
	}, {
		name:  "Interfaces/Empty/Object",
		inBuf: `{"k":"v"}`,
		inVal: new(interface{}),
		want: func() interface{} {
			var vi interface{} = map[string]interface{}{"k": "v"}
			return &vi
		}(),
	}, {
		name:  "Interfaces/Empty/Array",
		inBuf: `["v"]`,
		inVal: new(interface{}),
		want: func() interface{} {
			var vi interface{} = []interface{}{"v"}
			return &vi
		}(),
	}, {
		// NOTE: The semantics differs from v1,
		// where existing map entries were not merged into.
		// See https://golang.org/issue/26946.
		// See https://golang.org/issue/33993.
		name:  "Interfaces/Merge",
		inBuf: `{"k2":"v2"}`,
		inVal: func() interface{} {
			var vi interface{} = map[string]string{"k1": "v1"}
			return &vi
		}(),
		want: func() interface{} {
			var vi interface{} = map[string]string{"k1": "v1", "k2": "v2"}
			return &vi
		}(),
		// TODO: Add merge test for non-pointer named primitive.
		// TODO: Add merge test for non-pointer struct value.
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.inVal
			gotErr := tt.opts.Unmarshal(DecodeOptions{}, []byte(tt.inBuf), got)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Unmarshal output mismatch:\ngot  %v\nwant %v", got, tt.want)
			}
			if !reflect.DeepEqual(gotErr, tt.wantErr) {
				t.Errorf("Unmarshal error mismatch:\ngot  %v\nwant %v", gotErr, tt.wantErr)
			}
		})
	}
}

func TestUnmarshalReuse(t *testing.T) {
	t.Run("Bytes", func(t *testing.T) {
		in := make([]byte, 3)
		want := &in[0]
		if err := Unmarshal([]byte(`"AQID"`), &in); err != nil {
			t.Fatalf("Unmarshal error: %v", err)
		}
		got := &in[0]
		if got != want {
			t.Errorf("input buffer was not reused")
		}
	})
	t.Run("Slices", func(t *testing.T) {
		in := make([]int, 3)
		want := &in[0]
		if err := Unmarshal([]byte(`[0,1,2]`), &in); err != nil {
			t.Fatalf("Unmarshal error: %v", err)
		}
		got := &in[0]
		if got != want {
			t.Errorf("input slice was not reused")
		}
	})
	t.Run("Maps", func(t *testing.T) {
		in := make(map[string]string)
		want := reflect.ValueOf(in).Pointer()
		if err := Unmarshal([]byte(`{"key":"value"}`), &in); err != nil {
			t.Fatalf("Unmarshal error: %v", err)
		}
		got := reflect.ValueOf(in).Pointer()
		if got != want {
			t.Errorf("input map was not reused")
		}
	})
	t.Run("Pointers", func(t *testing.T) {
		in := addr(addr(addr("hello"))).(***string)
		want := **in
		if err := Unmarshal([]byte(`"goodbye"`), &in); err != nil {
			t.Fatalf("Unmarshal error: %v", err)
		}
		got := **in
		if got != want {
			t.Errorf("input pointer was not reused")
		}
	})
}
