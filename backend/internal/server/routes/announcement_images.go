package routes

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
	_ "golang.org/x/image/webp"
)

const maxAnnouncementImageBytes = 5 << 20

var announcementImageName = regexp.MustCompile(`^[a-f0-9]{32}\.(png|jpg|gif|webp)$`)

type announcementImages struct{ directory string }

func newAnnouncementImages(dataDir string) *announcementImages {
	return &announcementImages{directory: filepath.Join(dataDir, "announcement-images")}
}

func (h *announcementImages) upload(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxAnnouncementImageBytes+1024)
	file, _, err := c.Request.FormFile("image")
	if err != nil {
		response.BadRequest(c, "Image is required (maximum 5 MB)")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxAnnouncementImageBytes+1))
	if err != nil || len(data) == 0 || len(data) > maxAnnouncementImageBytes {
		response.BadRequest(c, "Image is empty or exceeds 5 MB")
		return
	}
	var extension string
	switch http.DetectContentType(data) {
	case "image/png":
		extension = ".png"
	case "image/jpeg":
		extension = ".jpg"
	case "image/gif":
		extension = ".gif"
	case "image/webp":
		extension = ".webp"
	default:
		response.BadRequest(c, "Only PNG, JPEG, GIF and WebP images are supported")
		return
	}
	decoded, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || decoded.Width <= 0 || decoded.Height <= 0 || decoded.Width > 8192 || decoded.Height > 8192 || int64(decoded.Width)*int64(decoded.Height) > 32_000_000 {
		response.BadRequest(c, "Image is invalid or exceeds the dimension limit")
		return
	}

	if err := os.MkdirAll(h.directory, 0755); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	name := hex.EncodeToString(id[:]) + extension
	path := filepath.Join(h.directory, name)
	output, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	written, writeErr := output.Write(data)
	closeErr := output.Close()
	if writeErr != nil || closeErr != nil || written != len(data) {
		_ = os.Remove(path)
		response.ErrorFrom(c, io.ErrShortWrite)
		return
	}
	response.Success(c, gin.H{"url": "/api/v1/announcement-images/" + name})
}

func (h *announcementImages) serve(c *gin.Context) {
	name := c.Param("name")
	if !announcementImageName.MatchString(name) {
		c.Status(http.StatusNotFound)
		return
	}
	path := filepath.Join(h.directory, name)
	file, err := os.Open(path)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		c.Status(http.StatusNotFound)
		return
	}
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeContent(c.Writer, c.Request, name, info.ModTime(), file)
}
