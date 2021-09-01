// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"testing/iotest"

	jsonv1 "encoding/json"
)

var benchV1 = os.Getenv("BENCHMARK_V1") != ""

var arshalTestdata = []struct {
	name   string
	raw    []byte
	val    interface{}
	new    func() interface{}
	skipV1 bool
}{{
	name: "Bool",
	raw:  []byte("true"),
	val:  addr(true),
	new:  func() interface{} { return new(bool) },
}, {
	name: "String",
	raw:  []byte(`"hello, world!"`),
	val:  addr("hello, world!"),
	new:  func() interface{} { return new(string) },
}, {
	name: "Int",
	raw:  []byte("-1234"),
	val:  addr(int64(-1234)),
	new:  func() interface{} { return new(int64) },
}, {
	name: "Uint",
	raw:  []byte("1234"),
	val:  addr(uint64(1234)),
	new:  func() interface{} { return new(uint64) },
}, {
	name: "Float",
	raw:  []byte("12.34"),
	val:  addr(float64(12.34)),
	new:  func() interface{} { return new(float64) },
}, {
	name: "Map/OneLarge",
	raw:  []byte(`{"A":"A","B":"B","C":"C","D":"D","E":"E","F":"F","G":"G","H":"H","I":"I","J":"J","K":"K","L":"L","M":"M","N":"N","O":"O","P":"P","Q":"Q","R":"R","S":"S","T":"T","U":"U","V":"V","W":"W","X":"X","Y":"Y","Z":"Z"}`),
	val:  addr(map[string]string{"A": "A", "B": "B", "C": "C", "D": "D", "E": "E", "F": "F", "G": "G", "H": "H", "I": "I", "J": "J", "K": "K", "L": "L", "M": "M", "N": "N", "O": "O", "P": "P", "Q": "Q", "R": "R", "S": "S", "T": "T", "U": "U", "V": "V", "W": "W", "X": "X", "Y": "Y", "Z": "Z"}),
	new:  func() interface{} { return new(map[string]string) },
}, {
	name: "Map/ManySmall",
	raw:  []byte(`{"A":{"K":"V"},"B":{"K":"V"},"C":{"K":"V"},"D":{"K":"V"},"E":{"K":"V"},"F":{"K":"V"},"G":{"K":"V"},"H":{"K":"V"},"I":{"K":"V"},"J":{"K":"V"},"K":{"K":"V"},"L":{"K":"V"},"M":{"K":"V"},"N":{"K":"V"},"O":{"K":"V"},"P":{"K":"V"},"Q":{"K":"V"},"R":{"K":"V"},"S":{"K":"V"},"T":{"K":"V"},"U":{"K":"V"},"V":{"K":"V"},"W":{"K":"V"},"X":{"K":"V"},"Y":{"K":"V"},"Z":{"K":"V"}}`),
	val:  addr(map[string]map[string]string{"A": {"K": "V"}, "B": {"K": "V"}, "C": {"K": "V"}, "D": {"K": "V"}, "E": {"K": "V"}, "F": {"K": "V"}, "G": {"K": "V"}, "H": {"K": "V"}, "I": {"K": "V"}, "J": {"K": "V"}, "K": {"K": "V"}, "L": {"K": "V"}, "M": {"K": "V"}, "N": {"K": "V"}, "O": {"K": "V"}, "P": {"K": "V"}, "Q": {"K": "V"}, "R": {"K": "V"}, "S": {"K": "V"}, "T": {"K": "V"}, "U": {"K": "V"}, "V": {"K": "V"}, "W": {"K": "V"}, "X": {"K": "V"}, "Y": {"K": "V"}, "Z": {"K": "V"}}),
	new:  func() interface{} { return new(map[string]map[string]string) },
}, {
	name: "Struct/OneLarge",
	raw:  []byte(`{"A":"A","B":"B","C":"C","D":"D","E":"E","F":"F","G":"G","H":"H","I":"I","J":"J","K":"K","L":"L","M":"M","N":"N","O":"O","P":"P","Q":"Q","R":"R","S":"S","T":"T","U":"U","V":"V","W":"W","X":"X","Y":"Y","Z":"Z"}`),
	val:  addr(struct{ A, B, C, D, E, F, G, H, I, J, K, L, M, N, O, P, Q, R, S, T, U, V, W, X, Y, Z string }{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N", "O", "P", "Q", "R", "S", "T", "U", "V", "W", "X", "Y", "Z"}),
	new: func() interface{} {
		return new(struct{ A, B, C, D, E, F, G, H, I, J, K, L, M, N, O, P, Q, R, S, T, U, V, W, X, Y, Z string })
	},
}, {
	name: "Struct/ManySmall",
	raw:  []byte(`{"A":{"K":"V"},"B":{"K":"V"},"C":{"K":"V"},"D":{"K":"V"},"E":{"K":"V"},"F":{"K":"V"},"G":{"K":"V"},"H":{"K":"V"},"I":{"K":"V"},"J":{"K":"V"},"K":{"K":"V"},"L":{"K":"V"},"M":{"K":"V"},"N":{"K":"V"},"O":{"K":"V"},"P":{"K":"V"},"Q":{"K":"V"},"R":{"K":"V"},"S":{"K":"V"},"T":{"K":"V"},"U":{"K":"V"},"V":{"K":"V"},"W":{"K":"V"},"X":{"K":"V"},"Y":{"K":"V"},"Z":{"K":"V"}}`),
	val: func() interface{} {
		V := struct{ K string }{"V"}
		return addr(struct{ A, B, C, D, E, F, G, H, I, J, K, L, M, N, O, P, Q, R, S, T, U, V, W, X, Y, Z struct{ K string } }{
			V, V, V, V, V, V, V, V, V, V, V, V, V, V, V, V, V, V, V, V, V, V, V, V, V, V,
		})
	}(),
	new: func() interface{} {
		return new(struct{ A, B, C, D, E, F, G, H, I, J, K, L, M, N, O, P, Q, R, S, T, U, V, W, X, Y, Z struct{ K string } })
	},
}, {
	name: "Slice/OneLarge",
	raw:  []byte(`["A","B","C","D","E","F","G","H","I","J","K","L","M","N","O","P","Q","R","S","T","U","V","W","X","Y","Z"]`),
	val:  addr([]string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N", "O", "P", "Q", "R", "S", "T", "U", "V", "W", "X", "Y", "Z"}),
	new:  func() interface{} { return new([]string) },
}, {
	name: "Slice/ManySmall",
	raw:  []byte(`[["A"],["B"],["C"],["D"],["E"],["F"],["G"],["H"],["I"],["J"],["K"],["L"],["M"],["N"],["O"],["P"],["Q"],["R"],["S"],["T"],["U"],["V"],["W"],["X"],["Y"],["Z"]]`),
	val:  addr([][]string{{"A"}, {"B"}, {"C"}, {"D"}, {"E"}, {"F"}, {"G"}, {"H"}, {"I"}, {"J"}, {"K"}, {"L"}, {"M"}, {"N"}, {"O"}, {"P"}, {"Q"}, {"R"}, {"S"}, {"T"}, {"U"}, {"V"}, {"W"}, {"X"}, {"Y"}, {"Z"}}),
	new:  func() interface{} { return new([][]string) },
}, {
	name: "Array/OneLarge",
	raw:  []byte(`["A","B","C","D","E","F","G","H","I","J","K","L","M","N","O","P","Q","R","S","T","U","V","W","X","Y","Z"]`),
	val:  addr([26]string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N", "O", "P", "Q", "R", "S", "T", "U", "V", "W", "X", "Y", "Z"}),
	new:  func() interface{} { return new([26]string) },
}, {
	name: "Array/ManySmall",
	raw:  []byte(`[["A"],["B"],["C"],["D"],["E"],["F"],["G"],["H"],["I"],["J"],["K"],["L"],["M"],["N"],["O"],["P"],["Q"],["R"],["S"],["T"],["U"],["V"],["W"],["X"],["Y"],["Z"]]`),
	val:  addr([26][1]string{{"A"}, {"B"}, {"C"}, {"D"}, {"E"}, {"F"}, {"G"}, {"H"}, {"I"}, {"J"}, {"K"}, {"L"}, {"M"}, {"N"}, {"O"}, {"P"}, {"Q"}, {"R"}, {"S"}, {"T"}, {"U"}, {"V"}, {"W"}, {"X"}, {"Y"}, {"Z"}}),
	new:  func() interface{} { return new([26][1]string) },
}, {
	name: "Bytes/Slice",
	raw:  []byte(`"47DEQpj8HBSa+/TImW+5JCeuQeRkm5NMpJWZG3hSuFU="`),
	val:  addr([]byte{0xe3, 0xb0, 0xc4, 0x42, 0x98, 0xfc, 0x1c, 0x14, 0x9a, 0xfb, 0xf4, 0xc8, 0x99, 0x6f, 0xb9, 0x24, 0x27, 0xae, 0x41, 0xe4, 0x64, 0x9b, 0x93, 0x4c, 0xa4, 0x95, 0x99, 0x1b, 0x78, 0x52, 0xb8, 0x55}),
	new:  func() interface{} { return new([]byte) },
}, {
	name:   "Bytes/Array",
	raw:    []byte(`"47DEQpj8HBSa+/TImW+5JCeuQeRkm5NMpJWZG3hSuFU="`),
	val:    addr([32]byte{0xe3, 0xb0, 0xc4, 0x42, 0x98, 0xfc, 0x1c, 0x14, 0x9a, 0xfb, 0xf4, 0xc8, 0x99, 0x6f, 0xb9, 0x24, 0x27, 0xae, 0x41, 0xe4, 0x64, 0x9b, 0x93, 0x4c, 0xa4, 0x95, 0x99, 0x1b, 0x78, 0x52, 0xb8, 0x55}),
	new:    func() interface{} { return new([32]byte) },
	skipV1: true,
}, {
	name: "Pointer",
	raw:  []byte("true"),
	val:  addr(addr(addr(addr(addr(addr(addr(addr(addr(addr(addr(true))))))))))),
	new:  func() interface{} { return new(**********bool) },
}, {
	name: "TextArshal",
	raw:  []byte(`"method"`),
	val:  new(textArshaler),
	new:  func() interface{} { return new(textArshaler) },
}, {
	name: "JSONArshalV1",
	raw:  []byte(`"method"`),
	val:  new(jsonArshalerV1),
	new:  func() interface{} { return new(jsonArshalerV1) },
}, {
	name:   "JSONArshalV2",
	raw:    []byte(`"method"`),
	val:    new(jsonArshalerV2),
	new:    func() interface{} { return new(jsonArshalerV2) },
	skipV1: true,
}}

type textArshaler struct{}

func (*textArshaler) MarshalText() ([]byte, error) {
	return []byte("method"), nil
}
func (*textArshaler) UnmarshalText(b []byte) error {
	if string(b) != "method" {
		return fmt.Errorf("UnmarshalText: got %q, want %q", b, "method")
	}
	return nil
}

type jsonArshalerV1 struct{}

func (*jsonArshalerV1) MarshalJSON() ([]byte, error) {
	return []byte(`"method"`), nil
}
func (*jsonArshalerV1) UnmarshalJSON(b []byte) error {
	if string(b) != `"method"` {
		return fmt.Errorf("UnmarshalJSON: got %q, want %q", b, `"method"`)
	}
	return nil
}

type jsonArshalerV2 struct{}

func (*jsonArshalerV2) MarshalNextJSON(enc *Encoder, opts MarshalOptions) error {
	return enc.WriteToken(String("method"))
}
func (*jsonArshalerV2) UnmarshalNextJSON(dec *Decoder, opts UnmarshalOptions) error {
	b, err := dec.ReadValue()
	if string(b) != `"method"` {
		return fmt.Errorf("UnmarshalNextJSON: got %q, want %q", b, `"method"`)
	}
	return err
}

func BenchmarkUnmarshal(b *testing.B) {
	for _, tt := range arshalTestdata {
		b.Run(tt.name, func(b *testing.B) {
			// Initial setup.
			unmarshal := Unmarshal
			if benchV1 {
				if tt.skipV1 {
					b.Skip("not supported in v1")
				}
				unmarshal = jsonv1.Unmarshal
			}

			// Run the benchmark.
			var val interface{}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				val = tt.new()
				if err := unmarshal(tt.raw, val); err != nil {
					b.Fatalf("Unmarshal error: %v", err)
				}
			}

			// Verify the results.
			b.StopTimer()
			if !reflect.DeepEqual(val, tt.val) {
				b.Fatalf("Unmarshal output mismatch:\ngot  %v\nwant %v", val, tt.val)
			}
		})
	}
}

func BenchmarkMarshal(b *testing.B) {
	for _, tt := range arshalTestdata {
		b.Run(tt.name, func(b *testing.B) {
			// Initial setup.
			marshal := Marshal
			if benchV1 {
				if tt.skipV1 {
					b.Skip("not supported in v1")
				}
				marshal = jsonv1.Marshal
			}

			// Run the benchmark.
			var raw []byte
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				var err error
				raw, err = marshal(tt.val)
				if err != nil {
					b.Fatalf("Marshal error: %v", err)
				}
			}

			// Verify the results.
			b.StopTimer()
			if !bytes.Equal(raw, tt.raw) {
				// Map marshaling in v2 are non-deterministic.
				byteHistogram := func(b []byte) (h [256]int) {
					for _, c := range b {
						h[c]++
					}
					return h
				}
				if !(strings.HasPrefix(tt.name, "Map/") && byteHistogram(raw) == byteHistogram(tt.raw)) {
					b.Fatalf("Marshal output mismatch:\ngot  %s\nwant %s", raw, tt.raw)
				}
			}
		})
	}
}

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
	{"LargeWhitespace/Object", []byte(ws + "{" + ws + `"name1"` + ws + ":" + ws + `"value"` + ws + "," + ws + `"name2"` + ws + ":" + ws + `"value"` + ws + "}" + ws)},
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

func BenchmarkRawValue(b *testing.B) {
	var data []byte
	for _, ts := range benchTestdata {
		if ts.name == "StarcraftSettings" {
			data = ts.data
		}
	}

	b.Run("IsValid", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			RawValue(data).IsValid()
		}
	})

	methods := []struct {
		name   string
		format func(*RawValue) error
	}{
		{"Compact", (*RawValue).Compact},
		{"Indent", func(v *RawValue) error { return v.Indent("\t", "    ") }},
		{"Canonicalize", (*RawValue).Canonicalize},
	}

	for _, method := range methods {
		b.Run(method.name, func(b *testing.B) {
			v := RawValue(string(data))
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				v = append(v[:0], data...) // reset with original input
				if err := method.format(&v); err != nil {
					b.Errorf("RawValue.%v error: %v", method.name, err)
				}
			}
		})
		b.Run(path.Join(method.name, "Noop"), func(b *testing.B) {
			v := RawValue(string(data))
			method.format(&v)
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if err := method.format(&v); err != nil {
					b.Errorf("RawValue.%v error: %v", method.name, err)
				}
			}
		})
	}
}
