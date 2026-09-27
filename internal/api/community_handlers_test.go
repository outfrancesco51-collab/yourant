package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"yourant/internal/community"
)

func TestHandleGetBanStatus(t *testing.T) {
	banSystem := community.NewBanSystem()
	h := &Handler{
		banSystem: banSystem,
	}

	// Ban user1
	banSystem.BanUser("user1", "toxicity", 7*24*time.Hour)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/community/ban/{id}", h.HandleGetBanStatus)

	req := httptest.NewRequest("GET", "/api/community/ban/user1", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp BanStatusResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if !resp.IsBanned {
		t.Errorf("Expected user to be banned")
	}
	if resp.Reason != "toxicity" {
		t.Errorf("Expected reason toxicity, got %s", resp.Reason)
	}

	// Unbanned user
	req = httptest.NewRequest("GET", "/api/community/ban/user2", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var resp2 BanStatusResponse
	json.NewDecoder(w.Body).Decode(&resp2)

	if resp2.IsBanned {
		t.Errorf("Expected user2 not to be banned")
	}
}
