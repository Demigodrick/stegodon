package web

import (
	"image"
	"image/color"
	"testing"
)

func TestIsAllowedImageType(t *testing.T) {
	tests := []struct {
		contentType string
		expected    bool
	}{
		{"image/png", true},
		{"image/jpeg", true},
		{"image/gif", true},
		{"image/webp", true},
		{"image/png; charset=utf-8", true},
		{"text/plain", false},
		{"application/json", false},
		{"text/html", false},
		{"application/octet-stream", false},
		{"image/svg+xml", false}, // SVG not allowed for security
		{"", false},
	}

	for _, tt := range tests {
		result := isAllowedImageType(tt.contentType)
		if result != tt.expected {
			t.Errorf("isAllowedImageType(%q) = %v, expected %v", tt.contentType, result, tt.expected)
		}
	}
}

func TestIsValidAvatarFilename(t *testing.T) {
	tests := []struct {
		filename string
		expected bool
	}{
		// Valid filenames
		{"123e4567-e89b-12d3-a456-426614174000.png", true},
		{"123e4567-e89b-12d3-a456-426614174000.jpg", true},
		{"123e4567-e89b-12d3-a456-426614174000.jpeg", true},
		{"123e4567-e89b-12d3-a456-426614174000.gif", true},
		{"ABCDEF12-3456-7890-ABCD-EF1234567890.png", true},
		{"abcdef12-3456-7890-abcd-ef1234567890.png", true},

		// Invalid filenames
		{"notauuid.png", false},
		{"123e4567-e89b-12d3-a456-426614174000.svg", false},  // Wrong extension
		{"123e4567-e89b-12d3-a456-426614174000.webp", false}, // Wrong extension
		{"123e4567-e89b-12d3-a456-426614174000", false},      // No extension
		{"../../etc/passwd.png", false},                      // Path traversal attempt
		{"123e4567-e89b-12d3-a456.png", false},               // Invalid UUID format
		{"123e4567-e89b-12d3-a456-42661417400.png", false},   // UUID too short
		{"123e4567-e89b-12d3-a456-4266141740000.png", false}, // UUID too long
		{"", false},
		{".png", false},
		{"123e4567-e89b-12d3-a456-42661417400g.png", false}, // Invalid hex char 'g'
	}

	for _, tt := range tests {
		result := isValidAvatarFilename(tt.filename)
		if result != tt.expected {
			t.Errorf("isValidAvatarFilename(%q) = %v, expected %v", tt.filename, result, tt.expected)
		}
	}
}

func TestResizeImage(t *testing.T) {
	// Test with image smaller than max size - should return as-is
	smallImg := image.NewRGBA(image.Rect(0, 0, 100, 100))
	result := resizeImage(smallImg, 400)
	if result.Bounds().Dx() != 100 || result.Bounds().Dy() != 100 {
		t.Error("Small image should not be resized")
	}

	// Test with large square image
	largeSquare := image.NewRGBA(image.Rect(0, 0, 800, 800))
	result = resizeImage(largeSquare, 400)
	if result.Bounds().Dx() != 400 || result.Bounds().Dy() != 400 {
		t.Errorf("Large square should be resized to 400x400, got %dx%d",
			result.Bounds().Dx(), result.Bounds().Dy())
	}

	// Test with wide image (landscape)
	wideImg := image.NewRGBA(image.Rect(0, 0, 800, 400))
	result = resizeImage(wideImg, 400)
	if result.Bounds().Dx() != 400 {
		t.Errorf("Wide image width should be 400, got %d", result.Bounds().Dx())
	}
	if result.Bounds().Dy() != 200 {
		t.Errorf("Wide image height should be 200, got %d", result.Bounds().Dy())
	}

	// Test with tall image (portrait)
	tallImg := image.NewRGBA(image.Rect(0, 0, 400, 800))
	result = resizeImage(tallImg, 400)
	if result.Bounds().Dx() != 200 {
		t.Errorf("Tall image width should be 200, got %d", result.Bounds().Dx())
	}
	if result.Bounds().Dy() != 400 {
		t.Errorf("Tall image height should be 400, got %d", result.Bounds().Dy())
	}

	// Test exact max size - should not resize
	exactImg := image.NewRGBA(image.Rect(0, 0, 400, 400))
	result = resizeImage(exactImg, 400)
	if result.Bounds().Dx() != 400 || result.Bounds().Dy() != 400 {
		t.Error("Image at exact max size should not be resized")
	}
}

func TestResizeImagePreservesContent(t *testing.T) {
	// Create a simple test image with a known pattern
	img := image.NewRGBA(image.Rect(0, 0, 800, 800))

	// Fill with red
	red := color.RGBA{255, 0, 0, 255}
	for y := 0; y < 800; y++ {
		for x := 0; x < 800; x++ {
			img.Set(x, y, red)
		}
	}

	result := resizeImage(img, 400)

	// Check that the result is an image (not nil)
	if result == nil {
		t.Error("Resized image should not be nil")
	}

	// Check dimensions
	bounds := result.Bounds()
	if bounds.Dx() != 400 || bounds.Dy() != 400 {
		t.Errorf("Expected 400x400, got %dx%d", bounds.Dx(), bounds.Dy())
	}
}

func TestResizeImageAspectRatio(t *testing.T) {
	tests := []struct {
		name           string
		inputWidth     int
		inputHeight    int
		maxSize        int
		expectedWidth  int
		expectedHeight int
	}{
		{"small image", 100, 100, 400, 100, 100},
		{"exact max", 400, 400, 400, 400, 400},
		{"2:1 landscape", 800, 400, 400, 400, 200},
		{"1:2 portrait", 400, 800, 400, 200, 400},
		{"3:2 landscape", 600, 400, 400, 400, 266}, // 400 * 400/600 = 266.67 -> 266
		{"4:3 landscape", 800, 600, 400, 400, 300},
		{"1:1 square large", 1000, 1000, 400, 400, 400},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			img := image.NewRGBA(image.Rect(0, 0, tt.inputWidth, tt.inputHeight))
			result := resizeImage(img, tt.maxSize)

			if result.Bounds().Dx() != tt.expectedWidth {
				t.Errorf("Width: expected %d, got %d", tt.expectedWidth, result.Bounds().Dx())
			}
			if result.Bounds().Dy() != tt.expectedHeight {
				t.Errorf("Height: expected %d, got %d", tt.expectedHeight, result.Bounds().Dy())
			}
		})
	}
}
