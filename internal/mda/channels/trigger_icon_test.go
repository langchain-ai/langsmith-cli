package channels

import (
	"bytes"
	"fmt"
	"image/png"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var expectedPalette = []iconColors{
	{foreground: rgb{0x7f, 0xc8, 0xff}, background: rgb{0x07, 0x16, 0x22}},
	{foreground: rgb{0xe3, 0xff, 0x8f}, background: rgb{0x1b, 0x22, 0x07}},
	{foreground: rgb{0xb2, 0x7d, 0x75}, background: rgb{0x22, 0x0b, 0x07}},
	{foreground: rgb{0xc7, 0x8e, 0xad}, background: rgb{0x22, 0x07, 0x16}},
	{foreground: rgb{0xd5, 0xc3, 0xf7}, background: rgb{0x10, 0x07, 0x22}},
}

func TestGenerateReturnsA512PixelRGBPNG(t *testing.T) {
	icon, err := generateIcon("Support Bot")
	require.NoError(t, err)

	assert.Equal(t, []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x02\x00\x00\x00\x02\x00\x08\x02"), icon.png[:26])
	// A PNG decoder reads it back at its full size.
	decoded, err := png.Decode(bytes.NewReader(icon.png))
	require.NoError(t, err)
	assert.Equal(t, 512, decoded.Bounds().Dx())
	assert.Equal(t, 512, decoded.Bounds().Dy())
}

func TestGenerateIsStableForTheSameName(t *testing.T) {
	first, err := generateIcon("Support Bot")
	require.NoError(t, err)
	second, err := generateIcon("Support Bot")
	require.NoError(t, err)
	assert.Equal(t, first, second)
}

func TestGeneratedBackgroundMatchesTheRenderedBackground(t *testing.T) {
	pixels := render(newIconModel("Support Bot"))
	icon, err := generateIcon("Support Bot")
	require.NoError(t, err)

	assert.Equal(t, hexColor(pixel(pixels, 0, 0)), icon.backgroundColor)
}

func TestIconModelUsesEveryLCColorPair(t *testing.T) {
	actual := map[iconColors]bool{}
	for index := range 100 {
		model := newIconModel(fmt.Sprintf("Slack Bot %d", index))
		actual[iconColors{foreground: model.foreground, background: model.background}] = true
	}
	expected := map[iconColors]bool{}
	for _, colors := range expectedPalette {
		expected[colors] = true
	}

	assert.Equal(t, expected, actual)
}

func TestRenderKeepsTheOuterPaddingAtTheBackgroundColor(t *testing.T) {
	model := newIconModel("Support Bot")
	pixels := render(model)

	inGlyph := func(value int) bool { return value >= iconPadding && value < iconPadding+iconGlyphSize }
	for y := range iconImageSize {
		for x := range iconImageSize {
			if !inGlyph(x) || !inGlyph(y) {
				require.Equal(t, model.background, pixel(pixels, x, y), "(%d, %d)", x, y)
			}
		}
	}
}

func TestRenderUsesOnlyTheSelectedPaletteColors(t *testing.T) {
	model := newIconModel("Support Bot")
	pixels := render(model)
	actual := map[rgb]bool{}
	for offset := 0; offset < len(pixels); offset += iconRGBChannels {
		actual[rgb{pixels[offset], pixels[offset+1], pixels[offset+2]}] = true
	}

	assert.Equal(t, map[rgb]bool{model.background: true, model.foreground: true}, actual)
}

func TestRenderPaintsOccupiedCellsAsSolidCircles(t *testing.T) {
	model := newIconModel("Support Bot")
	model.pattern = [iconPatternSize]bool{}
	model.pattern[0] = true
	pixels := render(model)

	left := iconPadding + 5
	top := iconPadding + 5
	center := iconPadding + iconCellSize/2

	assert.Equal(t,
		[]rgb{model.background, model.foreground, model.foreground},
		[]rgb{pixel(pixels, left, top), pixel(pixels, center, top), pixel(pixels, center, center)})
}

func TestRenderUsesOriginalVisibleOuterPadding(t *testing.T) {
	model := newIconModel("Support Bot")
	for i := range model.pattern {
		model.pattern[i] = true
	}
	pixels := render(model)

	assert.Equal(t, model.background, pixel(pixels, 101, 123))
	assert.Equal(t, model.foreground, pixel(pixels, 102, 123))
}

func TestGlyphUsesASixBySixGrid(t *testing.T) {
	assert.Equal(t, []int{6, 53, 43, 318, 97}, []int{iconGridSize, iconCellSize, iconDotSize, iconGlyphSize, iconPadding})
}

func TestGeneratedPatternsTouchAllFourGridEdges(t *testing.T) {
	for index := range 1000 {
		model := newIconModel(fmt.Sprintf("Slack Bot %d", index))

		require.True(t, patternTouchesAllEdges(model.pattern, model.foldRows),
			"pattern for Slack Bot %d does not touch every edge", index)
	}
}

func TestRenderPaintsALighterGlyph(t *testing.T) {
	model := newIconModel("Support Bot")
	pixels := render(model)
	brightness := func(color rgb) int { return int(color[0]) + int(color[1]) + int(color[2]) }

	lighter := false
	for y := range iconImageSize {
		for x := range iconImageSize {
			lighter = lighter || brightness(pixel(pixels, x, y)) > brightness(model.background)
		}
	}

	assert.True(t, lighter)
}

func pixel(pixels []byte, x, y int) rgb {
	offset := (y*iconImageSize + x) * iconRGBChannels
	return rgb{pixels[offset], pixels[offset+1], pixels[offset+2]}
}
