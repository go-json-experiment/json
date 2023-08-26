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
	"github.com/go-json-experiment/json/internal/jsonopts"
)

type (
	Options = jsonopts.Options

	Encoder = json.Encoder
	Decoder = json.Decoder

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

func NewEncoder(w io.Writer, opts ...Options) *Encoder { return json.NewEncoder(w, opts...) }
func NewDecoder(r io.Reader, opts ...Options) *Decoder { return json.NewDecoder(r, opts...) }

func AllowDuplicateNames(v bool) Options        { return json.AllowDuplicateNames(v) }
func AllowInvalidUTF8(v bool) Options           { return json.AllowInvalidUTF8(v) }
func EscapeForHTML(v bool) Options              { return json.EscapeForHTML(v) }
func EscapeForJS(v bool) Options                { return json.EscapeForJS(v) }
func WithEscapeFunc(fn func(rune) bool) Options { return json.WithEscapeFunc(fn) }
func Expand(v bool) Options                     { return json.Expand(v) }
func WithIndent(indent string) Options          { return json.WithIndent(indent) }
func WithIndentPrefix(prefix string) Options    { return json.WithIndentPrefix(prefix) }

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
