package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
)

const (
	// Channel name for PostgreSQL NOTIFY
	ScoresChangesChannel = "scores_changes"
)

// ScoreChange represents a notification payload from PostgreSQL
type ScoreChange struct {
	PlayerName string `json:"player_name"`
	Score      int64  `json:"score"`
	Op         string `json:"op"` // "insert", "update", or "delete"
}

// Listener handles PostgreSQL LISTEN/NOTIFY for score changes
type Listener struct {
	pool       *pgxpool.Pool
	logger     *zerolog.Logger
	changeChan chan ScoreChange
	errChan    chan error
}

// NewListener creates a new LISTEN/NOTIFY listener
func NewListener(pool *pgxpool.Pool, logger *zerolog.Logger) *Listener {
	return &Listener{
		pool:       pool,
		logger:     logger,
		changeChan: make(chan ScoreChange, 100), // Buffered channel
		errChan:    make(chan error, 10),
	}
}

// Start begins listening for notifications with automatic reconnection
func (l *Listener) Start(ctx context.Context) {
	go l.listen(ctx)
}

// Changes returns a channel that receives score change notifications
func (l *Listener) Changes() <-chan ScoreChange {
	return l.changeChan
}

// Errors returns a channel that receives listener errors
func (l *Listener) Errors() <-chan error {
	return l.errChan
}

func (l *Listener) listen(ctx context.Context) {
	backoff := time.Second
	maxBackoff := time.Minute

	for {
		select {
		case <-ctx.Done():
			l.logger.Info().Msg("listener shutting down")
			close(l.changeChan)
			close(l.errChan)
			return
		default:
		}

		// Acquire a connection from the pool
		conn, err := l.pool.Acquire(ctx)
		if err != nil {
			l.logger.Error().Err(err).Msg("failed to acquire connection for LISTEN")
			l.sendError(fmt.Errorf("acquire connection: %w", err))
			time.Sleep(backoff)
			backoff = min(backoff*2, maxBackoff)
			continue
		}

		// Issue LISTEN command
		_, err = conn.Exec(ctx, fmt.Sprintf("LISTEN %s", ScoresChangesChannel))
		if err != nil {
			l.logger.Error().Err(err).Msg("failed to LISTEN")
			conn.Release()
			l.sendError(fmt.Errorf("LISTEN command: %w", err))
			time.Sleep(backoff)
			backoff = min(backoff*2, maxBackoff)
			continue
		}

		l.logger.Info().Str("channel", ScoresChangesChannel).Msg("listening for notifications")
		backoff = time.Second // Reset backoff on successful connection

		// Wait for notifications
		for {
			notification, err := conn.Conn().WaitForNotification(ctx)
			if err != nil {
				l.logger.Error().Err(err).Msg("notification error, will reconnect")
				conn.Release()
				l.sendError(fmt.Errorf("wait for notification: %w", err))
				break
			}

			l.logger.Info().
				Str("channel", notification.Channel).
				Str("payload", notification.Payload).
				Msg("📨 DB NOTIFICATION received from PostgreSQL")

			// Parse the notification payload
			var change ScoreChange
			if err := json.Unmarshal([]byte(notification.Payload), &change); err != nil {
				l.logger.Error().
					Err(err).
					Str("payload", notification.Payload).
					Msg("❌ failed to parse notification payload")
				continue
			}

			l.logger.Info().
				Str("player", change.PlayerName).
				Int64("score", change.Score).
				Str("op", change.Op).
				Msg("✅ DB CHANGE detected - parsed successfully")

			// Send to channel (non-blocking with timeout)
			select {
			case l.changeChan <- change:
				l.logger.Info().
					Str("player", change.PlayerName).
					Int64("score", change.Score).
					Msg("📤 Change forwarded to subscribers")
			case <-time.After(time.Second):
				l.logger.Warn().Msg("⚠️  change channel full, dropping notification")
			case <-ctx.Done():
				conn.Release()
				return
			}
		}
	}
}

func (l *Listener) sendError(err error) {
	select {
	case l.errChan <- err:
	default:
		// Error channel full, log and drop
		l.logger.Warn().Err(err).Msg("error channel full, dropping error")
	}
}

func min(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
