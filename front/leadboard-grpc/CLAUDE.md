# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a **Godot 4.5 frontend client** for a real-time gRPC leaderboard system. It connects to a Go backend service to submit player scores and stream live leaderboard updates via gRPC.

**Backend Location**: `../../backend/` (Go service with PostgreSQL)

**Backend gRPC Endpoint**: `localhost:50051` (development)

## Technology Stack

- **Engine**: Godot 4.5 (GL Compatibility renderer)
- **Language**: GDScript
- **Protocol**: gRPC via Protocol Buffers
- **Addon**: Godobuf v0.6.1 (protobuf implementation for Godot)

## Project Status

This is a **minimal skeleton project**. The infrastructure is set up but no game-specific logic has been implemented yet. The main scene (`main_screen.tscn`) is an empty Control node.

## Directory Structure

```
leadboard-grpc/
├── project.godot           # Godot 4.5 project configuration
├── main_screen.tscn        # Main entry point (empty Control node)
├── icon.svg                # Project icon
├── addons/
│   └── protobuf/           # Godobuf addon (DO NOT MODIFY)
│       ├── plugin.cfg
│       ├── protobuf_ui.gd  # Editor plugin for proto compilation
│       ├── protobuf_core.gd # Serialization/deserialization
│       ├── parser.gd       # Proto file parser
│       └── test/           # Addon unit tests
└── docs/                   # Empty documentation folder
```

## Protobuf/gRPC Integration

### Using Godobuf Addon

The Godobuf addon provides an in-editor UI for compiling `.proto` files to GDScript:

1. **Access**: Bottom panel in Godot Editor → "Godobuf" tab
2. **Input**: Select `.proto` file path
3. **Output**: Choose destination for generated GDScript
4. **Compile**: Click "Compile" button

### Backend Proto Definition

Location: `../../backend/proto/leaderboard/v1/leaderboard.proto`

**Service**: `LeaderboardService`
- `SubmitScore` - Submit/update player score (unary RPC)
- `GetTopScores` - Fetch top N scores with pagination (unary RPC)
- `GetPlayerRank` - Get player's rank (unary RPC)
- `StreamLeaderboard` - Real-time leaderboard updates (server-streaming RPC)

**Key Messages**:
- `ScoreEntry` - Player name, score, timestamp
- `SubmitScoreRequest/Response` - Score submission
- `GetTopScoresRequest/Response` - Top scores query
- `GetPlayerRankRequest/Response` - Rank lookup
- `SubscribeRequest/LeaderboardUpdate` - Streaming updates

**Data Contracts**:
- Player names: 1-20 characters
- Scores: Non-negative int64
- Best score logic: Only highest score per player is kept
- Timestamps: RFC3339 format

### Regenerating Proto Bindings

When the backend's proto definition changes:

```bash
# From this directory, compile the backend's proto file:
# 1. Open Godot Editor
# 2. Go to "Godobuf" tab in bottom panel
# 3. Input: ../../backend/proto/leaderboard/v1/leaderboard.proto
# 4. Output: (choose a location, e.g., proto_generated/leaderboard.gd)
# 5. Click "Compile"
```

Alternatively, use the addon's command-line interface (see `addons/protobuf/protobuf_cmdln.gd`).

## Backend Service

### Starting the Backend

```bash
cd ../../backend

# Quick start (installs tools, starts services, runs migrations)
make quickstart

# Or step-by-step:
make install-tools    # Install buf, sqlc, migrate, staticcheck
make deps            # Download Go dependencies
make generate        # Generate protobuf + sqlc code
make compose-up      # Start PostgreSQL + backend with Docker
```

### Backend Endpoints

- **gRPC**: `localhost:50051`
- **REST Admin API**: `http://localhost:8080` (for testing)

### Testing Backend Connectivity

```bash
# List gRPC services
grpcurl -plaintext localhost:50051 list

# Submit a test score
grpcurl -plaintext -d '{"player_name": "TestPlayer", "score": 1000}' \
  localhost:50051 leaderboard.v1.LeaderboardService/SubmitScore

# Stream leaderboard updates
grpcurl -plaintext -d '{"initial_limit": 10}' \
  localhost:50051 leaderboard.v1.LeaderboardService/StreamLeaderboard
```

## Development Workflow

### Running the Frontend

1. Open project in Godot Editor
2. Press F5 or click "Run Project"
3. Currently shows an empty window (main_screen.tscn)

### Implementing Frontend Logic

**Recommended structure** (to be created):

```
leadboard-grpc/
├── scripts/
│   ├── leaderboard_client.gd    # gRPC client wrapper
│   ├── leaderboard_ui.gd        # UI controller
│   └── score_entry.gd           # Score entry component
├── scenes/
│   ├── main.tscn                # Main scene with UI
│   ├── leaderboard_view.tscn    # Leaderboard display
│   └── score_submit.tscn        # Score submission form
└── proto_generated/
    └── leaderboard.gd           # Generated from proto file
```

### Key Implementation Tasks

1. **Generate Proto Bindings**: Compile `leaderboard.proto` using Godobuf
2. **Create gRPC Client**: Wrap protobuf messages in a GDScript client class
3. **Build UI**: Design leaderboard display and score submission interface
4. **Connect Backend**: Implement gRPC calls using generated bindings
5. **Handle Streaming**: Subscribe to `StreamLeaderboard` for real-time updates
6. **Error Handling**: Handle network errors, validation failures, disconnections

### gRPC Communication Pattern

```gdscript
# Example structure (not implemented):
extends Node

var leaderboard_client = preload("res://proto_generated/leaderboard.gd").new()

func submit_score(player_name: String, score: int):
    var request = leaderboard_client.SubmitScoreRequest.new()
    request.player_name = player_name
    request.score = score

    # Call gRPC service (actual implementation depends on gRPC transport)
    var response = await call_grpc_service("SubmitScore", request)

    if response.applied:
        print("Score submitted: ", response.entry.score)

func stream_leaderboard():
    var request = leaderboard_client.SubscribeRequest.new()
    request.initial_limit = 10

    # Open streaming connection
    var stream = await call_grpc_stream("StreamLeaderboard", request)

    while true:
        var update = await stream.receive()
        match update.kind:
            leaderboard_client.LeaderboardUpdate.Kind.SNAPSHOT:
                # Initial leaderboard snapshot
                update_ui(update.snapshot)
            leaderboard_client.LeaderboardUpdate.Kind.UPSERT:
                # Player score updated
                add_or_update_entry(update.changed)
            leaderboard_client.LeaderboardUpdate.Kind.DELETE:
                # Player removed
                remove_entry(update.changed)
```

**Note**: The actual gRPC transport implementation (TCP connection, HTTP/2, etc.) is not provided by Godobuf and must be implemented separately or using a networking library.

## Backend Architecture

The backend uses:
- **PostgreSQL 18**: Primary database with LISTEN/NOTIFY for real-time updates
- **gRPC**: Primary API (port 50051)
- **REST API**: Admin endpoints (port 8080, no auth)
- **sqlc**: Type-safe SQL query generation
- **Clean Architecture**: Transport → Service → Store layers

### Real-time Updates

Backend uses PostgreSQL's LISTEN/NOTIFY:
1. Database trigger fires on score changes
2. Notification sent on `scores_changes` channel
3. Backend listener broadcasts to all streaming gRPC clients
4. Frontend receives `LeaderboardUpdate` messages

## Testing

### Backend Tests

```bash
cd ../../backend
make test                # Unit tests
make test-coverage       # Coverage report
make test-integration    # Integration tests with PostgreSQL
```

### Frontend Testing

Currently no test infrastructure. Consider adding:
- GDScript unit tests using Godot's built-in testing framework
- Integration tests with a mock backend
- UI tests for scene interactions

## Common Commands

### Backend Management

```bash
cd ../../backend

# Start/stop services
make compose-up          # Start PostgreSQL + backend
make compose-down        # Stop all services
make compose-logs        # View logs

# Development
make dev-db             # Start only PostgreSQL for local dev
make run                # Run backend locally (without Docker)

# Code generation
make proto              # Regenerate protobuf code
make generate           # Regenerate proto + sqlc

# Database
make migrate-up         # Apply migrations
make migrate-down       # Rollback last migration
make migrate-create NAME=feature  # Create new migration

# Code quality
make fmt                # Format Go code
make lint               # Run staticcheck
make check              # Run all checks (fmt, vet, lint, test)
```

### Frontend Development

```bash
# Open in Godot Editor
godot project.godot

# Run from command line
godot --path . --headless  # Headless mode

# Export project (requires export templates)
godot --export "Linux/X11" build/game.x86_64
```

## Important Notes

### DO NOT MODIFY

- `addons/protobuf/` - This is a third-party addon (Godobuf v0.6.1)
- `.godot/` - Auto-generated Godot cache directory

### Best Practices

- **Proto Changes**: Always regenerate GDScript bindings after backend proto updates
- **Scene Organization**: Keep UI scenes separate from logic scripts
- **Signals**: Use Godot signals for UI events and state changes
- **Error Handling**: Backend returns standard gRPC status codes:
  - `InvalidArgument`: Validation failure (name too long, negative score)
  - `NotFound`: Player not found (GetPlayerRank)
  - `Internal`: Server error
- **Connection Management**: Implement reconnection logic for network failures
- **Backpressure**: Handle high-frequency streaming updates efficiently

## Architecture Decisions

### Why Godot?

This is a game engine frontend for a leaderboard system, likely intended for integration into a game or interactive application.

### Why gRPC?

- Efficient binary protocol (Protocol Buffers)
- Bidirectional streaming support
- Type-safe communication
- Easier to work with than raw WebSockets for structured data

### Frontend-Backend Separation

The backend is a standalone Go service that can serve multiple frontends (Godot, web, mobile). The proto definition is the source of truth for the API contract.

## Troubleshooting

### "Cannot connect to backend"

1. Ensure backend is running: `cd ../../backend && make compose-up`
2. Check gRPC endpoint: `grpcurl -plaintext localhost:50051 list`
3. Verify network connectivity from Godot client

### "Proto compilation failed"

1. Ensure proto file path is correct
2. Check Godot console for Godobuf error messages
3. Verify proto file syntax using backend: `cd ../../backend && make proto-lint`

### "Backend returns InvalidArgument"

Check validation constraints:
- Player name: 1-20 characters
- Score: Non-negative int64

## Resources

- **Godot Documentation**: https://docs.godotengine.org/en/stable/
- **Godobuf GitHub**: https://github.com/oniksan/godobuf
- **Protocol Buffers**: https://protobuf.dev/
- **Backend README**: `../../backend/README.md`
