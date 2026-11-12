// Copyright (c) 2026, go-avkit
// SPDX-License-Identifier: BSD-3-Clause

package bitstream

import (
	"errors"
	"math/rand"
	"testing"
)

// refBits is the definition: n bits from bit position pos, most significant
// first, zeros past the end.
func refBits(data []byte, pos, n int) uint32 {
	var v uint32
	for i := 0; i < n; i++ {
		p := pos + i
		var b uint32
		if p>>3 < len(data) {
			b = uint32(data[p>>3]>>(7-uint(p&7))) & 1
		}
		v = v<<1 | b
	}
	return v
}

// TestWindowedReadsMatchTheDefinition: Bits, Peek and PeekPadded at every
// position and every width up to 32, over data of every length from 0 to 12 bytes
// (so the end of the data falls inside, at and past every window), against the
// bit-by-bit definition.
func TestWindowedReadsMatchTheDefinition(t *testing.T) {
	rng := rand.New(rand.NewSource(21))
	for size := 0; size <= 12; size++ {
		data := make([]byte, size)
		rng.Read(data)
		for pos := 0; pos <= size*8; pos++ {
			for n := 0; n <= 32; n++ {
				want := refBits(data, pos, n)
				r := NewReader(data)
				if err := r.Skip(pos); err != nil {
					t.Fatalf("size %d: Skip(%d): %v", size, pos, err)
				}
				pp, err := r.PeekPadded(n)
				if err != nil || pp != want {
					t.Fatalf("size %d pos %d n %d: PeekPadded = %#x, %v; want %#x", size, pos, n, pp, err, want)
				}
				if r.Pos() != pos {
					t.Fatalf("PeekPadded moved to %d from %d", r.Pos(), pos)
				}
				p, perr := r.Peek(n)
				bv, berr := r.Bits(n)
				if n > size*8-pos {
					if !errors.Is(perr, ErrShort) || !errors.Is(berr, ErrShort) {
						t.Fatalf("size %d pos %d n %d: Peek, Bits = %v, %v, want ErrShort", size, pos, n, perr, berr)
					}
					if r.Pos() != size*8 {
						t.Fatalf("size %d pos %d n %d: a short Bits left the position at %d, want the end", size, pos, n, r.Pos())
					}
					continue
				}
				if perr != nil || berr != nil || p != want || bv != want || r.Pos() != pos+n {
					t.Fatalf("size %d pos %d n %d: Peek %#x %v, Bits %#x %v, pos %d; want %#x", size, pos, n, p, perr, bv, berr, r.Pos(), want)
				}
			}
		}
	}
}

func TestSkipAndAlign(t *testing.T) {
	r := NewReader([]byte{0xAB, 0xCD, 0xEF})
	if err := r.Skip(0); err != nil || r.Pos() != 0 {
		t.Fatalf("Skip(0): %v, pos %d", err, r.Pos())
	}
	r.Align() // already aligned
	if r.Pos() != 0 {
		t.Fatalf("Align on a boundary moved to %d", r.Pos())
	}
	if err := r.Skip(3); err != nil {
		t.Fatal(err)
	}
	r.Align()
	if r.Pos() != 8 {
		t.Fatalf("Align from 3 gave %d, want 8", r.Pos())
	}
	if v, _ := r.Bits(8); v != 0xCD {
		t.Fatalf("after Align read %#x, want 0xcd", v)
	}
	if err := r.Skip(9); !errors.Is(err, ErrShort) || r.Pos() != 16 {
		t.Fatalf("Skip past the end: %v, pos %d; want ErrShort and no move", err, r.Pos())
	}
	if err := r.Skip(-1); !errors.Is(err, ErrWidth) {
		t.Fatalf("Skip(-1) = %v, want ErrWidth", err)
	}
	if err := r.Skip(8); err != nil || r.Left() != 0 {
		t.Fatalf("Skip to the end: %v, left %d", err, r.Left())
	}
}

func TestPeekPaddedWidth(t *testing.T) {
	r := NewReader([]byte{0xFF})
	for _, n := range []int{-1, 33} {
		if _, err := r.PeekPadded(n); !errors.Is(err, ErrWidth) {
			t.Errorf("PeekPadded(%d) = %v, want ErrWidth", n, err)
		}
	}
	if v, err := r.PeekPadded(0); v != 0 || err != nil {
		t.Errorf("PeekPadded(0) = %d, %v", v, err)
	}
}
