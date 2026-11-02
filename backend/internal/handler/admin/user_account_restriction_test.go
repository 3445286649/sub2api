package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adminaccess"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type restrictionAdminStub struct{ service.AdminService }

func (restrictionAdminStub) GetUser(_ context.Context, id int64) (*service.User, error) {
	if id != 7 {
		return nil, service.ErrUserNotFound
	}
	return &service.User{ID: id}, nil
}

type restrictionStoreStub struct {
	rules  service.UserAccountRestrictions
	writes int
	err    error
}

func (s *restrictionStoreStub) GetByUserID(context.Context, int64) (service.UserAccountRestrictions, error) {
	return s.rules, s.err
}
func (s *restrictionStoreStub) Replace(_ context.Context, _ int64, group int64, ids []int64) error {
	if s.err != nil {
		return s.err
	}
	s.rules[group] = ids
	s.writes++
	return nil
}

type restrictionInvalidatorStub struct {
	service.APIKeyAuthCacheInvalidator
	users []int64
}

func (s *restrictionInvalidatorStub) InvalidateAuthCacheByUserID(_ context.Context, id int64) {
	s.users = append(s.users, id)
}

func TestUserAccountRestrictionAdminEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &restrictionStoreStub{rules: service.UserAccountRestrictions{1: {11}, 2: {22}}}
	invalidator := &restrictionInvalidatorStub{}
	h := &UserHandler{adminService: restrictionAdminStub{}}
	h.ConfigureAccountRestrictions(repo, invalidator)
	router := gin.New()
	router.GET("/users/:id/account-restrictions", h.GetAccountRestrictions)
	router.PUT("/users/:id/groups/:group_id/account-restrictions", h.UpdateAccountRestrictions)
	call := func(method, path, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		return w
	}
	require.Equal(t, http.StatusOK, call("PUT", "/users/7/groups/1/account-restrictions", `{"account_ids":[12]}`).Code)
	require.Equal(t, []int64{12}, repo.rules[1])
	require.Equal(t, []int64{22}, repo.rules[2], "other group must remain unchanged")
	require.Equal(t, []int64{7}, invalidator.users)
	require.Equal(t, http.StatusOK, call("GET", "/users/7/account-restrictions", "").Code)
	for _, body := range []string{`{}`, `{"account_ids":null}`, `{"account_ids":"11"}`} {
		require.Equal(t, http.StatusBadRequest, call("PUT", "/users/7/groups/1/account-restrictions", body).Code)
	}
	require.Equal(t, http.StatusNotFound, call("PUT", "/users/8/groups/1/account-restrictions", `{"account_ids":[]}`).Code)
	require.Equal(t, 1, repo.writes)
	repo.err = service.ErrInvalidAccountRestriction
	require.Equal(t, http.StatusBadRequest, call("PUT", "/users/7/groups/1/account-restrictions", `{"account_ids":[99]}`).Code)
	require.Len(t, invalidator.users, 1, "failed writes must not invalidate success state")
	repo.err = nil
	require.Equal(t, http.StatusOK, call("PUT", "/users/7/groups/1/account-restrictions", `{"account_ids":[]}`).Code)
	require.Empty(t, repo.rules[1])
}

func TestUserAccountRestrictionRejectsDelegatedAdminKey(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("PUT", "/", strings.NewReader(`{"account_ids":[11]}`))
	c.Request = c.Request.WithContext(adminaccess.WithPrincipal(c.Request.Context(), &adminaccess.Principal{}))
	h := &UserHandler{}
	h.UpdateAccountRestrictions(c)
	require.Equal(t, http.StatusForbidden, w.Code)
}
