package handlers

import (
	"net/http"
	"strconv"

	"github.com/AnshKumar10/Matiks-Assignment/internal/leaderboard"
)

type LeaderboardHandler struct {
	service *leaderboard.Service
}

func NewLeaderboardHandler(service *leaderboard.Service) *LeaderboardHandler {
	return &LeaderboardHandler{service: service}
}

func (h *LeaderboardHandler) GetTop(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	limit := 10
	offset := 0

	if v := r.URL.Query().Get("limit"); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil || parsed <= 0 || parsed > 1000 {
			respondError(w, http.StatusBadRequest, "limit must be between 1 and 1000")
			return
		}
		limit = parsed
	}

	// parse offset
	if v := r.URL.Query().Get("offset"); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil || parsed < 0 {
			respondError(w, http.StatusBadRequest, "offset must be >= 0")
			return
		}
		offset = parsed
	}

	entries, err := h.service.GetTopLeaderboard(ctx, limit, offset)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := make([]LeaderboardResponse, 0, len(entries))
	for _, e := range entries {
		response = append(response, LeaderboardResponse{
			UserID:   e.UserID,
			Username: e.Username,
			Rating:   e.Rating,
			Rank:     e.Rank,
		})
	}

	respondJSON(w, http.StatusOK, response)
}

func (h *LeaderboardHandler) GetNearby(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		respondError(w, http.StatusBadRequest, "user_id is required")
		return
	}

	entries, err := h.service.GetNearbyPlayers(ctx, userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := make([]LeaderboardResponse, 0, len(entries))
	for _, e := range entries {
		response = append(response, LeaderboardResponse{
			UserID: e.UserID,
			Rating: e.Rating,
			Rank:   e.Rank,
		})
	}

	respondJSON(w, http.StatusOK, response)
}

func (h *LeaderboardHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		respondError(w, http.StatusBadRequest, "user_id is required")
		return
	}

	entry, err := h.service.GetUserRank(ctx, userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if entry == nil {
		respondError(w, http.StatusNotFound, "user not ranked")
		return
	}

	respondJSON(w, http.StatusOK, LeaderboardResponse{
		UserID:   entry.UserID,
		Username: entry.Username,
		Rating:   entry.Rating,
		Rank:     entry.Rank,
	})
}

func (h *LeaderboardHandler) Search(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := r.URL.Query().Get("q")
	if query == "" {
		respondError(w, http.StatusBadRequest, "query is required")
		return
	}

	results, err := h.service.SearchUsers(ctx, query)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := make([]LeaderboardResponse, 0, len(results))
	for _, u := range results {
		response = append(response, LeaderboardResponse{
			UserID:   u.UserID,
			Username: u.Username,
			Rating:   u.Rating,
			Rank:     u.Rank,
		})
	}

	respondJSON(w, http.StatusOK, response)
}
