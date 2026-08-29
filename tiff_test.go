package geotiff

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// buildTwoIFDTIFF synthesizes a minimal little-endian TIFF whose IFD chain is
// two directories long: IFD0 {ImageWidth=10} -> IFD1 {ImageWidth=20} -> end.
func buildTwoIFDTIFF() []byte {
	var b bytes.Buffer
	le := binary.LittleEndian

	put16 := func(v uint16) { binary.Write(&b, le, v) }
	put32 := func(v uint32) { binary.Write(&b, le, v) }
	entry := func(tag Tag, width uint16) {
		put16(uint16(tag))
		put16(uint16(SHORT))
		put32(1)
		put16(width) // value packed into the 4-byte ValOffs field
		put16(0)
	}

	// header: byte order, magic, offset of IFD0
	b.WriteString("II")
	put16(42)
	put32(8)
	// IFD0 at offset 8: 1 entry, next -> 26
	put16(1)
	entry(ImageWidth, 10)
	put32(26)
	// IFD1 at offset 26: 1 entry, next -> 0 (end of chain)
	put16(1)
	entry(ImageWidth, 20)
	put32(0)
	return b.Bytes()
}

// Next used to have an inverted error check and could never reach the second
// IFD — where a COG keeps its overview levels.
func TestNextWalksIFDChain(t *testing.T) {
	ifd, err := OpenTIFF(bytes.NewReader(buildTwoIFDTIFF()))
	if err != nil {
		t.Fatal(err)
	}
	if w, _ := ifd.WxL(); w != 10 {
		t.Fatalf("IFD0 ImageWidth = %d, want 10", w)
	}
	ifd2, err := ifd.Next()
	if err != nil {
		t.Fatal(err)
	}
	if ifd2 == nil {
		t.Fatal("Next returned nil for a two-IFD chain")
	}
	if w, _ := ifd2.WxL(); w != 20 {
		t.Fatalf("IFD1 ImageWidth = %d, want 20", w)
	}
	if end, err := ifd2.Next(); err != nil || end != nil {
		t.Fatalf("end of chain: got (%v, %v), want (nil, nil)", end, err)
	}
}
