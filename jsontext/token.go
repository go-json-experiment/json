// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package jsontext

import (
	"bytes"
	"errors"
	"math"
	"strconv"

	"github.com/go-json-experiment/json/internal/jsonflags"
	"github.com/go-json-experiment/json/internal/jsonwire"
)

// NOTE: Token is analogous to v1 json.Token.

var errInvalidToken = errors.New("invalid jsontext.Token")

func tokenTypeTag() *decodeBuffer {
	return &decodeBuffer{}
}

// these special tags will have nil buf
var (
	strTag   = tokenTypeTag()
	uintTag  = tokenTypeTag()
	intTag   = tokenTypeTag()
	floatTag = tokenTypeTag()
)

// RawToken likes [Token], and is returned by [Decoder.ReadToken].
//
// Use [Raw] to convert it to [Token] for [Encoder.WriteToken].
type RawToken struct {
	nonComparable

	// dBuf contains a reference to the dBuf decode buffer.
	// It is only valid if num == dBuf.previousOffsetStart().
	dBuf *decodeBuffer
	num  uint64
}

func (t RawToken) isRaw() bool {
	return t.dBuf.buf != nil
}

func (t RawToken) ensureValid() {
	if uint64(t.dBuf.previousOffsetStart()) != t.num {
		panic("invalid jsontext.Token; it has been voided by a subsequent json.Decoder call")
	}
}

// Token represents a lexical JSON token, which may be one of the following:
//   - a JSON literal (i.e., null, true, or false)
//   - a JSON string (e.g., "hello, world!")
//   - a JSON number (e.g., 123.456)
//   - a start or end delimiter for a JSON object (i.e., { or } )
//   - a start or end delimiter for a JSON array (i.e., [ or ] )
//
// A Token cannot represent entire array or object values, while a [Value] can.
// There is no Token to represent commas and colons since
// these structural tokens can be inferred from the surrounding context.
type Token struct {
	nonComparable

	// Tokens can exist in either a "raw" or an "exact" form.
	// Tokens produced by the Decoder are in the "raw" form.
	// Tokens returned by constructors are in the "exact" form.
	// The Encoder accepts Tokens in either the "raw" or "exact" form.

	// raw may contains a valid RawToken if raw.dBuf.buf is non-nil.
	//
	// If raw.dBuf equals to floatTag, intTag, or, uintTag,
	// the token is a JSON number in the "exact" form and
	// raw.num should be interpreted as a float64, int64, or uint64, respectively.
	//
	// If raw.dBuf equals to strTag, the token is a JSON string in the "string" form and
	// str is the unescaped JSON string.
	raw RawToken

	// str is the unescaped JSON string
	str string
}

// TODO: Does representing 1-byte delimiters as *decodeBuffer cause performance issues?

var (
	Null  Token = rawToken("null")
	False Token = rawToken("false")
	True  Token = rawToken("true")

	ObjectStart Token = rawToken("{")
	ObjectEnd   Token = rawToken("}")
	ArrayStart  Token = rawToken("[")
	ArrayEnd    Token = rawToken("]")

	nanString  = "NaN"
	pinfString = "Infinity"
	ninfString = "-Infinity"
)

func rawToken(s string) Token {
	return Token{raw: RawToken{
		dBuf: &decodeBuffer{buf: []byte(s), prevStart: 0, prevEnd: len(s)},
	}}
}

// Bool constructs a Token representing a JSON boolean.
func Bool(b bool) Token {
	if b {
		return True
	}
	return False
}

// String constructs a Token representing a JSON string.
// The provided string should contain valid UTF-8, otherwise invalid characters
// may be mangled as the Unicode replacement character.
func String(s string) Token {
	return Token{
		raw: RawToken{dBuf: strTag},
		str: s,
	}
}

// Float constructs a Token representing a JSON number.
// The values NaN, +Inf, and -Inf will be represented
// as a JSON string with the values "NaN", "Infinity", and "-Infinity",
// but still has kind '0' and cannot be used as object keys.
func Float(n float64) Token {
	return Token{
		raw: RawToken{dBuf: floatTag, num: math.Float64bits(n)},
	}
}

// Int constructs a Token representing a JSON number from an int64.
func Int(n int64) Token {
	return Token{
		raw: RawToken{dBuf: intTag, num: uint64(n)},
	}
}

// Uint constructs a Token representing a JSON number from a uint64.
func Uint(n uint64) Token {
	return Token{
		raw: RawToken{dBuf: uintTag, num: n},
	}
}

func Raw(t RawToken) Token {
	return Token{raw: t}
}

// Clone makes a copy of the Token such that its value remains valid
// even after a subsequent [Decoder.Read] call.
func (t RawToken) Clone() RawToken {
	if t.dBuf == nil {
		return t // zero value
	}
	// TODO: Allow caller to avoid any allocations?
	// Avoid copying globals.
	if t.dBuf.prevStart == 0 {
		switch t.dBuf {
		case Null.raw.dBuf:
			return Null.raw
		case False.raw.dBuf:
			return False.raw
		case True.raw.dBuf:
			return True.raw
		case ObjectStart.raw.dBuf:
			return ObjectStart.raw
		case ObjectEnd.raw.dBuf:
			return ObjectEnd.raw
		case ArrayStart.raw.dBuf:
			return ArrayStart.raw
		case ArrayEnd.raw.dBuf:
			return ArrayEnd.raw
		}
	}

	t.ensureValid()
	buf := bytes.Clone(t.dBuf.previousBuffer())
	return RawToken{dBuf: &decodeBuffer{buf: buf, prevStart: 0, prevEnd: len(buf)}}

}

// Clone makes a copy of the Token such that its value remains valid
// even after a subsequent [Decoder.Read] call.
func (t Token) Clone() Token {
	if t.raw.dBuf == nil {
		return t // zero value
	}
	if t.raw.isRaw() {
		return Token{raw: t.raw.Clone()}
	}
	return t // exact form.
}

// Bool returns the value for a JSON boolean.
// It panics if the token kind is not a JSON boolean.
func (t RawToken) Bool() bool {
	switch t.dBuf {
	case True.raw.dBuf:
		return true
	case False.raw.dBuf:
		return false
	default:
		panic("invalid JSON token kind: " + t.Kind().String())
	}
}

func (t Token) Bool() bool {
	return t.raw.Bool()
}

// appendString appends a JSON string to dst and returns it.
// It panics if t is not a JSON string.
func (t Token) appendString(dst []byte, flags *jsonflags.Flags) ([]byte, error) {
	if t.raw.isRaw() {
		// TODO: ensure vaild?
		// Handle raw string value.
		buf := t.raw.dBuf.previousBuffer()
		if Kind(buf[0]) == '"' {
			if jsonwire.ConsumeSimpleString(buf) == len(buf) {
				return append(dst, buf...), nil
			}
			dst, _, err := jsonwire.ReformatString(dst, buf, flags)
			return dst, err
		}
	} else if t.raw.dBuf == strTag {
		// Handle exact string value.
		return jsonwire.AppendQuote(dst, t.str, flags)
	}

	panic("invalid JSON token kind: " + t.Kind().String())
}

// String returns the unescaped string value for a JSON string.
// For other JSON kinds, this returns the raw JSON representation.
func (t RawToken) String() string {
	return string(t.bytes())
}

func (t RawToken) bytes() []byte {
	if t.dBuf == nil {
		return []byte("<invalid jsontext.RawToken>")
	}
	t.ensureValid()
	buf := t.dBuf.previousBuffer()
	if buf[0] == '"' {
		// TODO: Preserve ValueFlags in Token?
		isVerbatim := jsonwire.ConsumeSimpleString(buf) == len(buf)
		return jsonwire.UnquoteMayCopy(buf, isVerbatim)
	}
	// Handle tokens that are not JSON strings for fmt.Stringer.
	return buf
}

// String returns the unescaped string value for a JSON string.
// For other JSON kinds, this returns the raw JSON representation.
func (t Token) String() string {
	// This is inlinable to take advantage of "function outlining".
	// This avoids an allocation for the string(b) conversion
	// if the caller does not use the string in an escaping manner.
	// See https://blog.filippo.io/efficient-go-apis-with-the-inliner/
	s, b := t.string()
	if len(b) > 0 {
		return string(b)
	}
	return s
}

func (t Token) string() (string, []byte) {
	// Handle tokens that are not JSON strings for fmt.Stringer.
	switch t.raw.dBuf {
	case strTag:
		return t.str, nil
	case nil:
		return "<invalid jsontext.Token>", nil
	case floatTag:
		v := math.Float64frombits(t.raw.num)
		switch {
		case math.IsNaN(v):
			return nanString, nil
		case math.IsInf(v, +1):
			return pinfString, nil
		case math.IsInf(v, -1):
			return ninfString, nil
		default:
			return string(jsonwire.AppendFloat(nil, math.Float64frombits(t.raw.num), 64)), nil
		}
	case intTag:
		return strconv.FormatInt(int64(t.raw.num), 10), nil
	case uintTag:
		return strconv.FormatUint(uint64(t.raw.num), 10), nil
	}
	if t.raw.isRaw() {
		return "", t.raw.bytes()
	}
	return "<invalid jsontext.Token>", nil
}

// appendNumber appends a JSON number to dst and returns it.
// It panics if t is not a JSON number.
func (t Token) appendNumber(dst []byte, flags *jsonflags.Flags) ([]byte, error) {
	if t.raw.isRaw() {
		// Handle raw number value.
		buf := t.raw.dBuf.previousBuffer()
		if Kind(buf[0]).normalize() == '0' {
			dst, _, err := jsonwire.ReformatNumber(dst, buf, flags)
			return dst, err
		}
	} else {
		// Handle exact number value.
		switch t.raw.dBuf {
		case floatTag:
			v := math.Float64frombits(t.raw.num)
			switch {
			case math.IsNaN(v):
				return jsonwire.AppendQuote(dst, nanString, flags)
			case math.IsInf(v, +1):
				return jsonwire.AppendQuote(dst, pinfString, flags)
			case math.IsInf(v, -1):
				return jsonwire.AppendQuote(dst, ninfString, flags)
			default:
				return jsonwire.AppendFloat(dst, v, 64), nil
			}
		case intTag:
			return strconv.AppendInt(dst, int64(t.raw.num), 10), nil
		case uintTag:
			return strconv.AppendUint(dst, uint64(t.raw.num), 10), nil
		}
	}

	panic("invalid JSON token kind: " + t.Kind().String())
}

var ErrUnexpectedKind = errors.New("unexpected JSON token kind")

// ParseFloat parses the floating-point value for a JSON number.
// It returns a NaN, +Inf, or -Inf value for any JSON string
// with the values "NaN", "Infinity", or "-Infinity".
func (t RawToken) ParseFloat(bits int) (float64, error) {
	t.ensureValid()
	buf := t.dBuf.previousBuffer()
	if Kind(buf[0]).normalize() == '0' {
		return jsonwire.ParseFloat(buf, bits)
	}

	if buf[0] == '"' {
		switch t.String() {
		case nanString:
			return math.NaN(), nil
		case pinfString:
			return math.Inf(+1), nil
		case ninfString:
			return math.Inf(-1), nil
		}
	}

	return 0., ErrUnexpectedKind
}

// Float returns the floating-point value for a JSON number.
// It panics if the token is not created with [Float].
func (t Token) Float() float64 {
	if t.raw.dBuf == floatTag {
		return math.Float64frombits(t.raw.num)
	}
	panic("JSON token not created with Float")
}

func (t RawToken) ParseInt(bits int) (int64, error) {
	t.ensureValid()
	buf := t.dBuf.previousBuffer()
	if Kind(buf[0]).normalize() == '0' {
		return jsonwire.ParseInt(buf, bits)
	}
	return 0, ErrUnexpectedKind
}

// Int returns the signed integer value for a JSON number.
// It panics if the token is not created with [Int].
func (t Token) Int() int64 {
	if t.raw.dBuf == intTag {
		return int64(t.raw.num)
	}
	panic("JSON token not created with Int")
}

func (t RawToken) ParseUint(bits int) (uint64, error) {
	t.ensureValid()
	buf := t.dBuf.previousBuffer()
	if Kind(buf[0]).normalize() == '0' {
		return jsonwire.ParseUint(buf, bits)
	}
	return 0, ErrUnexpectedKind
}

// Uint returns the unsigned integer value for a JSON number.
// It panics if the token is not created with [Uint].
func (t Token) Uint() uint64 {
	if t.raw.dBuf == uintTag {
		return t.raw.num
	}
	panic("JSON token not created with Uint")
}

// Float returns the RawToken embedded.
// It panics if the token is not created with [Raw].
func (t Token) Raw() RawToken {
	if t.raw.isRaw() {
		return t.raw
	}
	panic("JSON token not created with Raw")
}

// Kind returns the token kind.
func (t RawToken) Kind() Kind {
	if t.dBuf == nil { // for zero value RawToken
		return invalidKind
	}
	t.ensureValid()
	return Kind(t.dBuf.buf[t.dBuf.prevStart]).normalize()
}

// Kind returns the token kind.
func (t Token) Kind() Kind {
	switch {
	case t.raw.dBuf == nil:
		return invalidKind // zero value Token
	case t.raw.isRaw():
		return t.raw.Kind()
	case t.raw.dBuf == intTag || t.raw.dBuf == uintTag || t.raw.dBuf == floatTag:
		// For NaN and Inf, we still return '0' as the Kind
		// even if it will be encoded as a string.
		// We don't want to use this for object key, right?
		return '0'
	case t.raw.dBuf == strTag:
		return '"'
	default:
		return invalidKind
	}
}

// Kind represents each possible JSON token kind with a single byte,
// which is conveniently the first byte of that kind's grammar
// with the restriction that numbers always be represented with '0':
//
//   - 'n': null
//   - 'f': false
//   - 't': true
//   - '"': string
//   - '0': number
//   - '{': object start
//   - '}': object end
//   - '[': array start
//   - ']': array end
//
// An invalid kind is usually represented using 0,
// but may be non-zero due to invalid JSON data.
type Kind byte

const invalidKind Kind = 0

// String prints the kind in a humanly readable fashion.
func (k Kind) String() string {
	switch k {
	case 'n':
		return "null"
	case 'f':
		return "false"
	case 't':
		return "true"
	case '"':
		return "string"
	case '0':
		return "number"
	case '{':
		return "{"
	case '}':
		return "}"
	case '[':
		return "["
	case ']':
		return "]"
	default:
		return "<invalid jsontext.Kind: " + jsonwire.QuoteRune(string(k)) + ">"
	}
}

// normalize coalesces all possible starting characters of a number as just '0'.
func (k Kind) normalize() Kind {
	if k == '-' || ('0' <= k && k <= '9') {
		return '0'
	}
	return k
}
