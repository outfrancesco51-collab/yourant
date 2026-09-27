package downloader_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yourant/internal/downloader"
)

func TestStorageGuard_ResolveSafePath_AggressiveTraversal(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "yourant_downloads_guard_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	guard, err := downloader.NewStorageGuard(tempDir)
	if err != nil {
		t.Fatalf("failed to instantiate StorageGuard: %v", err)
	}

	traversalPayloads := []struct {
		name    string
		payload string
	}{
		{"unix_simple_parent", "../evil.exe"},
		{"unix_double_parent", "../../etc/passwd"},
		{"unix_triple_parent", "../../../var/log/syslog"},
		{"windows_simple_parent", `..\evil.exe`},
		{"windows_double_parent", `..\..\Windows\System32\cmd.exe`},
		{"windows_nested_parent", `sub\..\..\evil.exe`},
		{"mixed_slashes_1", `sub/..\../evil.exe`},
		{"mixed_slashes_2", `..\sub/../../evil.exe`},
		{"quad_dot_slash", `....//....//evil.exe`},
		{"dot_dot_slash_dot", `.././../evil.exe`},
		{"encoded_dot_dot", `%2e%2e%2fevil.exe`},
		{"double_encoded", `%252e%252e%252fevil.exe`},
		{"absolute_windows_c", `C:\Windows\System32\calc.exe`},
		{"absolute_windows_d", `D:\hacked.txt`},
		{"absolute_unix_root", `/etc/shadow`},
		{"unc_network_share", `\\192.168.1.100\share\evil.exe`},
		{"nt_namespace", `\\?\C:\Windows\System32\notepad.exe`},
		{"device_namespace", `\\.\PhysicalDrive0`},
		{"current_dir_only", `.`},
		{"parent_dir_only", `..`},
		{"slashes_only", `///`},
		{"backslashes_only", `\\\\`},
	}

	for _, tc := range traversalPayloads {
		t.Run(tc.name, func(t *testing.T) {
			path, err := guard.ResolveSafePath(tc.payload)
			if err == nil {
				t.Fatalf("SECURITY VIOLATION: expected error for payload %q, got resolved path: %q", tc.payload, path)
			}
			if path != "" && !strings.HasPrefix(strings.ToLower(path), strings.ToLower(tempDir)) {
				t.Fatalf("CRITICAL ESCAPE: path %q is outside sandbox %q", path, tempDir)
			}
		})
	}
}

func TestStorageGuard_WindowsReservedDeviceNames(t *testing.T) {
	reservedNames := []string{
		"CON", "con", "cOn",
		"PRN", "prn",
		"AUX", "aux",
		"NUL", "nul",
		"COM1", "com1", "COM9", "com9",
		"LPT1", "lpt1", "LPT9", "lpt9",
		"CONIN$", "CONOUT$",
		"con.mp4", "CON.MKV", "aux.part", "nul.txt", "prn.tar.gz",
		"com1.json", "lpt3.bin",
	}

	for _, name := range reservedNames {
		t.Run(name, func(t *testing.T) {
			err := downloader.ValidateFilename(name)
			if err == nil {
				t.Errorf("expected ValidateFilename to reject Windows reserved name %q, but accepted", name)
			}

			sanitized := downloader.SanitizeFilename(name)
			upperSanitized := strings.ToUpper(sanitized)
			if upperSanitized == strings.ToUpper(name) && (strings.HasPrefix(upperSanitized, "CON") || strings.HasPrefix(upperSanitized, "AUX") || strings.HasPrefix(upperSanitized, "NUL") || strings.HasPrefix(upperSanitized, "PRN")) {
				t.Errorf("SanitizeFilename failed to neutralize reserved device name %q: got %q", name, sanitized)
			}
		})
	}
}

func TestStorageGuard_ForbiddenCharactersAndADS(t *testing.T) {
	tests := []struct {
		name  string
		input string
		valid bool
	}{
		{"ntfs_alternate_data_stream", "video.mp4:payload.exe", false},
		{"ntfs_data_stream", "video.mp4::$DATA", false},
		{"less_than", "anime<1>.mp4", false},
		{"greater_than", "anime>1.mp4", false},
		{"double_quote", "anime\"1\".mp4", false},
		{"pipe", "anime|1.mp4", false},
		{"asterisk", "anime*1.mp4", false},
		{"question_mark", "anime?1.mp4", false},
		{"null_byte", "anime\x00payload.mp4", false},
		{"control_char", "anime\x1Ftest.mp4", false},
		{"valid_japanese", "進撃の巨人_S4_EP1.mp4", true},
		{"valid_unicode_special", "SPY×FAMILY_01.mp4", true},
		{"valid_english", "Shingeki_No_Kyojin_Episode_01_1080p.mp4", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := downloader.ValidateFilename(tc.input)
			if tc.valid && err != nil {
				t.Errorf("expected %q to be valid, got error: %v", tc.input, err)
			}
			if !tc.valid && err == nil {
				t.Errorf("expected %q to be invalid, got nil error", tc.input)
			}

			sanitized := downloader.SanitizeFilename(tc.input)
			if strings.ContainsAny(sanitized, `<>:"/\|?*`) {
				t.Errorf("sanitized filename %q still contains forbidden characters", sanitized)
			}
		})
	}
}

func TestStorageGuard_WindowsTrailingCharsQuirk(t *testing.T) {
	inputs := []string{
		"file.mp4.",
		"file.mp4...",
		"file.mp4 ",
		"file.mp4    ",
		"file.mp4. . .",
		"....",
		"    ",
	}

	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			sanitized := downloader.SanitizeFilename(input)
			if strings.HasSuffix(sanitized, ".") || strings.HasSuffix(sanitized, " ") {
				t.Errorf("sanitized output %q still has trailing dot or space for input %q", sanitized, input)
			}
			if sanitized == "" {
				t.Errorf("sanitized output became empty for input %q", input)
			}
		})
	}
}

func TestStorageGuard_BaseDirContainment(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "yourant_containment_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	guard, err := downloader.NewStorageGuard(tempDir)
	if err != nil {
		t.Fatalf("failed to create guard: %v", err)
	}

	safePath, err := guard.ResolveSafePath("shingeki_01.mp4")
	if err != nil {
		t.Fatalf("failed to resolve valid path: %v", err)
	}
	expected := filepath.Join(guard.BaseDir(), "shingeki_01.mp4")
	if safePath != expected {
		t.Errorf("expected %q, got %q", expected, safePath)
	}

	_, err = guard.ResolveSafePath("../outside.mp4")
	if err == nil {
		t.Fatalf("expected error for traversal, got nil")
	}

	_, err = guard.ResolveSafePath("C:\\Windows\\win.ini")
	if err == nil {
		t.Fatalf("expected error for absolute path, got nil")
	}
}
