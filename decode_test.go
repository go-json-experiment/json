// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import (
	"errors"
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

// TestDecoder whether we can parse JSON with either tokens or raw values.
func TestDecoder(t *testing.T) {
	for _, td := range coderTestdata {
		for _, typeName := range []string{"Token", "Value", "TokenDelims"} {
			t.Run(path.Join(td.name, typeName), func(t *testing.T) {
				testDecoder(t, typeName, td)
			})
		}
	}
}
func testDecoder(t *testing.T, typeName string, td coderTestdataEntry) {
	dec := NewDecoder(strings.NewReader(td.in))
	switch typeName {
	case "Token":
		var tokens []Token
		for {
			tok, err := dec.ReadToken()
			if err != nil {
				if err == io.EOF {
					break
				}
				t.Fatalf("Decoder.ReadToken error: %v", err)
			}
			tokens = append(tokens, tok.Clone())
		}
		if !equalTokens(tokens, td.tokens) {
			t.Fatalf("tokens mismatch:\ngot  %v\nwant %v", tokens, td.tokens)
		}
	case "Value":
		val, err := dec.ReadValue()
		if err != nil {
			t.Fatalf("Decoder.ReadValue error: %v", err)
		}
		got := string(val)
		want := strings.TrimSpace(td.in)
		if got != want {
			t.Fatalf("Decoder.ReadValue = %s, want %s", got, want)
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
					t.Fatalf("Decoder.ReadToken error: %v", err)
				}
				tokens = append(tokens, tok.Clone())
			default:
				val, err := dec.ReadValue()
				if err != nil {
					if err == io.EOF {
						break loop
					}
					t.Fatalf("Decoder.ReadValue error: %v", err)
				}
				tokens = append(tokens, rawToken(string(val)))
			}
		}
		if !equalTokens(tokens, td.tokens) {
			t.Fatalf("tokens mismatch:\ngot  %v\nwant %v", tokens, td.tokens)
		}
	}
}

// TestFaultyDecoder tests that temporary I/O errors are not fatal.
func TestFaultyDecoder(t *testing.T) {
	for _, td := range coderTestdata {
		for _, typeName := range []string{"Token", "Value"} {
			t.Run(path.Join(td.name, typeName), func(t *testing.T) {
				testFaultyDecoder(t, typeName, td)
			})
		}
	}
}
func testFaultyDecoder(t *testing.T, typeName string, td coderTestdataEntry) {
	b := &FaultyBuffer{
		B:        []byte(td.in),
		MaxBytes: 1,
		MayError: io.ErrNoProgress,
	}

	// Read all the tokens.
	// If the underlying io.Reader is faulty, then Read may return
	// an error without changing the internal state machine.
	// In other words, I/O errors occur before syntax errors.
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
					t.Fatalf("%d: Decoder.ReadToken error: %v", len(tokens), err)
				}
				continue
			}
			tokens = append(tokens, tok.Clone())
		}
		if !equalTokens(tokens, td.tokens) {
			t.Fatalf("tokens mismatch:\ngot  %s\nwant %s", tokens, td.tokens)
		}
	case "Value":
		for {
			val, err := dec.ReadValue()
			if err != nil {
				if err == io.EOF {
					break
				}
				if !errors.Is(err, io.ErrNoProgress) {
					t.Fatalf("Decoder.ReadValue error: %v", err)
				}
				continue
			}
			got := string(val)
			want := strings.TrimSpace(td.in)
			if got != want {
				t.Fatalf("Decoder.ReadValue = %s, want %s", got, want)
			}
		}
	}
}

type decoderMethodCall struct {
	wantKind Kind
	wantOut  tokOrVal
	wantErr  error
}

var decoderErrorTestdata = []struct {
	name       string
	opts       DecodeOptions
	in         string
	calls      []decoderMethodCall
	wantOffset int
}{{
	name: "InvalidStart",
	in:   ` #`,
	calls: []decoderMethodCall{
		{'#', zeroToken, newInvalidCharacterError(byte('#'), "at start of token").withOffset(int64(len(" ")))},
		{'#', zeroValue, newInvalidCharacterError(byte('#'), "at start of value").withOffset(int64(len(" ")))},
	},
}, {
	name: "StreamN0",
	in:   ` `,
	calls: []decoderMethodCall{
		{0, zeroToken, io.EOF},
		{0, zeroValue, io.EOF},
	},
}, {
	name: "StreamN1",
	in:   ` null `,
	calls: []decoderMethodCall{
		{'n', Null, nil},
		{0, zeroToken, io.EOF},
		{0, zeroValue, io.EOF},
	},
	wantOffset: len(` null`),
}, {
	name: "StreamN2",
	in:   ` nullnull `,
	calls: []decoderMethodCall{
		{'n', Null, nil},
		{'n', Null, nil},
		{0, zeroToken, io.EOF},
		{0, zeroValue, io.EOF},
	},
	wantOffset: len(` nullnull`),
}, {
	name: "StreamN2/ExtraComma", // stream is whitespace delimited, not comma delimited
	in:   ` null , null `,
	calls: []decoderMethodCall{
		{'n', Null, nil},
		{'n', zeroToken, newInvalidCharacterError(',', `before next token`).withOffset(int64(len(` null `)))},
		{'n', zeroValue, newInvalidCharacterError(',', `before next token`).withOffset(int64(len(` null `)))},
	},
	wantOffset: len(` null`),
}, {
	name: "TruncatedNull",
	in:   `nul`,
	calls: []decoderMethodCall{
		{'n', zeroToken, io.ErrUnexpectedEOF},
		{'n', zeroValue, io.ErrUnexpectedEOF},
	},
}, {
	name: "InvalidNull",
	in:   `nulL`,
	calls: []decoderMethodCall{
		{'n', zeroToken, newInvalidCharacterError('L', `within literal null (expecting 'l')`).withOffset(int64(len(`nul`)))},
		{'n', zeroValue, newInvalidCharacterError('L', `within literal null (expecting 'l')`).withOffset(int64(len(`nul`)))},
	},
}, {
	name: "TruncatedFalse",
	in:   `fals`,
	calls: []decoderMethodCall{
		{'f', zeroToken, io.ErrUnexpectedEOF},
		{'f', zeroValue, io.ErrUnexpectedEOF},
	},
}, {
	name: "InvalidFalse",
	in:   `falsE`,
	calls: []decoderMethodCall{
		{'f', zeroToken, newInvalidCharacterError('E', `within literal false (expecting 'e')`).withOffset(int64(len(`fals`)))},
		{'f', zeroValue, newInvalidCharacterError('E', `within literal false (expecting 'e')`).withOffset(int64(len(`fals`)))},
	},
}, {
	name: "TruncatedTrue",
	in:   `tru`,
	calls: []decoderMethodCall{
		{'t', zeroToken, io.ErrUnexpectedEOF},
		{'t', zeroValue, io.ErrUnexpectedEOF},
	},
}, {
	name: "InvalidTrue",
	in:   `truE`,
	calls: []decoderMethodCall{
		{'t', zeroToken, newInvalidCharacterError('E', `within literal true (expecting 'e')`).withOffset(int64(len(`tru`)))},
		{'t', zeroValue, newInvalidCharacterError('E', `within literal true (expecting 'e')`).withOffset(int64(len(`tru`)))},
	},
}, {
	name: "TruncatedString",
	in:   `"start`,
	calls: []decoderMethodCall{
		{'"', zeroToken, io.ErrUnexpectedEOF},
		{'"', zeroValue, io.ErrUnexpectedEOF},
	},
}, {
	name: "InvalidString",
	in:   `"ok` + "\x00",
	calls: []decoderMethodCall{
		{'"', zeroToken, newInvalidCharacterError('\x00', `within string (expecting non-control character)`).withOffset(int64(len(`"ok`)))},
		{'"', zeroValue, newInvalidCharacterError('\x00', `within string (expecting non-control character)`).withOffset(int64(len(`"ok`)))},
	},
}, {
	name: "ValidString/AllowInvalidUTF8/Token",
	opts: DecodeOptions{AllowInvalidUTF8: true},
	in:   "\"living\xde\xad\xbe\xef\"",
	calls: []decoderMethodCall{
		{'"', rawToken("\"living\xde\xad\xbe\xef\""), nil},
	},
	wantOffset: len("\"living\xde\xad\xbe\xef\""),
}, {
	name: "ValidString/AllowInvalidUTF8/Value",
	opts: DecodeOptions{AllowInvalidUTF8: true},
	in:   "\"living\xde\xad\xbe\xef\"",
	calls: []decoderMethodCall{
		{'"', RawValue("\"living\xde\xad\xbe\xef\""), nil},
	},
	wantOffset: len("\"living\xde\xad\xbe\xef\""),
}, {
	name: "InvalidString/RejectInvalidUTF8",
	opts: DecodeOptions{AllowInvalidUTF8: false},
	in:   "\"living\xde\xad\xbe\xef\"",
	calls: []decoderMethodCall{
		{'"', zeroToken, (&SyntaxError{str: "invalid UTF-8 within string"}).withOffset(int64(len("\"living\xde\xad")))},
		{'"', zeroValue, (&SyntaxError{str: "invalid UTF-8 within string"}).withOffset(int64(len("\"living\xde\xad")))},
	},
}, {
	name: "TruncatedNumber",
	in:   `0.`,
	calls: []decoderMethodCall{
		{'0', zeroToken, io.ErrUnexpectedEOF},
		{'0', zeroValue, io.ErrUnexpectedEOF},
	},
}, {
	name: "InvalidNumber",
	in:   `0.e`,
	calls: []decoderMethodCall{
		{'0', zeroToken, newInvalidCharacterError('e', "within number (expecting digit)").withOffset(int64(len(`0.`)))},
		{'0', zeroValue, newInvalidCharacterError('e', "within number (expecting digit)").withOffset(int64(len(`0.`)))},
	},
}, {
	name: "TruncatedObject/AfterStart",
	in:   `{`,
	calls: []decoderMethodCall{
		{'{', zeroValue, io.ErrUnexpectedEOF},
		{'{', ObjectStart, nil},
		{0, zeroToken, io.ErrUnexpectedEOF},
		{0, zeroValue, io.ErrUnexpectedEOF},
	},
	wantOffset: len(`{`),
}, {
	name: "TruncatedObject/AfterName",
	in:   `{"0"`,
	calls: []decoderMethodCall{
		{'{', zeroValue, io.ErrUnexpectedEOF},
		{'{', ObjectStart, nil},
		{'"', String("0"), nil},
		{0, zeroToken, io.ErrUnexpectedEOF},
		{0, zeroValue, io.ErrUnexpectedEOF},
	},
	wantOffset: len(`{"0"`),
}, {
	name: "TruncatedObject/AfterColon",
	in:   `{"0":`,
	calls: []decoderMethodCall{
		{'{', zeroValue, io.ErrUnexpectedEOF},
		{'{', ObjectStart, nil},
		{'"', String("0"), nil},
		{0, zeroToken, io.ErrUnexpectedEOF},
		{0, zeroValue, io.ErrUnexpectedEOF},
	},
	wantOffset: len(`{"0"`),
}, {
	name: "TruncatedObject/AfterValue",
	in:   `{"0":0`,
	calls: []decoderMethodCall{
		{'{', zeroValue, io.ErrUnexpectedEOF},
		{'{', ObjectStart, nil},
		{'"', String("0"), nil},
		{'0', Uint(0), nil},
		{0, zeroToken, io.ErrUnexpectedEOF},
		{0, zeroValue, io.ErrUnexpectedEOF},
	},
	wantOffset: len(`{"0":0`),
}, {
	name: "TruncatedObject/AfterComma",
	in:   `{"0":0,`,
	calls: []decoderMethodCall{
		{'{', zeroValue, io.ErrUnexpectedEOF},
		{'{', ObjectStart, nil},
		{'"', String("0"), nil},
		{'0', Uint(0), nil},
		{0, zeroToken, io.ErrUnexpectedEOF},
		{0, zeroValue, io.ErrUnexpectedEOF},
	},
	wantOffset: len(`{"0":0`),
}, {
	name: "InvalidObject/MissingColon",
	in:   ` { "fizz" "buzz" } `,
	calls: []decoderMethodCall{
		{'{', zeroValue, newInvalidCharacterError('"', "after object name (expecting ':')").withOffset(int64(len(` { "fizz" `)))},
		{'{', ObjectStart, nil},
		{'"', String("fizz"), nil},
		{'"', zeroToken, errMissingColon.withOffset(int64(len(` { "fizz" `)))},
		{'"', zeroValue, errMissingColon.withOffset(int64(len(` { "fizz" `)))},
	},
	wantOffset: len(` { "fizz"`),
}, {
	name: "InvalidObject/MissingColon/GotComma",
	in:   ` { "fizz" , "buzz" } `,
	calls: []decoderMethodCall{
		{'{', zeroValue, newInvalidCharacterError(',', "after object name (expecting ':')").withOffset(int64(len(` { "fizz" `)))},
		{'{', ObjectStart, nil},
		{'"', String("fizz"), nil},
		{'"', zeroToken, errMissingColon.withOffset(int64(len(` { "fizz" `)))},
		{'"', zeroValue, errMissingColon.withOffset(int64(len(` { "fizz" `)))},
	},
	wantOffset: len(` { "fizz"`),
}, {
	name: "InvalidObject/MissingColon/GotHash",
	in:   ` { "fizz" # "buzz" } `,
	calls: []decoderMethodCall{
		{'{', zeroValue, newInvalidCharacterError('#', "after object name (expecting ':')").withOffset(int64(len(` { "fizz" `)))},
		{'{', ObjectStart, nil},
		{'"', String("fizz"), nil},
		{'#', zeroToken, errMissingColon.withOffset(int64(len(` { "fizz" `)))},
		{'#', zeroValue, errMissingColon.withOffset(int64(len(` { "fizz" `)))},
	},
	wantOffset: len(` { "fizz"`),
}, {
	name: "InvalidObject/MissingComma",
	in:   ` { "fizz" : "buzz" "gazz" } `,
	calls: []decoderMethodCall{
		{'{', zeroValue, newInvalidCharacterError('"', "after object value (expecting ',' or '}')").withOffset(int64(len(` { "fizz" : "buzz" `)))},
		{'{', ObjectStart, nil},
		{'"', String("fizz"), nil},
		{'"', String("buzz"), nil},
		{'"', zeroToken, errMissingComma.withOffset(int64(len(` { "fizz" : "buzz" `)))},
		{'"', zeroValue, errMissingComma.withOffset(int64(len(` { "fizz" : "buzz" `)))},
	},
	wantOffset: len(` { "fizz" : "buzz"`),
}, {
	name: "InvalidObject/MissingComma/GotColon",
	in:   ` { "fizz" : "buzz" : "gazz" } `,
	calls: []decoderMethodCall{
		{'{', zeroValue, newInvalidCharacterError(':', "after object value (expecting ',' or '}')").withOffset(int64(len(` { "fizz" : "buzz" `)))},
		{'{', ObjectStart, nil},
		{'"', String("fizz"), nil},
		{'"', String("buzz"), nil},
		{'"', zeroToken, errMissingComma.withOffset(int64(len(` { "fizz" : "buzz" `)))},
		{'"', zeroValue, errMissingComma.withOffset(int64(len(` { "fizz" : "buzz" `)))},
	},
	wantOffset: len(` { "fizz" : "buzz"`),
}, {
	name: "InvalidObject/MissingComma/GotHash",
	in:   ` { "fizz" : "buzz" # "gazz" } `,
	calls: []decoderMethodCall{
		{'{', zeroValue, newInvalidCharacterError('#', "after object value (expecting ',' or '}')").withOffset(int64(len(` { "fizz" : "buzz" `)))},
		{'{', ObjectStart, nil},
		{'"', String("fizz"), nil},
		{'"', String("buzz"), nil},
		{'#', zeroToken, errMissingComma.withOffset(int64(len(` { "fizz" : "buzz" `)))},
		{'#', zeroValue, errMissingComma.withOffset(int64(len(` { "fizz" : "buzz" `)))},
	},
	wantOffset: len(` { "fizz" : "buzz"`),
}, {
	name: "InvalidObject/ExtraComma/AfterStart",
	in:   ` { , } `,
	calls: []decoderMethodCall{
		{'{', zeroValue, newInvalidCharacterError(',', `at start of string (expecting '"')`).withOffset(int64(len(` { `)))},
		{'{', ObjectStart, nil},
		{'}', zeroToken, newInvalidCharacterError(',', `before next token`).withOffset(int64(len(` { `)))},
		{'}', zeroValue, newInvalidCharacterError(',', `before next token`).withOffset(int64(len(` { `)))},
	},
	wantOffset: len(` {`),
}, {
	name: "InvalidObject/ExtraComma/AfterValue",
	in:   ` { "fizz" : "buzz" , } `,
	calls: []decoderMethodCall{
		{'{', zeroValue, newInvalidCharacterError('}', `at start of string (expecting '"')`).withOffset(int64(len(` { "fizz" : "buzz" , `)))},
		{'{', ObjectStart, nil},
		{'"', String("fizz"), nil},
		{'"', String("buzz"), nil},
		{'}', zeroToken, newInvalidCharacterError(',', `before next token`).withOffset(int64(len(` { "fizz" : "buzz" `)))},
		{'}', zeroValue, newInvalidCharacterError(',', `before next token`).withOffset(int64(len(` { "fizz" : "buzz" `)))},
	},
	wantOffset: len(` { "fizz" : "buzz"`),
}, {
	name: "InvalidObject/InvalidName/GotNull",
	in:   ` { null : null } `,
	calls: []decoderMethodCall{
		{'{', zeroValue, newInvalidCharacterError('n', "at start of string (expecting '\"')").withOffset(int64(len(` { `)))},
		{'{', ObjectStart, nil},
		{'n', zeroToken, errMissingName.withOffset(int64(len(` { `)))},
		{'n', zeroValue, errMissingName.withOffset(int64(len(` { `)))},
	},
	wantOffset: len(` {`),
}, {
	name: "InvalidObject/InvalidName/GotFalse",
	in:   ` { false : false } `,
	calls: []decoderMethodCall{
		{'{', zeroValue, newInvalidCharacterError('f', "at start of string (expecting '\"')").withOffset(int64(len(` { `)))},
		{'{', ObjectStart, nil},
		{'f', zeroToken, errMissingName.withOffset(int64(len(` { `)))},
		{'f', zeroValue, errMissingName.withOffset(int64(len(` { `)))},
	},
	wantOffset: len(` {`),
}, {
	name: "InvalidObject/InvalidName/GotTrue",
	in:   ` { true : true } `,
	calls: []decoderMethodCall{
		{'{', zeroValue, newInvalidCharacterError('t', "at start of string (expecting '\"')").withOffset(int64(len(` { `)))},
		{'{', ObjectStart, nil},
		{'t', zeroToken, errMissingName.withOffset(int64(len(` { `)))},
		{'t', zeroValue, errMissingName.withOffset(int64(len(` { `)))},
	},
	wantOffset: len(` {`),
}, {
	name: "InvalidObject/InvalidName/GotNumber",
	in:   ` { 0 : 0 } `,
	calls: []decoderMethodCall{
		{'{', zeroValue, newInvalidCharacterError('0', "at start of string (expecting '\"')").withOffset(int64(len(` { `)))},
		{'{', ObjectStart, nil},
		{'0', zeroToken, errMissingName.withOffset(int64(len(` { `)))},
		{'0', zeroValue, errMissingName.withOffset(int64(len(` { `)))},
	},
	wantOffset: len(` {`),
}, {
	name: "InvalidObject/InvalidName/GotObject",
	in:   ` { {} : {} } `,
	calls: []decoderMethodCall{
		{'{', zeroValue, newInvalidCharacterError('{', "at start of string (expecting '\"')").withOffset(int64(len(` { `)))},
		{'{', ObjectStart, nil},
		{'{', zeroToken, errMissingName.withOffset(int64(len(` { `)))},
		{'{', zeroValue, errMissingName.withOffset(int64(len(` { `)))},
	},
	wantOffset: len(` {`),
}, {
	name: "InvalidObject/InvalidName/GotArray",
	in:   ` { [] : [] } `,
	calls: []decoderMethodCall{
		{'{', zeroValue, newInvalidCharacterError('[', "at start of string (expecting '\"')").withOffset(int64(len(` { `)))},
		{'{', ObjectStart, nil},
		{'[', zeroToken, errMissingName.withOffset(int64(len(` { `)))},
		{'[', zeroValue, errMissingName.withOffset(int64(len(` { `)))},
	},
	wantOffset: len(` {`),
}, {
	name: "InvalidObject/MismatchingDelim",
	in:   ` { ] `,
	calls: []decoderMethodCall{
		{'{', zeroValue, newInvalidCharacterError(']', "at start of string (expecting '\"')").withOffset(int64(len(` { `)))},
		{'{', ObjectStart, nil},
		{']', zeroToken, errMismatchDelim.withOffset(int64(len(` { `)))},
		{']', zeroValue, newInvalidCharacterError(']', "at start of value").withOffset(int64(len(` { `)))},
	},
	wantOffset: len(` {`),
}, {
	name: "ValidObject/InvalidValue",
	in:   ` { } `,
	calls: []decoderMethodCall{
		{'{', ObjectStart, nil},
		{'}', zeroValue, newInvalidCharacterError('}', "at start of value").withOffset(int64(len(" { ")))},
	},
	wantOffset: len(` {`),
}, {
	name: "ValidObject/DuplicateNames",
	opts: DecodeOptions{RejectDuplicateNames: true},
	in:   `{"0":0,"1":1} `,
	calls: []decoderMethodCall{
		{'{', ObjectStart, nil},
		{'"', String("0"), nil},
		{'0', Uint(0), nil},
		{'"', String("1"), nil},
		{'0', Uint(1), nil},
		{'}', ObjectEnd, nil},
	},
	wantOffset: len(`{"0":0,"1":1}`),
}, {
	name: "InvalidObject/DuplicateNames",
	opts: DecodeOptions{RejectDuplicateNames: true},
	in:   `{"0":0,"1":1,"0":0} `,
	calls: []decoderMethodCall{
		{'{', zeroValue, (&SyntaxError{str: `duplicate name "0" in object`}).withOffset(int64(len(`{"0":0,"1":1,`)))},
		{'{', ObjectStart, nil},
		{'"', String("0"), nil},
		{'0', Uint(0), nil},
		{'"', String("1"), nil},
		{'0', Uint(1), nil},
		{'"', zeroToken, (&SyntaxError{str: `duplicate name "0" in object`}).withOffset(int64(len(`{"0":0,"1":1,`)))},
		{'"', zeroValue, (&SyntaxError{str: `duplicate name "0" in object`}).withOffset(int64(len(`{"0":0,"1":1,`)))},
	},
	wantOffset: len(`{"0":0,"1":1`),
}, {
	name: "TruncatedArray/AfterStart",
	in:   `[`,
	calls: []decoderMethodCall{
		{'[', zeroValue, io.ErrUnexpectedEOF},
		{'[', ArrayStart, nil},
		{0, zeroToken, io.ErrUnexpectedEOF},
		{0, zeroValue, io.ErrUnexpectedEOF},
	},
	wantOffset: len(`[`),
}, {
	name: "TruncatedArray/AfterValue",
	in:   `[0`,
	calls: []decoderMethodCall{
		{'[', zeroValue, io.ErrUnexpectedEOF},
		{'[', ArrayStart, nil},
		{'0', Uint(0), nil},
		{0, zeroToken, io.ErrUnexpectedEOF},
		{0, zeroValue, io.ErrUnexpectedEOF},
	},
	wantOffset: len(`[0`),
}, {
	name: "TruncatedArray/AfterComma",
	in:   `[0,`,
	calls: []decoderMethodCall{
		{'[', zeroValue, io.ErrUnexpectedEOF},
		{'[', ArrayStart, nil},
		{'0', Uint(0), nil},
		{0, zeroToken, io.ErrUnexpectedEOF},
		{0, zeroValue, io.ErrUnexpectedEOF},
	},
	wantOffset: len(`[0`),
}, {
	name: "InvalidArray/MissingComma",
	in:   ` [ "fizz" "buzz" ] `,
	calls: []decoderMethodCall{
		{'[', zeroValue, newInvalidCharacterError('"', "after array value (expecting ',' or ']')").withOffset(int64(len(` [ "fizz" `)))},
		{'[', ArrayStart, nil},
		{'"', String("fizz"), nil},
		{'"', zeroToken, errMissingComma.withOffset(int64(len(` [ "fizz" `)))},
		{'"', zeroValue, errMissingComma.withOffset(int64(len(` [ "fizz" `)))},
	},
	wantOffset: len(` [ "fizz"`),
}, {
	name: "InvalidArray/MismatchingDelim",
	in:   ` [ } `,
	calls: []decoderMethodCall{
		{'[', zeroValue, newInvalidCharacterError('}', "at start of value").withOffset(int64(len(` [ `)))},
		{'[', ArrayStart, nil},
		{'}', zeroToken, errMismatchDelim.withOffset(int64(len(` { `)))},
		{'}', zeroValue, newInvalidCharacterError('}', "at start of value").withOffset(int64(len(` [ `)))},
	},
	wantOffset: len(` [`),
}, {
	name: "ValidArray/InvalidValue",
	in:   ` [ ] `,
	calls: []decoderMethodCall{
		{'[', ArrayStart, nil},
		{']', zeroValue, newInvalidCharacterError(']', "at start of value").withOffset(int64(len(" [ ")))},
	},
	wantOffset: len(` [`),
}}

// TestDecoderErrors test that Decoder errors occur when we expect and
// leaves the Decoder in a consistent state.
func TestDecoderErrors(t *testing.T) {
	for _, td := range decoderErrorTestdata {
		t.Run(path.Join(td.name), func(t *testing.T) {
			testDecoderErrors(t, td.opts, td.in, td.calls, td.wantOffset)
		})
	}
}
func testDecoderErrors(t *testing.T, opts DecodeOptions, in string, calls []decoderMethodCall, wantOffset int) {
	src := strings.NewReader(in)
	dec := opts.NewDecoder(src)
	for i, call := range calls {
		gotKind := dec.PeekKind()
		if gotKind != call.wantKind {
			t.Fatalf("%d: Decoder.PeekKind = %v, want %v", i, gotKind, call.wantKind)
		}

		var gotErr error
		switch wantOut := call.wantOut.(type) {
		case Token:
			var gotOut Token
			gotOut, gotErr = dec.ReadToken()
			if gotOut.String() != wantOut.String() {
				t.Fatalf("%d: Decoder.ReadToken = %v, want %v", i, gotOut, wantOut)
			}
		case RawValue:
			var gotOut RawValue
			gotOut, gotErr = dec.ReadValue()
			if string(gotOut) != string(wantOut) {
				t.Fatalf("%d: Decoder.ReadValue = %s, want %s", i, gotOut, wantOut)
			}
		}
		if !reflect.DeepEqual(gotErr, call.wantErr) {
			t.Fatalf("%d: error mismatch: got %#v, want %#v", i, gotErr, call.wantErr)
		}
	}
	gotOffset := int(dec.InputOffset())
	if gotOffset != wantOffset {
		t.Errorf("Decoder.InputOffset = %v, want %v", gotOffset, wantOffset)
	}
	gotUnread := string(dec.unreadBuffer()) // should be a prefix of wantUnread
	wantUnread := in[wantOffset:]
	if !strings.HasPrefix(wantUnread, gotUnread) {
		t.Errorf("Decoder.UnreadBuffer = %v, want %v", gotUnread, wantUnread)
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
			t.Errorf("Decoder.ReadToken error: %v", err)
		}
		got := tok.String()
		switch tok.Kind() {
		case '"':
			got = `"` + got + `"`
		case '{', '[':
			tok, err := dec.ReadToken()
			if err != nil {
				t.Errorf("Decoder.ReadToken error: %v", err)
			}
			got += tok.String()
		}
		if string(got) != string(want) {
			t.Errorf("ReadTokens = %s, want %s", got, want)
		}

		if err := <-errCh; err != nil {
			t.Errorf("Encoder.WriteValue error: %v", err)
		}
	}

	// Test synchronous ReadValue calls.
	for _, want := range values {
		go func() {
			errCh <- enc.WriteValue(RawValue(want))
		}()

		got, err := dec.ReadValue()
		if err != nil {
			t.Errorf("Decoder.ReadValue error: %v", err)
		}
		if string(got) != string(want) {
			t.Errorf("ReadValue = %s, want %s", got, want)
		}

		if err := <-errCh; err != nil {
			t.Errorf("Encoder.WriteValue error: %v", err)
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
		{"null", "x", 0, newInvalidCharacterError('x', "within literal null (expecting 'n')")},
		{"null", "nuxx", 2, newInvalidCharacterError('x', "within literal null (expecting 'l')")},

		{"false", "", 0, io.ErrUnexpectedEOF},
		{"false", "f", 1, io.ErrUnexpectedEOF},
		{"false", "fa", 2, io.ErrUnexpectedEOF},
		{"false", "fal", 3, io.ErrUnexpectedEOF},
		{"false", "fals", 4, io.ErrUnexpectedEOF},
		{"false", "false", 5, nil},
		{"false", "falsex", 5, nil},
		{"false", "x", 0, newInvalidCharacterError('x', "within literal false (expecting 'f')")},
		{"false", "falsx", 4, newInvalidCharacterError('x', "within literal false (expecting 'e')")},

		{"true", "", 0, io.ErrUnexpectedEOF},
		{"true", "t", 1, io.ErrUnexpectedEOF},
		{"true", "tr", 2, io.ErrUnexpectedEOF},
		{"true", "tru", 3, io.ErrUnexpectedEOF},
		{"true", "true", 4, nil},
		{"true", "truex", 4, nil},
		{"true", "x", 0, newInvalidCharacterError('x', "within literal true (expecting 't')")},
		{"true", "trux", 3, newInvalidCharacterError('x', "within literal true (expecting 'e')")},
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
	tests := []struct {
		in          string
		simple      bool
		want        int
		wantStr     string
		wantErr     error
		wantErrUTF8 error // error if validateUTF8 is specified
	}{
		{``, false, 0, "", io.ErrUnexpectedEOF, nil},
		{`"`, false, 1, "", io.ErrUnexpectedEOF, nil},
		{`""`, true, 2, "", nil, nil},
		{`""x`, true, 2, "", nil, nil},
		{` ""x`, false, 0, "", newInvalidCharacterError(' ', "at start of string (expecting '\"')"), nil},
		{`"hello`, false, 6, "hello", io.ErrUnexpectedEOF, nil},
		{`"hello"`, true, 7, "hello", nil, nil},
		{"\"\x00\"", false, 1, "", newInvalidCharacterError('\x00', "within string (expecting non-control character)"), nil},
		{`"\u0000"`, false, 8, "\x00", nil, nil},
		{"\"\x1f\"", false, 1, "", newInvalidCharacterError('\x1f', "within string (expecting non-control character)"), nil},
		{`"\u001f"`, false, 8, "\x1f", nil, nil},
		{`"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"`, true, 54, "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz", nil, nil},
		{"\" !#$%&'()*+,-./0123456789:;<=>?@[]^_`{|}~\x7f\"", true, 44, " !#$%&'()*+,-./0123456789:;<=>?@[]^_`{|}~\x7f", nil, nil},
		{"\"x\x80\"", false, 4, "x\ufffd", nil, &SyntaxError{str: "invalid UTF-8 within string"}},
		{"\"x\xff\"", false, 4, "x\ufffd", nil, &SyntaxError{str: "invalid UTF-8 within string"}},
		{"\"x\xc0", false, 3, "x\ufffd", io.ErrUnexpectedEOF, io.ErrUnexpectedEOF},
		{"\"x\xc0\x80\"", false, 5, "x\ufffd\ufffd", nil, &SyntaxError{str: "invalid UTF-8 within string"}},
		{"\"x\xe0", false, 3, "x\ufffd", io.ErrUnexpectedEOF, io.ErrUnexpectedEOF},
		{"\"x\xe0\x80", false, 4, "x\ufffd\ufffd", io.ErrUnexpectedEOF, io.ErrUnexpectedEOF},
		{"\"x\xe0\x80\x80\"", false, 6, "x\ufffd\ufffd\ufffd", nil, &SyntaxError{str: "invalid UTF-8 within string"}},
		{"\"x\xf0", false, 3, "x\ufffd", io.ErrUnexpectedEOF, io.ErrUnexpectedEOF},
		{"\"x\xf0\x80", false, 4, "x\ufffd\ufffd", io.ErrUnexpectedEOF, io.ErrUnexpectedEOF},
		{"\"x\xf0\x80\x80", false, 5, "x\ufffd\ufffd\ufffd", io.ErrUnexpectedEOF, io.ErrUnexpectedEOF},
		{"\"x\xf0\x80\x80\x80\"", false, 7, "x\ufffd\ufffd\ufffd\ufffd", nil, &SyntaxError{str: "invalid UTF-8 within string"}},
		{"\"x\xed\xba\xad\"", false, 6, "x\ufffd\ufffd\ufffd", nil, &SyntaxError{str: "invalid UTF-8 within string"}},
		{"\"\u0080\u00f6\u20ac\ud799\ue000\ufb33\ufffd\U0001f602\"", false, 25, "\u0080\u00f6\u20ac\ud799\ue000\ufb33\ufffd\U0001f602", nil, nil},
		{`"x\`, false, 2, "x", io.ErrUnexpectedEOF, nil},
		{`"x\"`, false, 4, "x\"", io.ErrUnexpectedEOF, nil},
		{`"x\x"`, false, 2, "x", &SyntaxError{str: `invalid escape sequence "\\x" within string`}, nil},
		{`"\"\\\/\b\f\n\r\t"`, false, 18, "\"\\/\b\f\n\r\t", nil, nil},
		{`"\u`, false, 1, "", io.ErrUnexpectedEOF, nil},
		{`"\uf`, false, 1, "", io.ErrUnexpectedEOF, nil},
		{`"\uff`, false, 1, "", io.ErrUnexpectedEOF, nil},
		{`"\ufff`, false, 1, "", io.ErrUnexpectedEOF, nil},
		{`"\ufffd`, false, 7, "\ufffd", io.ErrUnexpectedEOF, nil},
		{`"\ufffd"`, false, 8, "\ufffd", nil, nil},
		{`"\uABCD"`, false, 8, "\uabcd", nil, nil},
		{`"\uefX0"`, false, 1, "", &SyntaxError{str: `invalid escape sequence "\\uefX0" within string`}, nil},
		{`"\uDEAD"`, false, 8, "\ufffd", nil, io.ErrUnexpectedEOF},
		{`"\uDEAD______"`, false, 14, "\ufffd______", nil, &SyntaxError{str: "invalid unpaired surrogate half within string"}},
		{`"\uDEAD\uXXXX"`, false, 7, "\ufffd", &SyntaxError{str: `invalid escape sequence "\\uXXXX" within string`}, nil},
		{`"\uDEAD\uBEEF"`, false, 14, "\ufffd\ubeef", nil, &SyntaxError{str: `invalid surrogate pair in string`}},
		{`"\uD800\udead"`, false, 14, "\U000102ad", nil, nil},
		{`"\u0022\u005c\u002f\u0008\u000c\u000a\u000d\u0009"`, false, 50, "\"\\/\b\f\n\r\t", nil, nil},
		{`"\u0080\u00f6\u20ac\ud799\ue000\ufb33\ufffd\ud83d\ude02"`, false, 56, "\u0080\u00f6\u20ac\ud799\ue000\ufb33\ufffd\U0001f602", nil, nil},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			switch got := consumeSimpleString([]byte(tt.in)); {
			case tt.simple && got != tt.want:
				t.Errorf("consumeSimpleString(%q) = %v, want %v", tt.in, got, tt.want)
			case !tt.simple && got != 0:
				t.Errorf("consumeSimpleString(%q) = %v, want %v", tt.in, got, 0)
			}

			got, gotErr := consumeString([]byte(tt.in), false)
			if got != tt.want || !reflect.DeepEqual(gotErr, tt.wantErr) {
				t.Errorf("consumeString(%q, false) = (%v, %v), want (%v, %v)", tt.in, got, gotErr, tt.want, tt.wantErr)
			}
			switch got, gotErr := consumeString([]byte(tt.in), true); {
			case tt.wantErrUTF8 == nil && (got != tt.want || !reflect.DeepEqual(gotErr, tt.wantErr)):
				t.Errorf("consumeString(%q, true) = (%v, %v), want (%v, %v)", tt.in, got, gotErr, tt.want, tt.wantErr)
			case tt.wantErrUTF8 != nil && (got > tt.want || !reflect.DeepEqual(gotErr, tt.wantErrUTF8)):
				t.Errorf("consumeString(%q, true) = (%v, %v), want (%v, %v)", tt.in, got, gotErr, tt.want, tt.wantErrUTF8)
			}

			gotStr, gotOk := unescapeString(nil, []byte(tt.in[:got]))
			wantOk := tt.wantErr == nil
			if string(gotStr) != tt.wantStr || gotOk != wantOk {
				t.Errorf("unescapeString(nil, %q) = (%q, %v), want (%q, %v)", tt.in[:got], gotStr, gotOk, tt.wantStr, wantOk)
			}
			if _, gotOk := unescapeString(nil, []byte(tt.in)); got < len(tt.in) && gotOk {
				t.Errorf("unescapeString(nil, %q) = (_, true), want (_, false)", tt.in)
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
		{`"NaN"`, false, 0, newInvalidCharacterError('"', "within number (expecting digit)")},
		{`"Infinity"`, false, 0, newInvalidCharacterError('"', "within number (expecting digit)")},
		{`"-Infinity"`, false, 0, newInvalidCharacterError('"', "within number (expecting digit)")},
		{".0", false, 0, newInvalidCharacterError('.', "within number (expecting digit)")},
		{"0", true, 1, nil},
		{"-0", false, 2, nil},
		{"+0", false, 0, newInvalidCharacterError('+', "within number (expecting digit)")},
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
		{" 9876543210", true, 0, newInvalidCharacterError(' ', "within number (expecting digit)")},
		{"- 9876543210", false, 1, newInvalidCharacterError(' ', "within number (expecting digit)")},
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
		{"1.f", false, 2, newInvalidCharacterError('f', "within number (expecting digit)")},
		{"-1.f", false, 3, newInvalidCharacterError('f', "within number (expecting digit)")},
		{"1.e", false, 2, newInvalidCharacterError('e', "within number (expecting digit)")},
		{"-1.e", false, 3, newInvalidCharacterError('e', "within number (expecting digit)")},
		{"1e0", false, 3, nil},
		{"-1e0", false, 4, nil},
		{"1E0", false, 3, nil},
		{"-1E0", false, 4, nil},
		{"1Ex", false, 2, newInvalidCharacterError('x', "within number (expecting digit)")},
		{"-1Ex", false, 3, newInvalidCharacterError('x', "within number (expecting digit)")},
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
		{"99999999999999999999999999999999", math.MaxUint64, true},
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
