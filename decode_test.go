// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"path"
	"reflect"
	"strings"
	"testing"
	"testing/iotest"
)

// equalTokens reports whether to sequences of tokens formats the same way.
func equalTokens(xs, ys []Token) bool {
	if len(xs) != len(ys) {
		return false
	}
	for i := range xs {
		if !(reflect.DeepEqual(xs[i], ys[i]) || xs[i].String() == ys[i].String()) {
			return false
		}
	}
	return true
}

// TestDecoder tests whether we can parse JSON with either tokens or raw values.
func TestDecoder(t *testing.T) {
	for _, td := range coderTestdata {
		for _, typeName := range []string{"Token", "Value", "TokenDelims"} {
			t.Run(path.Join(td.name.name, typeName), func(t *testing.T) {
				testDecoder(t, td.name.where, typeName, td)
			})
		}
	}
}
func testDecoder(t *testing.T, where pc, typeName string, td coderTestdataEntry) {
	dec := NewDecoder(bytes.NewBufferString(td.in))
	switch typeName {
	case "Token":
		var tokens []Token
		var pointers []string
		for {
			tok, err := dec.ReadToken()
			if err != nil {
				if err == io.EOF {
					break
				}
				t.Fatalf("%s: Decoder.ReadToken error: %v", where, err)
			}
			tokens = append(tokens, tok.Clone())
			if td.pointers != nil {
				pointers = append(pointers, dec.StackPointer())
			}
		}
		if !equalTokens(tokens, td.tokens) {
			t.Fatalf("%s: tokens mismatch:\ngot  %v\nwant %v", where, tokens, td.tokens)
		}
		if !reflect.DeepEqual(pointers, td.pointers) {
			t.Fatalf("%s: pointers mismatch:\ngot  %q\nwant %q", where, pointers, td.pointers)
		}
	case "Value":
		val, err := dec.ReadValue()
		if err != nil {
			t.Fatalf("%s: Decoder.ReadValue error: %v", where, err)
		}
		got := string(val)
		want := strings.TrimSpace(td.in)
		if got != want {
			t.Fatalf("%s: Decoder.ReadValue = %s, want %s", where, got, want)
		}
	case "TokenDelims":
		// Use ReadToken for object/array delimiters, ReadValue otherwise.
		var tokens []Token
	loop:
		for {
			switch dec.PeekKind() {
			case '{', '}', '[', ']':
				tok, err := dec.ReadToken()
				if err != nil {
					if err == io.EOF {
						break loop
					}
					t.Fatalf("%s: Decoder.ReadToken error: %v", where, err)
				}
				tokens = append(tokens, tok.Clone())
			default:
				val, err := dec.ReadValue()
				if err != nil {
					if err == io.EOF {
						break loop
					}
					t.Fatalf("%s: Decoder.ReadValue error: %v", where, err)
				}
				tokens = append(tokens, rawToken(string(val)))
			}
		}
		if !equalTokens(tokens, td.tokens) {
			t.Fatalf("%s: tokens mismatch:\ngot  %v\nwant %v", where, tokens, td.tokens)
		}
	}
}

// TestFaultyDecoder tests that temporary I/O errors are not fatal.
func TestFaultyDecoder(t *testing.T) {
	for _, td := range coderTestdata {
		for _, typeName := range []string{"Token", "Value"} {
			t.Run(path.Join(td.name.name, typeName), func(t *testing.T) {
				testFaultyDecoder(t, td.name.where, typeName, td)
			})
		}
	}
}
func testFaultyDecoder(t *testing.T, where pc, typeName string, td coderTestdataEntry) {
	b := &FaultyBuffer{
		B:        []byte(td.in),
		MaxBytes: 1,
		MayError: io.ErrNoProgress,
	}

	// Read all the tokens.
	// If the underlying io.Reader is faulty, then Read may return
	// an error without changing the internal state machine.
	// In other words, I/O errors occur before syntactic errors.
	dec := NewDecoder(b)
	switch typeName {
	case "Token":
		var tokens []Token
		for {
			tok, err := dec.ReadToken()
			if err != nil {
				if err == io.EOF {
					break
				}
				if !errors.Is(err, io.ErrNoProgress) {
					t.Fatalf("%s: %d: Decoder.ReadToken error: %v", where, len(tokens), err)
				}
				continue
			}
			tokens = append(tokens, tok.Clone())
		}
		if !equalTokens(tokens, td.tokens) {
			t.Fatalf("%s: tokens mismatch:\ngot  %s\nwant %s", where, tokens, td.tokens)
		}
	case "Value":
		for {
			val, err := dec.ReadValue()
			if err != nil {
				if err == io.EOF {
					break
				}
				if !errors.Is(err, io.ErrNoProgress) {
					t.Fatalf("%s: Decoder.ReadValue error: %v", where, err)
				}
				continue
			}
			got := string(val)
			want := strings.TrimSpace(td.in)
			if got != want {
				t.Fatalf("%s: Decoder.ReadValue = %s, want %s", where, got, want)
			}
		}
	}
}

type decoderMethodCall struct {
	wantKind    Kind
	wantOut     tokOrVal
	wantErr     error
	wantPointer string
}

var decoderErrorTestdata = []struct {
	name       testName
	opts       DecodeOptions
	in         string
	calls      []decoderMethodCall
	wantOffset int
}{{
	name: name("InvalidStart"),
	in:   ` #`,
	calls: []decoderMethodCall{
		{'#', zeroToken, newInvalidCharacterError("#", "at start of token").withOffset(len64(" ")), ""},
		{'#', zeroValue, newInvalidCharacterError("#", "at start of value").withOffset(len64(" ")), ""},
	},
}, {
	name: name("StreamN0"),
	in:   ` `,
	calls: []decoderMethodCall{
		{0, zeroToken, io.EOF, ""},
		{0, zeroValue, io.EOF, ""},
	},
}, {
	name: name("StreamN1"),
	in:   ` null `,
	calls: []decoderMethodCall{
		{'n', Null, nil, ""},
		{0, zeroToken, io.EOF, ""},
		{0, zeroValue, io.EOF, ""},
	},
	wantOffset: len(` null`),
}, {
	name: name("StreamN2"),
	in:   ` nullnull `,
	calls: []decoderMethodCall{
		{'n', Null, nil, ""},
		{'n', Null, nil, ""},
		{0, zeroToken, io.EOF, ""},
		{0, zeroValue, io.EOF, ""},
	},
	wantOffset: len(` nullnull`),
}, {
	name: name("StreamN2/ExtraComma"), // stream is whitespace delimited, not comma delimited
	in:   ` null , null `,
	calls: []decoderMethodCall{
		{'n', Null, nil, ""},
		{0, zeroToken, newInvalidCharacterError(",", `before next token`).withOffset(len64(` null `)), ""},
		{0, zeroValue, newInvalidCharacterError(",", `before next token`).withOffset(len64(` null `)), ""},
	},
	wantOffset: len(` null`),
}, {
	name: name("TruncatedNull"),
	in:   `nul`,
	calls: []decoderMethodCall{
		{'n', zeroToken, io.ErrUnexpectedEOF, ""},
		{'n', zeroValue, io.ErrUnexpectedEOF, ""},
	},
}, {
	name: name("InvalidNull"),
	in:   `nulL`,
	calls: []decoderMethodCall{
		{'n', zeroToken, newInvalidCharacterError("L", `within literal null (expecting 'l')`).withOffset(len64(`nul`)), ""},
		{'n', zeroValue, newInvalidCharacterError("L", `within literal null (expecting 'l')`).withOffset(len64(`nul`)), ""},
	},
}, {
	name: name("TruncatedFalse"),
	in:   `fals`,
	calls: []decoderMethodCall{
		{'f', zeroToken, io.ErrUnexpectedEOF, ""},
		{'f', zeroValue, io.ErrUnexpectedEOF, ""},
	},
}, {
	name: name("InvalidFalse"),
	in:   `falsE`,
	calls: []decoderMethodCall{
		{'f', zeroToken, newInvalidCharacterError("E", `within literal false (expecting 'e')`).withOffset(len64(`fals`)), ""},
		{'f', zeroValue, newInvalidCharacterError("E", `within literal false (expecting 'e')`).withOffset(len64(`fals`)), ""},
	},
}, {
	name: name("TruncatedTrue"),
	in:   `tru`,
	calls: []decoderMethodCall{
		{'t', zeroToken, io.ErrUnexpectedEOF, ""},
		{'t', zeroValue, io.ErrUnexpectedEOF, ""},
	},
}, {
	name: name("InvalidTrue"),
	in:   `truE`,
	calls: []decoderMethodCall{
		{'t', zeroToken, newInvalidCharacterError("E", `within literal true (expecting 'e')`).withOffset(len64(`tru`)), ""},
		{'t', zeroValue, newInvalidCharacterError("E", `within literal true (expecting 'e')`).withOffset(len64(`tru`)), ""},
	},
}, {
	name: name("TruncatedString"),
	in:   `"start`,
	calls: []decoderMethodCall{
		{'"', zeroToken, io.ErrUnexpectedEOF, ""},
		{'"', zeroValue, io.ErrUnexpectedEOF, ""},
	},
}, {
	name: name("InvalidString"),
	in:   `"ok` + "\x00",
	calls: []decoderMethodCall{
		{'"', zeroToken, newInvalidCharacterError("\x00", `within string (expecting non-control character)`).withOffset(len64(`"ok`)), ""},
		{'"', zeroValue, newInvalidCharacterError("\x00", `within string (expecting non-control character)`).withOffset(len64(`"ok`)), ""},
	},
}, {
	name: name("ValidString/AllowInvalidUTF8/Token"),
	opts: DecodeOptions{AllowInvalidUTF8: true},
	in:   "\"living\xde\xad\xbe\xef\"",
	calls: []decoderMethodCall{
		{'"', rawToken("\"living\xde\xad\xbe\xef\""), nil, ""},
	},
	wantOffset: len("\"living\xde\xad\xbe\xef\""),
}, {
	name: name("ValidString/AllowInvalidUTF8/Value"),
	opts: DecodeOptions{AllowInvalidUTF8: true},
	in:   "\"living\xde\xad\xbe\xef\"",
	calls: []decoderMethodCall{
		{'"', RawValue("\"living\xde\xad\xbe\xef\""), nil, ""},
	},
	wantOffset: len("\"living\xde\xad\xbe\xef\""),
}, {
	name: name("InvalidString/RejectInvalidUTF8"),
	opts: DecodeOptions{AllowInvalidUTF8: false},
	in:   "\"living\xde\xad\xbe\xef\"",
	calls: []decoderMethodCall{
		{'"', zeroToken, errInvalidUTF8.withOffset(len64("\"living\xde\xad")), ""},
		{'"', zeroValue, errInvalidUTF8.withOffset(len64("\"living\xde\xad")), ""},
	},
}, {
	name: name("TruncatedNumber"),
	in:   `0.`,
	calls: []decoderMethodCall{
		{'0', zeroToken, io.ErrUnexpectedEOF, ""},
		{'0', zeroValue, io.ErrUnexpectedEOF, ""},
	},
}, {
	name: name("InvalidNumber"),
	in:   `0.e`,
	calls: []decoderMethodCall{
		{'0', zeroToken, newInvalidCharacterError("e", "within number (expecting digit)").withOffset(len64(`0.`)), ""},
		{'0', zeroValue, newInvalidCharacterError("e", "within number (expecting digit)").withOffset(len64(`0.`)), ""},
	},
}, {
	name: name("TruncatedObject/AfterStart"),
	in:   `{`,
	calls: []decoderMethodCall{
		{'{', zeroValue, io.ErrUnexpectedEOF, ""},
		{'{', ObjectStart, nil, ""},
		{0, zeroToken, io.ErrUnexpectedEOF, ""},
		{0, zeroValue, io.ErrUnexpectedEOF, ""},
	},
	wantOffset: len(`{`),
}, {
	name: name("TruncatedObject/AfterName"),
	in:   `{"0"`,
	calls: []decoderMethodCall{
		{'{', zeroValue, io.ErrUnexpectedEOF, ""},
		{'{', ObjectStart, nil, ""},
		{'"', String("0"), nil, ""},
		{0, zeroToken, io.ErrUnexpectedEOF, ""},
		{0, zeroValue, io.ErrUnexpectedEOF, ""},
	},
	wantOffset: len(`{"0"`),
}, {
	name: name("TruncatedObject/AfterColon"),
	in:   `{"0":`,
	calls: []decoderMethodCall{
		{'{', zeroValue, io.ErrUnexpectedEOF, ""},
		{'{', ObjectStart, nil, ""},
		{'"', String("0"), nil, ""},
		{0, zeroToken, io.ErrUnexpectedEOF, ""},
		{0, zeroValue, io.ErrUnexpectedEOF, ""},
	},
	wantOffset: len(`{"0"`),
}, {
	name: name("TruncatedObject/AfterValue"),
	in:   `{"0":0`,
	calls: []decoderMethodCall{
		{'{', zeroValue, io.ErrUnexpectedEOF, ""},
		{'{', ObjectStart, nil, ""},
		{'"', String("0"), nil, ""},
		{'0', Uint(0), nil, ""},
		{0, zeroToken, io.ErrUnexpectedEOF, ""},
		{0, zeroValue, io.ErrUnexpectedEOF, ""},
	},
	wantOffset: len(`{"0":0`),
}, {
	name: name("TruncatedObject/AfterComma"),
	in:   `{"0":0,`,
	calls: []decoderMethodCall{
		{'{', zeroValue, io.ErrUnexpectedEOF, ""},
		{'{', ObjectStart, nil, ""},
		{'"', String("0"), nil, ""},
		{'0', Uint(0), nil, ""},
		{0, zeroToken, io.ErrUnexpectedEOF, ""},
		{0, zeroValue, io.ErrUnexpectedEOF, ""},
	},
	wantOffset: len(`{"0":0`),
}, {
	name: name("InvalidObject/MissingColon"),
	in:   ` { "fizz" "buzz" } `,
	calls: []decoderMethodCall{
		{'{', zeroValue, newInvalidCharacterError("\"", "after object name (expecting ':')").withOffset(len64(` { "fizz" `)), ""},
		{'{', ObjectStart, nil, ""},
		{'"', String("fizz"), nil, ""},
		{0, zeroToken, errMissingColon.withOffset(len64(` { "fizz" `)), ""},
		{0, zeroValue, errMissingColon.withOffset(len64(` { "fizz" `)), ""},
	},
	wantOffset: len(` { "fizz"`),
}, {
	name: name("InvalidObject/MissingColon/GotComma"),
	in:   ` { "fizz" , "buzz" } `,
	calls: []decoderMethodCall{
		{'{', zeroValue, newInvalidCharacterError(",", "after object name (expecting ':')").withOffset(len64(` { "fizz" `)), ""},
		{'{', ObjectStart, nil, ""},
		{'"', String("fizz"), nil, ""},
		{0, zeroToken, errMissingColon.withOffset(len64(` { "fizz" `)), ""},
		{0, zeroValue, errMissingColon.withOffset(len64(` { "fizz" `)), ""},
	},
	wantOffset: len(` { "fizz"`),
}, {
	name: name("InvalidObject/MissingColon/GotHash"),
	in:   ` { "fizz" # "buzz" } `,
	calls: []decoderMethodCall{
		{'{', zeroValue, newInvalidCharacterError("#", "after object name (expecting ':')").withOffset(len64(` { "fizz" `)), ""},
		{'{', ObjectStart, nil, ""},
		{'"', String("fizz"), nil, ""},
		{0, zeroToken, errMissingColon.withOffset(len64(` { "fizz" `)), ""},
		{0, zeroValue, errMissingColon.withOffset(len64(` { "fizz" `)), ""},
	},
	wantOffset: len(` { "fizz"`),
}, {
	name: name("InvalidObject/MissingComma"),
	in:   ` { "fizz" : "buzz" "gazz" } `,
	calls: []decoderMethodCall{
		{'{', zeroValue, newInvalidCharacterError("\"", "after object value (expecting ',' or '}')").withOffset(len64(` { "fizz" : "buzz" `)), ""},
		{'{', ObjectStart, nil, ""},
		{'"', String("fizz"), nil, ""},
		{'"', String("buzz"), nil, ""},
		{0, zeroToken, errMissingComma.withOffset(len64(` { "fizz" : "buzz" `)), ""},
		{0, zeroValue, errMissingComma.withOffset(len64(` { "fizz" : "buzz" `)), ""},
	},
	wantOffset: len(` { "fizz" : "buzz"`),
}, {
	name: name("InvalidObject/MissingComma/GotColon"),
	in:   ` { "fizz" : "buzz" : "gazz" } `,
	calls: []decoderMethodCall{
		{'{', zeroValue, newInvalidCharacterError(":", "after object value (expecting ',' or '}')").withOffset(len64(` { "fizz" : "buzz" `)), ""},
		{'{', ObjectStart, nil, ""},
		{'"', String("fizz"), nil, ""},
		{'"', String("buzz"), nil, ""},
		{0, zeroToken, errMissingComma.withOffset(len64(` { "fizz" : "buzz" `)), ""},
		{0, zeroValue, errMissingComma.withOffset(len64(` { "fizz" : "buzz" `)), ""},
	},
	wantOffset: len(` { "fizz" : "buzz"`),
}, {
	name: name("InvalidObject/MissingComma/GotHash"),
	in:   ` { "fizz" : "buzz" # "gazz" } `,
	calls: []decoderMethodCall{
		{'{', zeroValue, newInvalidCharacterError("#", "after object value (expecting ',' or '}')").withOffset(len64(` { "fizz" : "buzz" `)), ""},
		{'{', ObjectStart, nil, ""},
		{'"', String("fizz"), nil, ""},
		{'"', String("buzz"), nil, ""},
		{0, zeroToken, errMissingComma.withOffset(len64(` { "fizz" : "buzz" `)), ""},
		{0, zeroValue, errMissingComma.withOffset(len64(` { "fizz" : "buzz" `)), ""},
	},
	wantOffset: len(` { "fizz" : "buzz"`),
}, {
	name: name("InvalidObject/ExtraComma/AfterStart"),
	in:   ` { , } `,
	calls: []decoderMethodCall{
		{'{', zeroValue, newInvalidCharacterError(",", `at start of string (expecting '"')`).withOffset(len64(` { `)), ""},
		{'{', ObjectStart, nil, ""},
		{0, zeroToken, newInvalidCharacterError(",", `before next token`).withOffset(len64(` { `)), ""},
		{0, zeroValue, newInvalidCharacterError(",", `before next token`).withOffset(len64(` { `)), ""},
	},
	wantOffset: len(` {`),
}, {
	name: name("InvalidObject/ExtraComma/AfterValue"),
	in:   ` { "fizz" : "buzz" , } `,
	calls: []decoderMethodCall{
		{'{', zeroValue, newInvalidCharacterError("}", `at start of string (expecting '"')`).withOffset(len64(` { "fizz" : "buzz" , `)), ""},
		{'{', ObjectStart, nil, ""},
		{'"', String("fizz"), nil, ""},
		{'"', String("buzz"), nil, ""},
		{0, zeroToken, newInvalidCharacterError(",", `before next token`).withOffset(len64(` { "fizz" : "buzz" `)), ""},
		{0, zeroValue, newInvalidCharacterError(",", `before next token`).withOffset(len64(` { "fizz" : "buzz" `)), ""},
	},
	wantOffset: len(` { "fizz" : "buzz"`),
}, {
	name: name("InvalidObject/InvalidName/GotNull"),
	in:   ` { null : null } `,
	calls: []decoderMethodCall{
		{'{', zeroValue, newInvalidCharacterError("n", "at start of string (expecting '\"')").withOffset(len64(` { `)), ""},
		{'{', ObjectStart, nil, ""},
		{'n', zeroToken, errMissingName.withOffset(len64(` { `)), ""},
		{'n', zeroValue, errMissingName.withOffset(len64(` { `)), ""},
	},
	wantOffset: len(` {`),
}, {
	name: name("InvalidObject/InvalidName/GotFalse"),
	in:   ` { false : false } `,
	calls: []decoderMethodCall{
		{'{', zeroValue, newInvalidCharacterError("f", "at start of string (expecting '\"')").withOffset(len64(` { `)), ""},
		{'{', ObjectStart, nil, ""},
		{'f', zeroToken, errMissingName.withOffset(len64(` { `)), ""},
		{'f', zeroValue, errMissingName.withOffset(len64(` { `)), ""},
	},
	wantOffset: len(` {`),
}, {
	name: name("InvalidObject/InvalidName/GotTrue"),
	in:   ` { true : true } `,
	calls: []decoderMethodCall{
		{'{', zeroValue, newInvalidCharacterError("t", "at start of string (expecting '\"')").withOffset(len64(` { `)), ""},
		{'{', ObjectStart, nil, ""},
		{'t', zeroToken, errMissingName.withOffset(len64(` { `)), ""},
		{'t', zeroValue, errMissingName.withOffset(len64(` { `)), ""},
	},
	wantOffset: len(` {`),
}, {
	name: name("InvalidObject/InvalidName/GotNumber"),
	in:   ` { 0 : 0 } `,
	calls: []decoderMethodCall{
		{'{', zeroValue, newInvalidCharacterError("0", "at start of string (expecting '\"')").withOffset(len64(` { `)), ""},
		{'{', ObjectStart, nil, ""},
		{'0', zeroToken, errMissingName.withOffset(len64(` { `)), ""},
		{'0', zeroValue, errMissingName.withOffset(len64(` { `)), ""},
	},
	wantOffset: len(` {`),
}, {
	name: name("InvalidObject/InvalidName/GotObject"),
	in:   ` { {} : {} } `,
	calls: []decoderMethodCall{
		{'{', zeroValue, newInvalidCharacterError("{", "at start of string (expecting '\"')").withOffset(len64(` { `)), ""},
		{'{', ObjectStart, nil, ""},
		{'{', zeroToken, errMissingName.withOffset(len64(` { `)), ""},
		{'{', zeroValue, errMissingName.withOffset(len64(` { `)), ""},
	},
	wantOffset: len(` {`),
}, {
	name: name("InvalidObject/InvalidName/GotArray"),
	in:   ` { [] : [] } `,
	calls: []decoderMethodCall{
		{'{', zeroValue, newInvalidCharacterError("[", "at start of string (expecting '\"')").withOffset(len64(` { `)), ""},
		{'{', ObjectStart, nil, ""},
		{'[', zeroToken, errMissingName.withOffset(len64(` { `)), ""},
		{'[', zeroValue, errMissingName.withOffset(len64(` { `)), ""},
	},
	wantOffset: len(` {`),
}, {
	name: name("InvalidObject/MismatchingDelim"),
	in:   ` { ] `,
	calls: []decoderMethodCall{
		{'{', zeroValue, newInvalidCharacterError("]", "at start of string (expecting '\"')").withOffset(len64(` { `)), ""},
		{'{', ObjectStart, nil, ""},
		{']', zeroToken, errMismatchDelim.withOffset(len64(` { `)), ""},
		{']', zeroValue, newInvalidCharacterError("]", "at start of value").withOffset(len64(` { `)), ""},
	},
	wantOffset: len(` {`),
}, {
	name: name("ValidObject/InvalidValue"),
	in:   ` { } `,
	calls: []decoderMethodCall{
		{'{', ObjectStart, nil, ""},
		{'}', zeroValue, newInvalidCharacterError("}", "at start of value").withOffset(len64(" { ")), ""},
	},
	wantOffset: len(` {`),
}, {
	name: name("ValidObject/UniqueNames"),
	in:   `{"0":0,"1":1} `,
	calls: []decoderMethodCall{
		{'{', ObjectStart, nil, ""},
		{'"', String("0"), nil, ""},
		{'0', Uint(0), nil, ""},
		{'"', String("1"), nil, ""},
		{'0', Uint(1), nil, ""},
		{'}', ObjectEnd, nil, ""},
	},
	wantOffset: len(`{"0":0,"1":1}`),
}, {
	name: name("ValidObject/DuplicateNames"),
	opts: DecodeOptions{AllowDuplicateNames: true},
	in:   `{"0":0,"0":0} `,
	calls: []decoderMethodCall{
		{'{', ObjectStart, nil, ""},
		{'"', String("0"), nil, ""},
		{'0', Uint(0), nil, ""},
		{'"', String("0"), nil, ""},
		{'0', Uint(0), nil, ""},
		{'}', ObjectEnd, nil, ""},
	},
	wantOffset: len(`{"0":0,"0":0}`),
}, {
	name: name("InvalidObject/DuplicateNames"),
	in:   `{"0":{},"1":{},"0":{}} `,
	calls: []decoderMethodCall{
		{'{', zeroValue, newDuplicateNameError(`"0"`).withOffset(len64(`{"0":{},"1":{},`)), ""},
		{'{', ObjectStart, nil, ""},
		{'"', String("0"), nil, ""},
		{'{', ObjectStart, nil, ""},
		{'}', ObjectEnd, nil, ""},
		{'"', String("1"), nil, ""},
		{'{', ObjectStart, nil, ""},
		{'}', ObjectEnd, nil, ""},
		{'"', zeroToken, newDuplicateNameError(`"0"`).withOffset(len64(`{"0":{},"1":{},`)), "/1"},
		{'"', zeroValue, newDuplicateNameError(`"0"`).withOffset(len64(`{"0":{},"1":{},`)), "/1"},
	},
	wantOffset: len(`{"0":{},"1":{}`),
}, {
	name: name("TruncatedArray/AfterStart"),
	in:   `[`,
	calls: []decoderMethodCall{
		{'[', zeroValue, io.ErrUnexpectedEOF, ""},
		{'[', ArrayStart, nil, ""},
		{0, zeroToken, io.ErrUnexpectedEOF, ""},
		{0, zeroValue, io.ErrUnexpectedEOF, ""},
	},
	wantOffset: len(`[`),
}, {
	name: name("TruncatedArray/AfterValue"),
	in:   `[0`,
	calls: []decoderMethodCall{
		{'[', zeroValue, io.ErrUnexpectedEOF, ""},
		{'[', ArrayStart, nil, ""},
		{'0', Uint(0), nil, ""},
		{0, zeroToken, io.ErrUnexpectedEOF, ""},
		{0, zeroValue, io.ErrUnexpectedEOF, ""},
	},
	wantOffset: len(`[0`),
}, {
	name: name("TruncatedArray/AfterComma"),
	in:   `[0,`,
	calls: []decoderMethodCall{
		{'[', zeroValue, io.ErrUnexpectedEOF, ""},
		{'[', ArrayStart, nil, ""},
		{'0', Uint(0), nil, ""},
		{0, zeroToken, io.ErrUnexpectedEOF, ""},
		{0, zeroValue, io.ErrUnexpectedEOF, ""},
	},
	wantOffset: len(`[0`),
}, {
	name: name("InvalidArray/MissingComma"),
	in:   ` [ "fizz" "buzz" ] `,
	calls: []decoderMethodCall{
		{'[', zeroValue, newInvalidCharacterError("\"", "after array value (expecting ',' or ']')").withOffset(len64(` [ "fizz" `)), ""},
		{'[', ArrayStart, nil, ""},
		{'"', String("fizz"), nil, ""},
		{0, zeroToken, errMissingComma.withOffset(len64(` [ "fizz" `)), ""},
		{0, zeroValue, errMissingComma.withOffset(len64(` [ "fizz" `)), ""},
	},
	wantOffset: len(` [ "fizz"`),
}, {
	name: name("InvalidArray/MismatchingDelim"),
	in:   ` [ } `,
	calls: []decoderMethodCall{
		{'[', zeroValue, newInvalidCharacterError("}", "at start of value").withOffset(len64(` [ `)), ""},
		{'[', ArrayStart, nil, ""},
		{'}', zeroToken, errMismatchDelim.withOffset(len64(` { `)), ""},
		{'}', zeroValue, newInvalidCharacterError("}", "at start of value").withOffset(len64(` [ `)), ""},
	},
	wantOffset: len(` [`),
}, {
	name: name("ValidArray/InvalidValue"),
	in:   ` [ ] `,
	calls: []decoderMethodCall{
		{'[', ArrayStart, nil, ""},
		{']', zeroValue, newInvalidCharacterError("]", "at start of value").withOffset(len64(" [ ")), ""},
	},
	wantOffset: len(` [`),
}, {
	name: name("InvalidDelim/AfterTopLevel"),
	in:   `"",`,
	calls: []decoderMethodCall{
		{'"', String(""), nil, ""},
		{0, zeroToken, newInvalidCharacterError([]byte(","), "before next token").withOffset(len64(`""`)), ""},
		{0, zeroValue, newInvalidCharacterError([]byte(","), "before next token").withOffset(len64(`""`)), ""},
	},
	wantOffset: len(`""`),
}, {
	name: name("InvalidDelim/AfterObjectStart"),
	in:   `{:`,
	calls: []decoderMethodCall{
		{'{', ObjectStart, nil, ""},
		{0, zeroToken, newInvalidCharacterError([]byte(":"), "before next token").withOffset(len64(`{`)), ""},
		{0, zeroValue, newInvalidCharacterError([]byte(":"), "before next token").withOffset(len64(`{`)), ""},
	},
	wantOffset: len(`{`),
}, {
	name: name("InvalidDelim/AfterObjectName"),
	in:   `{"",`,
	calls: []decoderMethodCall{
		{'{', ObjectStart, nil, ""},
		{'"', String(""), nil, ""},
		{0, zeroToken, errMissingColon.withOffset(len64(`{""`)), ""},
		{0, zeroValue, errMissingColon.withOffset(len64(`{""`)), ""},
	},
	wantOffset: len(`{""`),
}, {
	name: name("ValidDelim/AfterObjectName"),
	in:   `{"":`,
	calls: []decoderMethodCall{
		{'{', ObjectStart, nil, ""},
		{'"', String(""), nil, ""},
		{0, zeroToken, io.ErrUnexpectedEOF, ""},
		{0, zeroValue, io.ErrUnexpectedEOF, ""},
	},
	wantOffset: len(`{""`),
}, {
	name: name("InvalidDelim/AfterObjectValue"),
	in:   `{"":"":`,
	calls: []decoderMethodCall{
		{'{', ObjectStart, nil, ""},
		{'"', String(""), nil, ""},
		{'"', String(""), nil, ""},
		{0, zeroToken, errMissingComma.withOffset(len64(`{"":""`)), ""},
		{0, zeroValue, errMissingComma.withOffset(len64(`{"":""`)), ""},
	},
	wantOffset: len(`{"":""`),
}, {
	name: name("ValidDelim/AfterObjectValue"),
	in:   `{"":"",`,
	calls: []decoderMethodCall{
		{'{', ObjectStart, nil, ""},
		{'"', String(""), nil, ""},
		{'"', String(""), nil, ""},
		{0, zeroToken, io.ErrUnexpectedEOF, ""},
		{0, zeroValue, io.ErrUnexpectedEOF, ""},
	},
	wantOffset: len(`{"":""`),
}, {
	name: name("InvalidDelim/AfterArrayStart"),
	in:   `[,`,
	calls: []decoderMethodCall{
		{'[', ArrayStart, nil, ""},
		{0, zeroToken, newInvalidCharacterError([]byte(","), "before next token").withOffset(len64(`[`)), ""},
		{0, zeroValue, newInvalidCharacterError([]byte(","), "before next token").withOffset(len64(`[`)), ""},
	},
	wantOffset: len(`[`),
}, {
	name: name("InvalidDelim/AfterArrayValue"),
	in:   `["":`,
	calls: []decoderMethodCall{
		{'[', ArrayStart, nil, ""},
		{'"', String(""), nil, ""},
		{0, zeroToken, errMissingComma.withOffset(len64(`[""`)), ""},
		{0, zeroValue, errMissingComma.withOffset(len64(`[""`)), ""},
	},
	wantOffset: len(`[""`),
}, {
	name: name("ValidDelim/AfterArrayValue"),
	in:   `["",`,
	calls: []decoderMethodCall{
		{'[', ArrayStart, nil, ""},
		{'"', String(""), nil, ""},
		{0, zeroToken, io.ErrUnexpectedEOF, ""},
		{0, zeroValue, io.ErrUnexpectedEOF, ""},
	},
	wantOffset: len(`[""`),
}}

// TestDecoderErrors test that Decoder errors occur when we expect and
// leaves the Decoder in a consistent state.
func TestDecoderErrors(t *testing.T) {
	for _, td := range decoderErrorTestdata {
		t.Run(path.Join(td.name.name), func(t *testing.T) {
			testDecoderErrors(t, td.name.where, td.opts, td.in, td.calls, td.wantOffset)
		})
	}
}
func testDecoderErrors(t *testing.T, where pc, opts DecodeOptions, in string, calls []decoderMethodCall, wantOffset int) {
	src := bytes.NewBufferString(in)
	dec := opts.NewDecoder(src)
	for i, call := range calls {
		gotKind := dec.PeekKind()
		if gotKind != call.wantKind {
			t.Fatalf("%s: %d: Decoder.PeekKind = %v, want %v", where, i, gotKind, call.wantKind)
		}

		var gotErr error
		switch wantOut := call.wantOut.(type) {
		case Token:
			var gotOut Token
			gotOut, gotErr = dec.ReadToken()
			if gotOut.String() != wantOut.String() {
				t.Fatalf("%s: %d: Decoder.ReadToken = %v, want %v", where, i, gotOut, wantOut)
			}
		case RawValue:
			var gotOut RawValue
			gotOut, gotErr = dec.ReadValue()
			if string(gotOut) != string(wantOut) {
				t.Fatalf("%s: %d: Decoder.ReadValue = %s, want %s", where, i, gotOut, wantOut)
			}
		}
		if !reflect.DeepEqual(gotErr, call.wantErr) {
			t.Fatalf("%s: %d: error mismatch:\ngot  %v\nwant %v", where, i, gotErr, call.wantErr)
		}
		if call.wantPointer != "" {
			gotPointer := dec.StackPointer()
			if gotPointer != call.wantPointer {
				t.Fatalf("%s: %d: Decoder.StackPointer = %s, want %s", where, i, gotPointer, call.wantPointer)
			}
		}
	}
	gotOffset := int(dec.InputOffset())
	if gotOffset != wantOffset {
		t.Fatalf("%s: Decoder.InputOffset = %v, want %v", where, gotOffset, wantOffset)
	}
	gotUnread := string(dec.unreadBuffer()) // should be a prefix of wantUnread
	wantUnread := in[wantOffset:]
	if !strings.HasPrefix(wantUnread, gotUnread) {
		t.Fatalf("%s: Decoder.UnreadBuffer = %v, want %v", where, gotUnread, wantUnread)
	}
}

// TestBufferDecoder tests that we detect misuses of bytes.Buffer with Decoder.
func TestBufferDecoder(t *testing.T) {
	bb := bytes.NewBufferString("[null, false, true]")
	dec := NewDecoder(bb)
	var err error
	for {
		if _, err = dec.ReadToken(); err != nil {
			break
		}
		bb.WriteByte(' ') // not allowed to write to the buffer while reading
	}
	want := &ioError{action: "read", err: errBufferWriteAfterNext}
	if !reflect.DeepEqual(err, want) {
		t.Fatalf("error mismatch: got %v, want %v", err, want)
	}
}

var resumableDecoderTestdata = []string{
	`0`,
	`123456789`,
	`0.0`,
	`0.123456789`,
	`0e0`,
	`0e+0`,
	`0e123456789`,
	`0e+123456789`,
	`123456789.123456789e+123456789`,
	`-0`,
	`-123456789`,
	`-0.0`,
	`-0.123456789`,
	`-0e0`,
	`-0e-0`,
	`-0e123456789`,
	`-0e-123456789`,
	`-123456789.123456789e-123456789`,

	`""`,
	`"a"`,
	`"ab"`,
	`"abc"`,
	`"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"`,
	`"\"\\\/\b\f\n\r\t"`,
	`"\u0022\u005c\u002f\u0008\u000c\u000a\u000d\u0009"`,
	`"\ud800\udead"`,
	"\"\u0080\u00f6\u20ac\ud799\ue000\ufb33\ufffd\U0001f602\"",
	`"\u0080\u00f6\u20ac\ud799\ue000\ufb33\ufffd\ud83d\ude02"`,
}

// TestResumableDecoder tests that resume logic for parsing a
// JSON string and number properly works across every possible split point.
func TestResumableDecoder(t *testing.T) {
	for _, want := range resumableDecoderTestdata {
		t.Run("", func(t *testing.T) {
			dec := NewDecoder(iotest.OneByteReader(strings.NewReader(want)))
			got, err := dec.ReadValue()
			if err != nil {
				t.Fatalf("Decoder.ReadValue error: %v", err)
			}
			if string(got) != want {
				t.Fatalf("Decoder.ReadValue = %s, want %s", got, want)
			}
		})
	}
}

// TestBlockingDecoder verifies that JSON values except numbers can be
// synchronously sent and received on a blocking pipe without a deadlock.
// Numbers are the exception since termination cannot be determined until
// either the pipe ends or a non-numeric character is encountered.
func TestBlockingDecoder(t *testing.T) {
	values := []string{"null", "false", "true", `""`, `{}`, `[]`}

	r, w := net.Pipe()
	defer r.Close()
	defer w.Close()

	enc := NewEncoder(w)
	enc.options.omitTopLevelNewline = true
	dec := NewDecoder(r)

	errCh := make(chan error)

	// Test synchronous ReadToken calls.
	for _, want := range values {
		go func() {
			errCh <- enc.WriteValue(RawValue(want))
		}()

		tok, err := dec.ReadToken()
		if err != nil {
			t.Fatalf("Decoder.ReadToken error: %v", err)
		}
		got := tok.String()
		switch tok.Kind() {
		case '"':
			got = `"` + got + `"`
		case '{', '[':
			tok, err := dec.ReadToken()
			if err != nil {
				t.Fatalf("Decoder.ReadToken error: %v", err)
			}
			got += tok.String()
		}
		if got != want {
			t.Fatalf("ReadTokens = %s, want %s", got, want)
		}

		if err := <-errCh; err != nil {
			t.Fatalf("Encoder.WriteValue error: %v", err)
		}
	}

	// Test synchronous ReadValue calls.
	for _, want := range values {
		go func() {
			errCh <- enc.WriteValue(RawValue(want))
		}()

		got, err := dec.ReadValue()
		if err != nil {
			t.Fatalf("Decoder.ReadValue error: %v", err)
		}
		if string(got) != want {
			t.Fatalf("ReadValue = %s, want %s", got, want)
		}

		if err := <-errCh; err != nil {
			t.Fatalf("Encoder.WriteValue error: %v", err)
		}
	}
}

func TestPeekableDecoder(t *testing.T) {
	type operation any // PeekKind | ReadToken | ReadValue | BufferWrite
	type PeekKind struct {
		want Kind
	}
	type ReadToken struct {
		wantKind Kind
		wantErr  error
	}
	type ReadValue struct {
		wantKind Kind
		wantErr  error
	}
	type WriteString struct {
		in string
	}
	ops := []operation{
		PeekKind{0},
		WriteString{"[ "},
		ReadToken{0, io.EOF}, // previous error from PeekKind is cached once
		ReadToken{'[', nil},

		PeekKind{0},
		WriteString{"] "},
		ReadValue{0, io.ErrUnexpectedEOF}, // previous error from PeekKind is cached once
		ReadValue{0, newInvalidCharacterError("]", "at start of value").withOffset(2)},
		ReadToken{']', nil},

		WriteString{"[ "},
		ReadToken{'[', nil},

		WriteString{" null "},
		PeekKind{'n'},
		PeekKind{'n'},
		ReadToken{'n', nil},

		WriteString{", "},
		PeekKind{0},
		WriteString{"fal"},
		PeekKind{'f'},
		ReadValue{0, io.ErrUnexpectedEOF},
		WriteString{"se "},
		ReadValue{'f', nil},

		PeekKind{0},
		WriteString{" , "},
		PeekKind{0},
		WriteString{` "" `},
		ReadValue{0, io.ErrUnexpectedEOF}, // previous error from PeekKind is cached once
		ReadValue{'"', nil},

		WriteString{" , 0"},
		PeekKind{'0'},
		ReadToken{'0', nil},

		WriteString{" , {} , []"},
		PeekKind{'{'},
		ReadValue{'{', nil},
		ReadValue{'[', nil},

		WriteString{"]"},
		ReadToken{']', nil},
	}

	bb := struct{ *bytes.Buffer }{new(bytes.Buffer)}
	d := NewDecoder(bb)
	for i, op := range ops {
		switch op := op.(type) {
		case PeekKind:
			if got := d.PeekKind(); got != op.want {
				t.Fatalf("%d: Decoder.PeekKind() = %v, want %v", i, got, op.want)
			}
		case ReadToken:
			gotTok, gotErr := d.ReadToken()
			gotKind := gotTok.Kind()
			if gotKind != op.wantKind || !reflect.DeepEqual(gotErr, op.wantErr) {
				t.Fatalf("%d: Decoder.ReadToken() = (%v, %v), want (%v, %v)", i, gotKind, gotErr, op.wantKind, op.wantErr)
			}
		case ReadValue:
			gotVal, gotErr := d.ReadValue()
			gotKind := gotVal.Kind()
			if gotKind != op.wantKind || !reflect.DeepEqual(gotErr, op.wantErr) {
				t.Fatalf("%d: Decoder.ReadValue() = (%v, %v), want (%v, %v)", i, gotKind, gotErr, op.wantKind, op.wantErr)
			}
		case WriteString:
			bb.WriteString(op.in)
		default:
			panic(fmt.Sprintf("unknown operation: %T", op))
		}
	}
}

func TestConsumeWhitespace(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{"", 0},
		{"a", 0},
		{" a", 1},
		{" a ", 1},
		{" \n\r\ta", 4},
		{" \n\r\t \n\r\t \n\r\t \n\r\t", 16},
		{"\u00a0", 0}, // non-breaking space is not JSON whitespace
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			if got := consumeWhitespace([]byte(tt.in)); got != tt.want {
				t.Errorf("consumeWhitespace(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestConsumeLiteral(t *testing.T) {
	tests := []struct {
		literal string
		in      string
		want    int
		wantErr error
	}{
		{"null", "", 0, io.ErrUnexpectedEOF},
		{"null", "n", 1, io.ErrUnexpectedEOF},
		{"null", "nu", 2, io.ErrUnexpectedEOF},
		{"null", "nul", 3, io.ErrUnexpectedEOF},
		{"null", "null", 4, nil},
		{"null", "nullx", 4, nil},
		{"null", "x", 0, newInvalidCharacterError("x", "within literal null (expecting 'n')")},
		{"null", "nuxx", 2, newInvalidCharacterError("x", "within literal null (expecting 'l')")},

		{"false", "", 0, io.ErrUnexpectedEOF},
		{"false", "f", 1, io.ErrUnexpectedEOF},
		{"false", "fa", 2, io.ErrUnexpectedEOF},
		{"false", "fal", 3, io.ErrUnexpectedEOF},
		{"false", "fals", 4, io.ErrUnexpectedEOF},
		{"false", "false", 5, nil},
		{"false", "falsex", 5, nil},
		{"false", "x", 0, newInvalidCharacterError("x", "within literal false (expecting 'f')")},
		{"false", "falsx", 4, newInvalidCharacterError("x", "within literal false (expecting 'e')")},

		{"true", "", 0, io.ErrUnexpectedEOF},
		{"true", "t", 1, io.ErrUnexpectedEOF},
		{"true", "tr", 2, io.ErrUnexpectedEOF},
		{"true", "tru", 3, io.ErrUnexpectedEOF},
		{"true", "true", 4, nil},
		{"true", "truex", 4, nil},
		{"true", "x", 0, newInvalidCharacterError("x", "within literal true (expecting 't')")},
		{"true", "trux", 3, newInvalidCharacterError("x", "within literal true (expecting 'e')")},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			var got int
			switch tt.literal {
			case "null":
				got = consumeNull([]byte(tt.in))
			case "false":
				got = consumeFalse([]byte(tt.in))
			case "true":
				got = consumeTrue([]byte(tt.in))
			default:
				t.Errorf("invalid literal: %v", tt.literal)
			}
			switch {
			case tt.wantErr == nil && got != tt.want:
				t.Errorf("consume%v(%q) = %v, want %v", strings.Title(tt.literal), tt.in, got, tt.want)
			case tt.wantErr != nil && got != 0:
				t.Errorf("consume%v(%q) = %v, want %v", strings.Title(tt.literal), tt.in, got, 0)
			}

			got, gotErr := consumeLiteral([]byte(tt.in), tt.literal)
			if got != tt.want || !reflect.DeepEqual(gotErr, tt.wantErr) {
				t.Errorf("consumeLiteral(%q, %q) = (%v, %v), want (%v, %v)", tt.in, tt.literal, got, gotErr, tt.want, tt.wantErr)
			}
		})
	}
}

func TestConsumeString(t *testing.T) {
	var errPrev = errors.New("same as previous error")
	tests := []struct {
		in             string
		simple         bool
		want           int
		wantUTF8       int // consumed bytes if validateUTF8 is specified
		wantFlags      valueFlags
		wantUnquote    string
		wantErr        error
		wantErrUTF8    error // error if validateUTF8 is specified
		wantErrUnquote error
	}{
		{``, false, 0, 0, 0, "", io.ErrUnexpectedEOF, errPrev, errPrev},
		{`"`, false, 1, 1, 0, "", io.ErrUnexpectedEOF, errPrev, errPrev},
		{`""`, true, 2, 2, 0, "", nil, nil, nil},
		{`""x`, true, 2, 2, 0, "", nil, nil, newInvalidCharacterError("x", "after string value")},
		{` ""x`, false, 0, 0, 0, "", newInvalidCharacterError(" ", "at start of string (expecting '\"')"), errPrev, errPrev},
		{`"hello`, false, 6, 6, 0, "hello", io.ErrUnexpectedEOF, errPrev, errPrev},
		{`"hello"`, true, 7, 7, 0, "hello", nil, nil, nil},
		{"\"\x00\"", false, 1, 1, stringNonVerbatim | stringNonCanonical, "", newInvalidCharacterError("\x00", "within string (expecting non-control character)"), errPrev, errPrev},
		{`"\u0000"`, false, 8, 8, stringNonVerbatim, "\x00", nil, nil, nil},
		{"\"\x1f\"", false, 1, 1, stringNonVerbatim | stringNonCanonical, "", newInvalidCharacterError("\x1f", "within string (expecting non-control character)"), errPrev, errPrev},
		{`"\u001f"`, false, 8, 8, stringNonVerbatim, "\x1f", nil, nil, nil},
		{`"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"`, true, 54, 54, 0, "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz", nil, nil, nil},
		{"\" !#$%&'()*+,-./0123456789:;<=>?@[]^_`{|}~\x7f\"", true, 44, 44, 0, " !#$%&'()*+,-./0123456789:;<=>?@[]^_`{|}~\x7f", nil, nil, nil},
		{"\"x\x80\"", false, 4, 2, stringNonVerbatim | stringNonCanonical, "x\ufffd", nil, errInvalidUTF8, errPrev},
		{"\"x\xff\"", false, 4, 2, stringNonVerbatim | stringNonCanonical, "x\ufffd", nil, errInvalidUTF8, errPrev},
		{"\"x\xc0", false, 3, 2, stringNonVerbatim | stringNonCanonical, "x\ufffd", io.ErrUnexpectedEOF, errInvalidUTF8, io.ErrUnexpectedEOF},
		{"\"x\xc0\x80\"", false, 5, 2, stringNonVerbatim | stringNonCanonical, "x\ufffd\ufffd", nil, errInvalidUTF8, errPrev},
		{"\"x\xe0", false, 2, 2, 0, "x", io.ErrUnexpectedEOF, errPrev, errPrev},
		{"\"x\xe0\x80", false, 4, 2, stringNonVerbatim | stringNonCanonical, "x\ufffd\ufffd", io.ErrUnexpectedEOF, errInvalidUTF8, io.ErrUnexpectedEOF},
		{"\"x\xe0\x80\x80\"", false, 6, 2, stringNonVerbatim | stringNonCanonical, "x\ufffd\ufffd\ufffd", nil, errInvalidUTF8, errPrev},
		{"\"x\xf0", false, 2, 2, 0, "x", io.ErrUnexpectedEOF, errPrev, errPrev},
		{"\"x\xf0\x80", false, 4, 2, stringNonVerbatim | stringNonCanonical, "x\ufffd\ufffd", io.ErrUnexpectedEOF, errInvalidUTF8, io.ErrUnexpectedEOF},
		{"\"x\xf0\x80\x80", false, 5, 2, stringNonVerbatim | stringNonCanonical, "x\ufffd\ufffd\ufffd", io.ErrUnexpectedEOF, errInvalidUTF8, io.ErrUnexpectedEOF},
		{"\"x\xf0\x80\x80\x80\"", false, 7, 2, stringNonVerbatim | stringNonCanonical, "x\ufffd\ufffd\ufffd\ufffd", nil, errInvalidUTF8, errPrev},
		{"\"x\xed\xba\xad\"", false, 6, 2, stringNonVerbatim | stringNonCanonical, "x\ufffd\ufffd\ufffd", nil, errInvalidUTF8, errPrev},
		{"\"\u0080\u00f6\u20ac\ud799\ue000\ufb33\ufffd\U0001f602\"", false, 25, 25, 0, "\u0080\u00f6\u20ac\ud799\ue000\ufb33\ufffd\U0001f602", nil, nil, nil},
		{`"¢"`[:2], false, 1, 1, 0, "", io.ErrUnexpectedEOF, errPrev, errPrev},
		{`"¢"`[:3], false, 3, 3, 0, "¢", io.ErrUnexpectedEOF, errPrev, errPrev}, // missing terminating quote
		{`"¢"`[:4], false, 4, 4, 0, "¢", nil, nil, nil},
		{`"€"`[:2], false, 1, 1, 0, "", io.ErrUnexpectedEOF, errPrev, errPrev},
		{`"€"`[:3], false, 1, 1, 0, "", io.ErrUnexpectedEOF, errPrev, errPrev},
		{`"€"`[:4], false, 4, 4, 0, "€", io.ErrUnexpectedEOF, errPrev, errPrev}, // missing terminating quote
		{`"€"`[:5], false, 5, 5, 0, "€", nil, nil, nil},
		{`"𐍈"`[:2], false, 1, 1, 0, "", io.ErrUnexpectedEOF, errPrev, errPrev},
		{`"𐍈"`[:3], false, 1, 1, 0, "", io.ErrUnexpectedEOF, errPrev, errPrev},
		{`"𐍈"`[:4], false, 1, 1, 0, "", io.ErrUnexpectedEOF, errPrev, errPrev},
		{`"𐍈"`[:5], false, 5, 5, 0, "𐍈", io.ErrUnexpectedEOF, errPrev, errPrev}, // missing terminating quote
		{`"𐍈"`[:6], false, 6, 6, 0, "𐍈", nil, nil, nil},
		{`"x\`, false, 2, 2, stringNonVerbatim, "x", io.ErrUnexpectedEOF, errPrev, errPrev},
		{`"x\"`, false, 4, 4, stringNonVerbatim, "x\"", io.ErrUnexpectedEOF, errPrev, errPrev},
		{`"x\x"`, false, 2, 2, stringNonVerbatim | stringNonCanonical, "x", newInvalidEscapeSequenceError(`\x`), errPrev, errPrev},
		{`"\"\\\b\f\n\r\t"`, false, 16, 16, stringNonVerbatim, "\"\\\b\f\n\r\t", nil, nil, nil},
		{`"/"`, true, 3, 3, 0, "/", nil, nil, nil},
		{`"\/"`, false, 4, 4, stringNonVerbatim | stringNonCanonical, "/", nil, nil, nil},
		{`"\u002f"`, false, 8, 8, stringNonVerbatim | stringNonCanonical, "/", nil, nil, nil},
		{`"\u`, false, 1, 1, stringNonVerbatim, "", io.ErrUnexpectedEOF, errPrev, errPrev},
		{`"\uf`, false, 1, 1, stringNonVerbatim, "", io.ErrUnexpectedEOF, errPrev, errPrev},
		{`"\uff`, false, 1, 1, stringNonVerbatim, "", io.ErrUnexpectedEOF, errPrev, errPrev},
		{`"\ufff`, false, 1, 1, stringNonVerbatim, "", io.ErrUnexpectedEOF, errPrev, errPrev},
		{`"\ufffd`, false, 7, 7, stringNonVerbatim | stringNonCanonical, "\ufffd", io.ErrUnexpectedEOF, errPrev, errPrev},
		{`"\ufffd"`, false, 8, 8, stringNonVerbatim | stringNonCanonical, "\ufffd", nil, nil, nil},
		{`"\uABCD"`, false, 8, 8, stringNonVerbatim | stringNonCanonical, "\uabcd", nil, nil, nil},
		{`"\uefX0"`, false, 1, 1, stringNonVerbatim | stringNonCanonical, "", newInvalidEscapeSequenceError(`\uefX0`), errPrev, errPrev},
		{`"\uDEAD`, false, 7, 1, stringNonVerbatim | stringNonCanonical, "\ufffd", io.ErrUnexpectedEOF, errPrev, errPrev},
		{`"\uDEAD"`, false, 8, 1, stringNonVerbatim | stringNonCanonical, "\ufffd", nil, newInvalidEscapeSequenceError(`\uDEAD"`), errPrev},
		{`"\uDEAD______"`, false, 14, 1, stringNonVerbatim | stringNonCanonical, "\ufffd______", nil, newInvalidEscapeSequenceError(`\uDEAD______`), errPrev},
		{`"\uDEAD\uXXXX"`, false, 7, 1, stringNonVerbatim | stringNonCanonical, "\ufffd", newInvalidEscapeSequenceError(`\uXXXX`), newInvalidEscapeSequenceError(`\uDEAD\uXXXX`), newInvalidEscapeSequenceError(`\uXXXX`)},
		{`"\uDEAD\uBEEF"`, false, 14, 1, stringNonVerbatim | stringNonCanonical, "\ufffd\ubeef", nil, newInvalidEscapeSequenceError(`\uDEAD\uBEEF`), errPrev},
		{`"\uD800\udea`, false, 7, 1, stringNonVerbatim | stringNonCanonical, "\ufffd", io.ErrUnexpectedEOF, errPrev, errPrev},
		{`"\uD800\udb`, false, 7, 1, stringNonVerbatim | stringNonCanonical, "\ufffd", io.ErrUnexpectedEOF, newInvalidEscapeSequenceError(`\uD800\udb`), io.ErrUnexpectedEOF},
		{`"\uD800\udead"`, false, 14, 14, stringNonVerbatim | stringNonCanonical, "\U000102ad", nil, nil, nil},
		{`"\u0022\u005c\u002f\u0008\u000c\u000a\u000d\u0009"`, false, 50, 50, stringNonVerbatim | stringNonCanonical, "\"\\/\b\f\n\r\t", nil, nil, nil},
		{`"\u0080\u00f6\u20ac\ud799\ue000\ufb33\ufffd\ud83d\ude02"`, false, 56, 56, stringNonVerbatim | stringNonCanonical, "\u0080\u00f6\u20ac\ud799\ue000\ufb33\ufffd\U0001f602", nil, nil, nil},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			if tt.wantErrUTF8 == errPrev {
				tt.wantErrUTF8 = tt.wantErr
			}
			if tt.wantErrUnquote == errPrev {
				tt.wantErrUnquote = tt.wantErrUTF8
			}

			switch got := consumeSimpleString([]byte(tt.in)); {
			case tt.simple && got != tt.want:
				t.Errorf("consumeSimpleString(%q) = %v, want %v", tt.in, got, tt.want)
			case !tt.simple && got != 0:
				t.Errorf("consumeSimpleString(%q) = %v, want %v", tt.in, got, 0)
			}

			var gotFlags valueFlags
			got, gotErr := consumeString(&gotFlags, []byte(tt.in), false)
			if gotFlags != tt.wantFlags {
				t.Errorf("consumeString(%q, false) flags = %v, want %v", tt.in, gotFlags, tt.wantFlags)
			}
			if got != tt.want || !reflect.DeepEqual(gotErr, tt.wantErr) {
				t.Errorf("consumeString(%q, false) = (%v, %v), want (%v, %v)", tt.in, got, gotErr, tt.want, tt.wantErr)
			}

			got, gotErr = consumeString(&gotFlags, []byte(tt.in), true)
			if got != tt.wantUTF8 || !reflect.DeepEqual(gotErr, tt.wantErrUTF8) {
				t.Errorf("consumeString(%q, false) = (%v, %v), want (%v, %v)", tt.in, got, gotErr, tt.wantUTF8, tt.wantErrUTF8)
			}

			gotUnquote, gotErr := unescapeString(nil, tt.in)
			if string(gotUnquote) != tt.wantUnquote || !reflect.DeepEqual(gotErr, tt.wantErrUnquote) {
				t.Errorf("unescapeString(nil, %q) = (%q, %v), want (%q, %v)", tt.in[:got], gotUnquote, gotErr, tt.wantUnquote, tt.wantErrUnquote)
			}
		})
	}
}

func TestConsumeNumber(t *testing.T) {
	tests := []struct {
		in      string
		simple  bool
		want    int
		wantErr error
	}{
		{"", false, 0, io.ErrUnexpectedEOF},
		{`"NaN"`, false, 0, newInvalidCharacterError("\"", "within number (expecting digit)")},
		{`"Infinity"`, false, 0, newInvalidCharacterError("\"", "within number (expecting digit)")},
		{`"-Infinity"`, false, 0, newInvalidCharacterError("\"", "within number (expecting digit)")},
		{".0", false, 0, newInvalidCharacterError(".", "within number (expecting digit)")},
		{"0", true, 1, nil},
		{"-0", false, 2, nil},
		{"+0", false, 0, newInvalidCharacterError("+", "within number (expecting digit)")},
		{"1", true, 1, nil},
		{"-1", false, 2, nil},
		{"00", true, 1, nil},
		{"-00", false, 2, nil},
		{"01", true, 1, nil},
		{"-01", false, 2, nil},
		{"0i", true, 1, nil},
		{"-0i", false, 2, nil},
		{"0f", true, 1, nil},
		{"-0f", false, 2, nil},
		{"9876543210", true, 10, nil},
		{"-9876543210", false, 11, nil},
		{"9876543210x", true, 10, nil},
		{"-9876543210x", false, 11, nil},
		{" 9876543210", true, 0, newInvalidCharacterError(" ", "within number (expecting digit)")},
		{"- 9876543210", false, 1, newInvalidCharacterError(" ", "within number (expecting digit)")},
		{strings.Repeat("9876543210", 1000), true, 10000, nil},
		{"-" + strings.Repeat("9876543210", 1000), false, 1 + 10000, nil},
		{"0.", false, 1, io.ErrUnexpectedEOF},
		{"-0.", false, 2, io.ErrUnexpectedEOF},
		{"0e", false, 1, io.ErrUnexpectedEOF},
		{"-0e", false, 2, io.ErrUnexpectedEOF},
		{"0E", false, 1, io.ErrUnexpectedEOF},
		{"-0E", false, 2, io.ErrUnexpectedEOF},
		{"0.0", false, 3, nil},
		{"-0.0", false, 4, nil},
		{"0e0", false, 3, nil},
		{"-0e0", false, 4, nil},
		{"0E0", false, 3, nil},
		{"-0E0", false, 4, nil},
		{"0.0123456789", false, 12, nil},
		{"-0.0123456789", false, 13, nil},
		{"1.f", false, 2, newInvalidCharacterError("f", "within number (expecting digit)")},
		{"-1.f", false, 3, newInvalidCharacterError("f", "within number (expecting digit)")},
		{"1.e", false, 2, newInvalidCharacterError("e", "within number (expecting digit)")},
		{"-1.e", false, 3, newInvalidCharacterError("e", "within number (expecting digit)")},
		{"1e0", false, 3, nil},
		{"-1e0", false, 4, nil},
		{"1E0", false, 3, nil},
		{"-1E0", false, 4, nil},
		{"1Ex", false, 2, newInvalidCharacterError("x", "within number (expecting digit)")},
		{"-1Ex", false, 3, newInvalidCharacterError("x", "within number (expecting digit)")},
		{"1e-0", false, 4, nil},
		{"-1e-0", false, 5, nil},
		{"1e+0", false, 4, nil},
		{"-1e+0", false, 5, nil},
		{"1E-0", false, 4, nil},
		{"-1E-0", false, 5, nil},
		{"1E+0", false, 4, nil},
		{"-1E+0", false, 5, nil},
		{"1E+00500", false, 8, nil},
		{"-1E+00500", false, 9, nil},
		{"1E+00500x", false, 8, nil},
		{"-1E+00500x", false, 9, nil},
		{"9876543210.0123456789e+01234589x", false, 31, nil},
		{"-9876543210.0123456789e+01234589x", false, 32, nil},
		{"1_000_000", true, 1, nil},
		{"0x12ef", true, 1, nil},
		{"0x1p-2", true, 1, nil},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			switch got := consumeSimpleNumber([]byte(tt.in)); {
			case tt.simple && got != tt.want:
				t.Errorf("consumeSimpleNumber(%q) = %v, want %v", tt.in, got, tt.want)
			case !tt.simple && got != 0:
				t.Errorf("consumeSimpleNumber(%q) = %v, want %v", tt.in, got, 0)
			}

			got, gotErr := consumeNumber([]byte(tt.in))
			if got != tt.want || !reflect.DeepEqual(gotErr, tt.wantErr) {
				t.Errorf("consumeNumber(%q) = (%v, %v), want (%v, %v)", tt.in, got, gotErr, tt.want, tt.wantErr)
			}
		})
	}
}

func TestParseHexUint16(t *testing.T) {
	tests := []struct {
		in     string
		want   uint16
		wantOk bool
	}{
		{"", 0, false},
		{"a", 0, false},
		{"ab", 0, false},
		{"abc", 0, false},
		{"abcd", 0xabcd, true},
		{"abcde", 0, false},
		{"9eA1", 0x9ea1, true},
		{"gggg", 0, false},
		{"0000", 0x0000, true},
		{"1234", 0x1234, true},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got, gotOk := parseHexUint16([]byte(tt.in))
			if got != tt.want || gotOk != tt.wantOk {
				t.Errorf("parseHexUint16(%q) = (0x%04x, %v), want (0x%04x, %v)", tt.in, got, gotOk, tt.want, tt.wantOk)
			}
		})
	}
}

func TestParseDecUint(t *testing.T) {
	tests := []struct {
		in     string
		want   uint64
		wantOk bool
	}{
		{"", 0, false},
		{"0", 0, true},
		{"1", 1, true},
		{"-1", 0, false},
		{"1f", 0, false},
		{"00", 0, true},
		{"01", 1, true},
		{"10", 10, true},
		{"10.9", 0, false},
		{" 10", 0, false},
		{"10 ", 0, false},
		{"123456789", 123456789, true},
		{"123456789d", 0, false},
		{"18446744073709551614", math.MaxUint64 - 1, true},
		{"18446744073709551615", math.MaxUint64, true},
		{"99999999999999999999999999999999", math.MaxUint64, false},
		{"99999999999999999999999999999999f", 0, false},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got, gotOk := parseDecUint([]byte(tt.in))
			if got != tt.want || gotOk != tt.wantOk {
				t.Errorf("parseDecUint(%q) = (%v, %v), want (%v, %v)", tt.in, got, gotOk, tt.want, tt.wantOk)
			}
		})
	}
}

func TestParseFloat(t *testing.T) {
	tests := []struct {
		in     string
		want32 float64
		want64 float64
		wantOk bool
	}{
		{"0", 0, 0, true},
		{"-1", -1, -1, true},
		{"1", 1, 1, true},

		{"-16777215", -16777215, -16777215, true}, // -(1<<24 - 1)
		{"16777215", 16777215, 16777215, true},    // +(1<<24 - 1)
		{"-16777216", -16777216, -16777216, true}, // -(1<<24)
		{"16777216", 16777216, 16777216, true},    // +(1<<24)
		{"-16777217", -16777216, -16777217, true}, // -(1<<24 + 1)
		{"16777217", 16777216, 16777217, true},    // +(1<<24 + 1)

		{"-9007199254740991", -9007199254740992, -9007199254740991, true}, // -(1<<53 - 1)
		{"9007199254740991", 9007199254740992, 9007199254740991, true},    // +(1<<53 - 1)
		{"-9007199254740992", -9007199254740992, -9007199254740992, true}, // -(1<<53)
		{"9007199254740992", 9007199254740992, 9007199254740992, true},    // +(1<<53)
		{"-9007199254740993", -9007199254740992, -9007199254740992, true}, // -(1<<53 + 1)
		{"9007199254740993", 9007199254740992, 9007199254740992, true},    // +(1<<53 + 1)

		{"-1e1000", -math.MaxFloat32, -math.MaxFloat64, true},
		{"1e1000", +math.MaxFloat32, +math.MaxFloat64, true},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got32, gotOk32 := parseFloat([]byte(tt.in), 32)
			if got32 != tt.want32 || gotOk32 != tt.wantOk {
				t.Errorf("parseFloat(%q, 32) = (%v, %v), want (%v, %v)", tt.in, got32, gotOk32, tt.want32, tt.wantOk)
			}

			got64, gotOk64 := parseFloat([]byte(tt.in), 64)
			if got64 != tt.want64 || gotOk64 != tt.wantOk {
				t.Errorf("parseFloat(%q, 64) = (%v, %v), want (%v, %v)", tt.in, got64, gotOk64, tt.want64, tt.wantOk)
			}
		})
	}
}
