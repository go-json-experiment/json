// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package jsontext

import (
	"math"
	"reflect"
	"strconv"
	"testing"
)

const (
	maxInt64  = math.MaxInt64
	minInt64  = math.MinInt64
	maxUint64 = math.MaxUint64
	minUint64 = 0 // for consistency and readability purposes
)

func TestTokenStringAllocations(t *testing.T) {
	if testing.CoverMode() != "" {
		t.Skip("coverage mode breaks the compiler optimization this depends on")
	}

	tok := rawToken(`"hello"`)
	var m map[string]bool
	got := int(testing.AllocsPerRun(10, func() {
		// This function uses tok.String() is a non-escaping manner
		// (i.e., looking it up in a Go map). It should not allocate.
		if m[tok.String()] {
			panic("never executed")
		}
	}))
	if got > 0 {
		t.Errorf("Token.String allocated %d times, want 0", got)
	}
}

func TestTokenAccessors(t *testing.T) {
	type token struct {
		Bool   bool
		String string
		Float  float64
		Int    int64
		Uint   uint64
		Kind   Kind
	}

	tests := []struct {
		in   Token
		want token
	}{
		{Token{}, token{String: "<invalid jsontext.Token>"}},
		{Null, token{String: "null", Kind: 'n'}},
		{False, token{Bool: false, String: "false", Kind: 'f'}},
		{True, token{Bool: true, String: "true", Kind: 't'}},
		{Bool(false), token{Bool: false, String: "false", Kind: 'f'}},
		{Bool(true), token{Bool: true, String: "true", Kind: 't'}},
		{ObjectStart, token{String: "{", Kind: '{'}},
		{ObjectEnd, token{String: "}", Kind: '}'}},
		{ArrayStart, token{String: "[", Kind: '['}},
		{ArrayEnd, token{String: "]", Kind: ']'}},
		{String(""), token{String: "", Kind: '"'}},
		{String("hello, world!"), token{String: "hello, world!", Kind: '"'}},
		{rawToken(`"hello, world!"`), token{String: "hello, world!", Kind: '"'}},
		{Float(0), token{String: "0", Float: 0, Kind: '0'}},
		{Float(1.2), token{String: "1.2", Float: 1.2, Kind: '0'}},
		{Float(math.Copysign(0, -1)), token{String: "-0", Float: math.Copysign(0, -1), Int: 0, Uint: 0, Kind: '0'}},
		{Float(math.NaN()), token{String: "NaN", Float: math.NaN(), Int: 0, Uint: 0, Kind: '0'}},
		{Float(math.Inf(+1)), token{String: "+Inf", Float: math.Inf(+1), Kind: '0'}},
		{Float(math.Inf(-1)), token{String: "-Inf", Float: math.Inf(-1), Kind: '0'}},
		{Int(minInt64), token{String: "-9223372036854775808", Int: minInt64, Uint: minUint64, Kind: '0'}},
		{Int(minInt64 + 1), token{String: "-9223372036854775807", Int: minInt64 + 1, Kind: '0'}},
		{Int(-1), token{String: "-1", Int: -1, Kind: '0'}},
		{Int(0), token{String: "0", Int: 0, Kind: '0'}},
		{Int(+1), token{String: "1", Int: +1, Kind: '0'}},
		{Int(maxInt64 - 1), token{String: "9223372036854775806", Int: maxInt64 - 1, Kind: '0'}},
		{Int(maxInt64), token{String: "9223372036854775807", Int: maxInt64, Kind: '0'}},
		{Uint(minUint64), token{String: "0", Kind: '0'}},
		{Uint(minUint64 + 1), token{String: "1", Uint: minUint64 + 1, Kind: '0'}},
		{Uint(maxUint64 - 1), token{String: "18446744073709551614", Uint: maxUint64 - 1, Kind: '0'}},
		{Uint(maxUint64), token{String: "18446744073709551615", Uint: maxUint64, Kind: '0'}},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := token{
				Bool: func() bool {
					defer func() { recover() }()
					return tt.in.Bool()
				}(),
				String: tt.in.String(),
				Float: func() float64 {
					defer func() { recover() }()
					return tt.in.Float()
				}(),
				Int: func() int64 {
					defer func() { recover() }()
					return tt.in.Int()
				}(),
				Uint: func() uint64 {
					defer func() { recover() }()
					return tt.in.Uint()
				}(),
				Kind: tt.in.Kind(),
			}

			if got.Bool != tt.want.Bool {
				t.Errorf("Token(%s).Bool() = %v, want %v", tt.in, got.Bool, tt.want.Bool)
			}
			if got.String != tt.want.String {
				t.Errorf("Token(%s).String() = %v, want %v", tt.in, got.String, tt.want.String)
			}
			if math.Float64bits(got.Float) != math.Float64bits(tt.want.Float) {
				t.Errorf("Token(%s).Float() = %v, want %v", tt.in, got.Float, tt.want.Float)
			}
			if got.Int != tt.want.Int {
				t.Errorf("Token(%s).Int() = %v, want %v", tt.in, got.Int, tt.want.Int)
			}
			if got.Uint != tt.want.Uint {
				t.Errorf("Token(%s).Uint() = %v, want %v", tt.in, got.Uint, tt.want.Uint)
			}
			if got.Kind != tt.want.Kind {
				t.Errorf("Token(%s).Kind() = %v, want %v", tt.in, got.Kind, tt.want.Kind)
			}
		})
	}
}

func TestTokenAccessorRaw(t *testing.T) {
	if !reflect.DeepEqual(False, Raw(False.Raw())) {
		t.Error("False != Raw(False.Raw())")
	}

	raw := func() *RawToken {
		defer func() { recover() }()
		raw := Float(0.).Raw()
		return &raw
	}()
	if raw != nil {
		t.Error("Float(0.).Raw() should panic")
	}
}

func assertParse[T comparable](t *testing.T, s string, parse func(t RawToken, bits int) (T, error), wantV T, wantErr error) {
	t.Helper()
	gotV, gotErr := parse(rawToken(s).raw, 64)
	if gotV != wantV {
		t.Errorf("RawToken.ParseXXX(64) = %v, want %v", gotV, wantV)
	}
	if gotErr != wantErr {
		t.Errorf("RawToken.ParseXXX(64) error = %v, want %v", gotErr, wantErr)
	}
}

func TestTokenParseNumber(t *testing.T) {
	assertParse(t, `1.23`, RawToken.ParseFloat, 1.23, nil)
	assertParse(t, `1e1000`, RawToken.ParseFloat, math.Inf(+1), strconv.ErrRange)
	assertParse(t, `"anything"`, RawToken.ParseFloat, 0, ErrUnexpectedKind)

	assertParse(t, "123", RawToken.ParseInt, 123, nil)
	assertParse(t, "99999999999999999999", RawToken.ParseInt, math.MaxInt64, strconv.ErrRange)
	assertParse(t, "false", RawToken.ParseInt, 0, ErrUnexpectedKind)

	assertParse(t, "123", RawToken.ParseUint, 123, nil)
	assertParse(t, "-1", RawToken.ParseUint, 0, strconv.ErrSyntax)
	assertParse(t, "false", RawToken.ParseUint, 0, ErrUnexpectedKind)
}

func TestTokenClone(t *testing.T) {
	tests := []struct {
		in           Token
		wantExactRaw bool
	}{
		{Token{}, true},
		{Null, true},
		{False, true},
		{True, true},
		{ObjectStart, true},
		{ObjectEnd, true},
		{ArrayStart, true},
		{ArrayEnd, true},
		{String("hello, world!"), true},
		{rawToken(`"hello, world!"`), false},
		{Float(3.14159), true},
		{rawToken(`3.14159`), false},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := tt.in.Clone()
			if !reflect.DeepEqual(got, tt.in) {
				t.Errorf("Token(%s) == Token(%s).Clone() = false, want true", tt.in, tt.in)
			}
			gotExactRaw := got.raw.dBuf == tt.in.raw.dBuf
			if gotExactRaw != tt.wantExactRaw {
				t.Errorf("Token(%s).raw == Token(%s).Clone().raw = %v, want %v", tt.in, tt.in, gotExactRaw, tt.wantExactRaw)
			}
		})
	}
}
