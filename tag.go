// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

var errIgnoredField = errors.New("ignored field")

type fieldOptions struct {
	name      string
	nocase    bool
	inline    bool
	unknown   bool
	omitzero  bool
	omitempty bool
	string    bool
	format    string
}

// parseFieldOptions parses the `json` tag in a Go struct field as
// a structured set of options configuring parameters such as
// the JSON member name and other features.
// As a special case, it returns errIgnoredField if the field is ignored.
func parseFieldOptions(sf reflect.StructField) (out fieldOptions, err error) {
	var opts []tagOption
	if tag, ok := sf.Tag.Lookup("json"); ok {
		opts, ok = splitTagOptions(tag)
		if !ok {
			return fieldOptions{}, fmt.Errorf("Go struct field %q has malformed `json` tag: %s", sf.Name, sf.Tag.Get("json"))
		}
	}

	// Check whether this field is explicitly ignored.
	if len(opts) == 1 && opts[0] == unquotedDash {
		return fieldOptions{}, errIgnoredField
	}

	// Check whether this field is unexported.
	// TODO(https://golang.org/issue/41563): Use reflect.StructField.IsExported.
	if sf.PkgPath != "" {
		// In contrast to v1, v2 no longer forwards exported fields from
		// embedded fields of unexported types since Go reflection does not
		// allow the same set of operations that are available in normal cases
		// of purely exported fields.
		// See https://golang.org/issue/21357 and https://golang.org/issue/24153.
		if sf.Anonymous {
			return fieldOptions{}, fmt.Errorf("embedded Go struct field %q of an unexported type must be explicitly ignored with a `json:\"-\"` tag", sf.Type.Name())
		}
		// Tag options specified on an unexported field suggests user error.
		if opts != nil {
			return fieldOptions{}, fmt.Errorf("unexported Go struct field %q cannot have non-ignored `json` tag", sf.Name)
		}
		return fieldOptions{}, errIgnoredField
	}

	// Determine the JSON member name for this Go field.
	out.name = sf.Name // always starts with an uppercase character
	if len(opts) > 0 && opts[0] != unquotedEmpty {
		out.name = opts[0].value
	}

	// Handle any additional tag options (if any).
	if len(opts) <= 1 {
		return out, nil
	}
	seenKeys := make(map[string]bool)
	for _, opt := range opts[1:] {
		key, value := opt.value, ""
		if strings.HasPrefix(opt.value, "format=") {
			key, value = "format", opt.value[len("format="):]
		}
		if seenKeys[key] && key != "" {
			return fieldOptions{}, fmt.Errorf("Go struct field %q has duplicate appearance of `json` tag option %q", sf.Name, key)
		}
		seenKeys[key] = true
		switch key {
		case "nocase":
			out.nocase = true
		case "inline":
			out.inline = true
		case "unknown":
			out.unknown = true
			out.inline = true // implied by "unknown"
		case "omitzero":
			out.omitzero = true
		case "omitempty":
			out.omitempty = true
		case "string":
			out.string = true
		case "format":
			out.format = value
		default:
			// NOTE: Everything else is ignored. This does not mean it is
			// forward compatible to insert arbitrary tag options since
			// a future version of this package may understand that tag.
		}
	}
	return out, nil
}

type tagOption struct {
	quoted bool
	value  string
}

var (
	unquotedEmpty = tagOption{quoted: false, value: ""}
	unquotedDash  = tagOption{quoted: false, value: "-"}
)

// splitTagOptions splits the tag up as a comma-delimited sequence of options,
// where each option is either a quoted string or not.
func splitTagOptions(s string) (opts []tagOption, ok bool) {
	opts = make([]tagOption, 0)
	for len(s) > 0 {
		// Consume comma delimiter.
		if len(opts) > 0 {
			if len(s) == 0 || s[0] != ',' {
				return nil, false
			}
			s = s[len(`,`):]
		}

		// Parse as either a quoted or unquoted string.
		var n int
		opt := tagOption{quoted: len(s) > 0 && (s[0] == '"' || s[0] == '`')}
		if opt.quoted {
			var err error
			prefix, _ := quotedPrefix(s)
			opt.value, err = strconv.Unquote(prefix)
			if err != nil {
				return nil, false
			}
			n += len(prefix)
		} else {
			n = strings.IndexByte(s, ',')
			if n < 0 {
				n = len(s)
			}
			opt.value = s[:n]
		}
		opts = append(opts, opt)
		s = s[n:]
	}
	return opts, true
}

// quotedPrefix is identical to strconv.QuotedPrefix.
// TODO(https://golang.org/issue/45033): Use strconv.QuotedPrefix.
func quotedPrefix(s string) (prefix string, err error) {
	quotedPrefixLen := func(s string) int {
		if len(s) == 0 {
			return len(s)
		}
		switch s[0] {
		case '`':
			for i, r := range s[len("`"):] {
				if r == '`' {
					return len("`") + i + len("`")
				}
			}
		case '"':
			var inEscape bool
			for i, r := range s[len(`"`):] {
				switch {
				case inEscape:
					inEscape = false
				case r == '\\':
					inEscape = true
				case r == '"':
					return len(`"`) + i + len(`"`)
				}
			}
		}
		return len(s)
	}

	n := quotedPrefixLen(s)
	if _, err := strconv.Unquote(s[:n]); err != nil {
		return "", err
	}
	return s[:n], nil
}
