package downloader

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

var (
	ErrPathTraversal      = errors.New("security violation: path traversal detected")
	ErrReservedDeviceName = errors.New("security violation: windows reserved device name")
	ErrInvalidFilename    = errors.New("invalid filename: contains forbidden characters or empty")
	ErrEscapeDetected     = errors.New("security violation: file escapes target downloads directory")
)

// Windows forbidden characters: < > : " / \ | ? * and ASCII 0x00-0x1F
var forbiddenCharsRegex = regexp.MustCompile(`[<>:"/\\|?*\x00-\x1F]`)

// Windows reserved DOS device names (case-insensitive)
var reservedDeviceNames = map[string]bool{
	"CON": true, "PRN": true, "AUX": true, "NUL": true,
	"COM1": true, "COM2": true, "COM3": true, "COM4": true, "COM5": true,
	"COM6": true, "COM7": true, "COM8": true, "COM9": true,
	"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true, "LPT5": true,
	"LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
	"CONIN$": true, "CONOUT$": true,
}

// StorageGuard polices directory sandboxing and path traversal defenses.
type StorageGuard struct {
	baseDir string // Canonical absolute base directory
}

// NewStorageGuard creates a storage guard for baseDir and ensures the directory exists.
func NewStorageGuard(baseDir string) (*StorageGuard, error) {
	if strings.TrimSpace(baseDir) == "" {
		return nil, errors.New("base directory cannot be empty")
	}

	cleanBase := filepath.Clean(baseDir)
	absBase, err := filepath.Abs(cleanBase)
	if err != nil {
		return nil, fmt.Errorf("failed to determine absolute path for base directory: %w", err)
	}

	// Create directory if not exists
	if err := os.MkdirAll(absBase, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base directory: %w", err)
	}

	// Resolve symlinks if any
	evalBase, err := filepath.EvalSymlinks(absBase)
	if err == nil {
		absBase = evalBase
	}

	return &StorageGuard{baseDir: absBase}, nil
}

// BaseDir returns the policed base directory path.
func (sg *StorageGuard) BaseDir() string {
	return sg.baseDir
}

// SanitizeFilename transforms an untrusted filename into a safe, valid filesystem name.
func SanitizeFilename(rawName string) string {
	name := strings.TrimSpace(rawName)
	if name == "" {
		return "download_file.mp4"
	}

	// 1. Replace forbidden characters
	sanitized := forbiddenCharsRegex.ReplaceAllString(name, "_")

	// 2. Strip non-printable or dangerous unicode control characters
	sanitized = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == '\u0000' {
			return '_'
		}
		return r
	}, sanitized)

	// 3. Trim trailing dots and spaces (Windows auto-strips these)
	sanitized = strings.Trim(sanitized, ". ")

	if sanitized == "" {
		return "download_file.mp4"
	}

	// 4. Check for Windows reserved DOS device names
	ext := filepath.Ext(sanitized)
	base := strings.TrimSuffix(sanitized, ext)
	primaryPart := strings.Split(base, ".")[0]
	primaryUpper := strings.ToUpper(strings.TrimSpace(primaryPart))

	if reservedDeviceNames[primaryUpper] {
		sanitized = "_" + sanitized
	}

	// 5. Length clamping (max 200 bytes to stay well within NTFS 255-byte limit)
	if len(sanitized) > 200 {
		if len(ext) < 20 {
			stemLen := 200 - len(ext)
			sanitized = sanitized[:stemLen] + ext
		} else {
			sanitized = sanitized[:200]
		}
	}

	return sanitized
}

// ValidateFilename tests whether rawName is safe without mutation.
func ValidateFilename(rawName string) error {
	trimmed := strings.Trim(rawName, ". ")
	if trimmed == "" {
		return ErrInvalidFilename
	}

	if forbiddenCharsRegex.MatchString(rawName) {
		return ErrInvalidFilename
	}

	ext := filepath.Ext(trimmed)
	base := strings.TrimSuffix(trimmed, ext)
	primaryPart := strings.Split(base, ".")[0]
	primaryUpper := strings.ToUpper(strings.TrimSpace(primaryPart))

	if reservedDeviceNames[primaryUpper] {
		return ErrReservedDeviceName
	}

	return nil
}

// ResolveSafePath validates and joins a filename to baseDir, strictly verifying containment.
func (sg *StorageGuard) ResolveSafePath(rawName string) (string, error) {
	unescaped := rawName
	for i := 0; i < 5; i++ {
		u, err := url.QueryUnescape(unescaped)
		if err != nil || u == unescaped {
			break
		}
		unescaped = u
	}

	// Reject explicit traversal patterns in input up front
	if strings.Contains(unescaped, "..") || strings.Contains(unescaped, "/") || strings.Contains(unescaped, "\\") ||
		strings.Contains(unescaped, "\x00") || strings.Trim(rawName, ". ") == "" || rawName == "." || rawName == ".." {
		return "", ErrPathTraversal
	}

	safeName := SanitizeFilename(rawName)
	if safeName == "" || safeName == "." || safeName == ".." {
		return "", ErrInvalidFilename
	}

	// Join with base directory and clean
	targetPath := filepath.Clean(filepath.Join(sg.baseDir, safeName))

	// Assert relative path does not escape
	rel, err := filepath.Rel(sg.baseDir, targetPath)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrEscapeDetected, err)
	}

	// Boundary escape checks
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", ErrPathTraversal
	}

	// Case-insensitive prefix assertion for Windows drive containment
	normBase := strings.ToLower(filepath.Clean(sg.baseDir))
	if !strings.HasSuffix(normBase, string(filepath.Separator)) {
		normBase += string(filepath.Separator)
	}
	normTarget := strings.ToLower(targetPath)

	if !strings.HasPrefix(normTarget, normBase) {
		return "", ErrEscapeDetected
	}

	return targetPath, nil
}
