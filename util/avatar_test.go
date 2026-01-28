package util

import (
	"image"
	"image/color"
	"strings"
	"testing"
)

func TestRgbToAnsi256_PureBlack(t *testing.T) {
	idx := rgbToAnsi256(0, 0, 0)
	if idx != 16 {
		t.Errorf("Expected pure black → 16, got %d", idx)
	}
}

func TestRgbToAnsi256_PureWhite(t *testing.T) {
	idx := rgbToAnsi256(255, 255, 255)
	if idx != 231 {
		t.Errorf("Expected pure white → 231, got %d", idx)
	}
}

func TestRgbToAnsi256_PureRed(t *testing.T) {
	idx := rgbToAnsi256(255, 0, 0)
	if idx != 196 {
		t.Errorf("Expected pure red → 196, got %d", idx)
	}
}

func TestRgbToAnsi256_PureGreen(t *testing.T) {
	idx := rgbToAnsi256(0, 255, 0)
	if idx != 46 {
		t.Errorf("Expected pure green → 46, got %d", idx)
	}
}

func TestRgbToAnsi256_PureBlue(t *testing.T) {
	idx := rgbToAnsi256(0, 0, 255)
	if idx != 21 {
		t.Errorf("Expected pure blue → 21, got %d", idx)
	}
}

func TestRgbToAnsi256_MidGray(t *testing.T) {
	idx := rgbToAnsi256(128, 128, 128)
	// Mid-gray should map to the grayscale ramp
	if idx < 232 || idx > 255 {
		t.Errorf("Expected mid-gray in grayscale range 232-255, got %d", idx)
	}
}

func TestRenderImageToHalfBlocks_1x2(t *testing.T) {
	// Create a 1x2 image: top pixel red, bottom pixel blue
	img := image.NewRGBA(image.Rect(0, 0, 1, 2))
	img.Set(0, 0, color.RGBA{255, 0, 0, 255})
	img.Set(0, 1, color.RGBA{0, 0, 255, 255})

	result := RenderImageToHalfBlocks(img, 1, 1)

	if !strings.Contains(result, "▀") {
		t.Error("Expected half-block character in output")
	}
	if !strings.Contains(result, "\033[38;5;") {
		t.Error("Expected foreground ANSI escape code")
	}
	if !strings.Contains(result, "\033[48;5;") {
		t.Error("Expected background ANSI escape code")
	}
	if !strings.HasSuffix(result, "\033[0m") {
		t.Error("Expected reset escape code at end")
	}
}

func TestRenderImageToHalfBlocks_MultiRow(t *testing.T) {
	// Create a 2x4 image (should render as 2 cols x 2 rows of characters)
	img := image.NewRGBA(image.Rect(0, 0, 2, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 2; x++ {
			img.Set(x, y, color.RGBA{100, 150, 200, 255})
		}
	}

	result := RenderImageToHalfBlocks(img, 2, 2)

	lines := strings.Split(result, "\n")
	if len(lines) != 2 {
		t.Errorf("Expected 2 lines, got %d", len(lines))
	}

	for i, line := range lines {
		if !strings.HasSuffix(line, "\033[0m") {
			t.Errorf("Line %d should end with reset code", i)
		}
	}
}

func TestLoadAvatarImage_EmptyURL(t *testing.T) {
	img := LoadAvatarImage("")
	if img != nil {
		t.Error("Expected nil for empty URL")
	}
}

func TestLoadAvatarImage_NonexistentFile(t *testing.T) {
	img := LoadAvatarImage("/avatars/nonexistent-file-12345.png")
	if img != nil {
		t.Error("Expected nil for nonexistent file")
	}
}

func TestCubeIndex(t *testing.T) {
	tests := []struct {
		input    uint8
		expected int
	}{
		{0, 0},
		{47, 0},
		{48, 1},
		{95, 1},
		{114, 1},
		{115, 2},
		{135, 2},
		{155, 3},
		{195, 4},
		{235, 5},
		{255, 5},
	}

	for _, tt := range tests {
		result := cubeIndex(tt.input)
		if result != tt.expected {
			t.Errorf("cubeIndex(%d) = %d, expected %d", tt.input, result, tt.expected)
		}
	}
}

func TestCubeValue(t *testing.T) {
	tests := []struct {
		idx      int
		expected uint8
	}{
		{0, 0},
		{1, 95},
		{2, 135},
		{3, 175},
		{4, 215},
		{5, 255},
	}

	for _, tt := range tests {
		result := cubeValue(tt.idx)
		if result != tt.expected {
			t.Errorf("cubeValue(%d) = %d, expected %d", tt.idx, result, tt.expected)
		}
	}
}
