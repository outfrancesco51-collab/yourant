package api

import (
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// HandleStreamVideo serves local media files supporting RFC 7233 range requests.
func (h *Handler) HandleStreamVideo(w http.ResponseWriter, r *http.Request) {
	rawName := r.PathValue("name")
	if rawName == "" {
		writeError(w, http.StatusBadRequest, "Missing filename")
		return
	}

	fileName, err := url.PathUnescape(rawName)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid URL encoding")
		return
	}

	// 1. Rigorous Controlled Storage & Path Traversal Security Check
	safePath, err := h.downloader.ResolveSafePath(fileName)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Access denied: invalid file path")
		return
	}

	// 2. Stat Target File
	fileInfo, err := os.Stat(safePath)
	if os.IsNotExist(err) {
		writeError(w, http.StatusNotFound, "Video file not found")
		return
	}
	if err != nil || fileInfo.IsDir() {
		writeError(w, http.StatusBadRequest, "Target is not a valid file")
		return
	}

	// Do not stream incomplete .part files
	if strings.HasSuffix(fileInfo.Name(), ".part") {
		writeError(w, http.StatusConflict, "File download is currently in progress")
		return
	}

	// 3. Detect Media MIME Type
	ext := strings.ToLower(filepath.Ext(fileName))
	mimeType := "video/mp4"
	switch ext {
	case ".mp4":
		mimeType = "video/mp4"
	case ".webm":
		mimeType = "video/webm"
	case ".mkv":
		mimeType = "video/x-matroska"
	case ".mov":
		mimeType = "video/quicktime"
	case ".avi":
		mimeType = "video/x-msvideo"
	default:
		if detected := mime.TypeByExtension(ext); detected != "" {
			mimeType = detected
		}
	}

	// 4. Open File for Streaming
	file, err := os.Open(safePath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Cannot open video file")
		return
	}
	defer file.Close()

	// 5. Set Essential Streaming Headers
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Expose-Headers", "Content-Range, Accept-Ranges, Content-Length, Content-Type")

	// 6. Serve Content with RFC 7233 Compliance
	http.ServeContent(w, r, fileInfo.Name(), fileInfo.ModTime(), file)
}
