// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import (
	"errors"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

var errIgnoredField = errors.New("ignored field")

type structFields struct {
	flattened    []structField
	byActualName map[string]int   // index into flattened slice
	byFoldedName map[string][]int // index into flattened slice for all fold-equal fields
}

type structField struct {
	index   int // index into reflect.StructField.Field
	fncs    *arshaler
	isEmpty func(addressableValue) bool
	fieldOptions
}

func makeStructFields(t reflect.Type) (structFields, *SemanticError) {
	var fs structFields
	fs.byActualName = make(map[string]int, t.NumField())
	fs.byFoldedName = make(map[string][]int, t.NumField())

	var hasAnyJSONTag bool
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		_, hasTag := sf.Tag.Lookup("json")
		hasAnyJSONTag = hasAnyJSONTag || hasTag
		options, err := parseFieldOptions(sf)
		if err != nil {
			if err == errIgnoredField {
				continue
			}
			return fs, &SemanticError{GoType: t, Err: err}
		}

		// Process the JSON object name.
		if !utf8.ValidString(options.name) {
			err := fmt.Errorf("Go struct fields %s has JSON object name %q with invalid UTF-8", sf.Name, options.name)
			return fs, &SemanticError{GoType: t, Err: err}
		}
		if j, ok := fs.byActualName[options.name]; ok {
			err := fmt.Errorf("Go struct fields %s and %s conflict over JSON object name %q", t.Field(j).Name, t.Field(i).Name, options.name)
			return fs, &SemanticError{GoType: t, Err: err}
		}
		fs.byActualName[options.name] = len(fs.flattened)
		foldedName := string(foldName([]byte(options.name)))
		fs.byFoldedName[foldedName] = append(fs.byFoldedName[foldedName], len(fs.flattened)) // may have conflicts

		// Provide a function that can determine for certain that the value
		// would serialize as an empty JSON value.
		isEmpty := func(addressableValue) bool { return false }
		switch sf.Type.Kind() {
		case reflect.String, reflect.Map, reflect.Array, reflect.Slice:
			// If there is a custom marshal method, we cannot determine
			// upfront whether it will write an empty JSON value.
			if which, _ := implementsWhich(sf.Type, jsonMarshalerV2Type, jsonMarshalerV1Type, textMarshalerType); which != nil {
				break
			}
			isEmpty = func(va addressableValue) bool { return va.Len() == 0 }
		case reflect.Ptr, reflect.Interface:
			isEmpty = func(va addressableValue) bool { return va.IsNil() }
		}

		fs.flattened = append(fs.flattened, structField{
			index:        i,
			fncs:         lookupArshaler(sf.Type),
			isEmpty:      isEmpty,
			fieldOptions: options,
		})
	}

	// NOTE: New users to the json package are occasionally surprised that
	// unexported fields are ignored. This occurs by necessity due to our
	// inability to directly introspect such fields with Go reflection
	// without the use of unsafe.
	//
	// To reduce friction here, refuse to serialize any Go struct that
	// has no JSON serializable fields, has at least one Go struct field,
	// and does not have any `json` tags present. For example,
	// errors returned by errors.New would fail to serialize.
	if len(fs.flattened) == 0 && t.NumField() > 0 && !hasAnyJSONTag {
		err := errors.New("Go struct kind has no exported fields")
		return fs, &SemanticError{GoType: t, Err: err}
	}

	return fs, nil
}

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
	tag, hasTag := sf.Tag.Lookup("json")

	// Check whether this field is explicitly ignored.
	if tag == "-" {
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
			return fieldOptions{}, fmt.Errorf("embedded Go struct field %s of an unexported type must be explicitly ignored with a `json:\"-\"` tag", sf.Type.Name())
		}
		// Tag options specified on an unexported field suggests user error.
		if hasTag {
			return fieldOptions{}, fmt.Errorf("unexported Go struct field %s cannot have non-ignored `json:%q` tag", sf.Name, tag)
		}
		return fieldOptions{}, errIgnoredField
	}

	// Determine the JSON member name for this Go field. A user-specified name
	// may be provided as either an identifier or a single-quoted string.
	// The single-quoted string allows arbitrary characters in the name.
	// See https://golang.org/issue/2718 and https://golang.org/issue/3546.
	out.name = sf.Name // always starts with an uppercase character
	if len(tag) > 0 && !strings.HasPrefix(tag, ",") {
		// For better compatibility with v1, accept almost any unescaped name.
		n := len(tag) - len(strings.TrimLeftFunc(tag, func(r rune) bool {
			return !strings.ContainsRune(",\\'\"`", r) // reserve comma, backslash, and quotes
		}))
		opt := tag[:n]
		if n == 0 {
			// Allow a single quoted strings for arbitrary names.
			opt, n, err = consumeTagOption(tag)
			if err != nil {
				return fieldOptions{}, fmt.Errorf("Go struct field %s has malformed `json` tag: %v", sf.Name, err)
			}
		}
		out.name = opt
		tag = tag[n:]
	}

	// Handle any additional tag options (if any).
	seenOpts := make(map[string]bool)
	for len(tag) > 0 {
		// Consume comma delimiter.
		if tag[0] != ',' {
			return fieldOptions{}, fmt.Errorf("Go struct field %s has malformed `json` tag: invalid character %q before next option (expecting ',')", sf.Name, tag[0])
		}
		tag = tag[len(","):]
		if len(tag) == 0 {
			return fieldOptions{}, fmt.Errorf("Go struct field %s has malformed `json` tag: invalid trailing ',' character", sf.Name)
		}

		// Consume and process the tag option.
		opt, n, err := consumeTagOption(tag)
		if err != nil {
			return fieldOptions{}, fmt.Errorf("Go struct field %s has malformed `json` tag: %v", sf.Name, err)
		}
		rawOpt := tag[:n]
		tag = tag[n:]
		if strings.HasPrefix(rawOpt, "'") && strings.TrimFunc(opt, isLetterOrDigit) == "" {
			return fieldOptions{}, fmt.Errorf("Go struct field %s has unnecessarily quoted appearance of `json` tag option %s; specify %s instead", sf.Name, rawOpt, opt)
		}
		switch opt {
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
			if !strings.HasPrefix(tag, ":") {
				return fieldOptions{}, fmt.Errorf("Go struct field %s is missing value for `json` tag option format", sf.Name)
			}
			tag = tag[len(":"):]
			opt, n, err := consumeTagOption(tag)
			if err != nil {
				return fieldOptions{}, fmt.Errorf("Go struct field %s has malformed value for `json` tag option format: %v", sf.Name, err)
			}
			tag = tag[n:]
			out.format = opt
		default:
			// Reject keys that resemble one of the supported options.
			// This catches invalid mutants such as "omitEmpty" or "omit_empty".
			normOpt := strings.ReplaceAll(strings.ToLower(opt), "_", "")
			switch normOpt {
			case "nocase", "inline", "unknown", "omitzero", "omitempty", "string", "format":
				return fieldOptions{}, fmt.Errorf("Go struct field %s has invalid appearance of `json` tag option %s; specify %s instead", sf.Name, opt, normOpt)
			}

			// NOTE: Everything else is ignored. This does not mean it is
			// forward compatible to insert arbitrary tag options since
			// a future version of this package may understand that tag.
		}

		// Reject duplicates.
		if seenOpts[opt] {
			return fieldOptions{}, fmt.Errorf("Go struct field %s has duplicate appearance of `json` tag option %s", sf.Name, rawOpt)
		}
		seenOpts[opt] = true
	}
	return out, nil
}

func consumeTagOption(in string) (string, int, error) {
	switch r, _ := utf8.DecodeRuneInString(in); {
	// Option as a Go identifier.
	case r == '_' || unicode.IsLetter(r):
		n := len(in) - len(strings.TrimLeftFunc(in, isLetterOrDigit))
		return in[:n], n, nil
	// Option as a single-quoted string.
	case r == '\'':
		// The grammar is nearly identical to a double-quoted Go string literal,
		// but uses single quotes as the terminators. The reason for a custom
		// grammar is because both backtick and double quotes cannot be used
		// verbatim in a struct tag.
		//
		// Convert a single-quoted string to a double-quote string and rely on
		// strconv.Unquote to handle the rest.
		var inEscape bool
		b := []byte{'"'}
		n := len(`'`)
		for len(in) > n {
			r, rn := utf8.DecodeRuneInString(in[n:])
			switch {
			case inEscape:
				if r == '\'' {
					b = b[:len(b)-1] // remove escape character: `\'` => `'`
				}
				inEscape = false
			case r == '\\':
				inEscape = true
			case r == '"':
				b = append(b, '\\') // insert escape character: `"` => `\"`
			case r == '\'':
				b = append(b, '"')
				n += len(`'`)
				out, err := strconv.Unquote(string(b))
				if err != nil {
					return "", 0, fmt.Errorf("invalid single-quoted string: %s", in[:n])
				}
				return out, n, nil
			}
			b = append(b, in[n:][:rn]...)
			n += rn
		}
		if n > 10 {
			n = 10 // limit the amount of context printed in the error
		}
		return "", 0, fmt.Errorf("single-quoted string not terminated: %s...", in[:n])
	case len(in) == 0:
		return "", 0, io.ErrUnexpectedEOF
	default:
		return "", 0, fmt.Errorf("invalid character %q at start of option (expecting Unicode letter or single quote)", r)
	}
}

func isLetterOrDigit(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsNumber(r)
}
