//go:build unit

package handler

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type videoReferenceStorageStub struct {
	keys []string
}

func (s *videoReferenceStorageStub) Save(_ context.Context, key, contentType string, data []byte) (string, error) {
	s.keys = append(s.keys, key)
	return "https://assets.example/" + key, nil
}

func TestUploadVideoReferenceValidatesOwnerAndImage(t *testing.T) {
	storage := &videoReferenceStorageStub{}
	settings := service.NewImageStorageSettingService(nil, nil, nil, func(context.Context, *config.ImageStorageConfig) (service.ImageStorage, error) {
		return storage, nil
	}, config.ImageStorageConfig{Enabled: true, Bucket: "images", AccessKeyID: "test", SecretAccessKey: "test", Prefix: "images/"})
	h := ProvideAsyncImageHandler(nil, nil, settings)
	var imageData bytes.Buffer
	require.NoError(t, png.Encode(&imageData, image.NewRGBA(image.Rect(0, 0, 2, 2))))
	upload := func(data []byte, authenticated bool) *httptest.ResponseRecorder {
		var payload bytes.Buffer
		writer := multipart.NewWriter(&payload)
		part, err := writer.CreateFormFile("image", "cat.png")
		require.NoError(t, err)
		_, err = part.Write(data)
		require.NoError(t, err)
		require.NoError(t, writer.Close())
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/user/video-references", &payload)
		c.Request.Header.Set("Content-Type", writer.FormDataContentType())
		if authenticated {
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 7})
		}
		h.UploadVideoReference(c)
		return rec
	}
	require.Equal(t, http.StatusUnauthorized, upload(imageData.Bytes(), false).Code)
	require.Equal(t, http.StatusBadRequest, upload([]byte("not an image"), true).Code)
	require.Empty(t, storage.keys)
	first := upload(imageData.Bytes(), true)
	require.Equal(t, http.StatusOK, first.Code, first.Body.String())
	require.Contains(t, gjson.Get(first.Body.String(), "data.url").String(), "/video-references/7/")
	second := upload(imageData.Bytes(), true)
	require.Equal(t, http.StatusOK, second.Code)
	require.Len(t, storage.keys, 2)
	require.NotEqual(t, storage.keys[0], storage.keys[1])
	require.True(t, strings.HasSuffix(storage.keys[0], ".png"))
	h.referenceStorage = nil
	require.NotEqual(t, http.StatusOK, upload(imageData.Bytes(), true).Code)
	require.Len(t, storage.keys, 2)
}
