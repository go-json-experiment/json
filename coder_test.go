// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import (
	"bytes"
	"errors"
	"io"
	"math"
	"math/rand"
	"path"
	"strings"
	"testing"
)

var (
	zeroToken Token
	zeroValue RawValue
)

// tokOrVal is either a Token or a RawValue.
type tokOrVal interface{ Kind() Kind }

type coderTestdataEntry struct {
	name        string
	in          string
	outCompact  string
	outEscaped  string // outCompact if empty; escapes all runes in a string
	outIndented string // outCompact if empty; uses ">" for indent prefix and "_" for indent
	tokens      []Token
}

var coderTestdata = []coderTestdataEntry{{
	name:       "Null",
	in:         ` null `,
	outCompact: `null`,
	tokens:     []Token{Null},
}, {
	name:       "False",
	in:         ` false `,
	outCompact: `false`,
	tokens:     []Token{False},
}, {
	name:       "True",
	in:         ` true `,
	outCompact: `true`,
	tokens:     []Token{True},
}, {
	name:       "EmptyString",
	in:         ` "" `,
	outCompact: `""`,
	tokens:     []Token{String("")},
}, {
	name:       "SimpleString",
	in:         ` "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ" `,
	outCompact: `"abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"`,
	outEscaped: `"\u0061\u0062\u0063\u0064\u0065\u0066\u0067\u0068\u0069\u006a\u006b\u006c\u006d\u006e\u006f\u0070\u0071\u0072\u0073\u0074\u0075\u0076\u0077\u0078\u0079\u007a\u0041\u0042\u0043\u0044\u0045\u0046\u0047\u0048\u0049\u004a\u004b\u004c\u004d\u004e\u004f\u0050\u0051\u0052\u0053\u0054\u0055\u0056\u0057\u0058\u0059\u005a"`,
	tokens:     []Token{String("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")},
}, {
	name:       "ComplicatedString",
	in:         " \"Hello, 世界 🌟★☆✩🌠 " + "\u0080\u00f6\u20ac\ud799\ue000\ufb33\ufffd\U0001f602" + ` \ud800\udead \"\\\/\b\f\n\r\t \u0022\u005c\u002f\u0008\u000c\u000a\u000d\u0009" `,
	outCompact: "\"Hello, 世界 🌟★☆✩🌠 " + "\u0080\u00f6\u20ac\ud799\ue000\ufb33\ufffd\U0001f602" + " 𐊭 \\\"\\\\/\\b\\f\\n\\r\\t \\\"\\\\/\\b\\f\\n\\r\\t\"",
	outEscaped: `"\u0048\u0065\u006c\u006c\u006f\u002c\u0020\u4e16\u754c\u0020\ud83c\udf1f\u2605\u2606\u2729\ud83c\udf20\u0020\u0080\u00f6\u20ac\ud799\ue000\ufb33\ufffd\ud83d\ude02\u0020\ud800\udead\u0020\u0022\u005c\u002f\u0008\u000c\u000a\u000d\u0009\u0020\u0022\u005c\u002f\u0008\u000c\u000a\u000d\u0009"`,
	tokens:     []Token{rawToken("\"Hello, 世界 🌟★☆✩🌠 " + "\u0080\u00f6\u20ac\ud799\ue000\ufb33\ufffd\U0001f602" + " 𐊭 \\\"\\\\/\\b\\f\\n\\r\\t \\\"\\\\/\\b\\f\\n\\r\\t\"")},
}, {
	name:       "ZeroNumber",
	in:         ` 0 `,
	outCompact: `0`,
	tokens:     []Token{Uint(0)},
}, {
	name:       "SimpleNumber",
	in:         ` 123456789 `,
	outCompact: `123456789`,
	tokens:     []Token{Uint(123456789)},
}, {
	name:       "NegativeNumber",
	in:         ` -123456789 `,
	outCompact: `-123456789`,
	tokens:     []Token{Int(-123456789)},
}, {
	name:       "FractionalNumber",
	in:         " 0.123456789 ",
	outCompact: `0.123456789`,
	tokens:     []Token{Float(0.123456789)},
}, {
	name:       "ExponentNumber",
	in:         " 0e12456789 ",
	outCompact: `0e12456789`,
	tokens:     []Token{rawToken(`0e12456789`)},
}, {
	name:       "ExponentNumberP",
	in:         " 0e+12456789 ",
	outCompact: `0e+12456789`,
	tokens:     []Token{rawToken(`0e+12456789`)},
}, {
	name:       "ExponentNumberN",
	in:         " 0e-12456789 ",
	outCompact: `0e-12456789`,
	tokens:     []Token{rawToken(`0e-12456789`)},
}, {
	name:       "ComplicatedNumber",
	in:         ` -123456789.987654321E+0123456789 `,
	outCompact: `-123456789.987654321E+0123456789`,
	tokens:     []Token{rawToken(`-123456789.987654321E+0123456789`)},
}, {
	name: "Numbers",
	in: ` [
	0, -0, 0.0, -0.0, 1.00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001,
	-5e-324, 1e+100, 1.7976931348623157e+308, 9007199254740992,
	-9223372036854775808, 9223372036854775807, 0, 18446744073709551615
] `,
	outCompact: "[0,-0,0.0,-0.0,1.00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001,-5e-324,1e+100,1.7976931348623157e+308,9007199254740992,-9223372036854775808,9223372036854775807,0,18446744073709551615]",
	outIndented: `[
>_0,
>_-0,
>_0.0,
>_-0.0,
>_1.00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001,
>_-5e-324,
>_1e+100,
>_1.7976931348623157e+308,
>_9007199254740992,
>_-9223372036854775808,
>_9223372036854775807,
>_0,
>_18446744073709551615
>]`,
	tokens: []Token{
		ArrayStart,
		Float(0), Float(math.Copysign(0, -1)), rawToken(`0.0`), rawToken(`-0.0`), rawToken(`1.00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001`),
		Float(-5e-324), Float(1e100), Float(1.7976931348623157e+308), Float(9007199254740992),
		Int(minInt64), Int(maxInt64), Uint(minUint64), Uint(maxUint64),
		ArrayEnd,
	},
}, {
	name:       "ObjectN0",
	in:         ` { } `,
	outCompact: `{}`,
	tokens:     []Token{ObjectStart, ObjectEnd},
}, {
	name:        "ObjectN1",
	in:          ` { "0" : 0 } `,
	outCompact:  `{"0":0}`,
	outEscaped:  `{"\u0030":0}`,
	outIndented: "{\n>_\"0\": 0\n>}",
	tokens:      []Token{ObjectStart, String("0"), Uint(0), ObjectEnd},
}, {
	name:        "ObjectN2",
	in:          ` { "0" : 0 , "1" : 1 } `,
	outCompact:  `{"0":0,"1":1}`,
	outEscaped:  `{"\u0030":0,"\u0031":1}`,
	outIndented: "{\n>_\"0\": 0,\n>_\"1\": 1\n>}",
	tokens:      []Token{ObjectStart, String("0"), Uint(0), String("1"), Uint(1), ObjectEnd},
}, {
	name:        "ObjectNested",
	in:          ` { "0" : { "1" : { "2" : { "3" : { "4" : {  } } } } } } `,
	outCompact:  `{"0":{"1":{"2":{"3":{"4":{}}}}}}`,
	outEscaped:  `{"\u0030":{"\u0031":{"\u0032":{"\u0033":{"\u0034":{}}}}}}`,
	outIndented: "{\n>_\"0\": {\n>__\"1\": {\n>___\"2\": {\n>____\"3\": {\n>_____\"4\": {}\n>____}\n>___}\n>__}\n>_}\n>}",
	tokens:      []Token{ObjectStart, String("0"), ObjectStart, String("1"), ObjectStart, String("2"), ObjectStart, String("3"), ObjectStart, String("4"), ObjectStart, ObjectEnd, ObjectEnd, ObjectEnd, ObjectEnd, ObjectEnd, ObjectEnd},
}, {
	name:       "ArrayN0",
	in:         ` [ ] `,
	outCompact: `[]`,
	tokens:     []Token{ArrayStart, ArrayEnd},
}, {
	name:        "ArrayN1",
	in:          ` [ 0 ] `,
	outCompact:  `[0]`,
	outIndented: "[\n>_0\n>]",
	tokens:      []Token{ArrayStart, Uint(0), ArrayEnd},
}, {
	name:        "ArrayN2",
	in:          ` [ 0 , 1 ] `,
	outCompact:  `[0,1]`,
	outIndented: "[\n>_0,\n>_1\n>]",
	tokens:      []Token{ArrayStart, Uint(0), Uint(1), ArrayEnd},
}, {
	name:        "ArrayNested",
	in:          ` [ [ [ [ [ ] ] ] ] ] `,
	outCompact:  `[[[[[]]]]]`,
	outIndented: "[\n>_[\n>__[\n>___[\n>____[]\n>___]\n>__]\n>_]\n>]",
	tokens:      []Token{ArrayStart, ArrayStart, ArrayStart, ArrayStart, ArrayStart, ArrayEnd, ArrayEnd, ArrayEnd, ArrayEnd, ArrayEnd},
}, {
	name: "Everything",
	in: ` {
	"literals" : [ null , false , true ],
	"string" : "Hello, 世界" ,
	"number" : 3.14159 ,
	"arrayN0" : [ ] ,
	"arrayN1" : [ 0 ] ,
	"arrayN2" : [ 0 , 1 ] ,
	"objectN0" : { } ,
	"objectN1" : { "0" : 0 } ,
	"objectN2" : { "0" : 0 , "1" : 1 }
} `,
	outCompact: `{"literals":[null,false,true],"string":"Hello, 世界","number":3.14159,"arrayN0":[],"arrayN1":[0],"arrayN2":[0,1],"objectN0":{},"objectN1":{"0":0},"objectN2":{"0":0,"1":1}}`,
	outEscaped: `{"\u006c\u0069\u0074\u0065\u0072\u0061\u006c\u0073":[null,false,true],"\u0073\u0074\u0072\u0069\u006e\u0067":"\u0048\u0065\u006c\u006c\u006f\u002c\u0020\u4e16\u754c","\u006e\u0075\u006d\u0062\u0065\u0072":3.14159,"\u0061\u0072\u0072\u0061\u0079\u004e\u0030":[],"\u0061\u0072\u0072\u0061\u0079\u004e\u0031":[0],"\u0061\u0072\u0072\u0061\u0079\u004e\u0032":[0,1],"\u006f\u0062\u006a\u0065\u0063\u0074\u004e\u0030":{},"\u006f\u0062\u006a\u0065\u0063\u0074\u004e\u0031":{"\u0030":0},"\u006f\u0062\u006a\u0065\u0063\u0074\u004e\u0032":{"\u0030":0,"\u0031":1}}`,
	outIndented: `{
>_"literals": [
>__null,
>__false,
>__true
>_],
>_"string": "Hello, 世界",
>_"number": 3.14159,
>_"arrayN0": [],
>_"arrayN1": [
>__0
>_],
>_"arrayN2": [
>__0,
>__1
>_],
>_"objectN0": {},
>_"objectN1": {
>__"0": 0
>_},
>_"objectN2": {
>__"0": 0,
>__"1": 1
>_}
>}`,
	tokens: []Token{
		ObjectStart,
		String("literals"), ArrayStart, Null, False, True, ArrayEnd,
		String("string"), String("Hello, 世界"),
		String("number"), Float(3.14159),
		String("arrayN0"), ArrayStart, ArrayEnd,
		String("arrayN1"), ArrayStart, Uint(0), ArrayEnd,
		String("arrayN2"), ArrayStart, Uint(0), Uint(1), ArrayEnd,
		String("objectN0"), ObjectStart, ObjectEnd,
		String("objectN1"), ObjectStart, String("0"), Uint(0), ObjectEnd,
		String("objectN2"), ObjectStart, String("0"), Uint(0), String("1"), Uint(1), ObjectEnd,
		ObjectEnd,
	},
}}

// TestCoderInterleaved tests that we can interleave calls that operate on
// tokens and raw values. The only error condition is trying to operate on a
// raw value when the next token is an end of object or array.
func TestCoderInterleaved(t *testing.T) {
	for _, td := range coderTestdata {
		// In TokenFirst and ValueFirst, alternate between tokens and values.
		// In TokenDelims, only use tokens for object and array delimiters.
		for _, modeName := range []string{"TokenFirst", "ValueFirst", "TokenDelims"} {
			t.Run(path.Join(td.name, modeName), func(t *testing.T) {
				testCoderInterleaved(t, modeName, td)
			})
		}
	}
}
func testCoderInterleaved(t *testing.T, modeName string, td coderTestdataEntry) {
	src := strings.NewReader(td.in)
	dst := new(bytes.Buffer)
	dec := NewDecoder(src)
	enc := NewEncoder(dst)
	tickTock := modeName == "TokenFirst"
	for {
		if modeName == "TokenDelims" {
			switch dec.PeekKind() {
			case '{', '}', '[', ']':
				tickTock = true // as token
			default:
				tickTock = false // as value
			}
		}
		if tickTock {
			tok, err := dec.ReadToken()
			if err != nil {
				if err == io.EOF {
					break
				}
				t.Fatalf("Decoder.ReadToken error: %v", err)
			}
			if err := enc.WriteToken(tok); err != nil {
				t.Fatalf("Encoder.WriteToken error: %v", err)
			}
		} else {
			val, err := dec.ReadValue()
			if err != nil {
				// It is a syntax error to call ReadValue
				// at the end of an object or array.
				// Retry as a ReadToken call.
				expectError := dec.PeekKind() == '}' || dec.PeekKind() == ']'
				if expectError {
					if !errors.As(err, new(*SyntaxError)) {
						t.Errorf("Decoder.ReadToken error is %T, want %T", err, new(SyntaxError))
					}
					tickTock = !tickTock
					continue
				}

				if err == io.EOF {
					break
				}
				t.Fatalf("Decoder.ReadValue error: %v", err)
			}
			if err := enc.WriteValue(val); err != nil {
				t.Fatalf("Encoder.WriteValue error: %v", err)
			}
		}
		tickTock = !tickTock
	}

	got := dst.String()
	want := td.outCompact + "\n"
	if got != want {
		t.Errorf("output mismatch:\ngot  %q\nwant %q", got, want)
	}
}

// FaultyBuffer implements io.Reader and io.Writer.
// It may process fewer bytes than the provided buffer
// and may randomly return an error.
type FaultyBuffer struct {
	B []byte

	// MaxBytes is the maximum number of bytes read/written.
	// A random number of bytes within [0, MaxBytes] are processed.
	// A non-positive value is treated as infinity.
	MaxBytes int

	// MayError specifies whether to randomly provide this error.
	// Even if an error is returned, no bytes are dropped.
	MayError error

	// Rand to use for pseudo-random behavior.
	// If nil, it will be initialized with rand.NewSource(0).
	Rand rand.Source
}

func (p *FaultyBuffer) Read(b []byte) (int, error) {
	b = b[:copy(b[:p.mayTruncate(len(b))], p.B)]
	p.B = p.B[len(b):]
	if len(p.B) == 0 && (len(b) == 0 || p.randN(2) == 0) {
		return len(b), io.EOF
	}
	return len(b), p.mayError()
}

func (p *FaultyBuffer) Write(b []byte) (int, error) {
	b2 := b[:p.mayTruncate(len(b))]
	p.B = append(p.B, b2...)
	if len(b2) < len(b) {
		return len(b2), io.ErrShortWrite
	}
	return len(b2), p.mayError()
}

// mayTruncate may return a value between [0, n].
func (p *FaultyBuffer) mayTruncate(n int) int {
	if p.MaxBytes > 0 {
		if n > p.MaxBytes {
			n = p.MaxBytes
		}
		return p.randN(n + 1)
	}
	return n
}

// mayError may return a non-nil error.
func (p *FaultyBuffer) mayError() error {
	if p.MayError != nil && p.randN(2) == 0 {
		return p.MayError
	}
	return nil
}

func (p *FaultyBuffer) randN(n int) int {
	if p.Rand == nil {
		p.Rand = rand.NewSource(0)
	}
	return int(p.Rand.Int63() % int64(n))
}
