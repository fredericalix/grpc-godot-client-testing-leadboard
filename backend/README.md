# Videogame Leaderboard Backend

A production-grade, real-time leaderboard backend service built with Go, PostgreSQL, and gRPC.

## Features

- **gRPC API**: Primary interface for frontend applications
- **Real-time Updates**: Server-streaming leaderboard updates via PostgreSQL LISTEN/NOTIFY
- **Best Score Logic**: Automatically keeps only the best (highest) score per player
- **REST Admin API**: Simple endpoints for score management with Swagger/OpenAPI docs
- **Type-Safe SQL**: Using sqlc for compile-time SQL validation
- **Database Migrations**: Schema versioning with golang-migrate
- **Clean Architecture**: Clear separation of concerns (transport, service, store)
- **Robust Error Handling**: Comprehensive context usage and graceful shutdown
- **Production Ready**: Structured logging with emoji markers, connection pooling, health checks
- **Observable**: Detailed logging of the entire LISTEN/NOTIFY pipeline for debugging

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                         Clients                              │
│                                                              │
│  ┌─────────────────┐              ┌──────────────────┐      │
│  │   Frontend      │              │  Admin/Ops       │      │
│  │   (gRPC)        │              │  (REST/HTTP)     │      │
│  └────────┬────────┘              └─────────┬────────┘      │
└───────────┼──────────────────────────────────┼──────────────┘
            │                                  │
            │ gRPC (50051)                     │ HTTP (8080)
            │                                  │
┌───────────▼──────────────────────────────────▼──────────────┐
│                    Backend Application                       │
│                                                              │
│  ┌──────────────┐                      ┌─────────────────┐  │
│  │ gRPC Server  │                      │  REST Server    │  │
│  │ (Streaming)  │                      │  (Echo)         │  │
│  └──────┬───────┘                      └────────┬────────┘  │
│         │                                       │           │
│         └───────────────┬───────────────────────┘           │
│                         │                                   │
│                  ┌──────▼────────┐                          │
│                  │ Service Layer │                          │
│                  │ (Business     │                          │
│                  │  Logic)       │                          │
│                  └──────┬────────┘                          │
│                         │                                   │
│                  ┌──────▼────────┐                          │
│                  │  Store Layer  │                          │
│                  │  (sqlc)       │                          │
│                  └──────┬────────┘                          │
│                         │                                   │
│         ┌───────────────┼────────────────┐                  │
│         │               │                │                  │
│  ┌──────▼──────┐ ┌──────▼──────┐ ┌───────▼──────┐          │
│  │   Queries   │ │  LISTEN/    │ │  Connection  │          │
│  │   (CRUD)    │ │  NOTIFY     │ │  Pool        │          │
│  └─────────────┘ └─────────────┘ └──────────────┘          │
└──────────────────────────┬──────────────────────────────────┘
                           │
                           │ pgx/v5
                           │
┌──────────────────────────▼──────────────────────────────────┐
│                   PostgreSQL 18                              │
│                                                              │
│  ┌──────────────┐  ┌────────────────┐  ┌─────────────────┐ │
│  │ scores table │  │  Triggers &    │  │  NOTIFY         │ │
│  │              │  │  Constraints   │  │  Channel        │ │
│  └──────────────┘  └────────────────┘  └─────────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

## Quick Start

### Prerequisites

- Docker & Docker Compose
- Go 1.23+ (for local development)
- Make

### Complete Setup from Scratch

```bash
# Install required tools (buf, sqlc, migrate, staticcheck)
make install-tools

# Download dependencies
make deps

# Generate code (protobuf + sqlc)
make generate

# Start all services with Docker Compose
make compose-up

# Run database migrations (IMPORTANT!)
make migrate-up

# The system is now ready!
# gRPC: localhost:50051
# REST:  http://localhost:8080
# Swagger UI: http://localhost:8080/swagger/index.html
```

**Note:** The `quickstart` target handles all of the above automatically (including migrations).

### Development Setup (Local)

```bash
# Start only PostgreSQL
make dev-db

# Run database migrations
make migrate-up

# Generate code
make generate

# Build and run locally
make run
```

## Usage Examples

### gRPC API (grpcurl)

#### List Available Services

```bash
grpcurl -plaintext localhost:50051 list
```

#### Submit a Score

```bash
grpcurl -plaintext -d '{
  "player_name": "Alice",
  "score": 1000
}' localhost:50051 leaderboard.v1.LeaderboardService/SubmitScore
```

Response:
```json
{
  "applied": true,
  "entry": {
    "playerName": "Alice",
    "score": "1000",
    "updatedAt": "2025-01-15T10:30:00Z"
  }
}
```

#### Get Top Scores

```bash
grpcurl -plaintext -d '{
  "limit": 10,
  "offset": 0
}' localhost:50051 leaderboard.v1.LeaderboardService/GetTopScores
```

#### Get Player Rank

```bash
grpcurl -plaintext -d '{
  "player_name": "Alice"
}' localhost:50051 leaderboard.v1.LeaderboardService/GetPlayerRank
```

#### Stream Real-time Updates

```bash
grpcurl -plaintext -d '{
  "initial_limit": 10
}' localhost:50051 leaderboard.v1.LeaderboardService/StreamLeaderboard
```

This will:
1. Send an initial snapshot of the top 10 scores
2. Stream live updates as players submit new scores
3. Continue until cancelled (Ctrl+C)

### gRPC Client CLI

Build and use the provided client:

```bash
# Build the client
make client

# Stream leaderboard updates
./bin/client -cmd stream -limit 10

# Submit a score
./bin/client -cmd submit -player "Bob" -score 1500

# Get top scores
./bin/client -cmd top -limit 5

# Get player rank
./bin/client -cmd rank -player "Alice"
```

### REST API (Admin)

#### Create or Update Score (POST)

```bash
curl -X POST http://localhost:8080/scores \
  -H "Content-Type: application/json" \
  -d '{
    "player_name": "Charlie",
    "score": 2000
  }'
```

Response:
```json
{
  "player_name": "Charlie",
  "score": 2000,
  "updated_at": "2025-01-15T10:35:00Z",
  "applied": true
}
```

#### Update Score (PUT)

```bash
curl -X PUT http://localhost:8080/scores/Charlie \
  -H "Content-Type: application/json" \
  -d '{
    "score": 2500
  }'
```

#### Delete Score (DELETE)

```bash
curl -X DELETE http://localhost:8080/scores/Charlie
```

#### Health Check

```bash
curl http://localhost:8080/health
```

#### OpenAPI/Swagger Documentation

Interactive API documentation is available via Swagger UI:

**URL**: http://localhost:8080/swagger/index.html

The Swagger UI provides:
- Interactive API testing
- Complete endpoint documentation
- Request/response schemas
- Example payloads
- Try-it-out functionality

**OpenAPI Spec Files**:
- JSON: http://localhost:8080/swagger/doc.json
- YAML: Available in `docs/swagger.yaml`

To regenerate Swagger documentation after modifying REST endpoints:
```bash
make swagger
```

## Database Schema

### Table: `scores`

```sql
CREATE TABLE scores (
    player_name TEXT PRIMARY KEY,
    score BIGINT NOT NULL CHECK (score >= 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT player_name_length CHECK (char_length(player_name) <= 20 AND char_length(player_name) > 0)
);

-- Index for efficient leaderboard queries
CREATE INDEX idx_scores_leaderboard ON scores (score DESC, player_name);
```

### Constraints

- **player_name**: 1-20 characters, primary key
- **score**: Non-negative BIGINT
- **Best score logic**: Enforced via SQL upsert with `GREATEST()`

### Migration History

**Migration 0001** (`init`):
- Creates `scores` table with constraints
- Creates `idx_scores_leaderboard` index
- Creates `notify_score_change()` trigger function
- Creates `scores_change_trigger` trigger

**Migration 0002** (`update_notify_trigger`):
- Updates trigger to notify on **any score change** (not just increases)
- Enables notifications for score decreases and manual corrections
- Critical for real-time updates on direct database modifications

## LISTEN/NOTIFY Flow

### Channel: `scores_changes`

The system uses PostgreSQL's LISTEN/NOTIFY for real-time updates:

1. **Trigger**: A database trigger fires on INSERT, UPDATE, or DELETE
2. **Condition**: Notifies on **any score change** (increases, decreases, or deletions)
3. **Payload**: JSON with format:
   ```json
   {
     "player_name": "Alice",
     "score": 1000,
     "op": "insert"
   }
   ```
4. **Operations**: `insert`, `update`, or `delete`

### Backend Listener

- Automatically reconnects on connection loss (exponential backoff)
- Parses JSON payloads
- Broadcasts to all active gRPC streaming clients
- Buffers updates to handle backpressure
- Comprehensive logging with emoji markers for easy debugging:
  - 📨 DB notification received
  - ✅ Change parsed successfully
  - 📤 Change forwarded to subscribers
  - 🔔 Backend received notification
  - 📡 Broadcasting to clients
  - ✅ Broadcast complete

### Streaming Behavior

When a client calls `StreamLeaderboard`:
1. Receives immediate snapshot of top N scores
2. Receives incremental updates as they occur
3. Updates include:
   - `SNAPSHOT`: Initial state
   - `UPSERT`: New or improved score
   - `DELETE`: Admin removed a player

## Makefile Targets

### Code Generation

```bash
make proto        # Generate protobuf code with Buf
make sqlc         # Generate type-safe Go from SQL
make swagger      # Generate OpenAPI/Swagger docs
make generate     # Generate all (proto + sqlc + swagger)
```

### Database Migrations

```bash
make migrate-up         # Apply all migrations
make migrate-down       # Rollback last migration
make migrate-version    # Show current version
make migrate-create NAME=add_feature  # Create new migration
```

### Build & Run

```bash
make build        # Build server binary
make run          # Run server locally
make client       # Build gRPC client
make server       # Build and run server
```

### Testing

```bash
make test                # Run all tests
make test-coverage       # Generate coverage report
make test-integration    # Run integration tests only
```

### Docker Compose

```bash
make compose-up          # Start all services
make compose-down        # Stop all services
make compose-logs        # View logs
make compose-build       # Rebuild images
make compose-restart     # Restart all services
```

### Development

```bash
make dev-db       # Start only PostgreSQL
make dev          # Setup local dev environment
```

### Code Quality

```bash
make fmt          # Format code
make vet          # Run go vet
make lint         # Run staticcheck
make check        # Run all checks (fmt, vet, lint, test)
```

### Utilities

```bash
make clean            # Clean build artifacts
make install-tools    # Install dev tools
make deps             # Download dependencies
```

## Configuration

All configuration via environment variables (12-factor):

| Variable       | Default                          | Description                   |
|----------------|----------------------------------|-------------------------------|
| DATABASE_URL   | postgres://leaderboard:...       | PostgreSQL connection string  |
| GRPC_PORT      | 50051                            | gRPC server port              |
| REST_PORT      | 8080                             | REST API port                 |
| LOG_LEVEL      | info                             | Log level (debug/info/warn/error) |
| DEFAULT_LIMIT  | 10                               | Default leaderboard limit     |
| MAX_LIMIT      | 100                              | Maximum leaderboard limit     |

## Project Structure

```
.
├── proto/                      # Protobuf definitions
│   └── leaderboard/v1/
│       └── leaderboard.proto
├── gen/                        # Generated code (proto)
├── db/
│   ├── migrations/             # SQL migrations
│   │   ├── 0001_init.up.sql
│   │   ├── 0001_init.down.sql
│   │   ├── 0002_update_notify_trigger.up.sql
│   │   └── 0002_update_notify_trigger.down.sql
│   └── sql/
│       ├── queries.sql         # sqlc queries
│       └── sqlc.yaml           # sqlc config
├── internal/
│   ├── config/                 # Configuration
│   ├── log/                    # Logging (zerolog)
│   ├── store/                  # Database layer (sqlc)
│   ├── service/                # Business logic
│   ├── transport/
│   │   ├── grpc/              # gRPC handlers
│   │   └── rest/              # REST handlers (Echo)
│   └── notify/                # LISTEN/NOTIFY subscriber
├── cmd/
│   ├── server/                # Main server
│   └── client/                # gRPC client demo
├── scripts/
│   └── dev-wait-for-db.sh     # Helper scripts
├── Dockerfile
├── docker-compose.yml
├── Makefile
├── buf.yaml
├── buf.gen.yaml
├── go.mod
├── go.sum
├── LICENSE
└── README.md
```

## For Frontend Developers

### Connecting via gRPC

**Server Address**: `localhost:50051` (development)

**Protocol**: gRPC (unary + server-streaming)

**Package**: `leaderboard.v1`

### Service: LeaderboardService

#### 1. SubmitScore (Unary RPC)

Submit or update a player's score. Only applies if it's higher than the current best.

**Request**:
```protobuf
message SubmitScoreRequest {
  string player_name = 1;  // 1-20 characters
  int64  score = 2;        // non-negative
}
```

**Response**:
```protobuf
message SubmitScoreResponse {
  bool   applied = 1;      // true if score improved/created
  ScoreEntry entry = 2;    // current best score
}
```

#### 2. GetTopScores (Unary RPC)

Retrieve top N scores with pagination.

**Request**:
```protobuf
message GetTopScoresRequest {
  int32  limit = 1;   // default 10, max 100
  int32  offset = 2;  // pagination offset
}
```

**Response**:
```protobuf
message GetTopScoresResponse {
  repeated ScoreEntry entries = 1;
}
```

#### 3. GetPlayerRank (Unary RPC)

Get a player's rank (1 = best).

**Request**:
```protobuf
message GetPlayerRankRequest {
  string player_name = 1;
}
```

**Response**:
```protobuf
message GetPlayerRankResponse {
  bool   not_found = 1;
  int64  rank = 2;         // 1-based rank if found
  ScoreEntry entry = 3;
}
```

#### 4. StreamLeaderboard (Server-Streaming RPC)

Real-time leaderboard updates.

**Request**:
```protobuf
message SubscribeRequest {
  int32 initial_limit = 1;  // default 10
}
```

**Stream Response**:
```protobuf
message LeaderboardUpdate {
  enum Kind {
    KIND_UNSPECIFIED = 0;
    SNAPSHOT = 1;  // initial full list
    UPSERT   = 2;  // player score improved
    DELETE   = 3;  // player removed
  }
  Kind kind = 1;
  repeated ScoreEntry snapshot = 2;  // when kind == SNAPSHOT
  ScoreEntry changed = 3;            // when kind == UPSERT or DELETE
}
```

**Flow**:
1. Client calls `StreamLeaderboard`
2. Server immediately sends `SNAPSHOT` with top N scores
3. Server streams `UPSERT` messages when scores change
4. Server streams `DELETE` messages when admins remove players
5. Stream remains open until client disconnects

### Common Message

```protobuf
message ScoreEntry {
  string player_name = 1;
  int64  score = 2;
  string updated_at = 3;  // RFC3339 timestamp
}
```

### Error Handling

- **InvalidArgument**: Validation failure (name too long, negative score)
- **NotFound**: Player not found (GetPlayerRank only)
- **Internal**: Server error

### Data Contracts

- Player names: 1-20 characters
- Scores: Non-negative int64
- Ties: Allowed, broken by lexicographical order of player_name
- Best score: Only highest score per player is kept
- Timestamps: RFC3339 format

## Testing

### Unit Tests

```bash
make test
```

Tests cover:
- Input validation
- Business logic (best score rules)
- Error handling

### Integration Tests

Uses testcontainers-go with PostgreSQL 18:

```bash
make test-integration
```

Tests cover:
- Upsert behavior (keep best score)
- Top scores query and ordering
- Player rank calculation
- Delete operations
- Database constraints (name length)
- NOTIFY trigger (indirectly)

## Performance Notes

### Queries

- **UpsertScore**: O(log n) - primary key lookup
- **GetTopScores**: O(limit + offset) - index scan on `(score DESC, player_name)`
- **GetPlayerRank**: O(n) worst case - count of better scores
- **DeleteScore**: O(log n) - primary key lookup

### Optimizations

- Connection pooling (5-25 connections)
- Indexed leaderboard queries
- Prepared statements via sqlc
- Buffered notification channels
- Graceful backpressure handling

## Troubleshooting

### Verifying LISTEN/NOTIFY

To verify that real-time notifications are working:

#### 1. Check Migration Version

```bash
make migrate-version
```

Expected output: `2` (both migrations applied)

#### 2. Monitor Backend Logs

```bash
docker compose logs -f app | grep -E "(📨|🔔|📡)"
```

You should see emoji markers when changes occur.

#### 3. Test with Direct Database Modification

Open two terminals:

**Terminal 1 - Stream leaderboard:**
```bash
go run cmd/client/main.go stream 10
```

**Terminal 2 - Modify database directly:**
```bash
docker compose exec postgres psql -U leaderboard -d leaderboard \
  -c "UPDATE scores SET score = 9999 WHERE player_name = 'Alice';"
```

**Expected result:** Terminal 1 should immediately show:
```
🔔 UPDATE: Alice scored 9999 (updated: ...)
```

#### 4. Common Issues

**No notifications on direct DB updates:**
- Ensure migration version is 2: `make migrate-version`
- If version is 1, run: `make migrate-up`
- Check trigger exists:
  ```bash
  docker compose exec postgres psql -U leaderboard -d leaderboard \
    -c "\d+ scores"
  ```

**Client doesn't receive updates:**
- Check backend logs show `subscriber_count=1` when client connects
- Ensure backend shows `📡 Broadcasting to gRPC subscribers`
- Verify client is still connected (hasn't timed out)

**Container rebuild loses data:**
- Use `docker compose down` (without `-v`) to preserve volumes
- After `docker compose down -v`, run `make migrate-up` to recreate schema

## License

BSD 3-Clause License. See [LICENSE](LICENSE) for details.

## Contributing

This is a production-grade proof-of-concept. For contributions:
1. Run `make check` before committing
2. Add tests for new features
3. Update documentation
4. Follow existing code style

## Support

For issues or questions, please open an issue on GitHub.
