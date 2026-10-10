// Copyright (c) 2026, go-avkit
// SPDX-License-Identifier: BSD-3-Clause

package bitstream

import (
	"errors"
	"testing"
)

// TestAWidthTheValueCannotHold.
//
// ⛔ Bits shifts into a uint32. Asking for 33 used to give the SECOND bit to
// the thirty-third -- the top bit shifted out in silence -- with no error and
// the position correctly advanced, so nothing downstream could notice. The
// value returned was a field nobody asked for.
//
// A negative width read nothing and said nothing, which is how a width
// computed one field too early arrives here.
func TestAWidthTheValueCannotHold(t *testing.T) {
	data := []byte{0xDE, 0xAD, 0xBE, 0xEF, 0xCA, 0xFE, 0xBA, 0xBE}
	for _, n := range []int{-1, -32, 33, 40, 64, 1 << 20} {
		r := NewReader(data)
		v, err := r.Bits(n)
		if !errors.Is(err, ErrWidth) {
			t.Errorf("Bits(%d) = %#x, err %v; want ErrWidth", n, v, err)
		}
		if r.Pos() != 0 {
			t.Errorf("Bits(%d) consumed %d bits while refusing", n, r.Pos())
		}
	}
	// Peek goes through Bits, so it inherits the refusal.
	r := NewReader(data)
	if _, err := r.Peek(33); !errors.Is(err, ErrWidth) {
		t.Errorf("Peek(33): err = %v, want ErrWidth", err)
	}
}

// TestTheWidthsThatDoFitAreStillRead: zero and thirty-two are the ends of what
// the value holds, and refusing either would refuse conformant streams.
func TestTheWidthsThatDoFitAreStillRead(t *testing.T) {
	data := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	for _, c := range []struct {
		n    int
		want uint32
		pos  int
	}{
		{0, 0, 0},
		{1, 1, 1},
		{8, 0xDE, 8},
		{31, 0x6F56DF77, 31},
		{32, 0xDEADBEEF, 32},
	} {
		r := NewReader(data)
		v, err := r.Bits(c.n)
		if err != nil {
			t.Errorf("Bits(%d) was refused: %v", c.n, err)
			continue
		}
		if v != c.want || r.Pos() != c.pos {
			t.Errorf("Bits(%d) = %#x at %d, want %#x at %d", c.n, v, r.Pos(), c.want, c.pos)
		}
	}
}

// TestTheExpGolombReaderStillReachesItsLargestValue.
//
// ⛔ UE reads a leading-zero count and then that many bits, so a bound on Bits
// could cut the largest value the code can state. 2^32-1 needs a 32-bit
// suffix: 32 zeros, a one, then 32 zeros, sixty-five bits in all.
func TestTheExpGolombReaderStillReachesItsLargestValue(t *testing.T) {
	var b []byte
	var cur byte
	n := 0
	put := func(bit byte) {
		cur = cur<<1 | bit
		n++
		if n == 8 {
			b = append(b, cur)
			cur, n = 0, 0
		}
	}
	for i := 0; i < 32; i++ {
		put(0)
	}
	put(1)
	for i := 0; i < 32; i++ {
		put(0)
	}
	for n != 0 {
		put(0)
	}
	v, err := NewReader(b).UE()
	if err != nil {
		t.Fatalf("the largest Exp-Golomb value was refused: %v", err)
	}
	if v != 1<<32-1 {
		t.Errorf("UE = %d, want %d", v, uint32(1<<32-1))
	}
}
