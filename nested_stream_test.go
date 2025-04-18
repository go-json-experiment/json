package json_test

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/rogpeppe/ioseq"

	"github.com/go-json-experiment/json"
	"github.com/go-json-experiment/json/jsontext"
)

func TestNestedJSONEncoding(t *testing.T) {
	sample := Sample{
		Nested: jsonValue(Inner{
			X: 99,
			Other: jsonValue(&Inner2{
				Foo: `something "quoted"`,
				Big: StreamingString("this might be very big"),
			}),
		}),
	}
	data, err := json.Marshal(sample)
	if err != nil {
		t.Fatal(err)
	}
	var logBuf strings.Builder

	dec := jsontext.NewDecoder(iotest.OneByteReader(bytes.NewReader(data)), json.WithUnmarshalers(
		json.UnmarshalFromFunc(func(dec *jsontext.Decoder, s *StreamingString) error {
			if dec.PeekKind() != '"' {
				return fmt.Errorf("want string for streaming")
			}
			var buf strings.Builder
			for data, err := range dec.ReadStringAsSeq() {
				if err != nil {
					return err
				}
				fmt.Fprintf(&logBuf, "[%s]", data)
				buf.Write(data)
			}
			*s = StreamingString(buf.String())
			return nil
		}),
	))
	var sample1 Sample
	if err := json.UnmarshalDecode(dec, &sample1); err != nil {
		t.Fatalf("cannot unmarshal into sample1: %v", err)
	}

	if !reflect.DeepEqual(sample, sample1) {
		t.Fatal("failed to round trip")
	}
	if logBuf.String() != "[t][h][i][s][ ][m][i][g][h][t][ ][b][e][ ][v][e][r][y][ ][b][i][g]" {
		t.Fatalf("unexpected streaming log contents: %q", &logBuf)
	}

}

type StreamingString string

func jsonValue[T any](x T) EncodedJSON[T] {
	return EncodedJSON[T]{x}
}

type EncodedJSON[T any] = Encoded[T, JSONCodec]

type JSONCodec struct{}

func (JSONCodec) Unmarshal(r io.Reader, opts jsontext.Options, dst any) error {
	if err := json.UnmarshalRead(r, dst, opts); err != nil {
		return fmt.Errorf("JSONCodec.Unmarshal: (%T) %w", err, err)
	}
	return nil
}

func (JSONCodec) Marshal(w io.Writer, opts jsontext.Options, dst any) error {
	return json.MarshalWrite(w, dst)
}

type Sample struct {
	Nested EncodedJSON[Inner]
}

type Inner struct {
	X     int
	Other EncodedJSON[*Inner2]
}

type Inner2 struct {
	Foo string
	Bar string
	Big StreamingString
}

type Codec interface {
	Unmarshal(r io.Reader, opts jsontext.Options, dst any) error
	Marshal(w io.Writer, opts jsontext.Options, dst any) error
}

// Encoded represents an encoded string, encoded
// with the given Codec. The methods on Codec must
// be usable on its zero value.
type Encoded[T any, C Codec] struct {
	Data T
}

func (e *Encoded[T, C]) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	if dec.PeekKind() != '"' {
		return fmt.Errorf("unexpected token type %v", dec.PeekKind())
	}

	r := ioseq.ReaderFromSeq(dec.ReadStringAsSeq())
	defer r.Close()
	var c C
	if err := c.Unmarshal(r, dec.Options(), &e.Data); err != nil {
		return fmt.Errorf("Encoded.UnmarshalJSONFrom Unmarshal: %w", err)
	}
	return nil
}

func (e Encoded[T, C]) MarshalJSONTo(enc *jsontext.Encoder) error {
	// TODO there could/should be a streaming alternative to WriteToken
	var buf strings.Builder
	var c C
	if err := c.Marshal(&buf, enc.Options(), e.Data); err != nil {
		return err
	}
	return enc.WriteToken(jsontext.String(buf.String()))
}
