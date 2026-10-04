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

## Licence

BSD-3-Clause.
