package handlers

type LeaderboardResponse struct {
	UserID   string `json:"user_id"`
	Rating   int    `json:"rating"`
	Username string `json:"username"`
	Rank     int    `json:"rank"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}
