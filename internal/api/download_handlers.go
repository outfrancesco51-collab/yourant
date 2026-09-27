package api

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"yourant/internal/downloader"
)

// HandleGetDownloads returns active and past download tasks.
func (h *Handler) HandleGetDownloads(w http.ResponseWriter, r *http.Request) {
	tasks := h.downloader.ListTasks()
	if tasks == nil {
		tasks = []*downloader.DownloadTask{}
	}
	writeJSON(w, http.StatusOK, tasks)
}

// HandleStartDownload initiates a new offline media download.
func (h *Handler) HandleStartDownload(w http.ResponseWriter, r *http.Request) {
	var req downloader.DownloadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	if req.URL == "" {
		writeError(w, http.StatusBadRequest, "url is required")
		return
	}

	// Validate scheme
	parsedURL, err := url.Parse(req.URL)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		writeError(w, http.StatusBadRequest, "url must have http or https scheme")
		return
	}

	task, err := h.downloader.Enqueue(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, task)
}

// HandleCancelDownload cancels and cleans up a download task.
func (h *Handler) HandleCancelDownload(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "Missing download ID")
		return
	}

	if err := h.downloader.Cancel(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"id":      id,
	})
}

// HandleListFiles lists completed media files in the downloads directory.
func (h *Handler) HandleListFiles(w http.ResponseWriter, r *http.Request) {
	files, err := h.downloader.ListFiles()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to list downloaded files")
		return
	}

	if files == nil {
		files = []downloader.FileInfo{}
	}

	writeJSON(w, http.StatusOK, files)
}

// HandleDeleteFile deletes a specific media file from controlled storage.
func (h *Handler) HandleDeleteFile(w http.ResponseWriter, r *http.Request) {
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

	// Reject traversal in filename
	if strings.Contains(fileName, "..") || strings.Contains(fileName, "/") || strings.Contains(fileName, "\\") {
		writeError(w, http.StatusBadRequest, "Security violation: path traversal detected")
		return
	}

	if err := h.downloader.DeleteFile(fileName); err != nil {
		writeError(w, http.StatusNotFound, "File not found or cannot be deleted")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success":  true,
		"fileName": fileName,
	})
}
