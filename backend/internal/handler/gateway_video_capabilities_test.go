//go:build unit

package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestVideoCapabilitiesAuthenticationAndInput(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, authenticated := range []bool{false, true} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/v1/video-capabilities", nil)
		if authenticated {
			c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{Group: &service.Group{ID: 24}})
		}
		(&GatewayHandler{}).VideoCapabilities(c)
		if authenticated {
			require.Equal(t, http.StatusBadRequest, w.Code)
		} else {
			require.Equal(t, http.StatusUnauthorized, w.Code)
		}
	}
}
