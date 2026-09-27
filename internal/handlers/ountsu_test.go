package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"seanime/internal/core"
	"seanime/internal/ountsu"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestHandleOuntsuInvite(t *testing.T) {
	e := echo.New()
	hub := ountsu.NewHub()
	app := &core.App{
		OuntsuHub: hub,
	}
	h := &Handler{App: app}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ountsu/invite", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.HandleOuntsuInvite(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var res map[string]string
	err = json.Unmarshal(rec.Body.Bytes(), &res)
	assert.NoError(t, err)
	assert.NotEmpty(t, res["inviteKey"])
	assert.Equal(t, 8, len(res["inviteKey"]))
}

func TestHandleOuntsuWS_Validation(t *testing.T) {
	e := echo.New()
	hub := ountsu.NewHub()
	app := &core.App{
		OuntsuHub: hub,
	}
	h := &Handler{App: app}

	t.Run("missing parameters", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/ountsu/ws", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := h.HandleOuntsuWS(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("not implemented when hub is nil", func(t *testing.T) {
		hNil := &Handler{App: &core.App{}}
		req := httptest.NewRequest(http.MethodGet, "/api/v1/ountsu/ws?roomId=r1&userId=u1", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := hNil.HandleOuntsuWS(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusNotImplemented, rec.Code)
	})
}
