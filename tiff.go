// Decoding of files that adhere to the "OGC GeoTIFF standard"
//
//	https://docs.ogc.org/is/19-008r4/19-008r4.html
package geotiff

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math/big"
)

// A TIFF File consists of a linked list of Image File Directories
type IFD struct {
	r       io.ReaderAt
	order   binary.ByteOrder
	offset  uint32
	Tags    []Tag
	dir     map[Tag]int
	entries []entry
}

func (ifd *IFD) readUint16At(offset uint32) (uint16, error) {
	var buf [2]byte
	if _, err := ifd.r.ReadAt(buf[:], int64(offset)); err != nil {
		return 0, err
	}
	return ifd.order.Uint16(buf[:]), nil
}

func (ifd *IFD) readUint32At(offset uint32) (uint32, error) {
	var buf [4]byte
	if _, err := ifd.r.ReadAt(buf[:], int64(offset)); err != nil {
		return 0, err
	}
	return ifd.order.Uint32(buf[:]), nil
}

// OpenTIFF reads the first IFD from the TIFF file accessed through r
func OpenTIFF(r io.ReaderAt) (*IFD, error) {
	var header [8]byte
	if _, err := r.ReadAt(header[:], 0); err != nil {
		return nil, err
	}
	var order binary.ByteOrder
	switch binary.LittleEndian.Uint16(header[:]) {
	case 0x4949:
		order = binary.LittleEndian
	case 0x4d4d:
		order = binary.BigEndian
	default:
		return nil, fmt.Errorf("invalid byteorder %04x", order)
	}
	if v := order.Uint16(header[2:]); v != 42 {
		return nil, fmt.Errorf("invalid magic number %04x != 42", v)
	}

	ifd := &IFD{r: r, order: order, offset: order.Uint32(header[4:])}
	return ifd, ifd.loadEntries()

}

// Next returns the next IFD on the linked list of IFDs or nil at the end of the chain or on error.
func (ifd *IFD) Next() (*IFD, error) {
	numentries, err := ifd.readUint16At(ifd.offset)
	if err != nil {
		return nil, err
	}
	nextoffs, err := ifd.readUint32At(ifd.offset + 2 + 12*uint32(numentries))
	if err != nil {
		return nil, err
	}
	if nextoffs == 0 {
		return nil, nil
	}
	ifd = &IFD{r: ifd.r, order: ifd.order, offset: nextoffs}
	return ifd, ifd.loadEntries()
}

func (ifd *IFD) loadEntries() error {
	numentries, err := ifd.readUint16At(ifd.offset)
	if err != nil {
		return fmt.Errorf("loadEntries offset %d: %v", ifd.offset, err)
	}
	s := io.NewSectionReader(ifd.r, int64(ifd.offset)+2, int64(12*numentries))
	entries := make([]storedEntry, numentries)
	if err = binary.Read(s, ifd.order, &entries); err != nil {
		return err
	}
	ifd.dir = map[Tag]int{}
	for idx, v := range entries {
		ifd.dir[v.Tag] = len(ifd.Tags)
		ifd.Tags = append(ifd.Tags, v.Tag)

		if v.DataType.Size()*int(v.Count) > 4 {
			ifd.entries = append(ifd.entries, entry{v.DataType, v.Count, ifd.order.Uint32(v.ValOffs[:])})
		} else {
			ifd.entries = append(ifd.entries, entry{v.DataType, v.Count, ifd.offset + 2 + uint32(idx*12) + 8})
		}
	}
	return nil
}

type storedEntry struct {
	Tag      Tag
	DataType DataType
	Count    uint32
	ValOffs  [4]byte
}

type entry struct {
	DataType DataType
	Count    uint32
	offset   uint32
}

// returns a slice of the appropriate type OR an error OR nil if no such tag
func (ifd *IFD) Value(tag Tag) interface{} {
	idx, ok := ifd.dir[tag]
	if !ok {
		return nil
	}
	v := ifd.entries[idx]
	sr := io.NewSectionReader(ifd.r, int64(v.offset), int64(v.DataType.Size()*int(v.Count)))
	val := v.DataType.MkSlice(int(v.Count))
	err := binary.Read(sr, ifd.order, val)
	if err != nil {
		return fmt.Errorf("reading %v[%d] data for tag %v at offset %d: %v", v.DataType, v.Count, tag, v.offset, err)
	}
	// patch up some of the special datatypes
	switch v.DataType {
	case ASCII:
		val = string(val.([]byte))

	case RATIONAL:
		vv := val.([]Rational)
		ww := make([]*big.Rat, len(vv))
		for i, v := range vv {
			ww[i] = v.Rat()
		}
		val = ww

	case SRATIONAL:
		vv := val.([]SRational)
		ww := make([]*big.Rat, len(vv))
		for i, v := range vv {
			ww[i] = v.Rat()
		}
		val = ww
	}

	// handle some of the tags
	switch tag {
	case StripOffsets:
		if offs, ok := val.([]uint16); ok {
			ww := make([]uint32, len(offs))
			for i, v := range offs {
				ww[i] = uint32(v)
			}
			val = ww
		}

	case StripByteCounts:
		if offs, ok := val.([]uint16); ok {
			ww := make([]uint32, len(offs))
			for i, v := range offs {
				ww[i] = uint32(v)
			}
			val = ww
		}

	case Compression:
		if cmpr, ok := val.([]uint16); ok {
			ww := make([]CompressionType, len(cmpr))
			for i, v := range cmpr {
				ww[i] = CompressionType(v)
			}
			val = ww
		}

	case PhotometricInterpretation:
		if cmpr, ok := val.([]uint16); ok {
			ww := make([]PhotometricInterpretationType, len(cmpr))
			for i, v := range cmpr {
				ww[i] = PhotometricInterpretationType(v)
			}
			val = ww
		}

	case SampleFormat:
		if cmpr, ok := val.([]uint16); ok {
			ww := make([]SampleFormatType, len(cmpr))
			for i, v := range cmpr {
				ww[i] = SampleFormatType(v)
			}
			val = ww
		}

	}

	return val
}

func (ifd *IFD) WxL() (W, L int) {
	ww := ifd.Value(ImageWidth)
	switch vv := ww.(type) {
	case []uint16:
		W = int(vv[0])
	case []uint32:
		W = int(vv[0])
	}
	ll := ifd.Value(ImageLength)
	switch vv := ll.(type) {
	case []uint16:
		L = int(vv[0])
	case []uint32:
		L = int(vv[0])
	}
	return
}

// image returns a []pixel
// where pixel =
func (ifd *IFD) Image() interface{} {

	samplesperpixel, ok := ifd.Value(SamplesPerPixel).([]uint16)
	if !ok || len(samplesperpixel) != 1 {
		return fmt.Errorf("invalid samples per pixel:%v", ifd.Value(SamplesPerPixel))
	}

	bitspersample, ok := ifd.Value(BitsPerSample).([]uint16)
	if !ok || len(bitspersample) != int(samplesperpixel[0]) {
		return fmt.Errorf("invalid bits per sample:%v", ifd.Value(SamplesPerPixel))
	}

	if samplesperpixel[0] > 1 {
		for _, v := range bitspersample[1:] {
			if v != bitspersample[0] {
				return fmt.Errorf("can only handle uniform bits per sample at the moment, got %v", bitspersample)
			}
		}
	}

	sampleformat, ok := ifd.Value(SampleFormat).([]SampleFormatType)
	if !ok { // todo: this also overrides an invalid sampleformat, should return error instead
		sampleformat = make([]SampleFormatType, samplesperpixel[0])
		for i := range sampleformat {
			sampleformat[i] = Unsigned
		}
	} else if len(sampleformat) != int(samplesperpixel[0]) {
		return fmt.Errorf("invalid sampleformat:%v", ifd.Value(SampleFormat))
	} else if samplesperpixel[0] > 1 {
		for _, v := range sampleformat[1:] {
			if v != sampleformat[0] {
				return fmt.Errorf("can only handle uniform sampleformat at the moment, got %v", sampleformat)
			}
		}
	}

	W, L := ifd.WxL()
	N := W * L
	var img interface{}
	switch {
	case samplesperpixel[0] == 1 && bitspersample[0] == 8 && sampleformat[0] == Unsigned:
		img = make([]uint8, N)
	case samplesperpixel[0] == 1 && bitspersample[0] == 8 && sampleformat[0] == Signed:
		img = make([]int8, N)
	case samplesperpixel[0] == 1 && bitspersample[0] == 16 && sampleformat[0] == Unsigned:
		img = make([]uint16, N)
	case samplesperpixel[0] == 1 && bitspersample[0] == 16 && sampleformat[0] == Signed:
		img = make([]int16, N)
	case samplesperpixel[0] == 3 && bitspersample[0] == 8 && sampleformat[0] == Unsigned:
		img = make([][3]uint8, N)
	case samplesperpixel[0] == 3 && bitspersample[0] == 8 && sampleformat[0] == Signed:
		img = make([][3]int8, N)
	case samplesperpixel[0] == 3 && bitspersample[0] == 16 && sampleformat[0] == Unsigned:
		img = make([][3]uint16, N)
	case samplesperpixel[0] == 3 && bitspersample[0] == 16 && sampleformat[0] == Signed:
		img = make([][3]int16, N)
	default:
		return fmt.Errorf("unsupported image format [%d]%s%d", samplesperpixel[0], sampleformat[0], bitspersample[0])
	}

	cmptp, ok := ifd.Value(Compression).(CompressionType)
	if ok && cmptp != Uncompressed {
		return fmt.Errorf("cannot handle compression type: %v", ifd.Value(Compression))
	}

	offsets, ok := ifd.Value(StripOffsets).([]uint32)
	if !ok {
		return fmt.Errorf("StripOffsets has wrong data type %v", ifd.entries[ifd.dir[StripOffsets]])
	}
	counts, ok := ifd.Value(StripByteCounts).([]uint32)
	if !ok {
		return fmt.Errorf("StripByteCounts has wrong data type %v", ifd.entries[ifd.dir[StripByteCounts]])
	}
	if len(offsets) != len(counts) {
		return fmt.Errorf("len(StripOffsets) = %d,  len(StripByteCounts) = %d", len(offsets), len(counts))
	}

	var buf bytes.Buffer
	for i, v := range offsets {
		if _, err := io.Copy(&buf, io.NewSectionReader(ifd.r, int64(v), int64(counts[i]))); err != nil {
			return fmt.Errorf("reading strip %d @%d[%d]: %v", i, v, counts[i], err)
		}
	}

	n := int(samplesperpixel[0]) * int(bitspersample[0]) / 8
	if buf.Len() != W*L*n {
		return fmt.Errorf("strip data size %d does not match image size %d * %d", buf.Len(), W, L)
	}

	if err := binary.Read(&buf, ifd.order, img); err != nil {
		return err
	}

	return img
}
