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
	"net"
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

	structEmpty       struct{}
	structConflicting struct {
		A string `json:"conflict"`
		B string `json:"conflict"`
	}
	structNoneExported struct {
		unexported string
	}
	structUnexportedIgnored struct {
		ignored string `json:"-"`
	}
	structMalformedTag struct {
		Malformed string `json:"\""`
	}
	structUnexportedTag struct {
		unexported string `json:"name"`
	}
	structUnexportedEmbedded struct {
		namedString
	}
	structIgnoredUnexportedEmbedded struct {
		namedString `json:"-"`
	}
	structWeirdNames struct {
		Empty string `json:"''"`
		Comma string `json:"','"`
		Quote string `json:"'\"'"`
	}
	structNoCase struct {
		AaA string `json:",nocase"`
		AAa string `json:",nocase"`
		AAA string
	}
	structScalars struct {
		unexported bool
		Ignored    bool `json:"-"`

		Bool   bool
		String string
		Bytes  []byte
		Int    int64
		Uint   uint64
		Float  float64
	}
	structSlices struct {
		unexported bool
		Ignored    bool `json:"-"`

		SliceBool   []bool
		SliceString []string
		SliceBytes  [][]byte
		SliceInt    []int64
		SliceUint   []uint64
		SliceFloat  []float64
	}
	structMaps struct {
		unexported bool
		Ignored    bool `json:"-"`

		MapBool   map[string]bool
		MapString map[string]string
		MapBytes  map[string][]byte
		MapInt    map[string]int64
		MapUint   map[string]uint64
		MapFloat  map[string]float64
	}
	structAll struct {
		Bool          bool
		String        string
		Bytes         []byte
		Int           int64
		Uint          uint64
		Float         float64
		Map           map[string]string
		StructScalars structScalars
		StructMaps    structMaps
		StructSlices  structSlices
		Slice         []string
		Array         [1]string
		Ptr           *structAll
		Interface     interface{}
	}
	structStringifiedAll struct {
		Bool          bool                  `json:",string"`
		String        string                `json:",string"`
		Bytes         []byte                `json:",string"`
		Int           int64                 `json:",string"`
		Uint          uint64                `json:",string"`
		Float         float64               `json:",string"`
		Map           map[string]string     `json:",string"`
		StructScalars structScalars         `json:",string"`
		StructMaps    structMaps            `json:",string"`
		StructSlices  structSlices          `json:",string"`
		Slice         []string              `json:",string"`
		Array         [1]string             `json:",string"`
		Ptr           *structStringifiedAll `json:",string"`
		Interface     interface{}           `json:",string"`
	}
	structOmitZeroAll struct {
		Bool          bool               `json:",omitzero"`
		String        string             `json:",omitzero"`
		Bytes         []byte             `json:",omitzero"`
		Int           int64              `json:",omitzero"`
		Uint          uint64             `json:",omitzero"`
		Float         float64            `json:",omitzero"`
		Map           map[string]string  `json:",omitzero"`
		StructScalars structScalars      `json:",omitzero"`
		StructMaps    structMaps         `json:",omitzero"`
		StructSlices  structSlices       `json:",omitzero"`
		Slice         []string           `json:",omitzero"`
		Array         [1]string          `json:",omitzero"`
		Ptr           *structOmitZeroAll `json:",omitzero"`
		Interface     interface{}        `json:",omitzero"`
	}

	allMethods struct {
		method string // the method that was called
		value  []byte // the raw value to provide or store
	}
	allMethodsExceptJSONv2 struct {
		allMethods
		MarshalNextJSON   struct{} // cancel out MarshalNextJSON method with collision
		UnmarshalNextJSON struct{} // cancel out UnmarshalNextJSON method with collision
	}
	allMethodsExceptJSONv1 struct {
		allMethods
		MarshalJSON   struct{} // cancel out MarshalJSON method with collision
		UnmarshalJSON struct{} // cancel out UnmarshalJSON method with collision
	}
	allMethodsExceptText struct {
		allMethods
		MarshalText   struct{} // cancel out MarshalText method with collision
		UnmarshalText struct{} // cancel out UnmarshalText method with collision
	}
	onlyMethodJSONv2 struct {
		allMethods
		MarshalJSON   struct{} // cancel out MarshalJSON method with collision
		UnmarshalJSON struct{} // cancel out UnmarshalJSON method with collision
		MarshalText   struct{} // cancel out MarshalText method with collision
		UnmarshalText struct{} // cancel out UnmarshalText method with collision
	}
	onlyMethodJSONv1 struct {
		allMethods
		MarshalNextJSON   struct{} // cancel out MarshalNextJSON method with collision
		UnmarshalNextJSON struct{} // cancel out UnmarshalNextJSON method with collision
		MarshalText       struct{} // cancel out MarshalText method with collision
		UnmarshalText     struct{} // cancel out UnmarshalText method with collision
	}
	onlyMethodText struct {
		allMethods
		MarshalNextJSON   struct{} // cancel out MarshalNextJSON method with collision
		UnmarshalNextJSON struct{} // cancel out UnmarshalNextJSON method with collision
		MarshalJSON       struct{} // cancel out MarshalJSON method with collision
		UnmarshalJSON     struct{} // cancel out UnmarshalJSON method with collision
	}
	structMethodJSONv2  struct{ value string }
	structMethodJSONv1  struct{ value string }
	structMethodText    struct{ value string }
	marshalJSONv2Func   func(*Encoder, MarshalOptions) error
	marshalJSONv1Func   func() ([]byte, error)
	marshalTextFunc     func() ([]byte, error)
	unmarshalJSONv2Func func(*Decoder, UnmarshalOptions) error
	unmarshalJSONv1Func func([]byte) error
	unmarshalTextFunc   func([]byte) error
)

func (p *allMethods) MarshalNextJSON(enc *Encoder, mo MarshalOptions) error {
	if got, want := "MarshalNextJSON", p.method; got != want {
		return fmt.Errorf("called wrong method: got %v, want %v", got, want)
	}
	return enc.WriteValue(p.value)
}
func (p *allMethods) MarshalJSON() ([]byte, error) {
	if got, want := "MarshalJSON", p.method; got != want {
		return nil, fmt.Errorf("called wrong method: got %v, want %v", got, want)
	}
	return p.value, nil
}
func (p *allMethods) MarshalText() ([]byte, error) {
	if got, want := "MarshalText", p.method; got != want {
		return nil, fmt.Errorf("called wrong method: got %v, want %v", got, want)
	}
	return p.value, nil
}

func (p *allMethods) UnmarshalNextJSON(dec *Decoder, uo UnmarshalOptions) error {
	p.method = "UnmarshalNextJSON"
	val, err := dec.ReadValue()
	p.value = val
	return err
}
func (p *allMethods) UnmarshalJSON(val []byte) error {
	p.method = "UnmarshalJSON"
	p.value = val
	return nil
}
func (p *allMethods) UnmarshalText(val []byte) error {
	p.method = "UnmarshalText"
	p.value = val
	return nil
}

func (s structMethodJSONv2) MarshalNextJSON(enc *Encoder, mo MarshalOptions) error {
	return enc.WriteToken(String(s.value))
}
func (s *structMethodJSONv2) UnmarshalNextJSON(dec *Decoder, uo UnmarshalOptions) error {
	tok, err := dec.ReadToken()
	if err != nil {
		return err
	}
	if k := tok.Kind(); k != '"' {
		return &SemanticError{action: "unmarshal", JSONKind: k, GoType: structMethodJSONv2Type}
	}
	s.value = tok.String()
	return nil
}

func (s structMethodJSONv1) MarshalJSON() ([]byte, error) {
	return appendString(nil, s.value, false, nil)
}
func (s *structMethodJSONv1) UnmarshalJSON(b []byte) error {
	if k := RawValue(b).Kind(); k != '"' {
		return &SemanticError{action: "unmarshal", JSONKind: k, GoType: structMethodJSONv1Type}
	}
	b, _ = unescapeString(nil, b)
	s.value = string(b)
	return nil
}

func (s structMethodText) MarshalText() ([]byte, error) {
	return []byte(s.value), nil
}
func (s *structMethodText) UnmarshalText(b []byte) error {
	s.value = string(b)
	return nil
}

func (f marshalJSONv2Func) MarshalNextJSON(enc *Encoder, mo MarshalOptions) error {
	return f(enc, mo)
}
func (f marshalJSONv1Func) MarshalJSON() ([]byte, error) {
	return f()
}
func (f marshalTextFunc) MarshalText() ([]byte, error) {
	return f()
}
func (f unmarshalJSONv2Func) UnmarshalNextJSON(dec *Decoder, uo UnmarshalOptions) error {
	return f(dec, uo)
}
func (f unmarshalJSONv1Func) UnmarshalJSON(b []byte) error {
	return f(b)
}
func (f unmarshalTextFunc) UnmarshalText(b []byte) error {
	return f(b)
}

var (
	namedBoolType                = reflect.TypeOf((*namedBool)(nil)).Elem()
	intType                      = reflect.TypeOf((*int)(nil)).Elem()
	int8Type                     = reflect.TypeOf((*int8)(nil)).Elem()
	int16Type                    = reflect.TypeOf((*int16)(nil)).Elem()
	int32Type                    = reflect.TypeOf((*int32)(nil)).Elem()
	int64Type                    = reflect.TypeOf((*int64)(nil)).Elem()
	uintType                     = reflect.TypeOf((*uint)(nil)).Elem()
	uint8Type                    = reflect.TypeOf((*uint8)(nil)).Elem()
	uint16Type                   = reflect.TypeOf((*uint16)(nil)).Elem()
	uint32Type                   = reflect.TypeOf((*uint32)(nil)).Elem()
	uint64Type                   = reflect.TypeOf((*uint64)(nil)).Elem()
	sliceStringType              = reflect.TypeOf((*[]string)(nil)).Elem()
	array1StringType             = reflect.TypeOf((*[1]string)(nil)).Elem()
	array0ByteType               = reflect.TypeOf((*[0]byte)(nil)).Elem()
	array1ByteType               = reflect.TypeOf((*[1]byte)(nil)).Elem()
	array2ByteType               = reflect.TypeOf((*[2]byte)(nil)).Elem()
	array3ByteType               = reflect.TypeOf((*[3]byte)(nil)).Elem()
	array4ByteType               = reflect.TypeOf((*[4]byte)(nil)).Elem()
	mapStringStringType          = reflect.TypeOf((*map[string]string)(nil)).Elem()
	structConflictingType        = reflect.TypeOf((*structConflicting)(nil)).Elem()
	structNoneExportedType       = reflect.TypeOf((*structNoneExported)(nil)).Elem()
	structMalformedTagType       = reflect.TypeOf((*structMalformedTag)(nil)).Elem()
	structUnexportedTagType      = reflect.TypeOf((*structUnexportedTag)(nil)).Elem()
	structUnexportedEmbeddedType = reflect.TypeOf((*structUnexportedEmbedded)(nil)).Elem()
	allMethodsType               = reflect.TypeOf((*allMethods)(nil)).Elem()
	allMethodsExceptJSONv2Type   = reflect.TypeOf((*allMethodsExceptJSONv2)(nil)).Elem()
	allMethodsExceptJSONv1Type   = reflect.TypeOf((*allMethodsExceptJSONv1)(nil)).Elem()
	allMethodsExceptTextType     = reflect.TypeOf((*allMethodsExceptText)(nil)).Elem()
	onlyMethodJSONv2Type         = reflect.TypeOf((*onlyMethodJSONv2)(nil)).Elem()
	onlyMethodJSONv1Type         = reflect.TypeOf((*onlyMethodJSONv1)(nil)).Elem()
	onlyMethodTextType           = reflect.TypeOf((*onlyMethodText)(nil)).Elem()
	structMethodJSONv2Type       = reflect.TypeOf((*structMethodJSONv2)(nil)).Elem()
	structMethodJSONv1Type       = reflect.TypeOf((*structMethodJSONv1)(nil)).Elem()
	structMethodTextType         = reflect.TypeOf((*structMethodText)(nil)).Elem()
	marshalJSONv2FuncType        = reflect.TypeOf((*marshalJSONv2Func)(nil)).Elem()
	marshalJSONv1FuncType        = reflect.TypeOf((*marshalJSONv1Func)(nil)).Elem()
	marshalTextFuncType          = reflect.TypeOf((*marshalTextFunc)(nil)).Elem()
	unmarshalJSONv2FuncType      = reflect.TypeOf((*unmarshalJSONv2Func)(nil)).Elem()
	unmarshalJSONv1FuncType      = reflect.TypeOf((*unmarshalJSONv1Func)(nil)).Elem()
	unmarshalTextFuncType        = reflect.TypeOf((*unmarshalTextFunc)(nil)).Elem()
	ioReaderType                 = reflect.TypeOf((*io.Reader)(nil)).Elem()
	chanStringType               = reflect.TypeOf((*chan string)(nil)).Elem()
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
		mopts   MarshalOptions
		eopts   EncodeOptions
		in      interface{}
		want    string
		wantErr error

		canonicalize bool // canonicalize the output before comparing?
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
		name:  "Bools/NotStringified",
		mopts: MarshalOptions{StringifyNumbers: true},
		in:    []bool{false, true},
		want:  `[false,true]`,
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
		name:  "Bytes/NotStringified",
		mopts: MarshalOptions{StringifyNumbers: true},
		in:    [][]byte{nil, {}, {1}, {1, 2}, {1, 2, 3}},
		want:  `["","","AQ==","AQI=","AQID"]`,
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
		want: `[0,-128,-32768,-2147483648,-9223372036854775808,-6464]`,
	}, {
		name:  "Ints/Stringified",
		mopts: MarshalOptions{StringifyNumbers: true},
		in: []interface{}{
			int(0), int8(math.MinInt8), int16(math.MinInt16), int32(math.MinInt32), int64(math.MinInt64), namedInt64(-6464),
		},
		want: `["0","-128","-32768","-2147483648","-9223372036854775808","-6464"]`,
	}, {
		name: "Uints",
		in: []interface{}{
			uint(0), uint8(math.MaxUint8), uint16(math.MaxUint16), uint32(math.MaxUint32), uint64(math.MaxUint64), namedUint64(6464),
		},
		want: `[0,255,65535,4294967295,18446744073709551615,6464]`,
	}, {
		name:  "Uints/Stringified",
		mopts: MarshalOptions{StringifyNumbers: true},
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
		name:  "Floats/Stringified",
		mopts: MarshalOptions{StringifyNumbers: true},
		in: []interface{}{
			float32(math.MaxFloat32), float64(math.MaxFloat64), namedFloat64(64.64),
		},
		want: `["3.4028235e+38","1.7976931348623157e+308","64.64"]`,
	}, {
		name:    "Floats/Invalid/NaN",
		mopts:   MarshalOptions{StringifyNumbers: true},
		in:      math.NaN(),
		wantErr: &SemanticError{action: "marshal", GoType: float64Type, Err: fmt.Errorf("invalid value: %v", math.NaN())},
	}, {
		name:    "Floats/Invalid/PositiveInfinity",
		mopts:   MarshalOptions{StringifyNumbers: true},
		in:      math.Inf(+1),
		wantErr: &SemanticError{action: "marshal", GoType: float64Type, Err: fmt.Errorf("invalid value: %v", math.Inf(+1))},
	}, {
		name:    "Floats/Invalid/NegativeInfinity",
		mopts:   MarshalOptions{StringifyNumbers: true},
		in:      math.Inf(-1),
		wantErr: &SemanticError{action: "marshal", GoType: float64Type, Err: fmt.Errorf("invalid value: %v", math.Inf(-1))},
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
		wantErr: &SemanticError{action: "marshal", GoType: chanStringType},
	}, {
		name:         "Maps/ValidKey/Int",
		in:           map[int64]string{math.MinInt64: "MinInt64", 0: "Zero", math.MaxInt64: "MaxInt64"},
		canonicalize: true,
		want:         `{"-9223372036854775808":"MinInt64","0":"Zero","9223372036854775807":"MaxInt64"}`,
	}, {
		name:         "Maps/ValidKey/NamedInt",
		in:           map[namedInt64]string{math.MinInt64: "MinInt64", 0: "Zero", math.MaxInt64: "MaxInt64"},
		canonicalize: true,
		want:         `{"-9223372036854775808":"MinInt64","0":"Zero","9223372036854775807":"MaxInt64"}`,
	}, {
		name:         "Maps/ValidKey/Uint",
		in:           map[uint64]string{0: "Zero", math.MaxUint64: "MaxUint64"},
		canonicalize: true,
		want:         `{"0":"Zero","18446744073709551615":"MaxUint64"}`,
	}, {
		name:         "Maps/ValidKey/NamedUint",
		in:           map[namedUint64]string{0: "Zero", math.MaxUint64: "MaxUint64"},
		canonicalize: true,
		want:         `{"0":"Zero","18446744073709551615":"MaxUint64"}`,
	}, {
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
		canonicalize: true,
		want:         `{"-64":-32,"64":32,"64.64":32.32,"key":"key"}`,
	}, {
		name: "Maps/InvalidValue/Channel",
		in: map[string]chan string{
			"key": nil,
		},
		want:    `{"key"`,
		wantErr: &SemanticError{action: "marshal", GoType: chanStringType},
	}, {
		name: "Maps/RecursiveMap",
		in: recursiveMap{
			"fizz": {
				"foo": {},
				"bar": nil,
			},
			"buzz": nil,
		},
		canonicalize: true,
		want:         `{"buzz":{},"fizz":{"bar":{},"foo":{}}}`,
	}, {
		name: "Structs/Empty",
		in:   structEmpty{},
		want: `{}`,
	}, {
		name: "Structs/UnexportedIgnored",
		in:   structUnexportedIgnored{ignored: "ignored"},
		want: `{}`,
	}, {
		name: "Structs/IgnoredUnexportedEmbedded",
		in:   structIgnoredUnexportedEmbedded{namedString: "ignored"},
		want: `{}`,
	}, {
		name: "Structs/WeirdNames",
		in:   structWeirdNames{Empty: "empty", Comma: "comma", Quote: "quote"},
		want: `{"":"empty",",":"comma","\"":"quote"}`,
	}, {
		name: "Structs/NoCase",
		in:   structNoCase{AaA: "AaA", AAa: "AAa", AAA: "AAA"},
		want: `{"AaA":"AaA","AAa":"AAa","AAA":"AAA"}`,
	}, {
		name:  "Structs/Normal",
		eopts: EncodeOptions{Indent: "\t"},
		in: structAll{
			Bool:   true,
			String: "hello",
			Bytes:  []byte{1, 2, 3},
			Int:    -64,
			Uint:   +64,
			Float:  3.14159,
			Map:    map[string]string{"key": "value"},
			StructScalars: structScalars{
				Bool:   true,
				String: "hello",
				Bytes:  []byte{1, 2, 3},
				Int:    -64,
				Uint:   +64,
				Float:  3.14159,
			},
			StructMaps: structMaps{
				MapBool:   map[string]bool{"": true},
				MapString: map[string]string{"": "hello"},
				MapBytes:  map[string][]byte{"": []byte{1, 2, 3}},
				MapInt:    map[string]int64{"": -64},
				MapUint:   map[string]uint64{"": +64},
				MapFloat:  map[string]float64{"": 3.14159},
			},
			StructSlices: structSlices{
				SliceBool:   []bool{true},
				SliceString: []string{"hello"},
				SliceBytes:  [][]byte{[]byte{1, 2, 3}},
				SliceInt:    []int64{-64},
				SliceUint:   []uint64{+64},
				SliceFloat:  []float64{3.14159},
			},
			Slice:     []string{"fizz", "buzz"},
			Array:     [1]string{"goodbye"},
			Ptr:       new(structAll),
			Interface: (*structAll)(nil),
		},
		want: `{
	"Bool": true,
	"String": "hello",
	"Bytes": "AQID",
	"Int": -64,
	"Uint": 64,
	"Float": 3.14159,
	"Map": {
		"key": "value"
	},
	"StructScalars": {
		"Bool": true,
		"String": "hello",
		"Bytes": "AQID",
		"Int": -64,
		"Uint": 64,
		"Float": 3.14159
	},
	"StructMaps": {
		"MapBool": {
			"": true
		},
		"MapString": {
			"": "hello"
		},
		"MapBytes": {
			"": "AQID"
		},
		"MapInt": {
			"": -64
		},
		"MapUint": {
			"": 64
		},
		"MapFloat": {
			"": 3.14159
		}
	},
	"StructSlices": {
		"SliceBool": [
			true
		],
		"SliceString": [
			"hello"
		],
		"SliceBytes": [
			"AQID"
		],
		"SliceInt": [
			-64
		],
		"SliceUint": [
			64
		],
		"SliceFloat": [
			3.14159
		]
	},
	"Slice": [
		"fizz",
		"buzz"
	],
	"Array": [
		"goodbye"
	],
	"Ptr": {
		"Bool": false,
		"String": "",
		"Bytes": "",
		"Int": 0,
		"Uint": 0,
		"Float": 0,
		"Map": {},
		"StructScalars": {
			"Bool": false,
			"String": "",
			"Bytes": "",
			"Int": 0,
			"Uint": 0,
			"Float": 0
		},
		"StructMaps": {
			"MapBool": {},
			"MapString": {},
			"MapBytes": {},
			"MapInt": {},
			"MapUint": {},
			"MapFloat": {}
		},
		"StructSlices": {
			"SliceBool": [],
			"SliceString": [],
			"SliceBytes": [],
			"SliceInt": [],
			"SliceUint": [],
			"SliceFloat": []
		},
		"Slice": [],
		"Array": [
			""
		],
		"Ptr": null,
		"Interface": null
	},
	"Interface": null
}`,
	}, {
		name:  "Structs/Stringified",
		eopts: EncodeOptions{Indent: "\t"},
		in: structStringifiedAll{
			Bool:   true,
			String: "hello",
			Bytes:  []byte{1, 2, 3},
			Int:    -64,     // should be stringified
			Uint:   +64,     // should be stringified
			Float:  3.14159, // should be stringified
			Map:    map[string]string{"key": "value"},
			StructScalars: structScalars{
				Bool:   true,
				String: "hello",
				Bytes:  []byte{1, 2, 3},
				Int:    -64,     // should be stringified
				Uint:   +64,     // should be stringified
				Float:  3.14159, // should be stringified
			},
			StructMaps: structMaps{
				MapBool:   map[string]bool{"": true},
				MapString: map[string]string{"": "hello"},
				MapBytes:  map[string][]byte{"": []byte{1, 2, 3}},
				MapInt:    map[string]int64{"": -64},       // should be stringified
				MapUint:   map[string]uint64{"": +64},      // should be stringified
				MapFloat:  map[string]float64{"": 3.14159}, // should be stringified
			},
			StructSlices: structSlices{
				SliceBool:   []bool{true},
				SliceString: []string{"hello"},
				SliceBytes:  [][]byte{[]byte{1, 2, 3}},
				SliceInt:    []int64{-64},       // should be stringified
				SliceUint:   []uint64{+64},      // should be stringified
				SliceFloat:  []float64{3.14159}, // should be stringified
			},
			Slice:     []string{"fizz", "buzz"},
			Array:     [1]string{"goodbye"},
			Ptr:       new(structStringifiedAll), // should be stringified
			Interface: (*structStringifiedAll)(nil),
		},
		want: `{
	"Bool": true,
	"String": "hello",
	"Bytes": "AQID",
	"Int": "-64",
	"Uint": "64",
	"Float": "3.14159",
	"Map": {
		"key": "value"
	},
	"StructScalars": {
		"Bool": true,
		"String": "hello",
		"Bytes": "AQID",
		"Int": "-64",
		"Uint": "64",
		"Float": "3.14159"
	},
	"StructMaps": {
		"MapBool": {
			"": true
		},
		"MapString": {
			"": "hello"
		},
		"MapBytes": {
			"": "AQID"
		},
		"MapInt": {
			"": "-64"
		},
		"MapUint": {
			"": "64"
		},
		"MapFloat": {
			"": "3.14159"
		}
	},
	"StructSlices": {
		"SliceBool": [
			true
		],
		"SliceString": [
			"hello"
		],
		"SliceBytes": [
			"AQID"
		],
		"SliceInt": [
			"-64"
		],
		"SliceUint": [
			"64"
		],
		"SliceFloat": [
			"3.14159"
		]
	},
	"Slice": [
		"fizz",
		"buzz"
	],
	"Array": [
		"goodbye"
	],
	"Ptr": {
		"Bool": false,
		"String": "",
		"Bytes": "",
		"Int": "0",
		"Uint": "0",
		"Float": "0",
		"Map": {},
		"StructScalars": {
			"Bool": false,
			"String": "",
			"Bytes": "",
			"Int": "0",
			"Uint": "0",
			"Float": "0"
		},
		"StructMaps": {
			"MapBool": {},
			"MapString": {},
			"MapBytes": {},
			"MapInt": {},
			"MapUint": {},
			"MapFloat": {}
		},
		"StructSlices": {
			"SliceBool": [],
			"SliceString": [],
			"SliceBytes": [],
			"SliceInt": [],
			"SliceUint": [],
			"SliceFloat": []
		},
		"Slice": [],
		"Array": [
			""
		],
		"Ptr": null,
		"Interface": null
	},
	"Interface": null
}`,
	}, {
		name: "Structs/OmitZero/Zero",
		in:   structOmitZeroAll{},
		want: `{}`,
	}, {
		name:  "Structs/OmitZero/NonZero",
		eopts: EncodeOptions{Indent: "\t"},
		in: structOmitZeroAll{
			Bool:          true,                                   // not omitted since true is non-zero
			String:        " ",                                    // not omitted since non-empty string is non-zero
			Bytes:         []byte{},                               // not omitted since allocated slice is non-zero
			Int:           1,                                      // not omitted since 1 is non-zero
			Uint:          1,                                      // not omitted since 1 is non-zero
			Float:         math.Copysign(0, -1),                   // not omitted since -0 is technically non-zero
			Map:           map[string]string{},                    // not omitted since allocated map is non-zero
			StructScalars: structScalars{unexported: true},        // not omitted since unexported is non-zero
			StructSlices:  structSlices{Ignored: true},            // not omitted since Ignored is non-zero
			StructMaps:    structMaps{MapBool: map[string]bool{}}, // not omitted since MapBool is non-zero
			Slice:         []string{},                             // not omitted since allocated slice is non-zero
			Array:         [1]string{" "},                         // not omitted since single array element is non-zero
			Ptr:           new(structOmitZeroAll),                 // not omitted since pointer is non-zero (even if all fields of the struct value are zero)
			Interface:     (*structOmitZeroAll)(nil),              // not omitted since interface value is non-zero (even if interface value is a nil pointer)
		},
		want: `{
	"Bool": true,
	"String": " ",
	"Bytes": "",
	"Int": 1,
	"Uint": 1,
	"Float": -0,
	"Map": {},
	"StructScalars": {
		"Bool": false,
		"String": "",
		"Bytes": "",
		"Int": 0,
		"Uint": 0,
		"Float": 0
	},
	"StructMaps": {
		"MapBool": {},
		"MapString": {},
		"MapBytes": {},
		"MapInt": {},
		"MapUint": {},
		"MapFloat": {}
	},
	"StructSlices": {
		"SliceBool": [],
		"SliceString": [],
		"SliceBytes": [],
		"SliceInt": [],
		"SliceUint": [],
		"SliceFloat": []
	},
	"Slice": [],
	"Array": [
		" "
	],
	"Ptr": {},
	"Interface": null
}`,
	}, {
		name:    "Structs/Invalid/Conflicting",
		in:      structConflicting{},
		want:    `{`,
		wantErr: &SemanticError{action: "marshal", GoType: structConflictingType, Err: errors.New("Go struct fields A and B conflict over JSON object name \"conflict\"")},
	}, {
		name:    "Structs/Invalid/NoneExported",
		in:      structNoneExported{},
		want:    `{`,
		wantErr: &SemanticError{action: "marshal", GoType: structNoneExportedType, Err: errors.New("Go struct kind has no exported fields")},
	}, {
		name:    "Structs/Invalid/MalformedTag",
		in:      structMalformedTag{},
		want:    `{`,
		wantErr: &SemanticError{action: "marshal", GoType: structMalformedTagType, Err: errors.New("Go struct field Malformed has malformed `json` tag: invalid character '\"' at start of option (expecting Unicode letter or single quote)")},
	}, {
		name:    "Structs/Invalid/UnexportedTag",
		in:      structUnexportedTag{},
		want:    `{`,
		wantErr: &SemanticError{action: "marshal", GoType: structUnexportedTagType, Err: errors.New("unexported Go struct field unexported cannot have non-ignored `json:\"name\"` tag")},
	}, {
		name:    "Structs/Invalid/UnexportedEmbedded",
		in:      structUnexportedEmbedded{},
		want:    `{`,
		wantErr: &SemanticError{action: "marshal", GoType: structUnexportedEmbeddedType, Err: errors.New("embedded Go struct field namedString of an unexported type must be explicitly ignored with a `json:\"-\"` tag")},
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
		wantErr: &SemanticError{action: "marshal", GoType: chanStringType},
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
		want: `[-9223372036854775808,9223372036854775807]`,
	}, {
		name: "Arrays/Uint",
		in:   [2]uint64{0, math.MaxUint64},
		want: `[0,18446744073709551615]`,
	}, {
		name: "Arrays/Float",
		in:   [2]float64{-math.MaxFloat64, +math.MaxFloat64},
		want: `[-1.7976931348623157e+308,1.7976931348623157e+308]`,
	}, {
		name:    "Arrays/Invalid/Channel",
		in:      new([1]chan string),
		want:    `[`,
		wantErr: &SemanticError{action: "marshal", GoType: chanStringType},
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
	}, {
		name: "Methods/NilPointer",
		in:   struct{ X *allMethods }{X: (*allMethods)(nil)}, // method should not be called
		want: `{"X":null}`,
	}, {
		// NOTE: Fixes https://github.com/dominikh/go-tools/issues/975.
		name: "Methods/NilInterface",
		in:   struct{ X MarshalerV2 }{X: (*allMethods)(nil)}, // method should not be called
		want: `{"X":null}`,
	}, {
		name: "Methods/AllMethods",
		in:   struct{ X *allMethods }{X: &allMethods{method: "MarshalNextJSON", value: []byte(`"hello"`)}},
		want: `{"X":"hello"}`,
	}, {
		name: "Methods/AllMethodsExceptJSONv2",
		in:   struct{ X *allMethodsExceptJSONv2 }{X: &allMethodsExceptJSONv2{allMethods: allMethods{method: "MarshalJSON", value: []byte(`"hello"`)}}},
		want: `{"X":"hello"}`,
	}, {
		name: "Methods/AllMethodsExceptJSONv1",
		in:   struct{ X *allMethodsExceptJSONv1 }{X: &allMethodsExceptJSONv1{allMethods: allMethods{method: "MarshalNextJSON", value: []byte(`"hello"`)}}},
		want: `{"X":"hello"}`,
	}, {
		name: "Methods/AllMethodsExceptText",
		in:   struct{ X *allMethodsExceptText }{X: &allMethodsExceptText{allMethods: allMethods{method: "MarshalNextJSON", value: []byte(`"hello"`)}}},
		want: `{"X":"hello"}`,
	}, {
		name: "Methods/OnlyMethodJSONv2",
		in:   struct{ X *onlyMethodJSONv2 }{X: &onlyMethodJSONv2{allMethods: allMethods{method: "MarshalNextJSON", value: []byte(`"hello"`)}}},
		want: `{"X":"hello"}`,
	}, {
		name: "Methods/OnlyMethodJSONv1",
		in:   struct{ X *onlyMethodJSONv1 }{X: &onlyMethodJSONv1{allMethods: allMethods{method: "MarshalJSON", value: []byte(`"hello"`)}}},
		want: `{"X":"hello"}`,
	}, {
		name: "Methods/OnlyMethodText",
		in:   struct{ X *onlyMethodText }{X: &onlyMethodText{allMethods: allMethods{method: "MarshalText", value: []byte(`hello`)}}},
		want: `{"X":"hello"}`,
	}, {
		name: "Methods/IP",
		in:   net.IPv4(192, 168, 0, 100),
		want: `"192.168.0.100"`,
	}, {
		// NOTE: Fixes https://golang.org/issue/46516.
		name: "Methods/Anonymous",
		in:   struct{ X struct{ allMethods } }{X: struct{ allMethods }{allMethods{method: "MarshalNextJSON", value: []byte(`"hello"`)}}},
		want: `{"X":"hello"}`,
	}, {
		// NOTE: Fixes https://golang.org/issue/22967.
		name: "Methods/Addressable",
		in: struct {
			V allMethods
			M map[string]allMethods
			I interface{}
		}{
			V: allMethods{method: "MarshalNextJSON", value: []byte(`"hello"`)},
			M: map[string]allMethods{"K": {method: "MarshalNextJSON", value: []byte(`"hello"`)}},
			I: allMethods{method: "MarshalNextJSON", value: []byte(`"hello"`)},
		},
		want: `{"V":"hello","M":{"K":"hello"},"I":"hello"}`,
	}, {
		// NOTE: Fixes https://golang.org/issue/29732.
		name:         "Methods/MapKey/JSONv2",
		in:           map[structMethodJSONv2]string{{"k1"}: "v1", {"k2"}: "v2"},
		want:         `{"k1":"v1","k2":"v2"}`,
		canonicalize: true,
	}, {
		// NOTE: Fixes https://golang.org/issue/29732.
		name:         "Methods/MapKey/JSONv1",
		in:           map[structMethodJSONv1]string{{"k1"}: "v1", {"k2"}: "v2"},
		want:         `{"k1":"v1","k2":"v2"}`,
		canonicalize: true,
	}, {
		name:         "Methods/MapKey/Text",
		in:           map[structMethodText]string{{"k1"}: "v1", {"k2"}: "v2"},
		want:         `{"k1":"v1","k2":"v2"}`,
		canonicalize: true,
	}, {
		name: "Methods/Invalid/JSONv2/Error",
		in: marshalJSONv2Func(func(*Encoder, MarshalOptions) error {
			return errors.New("some error")
		}),
		wantErr: &SemanticError{action: "marshal", GoType: marshalJSONv2FuncType, Err: errors.New("some error")},
	}, {
		name: "Methods/Invalid/JSONv2/TooFew",
		in: marshalJSONv2Func(func(*Encoder, MarshalOptions) error {
			return nil // do nothing
		}),
		wantErr: &SemanticError{action: "marshal", GoType: marshalJSONv2FuncType, Err: errors.New("must write exactly one JSON value")},
	}, {
		name: "Methods/Invalid/JSONv2/TooMany",
		in: marshalJSONv2Func(func(enc *Encoder, mo MarshalOptions) error {
			enc.WriteToken(Null)
			enc.WriteToken(Null)
			return nil
		}),
		want:    `nullnull`,
		wantErr: &SemanticError{action: "marshal", GoType: marshalJSONv2FuncType, Err: errors.New("must write exactly one JSON value")},
	}, {
		name: "Methods/Invalid/JSONv1/Error",
		in: marshalJSONv1Func(func() ([]byte, error) {
			return nil, errors.New("some error")
		}),
		wantErr: &SemanticError{action: "marshal", GoType: marshalJSONv1FuncType, Err: errors.New("some error")},
	}, {
		name: "Methods/Invalid/JSONv1/Syntax",
		in: marshalJSONv1Func(func() ([]byte, error) {
			return []byte("invalid"), nil
		}),
		wantErr: &SemanticError{action: "marshal", JSONKind: 'i', GoType: marshalJSONv1FuncType, Err: newInvalidCharacterError('i', "at start of value")},
	}, {
		name: "Methods/Invalid/Text/Error",
		in: marshalTextFunc(func() ([]byte, error) {
			return nil, errors.New("some error")
		}),
		wantErr: &SemanticError{action: "marshal", JSONKind: '"', GoType: marshalTextFuncType, Err: errors.New("some error")},
	}, {
		name: "Methods/Invalid/Text/UTF8",
		in: marshalTextFunc(func() ([]byte, error) {
			return []byte("\xde\xad\xbe\xef"), nil
		}),
		wantErr: &SemanticError{action: "marshal", JSONKind: '"', GoType: marshalTextFuncType, Err: &SyntacticError{str: "invalid UTF-8 within string"}},
	}, {
		name: "Methods/Invalid/MapKey/JSONv2/Syntax",
		in: map[interface{}]string{
			addr(marshalJSONv2Func(func(enc *Encoder, mo MarshalOptions) error {
				return enc.WriteToken(Null)
			})): "invalid",
		},
		want:    `{`,
		wantErr: &SemanticError{action: "marshal", GoType: marshalJSONv2FuncType, Err: errMissingName},
	}, {
		name: "Methods/Invalid/MapKey/JSONv1/Syntax",
		in: map[interface{}]string{
			addr(marshalJSONv1Func(func() ([]byte, error) {
				return []byte(`null`), nil
			})): "invalid",
		},
		want:    `{`,
		wantErr: &SemanticError{action: "marshal", JSONKind: 'n', GoType: marshalJSONv1FuncType, Err: errMissingName},
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotErr := tt.mopts.Marshal(tt.eopts, tt.in)
			if tt.canonicalize {
				(*RawValue)(&got).Canonicalize()
			}
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
		dopts   DecodeOptions
		uopts   UnmarshalOptions
		inBuf   string
		inVal   interface{}
		want    interface{}
		wantErr error
	}{{
		name:    "Nil",
		inBuf:   `null`,
		wantErr: &SemanticError{action: "unmarshal", Err: errors.New("value must be passed as a non-nil pointer reference")},
	}, {
		name:    "NilPointer",
		inBuf:   `null`,
		inVal:   (*string)(nil),
		want:    (*string)(nil),
		wantErr: &SemanticError{action: "unmarshal", GoType: stringType, Err: errors.New("value must be passed as a non-nil pointer reference")},
	}, {
		name:    "NonPointer",
		inBuf:   `null`,
		inVal:   "unchanged",
		want:    "unchanged",
		wantErr: &SemanticError{action: "unmarshal", GoType: stringType, Err: errors.New("value must be passed as a non-nil pointer reference")},
	}, {
		name:    "Bools/TrailingJunk",
		inBuf:   `falsetrue`,
		inVal:   addr(true),
		want:    addr(false),
		wantErr: newInvalidCharacterError('t', "after top-level value"),
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
		uopts:   UnmarshalOptions{StringifyNumbers: true},
		inBuf:   `"false"`,
		inVal:   addr(true),
		want:    addr(true),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '"', GoType: boolType},
	}, {
		name:    "Bools/Invalid/StringifiedTrue",
		uopts:   UnmarshalOptions{StringifyNumbers: true},
		inBuf:   `"true"`,
		inVal:   addr(true),
		want:    addr(true),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '"', GoType: boolType},
	}, {
		name:    "Bools/Invalid/Number",
		inBuf:   `0`,
		inVal:   addr(true),
		want:    addr(true),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '0', GoType: boolType},
	}, {
		name:    "Bools/Invalid/String",
		inBuf:   `""`,
		inVal:   addr(true),
		want:    addr(true),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '"', GoType: boolType},
	}, {
		name:    "Bools/Invalid/Object",
		inBuf:   `{}`,
		inVal:   addr(true),
		want:    addr(true),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '{', GoType: boolType},
	}, {
		name:    "Bools/Invalid/Array",
		inBuf:   `[]`,
		inVal:   addr(true),
		want:    addr(true),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '[', GoType: boolType},
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
		wantErr: &SemanticError{action: "unmarshal", JSONKind: 'f', GoType: stringType},
	}, {
		name:    "Strings/Invalid/True",
		inBuf:   `true`,
		inVal:   addr("nochange"),
		want:    addr("nochange"),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: 't', GoType: stringType},
	}, {
		name:    "Strings/Invalid/Object",
		inBuf:   `{}`,
		inVal:   addr("nochange"),
		want:    addr("nochange"),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '{', GoType: stringType},
	}, {
		name:    "Strings/Invalid/Array",
		inBuf:   `[]`,
		inVal:   addr("nochange"),
		want:    addr("nochange"),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '[', GoType: stringType},
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
		uopts: UnmarshalOptions{StringifyNumbers: true},
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
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '"', GoType: array0ByteType, Err: func() error {
			_, err := base64.StdEncoding.Decode(make([]byte, 0), []byte("A"))
			return err
		}()},
	}, {
		name:    "Bytes/ByteArray0/Overflow",
		inBuf:   `"AA=="`,
		inVal:   new([0]byte),
		want:    addr([0]byte{}),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '"', GoType: array0ByteType, Err: errors.New("decoded base64 length of 1 mismatches array length of 0")},
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
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '"', GoType: array1ByteType, Err: func() error {
			_, err := base64.StdEncoding.Decode(make([]byte, 1), []byte("$$=="))
			return err
		}()},
	}, {
		name:    "Bytes/ByteArray1/Underflow",
		inBuf:   `""`,
		inVal:   new([1]byte),
		want:    addr([1]byte{}),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '"', GoType: array1ByteType, Err: errors.New("decoded base64 length of 0 mismatches array length of 1")},
	}, {
		name:    "Bytes/ByteArray1/Overflow",
		inBuf:   `"AQI="`,
		inVal:   new([1]byte),
		want:    addr([1]byte{}),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '"', GoType: array1ByteType, Err: errors.New("decoded base64 length of 2 mismatches array length of 1")},
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
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '"', GoType: array2ByteType, Err: func() error {
			_, err := base64.StdEncoding.Decode(make([]byte, 2), []byte("$$$="))
			return err
		}()},
	}, {
		name:    "Bytes/ByteArray2/Underflow",
		inBuf:   `"AQ=="`,
		inVal:   new([2]byte),
		want:    addr([2]byte{}),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '"', GoType: array2ByteType, Err: errors.New("decoded base64 length of 1 mismatches array length of 2")},
	}, {
		name:    "Bytes/ByteArray2/Overflow",
		inBuf:   `"AQID"`,
		inVal:   new([2]byte),
		want:    addr([2]byte{}),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '"', GoType: array2ByteType, Err: errors.New("decoded base64 length of 3 mismatches array length of 2")},
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
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '"', GoType: array3ByteType, Err: func() error {
			_, err := base64.StdEncoding.Decode(make([]byte, 3), []byte("$$$$"))
			return err
		}()},
	}, {
		name:    "Bytes/ByteArray3/Underflow",
		inBuf:   `"AQI="`,
		inVal:   new([3]byte),
		want:    addr([3]byte{}),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '"', GoType: array3ByteType, Err: errors.New("decoded base64 length of 2 mismatches array length of 3")},
	}, {
		name:    "Bytes/ByteArray3/Overflow",
		inBuf:   `"AQIDAQ=="`,
		inVal:   new([3]byte),
		want:    addr([3]byte{}),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '"', GoType: array3ByteType, Err: errors.New("decoded base64 length of 4 mismatches array length of 3")},
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
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '"', GoType: array4ByteType, Err: func() error {
			_, err := base64.StdEncoding.Decode(make([]byte, 4), []byte("$$$$$$=="))
			return err
		}()},
	}, {
		name:    "Bytes/ByteArray4/Underflow",
		inBuf:   `"AQID"`,
		inVal:   new([4]byte),
		want:    addr([4]byte{}),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '"', GoType: array4ByteType, Err: errors.New("decoded base64 length of 3 mismatches array length of 4")},
	}, {
		name:    "Bytes/ByteArray4/Overflow",
		inBuf:   `"AQIDBAU="`,
		inVal:   new([4]byte),
		want:    addr([4]byte{}),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '"', GoType: array4ByteType, Err: errors.New("decoded base64 length of 5 mismatches array length of 4")},
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
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '"', GoType: bytesType, Err: func() error {
			_, err := base64.StdEncoding.Decode(make([]byte, 0), []byte("AQ="))
			return err
		}()},
	}, {
		name:  "Bytes/Invalid/Unpadded2",
		inBuf: `"AQ"`,
		inVal: addr([]byte("nochange")),
		want:  addr([]byte("nochange")),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '"', GoType: bytesType, Err: func() error {
			_, err := base64.StdEncoding.Decode(make([]byte, 0), []byte("AQ"))
			return err
		}()},
	}, {
		name:  "Bytes/Invalid/Character",
		inBuf: `"@@@@"`,
		inVal: addr([]byte("nochange")),
		want:  addr([]byte("nochange")),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '"', GoType: bytesType, Err: func() error {
			_, err := base64.StdEncoding.Decode(make([]byte, 3), []byte("@@@@"))
			return err
		}()},
	}, {
		name:    "Bytes/Invalid/Bool",
		inBuf:   `true`,
		inVal:   addr([]byte("nochange")),
		want:    addr([]byte("nochange")),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: 't', GoType: bytesType},
	}, {
		name:    "Bytes/Invalid/Number",
		inBuf:   `0`,
		inVal:   addr([]byte("nochange")),
		want:    addr([]byte("nochange")),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '0', GoType: bytesType},
	}, {
		name:    "Bytes/Invalid/Object",
		inBuf:   `{}`,
		inVal:   addr([]byte("nochange")),
		want:    addr([]byte("nochange")),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '{', GoType: bytesType},
	}, {
		name:    "Bytes/Invalid/Array",
		inBuf:   `[]`,
		inVal:   addr([]byte("nochange")),
		want:    addr([]byte("nochange")),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '[', GoType: bytesType},
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
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '0', GoType: int8Type, Err: fmt.Errorf(`cannot parse "-129" as signed integer: %w`, strconv.ErrRange)},
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
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '0', GoType: int8Type, Err: fmt.Errorf(`cannot parse "128" as signed integer: %w`, strconv.ErrRange)},
	}, {
		name:    "Ints/Int16/MinOverflow",
		inBuf:   `-32769`,
		inVal:   addr(int16(-1)),
		want:    addr(int16(-1)),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '0', GoType: int16Type, Err: fmt.Errorf(`cannot parse "-32769" as signed integer: %w`, strconv.ErrRange)},
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
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '0', GoType: int16Type, Err: fmt.Errorf(`cannot parse "32768" as signed integer: %w`, strconv.ErrRange)},
	}, {
		name:    "Ints/Int32/MinOverflow",
		inBuf:   `-2147483649`,
		inVal:   addr(int32(-1)),
		want:    addr(int32(-1)),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '0', GoType: int32Type, Err: fmt.Errorf(`cannot parse "-2147483649" as signed integer: %w`, strconv.ErrRange)},
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
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '0', GoType: int32Type, Err: fmt.Errorf(`cannot parse "2147483648" as signed integer: %w`, strconv.ErrRange)},
	}, {
		name:    "Ints/Int64/MinOverflow",
		inBuf:   `-9223372036854775809`,
		inVal:   addr(int64(-1)),
		want:    addr(int64(-1)),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '0', GoType: int64Type, Err: fmt.Errorf(`cannot parse "-9223372036854775809" as signed integer: %w`, strconv.ErrRange)},
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
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '0', GoType: int64Type, Err: fmt.Errorf(`cannot parse "9223372036854775808" as signed integer: %w`, strconv.ErrRange)},
	}, {
		name:  "Ints/Named",
		inBuf: `-6464`,
		inVal: addr(namedInt64(0)),
		want:  addr(namedInt64(-6464)),
	}, {
		name:  "Ints/Stringified",
		uopts: UnmarshalOptions{StringifyNumbers: true},
		inBuf: `"-6464"`,
		inVal: new(int),
		want:  addr(int(-6464)),
	}, {
		name:  "Ints/Escaped",
		uopts: UnmarshalOptions{StringifyNumbers: true},
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
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '0', GoType: intType, Err: fmt.Errorf(`cannot parse "1.0" as signed integer: %w`, strconv.ErrSyntax)},
	}, {
		name:    "Ints/Invalid/Exponent",
		inBuf:   `1e0`,
		inVal:   addr(int(-1)),
		want:    addr(int(-1)),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '0', GoType: intType, Err: fmt.Errorf(`cannot parse "1e0" as signed integer: %w`, strconv.ErrSyntax)},
	}, {
		name:    "Ints/Invalid/StringifiedFraction",
		uopts:   UnmarshalOptions{StringifyNumbers: true},
		inBuf:   `"1.0"`,
		inVal:   addr(int(-1)),
		want:    addr(int(-1)),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '"', GoType: intType, Err: fmt.Errorf(`cannot parse "1.0" as signed integer: %w`, strconv.ErrSyntax)},
	}, {
		name:    "Ints/Invalid/StringifiedExponent",
		uopts:   UnmarshalOptions{StringifyNumbers: true},
		inBuf:   `"1e0"`,
		inVal:   addr(int(-1)),
		want:    addr(int(-1)),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '"', GoType: intType, Err: fmt.Errorf(`cannot parse "1e0" as signed integer: %w`, strconv.ErrSyntax)},
	}, {
		name:    "Ints/Invalid/Overflow",
		inBuf:   `100000000000000000000000000000`,
		inVal:   addr(int(-1)),
		want:    addr(int(-1)),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '0', GoType: intType, Err: fmt.Errorf(`cannot parse "100000000000000000000000000000" as signed integer: %w`, strconv.ErrRange)},
	}, {
		name:    "Ints/Invalid/OverflowSyntax",
		uopts:   UnmarshalOptions{StringifyNumbers: true},
		inBuf:   `"100000000000000000000000000000x"`,
		inVal:   addr(int(-1)),
		want:    addr(int(-1)),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '"', GoType: intType, Err: fmt.Errorf(`cannot parse "100000000000000000000000000000x" as signed integer: %w`, strconv.ErrSyntax)},
	}, {
		name:    "Ints/Invalid/Whitespace",
		uopts:   UnmarshalOptions{StringifyNumbers: true},
		inBuf:   `"0 "`,
		inVal:   addr(int(-1)),
		want:    addr(int(-1)),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '"', GoType: intType, Err: fmt.Errorf(`cannot parse "0 " as signed integer: %w`, strconv.ErrSyntax)},
	}, {
		name:    "Ints/Invalid/Bool",
		inBuf:   `true`,
		inVal:   addr(int(-1)),
		want:    addr(int(-1)),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: 't', GoType: intType},
	}, {
		name:    "Ints/Invalid/String",
		inBuf:   `"0"`,
		inVal:   addr(int(-1)),
		want:    addr(int(-1)),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '"', GoType: intType},
	}, {
		name:    "Ints/Invalid/Object",
		inBuf:   `{}`,
		inVal:   addr(int(-1)),
		want:    addr(int(-1)),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '{', GoType: intType},
	}, {
		name:    "Ints/Invalid/Array",
		inBuf:   `[]`,
		inVal:   addr(int(-1)),
		want:    addr(int(-1)),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '[', GoType: intType},
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
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '0', GoType: uint8Type, Err: fmt.Errorf(`cannot parse "256" as unsigned integer: %w`, strconv.ErrRange)},
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
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '0', GoType: uint16Type, Err: fmt.Errorf(`cannot parse "65536" as unsigned integer: %w`, strconv.ErrRange)},
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
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '0', GoType: uint32Type, Err: fmt.Errorf(`cannot parse "4294967296" as unsigned integer: %w`, strconv.ErrRange)},
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
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '0', GoType: uint64Type, Err: fmt.Errorf(`cannot parse "18446744073709551616" as unsigned integer: %w`, strconv.ErrRange)},
	}, {
		name:  "Uints/Named",
		inBuf: `6464`,
		inVal: addr(namedUint64(0)),
		want:  addr(namedUint64(6464)),
	}, {
		name:  "Uints/Stringified",
		uopts: UnmarshalOptions{StringifyNumbers: true},
		inBuf: `"6464"`,
		inVal: new(uint),
		want:  addr(uint(6464)),
	}, {
		name:  "Uints/Escaped",
		uopts: UnmarshalOptions{StringifyNumbers: true},
		inBuf: `"\u0036\u0034\u0036\u0034"`,
		inVal: new(uint),
		want:  addr(uint(6464)),
	}, {
		name:    "Uints/Invalid/NegativeOne",
		inBuf:   `-1`,
		inVal:   addr(uint(1)),
		want:    addr(uint(1)),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '0', GoType: uintType, Err: fmt.Errorf(`cannot parse "-1" as unsigned integer: %w`, strconv.ErrSyntax)},
	}, {
		name:    "Uints/Invalid/NegativeZero",
		inBuf:   `-0`,
		inVal:   addr(uint(1)),
		want:    addr(uint(1)),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '0', GoType: uintType, Err: fmt.Errorf(`cannot parse "-0" as unsigned integer: %w`, strconv.ErrSyntax)},
	}, {
		name:    "Uints/Invalid/Fraction",
		inBuf:   `1.0`,
		inVal:   addr(uint(10)),
		want:    addr(uint(10)),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '0', GoType: uintType, Err: fmt.Errorf(`cannot parse "1.0" as unsigned integer: %w`, strconv.ErrSyntax)},
	}, {
		name:    "Uints/Invalid/Exponent",
		inBuf:   `1e0`,
		inVal:   addr(uint(10)),
		want:    addr(uint(10)),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '0', GoType: uintType, Err: fmt.Errorf(`cannot parse "1e0" as unsigned integer: %w`, strconv.ErrSyntax)},
	}, {
		name:    "Uints/Invalid/StringifiedFraction",
		uopts:   UnmarshalOptions{StringifyNumbers: true},
		inBuf:   `"1.0"`,
		inVal:   addr(uint(10)),
		want:    addr(uint(10)),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '"', GoType: uintType, Err: fmt.Errorf(`cannot parse "1.0" as unsigned integer: %w`, strconv.ErrSyntax)},
	}, {
		name:    "Uints/Invalid/StringifiedExponent",
		uopts:   UnmarshalOptions{StringifyNumbers: true},
		inBuf:   `"1e0"`,
		inVal:   addr(uint(10)),
		want:    addr(uint(10)),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '"', GoType: uintType, Err: fmt.Errorf(`cannot parse "1e0" as unsigned integer: %w`, strconv.ErrSyntax)},
	}, {
		name:    "Uints/Invalid/Overflow",
		inBuf:   `100000000000000000000000000000`,
		inVal:   addr(uint(1)),
		want:    addr(uint(1)),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '0', GoType: uintType, Err: fmt.Errorf(`cannot parse "100000000000000000000000000000" as unsigned integer: %w`, strconv.ErrRange)},
	}, {
		name:    "Uints/Invalid/OverflowSyntax",
		uopts:   UnmarshalOptions{StringifyNumbers: true},
		inBuf:   `"100000000000000000000000000000x"`,
		inVal:   addr(uint(1)),
		want:    addr(uint(1)),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '"', GoType: uintType, Err: fmt.Errorf(`cannot parse "100000000000000000000000000000x" as unsigned integer: %w`, strconv.ErrSyntax)},
	}, {
		name:    "Uints/Invalid/Whitespace",
		uopts:   UnmarshalOptions{StringifyNumbers: true},
		inBuf:   `"0 "`,
		inVal:   addr(uint(1)),
		want:    addr(uint(1)),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '"', GoType: uintType, Err: fmt.Errorf(`cannot parse "0 " as unsigned integer: %w`, strconv.ErrSyntax)},
	}, {
		name:    "Uints/Invalid/Bool",
		inBuf:   `true`,
		inVal:   addr(uint(1)),
		want:    addr(uint(1)),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: 't', GoType: uintType},
	}, {
		name:    "Uints/Invalid/String",
		inBuf:   `"0"`,
		inVal:   addr(uint(1)),
		want:    addr(uint(1)),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '"', GoType: uintType},
	}, {
		name:    "Uints/Invalid/Object",
		inBuf:   `{}`,
		inVal:   addr(uint(1)),
		want:    addr(uint(1)),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '{', GoType: uintType},
	}, {
		name:    "Uints/Invalid/Array",
		inBuf:   `[]`,
		inVal:   addr(uint(1)),
		want:    addr(uint(1)),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '[', GoType: uintType},
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
		uopts: UnmarshalOptions{StringifyNumbers: true},
		inBuf: `"64.64"`,
		inVal: new(float64),
		want:  addr(float64(64.64)),
	}, {
		name:  "Floats/Escaped",
		uopts: UnmarshalOptions{StringifyNumbers: true},
		inBuf: `"\u0036\u0034\u002e\u0036\u0034"`,
		inVal: new(float64),
		want:  addr(float64(64.64)),
	}, {
		name:    "Floats/Invalid/NaN",
		uopts:   UnmarshalOptions{StringifyNumbers: true},
		inBuf:   `"NaN"`,
		inVal:   addr(float64(64.64)),
		want:    addr(float64(64.64)),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '"', GoType: float64Type, Err: fmt.Errorf(`cannot parse "NaN" as JSON number: %w`, strconv.ErrSyntax)},
	}, {
		name:    "Floats/Invalid/Infinity",
		uopts:   UnmarshalOptions{StringifyNumbers: true},
		inBuf:   `"Infinity"`,
		inVal:   addr(float64(64.64)),
		want:    addr(float64(64.64)),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '"', GoType: float64Type, Err: fmt.Errorf(`cannot parse "Infinity" as JSON number: %w`, strconv.ErrSyntax)},
	}, {
		name:    "Floats/Invalid/Whitespace",
		uopts:   UnmarshalOptions{StringifyNumbers: true},
		inBuf:   `"1 "`,
		inVal:   addr(float64(64.64)),
		want:    addr(float64(64.64)),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '"', GoType: float64Type, Err: fmt.Errorf(`cannot parse "1 " as JSON number: %w`, strconv.ErrSyntax)},
	}, {
		name:    "Floats/Invalid/GoSyntax",
		uopts:   UnmarshalOptions{StringifyNumbers: true},
		inBuf:   `"1p-2"`,
		inVal:   addr(float64(64.64)),
		want:    addr(float64(64.64)),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '"', GoType: float64Type, Err: fmt.Errorf(`cannot parse "1p-2" as JSON number: %w`, strconv.ErrSyntax)},
	}, {
		name:    "Floats/Invalid/Bool",
		inBuf:   `true`,
		inVal:   addr(float64(64.64)),
		want:    addr(float64(64.64)),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: 't', GoType: float64Type},
	}, {
		name:    "Floats/Invalid/String",
		inBuf:   `"0"`,
		inVal:   addr(float64(64.64)),
		want:    addr(float64(64.64)),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '"', GoType: float64Type},
	}, {
		name:    "Floats/Invalid/Object",
		inBuf:   `{}`,
		inVal:   addr(float64(64.64)),
		want:    addr(float64(64.64)),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '{', GoType: float64Type},
	}, {
		name:    "Floats/Invalid/Array",
		inBuf:   `[]`,
		inVal:   addr(float64(64.64)),
		want:    addr(float64(64.64)),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '[', GoType: float64Type},
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
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '"', GoType: boolType},
	}, {
		name:    "Maps/InvalidKey/NamedBool",
		inBuf:   `{"true":"false"}`,
		inVal:   new(map[namedBool]bool),
		want:    addr(make(map[namedBool]bool)),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '"', GoType: namedBoolType},
	}, {
		name:    "Maps/InvalidKey/Array",
		inBuf:   `{"key":"value"}`,
		inVal:   new(map[[1]string]string),
		want:    addr(make(map[[1]string]string)),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '"', GoType: array1StringType},
	}, {
		name:    "Maps/InvalidKey/Channel",
		inBuf:   `{"key":"value"}`,
		inVal:   new(map[chan string]string),
		want:    addr(make(map[chan string]string)),
		wantErr: &SemanticError{action: "unmarshal", GoType: chanStringType},
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
		wantErr: &SemanticError{action: "unmarshal", GoType: chanStringType},
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
		dopts: DecodeOptions{AllowDuplicateNames: true},
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
		wantErr: &SemanticError{action: "unmarshal", JSONKind: 't', GoType: mapStringStringType},
	}, {
		name:    "Maps/Invalid/String",
		inBuf:   `""`,
		inVal:   addr(map[string]string{"key": "value"}),
		want:    addr(map[string]string{"key": "value"}),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '"', GoType: mapStringStringType},
	}, {
		name:    "Maps/Invalid/Number",
		inBuf:   `0`,
		inVal:   addr(map[string]string{"key": "value"}),
		want:    addr(map[string]string{"key": "value"}),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '0', GoType: mapStringStringType},
	}, {
		name:    "Maps/Invalid/Array",
		inBuf:   `[]`,
		inVal:   addr(map[string]string{"key": "value"}),
		want:    addr(map[string]string{"key": "value"}),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '[', GoType: mapStringStringType},
	}, {
		name:  "Structs/Null",
		inBuf: `null`,
		inVal: addr(structAll{String: "something"}),
		want:  addr(structAll{}),
	}, {
		name:  "Structs/Empty",
		inBuf: `{}`,
		inVal: addr(structAll{
			String: "hello",
			Map:    map[string]string{},
			Slice:  []string{},
		}),
		want: addr(structAll{
			String: "hello",
			Map:    map[string]string{},
			Slice:  []string{},
		}),
	}, {
		name: "Structs/Normal",
		inBuf: `{
	"Bool": true,
	"String": "hello",
	"Bytes": "AQID",
	"Int": -64,
	"Uint": 64,
	"Float": 3.14159,
	"Map": {"key": "value"},
	"StructScalars": {
		"Bool": true,
		"String": "hello",
		"Bytes": "AQID",
		"Int": -64,
		"Uint": 64,
		"Float": 3.14159
	},
	"StructMaps": {
		"MapBool": {"": true},
		"MapString": {"": "hello"},
		"MapBytes": {"": "AQID"},
		"MapInt": {"": -64},
		"MapUint": {"": 64},
		"MapFloat": {"": 3.14159}
	},
	"StructSlices": {
		"SliceBool": [true],
		"SliceString": ["hello"],
		"SliceBytes": ["AQID"],
		"SliceInt": [-64],
		"SliceUint": [64],
		"SliceFloat": [3.14159]
	},
	"Slice": ["fizz","buzz"],
	"Array": ["goodbye"],
	"Ptr": {},
	"Interface": null
}`,
		inVal: new(structAll),
		want: addr(structAll{
			Bool:   true,
			String: "hello",
			Bytes:  []byte{1, 2, 3},
			Int:    -64,
			Uint:   +64,
			Float:  3.14159,
			Map:    map[string]string{"key": "value"},
			StructScalars: structScalars{
				Bool:   true,
				String: "hello",
				Bytes:  []byte{1, 2, 3},
				Int:    -64,
				Uint:   +64,
				Float:  3.14159,
			},
			StructMaps: structMaps{
				MapBool:   map[string]bool{"": true},
				MapString: map[string]string{"": "hello"},
				MapBytes:  map[string][]byte{"": []byte{1, 2, 3}},
				MapInt:    map[string]int64{"": -64},
				MapUint:   map[string]uint64{"": +64},
				MapFloat:  map[string]float64{"": 3.14159},
			},
			StructSlices: structSlices{
				SliceBool:   []bool{true},
				SliceString: []string{"hello"},
				SliceBytes:  [][]byte{[]byte{1, 2, 3}},
				SliceInt:    []int64{-64},
				SliceUint:   []uint64{+64},
				SliceFloat:  []float64{3.14159},
			},
			Slice: []string{"fizz", "buzz"},
			Array: [1]string{"goodbye"},
			Ptr:   new(structAll),
		}),
	}, {
		name: "Structs/Merge",
		inBuf: `{
	"Bool": false,
	"String": "goodbye",
	"Int": -64,
	"Float": 3.14159,
	"Map": {"k2": "v2"},
	"StructScalars": {
		"Bool": true,
		"String": "hello",
		"Bytes": "AQID",
		"Int": -64
	},
	"StructMaps": {
		"MapBool": {"": true},
		"MapString": {"": "hello"},
		"MapBytes": {"": "AQID"},
		"MapInt": {"": -64},
		"MapUint": {"": 64},
		"MapFloat": {"": 3.14159}
	},
	"StructSlices": {
		"SliceString": ["hello"],
		"SliceBytes": ["AQID"],
		"SliceInt": [-64],
		"SliceUint": [64]
	},
	"Slice": ["fizz","buzz"],
	"Array": ["goodbye"],
	"Ptr": {},
	"Interface": {"k2":"v2"}
}`,
		inVal: addr(structAll{
			Bool:   true,
			String: "hello",
			Bytes:  []byte{1, 2, 3},
			Uint:   +64,
			Float:  math.NaN(),
			Map:    map[string]string{"k1": "v1"},
			StructScalars: structScalars{
				String: "hello",
				Bytes:  make([]byte, 2, 4),
				Uint:   +64,
				Float:  3.14159,
			},
			StructMaps: structMaps{
				MapBool:  map[string]bool{"": false},
				MapBytes: map[string][]byte{"": []byte{}},
				MapInt:   map[string]int64{"": 123},
				MapFloat: map[string]float64{"": math.Inf(+1)},
			},
			StructSlices: structSlices{
				SliceBool:  []bool{true},
				SliceBytes: [][]byte{nil, nil},
				SliceInt:   []int64{-123},
				SliceUint:  []uint64{+123},
				SliceFloat: []float64{3.14159},
			},
			Slice:     []string{"buzz", "fizz", "gizz"},
			Array:     [1]string{"hello"},
			Ptr:       new(structAll),
			Interface: map[string]string{"k1": "v1"},
		}),
		want: addr(structAll{
			Bool:   false,
			String: "goodbye",
			Bytes:  []byte{1, 2, 3},
			Int:    -64,
			Uint:   +64,
			Float:  3.14159,
			Map:    map[string]string{"k1": "v1", "k2": "v2"},
			StructScalars: structScalars{
				Bool:   true,
				String: "hello",
				Bytes:  []byte{1, 2, 3},
				Int:    -64,
				Uint:   +64,
				Float:  3.14159,
			},
			StructMaps: structMaps{
				MapBool:   map[string]bool{"": true},
				MapString: map[string]string{"": "hello"},
				MapBytes:  map[string][]byte{"": []byte{1, 2, 3}},
				MapInt:    map[string]int64{"": -64},
				MapUint:   map[string]uint64{"": +64},
				MapFloat:  map[string]float64{"": 3.14159},
			},
			StructSlices: structSlices{
				SliceBool:   []bool{true},
				SliceString: []string{"hello"},
				SliceBytes:  [][]byte{[]byte{1, 2, 3}},
				SliceInt:    []int64{-64},
				SliceUint:   []uint64{+64},
				SliceFloat:  []float64{3.14159},
			},
			Slice:     []string{"fizz", "buzz"},
			Array:     [1]string{"goodbye"},
			Ptr:       new(structAll),
			Interface: map[string]string{"k1": "v1", "k2": "v2"},
		}),
	}, {
		name: "Structs/Stringified/Normal",
		inBuf: `{
	"Bool": true,
	"String": "hello",
	"Bytes": "AQID",
	"Int": -64,
	"Uint": 64,
	"Float": 3.14159,
	"Map": {"key": "value"},
	"StructScalars": {
		"Bool": true,
		"String": "hello",
		"Bytes": "AQID",
		"Int": -64,
		"Uint": 64,
		"Float": 3.14159
	},
	"StructMaps": {
		"MapBool": {"": true},
		"MapString": {"": "hello"},
		"MapBytes": {"": "AQID"},
		"MapInt": {"": -64},
		"MapUint": {"": 64},
		"MapFloat": {"": 3.14159}
	},
	"StructSlices": {
		"SliceBool": [true],
		"SliceString": ["hello"],
		"SliceBytes": ["AQID"],
		"SliceInt": [-64],
		"SliceUint": [64],
		"SliceFloat": [3.14159]
	},
	"Slice": ["fizz","buzz"],
	"Array": ["goodbye"],
	"Ptr": {},
	"Interface": null
}`,
		inVal: new(structStringifiedAll),
		want: addr(structStringifiedAll{
			Bool:   true,
			String: "hello",
			Bytes:  []byte{1, 2, 3},
			Int:    -64,     // may be stringified
			Uint:   +64,     // may be stringified
			Float:  3.14159, // may be stringified
			Map:    map[string]string{"key": "value"},
			StructScalars: structScalars{
				Bool:   true,
				String: "hello",
				Bytes:  []byte{1, 2, 3},
				Int:    -64,     // may be stringified
				Uint:   +64,     // may be stringified
				Float:  3.14159, // may be stringified
			},
			StructMaps: structMaps{
				MapBool:   map[string]bool{"": true},
				MapString: map[string]string{"": "hello"},
				MapBytes:  map[string][]byte{"": []byte{1, 2, 3}},
				MapInt:    map[string]int64{"": -64},       // may be stringified
				MapUint:   map[string]uint64{"": +64},      // may be stringified
				MapFloat:  map[string]float64{"": 3.14159}, // may be stringified
			},
			StructSlices: structSlices{
				SliceBool:   []bool{true},
				SliceString: []string{"hello"},
				SliceBytes:  [][]byte{[]byte{1, 2, 3}},
				SliceInt:    []int64{-64},       // may be stringified
				SliceUint:   []uint64{+64},      // may be stringified
				SliceFloat:  []float64{3.14159}, // may be stringified
			},
			Slice: []string{"fizz", "buzz"},
			Array: [1]string{"goodbye"},
			Ptr:   new(structStringifiedAll), // may be stringified
		}),
	}, {
		name: "Structs/Stringified/String",
		inBuf: `{
	"Bool": true,
	"String": "hello",
	"Bytes": "AQID",
	"Int": "-64",
	"Uint": "64",
	"Float": "3.14159",
	"Map": {"key": "value"},
	"StructScalars": {
		"Bool": true,
		"String": "hello",
		"Bytes": "AQID",
		"Int": "-64",
		"Uint": "64",
		"Float": "3.14159"
	},
	"StructMaps": {
		"MapBool": {"": true},
		"MapString": {"": "hello"},
		"MapBytes": {"": "AQID"},
		"MapInt": {"": "-64"},
		"MapUint": {"": "64"},
		"MapFloat": {"": "3.14159"}
	},
	"StructSlices": {
		"SliceBool": [true],
		"SliceString": ["hello"],
		"SliceBytes": ["AQID"],
		"SliceInt": ["-64"],
		"SliceUint": ["64"],
		"SliceFloat": ["3.14159"]
	},
	"Slice": ["fizz","buzz"],
	"Array": ["goodbye"],
	"Ptr": {},
	"Interface": null
}`,
		inVal: new(structStringifiedAll),
		want: addr(structStringifiedAll{
			Bool:   true,
			String: "hello",
			Bytes:  []byte{1, 2, 3},
			Int:    -64,     // may be stringified
			Uint:   +64,     // may be stringified
			Float:  3.14159, // may be stringified
			Map:    map[string]string{"key": "value"},
			StructScalars: structScalars{
				Bool:   true,
				String: "hello",
				Bytes:  []byte{1, 2, 3},
				Int:    -64,     // may be stringified
				Uint:   +64,     // may be stringified
				Float:  3.14159, // may be stringified
			},
			StructMaps: structMaps{
				MapBool:   map[string]bool{"": true},
				MapString: map[string]string{"": "hello"},
				MapBytes:  map[string][]byte{"": []byte{1, 2, 3}},
				MapInt:    map[string]int64{"": -64},       // may be stringified
				MapUint:   map[string]uint64{"": +64},      // may be stringified
				MapFloat:  map[string]float64{"": 3.14159}, // may be stringified
			},
			StructSlices: structSlices{
				SliceBool:   []bool{true},
				SliceString: []string{"hello"},
				SliceBytes:  [][]byte{[]byte{1, 2, 3}},
				SliceInt:    []int64{-64},       // may be stringified
				SliceUint:   []uint64{+64},      // may be stringified
				SliceFloat:  []float64{3.14159}, // may be stringified
			},
			Slice: []string{"fizz", "buzz"},
			Array: [1]string{"goodbye"},
			Ptr:   new(structStringifiedAll), // may be stringified
		}),
	}, {
		name:  "Structs/UnexportedIgnored",
		inBuf: `{"ignored":"unused"}`,
		inVal: new(structUnexportedIgnored),
		want:  new(structUnexportedIgnored),
	}, {
		name:  "Structs/IgnoredUnexportedEmbedded",
		inBuf: `{"namedString":"unused"}`,
		inVal: new(structIgnoredUnexportedEmbedded),
		want:  new(structIgnoredUnexportedEmbedded),
	}, {
		name:  "Structs/WeirdNames",
		inBuf: `{"":"empty",",":"comma","\"":"quote"}`,
		inVal: new(structWeirdNames),
		want:  addr(structWeirdNames{Empty: "empty", Comma: "comma", Quote: "quote"}),
	}, {
		name:  "Structs/NoCase/Exact",
		inBuf: `{"AaA":"AaA","AAa":"AAa","AAA":"AAA"}`,
		inVal: new(structNoCase),
		want:  addr(structNoCase{AaA: "AaA", AAa: "AAa", AAA: "AAA"}),
	}, {
		name:  "Structs/NoCase/Merge",
		inBuf: `{"AaA":"AaA","aaa":"aaa","aAa":"aAa"}`,
		inVal: new(structNoCase),
		want:  addr(structNoCase{AaA: "aAa"}),
	}, {
		name:    "Structs/Invalid/ErrUnexpectedEOF",
		inBuf:   ``,
		inVal:   addr(structAll{}),
		want:    addr(structAll{}),
		wantErr: io.ErrUnexpectedEOF,
	}, {
		name:    "Structs/Invalid/NestedErrUnexpectedEOF",
		inBuf:   `{"Ptr":`,
		inVal:   addr(structAll{}),
		want:    addr(structAll{Ptr: new(structAll)}),
		wantErr: io.ErrUnexpectedEOF,
	}, {
		name:    "Structs/Invalid/Conflicting",
		inBuf:   `{}`,
		inVal:   addr(structConflicting{}),
		want:    addr(structConflicting{}),
		wantErr: &SemanticError{action: "unmarshal", GoType: structConflictingType, Err: errors.New("Go struct fields A and B conflict over JSON object name \"conflict\"")},
	}, {
		name:    "Structs/Invalid/NoneExported",
		inBuf:   `{}`,
		inVal:   addr(structNoneExported{}),
		want:    addr(structNoneExported{}),
		wantErr: &SemanticError{action: "unmarshal", GoType: structNoneExportedType, Err: errors.New("Go struct kind has no exported fields")},
	}, {
		name:    "Structs/Invalid/MalformedTag",
		inBuf:   `{}`,
		inVal:   addr(structMalformedTag{}),
		want:    addr(structMalformedTag{}),
		wantErr: &SemanticError{action: "unmarshal", GoType: structMalformedTagType, Err: errors.New("Go struct field Malformed has malformed `json` tag: invalid character '\"' at start of option (expecting Unicode letter or single quote)")},
	}, {
		name:    "Structs/Invalid/UnexportedTag",
		inBuf:   `{}`,
		inVal:   addr(structUnexportedTag{}),
		want:    addr(structUnexportedTag{}),
		wantErr: &SemanticError{action: "unmarshal", GoType: structUnexportedTagType, Err: errors.New("unexported Go struct field unexported cannot have non-ignored `json:\"name\"` tag")},
	}, {
		name:    "Structs/Invalid/UnexportedEmbedded",
		inBuf:   `{}`,
		inVal:   addr(structUnexportedEmbedded{}),
		want:    addr(structUnexportedEmbedded{}),
		wantErr: &SemanticError{action: "unmarshal", GoType: structUnexportedEmbeddedType, Err: errors.New("embedded Go struct field namedString of an unexported type must be explicitly ignored with a `json:\"-\"` tag")},
	}, {
		name:  "Slices/Null",
		inBuf: `null`,
		inVal: addr([]string{"something"}),
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
		wantErr: &SemanticError{action: "unmarshal", GoType: chanStringType},
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
		wantErr: &SemanticError{action: "unmarshal", JSONKind: 't', GoType: sliceStringType},
	}, {
		name:    "Slices/Invalid/String",
		inBuf:   `""`,
		inVal:   addr([]string{"nochange"}),
		want:    addr([]string{"nochange"}),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '"', GoType: sliceStringType},
	}, {
		name:    "Slices/Invalid/Number",
		inBuf:   `0`,
		inVal:   addr([]string{"nochange"}),
		want:    addr([]string{"nochange"}),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '0', GoType: sliceStringType},
	}, {
		name:    "Slices/Invalid/Object",
		inBuf:   `{}`,
		inVal:   addr([]string{"nochange"}),
		want:    addr([]string{"nochange"}),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '{', GoType: sliceStringType},
	}, {
		name:  "Arrays/Null",
		inBuf: `null`,
		inVal: addr([1]string{"something"}),
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
		wantErr: &SemanticError{action: "unmarshal", GoType: chanStringType},
	}, {
		name:    "Arrays/Invalid/Underflow",
		inBuf:   `[]`,
		inVal:   new([1]string),
		want:    addr([1]string{}),
		wantErr: &SemanticError{action: "unmarshal", GoType: array1StringType, Err: errors.New("too few array elements")},
	}, {
		name:    "Arrays/Invalid/Overflow",
		inBuf:   `["1","2"]`,
		inVal:   new([1]string),
		want:    addr([1]string{"1"}),
		wantErr: &SemanticError{action: "unmarshal", GoType: array1StringType, Err: errors.New("too many array elements")},
	}, {
		name:    "Arrays/Invalid/Bool",
		inBuf:   `true`,
		inVal:   addr([1]string{"nochange"}),
		want:    addr([1]string{"nochange"}),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: 't', GoType: array1StringType},
	}, {
		name:    "Arrays/Invalid/String",
		inBuf:   `""`,
		inVal:   addr([1]string{"nochange"}),
		want:    addr([1]string{"nochange"}),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '"', GoType: array1StringType},
	}, {
		name:    "Arrays/Invalid/Number",
		inBuf:   `0`,
		inVal:   addr([1]string{"nochange"}),
		want:    addr([1]string{"nochange"}),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '0', GoType: array1StringType},
	}, {
		name:    "Arrays/Invalid/Object",
		inBuf:   `{}`,
		inVal:   addr([1]string{"nochange"}),
		want:    addr([1]string{"nochange"}),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '{', GoType: array1StringType},
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
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '"', GoType: ioReaderType, Err: errors.New("cannot derive concrete type for non-empty interface")},
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
		name:  "Interfaces/Merge/Map",
		inBuf: `{"k2":"v2"}`,
		inVal: func() interface{} {
			var vi interface{} = map[string]string{"k1": "v1"}
			return &vi
		}(),
		want: func() interface{} {
			var vi interface{} = map[string]string{"k1": "v1", "k2": "v2"}
			return &vi
		}(),
	}, {
		name:  "Interfaces/Merge/Struct",
		inBuf: `{"Array":["goodbye"]}`,
		inVal: func() interface{} {
			var vi interface{} = structAll{String: "hello"}
			return &vi
		}(),
		want: func() interface{} {
			var vi interface{} = structAll{String: "hello", Array: [1]string{"goodbye"}}
			return &vi
		}(),
	}, {
		name:  "Interfaces/Merge/NamedInt",
		inBuf: `64`,
		inVal: func() interface{} {
			var vi interface{} = namedInt64(-64)
			return &vi
		}(),
		want: func() interface{} {
			var vi interface{} = namedInt64(+64)
			return &vi
		}(),
	}, {
		name:  "Methods/NilPointer/Null",
		inBuf: `{"X":null}`,
		inVal: addr(struct{ X *allMethods }{X: (*allMethods)(nil)}),
		want:  addr(struct{ X *allMethods }{X: (*allMethods)(nil)}), // method should not be called
	}, {
		name:  "Methods/NilPointer/Value",
		inBuf: `{"X":"value"}`,
		inVal: addr(struct{ X *allMethods }{X: (*allMethods)(nil)}),
		want:  addr(struct{ X *allMethods }{X: &allMethods{method: "UnmarshalNextJSON", value: []byte(`"value"`)}}),
	}, {
		name:  "Methods/NilInterface/Null",
		inBuf: `{"X":null}`,
		inVal: addr(struct{ X MarshalerV2 }{X: (*allMethods)(nil)}),
		want:  addr(struct{ X MarshalerV2 }{X: nil}), // interface value itself is nil'd out
	}, {
		name:  "Methods/NilInterface/Value",
		inBuf: `{"X":"value"}`,
		inVal: addr(struct{ X MarshalerV2 }{X: (*allMethods)(nil)}),
		want:  addr(struct{ X MarshalerV2 }{X: &allMethods{method: "UnmarshalNextJSON", value: []byte(`"value"`)}}),
	}, {
		name:  "Methods/AllMethods",
		inBuf: `{"X":"hello"}`,
		inVal: new(struct{ X *allMethods }),
		want:  addr(struct{ X *allMethods }{X: &allMethods{method: "UnmarshalNextJSON", value: []byte(`"hello"`)}}),
	}, {
		name:  "Methods/AllMethodsExceptJSONv2",
		inBuf: `{"X":"hello"}`,
		inVal: new(struct{ X *allMethodsExceptJSONv2 }),
		want:  addr(struct{ X *allMethodsExceptJSONv2 }{X: &allMethodsExceptJSONv2{allMethods: allMethods{method: "UnmarshalJSON", value: []byte(`"hello"`)}}}),
	}, {
		name:  "Methods/AllMethodsExceptJSONv1",
		inBuf: `{"X":"hello"}`,
		inVal: new(struct{ X *allMethodsExceptJSONv1 }),
		want:  addr(struct{ X *allMethodsExceptJSONv1 }{X: &allMethodsExceptJSONv1{allMethods: allMethods{method: "UnmarshalNextJSON", value: []byte(`"hello"`)}}}),
	}, {
		name:  "Methods/AllMethodsExceptText",
		inBuf: `{"X":"hello"}`,
		inVal: new(struct{ X *allMethodsExceptText }),
		want:  addr(struct{ X *allMethodsExceptText }{X: &allMethodsExceptText{allMethods: allMethods{method: "UnmarshalNextJSON", value: []byte(`"hello"`)}}}),
	}, {
		name:  "Methods/OnlyMethodJSONv2",
		inBuf: `{"X":"hello"}`,
		inVal: new(struct{ X *onlyMethodJSONv2 }),
		want:  addr(struct{ X *onlyMethodJSONv2 }{X: &onlyMethodJSONv2{allMethods: allMethods{method: "UnmarshalNextJSON", value: []byte(`"hello"`)}}}),
	}, {
		name:  "Methods/OnlyMethodJSONv1",
		inBuf: `{"X":"hello"}`,
		inVal: new(struct{ X *onlyMethodJSONv1 }),
		want:  addr(struct{ X *onlyMethodJSONv1 }{X: &onlyMethodJSONv1{allMethods: allMethods{method: "UnmarshalJSON", value: []byte(`"hello"`)}}}),
	}, {
		name:  "Methods/OnlyMethodText",
		inBuf: `{"X":"hello"}`,
		inVal: new(struct{ X *onlyMethodText }),
		want:  addr(struct{ X *onlyMethodText }{X: &onlyMethodText{allMethods: allMethods{method: "UnmarshalText", value: []byte(`hello`)}}}),
	}, {
		name:  "Methods/IP",
		inBuf: `"192.168.0.100"`,
		inVal: new(net.IP),
		want:  addr(net.IPv4(192, 168, 0, 100)),
	}, {
		// NOTE: Fixes https://golang.org/issue/46516.
		name:  "Methods/Anonymous",
		inBuf: `{"X":"hello"}`,
		inVal: new(struct{ X struct{ allMethods } }),
		want:  addr(struct{ X struct{ allMethods } }{X: struct{ allMethods }{allMethods{method: "UnmarshalNextJSON", value: []byte(`"hello"`)}}}),
	}, {
		// NOTE: Fixes https://golang.org/issue/22967.
		name:  "Methods/Addressable",
		inBuf: `{"V":"hello","M":{"K":"hello"},"I":"hello"}`,
		inVal: addr(struct {
			V allMethods
			M map[string]allMethods
			I interface{}
		}{
			I: allMethods{}, // need to initialize with concrete value
		}),
		want: addr(struct {
			V allMethods
			M map[string]allMethods
			I interface{}
		}{
			V: allMethods{method: "UnmarshalNextJSON", value: []byte(`"hello"`)},
			M: map[string]allMethods{"K": {method: "UnmarshalNextJSON", value: []byte(`"hello"`)}},
			I: allMethods{method: "UnmarshalNextJSON", value: []byte(`"hello"`)},
		}),
	}, {
		// NOTE: Fixes https://golang.org/issue/29732.
		name:  "Methods/MapKey/JSONv2",
		inBuf: `{"k1":"v1b","k2":"v2"}`,
		inVal: addr(map[structMethodJSONv2]string{{"k1"}: "v1a", {"k3"}: "v3"}),
		want:  addr(map[structMethodJSONv2]string{{"k1"}: "v1b", {"k2"}: "v2", {"k3"}: "v3"}),
	}, {
		// NOTE: Fixes https://golang.org/issue/29732.
		name:  "Methods/MapKey/JSONv1",
		inBuf: `{"k1":"v1b","k2":"v2"}`,
		inVal: addr(map[structMethodJSONv1]string{{"k1"}: "v1a", {"k3"}: "v3"}),
		want:  addr(map[structMethodJSONv1]string{{"k1"}: "v1b", {"k2"}: "v2", {"k3"}: "v3"}),
	}, {
		name:  "Methods/MapKey/Text",
		inBuf: `{"k1":"v1b","k2":"v2"}`,
		inVal: addr(map[structMethodText]string{{"k1"}: "v1a", {"k3"}: "v3"}),
		want:  addr(map[structMethodText]string{{"k1"}: "v1b", {"k2"}: "v2", {"k3"}: "v3"}),
	}, {
		name:  "Methods/Invalid/JSONv2/Error",
		inBuf: `{}`,
		inVal: addr(unmarshalJSONv2Func(func(*Decoder, UnmarshalOptions) error {
			return errors.New("some error")
		})),
		wantErr: &SemanticError{action: "unmarshal", GoType: unmarshalJSONv2FuncType, Err: errors.New("some error")},
	}, {
		name: "Methods/Invalid/JSONv2/TooFew",
		inVal: addr(unmarshalJSONv2Func(func(*Decoder, UnmarshalOptions) error {
			return nil // do nothing
		})),
		wantErr: &SemanticError{action: "unmarshal", GoType: unmarshalJSONv2FuncType, Err: errors.New("must read exactly one JSON value")},
	}, {
		name:  "Methods/Invalid/JSONv2/TooMany",
		inBuf: `{}{}`,
		inVal: addr(unmarshalJSONv2Func(func(dec *Decoder, uo UnmarshalOptions) error {
			dec.ReadValue()
			dec.ReadValue()
			return nil
		})),
		wantErr: &SemanticError{action: "unmarshal", GoType: unmarshalJSONv2FuncType, Err: errors.New("must read exactly one JSON value")},
	}, {
		name:  "Methods/Invalid/JSONv1/Error",
		inBuf: `{}`,
		inVal: addr(unmarshalJSONv1Func(func([]byte) error {
			return errors.New("some error")
		})),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '{', GoType: unmarshalJSONv1FuncType, Err: errors.New("some error")},
	}, {
		name:  "Methods/Invalid/Text/Error",
		inBuf: `"value"`,
		inVal: addr(unmarshalTextFunc(func([]byte) error {
			return errors.New("some error")
		})),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '"', GoType: unmarshalTextFuncType, Err: errors.New("some error")},
	}, {
		name:  "Methods/Invalid/Text/Syntax",
		inBuf: `{}`,
		inVal: addr(unmarshalTextFunc(func([]byte) error {
			panic("should not be called")
		})),
		wantErr: &SemanticError{action: "unmarshal", JSONKind: '{', GoType: unmarshalTextFuncType, Err: errors.New("JSON value must be string type")},
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.inVal
			gotErr := tt.uopts.Unmarshal(tt.dopts, []byte(tt.inBuf), got)
			if !reflect.DeepEqual(got, tt.want) && tt.want != nil {
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
