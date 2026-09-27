package api

import (
	"net/http"
	"strconv"
)

// HandleGetTrending returns trending anime.
func (h *Handler) HandleGetTrending(w http.ResponseWriter, r *http.Request) {
	page, perPage := parsePagination(r)
	result, err := h.anilist.GetTrending(r.Context(), page, perPage)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to retrieve trending anime")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// HandleGetPopular returns popular anime.
func (h *Handler) HandleGetPopular(w http.ResponseWriter, r *http.Request) {
	page, perPage := parsePagination(r)
	result, err := h.anilist.GetPopular(r.Context(), page, perPage)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to retrieve popular anime")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// HandleSearchAnime searches anime by keyword.
func (h *Handler) HandleSearchAnime(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		q = r.URL.Query().Get("query")
	}

	page, perPage := parsePagination(r)
	result, err := h.anilist.Search(r.Context(), q, page, perPage)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to search anime")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// HandleGetAnimeDetail retrieves comprehensive details for a single anime by ID.
func (h *Handler) HandleGetAnimeDetail(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "Invalid anime ID")
		return
	}

	detail, err := h.anilist.GetDetail(r.Context(), id)
	if err != nil || detail == nil {
		writeError(w, http.StatusNotFound, "Anime not found")
		return
	}

	writeJSON(w, http.StatusOK, detail)
}

func parsePagination(r *http.Request) (int, int) {
	page := 1
	if pStr := r.URL.Query().Get("page"); pStr != "" {
		if p, err := strconv.Atoi(pStr); err == nil && p > 0 {
			page = p
		}
	}

	perPage := 20
	if ppStr := r.URL.Query().Get("perPage"); ppStr != "" {
		if pp, err := strconv.Atoi(ppStr); err == nil && pp > 0 {
			perPage = pp
		}
	}

	if perPage > 50 {
		perPage = 50
	}

	return page, perPage
}
