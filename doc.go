// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package json implements serialization of JSON
// as specified in RFC 4627, RFC 7159, RFC 7493, RFC 8259, and RFC 8785.
// JSON is a simple data interchange format that can represent
// primitive data types such as booleans, strings, and numbers,
// in addition to structured data types such as objects and arrays.
//
// # Terminology
//
// This package uses the terms "encode" and "decode" for syntactic functionality
// that is concerned with processing JSON based on its grammar, and
// uses the terms "marshal" and "unmarshal" for semantic functionality
// that determines the meaning of JSON values as Go values and vice-versa.
// It aims to provide a clear distinction between functionality that
// is purely concerned with encoding versus that of marshaling.
// For example, one can directly encode a stream of JSON tokens without
// needing to marshal a concrete Go value representing them.
// Similarly, one can decode a stream of JSON tokens without
// needing to unmarshal them into a concrete Go value.
//
// This package uses JSON terminology when discussing JSON, which may differ
// from related concepts in Go or elsewhere in computing literature.
//
//   - A JSON "object" refers to an unordered collection of name/value members;
//   - a JSON "array" refers to an ordered sequence of elements; and
//   - a JSON "value" refers to either a literal (i.e., null, false, or true),
//     string, number, object, or array.
//
// See RFC 8259 for more information.
//
// # Specifications
//
// Relevant specifications include RFC 4627, RFC 7159, RFC 7493, RFC 8259,
// and RFC 8785. Each RFC is generally a stricter subset of another RFC.
// In increasing order of strictness:
//
//   - RFC 4627 and RFC 7159 do not require (but recommend) the use of UTF-8
//     and also do not require (but recommend) that object names be unique.
//   - RFC 8259 requires the use of UTF-8,
//     but does not require (but recommends) that object names be unique.
//   - RFC 7493 requires the use of UTF-8
//     and also requires that object names be unique.
//   - RFC 8785 defines a canonical representation. It requires the use of UTF-8
//     and also requires that object names be unique and in a specific ordering.
//     It specifies exactly how strings and numbers must be formatted.
//
// The primary difference between RFC 4627 and RFC 7159 is that the former
// restricted top-level values to only JSON objects and arrays, while
// RFC 7159 and subsequent RFCs permit top-level values to additionally be
// JSON nulls, booleans, strings, or numbers.
//
// By default, this package operates on RFC 7493, but can be configured
// to operate according to the other RFC specifications.
// RFC 7493 is a stricter subset of RFC 8259 and fully compliant with it.
// In particular, it makes specific choices about behavior that RFC 8259
// leaves as undefined in order to ensure greater interoperability.
//
// JSON produced by Marshal functionality is not guaranteed to be deterministic.
// To obtain more stable JSON output, consider using RawValue.Canonicalize.
package json

// requireKeyedLiterals can be embedded in a struct to require keyed literals.
type requireKeyedLiterals struct{}

// nonComparable can be embedded in a struct to prevent comparability.
type nonComparable [0]func()
