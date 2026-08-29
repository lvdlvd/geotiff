package geotiff

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

// Raster decoding for the GeoTIFFs OpenTopography returns: tiled (or stripped),
// LZW- or Deflate-compressed, single-sample int16/float32. The base tiff.go
// only reads uncompressed strips via ifd.Image(); this reassembles tiles and
// decompresses, so the core reader stays untouched.
//
// Tags tiff.go/const.go don't name:
const (
	tagPredictor      Tag = 317
	tagTileWidth      Tag = 322
	tagTileLength     Tag = 323
	tagTileOffsets    Tag = 324
	tagTileByteCounts Tag = 325
)

// readSamples returns the full W*H raster as float32, regardless of
// tiling/compression. Single-sample only (DEMs).
func ReadSamples(ifd *IFD) ([]float32, error) {
	w, h := ifd.WxL()
	if w == 0 || h == 0 {
		return nil, fmt.Errorf("zero image size")
	}
	if spp := firstU16(ifd.Value(SamplesPerPixel)); spp != 1 {
		return nil, fmt.Errorf("samples/pixel=%d unsupported (DEM expects 1)", spp)
	}
	bps := int(firstU16(ifd.Value(BitsPerSample)))
	if bps != 8 && bps != 16 && bps != 32 {
		return nil, fmt.Errorf("bits/sample=%d unsupported", bps)
	}
	bpp := bps / 8

	sf := SampleFormatType(Unsigned)
	if v, ok := ifd.Value(SampleFormat).([]SampleFormatType); ok && len(v) > 0 {
		sf = v[0]
	}
	comp := Uncompressed
	if v, ok := ifd.Value(Compression).([]CompressionType); ok && len(v) > 0 {
		comp = v[0]
	}
	pred := int(firstU16(ifd.Value(tagPredictor)))
	if pred == 0 {
		pred = 1
	}
	if pred != 1 && pred != 2 && pred != 3 {
		return nil, fmt.Errorf("predictor %d unsupported", pred)
	}

	raster := make([]byte, w*h*bpp)
	// place copies one decoded block (bw wide, bh tall, row-major) into raster
	// at (x0,y0), clipped to the image edges.
	place := func(dec []byte, x0, y0, bw, bh int) error {
		if len(dec) < bw*bh*bpp {
			return fmt.Errorf("short block: %d < %d", len(dec), bw*bh*bpp)
		}
		for ry := 0; ry < bh; ry++ {
			yy := y0 + ry
			if yy >= h {
				break
			}
			cols := bw
			if x0+cols > w {
				cols = w - x0
			}
			if cols <= 0 {
				continue
			}
			src := dec[ry*bw*bpp:]
			copy(raster[(yy*w+x0)*bpp:(yy*w+x0+cols)*bpp], src[:cols*bpp])
		}
		return nil
	}

	if tw := int(firstU16(ifd.Value(tagTileWidth))); tw > 0 {
		th := int(firstU16(ifd.Value(tagTileLength)))
		offs := asUint32s(ifd.Value(tagTileOffsets))
		cnts := asUint32s(ifd.Value(tagTileByteCounts))
		across := (w + tw - 1) / tw
		down := (h + th - 1) / th
		if len(offs) < across*down || len(cnts) < across*down {
			return nil, fmt.Errorf("tile table %d/%d < %d", len(offs), len(cnts), across*down)
		}
		idx := 0
		for ty := 0; ty < down; ty++ {
			for tx := 0; tx < across; tx++ {
				dec, err := decompressBlock(ifd.r, offs[idx], cnts[idx], comp)
				if err != nil {
					return nil, fmt.Errorf("tile %d: %w", idx, err)
				}
				if pred == 2 {
					undoHPredictor(dec, tw, th, bpp)
				} else if pred == 3 {
					undoFloatPredictor(dec, tw, th, bpp, ifd.order)
				}
				if err := place(dec, tx*tw, ty*th, tw, th); err != nil {
					return nil, fmt.Errorf("tile %d: %w", idx, err)
				}
				idx++
			}
		}
	} else {
		offs := asUint32s(ifd.Value(StripOffsets))
		cnts := asUint32s(ifd.Value(StripByteCounts))
		rps := int(firstU16(ifd.Value(RowsPerStrip)))
		if rps <= 0 {
			rps = h
		}
		if len(offs) == 0 || len(offs) != len(cnts) {
			return nil, fmt.Errorf("strip table %d/%d", len(offs), len(cnts))
		}
		for i := range offs {
			dec, err := decompressBlock(ifd.r, offs[i], cnts[i], comp)
			if err != nil {
				return nil, fmt.Errorf("strip %d: %w", i, err)
			}
			rows := rps
			if i*rps+rows > h {
				rows = h - i*rps
			}
			if pred == 2 {
				undoHPredictor(dec, w, rows, bpp)
			} else if pred == 3 {
				undoFloatPredictor(dec, w, rows, bpp, ifd.order)
			}
			if err := place(dec, 0, i*rps, w, rows); err != nil {
				return nil, fmt.Errorf("strip %d: %w", i, err)
			}
		}
	}

	return bytesToFloat32(raster, w*h, bps, sf, ifd.order)
}

func decompressBlock(r io.ReaderAt, off, n uint32, comp CompressionType) ([]byte, error) {
	raw := make([]byte, n)
	if _, err := r.ReadAt(raw, int64(off)); err != nil && err != io.EOF {
		return nil, err
	}
	switch comp {
	case Uncompressed:
		return raw, nil
	case LZW:
		return lzwDecodeTIFF(raw) // TIFF's early-change variant; compress/lzw is incompatible
	case 8, 32946: // Deflate / Adobe Deflate
		zr, err := zlib.NewReader(bytes.NewReader(raw))
		if err != nil {
			return nil, err
		}
		defer zr.Close()
		return io.ReadAll(zr)
	default:
		return nil, fmt.Errorf("unsupported compression %v", comp)
	}
}

// undoHPredictor reverses TIFF horizontal differencing (Predictor=2): each
// sample in a row is stored as the delta from its left neighbour.
func undoHPredictor(b []byte, w, h, bpp int) {
	for y := 0; y < h; y++ {
		row := b[y*w*bpp:]
		for x := 1; x < w; x++ {
			switch bpp {
			case 1:
				row[x] += row[x-1]
			case 2:
				prev := binary.LittleEndian.Uint16(row[(x-1)*2:])
				cur := binary.LittleEndian.Uint16(row[x*2:])
				binary.LittleEndian.PutUint16(row[x*2:], prev+cur)
			case 4:
				prev := binary.LittleEndian.Uint32(row[(x-1)*4:])
				cur := binary.LittleEndian.Uint32(row[x*4:])
				binary.LittleEndian.PutUint32(row[x*4:], prev+cur)
			}
		}
	}
}

// undoFloatPredictor reverses the TIFF floating-point predictor (Predictor=3):
// per row the bytes were byte-plane-shuffled (most-significant plane first) then
// horizontally byte-differenced. Reverse it: accumulate bytes left-to-right, then
// de-shuffle the planes back into sample byte order. Single-sample (DEM) only;
// mirrors libtiff fpAcc with stride 1.
func undoFloatPredictor(b []byte, w, h, bpp int, order binary.ByteOrder) {
	rowBytes := w * bpp
	tmp := make([]byte, rowBytes)
	le := order != binary.BigEndian
	for y := 0; y < h; y++ {
		row := b[y*rowBytes : (y+1)*rowBytes]
		for i := 1; i < rowBytes; i++ { // horizontal byte accumulation
			row[i] += row[i-1]
		}
		copy(tmp, row)
		for c := 0; c < w; c++ { // de-shuffle byte planes
			for k := 0; k < bpp; k++ {
				if le {
					row[bpp*c+k] = tmp[(bpp-k-1)*w+c]
				} else {
					row[bpp*c+k] = tmp[k*w+c]
				}
			}
		}
	}
}

func bytesToFloat32(b []byte, n, bps int, sf SampleFormatType, order binary.ByteOrder) ([]float32, error) {
	out := make([]float32, n)
	switch {
	case bps == 8 && sf == Unsigned:
		for i := 0; i < n; i++ {
			out[i] = float32(b[i])
		}
	case bps == 16 && sf == Signed:
		for i := 0; i < n; i++ {
			out[i] = float32(int16(order.Uint16(b[i*2:])))
		}
	case bps == 16 && sf == Unsigned:
		for i := 0; i < n; i++ {
			out[i] = float32(order.Uint16(b[i*2:]))
		}
	case bps == 32 && sf == Float:
		for i := 0; i < n; i++ {
			out[i] = math.Float32frombits(order.Uint32(b[i*4:]))
		}
	default:
		return nil, fmt.Errorf("unsupported sample %d-bit format %v", bps, sf)
	}
	return out, nil
}

// helpers reading IFD values that may be SHORT or LONG.
func firstU16(v interface{}) uint16 {
	switch s := v.(type) {
	case []uint16:
		if len(s) > 0 {
			return s[0]
		}
	case []uint32:
		if len(s) > 0 {
			return uint16(s[0])
		}
	}
	return 0
}

func asUint32s(v interface{}) []uint32 {
	switch s := v.(type) {
	case []uint32:
		return s
	case []uint16:
		out := make([]uint32, len(s))
		for i, x := range s {
			out[i] = uint32(x)
		}
		return out
	}
	return nil
}
