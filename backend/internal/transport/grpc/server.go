package grpc

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/rs/zerolog"
	pb "github.com/yourorg/leaderboard/gen/leaderboard/v1"
	"github.com/yourorg/leaderboard/internal/notify"
	"github.com/yourorg/leaderboard/internal/service"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Server implements the gRPC LeaderboardService
type Server struct {
	pb.UnimplementedLeaderboardServiceServer
	svc            *service.Service
	logger         *zerolog.Logger
	notifyListener *notify.Listener

	// Broadcast channel for real-time updates
	mu          sync.RWMutex
	subscribers map[chan *pb.LeaderboardUpdate]struct{}

	defaultLimit int32
	maxLimit     int32
}

// NewServer creates a new gRPC server
func NewServer(svc *service.Service, listener *notify.Listener, logger *zerolog.Logger, defaultLimit, maxLimit int32) *Server {
	s := &Server{
		svc:            svc,
		logger:         logger,
		notifyListener: listener,
		subscribers:    make(map[chan *pb.LeaderboardUpdate]struct{}),
		defaultLimit:   defaultLimit,
		maxLimit:       maxLimit,
	}

	// Start broadcasting notifications to subscribers
	go s.broadcastNotifications()

	return s
}

// SubmitScore implements the SubmitScore RPC
func (s *Server) SubmitScore(ctx context.Context, req *pb.SubmitScoreRequest) (*pb.SubmitScoreResponse, error) {
	if req.PlayerName == "" {
		return nil, status.Error(codes.InvalidArgument, "player_name is required")
	}
	if req.Score < 0 {
		return nil, status.Error(codes.InvalidArgument, "score must be non-negative")
	}

	result, err := s.svc.SubmitScore(ctx, req.PlayerName, req.Score)
	if err != nil {
		if errors.Is(err, service.ErrInvalidPlayerName) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		if errors.Is(err, service.ErrInvalidScore) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		s.logger.Error().Err(err).Msg("failed to submit score")
		return nil, status.Error(codes.Internal, "failed to submit score")
	}

	return &pb.SubmitScoreResponse{
		Applied: result.Applied,
		Entry: &pb.ScoreEntry{
			PlayerName: result.PlayerName,
			Score:      result.Score,
			UpdatedAt:  result.UpdatedAt,
		},
	}, nil
}

// GetTopScores implements the GetTopScores RPC
func (s *Server) GetTopScores(ctx context.Context, req *pb.GetTopScoresRequest) (*pb.GetTopScoresResponse, error) {
	limit := req.Limit
	if limit <= 0 {
		limit = s.defaultLimit
	}
	if limit > s.maxLimit {
		limit = s.maxLimit
	}

	offset := req.Offset
	if offset < 0 {
		offset = 0
	}

	scores, err := s.svc.GetTopScores(ctx, limit, offset)
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to get top scores")
		return nil, status.Error(codes.Internal, "failed to get top scores")
	}

	entries := make([]*pb.ScoreEntry, len(scores))
	for i, score := range scores {
		entries[i] = &pb.ScoreEntry{
			PlayerName: score.PlayerName,
			Score:      score.Score,
			UpdatedAt:  score.UpdatedAt.Time.Format(time.RFC3339),
		}
	}

	return &pb.GetTopScoresResponse{
		Entries: entries,
	}, nil
}

// GetPlayerRank implements the GetPlayerRank RPC
func (s *Server) GetPlayerRank(ctx context.Context, req *pb.GetPlayerRankRequest) (*pb.GetPlayerRankResponse, error) {
	if req.PlayerName == "" {
		return nil, status.Error(codes.InvalidArgument, "player_name is required")
	}

	rank, score, err := s.svc.GetPlayerRank(ctx, req.PlayerName)
	if err != nil {
		if errors.Is(err, service.ErrPlayerNotFound) {
			return &pb.GetPlayerRankResponse{
				NotFound: true,
			}, nil
		}
		if errors.Is(err, service.ErrInvalidPlayerName) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		s.logger.Error().Err(err).Msg("failed to get player rank")
		return nil, status.Error(codes.Internal, "failed to get player rank")
	}

	return &pb.GetPlayerRankResponse{
		NotFound: false,
		Rank:     rank,
		Entry: &pb.ScoreEntry{
			PlayerName: score.PlayerName,
			Score:      score.Score,
			UpdatedAt:  score.UpdatedAt.Time.Format(time.RFC3339),
		},
	}, nil
}

// StreamLeaderboard implements the StreamLeaderboard server-streaming RPC
func (s *Server) StreamLeaderboard(req *pb.SubscribeRequest, stream pb.LeaderboardService_StreamLeaderboardServer) error {
	ctx := stream.Context()

	// Determine initial limit
	limit := req.InitialLimit
	if limit <= 0 {
		limit = s.defaultLimit
	}
	if limit > s.maxLimit {
		limit = s.maxLimit
	}

	// Send initial snapshot
	scores, err := s.svc.GetTopScores(ctx, limit, 0)
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to get initial snapshot")
		return status.Error(codes.Internal, "failed to get initial snapshot")
	}

	snapshot := make([]*pb.ScoreEntry, len(scores))
	for i, score := range scores {
		snapshot[i] = &pb.ScoreEntry{
			PlayerName: score.PlayerName,
			Score:      score.Score,
			UpdatedAt:  score.UpdatedAt.Time.Format(time.RFC3339),
		}
	}

	if err := stream.Send(&pb.LeaderboardUpdate{
		Kind:     pb.LeaderboardUpdate_SNAPSHOT,
		Snapshot: snapshot,
	}); err != nil {
		s.logger.Error().Err(err).Msg("failed to send initial snapshot")
		return status.Error(codes.Internal, "failed to send snapshot")
	}

	s.logger.Info().Int32("limit", limit).Msg("client subscribed to leaderboard stream")

	// Create a subscriber channel
	updateChan := make(chan *pb.LeaderboardUpdate, 50)
	s.addSubscriber(updateChan)
	defer s.removeSubscriber(updateChan)

	// Stream updates to client
	for {
		select {
		case <-ctx.Done():
			s.logger.Info().Msg("client disconnected from stream")
			return nil
		case update := <-updateChan:
			if err := stream.Send(update); err != nil {
				s.logger.Error().Err(err).Msg("failed to send update")
				return status.Error(codes.Internal, "failed to send update")
			}
		}
	}
}

// broadcastNotifications listens for database notifications and broadcasts them to subscribers
func (s *Server) broadcastNotifications() {
	s.logger.Info().Msg("🎧 Started listening for database changes to broadcast to gRPC clients")

	for change := range s.notifyListener.Changes() {
		s.logger.Info().
			Str("player", change.PlayerName).
			Int64("score", change.Score).
			Str("op", change.Op).
			Msg("🔔 BACKEND received change notification from DB listener")

		var kind pb.LeaderboardUpdate_Kind
		switch change.Op {
		case "insert", "update":
			kind = pb.LeaderboardUpdate_UPSERT
		case "delete":
			kind = pb.LeaderboardUpdate_DELETE
		default:
			s.logger.Warn().Str("op", change.Op).Msg("⚠️  unknown notification operation")
			continue
		}

		update := &pb.LeaderboardUpdate{
			Kind: kind,
			Changed: &pb.ScoreEntry{
				PlayerName: change.PlayerName,
				Score:      change.Score,
				UpdatedAt:  time.Now().Format(time.RFC3339), // Best effort timestamp
			},
		}

		s.logger.Info().
			Str("player", change.PlayerName).
			Str("kind", kind.String()).
			Msg("📡 Broadcasting to gRPC subscribers")

		s.broadcast(update)
	}
}

// broadcast sends an update to all subscribers
func (s *Server) broadcast(update *pb.LeaderboardUpdate) {
	s.mu.RLock()
	subscriberCount := len(s.subscribers)
	s.mu.RUnlock()

	s.logger.Info().
		Int("subscriber_count", subscriberCount).
		Str("player", update.Changed.PlayerName).
		Msg("📤 Sending update to gRPC subscribers")

	s.mu.RLock()
	defer s.mu.RUnlock()

	successCount := 0
	for ch := range s.subscribers {
		select {
		case ch <- update:
			successCount++
		default:
			// Channel full, skip (backpressure handling)
			s.logger.Warn().Msg("⚠️  subscriber channel full, skipping update")
		}
	}

	s.logger.Info().
		Int("sent_to", successCount).
		Int("total_subscribers", subscriberCount).
		Msg("✅ Update broadcast complete")
}

// addSubscriber registers a new subscriber
func (s *Server) addSubscriber(ch chan *pb.LeaderboardUpdate) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.subscribers[ch] = struct{}{}
	s.logger.Debug().Int("total", len(s.subscribers)).Msg("subscriber added")
}

// removeSubscriber unregisters a subscriber
func (s *Server) removeSubscriber(ch chan *pb.LeaderboardUpdate) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.subscribers, ch)
	close(ch)
	s.logger.Debug().Int("total", len(s.subscribers)).Msg("subscriber removed")
}
