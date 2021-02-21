// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import (
	"bytes"
	"compress/gzip"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"testing/iotest"

	jsonv1 "encoding/json"
)

var benchV1 = os.Getenv("BENCHMARK_V1") != ""

var benchTestdata = func() (out []struct {
	name string
	data []byte
}) {
	fis, err := ioutil.ReadDir("testdata")
	if err != nil {
		panic(err)
	}
	sort.Slice(fis, func(i, j int) bool { return fis[i].Name() < fis[j].Name() })
	for _, fi := range fis {
		if !strings.HasSuffix(fi.Name(), ".json.gz") {
			break
		}

		// Convert snake_case file name to CamelCase.
		words := strings.Split(strings.TrimSuffix(fi.Name(), ".json.gz"), "_")
		for i := range words {
			words[i] = strings.Title(words[i])
		}
		name := strings.Join(words, "")

		// Read and decompress the test data.
		b, err := ioutil.ReadFile(filepath.Join("testdata", fi.Name()))
		if err != nil {
			panic(err)
		}
		zr, err := gzip.NewReader(bytes.NewReader(b))
		if err != nil {
			panic(err)
		}
		data, err := ioutil.ReadAll(zr)
		if err != nil {
			panic(err)
		}

		out = append(out, struct {
			name string
			data []byte
		}{name, data})
	}
	return out
}()

func TestTestdata(t *testing.T) {
	for _, td := range benchTestdata {
		tokens := mustDecodeTokens(t, td.data)
		buffer := make([]byte, 0, 2*len(td.data))
		for _, modeName := range []string{"Streaming", "Buffered"} {
			for _, typeName := range []string{"Token", "Value"} {
				t.Run(path.Join(td.name, modeName, typeName, "Encoder"), func(t *testing.T) {
					runEncoder(t, modeName, typeName, buffer, td.data, tokens)
				})
				t.Run(path.Join(td.name, modeName, typeName, "Decoder"), func(t *testing.T) {
					runDecoder(t, modeName, typeName, buffer, td.data, tokens)
				})
			}
		}
	}
}
func BenchmarkTestdata(b *testing.B) {
	for _, td := range benchTestdata {
		tokens := mustDecodeTokens(b, td.data)
		buffer := make([]byte, 0, 2*len(td.data))
		for _, modeName := range []string{"Streaming", "Buffered"} {
			for _, typeName := range []string{"Token", "Value"} {
				b.Run(path.Join(td.name, modeName, typeName, "Encoder"), func(b *testing.B) {
					b.ReportAllocs()
					b.SetBytes(int64(len(td.data)))
					for i := 0; i < b.N; i++ {
						runEncoder(b, modeName, typeName, buffer, td.data, tokens)
					}
				})
				b.Run(path.Join(td.name, modeName, typeName, "Decoder"), func(b *testing.B) {
					b.ReportAllocs()
					b.SetBytes(int64(len(td.data)))
					for i := 0; i < b.N; i++ {
						runDecoder(b, modeName, typeName, buffer, td.data, tokens)
					}
				})
			}
		}
	}
}

func mustDecodeTokens(t testing.TB, data []byte) []Token {
	var tokens []Token
	dec := NewDecoder(bytes.NewReader(data))
	for {
		tok, err := dec.ReadToken()
		if err != nil {
			if err == io.EOF {
				break
			}
			t.Fatalf("Decoder.ReadToken error: %v", err)
		}

		// Prefer exact representation for JSON strings and numbers
		// since this more closely matches common use cases.
		switch tok.Kind() {
		case '"':
			tokens = append(tokens, String(tok.String()))
		case '0':
			tokens = append(tokens, Float(tok.Float()))
		default:
			tokens = append(tokens, tok.Clone())
		}
	}
	return tokens
}

func runEncoder(t testing.TB, modeName, typeName string, buffer, data []byte, tokens []Token) {
	if benchV1 {
		if modeName == "Buffered" {
			t.Skip("no support for direct buffered input in v1")
		}
		enc := jsonv1.NewEncoder(bytes.NewBuffer(buffer[:0]))
		switch typeName {
		case "Token":
			t.Skip("no support for encoding tokens in v1; see https://golang.org/issue/40127")
		case "Value":
			val := jsonv1.RawMessage(data)
			if err := enc.Encode(val); err != nil {
				t.Fatalf("Decoder.Encode error: %v", err)
			}
		}
		return
	}

	var enc *Encoder
	switch modeName {
	case "Streaming":
		enc = NewEncoder(bytes.NewBuffer(buffer[:0]))
	case "Buffered":
		enc = NewEncoder((*bytes.Buffer)(nil))
		enc.wr = nil
		enc.buf = buffer[:0]
	}
	switch typeName {
	case "Token":
		for _, tok := range tokens {
			if err := enc.WriteToken(tok); err != nil {
				t.Fatalf("Encoder.WriteToken error: %v", err)
			}
		}
	case "Value":
		if err := enc.WriteValue(data); err != nil {
			t.Fatalf("Encoder.WriteValue error: %v", err)
		}
	}
}

func runDecoder(t testing.TB, modeName, typeName string, buffer, data []byte, tokens []Token) {
	if benchV1 {
		if modeName == "Buffered" {
			t.Skip("no support for direct buffered output in v1")
		}
		dec := jsonv1.NewDecoder(bytes.NewReader(data))
		switch typeName {
		case "Token":
			for {
				if _, err := dec.Token(); err != nil {
					if err == io.EOF {
						break
					}
					t.Fatalf("Decoder.Token error: %v", err)
				}
			}
		case "Value":
			var val jsonv1.RawMessage
			if err := dec.Decode(&val); err != nil {
				t.Fatalf("Decoder.Decode error: %v", err)
			}
		}
		return
	}

	var dec *Decoder
	switch modeName {
	case "Streaming":
		dec = NewDecoder(bytes.NewReader(data))
	case "Buffered":
		dec = NewDecoder((*bytes.Buffer)(nil))
		dec.rd = nil
		dec.buf = data
	}
	switch typeName {
	case "Token":
		for {
			if _, err := dec.ReadToken(); err != nil {
				if err == io.EOF {
					break
				}
				t.Fatalf("Decoder.ReadToken error: %v", err)
			}
		}
	case "Value":
		if _, err := dec.ReadValue(); err != nil {
			t.Fatalf("Decoder.ReadValue error: %v", err)
		}
	}
}

var ws = strings.Repeat(" ", 4<<10)
var slowStreamingDecoderTestdata = []struct {
	name string
	data []byte
}{
	{"LargeString", []byte(`"` + strings.Repeat(" ", 4<<10) + `"`)},
	{"LargeNumber", []byte("0." + strings.Repeat("0", 4<<10))},
	{"LargeWhitespace/Null", []byte(ws + "null" + ws)},
	{"LargeWhitespace/Object", []byte(ws + "{" + ws + `"name"` + ws + ":" + ws + `"value"` + ws + "," + ws + `"name"` + ws + ":" + ws + `"value"` + ws + "}" + ws)},
	{"LargeWhitespace/Array", []byte(ws + "[" + ws + `"value"` + ws + "," + ws + `"value"` + ws + "]" + ws)},
}

func TestSlowStreamingDecoder(t *testing.T) {
	for _, td := range slowStreamingDecoderTestdata {
		for _, typeName := range []string{"Token", "Value"} {
			t.Run(path.Join(td.name, typeName), func(t *testing.T) {
				runSlowStreamingDecoder(t, typeName, td.data)
			})
		}
	}
}
func BenchmarkSlowStreamingDecoder(b *testing.B) {
	// TODO: The decoder has quadratic behavior on large JSON strings or numbers
	// when reading from a stream that only returns a few bytes at a time.
	// Fix this by adding support for preserving the parsing state
	// whenever we encounter a truncated JSON value.
	for _, td := range slowStreamingDecoderTestdata {
		for _, typeName := range []string{"Token", "Value"} {
			b.Run(path.Join(td.name, typeName), func(b *testing.B) {
				b.ReportAllocs()
				b.SetBytes(int64(len(td.data)))
				for i := 0; i < b.N; i++ {
					runSlowStreamingDecoder(b, typeName, td.data)
				}
			})
		}
	}
}

// runSlowStreamingDecoder tests a streaming Decoder operating on
// a slow io.Reader that only returns 1 byte at a time,
// which tends to exercise pathological behavior.
func runSlowStreamingDecoder(t testing.TB, typeName string, data []byte) {
	if benchV1 {
		dec := jsonv1.NewDecoder(iotest.OneByteReader(bytes.NewReader(data)))
		switch typeName {
		case "Token":
			for dec.More() {
				if _, err := dec.Token(); err != nil {
					t.Fatalf("Decoder.Token error: %v", err)
				}
			}
		case "Value":
			var val jsonv1.RawMessage
			if err := dec.Decode(&val); err != nil {
				t.Fatalf("Decoder.Decode error: %v", err)
			}
		}
		return
	}

	dec := NewDecoder(iotest.OneByteReader(bytes.NewReader(data)))
	switch typeName {
	case "Token":
		for dec.PeekKind() > 0 {
			if _, err := dec.ReadToken(); err != nil {
				t.Fatalf("Decoder.ReadToken error: %v", err)
			}
		}
	case "Value":
		if _, err := dec.ReadValue(); err != nil {
			t.Fatalf("Decoder.ReadValue error: %v", err)
		}
	}
}
