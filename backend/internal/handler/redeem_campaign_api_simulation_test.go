package handler_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/ent/redeemcode"
	userhandler "github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"
)

type apiEnvelope struct {
	Reason string          `json:"reason"`
	Data   json.RawMessage `json:"data"`
}

func TestRedeemCampaignLocalAPISimulation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	databasePath := filepath.ToSlash(filepath.Join(t.TempDir(), "redeem-campaign-api.db"))
	db, err := sql.Open("sqlite", "file:"+databasePath+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	driver := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(driver)))
	t.Cleanup(func() { _ = client.Close() })

	ctx := context.Background()
	admin, err := client.User.Create().
		SetEmail("api-campaign-admin@example.com").
		SetPasswordHash("hash").
		SetRole(service.RoleAdmin).
		SetStatus(service.StatusActive).
		Save(ctx)
	require.NoError(t, err)
	user, err := client.User.Create().
		SetEmail("api-campaign-user@example.com").
		SetPasswordHash("hash").
		SetRole(service.RoleUser).
		SetStatus(service.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	redeemRepo := repository.NewRedeemCodeRepository(client)
	userRepo := repository.NewUserRepository(client, db)
	redeemService := service.NewRedeemService(redeemRepo, userRepo, nil, nil, nil, client, nil, nil)
	adminRedeemHandler := adminhandler.NewRedeemHandler(nil, redeemService)
	userRedeemHandler := userhandler.NewRedeemHandler(redeemService)

	router := gin.New()
	adminRoute := router.Group("/api/v1/admin")
	adminRoute.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: admin.ID})
		c.Next()
	})
	adminRoute.POST("/redeem-campaigns/generate", adminRedeemHandler.GenerateCampaign)
	userRoute := router.Group("/api/v1")
	userRoute.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: user.ID})
		c.Next()
	})
	userRoute.POST("/redeem", userRedeemHandler.Redeem)

	generateResponse := performJSONRequest(t, router, http.MethodPost, "/api/v1/admin/redeem-campaigns/generate", `{"name":"api-campaign","count":2,"value":7}`)
	require.Equal(t, http.StatusOK, generateResponse.Code, generateResponse.Body.String())

	var generateEnvelope apiEnvelope
	require.NoError(t, json.Unmarshal(generateResponse.Body.Bytes(), &generateEnvelope))
	var codes []struct {
		ID   int64  `json:"id"`
		Code string `json:"code"`
	}
	require.NoError(t, json.Unmarshal(generateEnvelope.Data, &codes))
	require.Len(t, codes, 2)

	firstRedeem := performJSONRequest(t, router, http.MethodPost, "/api/v1/redeem", `{"code":"`+codes[0].Code+`"}`)
	require.Equal(t, http.StatusOK, firstRedeem.Code, firstRedeem.Body.String())

	secondRedeem := performJSONRequest(t, router, http.MethodPost, "/api/v1/redeem", `{"code":"`+codes[1].Code+`"}`)
	require.Equal(t, http.StatusConflict, secondRedeem.Code, secondRedeem.Body.String())
	var conflictEnvelope apiEnvelope
	require.NoError(t, json.Unmarshal(secondRedeem.Body.Bytes(), &conflictEnvelope))
	require.Equal(t, "REDEEM_CAMPAIGN_ALREADY_REDEEMED", conflictEnvelope.Reason)

	unusedCode, err := client.RedeemCode.Query().Where(redeemcode.IDEQ(codes[1].ID)).Only(ctx)
	require.NoError(t, err)
	require.Equal(t, service.StatusUnused, unusedCode.Status)
	updatedUser, err := client.User.Get(ctx, user.ID)
	require.NoError(t, err)
	require.InDelta(t, 7, updatedUser.Balance, 0.000001)
}

func performJSONRequest(t *testing.T, router http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}
