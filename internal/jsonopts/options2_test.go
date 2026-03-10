// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !goexperiment.jsonv2 || !go1.25

package jsonopts

import (
	"testing"

	"github.com/go-json-experiment/json/internal/jsonflags"
)

var sink bool

func BenchmarkGetBoolFlags(b *testing.B) {
	b.ReportAllocs()
	opts := DefaultOptionsV2
	for range b.N {
		sink = opts.Flags.Get(jsonflags.AllowDuplicateNames)
		sink = opts.Flags.Has(jsonflags.AllowDuplicateNames)
	}
}
