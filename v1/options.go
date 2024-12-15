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
// Values of this type can be passed to v2 functions like
// [jsonv2.Marshal] or [jsonv2.Unmarshal].
// Instead of referencing this type, use ["encoding/json/v2".Options].
type Options = jsonopts.Options

// DefaultOptionsV1 is the full set of all options that define v1 semantics.
// It is equivalent to the following boolean options being set to true:
//
//   - [FormatByteArrayAsArray]
//   - [FormatTimeDurationAsNanosecond]
//   - [MatchCaseSensitiveDelimiter]
//   - [OmitEmptyWithLegacyDefinition]
//   - [RejectFloatOverflow]
//   - [StringifyWithLegacySemantics]
//   - [UnmarshalArrayFromAnyLength]
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
//
// The [Marshal] and [Unmarshal] functions in this package are
// semantically identical to calling the v2 equivalents with this option:
//
//	jsonv2.Marshal(v, jsonv1.DefaultOptionsV1())
//	jsonv2.Unmarshal(b, v, jsonv1.DefaultOptionsV1())
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

// MatchCaseSensitiveDelimiter specifies that underscores and dashes are
// not to be ignored when performing case-insensitive name matching which
// occurs under [jsonv2.MatchCaseInsensitiveNames] or the `nocase` tag option.
// Thus, case-insensitive name matching is identical to [strings.EqualFold].
// Use of this option diminishes the ability of case-insensitive matching
// to be able to match common case variants (e.g, "foo_bar" with "fooBar").
//
// This affects either marshaling or unmarshaling.
// The v1 default is true.
func MatchCaseSensitiveDelimiter(v bool) Options {
	if v {
		return jsonflags.MatchCaseSensitiveDelimiter | 1
	} else {
		return jsonflags.MatchCaseSensitiveDelimiter | 0
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

// RejectFloatOverflow specifies that unmarshaling a JSON number that
// exceeds the maximum representation of a Go float32 or float64
// results in an error, rather than succeeding with the floating-point values
// set to either [math.MaxFloat32] or [math.MaxFloat64].
//
// This only affects unmarshaling and is ignored when marshaling.
// The v1 default is true.
func RejectFloatOverflow(v bool) Options {
	if v {
		return jsonflags.RejectFloatOverflow | 1
	} else {
		return jsonflags.RejectFloatOverflow | 0
	}
}

// StringifyWithLegacySemantics specifies that the `string` tag option
// may stringify bools and string values. It only takes effect on fields
// where the top-level type is a bool, string, numeric kind, or a pointer to
// such a kind. Specifically, `string` will not stringify bool, string,
// or numeric kinds within a composite data type
// (e.g., array, slice, struct, map, or interface).
//
// This affects either marshaling or unmarshaling.
// The v1 default is true.
func StringifyWithLegacySemantics(v bool) Options {
	// TODO: In v1, we would permit unmarshaling "null" (i.e., a quoted null)
	// as if it were just null. We do not support this in v2. Should we?
	if v {
		return jsonflags.StringifyWithLegacySemantics | 1
	} else {
		return jsonflags.StringifyWithLegacySemantics | 0
	}
}

// UnmarshalArrayFromAnyLength specifies that Go arrays can be unmarshaled
// from input JSON arrays of any length. If the JSON array is too short,
// then the remaining Go array elements are zeroed. If the JSON array
// is too long, then the excess JSON array elements are skipped over.
//
// This only affects unmarshaling and is ignored when marshaling.
// The v1 default is true.
func UnmarshalArrayFromAnyLength(v bool) Options {
	if v {
		return jsonflags.UnmarshalArrayFromAnyLength | 1
	} else {
		return jsonflags.UnmarshalArrayFromAnyLength | 0
	}
}

// unmarshalAnyWithRawNumber specifies that unmarshaling a JSON number into
// an empty Go interface should use the Number type instead of a float64.
func unmarshalAnyWithRawNumber(v bool) Options {
	if v {
		return jsonflags.UnmarshalAnyWithRawNumber | 1
	} else {
		return jsonflags.UnmarshalAnyWithRawNumber | 0
	}
}
