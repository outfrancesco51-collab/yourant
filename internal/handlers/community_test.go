package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"seanime/internal/community"
	"seanime/internal/core"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestHandleCommunity_ModerationFlow(t *testing.T) {
	e := echo.New()
	banSystem := community.NewBanSystem()
	autoMod := community.NewAutoMod(banSystem)
	app := &core.App{
		BanSystem: banSystem,
		AutoMod:   autoMod,
	}
	h := &Handler{App: app}

	// 1. Initial check: user is not banned
	{
		req := httptest.NewRequest(http.MethodGet, "/api/v1/community/ban/user123", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues("user123")

		err := h.HandleGetBanStatus(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		var res BanStatusResponse
		err = json.Unmarshal(rec.Body.Bytes(), &res)
		assert.NoError(t, err)
		assert.False(t, res.IsBanned)
	}

	// 2. Post toxic message: should sanitize and ban user
	{
		body := `{"userId":"user123","message":"I will murder you"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/community/post", strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := h.HandleCommunityPost(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		var res map[string]interface{}
		err = json.Unmarshal(rec.Body.Bytes(), &res)
		assert.NoError(t, err)
		assert.Equal(t, true, res["banned"])
		assert.Equal(t, "I will *** you", res["sanitized"])
	}

	// 3. Follow-up check: user is now banned
	{
		req := httptest.NewRequest(http.MethodGet, "/api/v1/community/ban/user123", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues("user123")

		err := h.HandleGetBanStatus(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		var res BanStatusResponse
		err = json.Unmarshal(rec.Body.Bytes(), &res)
		assert.NoError(t, err)
		assert.True(t, res.IsBanned)
		assert.NotEmpty(t, res.Reason)
		assert.Greater(t, res.DaysLeft, 6.0)
	}

	// 4. Post clean message from another user: not banned
	{
		body := `{"userId":"user456","message":"Hello world, love this anime!"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/community/post", strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := h.HandleCommunityPost(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		var res map[string]interface{}
		err = json.Unmarshal(rec.Body.Bytes(), &res)
		assert.NoError(t, err)
		assert.Equal(t, false, res["banned"])
		assert.Equal(t, "Hello world, love this anime!", res["sanitized"])
	}
}
