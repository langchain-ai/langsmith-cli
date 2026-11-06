package channels

// The icon a Slack channel's app gets when the channel names none: a mirrored
// six-by-six dot glyph and a color pair, both derived from the app's name.

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"unicode/utf16"
)

const (
	iconImageSize   = 512
	iconGridSize    = 6
	iconCellSize    = 53
	iconDotSize     = 43
	iconGlyphSize   = iconGridSize * iconCellSize
	iconPadding     = (iconImageSize - iconGlyphSize) / 2
	iconRGBChannels = 3
	iconPatternSize = iconGridSize * iconGridSize / 2
)

type rgb [iconRGBChannels]byte

type iconColors struct {
	foreground rgb
	background rgb
}

var lcPalette = [5]iconColors{
	{foreground: rgb{0x7f, 0xc8, 0xff}, background: rgb{0x07, 0x16, 0x22}},
	{foreground: rgb{0xe3, 0xff, 0x8f}, background: rgb{0x1b, 0x22, 0x07}},
	{foreground: rgb{0xb2, 0x7d, 0x75}, background: rgb{0x22, 0x0b, 0x07}},
	{foreground: rgb{0xc7, 0x8e, 0xad}, background: rgb{0x22, 0x07, 0x16}},
	{foreground: rgb{0xd5, 0xc3, 0xf7}, background: rgb{0x10, 0x07, 0x22}},
}

type iconModel struct {
	pattern    [iconPatternSize]bool
	foldRows   bool
	foreground rgb
	background rgb
}

// generatedIcon is a generated PNG and the background color it was drawn on.
type generatedIcon struct {
	png             []byte
	backgroundColor string
}

// generateIcon draws the icon for name.
func generateIcon(name string) (generatedIcon, error) {
	model := newIconModel(name)
	png, err := encodePNG(render(model))
	if err != nil {
		return generatedIcon{}, err
	}
	return generatedIcon{png: png, backgroundColor: hexColor(model.background)}, nil
}

func render(model iconModel) []byte {
	pixels := bytes.Repeat(model.background[:], iconImageSize*iconImageSize)

	for row := range iconGridSize {
		for column := range iconGridSize {
			if !model.pattern[mirroredIndex(row, column, model.foldRows)] {
				continue
			}
			paintDot(pixels, row, column, model.foreground)
		}
	}

	return pixels
}

func newIconModel(name string) iconModel {
	random := newXorShift32(fnv1a(name))
	randomPattern := func() [iconPatternSize]bool {
		var pattern [iconPatternSize]bool
		for i := range pattern {
			pattern[i] = random.nextBool()
		}
		return pattern
	}
	pattern := randomPattern()
	foldRows := random.nextBool()

	for range 63 {
		if patternTouchesAllEdges(pattern, foldRows) {
			break
		}
		pattern = randomPattern()
		foldRows = random.nextBool()
	}
	if !patternTouchesAllEdges(pattern, foldRows) {
		for i := range pattern {
			pattern[i] = true
		}
	}

	colors := lcPalette[int(random.next())%len(lcPalette)]

	return iconModel{
		pattern:    pattern,
		foldRows:   foldRows,
		foreground: colors.foreground,
		background: colors.background,
	}
}

func patternTouchesAllEdges(pattern [iconPatternSize]bool, foldRows bool) bool {
	var top, bottom, left, right bool
	for i := range iconGridSize {
		top = top || pattern[mirroredIndex(0, i, foldRows)]
		bottom = bottom || pattern[mirroredIndex(iconGridSize-1, i, foldRows)]
		left = left || pattern[mirroredIndex(i, 0, foldRows)]
		right = right || pattern[mirroredIndex(i, iconGridSize-1, foldRows)]
	}
	return top && bottom && left && right
}

func mirroredIndex(row, column int, foldRows bool) int {
	if foldRows {
		return min(row, iconGridSize-1-row)*iconGridSize + column
	}
	return row*(iconGridSize/2) + min(column, iconGridSize-1-column)
}

func paintDot(pixels []byte, row, column int, color rgb) {
	inset := (iconCellSize - iconDotSize) / 2
	left := iconPadding + column*iconCellSize + inset
	top := iconPadding + row*iconCellSize + inset

	for dotY := range iconDotSize {
		deltaY := absDiff(2*dotY+1, iconDotSize)
		for dotX := range iconDotSize {
			deltaX := absDiff(2*dotX+1, iconDotSize)
			if deltaX*deltaX+deltaY*deltaY > iconDotSize*iconDotSize {
				continue
			}

			offset := ((top+dotY)*iconImageSize + left + dotX) * iconRGBChannels
			copy(pixels[offset:offset+iconRGBChannels], color[:])
		}
	}
}

func absDiff(a, b int) int {
	if a > b {
		return a - b
	}
	return b - a
}

// fnv1a hashes name's UTF-16 code units, as the TypeScript runtime does.
func fnv1a(name string) uint32 {
	hash := uint32(0x811c_9dc5)
	for _, unit := range utf16.Encode([]rune(name)) {
		hash = (hash ^ uint32(unit)) * 0x0100_0193
	}
	return hash
}

type xorShift32 struct {
	state uint32
}

func newXorShift32(seed uint32) *xorShift32 {
	if seed == 0 {
		seed = 0x9e37_79b9
	}
	return &xorShift32{state: seed}
}

func (x *xorShift32) next() uint32 {
	x.state ^= x.state << 13
	x.state ^= x.state >> 17
	x.state ^= x.state << 5
	return x.state
}

func (x *xorShift32) nextBool() bool {
	return x.next() < 0x8000_0000
}

func hexColor(color rgb) string {
	return fmt.Sprintf("#%02x%02x%02x", color[0], color[1], color[2])
}

func encodePNG(pixels []byte) ([]byte, error) {
	rowBytes := iconImageSize * iconRGBChannels
	scanlines := make([]byte, 0, iconImageSize*(rowBytes+1))
	for row := 0; row < len(pixels); row += rowBytes {
		scanlines = append(scanlines, 0)
		scanlines = append(scanlines, pixels[row:row+rowBytes]...)
	}

	var compressed bytes.Buffer
	encoder := zlib.NewWriter(&compressed)
	if _, err := encoder.Write(scanlines); err != nil {
		return nil, fmt.Errorf("failed to encode generated channel icon: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return nil, fmt.Errorf("failed to finish generated channel icon: %w", err)
	}

	png := make([]byte, 0, compressed.Len()+57)
	png = append(png, pngSignature...)
	var header [13]byte
	binary.BigEndian.PutUint32(header[0:4], iconImageSize)
	binary.BigEndian.PutUint32(header[4:8], iconImageSize)
	copy(header[8:], []byte{8, 2, 0, 0, 0})
	png = writeChunk(png, "IHDR", header[:])
	png = writeChunk(png, "IDAT", compressed.Bytes())
	png = writeChunk(png, "IEND", nil)
	return png, nil
}

func writeChunk(png []byte, kind string, data []byte) []byte {
	png = binary.BigEndian.AppendUint32(png, uint32(len(data)))
	png = append(png, kind...)
	png = append(png, data...)

	crc := crc32.Update(crc32.ChecksumIEEE([]byte(kind)), crc32.IEEETable, data)
	return binary.BigEndian.AppendUint32(png, crc)
}
