// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build goexperiment.rangefunc

package jsontext

import (
	"iter"
	"slices"
	"testing"
)

func TestPointerTokens(t *testing.T) {
	// TODO(https://go.dev/issue/61899): Use slices.Collect.
	collect := func(seq iter.Seq[string]) (x []string) {
		for v := range seq {
			x = append(x, v)
		}
		return x
	}

	tests := []struct {
		in   Pointer
		want []string
	}{
		{in: "", want: nil},
		{in: "a", want: []string{"a"}},
		{in: "~", want: []string{"~"}},
		{in: "/a", want: []string{"a"}},
		{in: "/foo/bar", want: []string{"foo", "bar"}},
		{in: "///", want: []string{"", "", ""}},
		{in: "/~0~1", want: []string{"~/"}},
	}
	for _, tt := range tests {
		got := collect(tt.in.Tokens())
		if !slices.Equal(got, tt.want) {
			t.Errorf("Pointer(%q).Tokens = %q, want %q", tt.in, got, tt.want)
		}
	}
}
