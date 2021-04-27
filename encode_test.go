// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import (
	"bytes"
	"errors"
	"io"
	"math"
	"path"
	"reflect"
	"strings"
	"testing"
	"unicode"
)

// TestEncoder whether we can produce JSON with either tokens or raw values.
func TestEncoder(t *testing.T) {
	for _, td := range coderTestdata {
		for _, formatName := range []string{"Compact", "Escaped", "Indented"} {
			for _, typeName := range []string{"Token", "Value", "TokenDelims"} {
				t.Run(path.Join(td.name, typeName, formatName), func(t *testing.T) {
					testEncoder(t, formatName, typeName, td)
				})
			}
		}
	}
}
func testEncoder(t *testing.T, formatName, typeName string, td coderTestdataEntry) {
	var want string
	dst := new(bytes.Buffer)
	enc := NewEncoder(dst)
	enc.options.omitTopLevelNewline = true
	want = td.outCompacted
	switch formatName {
	case "Escaped":
		enc.options.EscapeRune = func(rune) bool { return true }
		if td.outEscaped != "" {
			want = td.outEscaped
		}
	case "Indented":
		enc.options.multiline = true
		enc.options.IndentPrefix = "\t"
		enc.options.Indent = "    "
		if td.outIndented != "" {
			want = td.outIndented
		}
	}

	switch typeName {
	case "Token":
		for _, tok := range td.tokens {
			if err := enc.WriteToken(tok); err != nil {
				t.Fatalf("Encoder.WriteToken error: %v", err)
			}
		}
	case "Value":
		if err := enc.WriteValue(RawValue(td.in)); err != nil {
			t.Fatalf("Encoder.WriteValue error: %v", err)
		}
	case "TokenDelims":
		// Use WriteToken for object/array delimiters, WriteValue otherwise.
		for _, tok := range td.tokens {
			switch tok.Kind() {
			case '{', '}', '[', ']':
				if err := enc.WriteToken(tok); err != nil {
					t.Fatalf("Encoder.WriteToken error: %v", err)
				}
			default:
				val := RawValue(tok.String())
				if tok.Kind() == '"' {
					val, _ = appendString(nil, tok.String(), false, nil)
				}
				if err := enc.WriteValue(val); err != nil {
					t.Fatalf("Encoder.WriteValue error: %v", err)
				}
			}
		}
	}

	got := dst.String()
	if got != want {
		t.Errorf("output mismatch:\ngot  %q\nwant %q", got, want)
	}
}

// TestFaultyEncoder tests that temporary I/O errors are not fatal.
func TestFaultyEncoder(t *testing.T) {
	for _, td := range coderTestdata {
		for _, typeName := range []string{"Token", "Value"} {
			t.Run(path.Join(td.name, typeName), func(t *testing.T) {
				testFaultyEncoder(t, typeName, td)
			})
		}
	}
}
func testFaultyEncoder(t *testing.T, typeName string, td coderTestdataEntry) {
	b := &FaultyBuffer{
		MaxBytes: 1,
		MayError: io.ErrShortWrite,
	}

	// Write all the tokens.
	// Even if the underlying io.Writer may be faulty,
	// writing a valid token or value is guaranteed to at least
	// be appended to the internal buffer.
	// In other words, syntax errors occur before I/O errors.
	enc := NewEncoder(b)
	switch typeName {
	case "Token":
		for i, tok := range td.tokens {
			err := enc.WriteToken(tok)
			if err != nil && !errors.Is(err, io.ErrShortWrite) {
				t.Fatalf("%d: Encoder.WriteToken error: %v", i, err)
			}
		}
	case "Value":
		err := enc.WriteValue(RawValue(td.in))
		if err != nil && !errors.Is(err, io.ErrShortWrite) {
			t.Fatalf("Encoder.WriteValue error: %v", err)
		}
	}
	gotOutput := string(append(b.B, enc.unflushedBuffer()...))
	wantOutput := td.outCompacted + "\n"
	if gotOutput != wantOutput {
		t.Fatalf("output mismatch:\ngot  %s\nwant %s", gotOutput, wantOutput)
	}
}

type encoderMethodCall struct {
	in      tokOrVal
	wantErr error
}

var encoderErrorTestdata = []struct {
	name    string
	opts    EncodeOptions
	calls   []encoderMethodCall
	wantOut string
}{{
	name: "InvalidToken",
	calls: []encoderMethodCall{
		{zeroToken, &SyntaxError{str: "invalid json.Token"}},
	},
}, {
	name: "InvalidValue",
	calls: []encoderMethodCall{
		{RawValue(`#`), newInvalidCharacterError('#', "at start of value")},
	},
}, {
	name: "InvalidValue/DoubleZero",
	calls: []encoderMethodCall{
		{RawValue(`00`), newInvalidCharacterError('0', "after top-level value")},
	},
}, {
	name: "TruncatedValue",
	calls: []encoderMethodCall{
		{zeroValue, io.ErrUnexpectedEOF},
	},
}, {
	name: "TruncatedNull",
	calls: []encoderMethodCall{
		{RawValue(`nul`), io.ErrUnexpectedEOF},
	},
}, {
	name: "InvalidNull",
	calls: []encoderMethodCall{
		{RawValue(`nulL`), newInvalidCharacterError('L', "within literal null (expecting 'l')")},
	},
}, {
	name: "TruncatedFalse",
	calls: []encoderMethodCall{
		{RawValue(`fals`), io.ErrUnexpectedEOF},
	},
}, {
	name: "InvalidFalse",
	calls: []encoderMethodCall{
		{RawValue(`falsE`), newInvalidCharacterError('E', "within literal false (expecting 'e')")},
	},
}, {
	name: "TruncatedTrue",
	calls: []encoderMethodCall{
		{RawValue(`tru`), io.ErrUnexpectedEOF},
	},
}, {
	name: "InvalidTrue",
	calls: []encoderMethodCall{
		{RawValue(`truE`), newInvalidCharacterError('E', "within literal true (expecting 'e')")},
	},
}, {
	name: "TruncatedString",
	calls: []encoderMethodCall{
		{RawValue(`"star`), io.ErrUnexpectedEOF},
	},
}, {
	name: "InvalidString",
	calls: []encoderMethodCall{
		{RawValue(`"ok` + "\x00"), newInvalidCharacterError('\x00', `within string (expecting non-control character)`)},
	},
}, {
	name: "ValidString/AllowInvalidUTF8/Token",
	opts: EncodeOptions{AllowInvalidUTF8: true},
	calls: []encoderMethodCall{
		{String("living\xde\xad\xbe\xef"), nil},
	},
	wantOut: "\"living\xde\xad\ufffd\ufffd\"\n",
}, {
	name: "ValidString/AllowInvalidUTF8/Value",
	opts: EncodeOptions{AllowInvalidUTF8: true},
	calls: []encoderMethodCall{
		{RawValue("\"living\xde\xad\xbe\xef\""), nil},
	},
	wantOut: "\"living\xde\xad\ufffd\ufffd\"\n",
}, {
	name: "InvalidString/RejectInvalidUTF8",
	opts: EncodeOptions{AllowInvalidUTF8: false},
	calls: []encoderMethodCall{
		{String("living\xde\xad\xbe\xef"), &SyntaxError{str: "invalid UTF-8 within string"}},
		{RawValue("\"living\xde\xad\xbe\xef\""), &SyntaxError{str: "invalid UTF-8 within string"}},
	},
}, {
	name: "TruncatedNumber",
	calls: []encoderMethodCall{
		{RawValue(`0.`), io.ErrUnexpectedEOF},
	},
}, {
	name: "InvalidNumber",
	calls: []encoderMethodCall{
		{RawValue(`0.e`), newInvalidCharacterError('e', "within number (expecting digit)")},
	},
}, {
	name: "TruncatedObject/AfterStart",
	calls: []encoderMethodCall{
		{RawValue(`{`), io.ErrUnexpectedEOF},
	},
}, {
	name: "TruncatedObject/AfterName",
	calls: []encoderMethodCall{
		{RawValue(`{"0"`), io.ErrUnexpectedEOF},
	},
}, {
	name: "TruncatedObject/AfterColon",
	calls: []encoderMethodCall{
		{RawValue(`{"0":`), io.ErrUnexpectedEOF},
	},
}, {
	name: "TruncatedObject/AfterValue",
	calls: []encoderMethodCall{
		{RawValue(`{"0":0`), io.ErrUnexpectedEOF},
	},
}, {
	name: "TruncatedObject/AfterComma",
	calls: []encoderMethodCall{
		{RawValue(`{"0":0,`), io.ErrUnexpectedEOF},
	},
}, {
	name: "InvalidObject/MissingColon",
	calls: []encoderMethodCall{
		{RawValue(` { "fizz" "buzz" } `), newInvalidCharacterError('"', "after object name (expecting ':')")},
		{RawValue(` { "fizz" , "buzz" } `), newInvalidCharacterError(',', "after object name (expecting ':')")},
	},
}, {
	name: "InvalidObject/MissingComma",
	calls: []encoderMethodCall{
		{RawValue(` { "fizz" : "buzz" "gazz" } `), newInvalidCharacterError('"', "after object value (expecting ',' or '}')")},
		{RawValue(` { "fizz" : "buzz" : "gazz" } `), newInvalidCharacterError(':', "after object value (expecting ',' or '}')")},
	},
}, {
	name: "InvalidObject/ExtraComma",
	calls: []encoderMethodCall{
		{RawValue(` { , } `), newInvalidCharacterError(',', `at start of string (expecting '"')`)},
		{RawValue(` { "fizz" : "buzz" , } `), newInvalidCharacterError('}', `at start of string (expecting '"')`)},
	},
}, {
	name: "InvalidObject/InvalidName",
	calls: []encoderMethodCall{
		{RawValue(`{ null }`), newInvalidCharacterError('n', `at start of string (expecting '"')`)},
		{RawValue(`{ false }`), newInvalidCharacterError('f', `at start of string (expecting '"')`)},
		{RawValue(`{ true }`), newInvalidCharacterError('t', `at start of string (expecting '"')`)},
		{RawValue(`{ 0 }`), newInvalidCharacterError('0', `at start of string (expecting '"')`)},
		{RawValue(`{ {} }`), newInvalidCharacterError('{', `at start of string (expecting '"')`)},
		{RawValue(`{ [] }`), newInvalidCharacterError('[', `at start of string (expecting '"')`)},
		{ObjectStart, nil},
		{Null, errMissingName},
		{RawValue(`null`), errMissingName},
		{False, errMissingName},
		{RawValue(`false`), errMissingName},
		{True, errMissingName},
		{RawValue(`true`), errMissingName},
		{Uint(0), errMissingName},
		{RawValue(`0`), errMissingName},
		{ObjectStart, errMissingName},
		{RawValue(`{}`), errMissingName},
		{ArrayStart, errMissingName},
		{RawValue(`[]`), errMissingName},
		{ObjectEnd, nil},
	},
	wantOut: "{}\n",
}, {
	name: "InvalidObject/InvalidValue",
	calls: []encoderMethodCall{
		{RawValue(`{ "0": x }`), newInvalidCharacterError('x', `at start of value`)},
	},
}, {
	name: "InvalidObject/MismatchingDelim",
	calls: []encoderMethodCall{
		{RawValue(` { ] `), newInvalidCharacterError(']', `at start of string (expecting '"')`)},
		{RawValue(` { "0":0 ] `), newInvalidCharacterError(']', `after object value (expecting ',' or '}')`)},
		{ObjectStart, nil},
		{ArrayEnd, errMismatchDelim},
		{RawValue(`]`), newInvalidCharacterError(']', "at start of value")},
		{ObjectEnd, nil},
	},
	wantOut: "{}\n",
}, {
	name: "ValidObject/DuplicateNames",
	opts: EncodeOptions{RejectDuplicateNames: true},
	calls: []encoderMethodCall{
		{ObjectStart, nil},
		{String("0"), nil},
		{Uint(0), nil},
		{String("1"), nil},
		{Uint(1), nil},
		{ObjectEnd, nil},
		{RawValue(` { "0" : 0 , "1" : 1 } `), nil},
	},
	wantOut: `{"0":0,"1":1}` + "\n" + `{"0":0,"1":1}` + "\n",
}, {
	name: "InvalidObject/DuplicateNames",
	opts: EncodeOptions{RejectDuplicateNames: true},
	calls: []encoderMethodCall{
		{ObjectStart, nil},
		{String("0"), nil},
		{Uint(0), nil},
		{String("0"), &SyntaxError{str: `duplicate name "0" in object`}},
		{RawValue(`"0"`), &SyntaxError{str: `duplicate name "0" in object`}},
		{String("1"), nil},
		{Uint(1), nil},
		{String("1"), &SyntaxError{str: `duplicate name "1" in object`}},
		{RawValue(`"1"`), &SyntaxError{str: `duplicate name "1" in object`}},
		{ObjectEnd, nil},
		{RawValue(` { "0" : 0 , "1" : 1 , "0" : 0 } `), &SyntaxError{str: `duplicate name "0" in object`}},
	},
	wantOut: `{"0":0,"1":1}` + "\n",
}, {
	name: "TruncatedArray/AfterStart",
	calls: []encoderMethodCall{
		{RawValue(`[`), io.ErrUnexpectedEOF},
	},
}, {
	name: "TruncatedArray/AfterValue",
	calls: []encoderMethodCall{
		{RawValue(`[0`), io.ErrUnexpectedEOF},
	},
}, {
	name: "TruncatedArray/AfterComma",
	calls: []encoderMethodCall{
		{RawValue(`[0,`), io.ErrUnexpectedEOF},
	},
}, {
	name: "TruncatedArray/MissingComma",
	calls: []encoderMethodCall{
		{RawValue(` [ "fizz" "buzz" ] `), newInvalidCharacterError('"', "after array value (expecting ',' or ']')")},
	},
}, {
	name: "InvalidArray/MismatchingDelim",
	calls: []encoderMethodCall{
		{RawValue(` [ } `), newInvalidCharacterError('}', `at start of value`)},
		{ArrayStart, nil},
		{ObjectEnd, errMismatchDelim},
		{RawValue(`}`), newInvalidCharacterError('}', "at start of value")},
		{ArrayEnd, nil},
	},
	wantOut: "[]\n",
}}

// TestEncoderErrors test that Encoder errors occur when we expect and
// leaves the Encoder in a consistent state.
func TestEncoderErrors(t *testing.T) {
	for _, td := range encoderErrorTestdata {
		t.Run(path.Join(td.name), func(t *testing.T) {
			testEncoderErrors(t, td.opts, td.calls, td.wantOut)
		})
	}
}
func testEncoderErrors(t *testing.T, opts EncodeOptions, calls []encoderMethodCall, wantOut string) {
	dst := new(bytes.Buffer)
	enc := opts.NewEncoder(dst)
	for i, call := range calls {
		var gotErr error
		switch tokVal := call.in.(type) {
		case Token:
			gotErr = enc.WriteToken(tokVal)
		case RawValue:
			gotErr = enc.WriteValue(tokVal)
		}
		if !reflect.DeepEqual(gotErr, call.wantErr) {
			t.Fatalf("%d: error mismatch: got %#v, want %#v", i, gotErr, call.wantErr)
		}
	}
	gotOut := dst.String() + string(enc.unflushedBuffer())
	if gotOut != wantOut {
		t.Errorf("output mismatch:\ngot  %q\nwant %q", gotOut, wantOut)
	}
	gotOffset := int(enc.OutputOffset())
	wantOffset := len(wantOut)
	if gotOffset != wantOffset {
		t.Errorf("Encoder.OutputOffset = %v, want %v", gotOffset, wantOffset)
	}
}

func TestAppendString(t *testing.T) {
	var (
		escapeNothing    = func(r rune) bool { return false }
		escapeHTML       = func(r rune) bool { return r == '<' || r == '>' || r == '&' || r == '\u2028' || r == '\u2029' }
		escapeNonASCII   = func(r rune) bool { return r > unicode.MaxASCII }
		escapeEverything = func(r rune) bool { return true }
	)

	tests := []struct {
		in          string
		escapeRune  func(rune) bool
		want        string
		wantErr     error
		wantErrUTF8 error
	}{
		{"", nil, `""`, nil, nil},
		{"hello", nil, `"hello"`, nil, nil},
		{"\x00", nil, `"\u0000"`, nil, nil},
		{"\x1f", nil, `"\u001f"`, nil, nil},
		{"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz", nil, `"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"`, nil, nil},
		{" !#$%&'()*+,-./0123456789:;<=>?@[]^_`{|}~\x7f", nil, "\" !#$%&'()*+,-./0123456789:;<=>?@[]^_`{|}~\x7f\"", nil, nil},
		{"x\x80\ufffd", nil, "\"x\ufffd\ufffd\"", nil, &SyntaxError{str: "invalid UTF-8 within string"}},
		{"x\xff\ufffd", nil, "\"x\ufffd\ufffd\"", nil, &SyntaxError{str: "invalid UTF-8 within string"}},
		{"x\x80\ufffd", escapeNonASCII, "\"x\\ufffd\\ufffd\"", nil, &SyntaxError{str: "invalid UTF-8 within string"}},
		{"x\xff\ufffd", escapeNonASCII, "\"x\\ufffd\\ufffd\"", nil, &SyntaxError{str: "invalid UTF-8 within string"}},
		{"x\xc0", nil, "\"x\ufffd\"", nil, &SyntaxError{str: "invalid UTF-8 within string"}},
		{"x\xc0\x80", nil, "\"x\ufffd\ufffd\"", nil, &SyntaxError{str: "invalid UTF-8 within string"}},
		{"x\xe0", nil, "\"x\ufffd\"", nil, &SyntaxError{str: "invalid UTF-8 within string"}},
		{"x\xe0\x80", nil, "\"x\ufffd\ufffd\"", nil, &SyntaxError{str: "invalid UTF-8 within string"}},
		{"x\xe0\x80\x80", nil, "\"x\ufffd\ufffd\ufffd\"", nil, &SyntaxError{str: "invalid UTF-8 within string"}},
		{"x\xf0", nil, "\"x\ufffd\"", nil, &SyntaxError{str: "invalid UTF-8 within string"}},
		{"x\xf0\x80", nil, "\"x\ufffd\ufffd\"", nil, &SyntaxError{str: "invalid UTF-8 within string"}},
		{"x\xf0\x80\x80", nil, "\"x\ufffd\ufffd\ufffd\"", nil, &SyntaxError{str: "invalid UTF-8 within string"}},
		{"x\xf0\x80\x80\x80", nil, "\"x\ufffd\ufffd\ufffd\ufffd\"", nil, &SyntaxError{str: "invalid UTF-8 within string"}},
		{"x\xed\xba\xad", nil, "\"x\ufffd\ufffd\ufffd\"", nil, &SyntaxError{str: "invalid UTF-8 within string"}},
		{"\"\\/\b\f\n\r\t", nil, `"\"\\/\b\f\n\r\t"`, nil, nil},
		{"\"\\/\b\f\n\r\t", escapeEverything, `"\u0022\u005c\u002f\u0008\u000c\u000a\u000d\u0009"`, nil, nil},
		{"٩(-̮̮̃-̃)۶ ٩(●̮̮̃•̃)۶ ٩(͡๏̯͡๏)۶ ٩(-̮̮̃•̃).", nil, `"٩(-̮̮̃-̃)۶ ٩(●̮̮̃•̃)۶ ٩(͡๏̯͡๏)۶ ٩(-̮̮̃•̃)."`, nil, nil},
		{"٩(-̮̮̃-̃)۶ ٩(●̮̮̃•̃)۶ ٩(͡๏̯͡๏)۶ ٩(-̮̮̃•̃).", escapeNonASCII, `"\u0669(-\u032e\u032e\u0303-\u0303)\u06f6 \u0669(\u25cf\u032e\u032e\u0303\u2022\u0303)\u06f6 \u0669(\u0361\u0e4f\u032f\u0361\u0e4f)\u06f6 \u0669(-\u032e\u032e\u0303\u2022\u0303)."`, nil, nil},
		{"٩(-̮̮̃-̃)۶ ٩(●̮̮̃•̃)۶ ٩(͡๏̯͡๏)۶ ٩(-̮̮̃•̃).", escapeEverything, `"\u0669\u0028\u002d\u032e\u032e\u0303\u002d\u0303\u0029\u06f6\u0020\u0669\u0028\u25cf\u032e\u032e\u0303\u2022\u0303\u0029\u06f6\u0020\u0669\u0028\u0361\u0e4f\u032f\u0361\u0e4f\u0029\u06f6\u0020\u0669\u0028\u002d\u032e\u032e\u0303\u2022\u0303\u0029\u002e"`, nil, nil},
		{"\u0080\u00f6\u20ac\ud799\ue000\ufb33\ufffd\U0001f602", nil, "\"\u0080\u00f6\u20ac\ud799\ue000\ufb33\ufffd\U0001f602\"", nil, nil},
		{"\u0080\u00f6\u20ac\ud799\ue000\ufb33\ufffd\U0001f602", escapeEverything, `"\u0080\u00f6\u20ac\ud799\ue000\ufb33\ufffd\ud83d\ude02"`, nil, nil},
		{"\u0000\u001f\u0020\u0022\u0026\u003c\u003e\u005c\u007f\u0080\u2028\u2029\ufffd\U0001f602", nil, "\"\\u0000\\u001f\u0020\\\"\u0026\u003c\u003e\\\\\u007f\u0080\u2028\u2029\ufffd\U0001f602\"", nil, nil},
		{"\u0000\u001f\u0020\u0022\u0026\u003c\u003e\u005c\u007f\u0080\u2028\u2029\ufffd\U0001f602", escapeNothing, "\"\\u0000\\u001f\u0020\\\"\u0026\u003c\u003e\\\\\u007f\u0080\u2028\u2029\ufffd\U0001f602\"", nil, nil},
		{"\u0000\u001f\u0020\u0022\u0026\u003c\u003e\u005c\u007f\u0080\u2028\u2029\ufffd\U0001f602", escapeHTML, "\"\\u0000\\u001f\u0020\\\"\\u0026\\u003c\\u003e\\\\\u007f\u0080\\u2028\\u2029\ufffd\U0001f602\"", nil, nil},
		{"\u0000\u001f\u0020\u0022\u0026\u003c\u003e\u005c\u007f\u0080\u2028\u2029\ufffd\U0001f602", escapeNonASCII, "\"\\u0000\\u001f\u0020\\\"\u0026\u003c\u003e\\\\\u007f\\u0080\\u2028\\u2029\\ufffd\\ud83d\\ude02\"", nil, nil},
		{"\u0000\u001f\u0020\u0022\u0026\u003c\u003e\u005c\u007f\u0080\u2028\u2029\ufffd\U0001f602", escapeEverything, "\"\\u0000\\u001f\\u0020\\u0022\\u0026\\u003c\\u003e\\u005c\\u007f\\u0080\\u2028\\u2029\\ufffd\\ud83d\\ude02\"", nil, nil},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got, gotErr := appendString(nil, tt.in, false, tt.escapeRune)
			if string(got) != tt.want || !reflect.DeepEqual(gotErr, tt.wantErr) {
				t.Errorf("appendString(nil, %q, false, ...) = (%s, %v), want (%s, %v)", tt.in, got, gotErr, tt.want, tt.wantErr)
			}
			switch got, gotErr := appendString(nil, tt.in, true, tt.escapeRune); {
			case tt.wantErrUTF8 == nil && (string(got) != tt.want || !reflect.DeepEqual(gotErr, tt.wantErr)):
				t.Errorf("appendString(nil, %q, true, ...) = (%s, %v), want (%s, %v)", tt.in, got, gotErr, tt.want, tt.wantErr)
			case tt.wantErrUTF8 != nil && (!strings.HasPrefix(tt.want, string(got)) || !reflect.DeepEqual(gotErr, tt.wantErrUTF8)):
				t.Errorf("appendString(nil, %q, true, ...) = (%s, %v), want (%s, %v)", tt.in, got, gotErr, tt.want, tt.wantErrUTF8)
			}
		})
	}
}

func TestAppendNumber(t *testing.T) {
	tests := []struct {
		in     float64
		want32 string
		want64 string
	}{
		{math.E, "2.7182817", "2.718281828459045"},
		{math.Pi, "3.1415927", "3.141592653589793"},
		{math.NaN(), `"NaN"`, `"NaN"`},
		{math.Inf(+1), `"Infinity"`, `"Infinity"`},
		{math.Inf(-1), `"-Infinity"`, `"-Infinity"`},
		{math.SmallestNonzeroFloat32, "1e-45", "1.401298464324817e-45"},
		{math.SmallestNonzeroFloat64, "0", "5e-324"},
		{math.MaxFloat32, "3.4028235e+38", "3.4028234663852886e+38"},
		{math.MaxFloat64, `"Infinity"`, "1.7976931348623157e+308"},
		{0.1111111111111111, "0.11111111", "0.1111111111111111"},
		{0.2222222222222222, "0.22222222", "0.2222222222222222"},
		{0.3333333333333333, "0.33333334", "0.3333333333333333"},
		{0.4444444444444444, "0.44444445", "0.4444444444444444"},
		{0.5555555555555555, "0.5555556", "0.5555555555555555"},
		{0.6666666666666666, "0.6666667", "0.6666666666666666"},
		{0.7777777777777777, "0.7777778", "0.7777777777777777"},
		{0.8888888888888888, "0.8888889", "0.8888888888888888"},
		{0.9999999999999999, "1", "0.9999999999999999"},

		// The following entries are from RFC 8785, appendix B
		// which are designed to ensure repeatable formatting of 64-bit floats.
		{math.Float64frombits(0x0000000000000000), "0", "0"},
		{math.Float64frombits(0x8000000000000000), "-0", "-0"}, // differs from RFC 8785
		{math.Float64frombits(0x0000000000000001), "0", "5e-324"},
		{math.Float64frombits(0x8000000000000001), "-0", "-5e-324"},
		{math.Float64frombits(0x7fefffffffffffff), `"Infinity"`, "1.7976931348623157e+308"},
		{math.Float64frombits(0xffefffffffffffff), `"-Infinity"`, "-1.7976931348623157e+308"},
		{math.Float64frombits(0x4340000000000000), "9007199000000000", "9007199254740992"},
		{math.Float64frombits(0xc340000000000000), "-9007199000000000", "-9007199254740992"},
		{math.Float64frombits(0x4430000000000000), "295147900000000000000", "295147905179352830000"},
		{math.Float64frombits(0x7fffffffffffffff), `"NaN"`, `"NaN"`},
		{math.Float64frombits(0x7ff0000000000000), `"Infinity"`, `"Infinity"`},
		{math.Float64frombits(0x44b52d02c7e14af5), "1e+23", "9.999999999999997e+22"},
		{math.Float64frombits(0x44b52d02c7e14af6), "1e+23", "1e+23"},
		{math.Float64frombits(0x44b52d02c7e14af7), "1e+23", "1.0000000000000001e+23"},
		{math.Float64frombits(0x444b1ae4d6e2ef4e), "1e+21", "999999999999999700000"},
		{math.Float64frombits(0x444b1ae4d6e2ef4f), "1e+21", "999999999999999900000"},
		{math.Float64frombits(0x444b1ae4d6e2ef50), "1e+21", "1e+21"},
		{math.Float64frombits(0x3eb0c6f7a0b5ed8c), "0.000001", "9.999999999999997e-7"},
		{math.Float64frombits(0x3eb0c6f7a0b5ed8d), "0.000001", "0.000001"},
		{math.Float64frombits(0x41b3de4355555553), "333333340", "333333333.3333332"},
		{math.Float64frombits(0x41b3de4355555554), "333333340", "333333333.33333325"},
		{math.Float64frombits(0x41b3de4355555555), "333333340", "333333333.3333333"},
		{math.Float64frombits(0x41b3de4355555556), "333333340", "333333333.3333334"},
		{math.Float64frombits(0x41b3de4355555557), "333333340", "333333333.33333343"},
		{math.Float64frombits(0xbecbf647612f3696), "-0.0000033333333", "-0.0000033333333333333333"},
		{math.Float64frombits(0x43143ff3c1cb0959), "1424953900000000", "1424953923781206.2"},

		// The following are select entries from RFC 8785, appendix B,
		// but modified for equivalent 32-bit behavior.
		{float64(math.Float32frombits(0x65a96815)), "9.999999e+22", "9.999998877476383e+22"},
		{float64(math.Float32frombits(0x65a96816)), "1e+23", "9.999999778196308e+22"},
		{float64(math.Float32frombits(0x65a96817)), "1.0000001e+23", "1.0000000678916234e+23"},
		{float64(math.Float32frombits(0x6258d725)), "999999900000000000000", "999999879303389000000"},
		{float64(math.Float32frombits(0x6258d726)), "999999950000000000000", "999999949672133200000"},
		{float64(math.Float32frombits(0x6258d727)), "1e+21", "1.0000000200408773e+21"},
		{float64(math.Float32frombits(0x6258d728)), "1.0000001e+21", "1.0000000904096215e+21"},
		{float64(math.Float32frombits(0x358637bc)), "9.999999e-7", "9.99999883788405e-7"},
		{float64(math.Float32frombits(0x358637bd)), "0.000001", "9.999999974752427e-7"},
		{float64(math.Float32frombits(0x358637be)), "0.0000010000001", "0.0000010000001111620804"},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			if got32 := string(appendNumber(nil, tt.in, 32)); got32 != tt.want32 {
				t.Errorf("appendNumber(nil, %v, 32) = %v, want %v", tt.in, got32, tt.want32)
			}
			if got64 := string(appendNumber(nil, tt.in, 64)); got64 != tt.want64 {
				t.Errorf("appendNumber(nil, %v, 64) = %v, want %v", tt.in, got64, tt.want64)
			}
		})
	}
}
