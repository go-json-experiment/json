// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package jsontext

import (
	"bytes"
	"errors"
	"io"
	"path"
	"reflect"
	"testing"

	"github.com/go-json-experiment/json/internal/jsonflags"
	"github.com/go-json-experiment/json/internal/jsontest"
	"github.com/go-json-experiment/json/internal/jsonwire"
)

// TestEncoder tests whether we can produce JSON with either tokens or raw values.
func TestEncoder(t *testing.T) {
	for _, td := range coderTestdata {
		for _, formatName := range []string{"Compact", "Indented"} {
			for _, typeName := range []string{"Token", "Value", "TokenDelims"} {
				t.Run(path.Join(td.name.Name, typeName, formatName), func(t *testing.T) {
					testEncoder(t, td.name.Where, formatName, typeName, td)
				})
			}
		}
	}
}
func testEncoder(t *testing.T, where jsontest.CasePos, formatName, typeName string, td coderTestdataEntry) {
	var want string
	var opts []Options
	dst := new(bytes.Buffer)
	opts = append(opts, jsonflags.OmitTopLevelNewline|1)
	want = td.outCompacted
	switch formatName {
	case "Indented":
		opts = append(opts, Multiline(true))
		opts = append(opts, WithIndentPrefix("\t"))
		opts = append(opts, WithIndent("    "))
		if td.outIndented != "" {
			want = td.outIndented
		}
	}
	enc := NewEncoder(dst, opts...)

	switch typeName {
	case "Token":
		var pointers []string
		for _, tok := range td.tokens {
			if err := enc.WriteToken(tok); err != nil {
				t.Fatalf("%s: Encoder.WriteToken error: %v", where, err)
			}
			if td.pointers != nil {
				pointers = append(pointers, enc.StackPointer())
			}
		}
		if !reflect.DeepEqual(pointers, td.pointers) {
			t.Fatalf("%s: pointers mismatch:\ngot  %q\nwant %q", where, pointers, td.pointers)
		}
	case "Value":
		if err := enc.WriteValue(Value(td.in)); err != nil {
			t.Fatalf("%s: Encoder.WriteValue error: %v", where, err)
		}
	case "TokenDelims":
		// Use WriteToken for object/array delimiters, WriteValue otherwise.
		for _, tok := range td.tokens {
			switch tok.Kind() {
			case '{', '}', '[', ']':
				if err := enc.WriteToken(tok); err != nil {
					t.Fatalf("%s: Encoder.WriteToken error: %v", where, err)
				}
			default:
				val := Value(tok.String())
				if tok.Kind() == '"' {
					val, _ = jsonwire.AppendQuote(nil, tok.String(), &jsonflags.Flags{})
				}
				if err := enc.WriteValue(val); err != nil {
					t.Fatalf("%s: Encoder.WriteValue error: %v", where, err)
				}
			}
		}
	}

	got := dst.String()
	if got != want {
		t.Errorf("%s: output mismatch:\ngot  %q\nwant %q", where, got, want)
	}
}

// TestFaultyEncoder tests that temporary I/O errors are not fatal.
func TestFaultyEncoder(t *testing.T) {
	for _, td := range coderTestdata {
		for _, typeName := range []string{"Token", "Value"} {
			t.Run(path.Join(td.name.Name, typeName), func(t *testing.T) {
				testFaultyEncoder(t, td.name.Where, typeName, td)
			})
		}
	}
}
func testFaultyEncoder(t *testing.T, where jsontest.CasePos, typeName string, td coderTestdataEntry) {
	b := &FaultyBuffer{
		MaxBytes: 1,
		MayError: io.ErrShortWrite,
	}

	// Write all the tokens.
	// Even if the underlying io.Writer may be faulty,
	// writing a valid token or value is guaranteed to at least
	// be appended to the internal buffer.
	// In other words, syntactic errors occur before I/O errors.
	enc := NewEncoder(b)
	switch typeName {
	case "Token":
		for i, tok := range td.tokens {
			err := enc.WriteToken(tok)
			if err != nil && !errors.Is(err, io.ErrShortWrite) {
				t.Fatalf("%s: %d: Encoder.WriteToken error: %v", where, i, err)
			}
		}
	case "Value":
		err := enc.WriteValue(Value(td.in))
		if err != nil && !errors.Is(err, io.ErrShortWrite) {
			t.Fatalf("%s: Encoder.WriteValue error: %v", where, err)
		}
	}
	gotOutput := string(append(b.B, enc.s.unflushedBuffer()...))
	wantOutput := td.outCompacted + "\n"
	if gotOutput != wantOutput {
		t.Fatalf("%s: output mismatch:\ngot  %s\nwant %s", where, gotOutput, wantOutput)
	}
}

type encoderMethodCall struct {
	in          tokOrVal
	wantErr     error
	wantPointer string
}

var encoderErrorTestdata = []struct {
	name    jsontest.CaseName
	opts    []Options
	calls   []encoderMethodCall
	wantOut string
}{{
	name: jsontest.Name("InvalidToken"),
	calls: []encoderMethodCall{
		{zeroToken, &SyntacticError{str: "invalid json.Token"}, ""},
	},
}, {
	name: jsontest.Name("InvalidValue"),
	calls: []encoderMethodCall{
		{Value(`#`), newInvalidCharacterError("#", "at start of value"), ""},
	},
}, {
	name: jsontest.Name("InvalidValue/DoubleZero"),
	calls: []encoderMethodCall{
		{Value(`00`), newInvalidCharacterError("0", "after top-level value").withOffset(len64(`0`)), ""},
	},
}, {
	name: jsontest.Name("TruncatedValue"),
	calls: []encoderMethodCall{
		{zeroValue, io.ErrUnexpectedEOF, ""},
	},
}, {
	name: jsontest.Name("TruncatedNull"),
	calls: []encoderMethodCall{
		{Value(`nul`), io.ErrUnexpectedEOF, ""},
	},
}, {
	name: jsontest.Name("InvalidNull"),
	calls: []encoderMethodCall{
		{Value(`nulL`), newInvalidCharacterError("L", "within literal null (expecting 'l')").withOffset(len64(`nul`)), ""},
	},
}, {
	name: jsontest.Name("TruncatedFalse"),
	calls: []encoderMethodCall{
		{Value(`fals`), io.ErrUnexpectedEOF, ""},
	},
}, {
	name: jsontest.Name("InvalidFalse"),
	calls: []encoderMethodCall{
		{Value(`falsE`), newInvalidCharacterError("E", "within literal false (expecting 'e')").withOffset(len64(`fals`)), ""},
	},
}, {
	name: jsontest.Name("TruncatedTrue"),
	calls: []encoderMethodCall{
		{Value(`tru`), io.ErrUnexpectedEOF, ""},
	},
}, {
	name: jsontest.Name("InvalidTrue"),
	calls: []encoderMethodCall{
		{Value(`truE`), newInvalidCharacterError("E", "within literal true (expecting 'e')").withOffset(len64(`tru`)), ""},
	},
}, {
	name: jsontest.Name("TruncatedString"),
	calls: []encoderMethodCall{
		{Value(`"star`), io.ErrUnexpectedEOF, ""},
	},
}, {
	name: jsontest.Name("InvalidString"),
	calls: []encoderMethodCall{
		{Value(`"ok` + "\x00"), newInvalidCharacterError("\x00", `within string (expecting non-control character)`).withOffset(len64(`"ok`)), ""},
	},
}, {
	name: jsontest.Name("ValidString/AllowInvalidUTF8/Token"),
	opts: []Options{AllowInvalidUTF8(true)},
	calls: []encoderMethodCall{
		{String("living\xde\xad\xbe\xef"), nil, ""},
	},
	wantOut: "\"living\xde\xad\ufffd\ufffd\"\n",
}, {
	name: jsontest.Name("ValidString/AllowInvalidUTF8/Value"),
	opts: []Options{AllowInvalidUTF8(true)},
	calls: []encoderMethodCall{
		{Value("\"living\xde\xad\xbe\xef\""), nil, ""},
	},
	wantOut: "\"living\xde\xad\ufffd\ufffd\"\n",
}, {
	name: jsontest.Name("InvalidString/RejectInvalidUTF8"),
	opts: []Options{AllowInvalidUTF8(false)},
	calls: []encoderMethodCall{
		{String("living\xde\xad\xbe\xef"), errInvalidUTF8, ""},
		{Value("\"living\xde\xad\xbe\xef\""), errInvalidUTF8.withOffset(len64("\"living\xde\xad")), ""},
	},
}, {
	name: jsontest.Name("TruncatedNumber"),
	calls: []encoderMethodCall{
		{Value(`0.`), io.ErrUnexpectedEOF, ""},
	},
}, {
	name: jsontest.Name("InvalidNumber"),
	calls: []encoderMethodCall{
		{Value(`0.e`), newInvalidCharacterError("e", "within number (expecting digit)").withOffset(len64(`0.`)), ""},
	},
}, {
	name: jsontest.Name("TruncatedObject/AfterStart"),
	calls: []encoderMethodCall{
		{Value(`{`), io.ErrUnexpectedEOF, ""},
	},
}, {
	name: jsontest.Name("TruncatedObject/AfterName"),
	calls: []encoderMethodCall{
		{Value(`{"0"`), io.ErrUnexpectedEOF, ""},
	},
}, {
	name: jsontest.Name("TruncatedObject/AfterColon"),
	calls: []encoderMethodCall{
		{Value(`{"0":`), io.ErrUnexpectedEOF, ""},
	},
}, {
	name: jsontest.Name("TruncatedObject/AfterValue"),
	calls: []encoderMethodCall{
		{Value(`{"0":0`), io.ErrUnexpectedEOF, ""},
	},
}, {
	name: jsontest.Name("TruncatedObject/AfterComma"),
	calls: []encoderMethodCall{
		{Value(`{"0":0,`), io.ErrUnexpectedEOF, ""},
	},
}, {
	name: jsontest.Name("InvalidObject/MissingColon"),
	calls: []encoderMethodCall{
		{Value(` { "fizz" "buzz" } `), newInvalidCharacterError("\"", "after object name (expecting ':')").withOffset(len64(` { "fizz" `)), ""},
		{Value(` { "fizz" , "buzz" } `), newInvalidCharacterError(",", "after object name (expecting ':')").withOffset(len64(` { "fizz" `)), ""},
	},
}, {
	name: jsontest.Name("InvalidObject/MissingComma"),
	calls: []encoderMethodCall{
		{Value(` { "fizz" : "buzz" "gazz" } `), newInvalidCharacterError("\"", "after object value (expecting ',' or '}')").withOffset(len64(` { "fizz" : "buzz" `)), ""},
		{Value(` { "fizz" : "buzz" : "gazz" } `), newInvalidCharacterError(":", "after object value (expecting ',' or '}')").withOffset(len64(` { "fizz" : "buzz" `)), ""},
	},
}, {
	name: jsontest.Name("InvalidObject/ExtraComma"),
	calls: []encoderMethodCall{
		{Value(` { , } `), newInvalidCharacterError(",", `at start of string (expecting '"')`).withOffset(len64(` { `)), ""},
		{Value(` { "fizz" : "buzz" , } `), newInvalidCharacterError("}", `at start of string (expecting '"')`).withOffset(len64(` { "fizz" : "buzz" , `)), ""},
	},
}, {
	name: jsontest.Name("InvalidObject/InvalidName"),
	calls: []encoderMethodCall{
		{Value(`{ null }`), newInvalidCharacterError("n", `at start of string (expecting '"')`).withOffset(len64(`{ `)), ""},
		{Value(`{ false }`), newInvalidCharacterError("f", `at start of string (expecting '"')`).withOffset(len64(`{ `)), ""},
		{Value(`{ true }`), newInvalidCharacterError("t", `at start of string (expecting '"')`).withOffset(len64(`{ `)), ""},
		{Value(`{ 0 }`), newInvalidCharacterError("0", `at start of string (expecting '"')`).withOffset(len64(`{ `)), ""},
		{Value(`{ {} }`), newInvalidCharacterError("{", `at start of string (expecting '"')`).withOffset(len64(`{ `)), ""},
		{Value(`{ [] }`), newInvalidCharacterError("[", `at start of string (expecting '"')`).withOffset(len64(`{ `)), ""},
		{ObjectStart, nil, ""},
		{Null, errMissingName.withOffset(len64(`{`)), ""},
		{Value(`null`), errMissingName.withOffset(len64(`{`)), ""},
		{False, errMissingName.withOffset(len64(`{`)), ""},
		{Value(`false`), errMissingName.withOffset(len64(`{`)), ""},
		{True, errMissingName.withOffset(len64(`{`)), ""},
		{Value(`true`), errMissingName.withOffset(len64(`{`)), ""},
		{Uint(0), errMissingName.withOffset(len64(`{`)), ""},
		{Value(`0`), errMissingName.withOffset(len64(`{`)), ""},
		{ObjectStart, errMissingName.withOffset(len64(`{`)), ""},
		{Value(`{}`), errMissingName.withOffset(len64(`{`)), ""},
		{ArrayStart, errMissingName.withOffset(len64(`{`)), ""},
		{Value(`[]`), errMissingName.withOffset(len64(`{`)), ""},
		{ObjectEnd, nil, ""},
	},
	wantOut: "{}\n",
}, {
	name: jsontest.Name("InvalidObject/InvalidValue"),
	calls: []encoderMethodCall{
		{Value(`{ "0": x }`), newInvalidCharacterError("x", `at start of value`).withOffset(len64(`{ "0": `)), ""},
	},
}, {
	name: jsontest.Name("InvalidObject/MismatchingDelim"),
	calls: []encoderMethodCall{
		{Value(` { ] `), newInvalidCharacterError("]", `at start of string (expecting '"')`).withOffset(len64(` { `)), ""},
		{Value(` { "0":0 ] `), newInvalidCharacterError("]", `after object value (expecting ',' or '}')`).withOffset(len64(` { "0":0 `)), ""},
		{ObjectStart, nil, ""},
		{ArrayEnd, errMismatchDelim.withOffset(len64(`{`)), ""},
		{Value(`]`), newInvalidCharacterError("]", "at start of value").withOffset(len64(`{`)), ""},
		{ObjectEnd, nil, ""},
	},
	wantOut: "{}\n",
}, {
	name: jsontest.Name("ValidObject/UniqueNames"),
	calls: []encoderMethodCall{
		{ObjectStart, nil, ""},
		{String("0"), nil, ""},
		{Uint(0), nil, ""},
		{String("1"), nil, ""},
		{Uint(1), nil, ""},
		{ObjectEnd, nil, ""},
		{Value(` { "0" : 0 , "1" : 1 } `), nil, ""},
	},
	wantOut: `{"0":0,"1":1}` + "\n" + `{"0":0,"1":1}` + "\n",
}, {
	name: jsontest.Name("ValidObject/DuplicateNames"),
	opts: []Options{AllowDuplicateNames(true)},
	calls: []encoderMethodCall{
		{ObjectStart, nil, ""},
		{String("0"), nil, ""},
		{Uint(0), nil, ""},
		{String("0"), nil, ""},
		{Uint(0), nil, ""},
		{ObjectEnd, nil, ""},
		{Value(` { "0" : 0 , "0" : 0 } `), nil, ""},
	},
	wantOut: `{"0":0,"0":0}` + "\n" + `{"0":0,"0":0}` + "\n",
}, {
	name: jsontest.Name("InvalidObject/DuplicateNames"),
	calls: []encoderMethodCall{
		{ObjectStart, nil, ""},
		{String("0"), nil, ""},
		{ObjectStart, nil, ""},
		{ObjectEnd, nil, ""},
		{String("0"), newDuplicateNameError(`"0"`).withOffset(len64(`{"0":{},`)), "/0"},
		{Value(`"0"`), newDuplicateNameError(`"0"`).withOffset(len64(`{"0":{},`)), "/0"},
		{String("1"), nil, ""},
		{ObjectStart, nil, ""},
		{ObjectEnd, nil, ""},
		{String("0"), newDuplicateNameError(`"0"`).withOffset(len64(`{"0":{},"1":{},`)), "/1"},
		{Value(`"0"`), newDuplicateNameError(`"0"`).withOffset(len64(`{"0":{},"1":{},`)), "/1"},
		{String("1"), newDuplicateNameError(`"1"`).withOffset(len64(`{"0":{},"1":{},`)), "/1"},
		{Value(`"1"`), newDuplicateNameError(`"1"`).withOffset(len64(`{"0":{},"1":{},`)), "/1"},
		{ObjectEnd, nil, ""},
		{Value(` { "0" : 0 , "1" : 1 , "0" : 0 } `), newDuplicateNameError(`"0"`).withOffset(len64(`{"0":{},"1":{}}` + "\n" + ` { "0" : 0 , "1" : 1 , `)), ""},
	},
	wantOut: `{"0":{},"1":{}}` + "\n",
}, {
	name: jsontest.Name("TruncatedArray/AfterStart"),
	calls: []encoderMethodCall{
		{Value(`[`), io.ErrUnexpectedEOF, ""},
	},
}, {
	name: jsontest.Name("TruncatedArray/AfterValue"),
	calls: []encoderMethodCall{
		{Value(`[0`), io.ErrUnexpectedEOF, ""},
	},
}, {
	name: jsontest.Name("TruncatedArray/AfterComma"),
	calls: []encoderMethodCall{
		{Value(`[0,`), io.ErrUnexpectedEOF, ""},
	},
}, {
	name: jsontest.Name("TruncatedArray/MissingComma"),
	calls: []encoderMethodCall{
		{Value(` [ "fizz" "buzz" ] `), newInvalidCharacterError("\"", "after array value (expecting ',' or ']')").withOffset(len64(` [ "fizz" `)), ""},
	},
}, {
	name: jsontest.Name("InvalidArray/MismatchingDelim"),
	calls: []encoderMethodCall{
		{Value(` [ } `), newInvalidCharacterError("}", `at start of value`).withOffset(len64(` [ `)), ""},
		{ArrayStart, nil, ""},
		{ObjectEnd, errMismatchDelim.withOffset(len64(`[`)), ""},
		{Value(`}`), newInvalidCharacterError("}", "at start of value").withOffset(len64(`[`)), ""},
		{ArrayEnd, nil, ""},
	},
	wantOut: "[]\n",
}}

// TestEncoderErrors test that Encoder errors occur when we expect and
// leaves the Encoder in a consistent state.
func TestEncoderErrors(t *testing.T) {
	for _, td := range encoderErrorTestdata {
		t.Run(path.Join(td.name.Name), func(t *testing.T) {
			testEncoderErrors(t, td.name.Where, td.opts, td.calls, td.wantOut)
		})
	}
}
func testEncoderErrors(t *testing.T, where jsontest.CasePos, opts []Options, calls []encoderMethodCall, wantOut string) {
	dst := new(bytes.Buffer)
	enc := NewEncoder(dst, opts...)
	for i, call := range calls {
		var gotErr error
		switch tokVal := call.in.(type) {
		case Token:
			gotErr = enc.WriteToken(tokVal)
		case Value:
			gotErr = enc.WriteValue(tokVal)
		}
		if !reflect.DeepEqual(gotErr, call.wantErr) {
			t.Fatalf("%s: %d: error mismatch:\ngot  %v\nwant %v", where, i, gotErr, call.wantErr)
		}
		if call.wantPointer != "" {
			gotPointer := enc.StackPointer()
			if gotPointer != call.wantPointer {
				t.Fatalf("%s: %d: Encoder.StackPointer = %s, want %s", where, i, gotPointer, call.wantPointer)
			}
		}
	}
	gotOut := dst.String() + string(enc.s.unflushedBuffer())
	if gotOut != wantOut {
		t.Fatalf("%s: output mismatch:\ngot  %q\nwant %q", where, gotOut, wantOut)
	}
	gotOffset := int(enc.OutputOffset())
	wantOffset := len(wantOut)
	if gotOffset != wantOffset {
		t.Fatalf("%s: Encoder.OutputOffset = %v, want %v", where, gotOffset, wantOffset)
	}
}
