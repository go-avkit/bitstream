// Copyright (c) 2026, go-avkit
// SPDX-License-Identifier: BSD-3-Clause

package bitstream

import (
	"bytes"
	"errors"
	"testing"
)

// TestSplitAnnexBTakesBothStartCodeLengths.
//
// ⛔ A four-byte start code is a three-byte one with a leading zero, and the zero
// belongs to the gap: a unit that kept it would have its header read out of a byte
// nobody wrote there. Trailing zeros go for the same reason in reverse -- an
// encoder may pad, and padding is not payload.
func TestSplitAnnexBTakesBothStartCodeLengths(t *testing.T) {
	stream := []byte{
		0, 0, 0, 1, 'a', 'b', // four-byte start code
		0, 0, 1, 'c', // three-byte
		0, 0, 0, 1, 'd', 'e', 'f', 0, 0, // with padding
	}
	units, err := SplitAnnexB(stream)
	if err != nil {
		t.Fatalf("SplitAnnexB: %v", err)
	}
	want := []string{"ab", "c", "def"}
	if len(units) != len(want) {
		t.Fatalf("%d units, want %d: %q", len(units), len(want), units)
	}
	for i, w := range want {
		if string(units[i]) != w {
			t.Errorf("unit %d = %q, want %q -- padding or a start code byte was kept", i+1, units[i], w)
		}
	}
}

func TestAStreamWithNoStartCodeIsRefused(t *testing.T) {
	if _, err := SplitAnnexB([]byte{'a', 'b'}); !errors.Is(err, ErrNoStartCode) {
		t.Errorf("err = %v, want ErrNoStartCode", err)
	}
	// Separators and nothing else hold no unit at all, which is a different
	// answer from a malformed stream.
	if _, err := SplitAnnexB([]byte{0, 0, 1}); !errors.Is(err, ErrNoStartCode) {
		t.Errorf("only separators: err = %v, want ErrNoStartCode", err)
	}
}

// TestAStreamCutAfterItsLastSeparatorKeepsWhatCameBefore: a cut stream is the
// normal state of a download in progress, and refusing the whole of it would lose
// every unit that did arrive.
func TestAStreamCutAfterItsLastSeparatorKeepsWhatCameBefore(t *testing.T) {
	units, err := SplitAnnexB([]byte{0, 0, 1, 'a', 0, 0, 1})
	if err != nil {
		t.Fatalf("SplitAnnexB: %v", err)
	}
	if len(units) != 1 || string(units[0]) != "a" {
		t.Fatalf("%d units: %q", len(units), units)
	}
}

// TestEveryLengthSizeIsReadBigEndian.
//
// ⛔ A configuration record states this size, and all four widths occur. One read
// with the wrong byte order or width lands on a plausible boundary rather than
// failing, so each is stated against a length only the right reading produces.
func TestEveryLengthSizeIsReadBigEndian(t *testing.T) {
	for _, tc := range []struct {
		size   int
		prefix []byte
	}{
		{1, []byte{2}},
		{2, []byte{0, 2}},
		{3, []byte{0, 0, 2}},
		{4, []byte{0, 0, 0, 2}},
	} {
		stream := append(append([]byte(nil), tc.prefix...), 'x', 'y')
		units, err := SplitLengthPrefixed(stream, tc.size)
		if err != nil {
			t.Errorf("size %d: %v", tc.size, err)
			continue
		}
		if len(units) != 1 || string(units[0]) != "xy" {
			t.Errorf("size %d: %q", tc.size, units)
		}
	}
	// A size of 258 is 02 01 one way round and 01 02 the other; only one of them
	// accounts for the payload.
	long := append([]byte{1, 2}, bytes.Repeat([]byte{'z'}, 258)...)
	units, err := SplitLengthPrefixed(long, 2)
	if err != nil {
		t.Fatalf("258 bytes: %v", err)
	}
	if len(units) != 1 || len(units[0]) != 258 {
		t.Errorf("%d units, first of %d bytes, want one of 258", len(units), len(units[0]))
	}
}

func TestAZeroLengthUnitIsSkippedRatherThanRefused(t *testing.T) {
	units, err := SplitLengthPrefixed([]byte{0, 0, 0, 0, 0, 0, 0, 1, 'y'}, 4)
	if err != nil {
		t.Fatalf("SplitLengthPrefixed: %v", err)
	}
	if len(units) != 1 || string(units[0]) != "y" {
		t.Fatalf("%d units: %q", len(units), units)
	}
}

// TestALengthPastTheEndIsRefused.
//
// ⛔ A wrong length size reads a length out of payload, and such a length is
// enormous. Walking on would either panic on a slice or land on a plausible
// boundary and report units nobody wrote.
func TestALengthPastTheEndIsRefused(t *testing.T) {
	if _, err := SplitLengthPrefixed([]byte{0, 0, 0, 99, 'a'}, 4); !errors.Is(err, ErrLengthOverrun) {
		t.Errorf("err = %v, want ErrLengthOverrun", err)
	}
	if _, err := SplitLengthPrefixed([]byte{0, 0}, 4); !errors.Is(err, ErrLengthOverrun) {
		t.Errorf("a tail too short to hold a length: err = %v, want ErrLengthOverrun", err)
	}
	for _, size := range []int{0, 5, -1} {
		if _, err := SplitLengthPrefixed([]byte{'a'}, size); !errors.Is(err, ErrEmptySize) {
			t.Errorf("size %d: err = %v, want ErrEmptySize", size, err)
		}
	}
	if units, err := SplitLengthPrefixed(nil, 4); err != nil || len(units) != 0 {
		t.Errorf("an empty stream: %q, %v", units, err)
	}
}

func TestUnescapeDropsOnlyARealEscape(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   []byte
		want []byte
	}{
		{"an escape", []byte{0, 0, 3, 1}, []byte{0, 0, 1}},
		{"two escapes", []byte{0, 0, 3, 0, 0, 3, 2}, []byte{0, 0, 0, 0, 2}},
		// ⛔ The control: a three after ONE zero, or after none, is payload.
		// Dropping every three after a zero would eat data and misalign
		// everything read afterwards.
		{"a three after one zero", []byte{0, 3, 1}, []byte{0, 3, 1}},
		{"a three after none", []byte{9, 3, 1}, []byte{9, 3, 1}},
		// The byte after an escape starts the count again, so this is two escapes
		// and not one followed by payload.
		{"a zero after an escape", []byte{0, 0, 3, 0, 0, 3, 0}, []byte{0, 0, 0, 0, 0}},
		{"nothing to do", []byte{1, 2, 3}, []byte{1, 2, 3}},
		{"nothing at all", nil, []byte{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Unescape(tc.in); !bytes.Equal(got, tc.want) {
				t.Errorf("Unescape(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}
