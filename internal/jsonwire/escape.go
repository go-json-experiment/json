// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package jsonwire

import "unicode/utf8"

// Validity of these checked in TestEscapeRunesTables.
var (
	escapeCanonical = EscapeRunes{
		asciiCache: [...]int8{
			-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1,
			-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1,
			00, 00, -1, 00, 00, 00, 00, 00, 00, 00, 00, 00, 00, 00, 00, 00,
			00, 00, 00, 00, 00, 00, 00, 00, 00, 00, 00, 00, 00, 00, 00, 00,
			00, 00, 00, 00, 00, 00, 00, 00, 00, 00, 00, 00, 00, 00, 00, 00,
			00, 00, 00, 00, 00, 00, 00, 00, 00, 00, 00, 00, -1, 00, 00, 00,
			00, 00, 00, 00, 00, 00, 00, 00, 00, 00, 00, 00, 00, 00, 00, 00,
			00, 00, 00, 00, 00, 00, 00, 00, 00, 00, 00, 00, 00, 00, 00, 00,
		},
		canonical: true,
	}
	escapeHTMLJS = EscapeRunes{
		asciiCache: [...]int8{
			-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1,
			-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1,
			00, 00, -1, 00, 00, 00, +1, 00, 00, 00, 00, 00, 00, 00, 00, 00,
			00, 00, 00, 00, 00, 00, 00, 00, 00, 00, 00, 00, +1, 00, +1, 00,
			00, 00, 00, 00, 00, 00, 00, 00, 00, 00, 00, 00, 00, 00, 00, 00,
			00, 00, 00, 00, 00, 00, 00, 00, 00, 00, 00, 00, -1, 00, 00, 00,
			00, 00, 00, 00, 00, 00, 00, 00, 00, 00, 00, 00, 00, 00, 00, 00,
			00, 00, 00, 00, 00, 00, 00, 00, 00, 00, 00, 00, 00, 00, 00, 00,
		},
		escapeHTML: true,
		escapeJS:   true,
	}
	escapeHTML = EscapeRunes{asciiCache: escapeHTMLJS.asciiCache, escapeHTML: true}
	escapeJS   = EscapeRunes{asciiCache: escapeCanonical.asciiCache, escapeJS: true}
)

// EscapeRunes reports whether a rune must be escaped.
type EscapeRunes struct {
	// asciiCache is a cache of whether an ASCII character must be escaped,
	// where 0 means not escaped, -1 escapes with the short sequence (e.g., \n),
	// and +1 escapes with the \uXXXX sequence.
	asciiCache [utf8.RuneSelf]int8

	canonical  bool // whether there are no custom escapes
	escapeHTML bool // should escape '<', '>', and '&'
	escapeJS   bool // should escape '\u2028' and '\u2029'

	escapeFunc func(rune) bool // arbitrary runes that need escaping; may be nil
}

// MakeEscapeRunes constructs an escape table for the escape parameters.
func MakeEscapeRunes(html, js bool, fn func(rune) bool) *EscapeRunes {
	if fn == nil {
		switch [2]bool{html, js} {
		case [2]bool{false, false}:
			return &escapeCanonical
		case [2]bool{true, true}:
			return &escapeHTMLJS
		case [2]bool{true, false}:
			return &escapeHTML
		case [2]bool{false, true}:
			return &escapeJS
		}
	}
	return makeEscapeRunesSlow(html, js, fn)
}

func makeEscapeRunesSlow(html, js bool, fn func(rune) bool) *EscapeRunes {
	e := EscapeRunes{escapeHTML: html, escapeJS: js, escapeFunc: fn}
	e.canonical = !e.escapeHTML && !e.escapeJS && e.escapeFunc == nil

	// Escape characters that are required by JSON.
	for i := 0; i < ' '; i++ {
		e.asciiCache[i] = -1
	}
	e.asciiCache['\\'] = -1
	e.asciiCache['"'] = -1

	// Escape characters with significance in HTML.
	if e.escapeHTML {
		e.asciiCache['<'] = +1
		e.asciiCache['>'] = +1
		e.asciiCache['&'] = +1
	}

	// Escape characters specified by the user-provided function.
	if e.escapeFunc != nil {
		for r := range e.asciiCache[:] {
			if e.escapeFunc(rune(r)) {
				e.asciiCache[r] = +1
			}
		}
	}

	return &e
}

// IsCanonical reports whether this uses canonical escaping,
// which is the minimal amount of escaping to produce a valid JSON string.
func (e *EscapeRunes) IsCanonical() bool { return e.canonical }

// HasEscapeFunc reports whether EscapeFunc is in use.
func (e *EscapeRunes) HasEscapeFunc() bool { return e.escapeFunc != nil }

// needEscapeASCII reports whether c must be escaped.
// It assumes c < utf8.RuneSelf.
func (e *EscapeRunes) needEscapeASCII(c byte) bool {
	return e.asciiCache[c] != 0
}

// needEscapeASCIIAsUTF16 reports whether c must be escaped using a \uXXXX sequence.
func (e *EscapeRunes) needEscapeASCIIAsUTF16(c byte) bool {
	return e.asciiCache[c] > 0
}

// needEscapeRune reports whether r must be escaped.
// It assumes r >= utf8.RuneSelf.
func (e *EscapeRunes) needEscapeRune(r rune) bool {
	return (e.escapeJS && (r == '\u2028' || r == '\u2029')) || (e.escapeFunc != nil && e.escapeFunc(r))
}
