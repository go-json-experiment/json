// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import (
	"fmt"
	"reflect"
	"testing"
	"unicode"

	jsonv1 "encoding/json"
)

var equalFoldTestdata = []struct {
	in1, in2 string
	want     bool
}{
	{"", "", true},
	{"abc", "abc", true},
	{"ABcd", "ABcd", true},
	{"123abc", "123ABC", true},
	{"αβδ", "ΑΒΔ", true},
	{"abc", "xyz", false},
	{"abc", "XYZ", false},
	{"abcdefghijk", "abcdefghijX", false},
	{"abcdefghijk", "abcdefghij\u212A", true},
	{"abcdefghijK", "abcdefghij\u212A", true},
	{"abcdefghijkz", "abcdefghij\u212Ay", false},
	{"abcdefghijKz", "abcdefghij\u212Ay", false},
	{"1", "2", false},
	{"utf-8", "US-ASCII", false},
	{"hello, world!", "hello, world!", true},
	{"hello, world!", "Hello, World!", true},
	{"hello, world!", "HELLO, WORLD!", true},
	{"hello, world!", "jello, world!", false},
	{"γειά, κόσμε!", "γειά, κόσμε!", true},
	{"γειά, κόσμε!", "Γειά, Κόσμε!", true},
	{"γειά, κόσμε!", "ΓΕΙΆ, ΚΌΣΜΕ!", true},
	{"γειά, κόσμε!", "ΛΕΙΆ, ΚΌΣΜΕ!", false},
	{"AESKey", "aesKey", true},
	{"γειά, κόσμε!", "Γ\xce_\xb5ιά, Κόσμε!", false},
}

func TestEqualFold(t *testing.T) {
	for _, tt := range equalFoldTestdata {
		got := equalFold([]byte(tt.in1), []byte(tt.in2))
		if got != tt.want {
			t.Errorf("equalFold(%q, %q) = %v, want %v", tt.in1, tt.in2, got, tt.want)
		}
	}
}

func equalFold(x, y []byte) bool {
	return string(foldName(x)) == string(foldName(y))
}

func TestFoldRune(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	var foldSet []rune
	for r := rune(0); r <= unicode.MaxRune; r++ {
		// Derive all runes that are all part of the same fold set.
		foldSet = foldSet[:0]
		for r0 := r; r != r0 || len(foldSet) == 0; r = unicode.SimpleFold(r) {
			foldSet = append(foldSet, r)
		}

		// Normalized form of each rune in a foldset must be the same and
		// also be within the set itself.
		var withinSet bool
		rr0 := foldRune(foldSet[0])
		for _, r := range foldSet {
			withinSet = withinSet || rr0 == r
			rr := foldRune(r)
			if rr0 != rr {
				t.Errorf("foldRune(%q) = %q, want %q", r, rr, rr0)
			}
		}
		if !withinSet {
			t.Errorf("foldRune(%q) = %q not in fold set %q", foldSet[0], rr0, string(foldSet))
		}
	}
}

// BenchmarkUnmarshalUnknown unmarshals an unknown field into a struct with
// varying number of fields. Since the unknown field does not directly match
// any known field by name, it must fall back on case-insensitive matching.
func BenchmarkUnmarshalUnknown(b *testing.B) {
	in := []byte(`{"NameUnknown":null}`)
	for _, n := range []int{1, 2, 5, 10, 20, 50, 100} {
		unmarshal := Unmarshal
		if benchV1 {
			unmarshal = jsonv1.Unmarshal
		}

		var fields []reflect.StructField
		for i := 0; i < n; i++ {
			fields = append(fields, reflect.StructField{
				Name: fmt.Sprintf("Name%d", i),
				Type: reflect.TypeOf(0),
				Tag:  `json:",nocase"`,
			})
		}
		out := reflect.New(reflect.StructOf(fields)).Interface()

		b.Run(fmt.Sprintf("N%d", n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				if err := unmarshal(in, out); err != nil {
					b.Fatalf("Unmarshal error: %v", err)
				}
			}
		})
	}
}
