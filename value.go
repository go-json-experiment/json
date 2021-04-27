// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import (
	"bytes"
	"errors"
	"io"
	"sort"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

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
// according to RFC 8259, section 2.
//
// It does not verify whether an object has duplicate names or
// whether numbers are representable within the limits
// of any common numeric type (e.g., float64, int64, or uint64).
// It does verify that the input is properly encoded as UTF-8 and
// that escape sequences within strings decode to valid Unicode codepoints.
func (v RawValue) IsValid() bool {
	d := new(Decoder) // TODO: Pool this.
	d.state.init()
	d.buf = v
	_, errVal := d.ReadValue()
	_, errEOF := d.ReadToken()
	return errVal == nil && errEOF == io.EOF
}

// Compact removes all whitespace from the raw JSON value.
//
// It does not reformat JSON strings to use any other representation.
// It is guaranteed to succeed if the input is valid.
// If the value is already compact, then the buffer is not mutated.
func (v *RawValue) Compact() error {
	return v.reformat(false, false, "", "")
}

// Indent reformats the whitespace in the raw JSON value so that each element
// in a JSON object or array begins on a new, indented line beginning with
// prefix followed by one or more copies of indent according to the nesting.
// The value does not begin with the prefix nor any indention,
// to make it easier to embed inside other formatted JSON data.
//
// It does not reformat JSON strings to use any other representation.
// It is guaranteed to succeed if the input is valid.
// If the value is already indented properly, then the buffer is not mutated.
func (v *RawValue) Indent(prefix, indent string) error {
	return v.reformat(false, true, prefix, indent)
}

// Canonicalize canonicalizes the raw JSON value according to the
// JSON Canonicalization Scheme (JCS) as defined by RFC 8785
// where it produces a stable representation of a JSON value.
//
// The output stability is dependent on the stability of the application data
// (see RFC 8785, Appendix E). It cannot produce stable output from
// fundamentally unstable input. For example, if the JSON value
// contains ephemeral data (e.g., a frequently changing timestamp),
// then the value is still unstable regardless of whether this is called.
//
// Note that JCS treats all JSON numbers as IEEE 754 double precision numbers.
// Any numbers with precision beyond what is representable by that form
// will lose their precision when canonicalized. For example, integer values
// beyond ±2⁵³ will lose their precision. It is recommended that
// int64 and uint64 data types be represented as a JSON string.
//
// It is possible that Canonicalize reports an error even if the value is valid
// according to IsValid (which only validates against RFC 8259).
// JCS only operates on input that is compliant with RFC 7493,
// which requires that JSON objects must not have duplicate member names.
// If the value is already canonicalized, then the buffer is not mutated.
func (v *RawValue) Canonicalize() error {
	return v.reformat(true, false, "", "")
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

func (v *RawValue) reformat(canonical, multiline bool, prefix, indent string) error {
	e := new(Encoder) // TODO: Pool this.
	e.state.init()

	if canonical {
		e.options.AllowInvalidUTF8 = false    // per RFC 8785, section 3.2.4
		e.options.RejectDuplicateNames = true // per RFC 8785, section 3.1
		e.options.canonicalizeNumbers = true  // per RFC 8785, section 3.2.2.3
		e.options.EscapeRune = nil            // per RFC 8785, section 3.2.2.2
		e.options.multiline = false           // per RFC 8785, section 3.2.1
	} else {
		if s := strings.Trim(prefix, " \t"); len(s) > 0 {
			panic("json: invalid character " + escapeCharacter(s[0]) + " in indent prefix")
		}
		if s := strings.Trim(indent, " \t"); len(s) > 0 {
			panic("json: invalid character " + escapeCharacter(s[0]) + " in indent")
		}
		e.options.AllowInvalidUTF8 = true
		e.options.preserveRawStrings = true
		e.options.multiline = multiline // in case indent is empty
		e.options.IndentPrefix = prefix
		e.options.Indent = indent
	}
	e.options.omitTopLevelNewline = true

	// Write the entire value to reformat all tokens and whitespace.
	if err := e.WriteValue(*v); err != nil {
		return err
	}

	// For canonical output, may need to reorder object members.
	if canonical {
		RawValue(e.buf).reorderObjects(nil) // per RFC 8785, section 3.2.3
	}

	// Store the result back into the value if different.
	if !bytes.Equal(*v, e.buf) {
		*v = append((*v)[:0], e.buf...)
	}
	return nil
}

// reorderObjects recursively reorders all object members in place
// according to the ordering specified in RFC 8785, section 3.2.3.
//
// Pre-conditions:
//	• The value is valid (i.e., no decoder errors should ever occur).
//	• The value is compact (i.e., no whitespace is present).
//	• Initial call is provided a nil Decoder.
//
// Post-conditions:
//	• Exactly one JSON value is read from the Decoder.
//	• All fully-parsed JSON objects are reordered by directly moving
//	  the members in the value buffer.
//
// The runtime is approximately O(n·log(n)) + O(m·log(m)),
// where n is len(v) and m is the total number of object members.
func (v RawValue) reorderObjects(d *Decoder) {
	if d == nil {
		d = new(Decoder) // TODO: Pool this.
		d.state.init()
		d.buf = v
	}

	switch tok, _ := d.ReadToken(); tok.Kind() {
	case '{':
		// Iterate and collect the name and offsets for every object member.
		type member struct {
			// name is the unescaped name.
			name []byte
			// before and after are byte offsets into v that represents
			// the entire name/value pair. It may contain leading commas.
			before, after int64
		}
		var members []member // TODO: Pool this.
		var prevName []byte
		isSorted := true

		beforeBody := d.InputOffset() // offset after '{'
		for d.PeekKind() != '}' {
			beforeName := d.InputOffset()
			name, _ := d.ReadValue()
			name, _ = unescapeString(nil, name) // TODO: Pool the needed buffer?
			v.reorderObjects(d)
			afterValue := d.InputOffset()

			if isSorted && len(members) > 0 {
				isSorted = lessUTF16(prevName, name)
			}
			members = append(members, member{name, beforeName, afterValue})
			prevName = name
		}
		afterBody := d.InputOffset() // offset before '}'
		d.ReadToken()

		// Sort the members; return early if it's already sorted.
		if isSorted {
			return
		}
		sort.Slice(members, func(i, j int) bool {
			return lessUTF16(members[i].name, members[j].name)
		})

		// Append the reordered members to a new buffer,
		// then copy the reordered members back over the original members.
		// Avoid swapping in place since each member may be a different size
		// where moving a member over a smaller member may corrupt the data
		// for subsequent members before they have been moved.
		//
		// The following invariant must hold:
		//	sum([m.after-m.before for m in members]) == afterBody-beforeBody
		sorted := make([]byte, 0, afterBody-beforeBody) // TODO: Pool this.
		for i, member := range members {
			if v[member.before] == ',' {
				member.before++ // trim leading comma
			}
			sorted = append(sorted, v[member.before:member.after]...)
			if i < len(members)-1 {
				sorted = append(sorted, ',') // append trailing comma
			}
		}
		if int(afterBody-beforeBody) != len(sorted) {
			panic("BUG: length invariant violated")
		}
		copy(v[beforeBody:afterBody], sorted)
	case '[':
		for d.PeekKind() != ']' {
			v.reorderObjects(d)
		}
		d.ReadToken()
	}
}

// lessUTF16 reports whether x is lexicographically less than y according
// to the UTF-16 codepoints of the UTF-8 encoded input strings.
// This implements the ordering specified in RFC 8785, section 3.2.3.
// The inputs must be valid UTF-8, otherwise this may panic.
func lessUTF16(x, y []byte) bool {
	// NOTE: This is an optimized, allocation-free implementation
	// of lessUTF16Simple in value_test.go. FuzzLessUTF16 verifies that the
	// two implementations agree on the result of comparing any two strings.

	isUTF16Self := func(r rune) bool {
		return ('\u0000' <= r && r <= '\uD7FF') || ('\uE000' <= r && r <= '\uFFFF')
	}

	for {
		if len(x) == 0 || len(y) == 0 {
			return len(x) < len(y)
		}

		// ASCII fast-path.
		if x[0] < utf8.RuneSelf || y[0] < utf8.RuneSelf {
			if x[0] != y[0] {
				return x[0] < y[0]
			}
			x, y = x[1:], y[1:]
			continue
		}

		// Decode next pair of runes as UTF-8.
		rx, nx := utf8.DecodeRune(x)
		ry, ny := utf8.DecodeRune(y)
		switch {

		// Both runes encode as either a single or surrogate pair
		// of UTF-16 codepoints.
		case isUTF16Self(rx) == isUTF16Self(ry):
			if rx != ry {
				return rx < ry
			}

		// The x rune is a single UTF-16 codepoint, while
		// the y rune is a surrogate pair of UTF-16 codepoints.
		case isUTF16Self(rx):
			ry, _ := utf16.EncodeRune(ry)
			if rx != ry {
				return rx < ry
			}
			panic("BUG: invalid UTF-8") // implies rx is an unpaired surrogate half

		// The y rune is a single UTF-16 codepoint, while
		// the x rune is a surrogate pair of UTF-16 codepoints.
		case isUTF16Self(ry):
			rx, _ := utf16.EncodeRune(rx)
			if rx != ry {
				return rx < ry
			}
			panic("BUG: invalid UTF-8") // implies ry is an unpaired surrogate half
		}
		x, y = x[nx:], y[ny:]
	}
}
