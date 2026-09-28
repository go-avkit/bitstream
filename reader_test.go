// Copyright (c) 2026, go-avkit
// SPDX-License-Identifier: BSD-3-Clause

package bitstream

import (
	"errors"
	"testing"
)

// fromBits packs a string of '0' and '1' into bytes, so a test states the code it
// means instead of a hex value nobody can check by eye.
func fromBits(s string) []byte {
	var out []byte
	for i, c := range s {
		if i%8 == 0 {
			out = append(out, 0)
		}
		if c == '1' {
			out[i/8] |= 1 << (7 - uint(i%8))
		}
	}
	return out
}

func TestUnsignedExpGolomb(t *testing.T) {
	for _, tc := range []struct {
		code string
		want uint32
	}{
		{"1", 0}, {"010", 1}, {"011", 2}, {"00100", 3}, {"00101", 4},
		{"00110", 5}, {"00111", 6}, {"0001000", 7}, {"0001111", 14},
		{"000010000", 15},
	} {
		got, err := NewReader(fromBits(tc.code)).UE()
		if err != nil {
			t.Errorf("%s: %v", tc.code, err)
			continue
		}
		if got != tc.want {
			t.Errorf("%s = %d, want %d", tc.code, got, tc.want)
		}
	}
}

func TestSignedExpGolomb(t *testing.T) {
	// The folding: an odd code is positive, an even one negative, and zero is the
	// only code that means zero.
	for _, tc := range []struct {
		code string
		want int32
	}{
		{"1", 0}, {"010", 1}, {"011", -1}, {"00100", 2},
		{"00101", -2}, {"00110", 3}, {"00111", -3},
	} {
		got, err := NewReader(fromBits(tc.code)).SE()
		if err != nil {
			t.Errorf("%s: %v", tc.code, err)
			continue
		}
		if got != tc.want {
			t.Errorf("%s = %d, want %d", tc.code, got, tc.want)
		}
	}
}

// TestACodeThatRunsOffTheEndIsRefused.
//
// ⛔ The promise has to outrun the BYTE, not just the bits written: a payload is
// byte-aligned, so a reader ends at a byte boundary and a code written short is
// completed by the padding of its own last byte. Seven leading zeros and a one
// fill one byte exactly and promise seven more bits that no second byte provides.
func TestACodeThatRunsOffTheEndIsRefused(t *testing.T) {
	if _, err := NewReader(fromBits("00000001")).UE(); !errors.Is(err, ErrShort) {
		t.Errorf("err = %v, want ErrShort", err)
	}
	// The premise of the paragraph above, asserted: within one byte the padding
	// does complete the code, and that is not a defect but what byte alignment
	// means.
	if v, err := NewReader(fromBits("00011")).UE(); err != nil {
		t.Errorf("a code completed by its own padding: %v", err)
	} else if v == 0 {
		t.Error("the padding read as nothing")
	}
	if _, err := NewReader(nil).Bit(); !errors.Is(err, ErrShort) {
		t.Errorf("empty: err = %v, want ErrShort", err)
	}
	if _, err := NewReader([]byte{0xFF}).Bits(9); !errors.Is(err, ErrShort) {
		t.Errorf("a width it cannot fill: err = %v, want ErrShort", err)
	}
	if _, err := NewReader(nil).SE(); !errors.Is(err, ErrShort) {
		t.Errorf("SE must pass on the refusal underneath it: err = %v", err)
	}
}

// TestARunOfZerosIsRefusedRatherThanCounted is the control on the bound: padding
// and a truncated unit both read as a long run of zeros, and counting them to the
// end of the stream would shift a value nobody wrote.
func TestARunOfZerosIsRefusedRatherThanCounted(t *testing.T) {
	zeros := make([]byte, 64) // 512 leading zeros
	if _, err := NewReader(zeros).UE(); !errors.Is(err, ErrTooLong) {
		t.Errorf("err = %v, want ErrTooLong", err)
	}
	if _, err := NewReader(zeros).SE(); !errors.Is(err, ErrTooLong) {
		t.Errorf("SE: err = %v, want ErrTooLong", err)
	}
}

func TestPosLeftAndPeek(t *testing.T) {
	r := NewReader([]byte{0b1010_1100, 0b1111_0000})
	if r.Left() != 16 || r.Pos() != 0 {
		t.Fatalf("Left %d Pos %d at the start", r.Left(), r.Pos())
	}
	if v, err := r.Bits(4); err != nil || v != 0b1010 {
		t.Fatalf("Bits(4) = %b, %v", v, err)
	}
	if r.Pos() != 4 || r.Left() != 12 {
		t.Errorf("Pos %d Left %d after four bits", r.Pos(), r.Left())
	}
	if v, err := r.Bits(8); err != nil || v != 0b1100_1111 {
		t.Errorf("Bits(8) across a byte boundary = %b, %v", v, err)
	}
	// ⛔ Peek must not consume: the whole point is to ask what is left without
	// spending the field being asked about. And a refused Peek must not move
	// either.
	before := r.Pos()
	if _, err := r.Peek(4); err != nil {
		t.Fatal(err)
	}
	if r.Pos() != before {
		t.Errorf("Peek consumed %d bits", r.Pos()-before)
	}
	if _, err := r.Peek(5); !errors.Is(err, ErrShort) {
		t.Errorf("Peek past the end: err = %v, want ErrShort", err)
	}
	if r.Pos() != before {
		t.Errorf("a refused Peek moved the reader by %d bits", r.Pos()-before)
	}
}

func TestMoreDataTellsAFieldFromPadding(t *testing.T) {
	for _, tc := range []struct {
		name string
		bits string
		want bool
	}{
		{"nothing at all", "", false},
		{"exactly the padding", "10000000", false},
		{"a field before the padding", "01000000", true},
		{"more than a byte left", "100000000", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := NewReader(fromBits(tc.bits)).MoreData(); got != tc.want {
				t.Errorf("MoreData = %v, want %v", got, tc.want)
			}
		})
	}
	// And after reading up to the padding, there is nothing more -- which is the
	// question a parameter set actually asks.
	//
	// ⛔ The fixture has to end in REAL padding: a one and then zeros. The first
	// version of this ended in twelve zero bits, which is not padding at all, and
	// MoreData was right to call it a field. Two Exp-Golomb fields spend four
	// bits here, so the padding is the four that finish the byte.
	r := NewReader(fromBits("01011000"))
	if v, err := r.UE(); err != nil || v != 1 {
		t.Fatalf("first field = %d, %v", v, err)
	}
	if v, err := r.UE(); err != nil || v != 0 {
		t.Fatalf("second field = %d, %v", v, err)
	}
	if r.Left() != 4 {
		t.Fatalf("%d bits left, want the four of padding -- the fixture is not what this test needs", r.Left())
	}
	if r.MoreData() {
		t.Error("padding after two fields read as a third")
	}
}
