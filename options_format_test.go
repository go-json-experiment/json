// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/go-json-experiment/json/jsontext"
)

func TestExperimentalFormatSupport(t *testing.T) {
	const jsonValue = `{"Duration":"PT1H1M1S"}`
	const wantDuration = time.Hour + time.Minute + time.Second
	goValue := struct {
		Duration time.Duration `json:",format:iso8601"`
	}{wantDuration}

	for _, name := range []string{"PerCall", "Global"} {
		t.Run(name, func(t *testing.T) {
			opts := []Options{ExperimentalSupportFormatTag(true)}
			if name == "Global" {
				opts = nil
				ExperimentalGlobalSupportFormatTag(true)
				defer ExperimentalGlobalSupportFormatTag(false)
			}

			t.Run("Marshal", func(t *testing.T) {
				switch got, err := Marshal(goValue, opts...); {
				case err != nil:
					t.Fatalf("Marshal error: %v", err)
				case string(got) != jsonValue:
					t.Fatalf("Marshal = `%s`, want `%s`", string(got), jsonValue)
				}
			})

			t.Run("MarshalWrite", func(t *testing.T) {
				got := new(bytes.Buffer)
				switch err := MarshalWrite(got, goValue, opts...); {
				case err != nil:
					t.Fatalf("MarshalWrite error: %v", err)
				case got.String() != jsonValue:
					t.Fatalf("MarshalWrite = `%s`, want `%s`", got.String(), jsonValue)
				}
			})

			t.Run("MarshalEncode", func(t *testing.T) {
				got := new(bytes.Buffer)
				enc := jsontext.NewEncoder(got)
				switch err := MarshalEncode(enc, goValue, opts...); {
				case err != nil:
					t.Fatalf("MarshalEncode error: %v", err)
				case strings.TrimSpace(got.String()) != jsonValue:
					t.Fatalf("MarshalEncode = `%s`, want `%s`", strings.TrimSpace(got.String()), jsonValue)
				}
			})

			t.Run("Unmarshal", func(t *testing.T) {
				switch err := Unmarshal([]byte(jsonValue), &goValue, opts...); {
				case err != nil:
					t.Fatalf("Unmarshal error: %v", err)
				case goValue.Duration != wantDuration:
					t.Fatalf("Unmarshal = %v, want %v", goValue.Duration, wantDuration)
				}
			})

			t.Run("UnmarshalRead", func(t *testing.T) {
				switch err := UnmarshalRead(strings.NewReader(jsonValue), &goValue, opts...); {
				case err != nil:
					t.Fatalf("UnmarshalRead error: %v", err)
				case goValue.Duration != wantDuration:
					t.Fatalf("UnmarshalRead = %v, want %v", goValue.Duration, wantDuration)
				}
			})

			t.Run("UnmarshalDecode", func(t *testing.T) {
				dec := jsontext.NewDecoder(strings.NewReader(jsonValue))
				switch err := UnmarshalDecode(dec, &goValue, opts...); {
				case err != nil:
					t.Fatalf("UnmarshalDecode error: %v", err)
				case goValue.Duration != wantDuration:
					t.Fatalf("UnmarshalDecode = %v, want %v", goValue.Duration, wantDuration)
				}
			})
		})
	}
}

func BenchmarkGlobalExperimentalFormatSupport(b *testing.B) {
	defer ExperimentalGlobalSupportFormatTag(false)
	for _, enabled := range []bool{false, true} {
		ExperimentalGlobalSupportFormatTag(enabled)
		b.Run(fmt.Sprintf("enable:%v", enabled), func(b *testing.B) {
			boolVal := new(bool)
			jsonVal := []byte(`false`)
			b.Run("Marshal", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					jsonVal, _ = Marshal(boolVal)
				}
			})
			b.Run("Unmarshal", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					Unmarshal(jsonVal, boolVal)
				}
			})
		})
	}
}
