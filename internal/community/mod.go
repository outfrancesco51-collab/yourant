package community

import (
	"regexp"
	"sync"
	"time"
)

type BanRecord struct {
	UserID     string    `json:"userId"`
	Reason     string    `json:"reason"`
	Expiration time.Time `json:"expiration"`
}

type BanSystem struct {
	bans map[string]*BanRecord
	mu   sync.RWMutex
}

func NewBanSystem() *BanSystem {
	return &BanSystem{
		bans: make(map[string]*BanRecord),
	}
}

func (bs *BanSystem) BanUser(userID, reason string, duration time.Duration) {
	bs.mu.Lock()
	defer bs.mu.Unlock()
	bs.bans[userID] = &BanRecord{
		UserID:     userID,
		Reason:     reason,
		Expiration: time.Now().Add(duration),
	}
}

func (bs *BanSystem) GetBan(userID string) (*BanRecord, bool) {
	bs.mu.RLock()
	defer bs.mu.RUnlock()
	record, ok := bs.bans[userID]
	if ok {
		if time.Now().After(record.Expiration) {
			return nil, false
		}
		return record, true
	}
	return nil, false
}

type AutoMod struct {
	banSystem  *BanSystem
	toxicRegex *regexp.Regexp
}

func NewAutoMod(banSystem *BanSystem) *AutoMod {
	return &AutoMod{
		banSystem:  banSystem,
		// Regex to detect death wishes and threats
		toxicRegex: regexp.MustCompile(`(?i)\b(kill yourself|die|death to|murder|threaten)\b`),
	}
}

// PushNotification simulates pushing to a "bell".
func (am *AutoMod) PushNotification(userID, message string) {
	// Simulated bell push notification mechanism
}

// ProcessMessage checks for guidelines violations, sanitizes the message,
// issues a ban if necessary, and returns the sanitized message and whether a ban occurred.
func (am *AutoMod) ProcessMessage(userID, message string) (string, bool) {
	if am.toxicRegex.MatchString(message) {
		sanitized := am.toxicRegex.ReplaceAllString(message, "***")
		
		am.PushNotification(userID, "Your message violated community guidelines. You have been banned for 7 days.")
		am.banSystem.BanUser(userID, "Threatening behavior or death wishes", 7*24*time.Hour)
		
		return sanitized, true
	}
	
	return message, false
}
