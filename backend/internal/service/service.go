package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog"
	"github.com/yourorg/leaderboard/internal/store"
)

var (
	// ErrPlayerNotFound is returned when a player doesn't exist
	ErrPlayerNotFound = errors.New("player not found")

	// ErrInvalidPlayerName is returned when player name validation fails
	ErrInvalidPlayerName = errors.New("invalid player name")

	// ErrInvalidScore is returned when score validation fails
	ErrInvalidScore = errors.New("invalid score")

	// ErrInvalidLimit is returned when limit parameter is invalid
	ErrInvalidLimit = errors.New("invalid limit")
)

const (
	MaxPlayerNameLength = 20
	MinPlayerNameLength = 1
)

// Service implements the leaderboard business logic
type Service struct {
	store  *store.Store
	logger *zerolog.Logger
}

// New creates a new Service instance
func New(s *store.Store, logger *zerolog.Logger) *Service {
	return &Service{
		store:  s,
		logger: logger,
	}
}

// ScoreResult represents the result of a score submission
type ScoreResult struct {
	PlayerName string
	Score      int64
	UpdatedAt  string
	Applied    bool // true if the score was new or improved
}

// SubmitScore submits or updates a player's score
// Returns true if the score was applied (new or improved)
func (s *Service) SubmitScore(ctx context.Context, playerName string, score int64) (*ScoreResult, error) {
	// Validate input
	if err := s.validatePlayerName(playerName); err != nil {
		return nil, err
	}
	if err := s.validateScore(score); err != nil {
		return nil, err
	}

	// Get current score before upsert (if exists)
	var oldScore int64
	var hadScore bool
	currentScore, err := s.store.GetPlayerScore(ctx, playerName)
	if err == nil {
		oldScore = currentScore.Score
		hadScore = true
	} else if !errors.Is(err, pgx.ErrNoRows) {
		s.logger.Error().Err(err).Str("player", playerName).Msg("failed to get current score")
		return nil, fmt.Errorf("get current score: %w", err)
	}

	// Perform upsert
	result, err := s.store.UpsertScore(ctx, store.UpsertScoreParams{
		PlayerName: playerName,
		Score:      score,
	})
	if err != nil {
		s.logger.Error().Err(err).Str("player", playerName).Int64("score", score).Msg("failed to upsert score")
		return nil, fmt.Errorf("upsert score: %w", err)
	}

	// Determine if the score was applied (improved or created)
	applied := !hadScore || result.Score > oldScore

	return &ScoreResult{
		PlayerName: result.PlayerName,
		Score:      result.Score,
		UpdatedAt:  result.UpdatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
		Applied:    applied,
	}, nil
}

// GetTopScores retrieves the top N scores with pagination
func (s *Service) GetTopScores(ctx context.Context, limit, offset int32) ([]store.Score, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("%w: limit must be positive", ErrInvalidLimit)
	}
	if offset < 0 {
		return nil, fmt.Errorf("%w: offset must be non-negative", ErrInvalidLimit)
	}

	scores, err := s.store.GetTopScores(ctx, store.GetTopScoresParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		s.logger.Error().Err(err).Int32("limit", limit).Int32("offset", offset).Msg("failed to get top scores")
		return nil, fmt.Errorf("get top scores: %w", err)
	}

	return scores, nil
}

// GetPlayerRank calculates and returns a player's rank
func (s *Service) GetPlayerRank(ctx context.Context, playerName string) (int64, *store.Score, error) {
	if err := s.validatePlayerName(playerName); err != nil {
		return 0, nil, err
	}

	// First, check if player exists and get their score
	score, err := s.store.GetPlayerScore(ctx, playerName)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil, ErrPlayerNotFound
		}
		s.logger.Error().Err(err).Str("player", playerName).Msg("failed to get player score")
		return 0, nil, fmt.Errorf("get player score: %w", err)
	}

	// Calculate rank
	rank, err := s.store.GetPlayerRank(ctx, playerName)
	if err != nil {
		s.logger.Error().Err(err).Str("player", playerName).Msg("failed to get player rank")
		return 0, nil, fmt.Errorf("get player rank: %w", err)
	}

	return int64(rank), &score, nil
}

// DeleteScore removes a player's score entry
func (s *Service) DeleteScore(ctx context.Context, playerName string) error {
	if err := s.validatePlayerName(playerName); err != nil {
		return err
	}

	if err := s.store.DeleteScore(ctx, playerName); err != nil {
		s.logger.Error().Err(err).Str("player", playerName).Msg("failed to delete score")
		return fmt.Errorf("delete score: %w", err)
	}

	s.logger.Info().Str("player", playerName).Msg("score deleted")
	return nil
}

func (s *Service) validatePlayerName(name string) error {
	if len(name) < MinPlayerNameLength || len(name) > MaxPlayerNameLength {
		return fmt.Errorf("%w: player name must be between %d and %d characters",
			ErrInvalidPlayerName, MinPlayerNameLength, MaxPlayerNameLength)
	}
	// Additional validation could be added here (e.g., character set restrictions)
	return nil
}

func (s *Service) validateScore(score int64) error {
	if score < 0 {
		return fmt.Errorf("%w: score must be non-negative", ErrInvalidScore)
	}
	return nil
}
