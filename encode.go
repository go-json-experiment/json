// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import (
	"io"
	"math"
	"math/bits"
	"strconv"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

// Encoder is a streaming encoder for raw JSON values and tokens.
//
// WriteToken and WriteValue calls may be interleaved.
// For example, the following JSON value:
//
//	{"name":"value","array":[null,false,true,3.14159],"object":{"k":"v"}}
//
// can be composed with the following calls (ignoring errors for brevity):
//
//	e.WriteToken(StartObject)        // {
//	e.WriteToken(String("name"))     // "name"
//	e.WriteToken(String("value"))    // "value"
//	e.WriteValue(Value(`"array"`))   // "array"
//	e.WriteToken(StartArray)         // [
//	e.WriteToken(Null)               // null
//	e.WriteToken(False)              // false
//	e.WriteValue(Value("true"))      // true
//	e.WriteToken(Number(3.14159))    // 3.14159
//	e.WriteToken(EndArray)           // ]
//	e.WriteValue(Value(`"object"`))  // "object"
//	e.WriteValue(Value(`{"k":"v"}`)) // {"k":"v"}
//	e.WriteToken(EndObject)          // }
//
// The above is one of many possible sequence of calls and
// may not represent the most sensible method to call for any given token/value.
// For example, it is probably more common to call WriteToken with a string
// for object names.
type Encoder struct{}

// NewEncoder constructs a new streaming encoder writing to w.
// It flushes the internal buffer when the buffer is sufficiently full or
// when a top-level value has been written.
func NewEncoder(w io.Writer) *Encoder {
	panic("not implemented")
}

// WriteToken writes the next Token and advances the internal write offset.
func (e *Encoder) WriteToken(t Token) error {
	// TODO: May the Encoder alias the provided Token?
	panic("not implemented")
}

// WriteValue writes the next Value and advances the internal write offset.
func (e *Encoder) WriteValue(v Value) error {
	// TODO: May the Encoder alias the provided Value?
	panic("not implemented")
}

// appendString appends s to dst as a JSON string per RFC 7159, section 7.
//
// If validateUTF8 is specified, this rejects input that contains invalid UTF-8
// otherwise it is replaced with the Unicode replacement character.
// If escapeRune is provided, it specifies which runes to escape using
// hexadecimal sequences. If nil, the shortest representable form is used,
// which is also the canonical form for strings (RFC 8785, section 3.2.2.2).
//
// Note that this API allows full control over the formatting of strings
// except for whether a forward solidus '/' may be formatted as '\/' and
// the casing of hexadecimal Unicode escape sequences.
func appendString(dst []byte, s string, validateUTF8 bool, escapeRune func(rune) bool) ([]byte, error) {
	dst = append(dst, '"')
	for len(s) > 0 {
		// Optimize for long sequences of unescaped characters.
		if escapeRune == nil {
			var n int
			for len(s) > n && (' ' <= s[n] && s[n] != '\\' && s[n] != '"' && s[n] <= unicode.MaxASCII) {
				n++
			}
			dst, s = append(dst, s[:n]...), s[n:]
			if len(s) == 0 {
				break
			}
		}

		switch r, rn := utf8.DecodeRuneInString(s); {
		case r == utf8.RuneError && rn == 1:
			if validateUTF8 {
				return dst, &SyntaxError{str: "invalid UTF-8 within string"}
			}
			dst = append(dst, `\ufffd`...)
			s = s[1:]
		case escapeRune != nil && escapeRune(r):
			if r1, r2 := utf16.EncodeRune(r); r1 != '\ufffd' && r2 != '\ufffd' {
				dst = append(dst, "\\u"...)
				dst = appendHexUint16(dst, uint16(r1))
				dst = append(dst, "\\u"...)
				dst = appendHexUint16(dst, uint16(r2))
			} else {
				dst = append(dst, "\\u"...)
				dst = appendHexUint16(dst, uint16(r))
			}
			s = s[rn:]
		case r < ' ' || r == '"' || r == '\\':
			switch r {
			case '"', '\\':
				dst = append(dst, '\\', byte(r))
			case '\b':
				dst = append(dst, "\\b"...)
			case '\f':
				dst = append(dst, "\\f"...)
			case '\n':
				dst = append(dst, "\\n"...)
			case '\r':
				dst = append(dst, "\\r"...)
			case '\t':
				dst = append(dst, "\\t"...)
			default:
				dst = append(dst, "\\u"...)
				dst = appendHexUint16(dst, uint16(r))
			}
			s = s[rn:]
		default:
			dst, s = append(dst, s[:rn]...), s[rn:]
		}
	}
	dst = append(dst, '"')
	return dst, nil
}

// reformatString consumes a JSON string from src and appends it to dst,
// reformatting it if necessary for the given escapeRune parameter.
// It returns the appended output and the remainder of the input.
func reformatString(dst, src []byte, validateUTF8 bool, escapeRune func(rune) bool) ([]byte, []byte, error) {
	n, err := consumeString(src, validateUTF8)
	if err != nil {
		return dst, src[n:], err
	}
	// TODO: Implement a direct, raw-to-raw reformat for strings.
	// If the escapeRune option would have resulted in no changes to the output,
	// it would be faster to simply append src to dst without going through
	// an intermediary representation in a separate buffer.
	b, _ := unescapeString(make([]byte, 0, n), src[:n])
	dst, _ = appendString(dst, string(b), validateUTF8, escapeRune)
	return dst, src[n:], nil
}

// appendNumber appends v to dst as a JSON number per RFC 7159, section 6.
// It formats numbers similar to the ES6 number-to-string conversion.
// See https://golang.org/issue/14135.
//
// The output is identical to ECMA-262, 6th edition, section 7.1.12.1
// for 64-bit floating-point numbers except
// for -0, which is formatted as -0 instead of just 0,
// NaN, which is formatted as the JSON string "NaN",
// +Inf, which is formatted as the JSON string "Infinity", and
// -Inf, which is formatted as the JSON string "-Infinity".
//
// For 32-bit floating-point numbers,
// the output is a 32-bit equivalent of the algorithm.
// Note that ECMA-262 specifies no algorithm for 32-bit numbers.
func appendNumber(dst []byte, v float64, bits int) []byte {
	if bits == 32 {
		v = float64(float32(v))
	}

	switch {
	case math.IsNaN(v):
		return append(dst, `"NaN"`...)
	case math.IsInf(v, +1):
		return append(dst, `"Infinity"`...)
	case math.IsInf(v, -1):
		return append(dst, `"-Infinity"`...)
	}

	abs := math.Abs(v)
	fmt := byte('f')
	if abs != 0 {
		if bits == 64 && (float64(abs) < 1e-6 || float64(abs) >= 1e21) ||
			bits == 32 && (float32(abs) < 1e-6 || float32(abs) >= 1e21) {
			fmt = 'e'
		}
	}
	dst = strconv.AppendFloat(dst, v, fmt, -1, bits)
	if fmt == 'e' {
		// Clean up e-09 to e-9.
		n := len(dst)
		if n >= 4 && dst[n-4] == 'e' && dst[n-3] == '-' && dst[n-2] == '0' {
			dst[n-2] = dst[n-1]
			dst = dst[:n-1]
		}
	}
	return dst
}

// appendHexUint16 appends v to dst as a 4-byte hexadecimal number.
func appendHexUint16(dst []byte, v uint16) []byte {
	dst = append(dst, "0000"[1+(bits.Len16(v)-1)/4:]...)
	dst = strconv.AppendUint(dst, uint64(v), 16)
	return dst
}
