package handlers

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

type BanStatusResponse struct {
	IsBanned bool    `json:"isBanned"`
	Reason   string  `json:"reason,omitempty"`
	DaysLeft float64 `json:"daysLeft,omitempty"`
}

// HandleGetBanStatus returns the ban status and remaining duration for a given user ID.
//
//	@summary gets user ban status
//	@route /api/v1/community/ban/:id [GET]
//	@returns BanStatusResponse
func (h *Handler) HandleGetBanStatus(c echo.Context) error {
	if h.App.BanSystem == nil {
		return c.String(http.StatusNotImplemented, "Community mod not enabled")
	}

	userID := c.Param("id")
	if userID == "" {
		return c.String(http.StatusBadRequest, "missing user id")
	}

	record, banned := h.App.BanSystem.GetBan(userID)
	var resp BanStatusResponse

	if banned && record != nil {
		resp.IsBanned = true
		resp.Reason = record.Reason
		resp.DaysLeft = time.Until(record.Expiration).Hours() / 24.0
	} else {
		resp.IsBanned = false
	}

	return c.JSON(http.StatusOK, resp)
}

// HandleCommunityPost processes a community post through AutoMod rules, sanitizes content and enforces bans.
//
//	@summary submits a community post for moderation
//	@route /api/v1/community/post [POST]
//	@returns map[string]interface{}
func (h *Handler) HandleCommunityPost(c echo.Context) error {
	if h.App.AutoMod == nil {
		return c.String(http.StatusNotImplemented, "Community mod not enabled")
	}

	var req struct {
		UserID  string `json:"userId"`
		Message string `json:"message"`
	}

	if err := c.Bind(&req); err != nil {
		return c.String(http.StatusBadRequest, "invalid request")
	}

	sanitizedMsg, wasBanned := h.App.AutoMod.ProcessMessage(req.UserID, req.Message)

	return c.JSON(http.StatusOK, map[string]interface{}{
		"sanitized": sanitizedMsg,
		"banned":    wasBanned,
	})
}
