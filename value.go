// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import "errors"

// NOTE: RawValue is analogous to v1 json.RawMessage.

// RawValue represents a single raw JSON value, which may be one of the following:
//	• a JSON literal (i.e., null, true, or false)
//	• a JSON string (e.g., "hello, world!")
//	• a JSON number (e.g., 123.456)
//	• an entire JSON object (e.g., {"fizz":"buzz"} )
//	• an entire JSON array (e.g., [1,2,3] )
//
// RawValue can represent entire array or object values, while Token cannot.
type RawValue []byte

// IsValid reports whether the raw JSON value is syntactically valid
// according to RFC 7159, section 2.
//
// Of particular note, it does not verify whether an object has duplicate names
// or whether numbers are representable within the limits
// of any common numeric type (e.g., float64, int64, or uint64).
func (v RawValue) IsValid() bool {
	// NOTE: This is equivalent to v1 json.Valid.
	// TODO: Should this validate for invalid UTF-8 or not? v1 did not.
	//	Note that the ABNF in RFC 7159 does not explicitly forbid unpaired
	//	surrogate halves, but warns against them in section 8.2.
	//	This affects the documentation on Canonicalize regarding whether
	//	Canonicalize is guaranteed to succeed if IsValid reports true.
	panic("not implemented")
}

// Compact removes all whitespace from the raw JSON value.
// It does not rewrite JSON strings to use their minimal representation.
//
// It is guaranteed to succeed if the input is valid.
func (v *RawValue) Compact() error {
	// NOTE: This is equivalent to v1 json.Compact.
	// TODO: Should this mutate b in-place or should it return a new RawValue?
	//	• It is possibly more performant to mutate b in-place
	//	  if we can reuse the buffer and not allocate a new one.
	//	• It is assumed that a user rarely wants to use the non-compacted from
	//	  form after calling this method.
	// TODO: This can't be implemented in with an Encoder since json.Compact
	//	preserves string formatting exactly as is. Consider dropping this method
	//	if we had an option to preserve JSON strings verbatim.
	panic("not implemented")
}

// Indent reformats the whitespace in the raw JSON value so that each element
// in a JSON object or array begins on a new, indented line beginning with
// prefix followed by one or more copies of indent according to the nesting.
// The value does not begin with the prefix nor any indention,
// to make it easier to embed inside other formatted JSON data.
//
// It is guaranteed to succeed if the input is valid.
func (v *RawValue) Indent(prefix, indent string) error {
	// NOTE: This is equivalent to v1 json.Indent.
	// TODO: The v1 json.Indent allows any character,
	//	which would produce invalid JSON output.
	//	Such behavior is undocumented and probably a bug?
	// TODO: The v1 json.Indent would preserve any trailing whitespace.
	//	Is this behavior that we want to preserve in v2?
	// TODO: Should this mutate b in-place or should it return a new RawValue?
	// TODO: This can't be implemented in with an Encoder since json.Indent
	//	preserves string formatting exactly as is. Consider dropping this method
	//	if we had an option to preserve JSON strings verbatim.
	panic("not implemented")
}

// Canonicalize canonicalizes the raw JSON value according to
// JSON Canonicalization Scheme (JCS) as defined by RFC 8785.
// The output stability is dependent on the stability of the application data
// (see RFC 8785, Appendix E).
//
// It is guaranteed to succeed if the input is valid
// and the output is guaranteed to also be compact.
func (v *RawValue) Canonicalize() error {
	// TODO: Should this mutate b in-place or should it return a new RawValue?
	panic("not implemented")
}

// TODO: Instead of implementing v1 Marshaler/Unmarshaler,
// consider implementing the v2 versions instead.

// MarshalJSON returns v as the JSON encoding of v.
// It returns the stored value as the raw JSON output without any validation.
// If v is nil, then this returns a JSON null.
func (v RawValue) MarshalJSON() ([]byte, error) {
	// NOTE: This matches the behavior of v1 json.RawMessage.MarshalJSON.
	if v == nil {
		return []byte("null"), nil
	}
	return v, nil
}

// UnmarshalJSON sets v as the JSON encoding of b.
// It stores a copy of the provided raw JSON input without any validation.
func (v *RawValue) UnmarshalJSON(b []byte) error {
	// NOTE: This matches the behavior of v1 json.RawMessage.UnmarshalJSON.
	if v == nil {
		return errors.New("json.RawValue: UnmarshalJSON on nil pointer")
	}
	*v = append((*v)[:0], b...)
	return nil
}

// Kind returns the starting token kind.
func (v RawValue) Kind() Kind {
	if v := v[consumeWhitespace(v):]; len(v) > 0 {
		return Kind(v[0]).normalize()
	}
	return invalidKind
}
