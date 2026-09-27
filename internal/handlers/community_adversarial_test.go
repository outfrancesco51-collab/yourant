package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"seanime/internal/community"
	"seanime/internal/core"
	"seanime/internal/handlers"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupCommunityServer(t *testing.T) (*httptest.Server, *community.BanSystem, *community.AutoMod) {
	e := echo.New()
	banSystem := community.NewBanSystem()
	autoMod := community.NewAutoMod(banSystem)
	app := &core.App{
		BanSystem: banSystem,
		AutoMod:   autoMod,
	}
	h := &handlers.Handler{App: app}

	v1 := e.Group("/api/v1")
	v1Community := v1.Group("/community")
	v1Community.GET("/ban/:id", h.HandleGetBanStatus)
	v1Community.POST("/post", h.HandleCommunityPost)

	ts := httptest.NewServer(e)
	t.Cleanup(ts.Close)
	return ts, banSystem, autoMod
}

func TestCommunity_AdversarialInputs_And_Bans(t *testing.T) {
	ts, banSystem, _ := setupCommunityServer(t)

	testCases := []struct {
		name          string
		userID        string
		message       string
		expectBanned  bool
		expectCleanIn string
	}{
		// Toxic variations
		{
			name:          "mixed case kill yourself",
			userID:        "user_toxic_1",
			message:       "I think you should KiLl YoUrSeLf immediately",
			expectBanned:  true,
			expectCleanIn: "***",
		},
		{
			name:          "uppercase DIE",
			userID:        "user_toxic_2",
			message:       "Just DIE already!",
			expectBanned:  true,
			expectCleanIn: "***",
		},
		{
			name:          "sentence with murder",
			userID:        "user_toxic_3",
			message:       "I will murder that character in the game",
			expectBanned:  true,
			expectCleanIn: "***",
		},
		{
			name:          "threaten with punctuation",
			userID:        "user_toxic_4",
			message:       "Why do you threaten everyone here?",
			expectBanned:  true,
			expectCleanIn: "***",
		},
		{
			name:          "death to with prefix",
			userID:        "user_toxic_5",
			message:       "It was death to the villains",
			expectBanned:  true,
			expectCleanIn: "***",
		},

		// Benign edge cases (should NOT trigger ban)
		{
			name:          "diet coke boundary check",
			userID:        "user_benign_1",
			message:       "I love drinking diet coke while watching anime",
			expectBanned:  false,
			expectCleanIn: "diet coke",
		},
		{
			name:          "medieval boundary check",
			userID:        "user_benign_2",
			message:       "This fantasy anime has great medieval architecture",
			expectBanned:  false,
			expectCleanIn: "medieval",
		},
		{
			name:          "audience boundary check",
			userID:        "user_benign_3",
			message:       "The audience gave a standing ovation",
			expectBanned:  false,
			expectCleanIn: "audience",
		},
		{
			name:          "soldier boundary check",
			userID:        "user_benign_4",
			message:       "The soldier protected the kingdom",
			expectBanned:  false,
			expectCleanIn: "soldier",
		},
		{
			name:          "threat detection boundary check",
			userID:        "user_benign_5",
			message:       "The threat level was maximum in episode 5",
			expectBanned:  false,
			expectCleanIn: "threat level",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(map[string]string{
				"userId":  tc.userID,
				"message": tc.message,
			})
			resp, err := http.Post(ts.URL+"/api/v1/community/post", "application/json", bytes.NewReader(body))
			require.NoError(t, err)
			defer resp.Body.Close()
			assert.Equal(t, http.StatusOK, resp.StatusCode)

			var postResult map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&postResult)
			require.NoError(t, err)

			assert.Equal(t, tc.expectBanned, postResult["banned"])
			sanitized := postResult["sanitized"].(string)
			assert.Contains(t, sanitized, tc.expectCleanIn)

			// Query ban status via GET endpoint
			getResp, err := http.Get(ts.URL + "/api/v1/community/ban/" + tc.userID)
			require.NoError(t, err)
			defer getResp.Body.Close()
			assert.Equal(t, http.StatusOK, getResp.StatusCode)

			var status handlers.BanStatusResponse
			err = json.NewDecoder(getResp.Body).Decode(&status)
			require.NoError(t, err)

			assert.Equal(t, tc.expectBanned, status.IsBanned)
			if tc.expectBanned {
				assert.NotEmpty(t, status.Reason)
				// Ban should be ~7 days (between 6.9 and 7.1 days)
				assert.GreaterOrEqual(t, status.DaysLeft, 6.9)
				assert.LessOrEqual(t, status.DaysLeft, 7.1)
			}
		})
	}

	// Verify manual short-term ban expiration
	banSystem.BanUser("temp_user", "Testing expiration", 50*time.Millisecond)
	record, banned := banSystem.GetBan("temp_user")
	assert.True(t, banned)
	assert.NotNil(t, record)

	// Wait for expiration
	time.Sleep(70 * time.Millisecond)
	recordExpired, bannedAfter := banSystem.GetBan("temp_user")
	assert.False(t, bannedAfter, "Ban must expire after duration has elapsed")
	assert.Nil(t, recordExpired)
}

func TestCommunity_HighConcurrency_PostsAndBans(t *testing.T) {
	ts, _, _ := setupCommunityServer(t)

	const workerCount = 20
	const requestsPerWorker = 30
	var wg sync.WaitGroup
	wg.Add(workerCount)

	for w := 0; w < workerCount; w++ {
		go func(workerID int) {
			defer wg.Done()
			for r := 0; r < requestsPerWorker; r++ {
				uid := strings.Repeat("usr_", 1) + string(rune('A'+workerID))
				isToxic := (r % 2 == 1)
				msg := "Harmless discussion about anime plot"
				if isToxic {
					msg = "Go die right now you fool"
				}

				body, _ := json.Marshal(map[string]string{
					"userId":  uid,
					"message": msg,
				})

				resp, err := http.Post(ts.URL+"/api/v1/community/post", "application/json", bytes.NewReader(body))
				if err != nil {
					t.Errorf("Post failed: %v", err)
					return
				}
				_ = resp.Body.Close()

				// Concurrent read of ban status
				statusResp, err := http.Get(ts.URL + "/api/v1/community/ban/" + uid)
				if err != nil {
					t.Errorf("Get ban failed: %v", err)
					return
				}
				_ = statusResp.Body.Close()
			}
		}(w)
	}

	wg.Wait()
}

func TestCommunity_MalformedRequests(t *testing.T) {
	ts, _, _ := setupCommunityServer(t)

	// Post malformed JSON
	resp, err := http.Post(ts.URL+"/api/v1/community/post", "application/json", strings.NewReader(`{malformed: json`))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	// Post empty body: Echo binds empty body to default zero-value struct
	respEmpty, err := http.Post(ts.URL+"/api/v1/community/post", "application/json", strings.NewReader(``))
	require.NoError(t, err)
	defer respEmpty.Body.Close()
	assert.Equal(t, http.StatusOK, respEmpty.StatusCode)
	var emptyResult map[string]interface{}
	_ = json.NewDecoder(respEmpty.Body).Decode(&emptyResult)
	assert.Equal(t, false, emptyResult["banned"])
	assert.Equal(t, "", emptyResult["sanitized"])

	// Query nonexistent user
	getResp, err := http.Get(ts.URL + "/api/v1/community/ban/nonexistent_user_999")
	require.NoError(t, err)
	defer getResp.Body.Close()
	assert.Equal(t, http.StatusOK, getResp.StatusCode)

	var status handlers.BanStatusResponse
	_ = json.NewDecoder(getResp.Body).Decode(&status)
	assert.False(t, status.IsBanned)
}
