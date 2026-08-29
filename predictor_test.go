package geotiff

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"
)

// encodeFloatPredictor is the forward TIFF float predictor (inverse of
// undoFloatPredictor): shuffle byte planes (MSB-plane first for LE) then
// horizontally byte-difference. Test-only, to round-trip the decoder.
func encodeFloatPredictor(b []byte, w, h, bpp int, order binary.ByteOrder) {
	rowBytes := w * bpp
	tmp := make([]byte, rowBytes)
	le := order != binary.BigEndian
	for y := 0; y < h; y++ {
		row := b[y*rowBytes : (y+1)*rowBytes]
		copy(tmp, row)
		for c := 0; c < w; c++ {
			for k := 0; k < bpp; k++ {
				if le {
					row[(bpp-k-1)*w+c] = tmp[bpp*c+k]
				} else {
					row[k*w+c] = tmp[bpp*c+k]
				}
			}
		}
		for i := rowBytes - 1; i >= 1; i-- {
			row[i] -= row[i-1]
		}
	}
}

func TestFloatPredictorRoundTrip(t *testing.T) {
	const w, h, bpp = 5, 3, 4
	vals := []float32{
		0, 1.5, -2.25, 1000.5, 1001.0,
		1001.25, 980.0, 5.5, -100.0, 0.125,
		3000.0, 2999.5, 2999.0, 42.0, 42.5,
	}
	orig := make([]byte, w*h*bpp)
	for i, v := range vals {
		binary.LittleEndian.PutUint32(orig[i*4:], math.Float32bits(v))
	}
	enc := append([]byte(nil), orig...)
	encodeFloatPredictor(enc, w, h, bpp, binary.LittleEndian)
	if bytes.Equal(enc, orig) {
		t.Fatal("encode produced identical bytes (predictor no-op?)")
	}
	undoFloatPredictor(enc, w, h, bpp, binary.LittleEndian)
	if !bytes.Equal(enc, orig) {
		t.Fatal("float predictor round-trip mismatch")
	}
}
