// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package json implements legacy support for v1 [encoding/json].
//
// The entirety of the v1 API will eventually live in this package.
// For the time being, it only contains options needed to configure the v2 API
// to operate under v1 semantics.
package json

import (
	jsonv2 "github.com/go-json-experiment/json"
	"github.com/go-json-experiment/json/internal/jsonflags"
	"github.com/go-json-experiment/json/internal/jsonopts"
	"github.com/go-json-experiment/json/jsontext"
)

// Reference the jsonv2 and jsontext packages to assist pkgsite
// in being able to hotlink  references to those packages.
var (
	_ = jsonv2.Deterministic
	_ = jsontext.AllowDuplicateNames
)

// Options are a set of options to configure the v2 "json" package
// to operate with v1 semantics for particular features.
// Instead of referencing this type, use ["encoding/json/v2".Options].
type Options = jsonopts.Options

// DefaultOptionsV1 is the full set of all options that define v1 semantics.
// It is equivalent to the following boolean options being set to true:
//
//   - [FormatByteArrayAsArray]
//   - [FormatTimeDurationAsNanosecond]
//   - [OmitEmptyWithLegacyDefinition]
//   - [jsonv2.Deterministic]
//   - [jsonv2.FormatNilSliceAsNull]
//   - [jsonv2.FormatNilMapAsNull]
//   - [jsontext.AllowDuplicateNames]
//   - [jsontext.AllowInvalidUTF8]
//   - [jsontext.EscapeForHTML]
//   - [jsontext.EscapeForJS]
//
// All other boolean options are set to false.
// All non-boolean options are set to the zero value,
// except for [jsontext.WithIndent], which defaults to "\t".
func DefaultOptionsV1() Options {
	return &jsonopts.DefaultOptionsV1
}

// FormatByteArrayAsArray specifies that a [N]byte array is formatted
// by default as a JSON array of byte values in contrast to v2 default
// of using a JSON string with the base64 encoding of the value.
// If a [N]byte field has a `format` tag option,
// then the specified formatting takes precedence.
//
// This affects either marshaling or unmarshaling.
// The v1 default is true.
func FormatByteArrayAsArray(v bool) Options {
	if v {
		return jsonflags.FormatByteArrayAsArray | 1
	} else {
		return jsonflags.FormatByteArrayAsArray | 0
	}
}

// FormatTimeDurationAsNanosecond specifies that [time.Duration] is formatted
// by default as a JSON number representing the number of nanoseconds
// in contrast to the v2 default of using a JSON string with the duration
// formatted with [time.Duration.String].
// If a duration field has a `format` tag option,
// then the specified formatting takes precedence.
//
// This affects either marshaling or unmarshaling.
// The v1 default is true.
func FormatTimeDurationAsNanosecond(v bool) Options {
	if v {
		return jsonflags.FormatTimeDurationAsNanosecond | 1
	} else {
		return jsonflags.FormatTimeDurationAsNanosecond | 0
	}
}

// OmitEmptyWithLegacyDefinition specifies that the `omitempty` tag option
// follows a definition of empty where a field is omitted if the Go value is
// false, 0, a nil pointer, a nil interface value,
// or any empty array, slice, map, or string.
// This overrides the v2 semantic where a field is empty if the value
// marshals as a JSON null or an empty JSON string, object, or array.
//
// The v1 and v2 definitions of `omitempty` are practically the same for
// Go strings, slices, arrays, and maps. Usages of `omitempty` on
// Go bools, ints, uints floats, pointers, and interfaces should migrate to use
// the `omitzero` tag option, which omits a field if it is the zero Go value.
//
// This only affects marshaling and is ignored when unmarshaling.
// The v1 default is true.
func OmitEmptyWithLegacyDefinition(v bool) Options {
	if v {
		return jsonflags.OmitEmptyWithLegacyDefinition | 1
	} else {
		return jsonflags.OmitEmptyWithLegacyDefinition | 0
	}
}
