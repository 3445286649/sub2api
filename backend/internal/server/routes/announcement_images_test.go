package routes

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAnnouncementImageUploadAndServe(t *testing.T) {
	gin.SetMode(gin.TestMode)
	images := newAnnouncementImages(t.TempDir())
	router := gin.New()
	router.POST("/admin/announcements/images", images.upload)
	router.GET("/announcement-images/:name", images.serve)

	requestImage := func(data []byte) *httptest.ResponseRecorder {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		part, err := writer.CreateFormFile("image", "sample.png")
		require.NoError(t, err)
		_, err = part.Write(data)
		require.NoError(t, err)
		require.NoError(t, writer.Close())
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/admin/announcements/images", &body)
		request.Header.Set("Content-Type", writer.FormDataContentType())
		router.ServeHTTP(recorder, request)
		return recorder
	}

	var pngData bytes.Buffer
	require.NoError(t, png.Encode(&pngData, image.NewNRGBA(image.Rect(0, 0, 1, 1))))
	uploaded := requestImage(pngData.Bytes())
	require.Equal(t, http.StatusOK, uploaded.Code)
	var result struct {
		Data struct {
			URL string `json:"url"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(uploaded.Body.Bytes(), &result))
	require.Regexp(t, `/api/v1/announcement-images/[a-f0-9]{32}\.png$`, result.Data.URL)

	served := httptest.NewRecorder()
	router.ServeHTTP(served, httptest.NewRequest(http.MethodGet, strings.TrimPrefix(result.Data.URL, "/api/v1"), nil))
	require.Equal(t, http.StatusOK, served.Code)
	require.Equal(t, pngData.Bytes(), served.Body.Bytes())
	require.Equal(t, "nosniff", served.Header().Get("X-Content-Type-Options"))

	require.Equal(t, http.StatusBadRequest, requestImage([]byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`)).Code)
	require.Equal(t, http.StatusBadRequest, requestImage(bytes.Repeat([]byte("x"), maxAnnouncementImageBytes+1)).Code)

	invalid := httptest.NewRecorder()
	router.ServeHTTP(invalid, httptest.NewRequest(http.MethodGet, "/announcement-images/invalid.svg", nil))
	require.Equal(t, http.StatusNotFound, invalid.Code)
}

func TestAnnouncementImageUploadRequiresAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	deny := middleware.AdminAuthMiddleware(func(c *gin.Context) {
		c.AbortWithStatus(http.StatusUnauthorized)
	})
	passAudit := middleware.AuditLogMiddleware(func(c *gin.Context) { c.Next() })
	passStepUp := middleware.StepUpAuthMiddleware(func(c *gin.Context) { c.Next() })
	RegisterAdminRoutes(router.Group("/api/v1"), &handler.Handlers{Admin: &handler.AdminHandlers{}}, deny, passAudit, passStepUp, nil, nil, t.TempDir())
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/admin/announcements/images", nil))
	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}
