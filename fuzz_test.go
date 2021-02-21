// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// NOTE: This file relies on experimental fuzzing support in the Go toolchain
// provided by a build of the "go" command from the "dev.fuzz" branch.
//
// To manually run the fuzzer:
//	go test -tags=dev.fuzz -fuzz=Coder
//
// +build dev.fuzz

package json

import (
	"bytes"
	"errors"
	"io"
	"math/rand"
	"testing"
)

func FuzzCoder(f *testing.F) {
	// Add a number of inputs to the corpus including valid and invalid data.
	for _, td := range coderTestdata {
		f.Add([]byte(td.in))
	}
	for _, td := range decoderErrorTestdata {
		f.Add([]byte(td.in))
	}
	for _, td := range encoderErrorTestdata {
		f.Add([]byte(td.wantOut))
	}
	for _, td := range benchTestdata {
		f.Add([]byte(td.data))
	}

	f.Fuzz(func(t *testing.T, b []byte) {
		var tokVals []tokOrVal
		// TODO: Use dedicated seed when structured arguments are supported.
		rn := rand.New(rand.NewSource(int64(len(b))))

		// Read a sequence of tokens or values. Skip the test for any errors
		// since we expect this with randomly generated fuzz inputs.
		src := bytes.NewReader(b)
		dec := NewDecoder(src)
		for {
			if rn.Intn(8) > 0 {
				tok, err := dec.ReadToken()
				if err != nil {
					if err == io.EOF {
						break
					}
					t.Skipf("Decoder.ReadToken error: %v", err)
				}
				tokVals = append(tokVals, tok.Clone())
			} else {
				val, err := dec.ReadValue()
				if err != nil {
					expectError := dec.PeekKind() == '}' || dec.PeekKind() == ']'
					if expectError && errors.As(err, new(*SyntaxError)) {
						continue
					}
					if err == io.EOF {
						break
					}
					t.Skipf("Decoder.ReadValue error: %v", err)
				}
				tokVals = append(tokVals, append(zeroValue, val...))
			}
		}

		// Write a sequence of tokens or values. Fail the test for any errors
		// since the previous stage guarantees that the input is valid.
		dst := new(bytes.Buffer)
		enc := NewEncoder(dst)
		for _, tokVal := range tokVals {
			switch tokVal := tokVal.(type) {
			case Token:
				if err := enc.WriteToken(tokVal); err != nil {
					t.Fatalf("Encoder.WriteToken error: %v", err)
				}
			case RawValue:
				if err := enc.WriteValue(tokVal); err != nil {
					t.Fatalf("Encoder.WriteValue error: %v", err)
				}
			}
		}

		// Encoded output and original input must decode to the same thing.
		var got, want []Token
		for dec := NewDecoder(bytes.NewReader(b)); dec.PeekKind() > 0; {
			tok, err := dec.ReadToken()
			if err != nil {
				t.Fatalf("Decoder.ReadToken error: %v", err)
			}
			got = append(got, tok.Clone())
		}
		for dec := NewDecoder(dst); dec.PeekKind() > 0; {
			tok, err := dec.ReadToken()
			if err != nil {
				t.Fatalf("Decoder.ReadToken error: %v", err)
			}
			want = append(want, tok.Clone())
		}
		if !equalTokens(got, want) {
			t.Fatalf("mismatching output:\ngot  %v\nwant %v", got, want)
		}
	})
}
