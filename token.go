// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

// NOTE: Token is analogous to v1 json.Token.

// Token represents a lexical JSON token, which may be one of the following:
//	• a JSON literal (i.e., null, true, or false)
//	• a JSON string (e.g., "hello, world!")
//	• a JSON number (e.g., 123.456)
//	• a start or end delimiter for a JSON object (i.e., { or } )
//	• a start or end delimiter for a JSON array (i.e., [ or ] )
//
// A Token cannot represent entire array or object values, while a RawValue can.
// There is no Token to represent commas and colons since
// these structural tokens can be inferred from the surrounding context.
type Token struct {
	// NOTE: This is an opaque type that functionally represents a union type.
	// We use a concrete struct type instead of an interface type to have
	// fine granularity control over allocations and mutations.
	//
	// If unsafe were used, the size of this could be reduced to 24B.
	// See "google.golang.org/protobuf/reflect/protoreflect".Value for example.
	//
	// Using an opaque, concrete type resolves:
	//	https://golang.org/issue/40127
	//	https://golang.org/issue/40128
}

var (
	Null  Token // TODO: Make this the zero value of Token
	True  Token
	False Token

	ObjectStart Token
	ObjectEnd   Token
	ArrayStart  Token
	ArrayEnd    Token
)

// Bool constructs a Token representing a JSON boolean.
func Bool(b bool) Token {
	panic("not implemented")
}

// String construct a Token representing a JSON string.
// The provided string should contain valid UTF-8, otherwise invalid characters
// may be mangled as the Unicode replacement character.
func String(s string) Token {
	panic("not implemented")
}

// Float constructs a Token representing a JSON number.
// The values NaN, +Inf, and -Inf will be represented
// as a JSON string with the values "NaN", "Infinity", and "-Infinity".
func Float(n float64) Token {
	panic("not implemented")
}

// Int constructs a Token representing a JSON number from an int64.
func Int(n int64) Token {
	panic("not implemented")
}

// Uint constructs a Token representing a JSON number from a uint64.
func Uint(n uint64) Token {
	panic("not implemented")
}

// Bool returns the value for a JSON boolean.
// For other JSON kinds, this returns false.
func (t Token) Bool() bool {
	panic("not implemented")
}

// String returns the unescaped string value for a JSON string.
// For other JSON kinds, this returns the raw JSON represention.
func (t Token) String() string {
	panic("not implemented")
}

// Float returns the floating-point value for a JSON number.
// It returns a NaN, +Inf, or -Inf value for any JSON string
// with the values "NaN", "Infinity", or "-Infinity".
// It returns 0 for all other cases.
func (t Token) Float() float64 {
	panic("not implemented")
}

// Int returns the signed integer value for a JSON number.
// The fractional component of any number is ignored (truncation toward zero).
// Any number beyond the representation of an int64 will be saturated
// to the closest representable value.
// It returns math.MaxInt64 or math.MinInt64 for any JSON string
// with the values "Infinity" or "-Infinity".
// It returns 0 for all other cases.
func (t Token) Int() int64 {
	panic("not implemented")
}

// Uint returns the unsigned integer value for a JSON number.
// The fractional component of any number is ignored (truncation toward zero).
// Any number beyond the representation of an int64 will be saturated
// to the closest representable value.
// It returns math.MaxUint64 for a JSON string with the value "Infinity".
// It returns 0 for all other cases.
func (t Token) Uint() uint64 {
	panic("not implemented")
}

// Kind returns the token kind.
func (t Token) Kind() Kind {
	panic("not implemented")
}

// Clone makes a copy of the Token such that its value remains valid
// even after a subsequent Decoder.Read call.
func (t Token) Clone() Token {
	panic("not implemented")
}

// Kind represents each possible JSON token kind with a single byte,
// which is conveniently the first byte of that kind's grammar
// with the restriction that numbers always be represented with '0':
//
//	• 'n': null
//	• 'f': false
//	• 't': true
//	• '"': string
//	• '0': number
//	• '{': object start
//	• '}': object end
//	• '[': array start
//	• ']': array end
//
// An invalid kind is usually represented using 0,
// but may be non-zero due to invalid JSON data.
type Kind byte

// String prints the kind in a humanly readable fashion.
func (k Kind) String() string {
	panic("not implemented")
}
