# geotiff

A small, stdlib-only Go reader for TIFF / [OGC GeoTIFF](https://docs.ogc.org/is/19-008r4/19-008r4.html)
rasters, geared towards single-band DEMs and satellite imagery
(OpenTopography AW3D30/GLO-30, Sentinel-2 COGs).

## Features

- Plain TIFF IFD walking: `OpenTIFF(io.ReaderAt)` returns the first
  `*IFD`; `Next()` follows the IFD chain (e.g. COG overview levels);
  `Value(tag)` gives raw tag access, with named constants for the
  TIFF and GeoTIFF tag sets.
- `ReadSamples(ifd)` returns the full W×L raster as `[]float32`
  regardless of layout: tiled or stripped, uncompressed, LZW or
  Deflate, with TIFF predictor 2 (horizontal differencing) and
  predictor 3 (floating-point) undone. Single-sample (grayscale)
  images only — int16, uint16 and float32 sample formats.
- `Image()` on uncompressed stripped grayscale/RGB IFDs returns a
  standard `image.Image`.

No cgo, no dependencies outside the standard library.

## Example

```go
f, _ := os.Open("dem.tif")
ifd, err := geotiff.OpenTIFF(f)
if err != nil { ... }
w, l := ifd.WxL()
samples, err := geotiff.ReadSamples(ifd) // len == w*l, row-major
```

## Provenance

The core TIFF reader was originally written by lvd; it was extended
in the vnavsim/edgelab visual-navigation projects and then in
geostore, where it grew tiling, LZW/Deflate decompression and the
two predictors to read the GeoTIFF DEMs served by OpenTopography.
Promoted to a standalone module in 2026 when it reached its third
consumer.
