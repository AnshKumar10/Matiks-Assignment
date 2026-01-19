package leaderboard

import (
	"context"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetTopLeaderboard(ctx context.Context, limit, offset int) ([]LeaderboardEntry, error) {
	entries, err := s.repo.GetTop(ctx, limit, offset)
	if err != nil {
		return nil, err
	}

	userIDs := make([]string, len(entries))
	for i, e := range entries {
		userIDs[i] = e.UserID
	}

	usernames, err := s.repo.GetUsernames(ctx, userIDs)
	if err != nil {
		return nil, err
	}

	for i := range entries {
		entries[i].Username = usernames[entries[i].UserID]
	}

	return entries, nil
}

// GetNearbyPlayers returns users around a given user
func (s *Service) GetNearbyPlayers(ctx context.Context, userID string) ([]LeaderboardEntry, error) {
	entries, err := s.repo.GetNearby(ctx, userID, 4)
	if err != nil {
		return nil, err
	}

	userIDs := make([]string, len(entries))
	for i, e := range entries {
		userIDs[i] = e.UserID
	}

	usernames, err := s.repo.GetUsernames(ctx, userIDs)
	if err != nil {
		return nil, err
	}

	for i := range entries {
		entries[i].Username = usernames[entries[i].UserID]
	}

	return entries, nil
}

// GetUserRank returns a user's rank and rating
func (s *Service) GetUserRank(ctx context.Context, userID string) (*LeaderboardEntry, error) {
	rating, err := s.repo.GetUserRating(ctx, userID)
	if err != nil {
		return nil, err
	}

	if rating == 0 {
		return nil, nil
	}

	rank, err := s.repo.GetDenseRank(ctx, userID)
	if err != nil {
		return nil, err
	}

	username, err := s.repo.GetUsername(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &LeaderboardEntry{
		UserID:   userID,
		Username: username,
		Rating:   rating,
		Rank:     rank,
	}, nil
}

// SearchUsers searches users by username
func (s *Service) SearchUsers(ctx context.Context, query string) ([]LeaderboardEntry, error) {
	return s.repo.SearchUsers(ctx, query)
}
