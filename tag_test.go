// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import (
	"errors"
	"reflect"
	"testing"
)

type unexported struct{}

func TestParseTagOptions(t *testing.T) {
	tests := []struct {
		name     string
		in       interface{} // must be a struct with a single field
		wantOpts fieldOptions
		wantErr  error
	}{{
		name: "GoName",
		in: struct {
			FieldName int
		}{},
		wantOpts: fieldOptions{name: "FieldName"},
	}, {
		name: "GoNameWithOptions",
		in: struct {
			FieldName int `json:",inline"`
		}{},
		wantOpts: fieldOptions{name: "FieldName", inline: true},
	}, {
		name: "Empty",
		in: struct {
			V int `json:""`
		}{},
		wantOpts: fieldOptions{name: "V"},
	}, {
		name: "Unexported",
		in: struct {
			v int `json:"Hello"`
		}{},
		wantErr: errors.New("unexported Go struct field v cannot have non-ignored `json:\"Hello\"` tag"),
	}, {
		name: "UnexportedEmpty",
		in: struct {
			v int `json:""`
		}{},
		wantErr: errors.New("unexported Go struct field v cannot have non-ignored `json:\"\"` tag"),
	}, {
		name: "EmbedUnexported",
		in: struct {
			unexported
		}{},
		wantErr: errors.New("embedded Go struct field unexported of an unexported type must be explicitly ignored with a `json:\"-\"` tag"),
	}, {
		name: "Ignored",
		in: struct {
			V int `json:"-"`
		}{},
		wantErr: errIgnoredField,
	}, {
		name: "IgnoredEmbedUnexported",
		in: struct {
			unexported `json:"-"`
		}{},
		wantErr: errIgnoredField,
	}, {
		name: "DashComma",
		in: struct {
			V int `json:"-,"`
		}{},
		wantErr: errors.New("Go struct field V has malformed `json` tag: invalid trailing ',' character"),
	}, {
		name: "QuotedDashName",
		in: struct {
			V int `json:"'-'"`
		}{},
		wantOpts: fieldOptions{name: "-"},
	}, {
		name: "LatinPunctuationName",
		in: struct {
			V int `json:"$%-/"`
		}{},
		wantOpts: fieldOptions{name: "$%-/"},
	}, {
		name: "QuotedLatinPunctuationName",
		in: struct {
			V int `json:"'$%-/'"`
		}{},
		wantOpts: fieldOptions{name: "$%-/"},
	}, {
		name: "LatinDigitsName",
		in: struct {
			V int `json:"0123456789"`
		}{},
		wantOpts: fieldOptions{name: "0123456789"},
	}, {
		name: "QuotedLatinDigitsName",
		in: struct {
			V int `json:"'0123456789'"`
		}{},
		wantOpts: fieldOptions{name: "0123456789"},
	}, {
		name: "LatinUppercaseName",
		in: struct {
			V int `json:"ABCDEFGHIJKLMOPQRSTUVWXYZ"`
		}{},
		wantOpts: fieldOptions{name: "ABCDEFGHIJKLMOPQRSTUVWXYZ"},
	}, {
		name: "LatinLowercaseName",
		in: struct {
			V int `json:"abcdefghijklmnopqrstuvwxyz_"`
		}{},
		wantOpts: fieldOptions{name: "abcdefghijklmnopqrstuvwxyz_"},
	}, {
		name: "GreekName",
		in: struct {
			V string `json:"Ελλάδα"`
		}{},
		wantOpts: fieldOptions{name: "Ελλάδα"},
	}, {
		name: "QuotedGreekName",
		in: struct {
			V string `json:"'Ελλάδα'"`
		}{},
		wantOpts: fieldOptions{name: "Ελλάδα"},
	}, {
		name: "ChineseName",
		in: struct {
			V string `json:"世界"`
		}{},
		wantOpts: fieldOptions{name: "世界"},
	}, {
		name: "QuotedChineseName",
		in: struct {
			V string `json:"'世界'"`
		}{},
		wantOpts: fieldOptions{name: "世界"},
	}, {
		name: "PercentSlashName",
		in: struct {
			V int `json:"text/html%"`
		}{},
		wantOpts: fieldOptions{name: "text/html%"},
	}, {
		name: "QuotedPercentSlashName",
		in: struct {
			V int `json:"'text/html%'"`
		}{},
		wantOpts: fieldOptions{name: "text/html%"},
	}, {
		name: "PunctuationName",
		in: struct {
			V string `json:"!#$%&()*+-./:;<=>?@[]^_{|}~ "`
		}{},
		wantOpts: fieldOptions{name: "!#$%&()*+-./:;<=>?@[]^_{|}~ "},
	}, {
		name: "QuotedPunctuationName",
		in: struct {
			V string `json:"'!#$%&()*+-./:;<=>?@[]^_{|}~ '"`
		}{},
		wantOpts: fieldOptions{name: "!#$%&()*+-./:;<=>?@[]^_{|}~ "},
	}, {
		name: "EmptyName",
		in: struct {
			V int `json:"''"`
		}{},
		wantOpts: fieldOptions{name: ""},
	}, {
		name: "SpaceName",
		in: struct {
			V int `json:"' '"`
		}{},
		wantOpts: fieldOptions{name: " "},
	}, {
		name: "CommaQuotes",
		in: struct {
			V int `json:"',\\'\"\\\"'"`
		}{},
		wantOpts: fieldOptions{name: `,'""`},
	}, {
		name: "SingleComma",
		in: struct {
			V int `json:","`
		}{},
		wantErr: errors.New("Go struct field V has malformed `json` tag: invalid trailing ',' character"),
	}, {
		name: "SuperfluousCommas",
		in: struct {
			V int `json:",,,,\"\",,inline,unknown,,,,"`
		}{},
		wantErr: errors.New("Go struct field V has malformed `json` tag: invalid character ',' at start of option (expecting Unicode letter or single quote)"),
	}, {
		name: "NoCaseOption",
		in: struct {
			FieldName int `json:",nocase"`
		}{},
		wantOpts: fieldOptions{
			name:   "FieldName",
			nocase: true,
		},
	}, {
		name: "InlineOption",
		in: struct {
			FieldName int `json:",inline"`
		}{},
		wantOpts: fieldOptions{
			name:   "FieldName",
			inline: true,
		},
	}, {
		name: "UnknownOption",
		in: struct {
			FieldName int `json:",unknown"`
		}{},
		wantOpts: fieldOptions{name: "FieldName", inline: true, unknown: true},
	}, {
		name: "OmitZeroOption",
		in: struct {
			FieldName int `json:",omitzero"`
		}{},
		wantOpts: fieldOptions{name: "FieldName", omitzero: true},
	}, {
		name: "OmitEmptyOption",
		in: struct {
			FieldName int `json:",omitempty"`
		}{},
		wantOpts: fieldOptions{name: "FieldName", omitempty: true},
	}, {
		name: "StringOption",
		in: struct {
			FieldName int `json:",string"`
		}{},
		wantOpts: fieldOptions{name: "FieldName", string: true},
	}, {
		name: "FormatOptionEqual",
		in: struct {
			FieldName int `json:",format=fizzbuzz"`
		}{},
		wantErr: errors.New("Go struct field FieldName is missing value for `json` tag option format"),
	}, {
		name: "FormatOptionColon",
		in: struct {
			FieldName int `json:",format:fizzbuzz"`
		}{},
		wantOpts: fieldOptions{
			name:   "FieldName",
			format: "fizzbuzz",
		},
	}, {
		name: "FormatOptionQuoted",
		in: struct {
			FieldName int `json:",format:'2006-01-02'"`
		}{},
		wantOpts: fieldOptions{
			name:   "FieldName",
			format: "2006-01-02",
		},
	}, {
		name: "FormatOptionInvalid",
		in: struct {
			FieldName int `json:",format:'2006-01-02"`
		}{},
		wantErr: errors.New("Go struct field FieldName has malformed value for `json` tag option format: single-quoted string not terminated: '2006-01-0..."),
	}, {
		name: "DuplicateFormatOptions",
		in: struct {
			FieldName int `json:",format:alpha,format:bravo"`
		}{},
		wantErr: errors.New("Go struct field FieldName has duplicate appearance of `json` tag option format"),
	}, {
		name: "AllOptions",
		in: struct {
			FieldName int `json:",nocase,inline,unknown,omitzero,omitempty,string,format:format"`
		}{},
		wantOpts: fieldOptions{
			name:      "FieldName",
			nocase:    true,
			inline:    true,
			unknown:   true,
			omitzero:  true,
			omitempty: true,
			string:    true,
			format:    "format",
		},
	}, {
		name: "AllOptionsQuoted",
		in: struct {
			FieldName int `json:",'nocase','inline','unknown','omitzero','omitempty','string','format':'format'"`
		}{},
		wantErr: errors.New("Go struct field FieldName has unnecessarily quoted appearance of `json` tag option 'nocase'; specify nocase instead"),
	}, {
		name: "AllOptionsCaseSensitive",
		in: struct {
			FieldName int `json:",NOCASE,INLINE,UNKNOWN,OMITZERO,OMITEMPTY,STRING,FORMAT:FORMAT"`
		}{},
		wantErr: errors.New("Go struct field FieldName has invalid appearance of `json` tag option NOCASE; specify nocase instead"),
	}, {
		name: "AllOptionsSpaceSensitive",
		in: struct {
			FieldName int `json:", nocase , inline , unknown , omitzero , omitempty , string , format:format "`
		}{},
		wantErr: errors.New("Go struct field FieldName has malformed `json` tag: invalid character ' ' at start of option (expecting Unicode letter or single quote)"),
	}, {
		name: "UnknownOption",
		in: struct {
			FieldName int `json:",inline,whoknows,string"`
		}{},
		wantOpts: fieldOptions{
			name:   "FieldName",
			inline: true,
			string: true,
		},
	}, {
		name: "MalformedQuotedString/MissingQuote",
		in: struct {
			FieldName int `json:"'hello,string"`
		}{},
		wantErr: errors.New("Go struct field FieldName has malformed `json` tag: single-quoted string not terminated: 'hello,str..."),
	}, {
		name: "MalformedQuotedString/MissingComma",
		in: struct {
			FieldName int `json:"'hello'inline,string"`
		}{},
		wantErr: errors.New("Go struct field FieldName has malformed `json` tag: invalid character 'i' before next option (expecting ',')"),
	}, {
		name: "MalformedQuotedString/InvalidEscape",
		in: struct {
			FieldName int `json:"'hello\\u####',inline,string"`
		}{},
		wantErr: errors.New("Go struct field FieldName has malformed `json` tag: invalid single-quoted string: 'hello\\u####'"),
	}, {
		name: "MisnamedTag",
		in: struct {
			V int `jsom:"Misnamed"`
		}{},
		wantOpts: fieldOptions{
			name: "V",
		},
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := reflect.TypeOf(tt.in).Field(0)
			gotOpts, gotErr := parseFieldOptions(fs)
			if !reflect.DeepEqual(gotOpts, tt.wantOpts) || !reflect.DeepEqual(gotErr, tt.wantErr) {
				t.Errorf("parseFieldOptions(%T) = (%v, %v), want (%v, %v)", tt.in, gotOpts, gotErr, tt.wantOpts, tt.wantErr)
			}
		})
	}
}
