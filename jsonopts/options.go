// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !goexperiment.jsonv2 || !go1.25

package jsonopts

import (
	"github.com/go-json-experiment/json/internal/jsonopts"
)

// Option represents either a single or a set of options.
//
// It can be functionally thought of as a Go map of option properties
// (even though the underlying implementation avoids Go maps for performance).
//
// The constructors (e.g., Deterministic) return a singular option value:
//
//	opt := Deterministic(true)
//
// which is analogous to creating a single entry map:
//
//	opt := Options{"Deterministic": true}
//
// [JoinOptions] composes multiple options values to together:
//
//	out := JoinOptions(opts...)
//
// which is analogous to making a new map and copying the options over:
//
//	out := make(Options)
//	for _, m := range opts {
//		for k, v := range m {
//			out[k] = v
//		}
//	}
//
// [GetOption] looks up the value of options parameter:
//
//	v, ok := GetOption(opts, Deterministic)
//
// which is analogous to a Go map lookup:
//
//	v, ok := Options["Deterministic"]
//
// The only way it represents a set of options
// is through the concrete [Options] type.
type Option = jsonopts.Option

// Options represents a set of zero or more options.
// The zero value of Options is an empty set.
// Once options is constructed, it is immutable.
type Options = jsonopts.Options

// Join is semantically equivalent to:
//
//	var os Options // zero value of options
//	return os.Join(opts...)
func Join(opts ...Option) Options {
	return jsonopts.JoinOptions(opts...)
}

// Get retrieves option T,
// returning the zero value if T is not in the set.
func Get[T Option](os Options) T {
	// TODO: Switch to generic method on Options.
	v, _ := jsonopts.GetOption[T](os)
	return v
}

// Has reports whether option T is in the set.
func Has[T Option](os Options) bool {
	// TODO: Switch to generic method on Options.
	_, ok := jsonopts.GetOption[T](os)
	return ok
}
