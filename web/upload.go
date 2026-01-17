package web

import (
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/deemkeen/stegodon/db"
	"github.com/deemkeen/stegodon/util"
	"github.com/gin-gonic/gin"
	"golang.org/x/image/draw"
)

const (
	maxUploadSize  = 2 * 1024 * 1024 // 2MB
	maxAvatarSize  = 400             // Max dimension for avatars
	avatarsDirName = "avatars"
)

// getAvatarsDir returns the path to the avatars directory
func getAvatarsDir() (string, error) {
	configDir, err := util.GetConfigDir()
	if err != nil {
		return "", err
	}
	avatarsDir := filepath.Join(configDir, avatarsDirName)

	// Create directory if it doesn't exist
	if err := os.MkdirAll(avatarsDir, 0755); err != nil {
		return "", err
	}

	return avatarsDir, nil
}

// HandleUploadForm shows the upload form for a valid token
func HandleUploadForm(c *gin.Context, conf *util.AppConfig) {
	token := c.Param("token")

	database := db.GetDB()
	accountId, tokenType, err := database.ValidateUploadToken(token)
	if err != nil {
		log.Printf("Invalid upload token: %v", err)
		c.HTML(400, "upload.html", gin.H{
			"Error": "Invalid or expired upload link. Please generate a new one from the TUI.",
		})
		return
	}

	// Get account info for display
	err, account := database.ReadAccById(accountId)
	if err != nil || account == nil {
		log.Printf("Account not found for upload token: %v", err)
		c.HTML(400, "upload.html", gin.H{
			"Error": "Account not found.",
		})
		return
	}

	c.HTML(200, "upload.html", gin.H{
		"Username":  account.Username,
		"TokenType": tokenType,
		"Token":     token,
	})
}

// HandleUploadSubmit processes the uploaded file
func HandleUploadSubmit(c *gin.Context, conf *util.AppConfig) {
	token := c.Param("token")

	database := db.GetDB()
	accountId, tokenType, err := database.ValidateUploadToken(token)
	if err != nil {
		log.Printf("Invalid upload token on submit: %v", err)
		c.HTML(400, "upload.html", gin.H{
			"Error": "Invalid or expired upload link. Please generate a new one from the TUI.",
		})
		return
	}

	// Get account info
	err, account := database.ReadAccById(accountId)
	if err != nil || account == nil {
		log.Printf("Account not found for upload: %v", err)
		c.HTML(400, "upload.html", gin.H{
			"Error": "Account not found.",
		})
		return
	}

	// Limit request body size
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadSize)

	// Get the uploaded file
	file, header, err := c.Request.FormFile("avatar")
	if err != nil {
		log.Printf("Failed to get uploaded file: %v", err)
		c.HTML(400, "upload.html", gin.H{
			"Username":  account.Username,
			"TokenType": tokenType,
			"Token":     token,
			"Error":     "Please select a file to upload.",
		})
		return
	}
	defer file.Close()

	// Check file size
	if header.Size > maxUploadSize {
		c.HTML(400, "upload.html", gin.H{
			"Username":  account.Username,
			"TokenType": tokenType,
			"Token":     token,
			"Error":     "File too large. Maximum size is 2MB.",
		})
		return
	}

	// Detect content type from file content (not just header)
	buffer := make([]byte, 512)
	_, err = file.Read(buffer)
	if err != nil && err != io.EOF {
		log.Printf("Failed to read file for type detection: %v", err)
		c.HTML(400, "upload.html", gin.H{
			"Username":  account.Username,
			"TokenType": tokenType,
			"Token":     token,
			"Error":     "Failed to process uploaded file.",
		})
		return
	}

	// Reset file position
	file.Seek(0, 0)

	contentType := http.DetectContentType(buffer)
	if !isAllowedImageType(contentType) {
		c.HTML(400, "upload.html", gin.H{
			"Username":  account.Username,
			"TokenType": tokenType,
			"Token":     token,
			"Error":     "Invalid file type. Only PNG, JPEG, GIF, and WebP images are allowed.",
		})
		return
	}

	// Decode the image
	img, format, err := image.Decode(file)
	if err != nil {
		log.Printf("Failed to decode image: %v", err)
		c.HTML(400, "upload.html", gin.H{
			"Username":  account.Username,
			"TokenType": tokenType,
			"Token":     token,
			"Error":     "Failed to decode image. Please upload a valid image file.",
		})
		return
	}

	log.Printf("Uploaded image format: %s, size: %dx%d", format, img.Bounds().Dx(), img.Bounds().Dy())

	// Resize image if needed
	resized := resizeImage(img, maxAvatarSize)

	// Get avatars directory
	avatarsDir, err := getAvatarsDir()
	if err != nil {
		log.Printf("Failed to get avatars directory: %v", err)
		c.HTML(500, "upload.html", gin.H{
			"Username":  account.Username,
			"TokenType": tokenType,
			"Token":     token,
			"Error":     "Server error. Please try again later.",
		})
		return
	}

	// Save as PNG for consistency
	filename := fmt.Sprintf("%s.png", accountId.String())
	filepath := filepath.Join(avatarsDir, filename)

	outFile, err := os.Create(filepath)
	if err != nil {
		log.Printf("Failed to create avatar file: %v", err)
		c.HTML(500, "upload.html", gin.H{
			"Username":  account.Username,
			"TokenType": tokenType,
			"Token":     token,
			"Error":     "Server error. Please try again later.",
		})
		return
	}
	defer outFile.Close()

	if err := png.Encode(outFile, resized); err != nil {
		log.Printf("Failed to encode avatar as PNG: %v", err)
		c.HTML(500, "upload.html", gin.H{
			"Username":  account.Username,
			"TokenType": tokenType,
			"Token":     token,
			"Error":     "Server error. Please try again later.",
		})
		return
	}

	// Update account's avatar URL
	avatarURL := fmt.Sprintf("/avatars/%s", filename)
	if err := database.UpdateAccountAvatar(accountId, avatarURL); err != nil {
		log.Printf("Failed to update avatar URL in database: %v", err)
		c.HTML(500, "upload.html", gin.H{
			"Username":  account.Username,
			"TokenType": tokenType,
			"Token":     token,
			"Error":     "Server error. Please try again later.",
		})
		return
	}

	// Delete the used token
	if err := database.DeleteUploadToken(token); err != nil {
		log.Printf("Warning: Failed to delete used upload token: %v", err)
	}

	log.Printf("Avatar uploaded successfully for user %s", account.Username)

	c.HTML(200, "upload.html", gin.H{
		"Username": account.Username,
		"Success":  "Avatar uploaded successfully! You can close this page.",
	})
}

// isAllowedImageType checks if the content type is an allowed image type
func isAllowedImageType(contentType string) bool {
	allowed := []string{
		"image/png",
		"image/jpeg",
		"image/gif",
		"image/webp",
	}
	for _, t := range allowed {
		if strings.HasPrefix(contentType, t) {
			return true
		}
	}
	return false
}

// resizeImage resizes an image to fit within maxSize while maintaining aspect ratio
func resizeImage(img image.Image, maxSize int) image.Image {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// If image is already small enough, return as is
	if width <= maxSize && height <= maxSize {
		return img
	}

	// Calculate new dimensions maintaining aspect ratio
	var newWidth, newHeight int
	if width > height {
		newWidth = maxSize
		newHeight = (height * maxSize) / width
	} else {
		newHeight = maxSize
		newWidth = (width * maxSize) / height
	}

	// Create a new RGBA image for the resized result
	dst := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))

	// Use high-quality resampling
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, bounds, draw.Over, nil)

	return dst
}

// ServeAvatar serves avatar images from the avatars directory
func ServeAvatar(c *gin.Context, conf *util.AppConfig) {
	filename := c.Param("filename")

	// Security: only allow specific filename patterns
	if !isValidAvatarFilename(filename) {
		c.Status(404)
		return
	}

	avatarsDir, err := getAvatarsDir()
	if err != nil {
		c.Status(500)
		return
	}

	avatarPath := filepath.Join(avatarsDir, filename)

	// Check if file exists and get modification time
	fileInfo, err := os.Stat(avatarPath)
	if os.IsNotExist(err) {
		c.Status(404)
		return
	}
	if err != nil {
		c.Status(500)
		return
	}

	// Generate ETag based on modification time and size
	modTime := fileInfo.ModTime()
	etag := fmt.Sprintf(`"%x-%x"`, modTime.Unix(), fileInfo.Size())

	// Check If-None-Match header for cache validation
	if match := c.GetHeader("If-None-Match"); match == etag {
		c.Status(304) // Not Modified
		return
	}

	// Determine content type
	contentType := "image/png"
	if strings.HasSuffix(filename, ".jpg") || strings.HasSuffix(filename, ".jpeg") {
		contentType = "image/jpeg"
	} else if strings.HasSuffix(filename, ".gif") {
		contentType = "image/gif"
	}

	c.Header("Content-Type", contentType)
	c.Header("Cache-Control", "no-cache") // Always revalidate with server
	c.Header("ETag", etag)
	c.Header("Last-Modified", modTime.UTC().Format("Mon, 02 Jan 2006 15:04:05 GMT"))
	c.File(avatarPath)
}

// isValidAvatarFilename checks if the filename is a valid avatar filename
func isValidAvatarFilename(filename string) bool {
	// Must be a UUID followed by .png, .jpg, .jpeg, or .gif
	validExtensions := []string{".png", ".jpg", ".jpeg", ".gif"}
	hasValidExt := false
	for _, ext := range validExtensions {
		if strings.HasSuffix(filename, ext) {
			hasValidExt = true
			break
		}
	}
	if !hasValidExt {
		return false
	}

	// Extract the UUID part and validate
	baseName := filename
	for _, ext := range validExtensions {
		baseName = strings.TrimSuffix(baseName, ext)
	}

	// Simple UUID validation (36 chars with hyphens)
	if len(baseName) != 36 {
		return false
	}

	// Check format: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
	parts := strings.Split(baseName, "-")
	if len(parts) != 5 {
		return false
	}
	expectedLengths := []int{8, 4, 4, 4, 12}
	for i, part := range parts {
		if len(part) != expectedLengths[i] {
			return false
		}
		// Check that it's hex
		for _, c := range part {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				return false
			}
		}
	}

	return true
}

// Register image format decoders
func init() {
	// PNG and JPEG are registered by default
	// GIF needs explicit import for decode
	image.RegisterFormat("gif", "GIF8", gif.Decode, gif.DecodeConfig)
	image.RegisterFormat("jpeg", "\xff\xd8", jpeg.Decode, jpeg.DecodeConfig)
}
