// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import (
	"io"
	"math"
	"strconv"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

// Decoder is a streaming decoder for raw JSON values and tokens.
//
// ReadToken and ReadValue calls may be interleaved.
// For example, the following JSON value:
//
//	{"name":"value","array":[null,false,true,3.14159],"object":{"k":"v"}}
//
// can be parsed with the following calls (ignoring errors for brevity):
//
//	d.ReadToken() // {
//	d.ReadToken() // "name"
//	d.ReadToken() // "value"
//	d.ReadValue() // "array"
//	d.ReadToken() // [
//	d.ReadToken() // null
//	d.ReadToken() // false
//	d.ReadValue() // true
//	d.ReadToken() // 3.14159
//	d.ReadToken() // ]
//	d.ReadValue() // "object"
//	d.ReadValue() // {"k":"v"}
//	d.ReadToken() // }
//
// The above is one of many possible sequence of calls and
// may not represent the most sensible method to call for any given token/value.
// For example, it is probably more common to call ReadToken to obtain a
// string token for object names.
type Decoder struct{}

// NewDecoder constructs a new streaming decoder reading from r.
func NewDecoder(r io.Reader) *Decoder {
	panic("not implemented")
}

// PeekKind retrieves the next token kind, but does not advance the read offset.
// It returns 0 if there are no more tokens.
// It returns an invalid non-zero kind if an error occurs.
func (d *Decoder) PeekKind() Kind {
	panic("not implemented")
}

// ReadToken reads the next Token, advancing the read offset.
// The returned token is only valid until the next Peek or Read call.
// It returns io.EOF if there are no more tokens.
func (d *Decoder) ReadToken() (Token, error) {
	// TODO: May the user allow Token to escape?
	panic("not implemented")
}

// ReadValue returns the next raw JSON value, advancing the read offset.
// The value is stripped of any leading or trailing whitespace.
// The returned value is only valid until the next Peek or Read call and
// may not be mutated while the Decoder remains in use.
// It returns io.EOF if there are no more values.
func (d *Decoder) ReadValue() (Value, error) {
	panic("not implemented")
}

// TODO: How should internal buffering work? For performance, Decoder will read
//	decently sized chunks, which may cause it read past the next JSON value.
//	In v1, json.Decoder.Buffered provides access to the excess data.
//	Alternatively, we could take the "compress/flate" approach and guarantee
//	that we never read past the last delimiter if the provided io.Reader
//	also implements io.ByteReader.

// consumeWhitespace consumes leading JSON whitespace per RFC 7159, section 2.
func consumeWhitespace(b []byte) (n int) {
	// NOTE: The arguments and logic are kept simple to keep this inlineable.
loop:
	if len(b) > n && (b[n] == ' ' || b[n] == '\t' || b[n] == '\r' || b[n] == '\n') {
		n++
		goto loop // TODO(https://golang.org/issue/14768): Use for loop when Go1.16 is released.
	}
	return n
}

// consumeNull consumes the next JSON null literal per RFC 7159, section 3.
// It returns 0 if it is invalid, in which case consumeLiteral should be used.
func consumeNull(b []byte) int {
	// NOTE: The arguments and logic are kept simple to keep this inlineable.
	const literal = "null"
	if len(b) >= len(literal) && string(b[:len(literal)]) == literal {
		return len(literal)
	}
	return 0
}

// consumeFalse consumes the next JSON false literal per RFC 7159, section 3.
// It returns 0 if it is invalid, in which case consumeLiteral should be used.
func consumeFalse(b []byte) int {
	// NOTE: The arguments and logic are kept simple to keep this inlineable.
	const literal = "false"
	if len(b) >= len(literal) && string(b[:len(literal)]) == literal {
		return len(literal)
	}
	return 0
}

// consumeTrue consumes the next JSON true literal per RFC 7159, section 3.
// It returns 0 if it is invalid, in which case consumeLiteral should be used.
func consumeTrue(b []byte) int {
	// NOTE: The arguments and logic are kept simple to keep this inlineable.
	const literal = "true"
	if len(b) >= len(literal) && string(b[:len(literal)]) == literal {
		return len(literal)
	}
	return 0
}

// consumeLiteral consumes the next JSON literal per RFC 7159, section 3.
// If the input appears truncated, it returns io.ErrUnexpectedEOF.
func consumeLiteral(b []byte, lit string) (n int, err error) {
	for i := 0; i < len(b) && i < len(lit); i++ {
		if b[i] != lit[i] {
			return i, newInvalidCharacterError(b[i], "within literal "+lit+" (expecting "+escapeCharacter(lit[i])+")")
		}
	}
	if len(b) < len(lit) {
		return len(b), io.ErrUnexpectedEOF
	}
	return len(lit), nil
}

// consumeSimpleString consumes the next JSON string per RFC 7159, section 7
// but is limited to the grammar for an ASCII string without escape sequences.
// It returns 0 if it is invalid or more complicated than a simple string,
// in which case consumeString should be called.
func consumeSimpleString(b []byte) (n int) {
	// NOTE: The arguments and logic are kept simple to keep this inlineable.
	if len(b) > 0 && b[0] == '"' {
		n++
	loop:
		if len(b) > n && (' ' <= b[n] && b[n] != '\\' && b[n] != '"' && b[n] <= unicode.MaxASCII) {
			n++
			goto loop // TODO(https://golang.org/issue/14768): Use for loop when Go1.16 is released.
		}
		if len(b) > n && b[n] == '"' {
			n++
			return n
		}
	}
	return 0
}

// consumeString consumes the next JSON string per RFC 7159, section 7.
// If validateUTF8 is false, then this allows the presence of invalid UTF-8
// characters within the string itself.
// It reports the number of bytes consumed and whether an error was encounted.
// If the input appears truncated, it returns io.ErrUnexpectedEOF.
func consumeString(b []byte, validateUTF8 bool) (n int, err error) {
	// TODO: Add a "continuation offset" argument that allows this function
	// to start at some offset into b where the previous bytes are validated.
	// The offset must not point to the middle of a multi-byte UTF-8 character
	// or an escape sequence.

	// Consume the leading quote.
	switch {
	case len(b) == 0:
		return n, io.ErrUnexpectedEOF
	case b[0] == '"':
		n++
	default:
		return n, newInvalidCharacterError(b[n], `at start of string (expecting '"')`)
	}

	// Consume every character in the string.
	for len(b) > n {
		// Optimize for long sequences of unescaped characters.
		for len(b) > n && (' ' <= b[n] && b[n] != '\\' && b[n] != '"' && b[n] <= unicode.MaxASCII) {
			n++
		}
		if len(b) == n {
			return n, io.ErrUnexpectedEOF
		}

		switch r, rn := utf8.DecodeRune(b[n:]); {
		case r == utf8.RuneError && rn == 1: // invalid UTF-8
			if validateUTF8 {
				switch {
				case b[n]&0b111_00000 == 0b110_00000 && len(b) < n+2:
					return n, io.ErrUnexpectedEOF
				case b[n]&0b1111_0000 == 0b1110_0000 && len(b) < n+3:
					return n, io.ErrUnexpectedEOF
				case b[n]&0b11111_000 == 0b11110_000 && len(b) < n+4:
					return n, io.ErrUnexpectedEOF
				default:
					return n, &SyntaxError{str: "invalid UTF-8 within string"}
				}
			}
			n++
		case r < ' ': // invalid control character
			return n, newInvalidCharacterError(b[n], "within string (expecting non-control character)")
		case r == '"': // terminating quote
			n++
			return n, nil
		case r == '\\': // escape sequence
			if len(b) < n+2 {
				return n, io.ErrUnexpectedEOF
			}
			switch r := b[n+1]; r {
			case '"', '\\', '/', 'b', 'f', 'n', 'r', 't':
				n += 2
			case 'u':
				if len(b) < n+6 {
					return n, io.ErrUnexpectedEOF
				}
				v1, ok := parseHexUint16(b[n+2 : n+6])
				if !ok {
					return n, &SyntaxError{str: "invalid escape sequence " + strconv.Quote(string(b[n:n+6])) + " within string"}
				}
				n += 6

				if validateUTF8 && utf16.IsSurrogate(rune(v1)) {
					if len(b) >= n+2 && (b[n] != '\\' || b[n+1] != 'u') {
						return n, &SyntaxError{str: "invalid unpaired surrogate half within string"}
					}
					if len(b) < n+6 {
						return n, io.ErrUnexpectedEOF
					}
					v2, ok := parseHexUint16(b[n+2 : n+6])
					if !ok {
						return n, &SyntaxError{str: "invalid escape sequence " + strconv.Quote(string(b[n:n+6])) + " within string"}
					}
					if utf16.DecodeRune(rune(v1), rune(v2)) == unicode.ReplacementChar {
						return n, &SyntaxError{str: "invalid surrogate pair in string"}
					}
					n += 6
				}
			default:
				return n, &SyntaxError{str: "invalid escape sequence " + strconv.Quote(string(b[n:n+2])) + " within string"}
			}
		default:
			n += rn
		}
	}
	return n, io.ErrUnexpectedEOF
}

// unescapeString append the unescaped for of a JSON string in src to dst.
// Any invalid UTF-8 within the string will be replaced with utf8.RuneError.
// The input must be an entire JSON string with no surrounding whitespace.
func unescapeString(dst, src []byte) (v []byte, ok bool) {
	// Consume quote delimiters.
	if len(src) < 1 || src[0] != '"' {
		return dst, false
	}
	src = src[1:]

	// Consume every character until completion.
	for len(src) > 0 {
		// Optimize for long sequences of unescaped characters.
		var n int
		for len(src) > n && (' ' <= src[n] && src[n] != '\\' && src[n] != '"' && src[n] <= unicode.MaxASCII) {
			n++
		}
		dst, src = append(dst, src[:n]...), src[n:]
		if len(src) == 0 {
			break
		}

		switch r, rn := utf8.DecodeRune(src); {
		case r == utf8.RuneError && rn == 1:
			// NOTE: An unescaped string may be longer than the escaped string
			// because invalid UTF-8 bytes are being replaced.
			dst, src = append(dst, "\uFFFD"...), src[1:]
		case r < ' ':
			return dst, false // invalid control character or unescaped quote
		case r == '"':
			src = src[1:]
			return dst, len(src) == 0
		case r == '\\':
			if len(src) < 2 {
				return dst, false // truncated escape sequence
			}
			switch r := src[1]; r {
			case '"', '\\', '/':
				dst, src = append(dst, r), src[2:]
			case 'b':
				dst, src = append(dst, '\b'), src[2:]
			case 'f':
				dst, src = append(dst, '\f'), src[2:]
			case 'n':
				dst, src = append(dst, '\n'), src[2:]
			case 'r':
				dst, src = append(dst, '\r'), src[2:]
			case 't':
				dst, src = append(dst, '\t'), src[2:]
			case 'u':
				if len(src) < 6 {
					return dst, false // truncated escape sequence
				}
				v1, ok := parseHexUint16(src[2:6])
				if !ok {
					return dst, false // invalid escape sequence
				}
				src = src[6:]

				// Check whether this is a surrogate halve.
				r := rune(v1)
				if utf16.IsSurrogate(r) {
					r = unicode.ReplacementChar // assume failure unless the following succeeds
					if len(src) >= 6 && src[0] == '\\' && src[1] == 'u' {
						if v2, ok := parseHexUint16(src[2:6]); ok {
							if r = utf16.DecodeRune(rune(v1), rune(v2)); r != unicode.ReplacementChar {
								src = src[6:]
							}
						}
					}
				}

				var arr [utf8.UTFMax]byte
				dst = append(dst, arr[:utf8.EncodeRune(arr[:], r)]...)
			default:
				return dst, false // invalid escape sequence
			}
		default:
			dst, src = append(dst, src[:rn]...), src[rn:]
		}
	}

	return dst, false // truncated input
}

// consumeSimpleNumber consumes the next JSON number per RFC 7159, section 6
// but is limited to the grammar for a positive integer.
// It returns 0 if it is invalid or more complicated than a simple integer,
// in which case consumeNumber should be called.
func consumeSimpleNumber(b []byte) (n int) {
	// NOTE: The arguments and logic are kept simple to keep this inlineable.
	if len(b) > 0 {
		if b[0] == '0' {
			n++
		} else if '1' <= b[0] && b[0] <= '9' {
			n++
		loop:
			if len(b) > n && ('0' <= b[n] && b[n] <= '9') {
				n++
				goto loop // TODO(https://golang.org/issue/14768): Use for loop when Go1.16 is released.
			}
		} else {
			return 0
		}
		if len(b) == n || !(b[n] == '.' || b[n] == 'e' || b[n] == 'E') {
			return n
		}
	}
	return 0
}

// consumeNumber consumes the next JSON number per RFC 7159, section 6.
// It reports the number of bytes consumed and whether an error was encounted.
// If the input appears truncated, it returns io.ErrUnexpectedEOF.
//
// Note that JSON numbers are not self-terminating.
// If the entire input is consumed, then the caller needs to consider whether
// there may be subsequent unread data that may still be part of this number.
func consumeNumber(b []byte) (n int, err error) {
	// Consume optional minus sign.
	if len(b) > 0 && b[0] == '-' {
		n++
	}

	// Consume required integer component.
	switch {
	case len(b) == n:
		return n, io.ErrUnexpectedEOF
	case b[n] == '0':
		n++
	case '1' <= b[n] && b[n] <= '9':
		n++
		for len(b) > n && ('0' <= b[n] && b[n] <= '9') {
			n++
		}
	default:
		return n, newInvalidCharacterError(b[n], "within number (expecting digit)")
	}

	// Consume optional fractional component.
	if len(b) > n && b[n] == '.' {
		n++
		switch {
		case len(b) == n:
			return n, io.ErrUnexpectedEOF
		case '0' <= b[n] && b[n] <= '9':
			n++
		default:
			return n, newInvalidCharacterError(b[n], "within number (expecting digit)")
		}
		for len(b) > n && ('0' <= b[n] && b[n] <= '9') {
			n++
		}
	}

	// Consume optional exponent component.
	if len(b) > n && (b[n] == 'e' || b[n] == 'E') {
		n++
		if len(b) > n && (b[n] == '-' || b[n] == '+') {
			n++
		}
		switch {
		case len(b) == n:
			return n, io.ErrUnexpectedEOF
		case '0' <= b[n] && b[n] <= '9':
			n++
		default:
			return n, newInvalidCharacterError(b[n], "within number (expecting digit)")
		}
		for len(b) > n && ('0' <= b[n] && b[n] <= '9') {
			n++
		}
	}

	// TODO: Should we return io.ErrUnexpectedEOF if len(b) == n to force the
	// caller to handle whether the number is truly terminated?
	return n, nil
}

// parseHexUint16 is similar to strconv.ParseUint,
// but operates directly on []byte and is optimized for base-16.
// See https://golang.org/issue/42429.
func parseHexUint16(b []byte) (v uint16, ok bool) {
	if len(b) != 4 {
		return 0, false
	}
	for _, c := range b[:4] {
		switch {
		case '0' <= c && c <= '9':
			c = c - '0'
		case 'a' <= c && c <= 'f':
			c = 10 + c - 'a'
		case 'A' <= c && c <= 'F':
			c = 10 + c - 'A'
		default:
			return 0, false
		}
		v = v*16 + uint16(c)
	}
	return v, true
}

// parseDecUint is similar to strconv.ParseUint,
// but operates directly on []byte and is optimized for base-10.
// If the number is syntactically valid but overflows uint64,
// then it returns (math.MaxUint64, true).
// See https://golang.org/issue/42429.
func parseDecUint(b []byte) (v uint64, ok bool) {
	// Overflow logic is based on strconv/atoi.go:138-149 from Go1.15, where:
	//	• cutoff is equal to math.MaxUint64/10+1, and
	//	• the n1 > maxVal check is unnecessary
	//	  since maxVal is equivalent to math.MaxUint64.
	var n int
	var overflow bool
	for len(b) > n && ('0' <= b[n] && b[n] <= '9') {
		overflow = overflow || v >= math.MaxUint64/10+1
		v *= 10

		v1 := v + uint64(b[n]-'0')
		overflow = overflow || v1 < v
		v = v1

		n++
	}
	if overflow {
		v = math.MaxUint64
	}
	if n == 0 || len(b) != n {
		return 0, false
	}
	return v, true
}
