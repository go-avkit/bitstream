// Copyright (c) 2026, go-avkit
// SPDX-License-Identifier: BSD-3-Clause

package bitstream

import (
	"errors"
	"fmt"
)

// Errors a reader can refuse with.
var (
	// ErrShort means the bitstream ended in the middle of a field.
	ErrShort = errors.New("bitstream: ends inside a field")
	// ErrTooLong means an Exp-Golomb code claims more leading zeros than any
	// value these formats state could need.
	ErrTooLong = errors.New("bitstream: Exp-Golomb code is longer than any value needs")

	// ErrWidth means a read was asked for a number of bits the value it returns
	// cannot hold, or a negative one. Both used to be answered silently.
	ErrWidth = errors.New("bitstream: a width the value cannot hold")
)

// maxLeadingZeros bounds an Exp-Golomb prefix.
//
// ⛔ It is here because a run of zero bytes -- padding, or a truncated unit read
// as payload -- otherwise counts leading zeros until the stream ends and then
// shifts a value nobody wrote. 32 is past anything these formats state and keeps
// the refusal a refusal rather than a walk through the rest of a long stream.
const maxLeadingZeros = 32

// Reader reads the fields of a payload: single bits, fixed-width values, and the
// Exp-Golomb integers these formats write their parameter sets with.
//
// It is given an UNESCAPED payload. Escaping is undone by Unescape, and keeping
// the two apart means a caller that only walks past units never pays for a copy of
// one.
type Reader struct {
	data []byte
	pos  int // bits consumed
}

// NewReader reads the bits of data.
func NewReader(data []byte) *Reader { return &Reader{data: data} }

// Pos is how many bits have been read, which is what lets a caller say where a
// refusal happened rather than only that one did.
func (r *Reader) Pos() int { return r.pos }

// Left is how many bits remain. It is what tells a reader whether an optional
// field follows or only the bits that end a payload.
func (r *Reader) Left() int { return len(r.data)*8 - r.pos }

// Bit reads one bit.
func (r *Reader) Bit() (uint32, error) {
	if r.pos >= len(r.data)*8 {
		return 0, ErrShort
	}
	b := r.data[r.pos/8]
	shift := 7 - uint(r.pos%8)
	r.pos++
	return uint32(b>>shift) & 1, nil
}

// Bits reads n bits, most significant first.
func (r *Reader) Bits(n int) (uint32, error) {
	// ⛔ A width the RETURN TYPE cannot hold. Asking for 33 bits used to give
	// the SECOND to the thirty-third -- the top bit shifted out of the uint32
	// in silence -- with no error and the position correctly advanced, so
	// nothing downstream could notice. The value was a field nobody asked for.
	//
	// A negative width read nothing and said nothing, which is how a width
	// computed one field too early arrives here.
	if n < 0 || n > 32 {
		return 0, fmt.Errorf("%w: %d bits do not fit the value read", ErrWidth, n)
	}
	var v uint32
	for i := 0; i < n; i++ {
		b, err := r.Bit()
		if err != nil {
			return 0, err
		}
		v = v<<1 | b
	}
	return v, nil
}

// Peek reads n bits without consuming them.
//
// It exists for one question these formats ask and which cannot be answered any
// other way: whether what is left is a syntax element or the padding that ends a
// payload. Answering it by reading would consume the field it was asking about.
func (r *Reader) Peek(n int) (uint32, error) {
	at := r.pos
	v, err := r.Bits(n)
	r.pos = at
	return v, err
}

// UE reads an unsigned Exp-Golomb integer: n zeros, a one, then n more bits,
// giving a value of 2^n - 1 plus those bits.
func (r *Reader) UE() (uint32, error) {
	zeros := 0
	for {
		b, err := r.Bit()
		if err != nil {
			return 0, err
		}
		if b == 1 {
			break
		}
		zeros++
		if zeros > maxLeadingZeros {
			return 0, ErrTooLong
		}
	}
	if zeros == 0 {
		return 0, nil
	}
	rest, err := r.Bits(zeros)
	if err != nil {
		return 0, err
	}
	return (1<<uint(zeros) - 1) + rest, nil
}

// SE reads a signed Exp-Golomb integer, which is the unsigned one folded so that
// 1 means +1, 2 means -1, 3 means +2, and so on.
func (r *Reader) SE() (int32, error) {
	v, err := r.UE()
	if err != nil {
		return 0, err
	}
	if v%2 == 0 {
		// An even code is a negative value, and zero is zero.
		return -int32(v / 2), nil
	}
	return int32(v/2) + 1, nil
}

// MoreData says whether any syntax element remains before the bits that end a
// payload.
//
// Those bits are a single one followed by zeros to the byte boundary, so what is
// left is a field only if a one appears after the first one, or the first one is
// not where the padding would put it. Reading them as a field is how an optional
// tail comes to be read from a payload that does not have one.
func (r *Reader) MoreData() bool {
	left := r.Left()
	if left == 0 {
		// Nothing at all is left, not even the bits that should end a payload.
		// Every field did read, so a caller is told there is nothing more rather
		// than being handed a refusal over a byte of padding.
		return false
	}
	// Anything longer than a byte of padding must hold a field.
	if left > 8 {
		return true
	}
	// Peek cannot refuse this: left IS what remains, so the bits asked for are
	// exactly the bits there are.
	rest, _ := r.Peek(left)
	// The trailing pattern is one followed by zeros: as a number, a single bit
	// set at the top of what is left.
	return rest != 1<<uint(left-1)
}
