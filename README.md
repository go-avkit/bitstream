# bitstream

Pure-Go (CGO=0) primitives every H.264/H.265-family reader needs before it can
parse anything: a bit reader with the formats' own integer codings, the two NAL
framings, and the unescaping that a payload is meaningless without.

```go
units, err := bitstream.SplitAnnexB(data)   // 00 00 01 start codes
r := bitstream.NewReader(bitstream.Unescape(units[0]))
profile, err := r.Bits(8)
id, err := r.UE()                            // Exp-Golomb, unsigned
delta, err := r.SE()                         // Exp-Golomb, signed
```

## What is in it

| | |
|---|---|
| `SplitAnnexB` | units separated by `00 00 01` / `00 00 00 01` start codes |
| `SplitLengthPrefixed` | units behind a 1-, 2- or 4-byte length, as a sample from an MP4 track carries them |
| `Unescape` | removes the emulation-prevention bytes (`00 00 03`) |
| `Reader` | `Bit`, `Bits`, `Peek`, `UE`, `SE`, `Pos`, `Left`, `MoreData` |

**`Unescape` is the one that gets forgotten.** A unit read without it is not
broken, it is subtly *wrong*: the bytes are all there, in order, with an extra
`03` wherever two zeroes met — so the fields after that point decode to
plausible values that belong to nothing.

`MoreData` is `more_rbsp_data()`: it answers whether anything but the trailing
stop bit and padding remains, which is how a reader knows an optional syntax
element is present at all.

Every read returns an error rather than a zero: a bit beyond the end of a unit
is a truncated stream, and a reader that answered 0 would carry on parsing
whatever follows.

**Windows, macOS, Linux; six 64-bit architectures.** 100% statement coverage,
gated in CI.

## A width the value cannot hold

`Bits` returns a `uint32`, so it reads **0 to 32** bits and refuses anything
else with `ErrWidth`. There is no 64-bit reader, so no caller can want more.

⛔ **It used to answer silently.** `Bits(33)` gave the *second* bit to the
thirty-third — the top one shifted out of the `uint32` — with **no error** and
the position correctly advanced, so nothing downstream could notice. The value
returned was a field nobody asked for:

```
Bits(33)          = 0xbd5b7ddf
Bit(); Bits(32)   = 0xbd5b7ddf    identical
```

A **negative** width read nothing and said nothing, which is how a width
computed one field too early arrives here.

The refusal happens **before** any bit is consumed, so a caller that handles the
error can read the field another way. Both ends are read: 0 and 32 are widths
the value holds, and `UE` needs a 32-bit read to reach its largest value
(2³²−1, from a 65-bit code) — a bound at 31 breaks it, and there is a witness.

## Licence

BSD-3-Clause.
