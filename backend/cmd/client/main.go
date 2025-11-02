package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	pb "github.com/yourorg/leaderboard/gen/leaderboard/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	// Command-line flags
	addr := flag.String("addr", "localhost:50051", "gRPC server address")
	cmd := flag.String("cmd", "stream", "command to execute: stream, submit, top, rank")
	player := flag.String("player", "", "player name (for submit and rank)")
	score := flag.Int64("score", 0, "score value (for submit)")
	limit := flag.Int("limit", 10, "limit for top scores or stream")
	flag.Parse()

	if err := run(*addr, *cmd, *player, *score, int32(*limit)); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(addr, cmd, player string, score int64, limit int32) error {
	// Create gRPC connection
	ctx := context.Background()
	conn, err := grpc.DialContext(
		ctx,
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithTimeout(5*time.Second),
	)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer conn.Close()

	client := pb.NewLeaderboardServiceClient(conn)

	switch cmd {
	case "stream":
		return streamLeaderboard(ctx, client, limit)
	case "submit":
		return submitScore(ctx, client, player, score)
	case "top":
		return getTopScores(ctx, client, limit)
	case "rank":
		return getPlayerRank(ctx, client, player)
	default:
		return fmt.Errorf("unknown command: %s", cmd)
	}
}

// streamLeaderboard demonstrates the server-streaming RPC
func streamLeaderboard(ctx context.Context, client pb.LeaderboardServiceClient, limit int32) error {
	fmt.Printf("Subscribing to leaderboard stream (limit=%d)...\n", limit)

	stream, err := client.StreamLeaderboard(ctx, &pb.SubscribeRequest{
		InitialLimit: limit,
	})
	if err != nil {
		return fmt.Errorf("stream leaderboard: %w", err)
	}

	for {
		update, err := stream.Recv()
		if err == io.EOF {
			fmt.Println("Stream closed by server")
			return nil
		}
		if err != nil {
			return fmt.Errorf("receive: %w", err)
		}

		switch update.Kind {
		case pb.LeaderboardUpdate_SNAPSHOT:
			fmt.Println("\n=== SNAPSHOT ===")
			for i, entry := range update.Snapshot {
				fmt.Printf("%d. %s: %d (updated: %s)\n",
					i+1, entry.PlayerName, entry.Score, entry.UpdatedAt)
			}
			fmt.Println("================\n")
			fmt.Println("Waiting for updates... (Press Ctrl+C to stop)")

		case pb.LeaderboardUpdate_UPSERT:
			fmt.Printf("🔔 UPDATE: %s scored %d (updated: %s)\n",
				update.Changed.PlayerName, update.Changed.Score, update.Changed.UpdatedAt)

		case pb.LeaderboardUpdate_DELETE:
			fmt.Printf("🗑️  DELETE: %s removed from leaderboard\n",
				update.Changed.PlayerName)

		default:
			fmt.Printf("Unknown update kind: %v\n", update.Kind)
		}
	}
}

// submitScore demonstrates the unary RPC for submitting scores
func submitScore(ctx context.Context, client pb.LeaderboardServiceClient, player string, score int64) error {
	if player == "" {
		return fmt.Errorf("player name is required")
	}

	fmt.Printf("Submitting score: %s = %d\n", player, score)

	resp, err := client.SubmitScore(ctx, &pb.SubmitScoreRequest{
		PlayerName: player,
		Score:      score,
	})
	if err != nil {
		return fmt.Errorf("submit score: %w", err)
	}

	if resp.Applied {
		fmt.Printf("✅ Score applied! New best: %d (updated: %s)\n",
			resp.Entry.Score, resp.Entry.UpdatedAt)
	} else {
		fmt.Printf("ℹ️  Score not applied. Current best: %d (updated: %s)\n",
			resp.Entry.Score, resp.Entry.UpdatedAt)
	}

	return nil
}

// getTopScores demonstrates retrieving top scores
func getTopScores(ctx context.Context, client pb.LeaderboardServiceClient, limit int32) error {
	fmt.Printf("Getting top %d scores...\n", limit)

	resp, err := client.GetTopScores(ctx, &pb.GetTopScoresRequest{
		Limit:  limit,
		Offset: 0,
	})
	if err != nil {
		return fmt.Errorf("get top scores: %w", err)
	}

	fmt.Println("\n=== TOP SCORES ===")
	for i, entry := range resp.Entries {
		fmt.Printf("%d. %s: %d (updated: %s)\n",
			i+1, entry.PlayerName, entry.Score, entry.UpdatedAt)
	}
	fmt.Println("==================\n")

	return nil
}

// getPlayerRank demonstrates getting a player's rank
func getPlayerRank(ctx context.Context, client pb.LeaderboardServiceClient, player string) error {
	if player == "" {
		return fmt.Errorf("player name is required")
	}

	fmt.Printf("Getting rank for: %s\n", player)

	resp, err := client.GetPlayerRank(ctx, &pb.GetPlayerRankRequest{
		PlayerName: player,
	})
	if err != nil {
		return fmt.Errorf("get player rank: %w", err)
	}

	if resp.NotFound {
		fmt.Printf("❌ Player '%s' not found in leaderboard\n", player)
		return nil
	}

	fmt.Printf("🏆 Rank: #%d\n", resp.Rank)
	fmt.Printf("   Score: %d\n", resp.Entry.Score)
	fmt.Printf("   Updated: %s\n", resp.Entry.UpdatedAt)

	return nil
}
