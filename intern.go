// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import (
	"encoding/binary"
	"math/bits"
)

// stringCache is a cache for strings converted from a []byte.
type stringCache [256]string // 256*unsafe.Sizeof(string("")) => 4KiB

// make returns the string form of b.
// It returns a pre-allocated string from c if present, otherwise
// it allocated a new string, inserts it into the cache, and returns it.
func (c *stringCache) make(b []byte) string {
	const (
		minCachedLen = 2   // single byte strings are already interned by the runtime
		maxCachedLen = 256 // large enough for UUIDs, IPv6 addresses, SHA-256 checksums, etc.
	)
	if c == nil || len(b) < minCachedLen || len(b) > maxCachedLen {
		return string(b)
	}

	// Compute a hash from the fixed-width prefix and suffix of the string.
	// This ensures hashing a string is a constant time operation.
	var lo, hi uint64
	switch {
	case len(b) >= 8:
		lo = uint64(binary.LittleEndian.Uint64(b[:8]))
		hi = uint64(binary.LittleEndian.Uint64(b[len(b)-8:]))
	case len(b) >= 4:
		lo = uint64(binary.LittleEndian.Uint32(b[:4]))
		hi = uint64(binary.LittleEndian.Uint32(b[len(b)-4:]))
	case len(b) >= 2:
		lo = uint64(binary.LittleEndian.Uint16(b[:2]))
		hi = uint64(binary.LittleEndian.Uint16(b[len(b)-2:]))
	}
	n := uint64(len(b))
	h := hash128(lo^n, hi^n) // include the length as part of the hash

	// Check the cache for the string.
	i := h % uint64(len(*c))
	if s := (*c)[i]; s == string(b) {
		return s
	}
	s := string(b)
	(*c)[i] = s
	return s
}

// hash128 returns the hash of two uint64s as a single uint64.
func hash128(lo, hi uint64) uint64 {
	// If avalanche=true, this is identical to XXH64 hash on a 16B string:
	//	var b [16]byte
	//	binary.LittleEndian.PutUint64(b[:8], lo)
	//	binary.LittleEndian.PutUint64(b[8:], hi)
	//	return xxhash.Sum64(b[:])
	const (
		prime1 = 0x9e3779b185ebca87
		prime2 = 0xc2b2ae3d27d4eb4f
		prime3 = 0x165667b19e3779f9
		prime4 = 0x85ebca77c2b2ae63
		prime5 = 0x27d4eb2f165667c5
	)
	h := prime5 + uint64(16)
	h ^= bits.RotateLeft64(lo*prime2, 31) * prime1
	h = bits.RotateLeft64(h, 27)*prime1 + prime4
	h ^= bits.RotateLeft64(hi*prime2, 31) * prime1
	h = bits.RotateLeft64(h, 27)*prime1 + prime4
	// Skip final mix (avalanche) step of XXH64 for performance reasons.
	// Empirical testing shows that the improvements in unbiased distribution
	// does not outweigh the extra cost in computational complexity.
	const avalanche = false
	if avalanche {
		h ^= h >> 33
		h *= prime2
		h ^= h >> 29
		h *= prime3
		h ^= h >> 32
	}
	return h
}
