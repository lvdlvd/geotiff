package geotiff

// lzwDecodeTIFF decodes a TIFF-flavoured LZW stream (TIFF6 §13). It differs
// from compress/lzw (GIF/PDF flavour) in the "early change": the code width is
// increased one code before the table is full, which is why the stdlib reader
// desyncs and reports "invalid code" on TIFF data. Codes are packed MSB-first,
// 9..12 bits, with ClearCode=256 and EndOfInfo=257.
func lzwDecodeTIFF(src []byte) ([]byte, error) {
	const (
		clearCode = 256
		eoiCode   = 257
		maxWidth  = 12
	)
	var (
		prefix [4096]int
		suffix [4096]byte
		first  [4096]byte
		stack  [4096]byte
	)
	reset := func() (int, int) {
		for i := 0; i < 256; i++ {
			suffix[i] = byte(i)
			first[i] = byte(i)
		}
		return 258, 9
	}

	out := make([]byte, 0, len(src)*3)
	bitPos, nbits := 0, len(src)*8
	readCode := func(width int) (int, bool) {
		if bitPos+width > nbits {
			return 0, false
		}
		c := 0
		for i := 0; i < width; i++ {
			b := src[bitPos>>3]
			c = (c << 1) | int((b>>(7-(bitPos&7)))&1)
			bitPos++
		}
		return c, true
	}

	nextCode, width := reset()
	oldCode := -1
	for {
		code, ok := readCode(width)
		if !ok || code == eoiCode {
			break
		}
		if code == clearCode {
			nextCode, width = reset()
			oldCode = -1
			continue
		}
		if oldCode == -1 { // first symbol after a clear is a literal
			out = append(out, first[code])
			oldCode = code
			continue
		}

		sp := 0
		cur := code
		if code >= nextCode { // KwKwK: code not yet in table
			stack[sp] = first[oldCode]
			sp++
			cur = oldCode
		}
		for cur >= 256 {
			stack[sp] = suffix[cur]
			sp++
			cur = prefix[cur]
		}
		stack[sp] = byte(cur)
		sp++
		for i := sp - 1; i >= 0; i-- { // emit the string (stack is reversed)
			out = append(out, stack[i])
		}

		if nextCode < 4096 { // append oldString + firstChar(curString)
			prefix[nextCode] = oldCode
			suffix[nextCode] = byte(cur)
			first[nextCode] = first[oldCode]
			nextCode++
			if nextCode == (1<<width)-1 && width < maxWidth { // early change
				width++
			}
		}
		oldCode = code
	}
	return out, nil
}
