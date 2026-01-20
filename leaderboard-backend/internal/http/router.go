package http

import (
	"net/http"
	"time"

	"github.com/AnshKumar10/Matiks-Assignment/internal/http/handlers"
	"github.com/AnshKumar10/Matiks-Assignment/internal/leaderboard"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
)

func NewRouter(lbService *leaderboard.Service) http.Handler {
	r := chi.NewRouter()

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:8081", "https://matiks-assignment.vercel.app"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           int((12 * time.Hour).Seconds()),
	}))

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("ok"))
	})

	lbHandler := handlers.NewLeaderboardHandler(lbService)

	r.Post("/random-rating", lbHandler.UpdateRandomUserRating)

	r.Route("/leaderboard", func(r chi.Router) {
		r.Get("/", lbHandler.GetTop)          // Top 10
		r.Get("/nearby", lbHandler.GetNearby) // ±4 users
		r.Get("/me", lbHandler.GetMe)         // current user rank
		r.Get("/users/search", lbHandler.Search)
	})

	return r
}
