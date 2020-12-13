// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import (
	"io"
	"math"
	"reflect"
	"strings"
	"testing"
)

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
		{`"\uDEAD\u"`, false, 7, "\ufffd", io.ErrUnexpectedEOF, nil},
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
		{"0.", false, 2, io.ErrUnexpectedEOF},
		{"-0.", false, 3, io.ErrUnexpectedEOF},
		{"0e", false, 2, io.ErrUnexpectedEOF},
		{"-0e", false, 3, io.ErrUnexpectedEOF},
		{"0E", false, 2, io.ErrUnexpectedEOF},
		{"-0E", false, 3, io.ErrUnexpectedEOF},
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
