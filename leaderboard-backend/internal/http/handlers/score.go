package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/AnshKumar10/Matiks-Assignment/internal/workers"
)

type ScoreHandler struct {
	queue workers.Queue
}

func NewScoreHandler(queue workers.Queue) *ScoreHandler {
	return &ScoreHandler{queue: queue}
}

type scoreUpdateRequest struct {
	UserID string `json:"user_id"`
	Delta  int    `json:"delta"`
}

func (h *ScoreHandler) UpdateScore(w http.ResponseWriter, r *http.Request) {
	var req scoreUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request")
		return
	}

	if req.UserID == "" || req.Delta == 0 {
		respondError(w, http.StatusBadRequest, "invalid payload")
		return
	}

	event := workers.ScoreUpdateEvent{
		UserID:    req.UserID,
		Delta:     req.Delta,
		Timestamp: time.Now(),
	}

	if err := h.queue.Publish(r.Context(), event); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to enqueue")
		return
	}

	w.WriteHeader(http.StatusAccepted)
}
