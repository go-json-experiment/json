// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package jsontext implements syntactic processing of JSON.
//
// At present, the declarations in this package are aliases
// to the equivalent declarations in the v2 "json" package
// as those declarations will be directly moved here in the future.
package jsontext

import (
	"io"

	"github.com/go-json-experiment/json"
)

type (
	EncodeOptions = json.EncodeOptions
	Encoder       = json.Encoder

	DecodeOptions = json.DecodeOptions
	Decoder       = json.Decoder

	Kind  = json.Kind
	Token = json.Token
	Value = json.RawValue

	SyntacticError = json.SyntacticError
)

func AppendQuote[Bytes ~[]byte | ~string](dst []byte, src Bytes) ([]byte, error) {
	return json.AppendQuote(dst, src)
}

func AppendUnquote[Bytes ~[]byte | ~string](dst []byte, src Bytes) ([]byte, error) {
	return json.AppendUnquote(dst, src)
}

func NewEncoder(w io.Writer) *Encoder { return json.NewEncoder(w) }
func NewDecoder(r io.Reader) *Decoder { return json.NewDecoder(r) }

var (
	Null  Token = json.Null
	False Token = json.False
	True  Token = json.True

	ObjectStart Token = json.ObjectStart
	ObjectEnd   Token = json.ObjectEnd
	ArrayStart  Token = json.ArrayStart
	ArrayEnd    Token = json.ArrayEnd
)

func Bool(b bool) Token     { return json.Bool(b) }
func Float(n float64) Token { return json.Float(n) }
func Int(n int64) Token     { return json.Int(n) }
func String(s string) Token { return json.String(s) }
func Uint(n uint64) Token   { return json.Uint(n) }
