// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import (
	"bytes"
	"cmp"
	"io"
	"reflect"
	"slices"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/go-json-experiment/json/internal/jsontest"
)

type rawValueTestdataEntry struct {
	name                jsontest.CaseName
	in                  string
	wantValid           bool
	wantCompacted       string
	wantCompactErr      error  // implies wantCompacted is in
	wantIndented        string // wantCompacted if empty; uses "\t" for indent prefix and "    " for indent
	wantIndentErr       error  // implies wantCompacted is in
	wantCanonicalized   string // wantCompacted if empty
	wantCanonicalizeErr error  // implies wantCompacted is in
}

var rawValueTestdata = append(func() (out []rawValueTestdataEntry) {
	// Initialize rawValueTestdata from coderTestdata.
	for _, td := range coderTestdata {
		// NOTE: The Compact method preserves the raw formatting of strings,
		// while the Encoder (by default) does not.
		if td.name.Name == "ComplicatedString" {
			td.outCompacted = strings.TrimSpace(td.in)
		}
		out = append(out, rawValueTestdataEntry{
			name:              td.name,
			in:                td.in,
			wantValid:         true,
			wantCompacted:     td.outCompacted,
			wantIndented:      td.outIndented,
			wantCanonicalized: td.outCanonicalized,
		})
	}
	return out
}(), []rawValueTestdataEntry{{
	name: jsontest.Name("RFC8785/Primitives"),
	in: `{
		"numbers": [333333333.33333329, 1E30, 4.50,
					2e-3, 0.000000000000000000000000001],
		"string": "\u20ac$\u000F\u000aA'\u0042\u0022\u005c\\\"\/",
		"literals": [null, true, false]
	}`,
	wantValid:     true,
	wantCompacted: `{"numbers":[333333333.33333329,1E30,4.50,2e-3,0.000000000000000000000000001],"string":"\u20ac$\u000F\u000aA'\u0042\u0022\u005c\\\"\/","literals":[null,true,false]}`,
	wantIndented: `{
	    "numbers": [
	        333333333.33333329,
	        1E30,
	        4.50,
	        2e-3,
	        0.000000000000000000000000001
	    ],
	    "string": "\u20ac$\u000F\u000aA'\u0042\u0022\u005c\\\"\/",
	    "literals": [
	        null,
	        true,
	        false
	    ]
	}`,
	wantCanonicalized: `{"literals":[null,true,false],"numbers":[333333333.3333333,1e+30,4.5,0.002,1e-27],"string":"€$\u000f\nA'B\"\\\\\"/"}`,
}, {
	name: jsontest.Name("RFC8785/ObjectOrdering"),
	in: `{
		"\u20ac": "Euro Sign",
		"\r": "Carriage Return",
		"\ufb33": "Hebrew Letter Dalet With Dagesh",
		"1": "One",
		"\ud83d\ude00": "Emoji: Grinning Face",
		"\u0080": "Control",
		"\u00f6": "Latin Small Letter O With Diaeresis"
	}`,
	wantValid:     true,
	wantCompacted: `{"\u20ac":"Euro Sign","\r":"Carriage Return","\ufb33":"Hebrew Letter Dalet With Dagesh","1":"One","\ud83d\ude00":"Emoji: Grinning Face","\u0080":"Control","\u00f6":"Latin Small Letter O With Diaeresis"}`,
	wantIndented: `{
	    "\u20ac": "Euro Sign",
	    "\r": "Carriage Return",
	    "\ufb33": "Hebrew Letter Dalet With Dagesh",
	    "1": "One",
	    "\ud83d\ude00": "Emoji: Grinning Face",
	    "\u0080": "Control",
	    "\u00f6": "Latin Small Letter O With Diaeresis"
	}`,
	wantCanonicalized: `{"\r":"Carriage Return","1":"One","":"Control","ö":"Latin Small Letter O With Diaeresis","€":"Euro Sign","😀":"Emoji: Grinning Face","דּ":"Hebrew Letter Dalet With Dagesh"}`,
}, {
	name:          jsontest.Name("LargeIntegers"),
	in:            ` [ -9223372036854775808 , 9223372036854775807 ] `,
	wantValid:     true,
	wantCompacted: `[-9223372036854775808,9223372036854775807]`,
	wantIndented: `[
	    -9223372036854775808,
	    9223372036854775807
	]`,
	wantCanonicalized: `[-9223372036854776000,9223372036854776000]`, // NOTE: Loss of precision due to numbers being treated as floats.
}, {
	name:                jsontest.Name("InvalidUTF8"),
	in:                  `  "living` + "\xde\xad\xbe\xef" + `\ufffd�"  `,
	wantValid:           false, // uses RFC 7493 as the definition; which validates UTF-8
	wantCompacted:       `"living` + "\xde\xad\xbe\xef" + `\ufffd�"`,
	wantCanonicalizeErr: errInvalidUTF8.withOffset(len64(`  "living` + "\xde\xad")),
}, {
	name:                jsontest.Name("InvalidUTF8/SurrogateHalf"),
	in:                  `"\ud800"`,
	wantValid:           false, // uses RFC 7493 as the definition; which validates UTF-8
	wantCompacted:       `"\ud800"`,
	wantCanonicalizeErr: newInvalidEscapeSequenceError(`\ud800"`).withOffset(len64(`"`)),
}, {
	name:              jsontest.Name("UppercaseEscaped"),
	in:                `"\u000B"`,
	wantValid:         true,
	wantCompacted:     `"\u000B"`,
	wantCanonicalized: `"\u000b"`,
}, {
	name:          jsontest.Name("DuplicateNames"),
	in:            ` { "0" : 0 , "1" : 1 , "0" : 0 }`,
	wantValid:     false, // uses RFC 7493 as the definition; which does check for object uniqueness
	wantCompacted: `{"0":0,"1":1,"0":0}`,
	wantIndented: `{
	    "0": 0,
	    "1": 1,
	    "0": 0
	}`,
	wantCanonicalizeErr: newDuplicateNameError(`"0"`).withOffset(len64(` { "0" : 0 , "1" : 1 , `)),
}, {
	name:                jsontest.Name("Whitespace"),
	in:                  " \n\r\t",
	wantValid:           false,
	wantCompacted:       " \n\r\t",
	wantCompactErr:      io.ErrUnexpectedEOF,
	wantIndentErr:       io.ErrUnexpectedEOF,
	wantCanonicalizeErr: io.ErrUnexpectedEOF,
}}...)

func TestRawValueMethods(t *testing.T) {
	for _, td := range rawValueTestdata {
		t.Run(td.name.Name, func(t *testing.T) {
			if td.wantIndented == "" {
				td.wantIndented = td.wantCompacted
			}
			if td.wantCanonicalized == "" {
				td.wantCanonicalized = td.wantCompacted
			}
			if td.wantCompactErr != nil {
				td.wantCompacted = td.in
			}
			if td.wantIndentErr != nil {
				td.wantIndented = td.in
			}
			if td.wantCanonicalizeErr != nil {
				td.wantCanonicalized = td.in
			}

			v := RawValue(td.in)
			gotValid := v.IsValid()
			if gotValid != td.wantValid {
				t.Errorf("%s: RawValue.IsValid = %v, want %v", td.name.Where, gotValid, td.wantValid)
			}

			gotCompacted := RawValue(td.in)
			gotCompactErr := gotCompacted.Compact()
			if string(gotCompacted) != td.wantCompacted {
				t.Errorf("%s: RawValue.Compact = %s, want %s", td.name.Where, gotCompacted, td.wantCompacted)
			}
			if !reflect.DeepEqual(gotCompactErr, td.wantCompactErr) {
				t.Errorf("%s: RawValue.Compact error mismatch:\ngot  %v\nwant %v", td.name.Where, gotCompactErr, td.wantCompactErr)
			}

			gotIndented := RawValue(td.in)
			gotIndentErr := gotIndented.Indent("\t", "    ")
			if string(gotIndented) != td.wantIndented {
				t.Errorf("%s: RawValue.Indent = %s, want %s", td.name.Where, gotIndented, td.wantIndented)
			}
			if !reflect.DeepEqual(gotIndentErr, td.wantIndentErr) {
				t.Errorf("%s: RawValue.Indent error mismatch:\ngot  %v\nwant %v", td.name.Where, gotIndentErr, td.wantIndentErr)
			}

			gotCanonicalized := RawValue(td.in)
			gotCanonicalizeErr := gotCanonicalized.Canonicalize()
			if string(gotCanonicalized) != td.wantCanonicalized {
				t.Errorf("%s: RawValue.Canonicalize = %s, want %s", td.name.Where, gotCanonicalized, td.wantCanonicalized)
			}
			if !reflect.DeepEqual(gotCanonicalizeErr, td.wantCanonicalizeErr) {
				t.Errorf("%s: RawValue.Canonicalize error mismatch:\ngot  %v\nwant %v", td.name.Where, gotCanonicalizeErr, td.wantCanonicalizeErr)
			}
		})
	}
}

var compareUTF16Testdata = []string{"", "\r", "1", "\u0080", "\u00f6", "\u20ac", "\U0001f600", "\ufb33"}

func TestCompareUTF16(t *testing.T) {
	for i, si := range compareUTF16Testdata {
		for j, sj := range compareUTF16Testdata {
			got := compareUTF16([]byte(si), []byte(sj))
			want := cmp.Compare(i, j)
			if got != want {
				t.Errorf("compareUTF16(%q, %q) = %v, want %v", si, sj, got, want)
			}
		}
	}
}

func FuzzCompareUTF16(f *testing.F) {
	for _, td1 := range compareUTF16Testdata {
		for _, td2 := range compareUTF16Testdata {
			f.Add([]byte(td1), []byte(td2))
		}
	}

	// compareUTF16Simple is identical to compareUTF16,
	// but relies on naively converting a string to a []uint16 codepoints.
	// It is easy to verify as correct, but is slow.
	compareUTF16Simple := func(x, y []byte) int {
		ux := utf16.Encode([]rune(string(x)))
		uy := utf16.Encode([]rune(string(y)))
		if n := slices.Compare(ux, uy); n != 0 {
			return n
		}
		return bytes.Compare(x, y) // only occurs for strings with invalid UTF-8
	}

	f.Fuzz(func(t *testing.T, s1, s2 []byte) {
		// Compare the optimized and simplified implementations.
		got := compareUTF16(s1, s2)
		want := compareUTF16Simple(s1, s2)
		if got != want {
			t.Errorf("compareUTF16(%q, %q) = %v, want %v", s1, s2, got, want)
		}
	})
}
