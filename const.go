package geotiff

import "math/big"

//go:generate stringer -type DataType,Tag,CompressionType,PhotometricInterpretationType,SampleFormatType

type DataType uint16

const (
	BYTE      DataType = 1  // uint8
	ASCII     DataType = 2  // zero terminated string
	SHORT     DataType = 3  // uint16
	LONG      DataType = 4  // uint32
	RATIONAL  DataType = 5  // struct { num, denom uint32 }
	SBYTE     DataType = 6  // int8
	UNDEFINED DataType = 7  // byte
	SSHORT    DataType = 8  // int16
	SLONG     DataType = 9  // int32
	SRATIONAL DataType = 10 // struct { num, denom int32 }
	FLOAT     DataType = 11 // float32
	DOUBLE    DataType = 12 // float64
)

var dtSize = [16]int{
	0, 1, 1, 2,
	4, 8, 1, 1,
	2, 4, 8, 4,
	8, 0, 0, 0,
}

func (dt DataType) Size() int {
	if dt < 16 {
		return dtSize[dt]
	}
	return 0
}

type Rational struct{ A, B uint32 }
type SRational struct{ A, B int32 }

func (r Rational) Rat() *big.Rat  { return big.NewRat(int64(r.A), int64(r.B)) }
func (r SRational) Rat() *big.Rat { return big.NewRat(int64(r.A), int64(r.B)) }

func (dt DataType) MkSlice(n int) interface{} {
	if n <= 0 {
		return nil
	}
	switch dt {
	case BYTE:
		return make([]byte, n)
	case ASCII:
		return make([]byte, n)
	case SHORT:
		return make([]uint16, n)
	case LONG:
		return make([]uint32, n)
	case RATIONAL:
		return make([]Rational, n)
	case SBYTE:
		return make([]int8, n)
	case UNDEFINED:
		return make([]byte, n)
	case SSHORT:
		return make([]int16, n)
	case SLONG:
		return make([]int32, n)
	case SRATIONAL:
		return make([]SRational, n)
	case FLOAT:
		return make([]float32, n)
	case DOUBLE:
		return make([]float64, n)
	}
	return nil
}

type Tag uint16

const (
	NewSubfileType            Tag = 254   //  *uint32
	SubfileType               Tag = 255   //  *uint16
	ImageWidth                Tag = 256   //  *uint32
	ImageLength               Tag = 257   //  *uint32
	BitsPerSample             Tag = 258   //  []uint16
	Compression               Tag = 259   //  *uint16
	PhotometricInterpretation Tag = 262   //  *uint16
	Threshholding             Tag = 263   //  *uint16
	CellWidth                 Tag = 264   //  *uint16
	CellLength                Tag = 265   //  *uint16
	FillOrder                 Tag = 266   //  *uint16
	ImageDescription          Tag = 270   //  *string
	Make                      Tag = 271   //  *string
	Model                     Tag = 272   //  *string
	StripOffsets              Tag = 273   //  []uint32
	Orientation               Tag = 274   //  *uint16
	SamplesPerPixel           Tag = 277   //  *uint16
	RowsPerStrip              Tag = 278   //  *uint32
	StripByteCounts           Tag = 279   //  []uint32
	MinSampleValue            Tag = 280   //  *uint16
	MaxSampleValue            Tag = 281   //  *uint16
	XResolution               Tag = 282   //  *big.Rat
	YResolution               Tag = 283   //  *big.Rat
	PlanarConfiguration       Tag = 284   //  *uint16
	FreeOffsets               Tag = 288   //  []uint32
	FreeByteCounts            Tag = 289   //  []uint32
	GrayResponseUnit          Tag = 290   //  *uint16
	GrayResponseCurve         Tag = 291   //  []uint16
	ResolutionUnit            Tag = 296   //  *uint16
	Software                  Tag = 305   //  *string
	DateTime                  Tag = 306   //  *string
	Artist                    Tag = 315   //  *string
	HostComputer              Tag = 316   //  *string
	ColorMap                  Tag = 320   //  []uint16
	ExtraSamples              Tag = 338   //  []byte
	SampleFormat              Tag = 339   // *short
	Copyright                 Tag = 33432 //  *string

	ModelPixelScale     Tag = 33550 //  []float64
	ModelTiepoint       Tag = 33922 //  []float64
	ModelTransformation Tag = 34264 //  []float64
	GeoKeyDirectory     Tag = 34735 //  []uint16
	GeoDoubleParams     Tag = 34736 //  []float64
	GeoAsciiParams      Tag = 34737 //  *string

	GDALMetadata Tag = 42112
	GDALNodata   Tag = 42113
)

type CompressionType uint16

const (
	Uncompressed CompressionType = 1
	CCITT1D      CompressionType = 2
	Group3Fax    CompressionType = 3
	Group4Fax    CompressionType = 4
	LZW          CompressionType = 5
	JPEG         CompressionType = 6
	PackBits     CompressionType = 32773
)

type PhotometricInterpretationType uint16

const (
	WhiteIsZero      PhotometricInterpretationType = 0
	BlackIsZero      PhotometricInterpretationType = 1
	RGB              PhotometricInterpretationType = 2
	RGBPalette       PhotometricInterpretationType = 3
	TransparencyMask PhotometricInterpretationType = 4
	CMYK             PhotometricInterpretationType = 5
	YCbCr            PhotometricInterpretationType = 6
	CIELab           PhotometricInterpretationType = 8
)

type SampleFormatType uint16

const (
	Unsigned  SampleFormatType = 1
	Signed    SampleFormatType = 2
	Float     SampleFormatType = 3
	Undefined SampleFormatType = 4
)
