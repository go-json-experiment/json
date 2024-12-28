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
	"encoding"

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
	_ = encoding.TextMarshaler(nil)
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
//   - [CallMethodsWithLegacySemantics]
//   - [EscapeInvalidUTF8]
//   - [FormatBytesWithLegacySemantics]
//   - [FormatTimeDurationAsNanosecond]
//   - [IgnoreStructErrors]
//   - [MatchCaseSensitiveDelimiter]
//   - [MergeWithLegacySemantics]
//   - [OmitEmptyWithLegacyDefinition]
//   - [PreserveRawStrings]
//   - [RejectFloatOverflow]
//   - [ReportLegacyErrorValues]
//   - [StringifyWithLegacySemantics]
//   - [UnmarshalArrayFromAnyLength]
//   - [jsonv2.Deterministic]
//   - [jsonv2.FormatNilMapAsNull]
//   - [jsonv2.FormatNilSliceAsNull]
//   - [jsonv2.MatchCaseInsensitiveNames]
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

// CallMethodsWithLegacySemantics specifies that calling of type-provided
// marshal and unmarshal methods follow legacy semantics:
//
//   - When marshaling, a marshal method declared on a pointer receiver
//     is only called if the Go value is addressable.
//     Values obtained from an interface or map element are not addressable.
//     Values obtained from a pointer or slice element are addressable.
//     Values obtained from an array element or struct field inherit
//     the addressability of the parent. In contrast, the v2 semantic
//     is to always call marshal methods regardless of addressability.
//
//   - When marshaling or unmarshaling, the [Marshaler] or [Unmarshaler]
//     methods are ignored for map keys. However, [encoding.TextMarshaler]
//     or [encoding.TextUnmarshaler] are still callable.
//     In contrast, the v2 semantic is to serialize map keys
//     like any other value (with regard to calling methods),
//     which may include calling [Marshaler] or [Unmarshaler] methods,
//     where it is the implementation's responsibility to represent the
//     Go value as a JSON string (as required for JSON object names).
//
//   - When marshaling, if a map key value implements a marshal method
//     and is a nil pointer, then it is serialized as an empty JSON string.
//     In contrast, the v2 semantic is to report an error.
//
//   - When marshaling, if an interface type implements a marshal method
//     and the interface value is a nil pointer to a concrete type,
//     then the marshal method is always called.
//     In contrast, the v2 semantic is to never directly call methods
//     on interface values and to instead defer evaluation based upon
//     the underlying concrete value. Similar to non-interface values,
//     marshal methods are not called on nil pointers and
//     are instead serialized as a JSON null.
//
// This affects either marshaling or unmarshaling.
// The v1 default is true.
func CallMethodsWithLegacySemantics(v bool) Options {
	if v {
		return jsonflags.CallMethodsWithLegacySemantics | 1
	} else {
		return jsonflags.CallMethodsWithLegacySemantics | 0
	}
}

// EscapeInvalidUTF8 specifies that bytes of invalid UTF-8 within JSON strings
// should be escaped as a hexadecimal Unicode codepoint (i.e., \ufffd)
// of the Unicode replacement character as opposed to being encoded
// as the Unicode replacement character verbatim (without escaping).
// This option has no effect if [jsontext.AllowInvalidUTF8] is false.
//
// This only affects encoding and is ignored when decoding.
// The v1 default is true.
func EscapeInvalidUTF8(v bool) Options {
	if v {
		return jsonflags.EscapeInvalidUTF8 | 1
	} else {
		return jsonflags.EscapeInvalidUTF8 | 0
	}
}

// FormatBytesWithLegacySemantics specifies that handling of
// []~byte and [N]~byte types follow legacy semantics:
//
//   - A Go [N]~byte is always treated as as a normal Go array
//     in contrast to the v2 default of treating [N]byte as
//     using some form of binary data encoding (RFC 4648).
//
//   - A Go []~byte is to be treated as using some form of
//     binary data encoding (RFC 4648) in contrast to the v2 default
//     of only treating []byte as such. In particular, v2 does not
//     treat slices of named byte types as representing binary data.
//
//   - When marshaling, if a named byte implements a marshal method,
//     then the slice is serialized as a JSON array of elements,
//     each of which call the marshal method.
//
//   - When unmarshaling, if the input is a JSON array,
//     then unmarshal into the []~byte as if it were a normal Go slice.
//     In contrast, the v2 default is to report an error unmarshaling
//     a JSON array when expecting some form of binary data encoding.
//
// This affects either marshaling or unmarshaling.
// The v1 default is true.
func FormatBytesWithLegacySemantics(v bool) Options {
	if v {
		return jsonflags.FormatBytesWithLegacySemantics | 1
	} else {
		return jsonflags.FormatBytesWithLegacySemantics | 0
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

// IgnoreStructErrors specifies that a Go struct with structural errors
// should not result in a runtime error when marshaling or unmarshaling.
// Such errors usually occur because of a malformed struct field tag
// as it pertains to JSON serialization.
//
// This affects either marshaling or unmarshaling.
// The v1 default is true.
func IgnoreStructErrors(v bool) Options {
	if v {
		return jsonflags.IgnoreStructErrors | 1
	} else {
		return jsonflags.IgnoreStructErrors | 0
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

// MergeWithLegacySemantics specifies that unmarshaling into a non-zero
// Go value follows legacy semantics:
//
//   - When unmarshaling a JSON null, this preserves the original Go value
//     if the kind is a bool, int, uint, float, string, array, or struct.
//     Otherwise, it zeros the Go value.
//     In contrast, the default v2 behavior is to consistently and always
//     zero the Go value when unmarshaling a JSON null into it.
//
//   - When unmarshaling a JSON value other than null, this merges into
//     the original Go value for array elements, slice elements,
//     struct fields (but not map values),
//     pointer values, and interface values (only if a non-nil pointer).
//     In contrast, the default v2 behavior is to merge into the Go value
//     for struct fields, map values, pointer values, and interface values.
//     In general, the v2 semantic merges when unmarshaling a JSON object,
//     otherwise it replaces the original value.
//
// This only affects unmarshaling and is ignored when marshaling.
// The v1 default is true.
func MergeWithLegacySemantics(v bool) Options {
	if v {
		return jsonflags.MergeWithLegacySemantics | 1
	} else {
		return jsonflags.MergeWithLegacySemantics | 0
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

// PreserveRawStrings specifies that raw JSON string values passed to
// [jsontext.Encoder.WriteValue] and [jsontext.Encoder.WriteToken]
// preserve their original encoding.
// However, characters that still need escaping according to
// [jsontext.EscapeForHTML] and [jsontext.EscapeForJS] are escaped.
//
// This only affects encoding and is ignored when decoding.
// The v1 default is true.
func PreserveRawStrings(v bool) Options {
	if v {
		return jsonflags.PreserveRawStrings | 1
	} else {
		return jsonflags.PreserveRawStrings | 0
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

// ReportLegacyErrorValues specifies that Marshal and Unmarshal should
// return legacy error values such as [SyntaxError], [MarshalerError],
// [UnsupportedTypeError], [UnsupportedValueError],
// [InvalidUnmarshalError], or [UnmarshalTypeError] instead of the
// [jsonv2.SemanticError] or [jsontext.SyntacticError].
//
// This affects either marshaling or unmarshaling.
// The v1 default is true.
func ReportLegacyErrorValues(v bool) Options {
	if v {
		return jsonflags.ReportLegacyErrorValues | 1
	} else {
		return jsonflags.ReportLegacyErrorValues | 0
	}
}

// StringifyWithLegacySemantics specifies that the `string` tag option
// may stringify bools and string values. It only takes effect on fields
// where the top-level type is a bool, string, numeric kind, or a pointer to
// such a kind. Specifically, `string` will not stringify bool, string,
// or numeric kinds within a composite data type
// (e.g., array, slice, struct, map, or interface).
//
// When marshaling, such Go values are serialized as their usual
// JSON representation, but quoted within a JSON string.
// When unmarshaling, such Go values must be deserialized from
// a JSON string containing their usual JSON representation.
// A JSON null quoted in a JSON string is a valid substitute for JSON null
// while unmarshaling into a Go value that `string` takes effect on.
//
// This affects either marshaling or unmarshaling.
// The v1 default is true.
func StringifyWithLegacySemantics(v bool) Options {
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
