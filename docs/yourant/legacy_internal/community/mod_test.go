package community

import (
	"testing"
	"time"
)

func TestAutoMod_ProcessMessage(t *testing.T) {
	banSystem := NewBanSystem()
	autoMod := NewAutoMod(banSystem)

	tests := []struct {
		name         string
		message      string
		wantSanitize string
		wantBanned   bool
	}{
		{
			name:         "clean message",
			message:      "I think the new episode is great!",
			wantSanitize: "I think the new episode is great!",
			wantBanned:   false,
		},
		{
			name:         "toxic message with kill yourself",
			message:      "you are stupid, kill yourself",
			wantSanitize: "you are stupid, ***",
			wantBanned:   true,
		},
		{
			name:         "toxic message with threaten",
			message:      "I will threaten you",
			wantSanitize: "I will *** you",
			wantBanned:   true,
		},
		{
			name:         "clean message with similar word",
			message:      "diet coke",
			wantSanitize: "diet coke",
			wantBanned:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, banned := autoMod.ProcessMessage("user1", tt.message)
			if got != tt.wantSanitize {
				t.Errorf("AutoMod.ProcessMessage() got = %v, want %v", got, tt.wantSanitize)
			}
			if banned != tt.wantBanned {
				t.Errorf("AutoMod.ProcessMessage() banned = %v, want %v", banned, tt.wantBanned)
			}
		})
	}

	// Verify ban was created
	record, banned := banSystem.GetBan("user1")
	if !banned || record == nil {
		t.Errorf("Expected user1 to be banned")
	}
	if record.Reason != "Threatening behavior or death wishes" {
		t.Errorf("Expected correct ban reason, got %s", record.Reason)
	}
	if time.Until(record.Expiration) < 6*24*time.Hour {
		t.Errorf("Expected ban to be around 7 days")
	}
}
