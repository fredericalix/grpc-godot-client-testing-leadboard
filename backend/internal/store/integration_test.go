// +build integration

package store_test

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/yourorg/leaderboard/internal/store"
)

func setupTestDB(t *testing.T) (*store.Store, func()) {
	ctx := context.Background()

	// Start PostgreSQL container
	postgresContainer, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:18-alpine"),
		postgres.WithDatabase("leaderboard_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		t.Fatalf("failed to start postgres container: %s", err)
	}

	// Get connection string
	connStr, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get connection string: %s", err)
	}

	// Run migrations
	if err := runMigrations(connStr); err != nil {
		postgresContainer.Terminate(ctx)
		t.Fatalf("failed to run migrations: %s", err)
	}

	// Create connection pool
	pool, err := store.NewPool(ctx, connStr)
	if err != nil {
		postgresContainer.Terminate(ctx)
		t.Fatalf("failed to create pool: %s", err)
	}

	st := store.NewStore(pool)

	cleanup := func() {
		pool.Close()
		if err := postgresContainer.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %s", err)
		}
	}

	return st, cleanup
}

func runMigrations(connStr string) error {
	// Open connection for migrations
	db, err := sql.Open("pgx", connStr)
	if err != nil {
		return err
	}
	defer db.Close()

	// Read and execute migration file
	migrationPath := filepath.Join("..", "..", "db", "migrations", "0001_init.up.sql")

	// Simple migration runner - in production, use golang-migrate
	migrations := []string{
		// Create table
		`CREATE TABLE scores (
			player_name TEXT PRIMARY KEY,
			score BIGINT NOT NULL CHECK (score >= 0),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			CONSTRAINT player_name_length CHECK (char_length(player_name) <= 20 AND char_length(player_name) > 0)
		)`,
		// Create index
		`CREATE INDEX idx_scores_leaderboard ON scores (score DESC, player_name)`,
		// Create trigger function
		`CREATE OR REPLACE FUNCTION notify_score_change()
		RETURNS TRIGGER AS $$
		DECLARE
			payload JSON;
			operation TEXT;
		BEGIN
			IF TG_OP = 'DELETE' THEN
				operation := 'delete';
				payload := json_build_object(
					'player_name', OLD.player_name,
					'score', OLD.score,
					'op', operation
				);
				PERFORM pg_notify('scores_changes', payload::text);
				RETURN OLD;
			ELSIF TG_OP = 'INSERT' THEN
				operation := 'insert';
				payload := json_build_object(
					'player_name', NEW.player_name,
					'score', NEW.score,
					'op', operation
				);
				PERFORM pg_notify('scores_changes', payload::text);
				RETURN NEW;
			ELSIF TG_OP = 'UPDATE' THEN
				IF NEW.score > OLD.score THEN
					operation := 'update';
					payload := json_build_object(
						'player_name', NEW.player_name,
						'score', NEW.score,
						'op', operation
					);
					PERFORM pg_notify('scores_changes', payload::text);
				END IF;
				RETURN NEW;
			END IF;
			RETURN NULL;
		END;
		$$ LANGUAGE plpgsql`,
		// Create trigger
		`CREATE TRIGGER scores_change_trigger
		AFTER INSERT OR UPDATE OR DELETE ON scores
		FOR EACH ROW
		EXECUTE FUNCTION notify_score_change()`,
	}

	for _, migration := range migrations {
		if _, err := db.Exec(migration); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}

	return nil
}

func TestUpsertScore(t *testing.T) {
	st, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// First insert
	result1, err := st.UpsertScore(ctx, store.UpsertScoreParams{
		PlayerName: "Alice",
		Score:      100,
	})
	if err != nil {
		t.Fatalf("first upsert failed: %s", err)
	}
	if result1.Score != 100 {
		t.Errorf("expected score 100, got %d", result1.Score)
	}

	// Update with higher score - should succeed
	result2, err := st.UpsertScore(ctx, store.UpsertScoreParams{
		PlayerName: "Alice",
		Score:      200,
	})
	if err != nil {
		t.Fatalf("second upsert failed: %s", err)
	}
	if result2.Score != 200 {
		t.Errorf("expected score 200, got %d", result2.Score)
	}

	// Update with lower score - should keep higher score
	result3, err := st.UpsertScore(ctx, store.UpsertScoreParams{
		PlayerName: "Alice",
		Score:      150,
	})
	if err != nil {
		t.Fatalf("third upsert failed: %s", err)
	}
	if result3.Score != 200 {
		t.Errorf("expected score to remain 200, got %d", result3.Score)
	}
}

func TestGetTopScores(t *testing.T) {
	st, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Insert test data
	testPlayers := []struct {
		name  string
		score int64
	}{
		{"Alice", 1000},
		{"Bob", 800},
		{"Charlie", 1200},
		{"Diana", 900},
		{"Eve", 1100},
	}

	for _, p := range testPlayers {
		_, err := st.UpsertScore(ctx, store.UpsertScoreParams{
			PlayerName: p.name,
			Score:      p.score,
		})
		if err != nil {
			t.Fatalf("failed to insert %s: %s", p.name, err)
		}
	}

	// Get top 3
	scores, err := st.GetTopScores(ctx, store.GetTopScoresParams{
		Limit:  3,
		Offset: 0,
	})
	if err != nil {
		t.Fatalf("GetTopScores failed: %s", err)
	}

	if len(scores) != 3 {
		t.Fatalf("expected 3 scores, got %d", len(scores))
	}

	// Verify order (descending)
	expectedOrder := []string{"Charlie", "Eve", "Alice"}
	for i, name := range expectedOrder {
		if scores[i].PlayerName != name {
			t.Errorf("position %d: expected %s, got %s", i, name, scores[i].PlayerName)
		}
	}
}

func TestGetPlayerRank(t *testing.T) {
	st, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Insert test data
	testPlayers := []struct {
		name  string
		score int64
	}{
		{"Alice", 1000},
		{"Bob", 800},
		{"Charlie", 1200},
	}

	for _, p := range testPlayers {
		_, err := st.UpsertScore(ctx, store.UpsertScoreParams{
			PlayerName: p.name,
			Score:      p.score,
		})
		if err != nil {
			t.Fatalf("failed to insert %s: %s", p.name, err)
		}
	}

	// Check Charlie's rank (should be 1 - highest score)
	rank, err := st.GetPlayerRank(ctx, "Charlie")
	if err != nil {
		t.Fatalf("GetPlayerRank failed: %s", err)
	}
	if rank != 1 {
		t.Errorf("expected rank 1 for Charlie, got %d", rank)
	}

	// Check Alice's rank (should be 2)
	rank, err = st.GetPlayerRank(ctx, "Alice")
	if err != nil {
		t.Fatalf("GetPlayerRank failed: %s", err)
	}
	if rank != 2 {
		t.Errorf("expected rank 2 for Alice, got %d", rank)
	}

	// Check Bob's rank (should be 3)
	rank, err = st.GetPlayerRank(ctx, "Bob")
	if err != nil {
		t.Fatalf("GetPlayerRank failed: %s", err)
	}
	if rank != 3 {
		t.Errorf("expected rank 3 for Bob, got %d", rank)
	}
}

func TestDeleteScore(t *testing.T) {
	st, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Insert a score
	_, err := st.UpsertScore(ctx, store.UpsertScoreParams{
		PlayerName: "Alice",
		Score:      100,
	})
	if err != nil {
		t.Fatalf("insert failed: %s", err)
	}

	// Verify it exists
	score, err := st.GetPlayerScore(ctx, "Alice")
	if err != nil {
		t.Fatalf("GetPlayerScore failed: %s", err)
	}
	if score.Score != 100 {
		t.Errorf("expected score 100, got %d", score.Score)
	}

	// Delete it
	err = st.DeleteScore(ctx, "Alice")
	if err != nil {
		t.Fatalf("DeleteScore failed: %s", err)
	}

	// Verify it's gone
	_, err = st.GetPlayerScore(ctx, "Alice")
	if err == nil {
		t.Error("expected error for non-existent player, got nil")
	}
}

func TestPlayerNameLengthConstraint(t *testing.T) {
	st, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Try to insert a name longer than 20 characters
	_, err := st.UpsertScore(ctx, store.UpsertScoreParams{
		PlayerName: "ThisNameIsWayTooLongAndShouldFail", // 34 characters
		Score:      100,
	})
	if err == nil {
		t.Error("expected error for name > 20 chars, got nil")
	}

	// Valid 20-character name should work
	_, err = st.UpsertScore(ctx, store.UpsertScoreParams{
		PlayerName: "12345678901234567890", // exactly 20 characters
		Score:      100,
	})
	if err != nil {
		t.Errorf("expected success for 20-char name, got error: %s", err)
	}
}
