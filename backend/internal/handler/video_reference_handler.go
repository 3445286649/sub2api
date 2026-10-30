package handler

import (
	"bytes"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	_ "golang.org/x/image/webp"
)

const maxVideoReferenceBytes = 5 << 20

func (h *AsyncImageHandler) UploadVideoReference(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if h == nil || h.referenceStorage == nil {
		response.ErrorFrom(c, http.ErrNotSupported)
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxVideoReferenceBytes+1024)
	file, _, err := c.Request.FormFile("image")
	if err != nil {
		response.BadRequest(c, "Image is required (maximum 5 MB)")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxVideoReferenceBytes+1))
	if err != nil || len(data) == 0 || len(data) > maxVideoReferenceBytes {
		response.BadRequest(c, "Image is empty or exceeds 5 MB")
		return
	}
	contentType := http.DetectContentType(data)
	if contentType != "image/png" && contentType != "image/jpeg" && contentType != "image/webp" {
		response.BadRequest(c, "Only PNG, JPEG and WebP images are supported")
		return
	}
	imageInfo, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || imageInfo.Width <= 0 || imageInfo.Height <= 0 || imageInfo.Width > 8192 || imageInfo.Height > 8192 || int64(imageInfo.Width)*int64(imageInfo.Height) > 32_000_000 {
		response.BadRequest(c, "Image is invalid or exceeds the dimension limit")
		return
	}
	url, err := h.referenceStorage.SaveVideoReference(c.Request.Context(), subject.UserID, contentType, data)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"url": url})
}
