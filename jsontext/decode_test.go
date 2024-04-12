// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package jsontext

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"path"
	"reflect"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/go-json-experiment/json/internal/jsonflags"
	"github.com/go-json-experiment/json/internal/jsontest"
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
			t.Run(path.Join(td.name.Name, typeName), func(t *testing.T) {
				testDecoder(t, td.name.Where, typeName, td)
			})
		}
	}
}
func testDecoder(t *testing.T, where jsontest.CasePos, typeName string, td coderTestdataEntry) {
	dec := NewDecoder(bytes.NewBufferString(td.in))
	switch typeName {
	case "Token":
		var tokens []Token
		var pointers []Pointer
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
			t.Run(path.Join(td.name.Name, typeName), func(t *testing.T) {
				testFaultyDecoder(t, td.name.Where, typeName, td)
			})
		}
	}
}
func testFaultyDecoder(t *testing.T, where jsontest.CasePos, typeName string, td coderTestdataEntry) {
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
	wantPointer Pointer
}

var decoderErrorTestdata = []struct {
	name       jsontest.CaseName
	opts       []Options
	in         string
	calls      []decoderMethodCall
	wantOffset int
}{{
	name: jsontest.Name("InvalidStart"),
	in:   ` #`,
	calls: []decoderMethodCall{
		{'#', zeroToken, newInvalidCharacterError("#", "at start of token").withOffset(len64(" ")), ""},
		{'#', zeroValue, newInvalidCharacterError("#", "at start of value").withOffset(len64(" ")), ""},
	},
}, {
	name: jsontest.Name("StreamN0"),
	in:   ` `,
	calls: []decoderMethodCall{
		{0, zeroToken, io.EOF, ""},
		{0, zeroValue, io.EOF, ""},
	},
}, {
	name: jsontest.Name("StreamN1"),
	in:   ` null `,
	calls: []decoderMethodCall{
		{'n', Null, nil, ""},
		{0, zeroToken, io.EOF, ""},
		{0, zeroValue, io.EOF, ""},
	},
	wantOffset: len(` null`),
}, {
	name: jsontest.Name("StreamN2"),
	in:   ` nullnull `,
	calls: []decoderMethodCall{
		{'n', Null, nil, ""},
		{'n', Null, nil, ""},
		{0, zeroToken, io.EOF, ""},
		{0, zeroValue, io.EOF, ""},
	},
	wantOffset: len(` nullnull`),
}, {
	name: jsontest.Name("StreamN2/ExtraComma"), // stream is whitespace delimited, not comma delimited
	in:   ` null , null `,
	calls: []decoderMethodCall{
		{'n', Null, nil, ""},
		{0, zeroToken, newInvalidCharacterError(",", `before next token`).withOffset(len64(` null `)), ""},
		{0, zeroValue, newInvalidCharacterError(",", `before next token`).withOffset(len64(` null `)), ""},
	},
	wantOffset: len(` null`),
}, {
	name: jsontest.Name("TruncatedNull"),
	in:   `nul`,
	calls: []decoderMethodCall{
		{'n', zeroToken, io.ErrUnexpectedEOF, ""},
		{'n', zeroValue, io.ErrUnexpectedEOF, ""},
	},
}, {
	name: jsontest.Name("InvalidNull"),
	in:   `nulL`,
	calls: []decoderMethodCall{
		{'n', zeroToken, newInvalidCharacterError("L", `within literal null (expecting 'l')`).withOffset(len64(`nul`)), ""},
		{'n', zeroValue, newInvalidCharacterError("L", `within literal null (expecting 'l')`).withOffset(len64(`nul`)), ""},
	},
}, {
	name: jsontest.Name("TruncatedFalse"),
	in:   `fals`,
	calls: []decoderMethodCall{
		{'f', zeroToken, io.ErrUnexpectedEOF, ""},
		{'f', zeroValue, io.ErrUnexpectedEOF, ""},
	},
}, {
	name: jsontest.Name("InvalidFalse"),
	in:   `falsE`,
	calls: []decoderMethodCall{
		{'f', zeroToken, newInvalidCharacterError("E", `within literal false (expecting 'e')`).withOffset(len64(`fals`)), ""},
		{'f', zeroValue, newInvalidCharacterError("E", `within literal false (expecting 'e')`).withOffset(len64(`fals`)), ""},
	},
}, {
	name: jsontest.Name("TruncatedTrue"),
	in:   `tru`,
	calls: []decoderMethodCall{
		{'t', zeroToken, io.ErrUnexpectedEOF, ""},
		{'t', zeroValue, io.ErrUnexpectedEOF, ""},
	},
}, {
	name: jsontest.Name("InvalidTrue"),
	in:   `truE`,
	calls: []decoderMethodCall{
		{'t', zeroToken, newInvalidCharacterError("E", `within literal true (expecting 'e')`).withOffset(len64(`tru`)), ""},
		{'t', zeroValue, newInvalidCharacterError("E", `within literal true (expecting 'e')`).withOffset(len64(`tru`)), ""},
	},
}, {
	name: jsontest.Name("TruncatedString"),
	in:   `"start`,
	calls: []decoderMethodCall{
		{'"', zeroToken, io.ErrUnexpectedEOF, ""},
		{'"', zeroValue, io.ErrUnexpectedEOF, ""},
	},
}, {
	name: jsontest.Name("InvalidString"),
	in:   `"ok` + "\x00",
	calls: []decoderMethodCall{
		{'"', zeroToken, newInvalidCharacterError("\x00", `within string (expecting non-control character)`).withOffset(len64(`"ok`)), ""},
		{'"', zeroValue, newInvalidCharacterError("\x00", `within string (expecting non-control character)`).withOffset(len64(`"ok`)), ""},
	},
}, {
	name: jsontest.Name("ValidString/AllowInvalidUTF8/Token"),
	opts: []Options{AllowInvalidUTF8(true)},
	in:   "\"living\xde\xad\xbe\xef\"",
	calls: []decoderMethodCall{
		{'"', rawToken("\"living\xde\xad\xbe\xef\""), nil, ""},
	},
	wantOffset: len("\"living\xde\xad\xbe\xef\""),
}, {
	name: jsontest.Name("ValidString/AllowInvalidUTF8/Value"),
	opts: []Options{AllowInvalidUTF8(true)},
	in:   "\"living\xde\xad\xbe\xef\"",
	calls: []decoderMethodCall{
		{'"', Value("\"living\xde\xad\xbe\xef\""), nil, ""},
	},
	wantOffset: len("\"living\xde\xad\xbe\xef\""),
}, {
	name: jsontest.Name("InvalidString/RejectInvalidUTF8"),
	opts: []Options{AllowInvalidUTF8(false)},
	in:   "\"living\xde\xad\xbe\xef\"",
	calls: []decoderMethodCall{
		{'"', zeroToken, errInvalidUTF8.withOffset(len64("\"living\xde\xad")), ""},
		{'"', zeroValue, errInvalidUTF8.withOffset(len64("\"living\xde\xad")), ""},
	},
}, {
	name: jsontest.Name("TruncatedNumber"),
	in:   `0.`,
	calls: []decoderMethodCall{
		{'0', zeroToken, io.ErrUnexpectedEOF, ""},
		{'0', zeroValue, io.ErrUnexpectedEOF, ""},
	},
}, {
	name: jsontest.Name("InvalidNumber"),
	in:   `0.e`,
	calls: []decoderMethodCall{
		{'0', zeroToken, newInvalidCharacterError("e", "within number (expecting digit)").withOffset(len64(`0.`)), ""},
		{'0', zeroValue, newInvalidCharacterError("e", "within number (expecting digit)").withOffset(len64(`0.`)), ""},
	},
}, {
	name: jsontest.Name("TruncatedObject/AfterStart"),
	in:   `{`,
	calls: []decoderMethodCall{
		{'{', zeroValue, io.ErrUnexpectedEOF, ""},
		{'{', ObjectStart, nil, ""},
		{0, zeroToken, io.ErrUnexpectedEOF, ""},
		{0, zeroValue, io.ErrUnexpectedEOF, ""},
	},
	wantOffset: len(`{`),
}, {
	name: jsontest.Name("TruncatedObject/AfterName"),
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
	name: jsontest.Name("TruncatedObject/AfterColon"),
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
	name: jsontest.Name("TruncatedObject/AfterValue"),
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
	name: jsontest.Name("TruncatedObject/AfterComma"),
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
	name: jsontest.Name("InvalidObject/MissingColon"),
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
	name: jsontest.Name("InvalidObject/MissingColon/GotComma"),
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
	name: jsontest.Name("InvalidObject/MissingColon/GotHash"),
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
	name: jsontest.Name("InvalidObject/MissingComma"),
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
	name: jsontest.Name("InvalidObject/MissingComma/GotColon"),
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
	name: jsontest.Name("InvalidObject/MissingComma/GotHash"),
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
	name: jsontest.Name("InvalidObject/ExtraComma/AfterStart"),
	in:   ` { , } `,
	calls: []decoderMethodCall{
		{'{', zeroValue, newInvalidCharacterError(",", `at start of string (expecting '"')`).withOffset(len64(` { `)), ""},
		{'{', ObjectStart, nil, ""},
		{0, zeroToken, newInvalidCharacterError(",", `before next token`).withOffset(len64(` { `)), ""},
		{0, zeroValue, newInvalidCharacterError(",", `before next token`).withOffset(len64(` { `)), ""},
	},
	wantOffset: len(` {`),
}, {
	name: jsontest.Name("InvalidObject/ExtraComma/AfterValue"),
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
	name: jsontest.Name("InvalidObject/InvalidName/GotNull"),
	in:   ` { null : null } `,
	calls: []decoderMethodCall{
		{'{', zeroValue, newInvalidCharacterError("n", "at start of string (expecting '\"')").withOffset(len64(` { `)), ""},
		{'{', ObjectStart, nil, ""},
		{'n', zeroToken, errMissingName.withOffset(len64(` { `)), ""},
		{'n', zeroValue, errMissingName.withOffset(len64(` { `)), ""},
	},
	wantOffset: len(` {`),
}, {
	name: jsontest.Name("InvalidObject/InvalidName/GotFalse"),
	in:   ` { false : false } `,
	calls: []decoderMethodCall{
		{'{', zeroValue, newInvalidCharacterError("f", "at start of string (expecting '\"')").withOffset(len64(` { `)), ""},
		{'{', ObjectStart, nil, ""},
		{'f', zeroToken, errMissingName.withOffset(len64(` { `)), ""},
		{'f', zeroValue, errMissingName.withOffset(len64(` { `)), ""},
	},
	wantOffset: len(` {`),
}, {
	name: jsontest.Name("InvalidObject/InvalidName/GotTrue"),
	in:   ` { true : true } `,
	calls: []decoderMethodCall{
		{'{', zeroValue, newInvalidCharacterError("t", "at start of string (expecting '\"')").withOffset(len64(` { `)), ""},
		{'{', ObjectStart, nil, ""},
		{'t', zeroToken, errMissingName.withOffset(len64(` { `)), ""},
		{'t', zeroValue, errMissingName.withOffset(len64(` { `)), ""},
	},
	wantOffset: len(` {`),
}, {
	name: jsontest.Name("InvalidObject/InvalidName/GotNumber"),
	in:   ` { 0 : 0 } `,
	calls: []decoderMethodCall{
		{'{', zeroValue, newInvalidCharacterError("0", "at start of string (expecting '\"')").withOffset(len64(` { `)), ""},
		{'{', ObjectStart, nil, ""},
		{'0', zeroToken, errMissingName.withOffset(len64(` { `)), ""},
		{'0', zeroValue, errMissingName.withOffset(len64(` { `)), ""},
	},
	wantOffset: len(` {`),
}, {
	name: jsontest.Name("InvalidObject/InvalidName/GotObject"),
	in:   ` { {} : {} } `,
	calls: []decoderMethodCall{
		{'{', zeroValue, newInvalidCharacterError("{", "at start of string (expecting '\"')").withOffset(len64(` { `)), ""},
		{'{', ObjectStart, nil, ""},
		{'{', zeroToken, errMissingName.withOffset(len64(` { `)), ""},
		{'{', zeroValue, errMissingName.withOffset(len64(` { `)), ""},
	},
	wantOffset: len(` {`),
}, {
	name: jsontest.Name("InvalidObject/InvalidName/GotArray"),
	in:   ` { [] : [] } `,
	calls: []decoderMethodCall{
		{'{', zeroValue, newInvalidCharacterError("[", "at start of string (expecting '\"')").withOffset(len64(` { `)), ""},
		{'{', ObjectStart, nil, ""},
		{'[', zeroToken, errMissingName.withOffset(len64(` { `)), ""},
		{'[', zeroValue, errMissingName.withOffset(len64(` { `)), ""},
	},
	wantOffset: len(` {`),
}, {
	name: jsontest.Name("InvalidObject/MismatchingDelim"),
	in:   ` { ] `,
	calls: []decoderMethodCall{
		{'{', zeroValue, newInvalidCharacterError("]", "at start of string (expecting '\"')").withOffset(len64(` { `)), ""},
		{'{', ObjectStart, nil, ""},
		{']', zeroToken, errMismatchDelim.withOffset(len64(` { `)), ""},
		{']', zeroValue, newInvalidCharacterError("]", "at start of value").withOffset(len64(` { `)), ""},
	},
	wantOffset: len(` {`),
}, {
	name: jsontest.Name("ValidObject/InvalidValue"),
	in:   ` { } `,
	calls: []decoderMethodCall{
		{'{', ObjectStart, nil, ""},
		{'}', zeroValue, newInvalidCharacterError("}", "at start of value").withOffset(len64(" { ")), ""},
	},
	wantOffset: len(` {`),
}, {
	name: jsontest.Name("ValidObject/UniqueNames"),
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
	name: jsontest.Name("ValidObject/DuplicateNames"),
	opts: []Options{AllowDuplicateNames(true)},
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
	name: jsontest.Name("InvalidObject/DuplicateNames"),
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
	name: jsontest.Name("TruncatedArray/AfterStart"),
	in:   `[`,
	calls: []decoderMethodCall{
		{'[', zeroValue, io.ErrUnexpectedEOF, ""},
		{'[', ArrayStart, nil, ""},
		{0, zeroToken, io.ErrUnexpectedEOF, ""},
		{0, zeroValue, io.ErrUnexpectedEOF, ""},
	},
	wantOffset: len(`[`),
}, {
	name: jsontest.Name("TruncatedArray/AfterValue"),
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
	name: jsontest.Name("TruncatedArray/AfterComma"),
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
	name: jsontest.Name("InvalidArray/MissingComma"),
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
	name: jsontest.Name("InvalidArray/MismatchingDelim"),
	in:   ` [ } `,
	calls: []decoderMethodCall{
		{'[', zeroValue, newInvalidCharacterError("}", "at start of value").withOffset(len64(` [ `)), ""},
		{'[', ArrayStart, nil, ""},
		{'}', zeroToken, errMismatchDelim.withOffset(len64(` { `)), ""},
		{'}', zeroValue, newInvalidCharacterError("}", "at start of value").withOffset(len64(` [ `)), ""},
	},
	wantOffset: len(` [`),
}, {
	name: jsontest.Name("ValidArray/InvalidValue"),
	in:   ` [ ] `,
	calls: []decoderMethodCall{
		{'[', ArrayStart, nil, ""},
		{']', zeroValue, newInvalidCharacterError("]", "at start of value").withOffset(len64(" [ ")), ""},
	},
	wantOffset: len(` [`),
}, {
	name: jsontest.Name("InvalidDelim/AfterTopLevel"),
	in:   `"",`,
	calls: []decoderMethodCall{
		{'"', String(""), nil, ""},
		{0, zeroToken, newInvalidCharacterError([]byte(","), "before next token").withOffset(len64(`""`)), ""},
		{0, zeroValue, newInvalidCharacterError([]byte(","), "before next token").withOffset(len64(`""`)), ""},
	},
	wantOffset: len(`""`),
}, {
	name: jsontest.Name("InvalidDelim/AfterObjectStart"),
	in:   `{:`,
	calls: []decoderMethodCall{
		{'{', ObjectStart, nil, ""},
		{0, zeroToken, newInvalidCharacterError([]byte(":"), "before next token").withOffset(len64(`{`)), ""},
		{0, zeroValue, newInvalidCharacterError([]byte(":"), "before next token").withOffset(len64(`{`)), ""},
	},
	wantOffset: len(`{`),
}, {
	name: jsontest.Name("InvalidDelim/AfterObjectName"),
	in:   `{"",`,
	calls: []decoderMethodCall{
		{'{', ObjectStart, nil, ""},
		{'"', String(""), nil, ""},
		{0, zeroToken, errMissingColon.withOffset(len64(`{""`)), ""},
		{0, zeroValue, errMissingColon.withOffset(len64(`{""`)), ""},
	},
	wantOffset: len(`{""`),
}, {
	name: jsontest.Name("ValidDelim/AfterObjectName"),
	in:   `{"":`,
	calls: []decoderMethodCall{
		{'{', ObjectStart, nil, ""},
		{'"', String(""), nil, ""},
		{0, zeroToken, io.ErrUnexpectedEOF, ""},
		{0, zeroValue, io.ErrUnexpectedEOF, ""},
	},
	wantOffset: len(`{""`),
}, {
	name: jsontest.Name("InvalidDelim/AfterObjectValue"),
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
	name: jsontest.Name("ValidDelim/AfterObjectValue"),
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
	name: jsontest.Name("InvalidDelim/AfterArrayStart"),
	in:   `[,`,
	calls: []decoderMethodCall{
		{'[', ArrayStart, nil, ""},
		{0, zeroToken, newInvalidCharacterError([]byte(","), "before next token").withOffset(len64(`[`)), ""},
		{0, zeroValue, newInvalidCharacterError([]byte(","), "before next token").withOffset(len64(`[`)), ""},
	},
	wantOffset: len(`[`),
}, {
	name: jsontest.Name("InvalidDelim/AfterArrayValue"),
	in:   `["":`,
	calls: []decoderMethodCall{
		{'[', ArrayStart, nil, ""},
		{'"', String(""), nil, ""},
		{0, zeroToken, errMissingComma.withOffset(len64(`[""`)), ""},
		{0, zeroValue, errMissingComma.withOffset(len64(`[""`)), ""},
	},
	wantOffset: len(`[""`),
}, {
	name: jsontest.Name("ValidDelim/AfterArrayValue"),
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
		t.Run(path.Join(td.name.Name), func(t *testing.T) {
			testDecoderErrors(t, td.name.Where, td.opts, td.in, td.calls, td.wantOffset)
		})
	}
}
func testDecoderErrors(t *testing.T, where jsontest.CasePos, opts []Options, in string, calls []decoderMethodCall, wantOffset int) {
	src := bytes.NewBufferString(in)
	dec := NewDecoder(src, opts...)
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
		case Value:
			var gotOut Value
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
	gotUnread := string(dec.s.unreadBuffer()) // should be a prefix of wantUnread
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

	enc := NewEncoder(w, jsonflags.OmitTopLevelNewline|1)
	dec := NewDecoder(r)

	errCh := make(chan error)

	// Test synchronous ReadToken calls.
	for _, want := range values {
		go func() {
			errCh <- enc.WriteValue(Value(want))
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
			errCh <- enc.WriteValue(Value(want))
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
