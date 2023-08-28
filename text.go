// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import (
	"io"

	"github.com/go-json-experiment/json/jsontext"
)

// Deprecated: Use [github.com/go-json-experiment/json/jsontext.Encoder] instead.
type Encoder = jsontext.Encoder

// Deprecated: Use [github.com/go-json-experiment/json/jsontext.Decoder] instead.
type Decoder = jsontext.Decoder

// Deprecated: Use [github.com/go-json-experiment/json/jsontext.Kind] instead.
type Kind = jsontext.Kind

// Deprecated: Use [github.com/go-json-experiment/json/jsontext.Token] instead.
type Token = jsontext.Token

// Deprecated: Use [github.com/go-json-experiment/json/jsontext.Value] instead.
type RawValue = jsontext.Value

// Deprecated: Use [github.com/go-json-experiment/json/jsontext.SyntacticError] instead.
type SyntacticError = jsontext.SyntacticError

// Deprecated: Use [github.com/go-json-experiment/json/jsontext.AppendQuote] instead.
func AppendQuote[Bytes ~[]byte | ~string](dst []byte, src Bytes) ([]byte, error) {
	return jsontext.AppendQuote(dst, src)
}

// Deprecated: Use [github.com/go-json-experiment/json/jsontext.AppendUnquote] instead.
func AppendUnquote[Bytes ~[]byte | ~string](dst []byte, src Bytes) ([]byte, error) {
	return jsontext.AppendUnquote(dst, src)
}

// Deprecated: Use [github.com/go-json-experiment/json/jsontext.NewEncoder] instead.
func NewEncoder(w io.Writer, opts ...Options) *Encoder { return jsontext.NewEncoder(w, opts...) }

// Deprecated: Use [github.com/go-json-experiment/json/jsontext.NewDecoder] instead.
func NewDecoder(r io.Reader, opts ...Options) *Decoder { return jsontext.NewDecoder(r, opts...) }

// Deprecated: Use [github.com/go-json-experiment/json/jsontext.AllowDuplicateNames] instead.
func AllowDuplicateNames(v bool) Options { return jsontext.AllowDuplicateNames(v) }

// Deprecated: Use [github.com/go-json-experiment/json/jsontext.AllowInvalidUTF8] instead.
func AllowInvalidUTF8(v bool) Options { return jsontext.AllowInvalidUTF8(v) }

// Deprecated: Use [github.com/go-json-experiment/json/jsontext.EscapeForHTML] instead.
func EscapeForHTML(v bool) Options { return jsontext.EscapeForHTML(v) }

// Deprecated: Use [github.com/go-json-experiment/json/jsontext.EscapeForJS] instead.
func EscapeForJS(v bool) Options { return jsontext.EscapeForJS(v) }

// Deprecated: Use [github.com/go-json-experiment/json/jsontext.WithEscapeFunc] instead.
func WithEscapeFunc(fn func(rune) bool) Options { return jsontext.WithEscapeFunc(fn) }

// Deprecated: Use [github.com/go-json-experiment/json/jsontext.Expand] instead.
func Expand(v bool) Options { return jsontext.Expand(v) }

// Deprecated: Use [github.com/go-json-experiment/json/jsontext.WithIndent] instead.
func WithIndent(indent string) Options { return jsontext.WithIndent(indent) }

// Deprecated: Use [github.com/go-json-experiment/json/jsontext.WithIndentPrefix] instead.
func WithIndentPrefix(prefix string) Options { return jsontext.WithIndentPrefix(prefix) }

// Deprecated: Use [github.com/go-json-experiment/json/jsontext.Null] instead.
var Null Token = jsontext.Null

// Deprecated: Use [github.com/go-json-experiment/json/jsontext.False] instead.
var False Token = jsontext.False

// Deprecated: Use [github.com/go-json-experiment/json/jsontext.True] instead.
var True Token = jsontext.True

// Deprecated: Use [github.com/go-json-experiment/json/jsontext.ObjectStart] instead.
var ObjectStart Token = jsontext.ObjectStart

// Deprecated: Use [github.com/go-json-experiment/json/jsontext.ObjectEnd] instead.
var ObjectEnd Token = jsontext.ObjectEnd

// Deprecated: Use [github.com/go-json-experiment/json/jsontext.ArrayStart] instead.
var ArrayStart Token = jsontext.ArrayStart

// Deprecated: Use [github.com/go-json-experiment/json/jsontext.ArrayEnd] instead.
var ArrayEnd Token = jsontext.ArrayEnd

// Deprecated: Use [github.com/go-json-experiment/json/jsontext.Bool] instead.
func Bool(b bool) Token { return jsontext.Bool(b) }

// Deprecated: Use [github.com/go-json-experiment/json/jsontext.Float] instead.
func Float(n float64) Token { return jsontext.Float(n) }

// Deprecated: Use [github.com/go-json-experiment/json/jsontext.Int] instead.
func Int(n int64) Token { return jsontext.Int(n) }

// Deprecated: Use [github.com/go-json-experiment/json/jsontext.String] instead.
func String(s string) Token { return jsontext.String(s) }

// Deprecated: Use [github.com/go-json-experiment/json/jsontext.Uint] instead.
func Uint(n uint64) Token { return jsontext.Uint(n) }
