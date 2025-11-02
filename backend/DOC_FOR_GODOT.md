# Godot Frontend Integration Guide

This document provides comprehensive guidance for building a Godot Engine frontend that connects to the Leaderboard Backend via gRPC.

## Table of Contents

1. [Overview](#overview)
2. [Backend Services](#backend-services)
3. [gRPC in Godot](#grpc-in-godot)
4. [API Reference](#api-reference)
5. [Implementation Guide](#implementation-guide)
6. [Real-Time Streaming](#real-time-streaming)
7. [UI/UX Recommendations](#uiux-recommendations)
8. [Testing](#testing)
9. [Common Patterns](#common-patterns)
10. [Troubleshooting](#troubleshooting)

## Overview

### Backend Architecture

The backend provides a **real-time videogame leaderboard** with:
- **gRPC API** on `localhost:50051` (primary interface)
- **REST API** on `localhost:8080` (admin/testing only)
- **PostgreSQL LISTEN/NOTIFY** for real-time updates
- **Server-streaming** for live leaderboard updates

### Key Concepts

1. **Best Score Only**: Each player keeps only their highest score
2. **Real-Time Updates**: All connected clients receive instant notifications when scores change
3. **Automatic Ranking**: Backend calculates ranks (1 = best)
4. **Validation**: Player names (1-20 chars), scores (non-negative)

### Data Flow

```
Godot Client → gRPC (50051) → Backend → PostgreSQL
                                ↓
                          LISTEN/NOTIFY
                                ↓
                    Broadcast to all clients
                                ↓
                          Godot Receives Update
```

## Backend Services

### Available Endpoints

The backend runs two services:

#### gRPC Service (Port 50051)
- **SubmitScore**: Submit/update a player's score
- **GetTopScores**: Retrieve top N players
- **GetPlayerRank**: Get specific player's rank
- **StreamLeaderboard**: Real-time leaderboard updates (server-streaming)

#### REST Service (Port 8080)
- For admin/testing only
- Swagger UI: http://localhost:8080/swagger/index.html
- **Not recommended for Godot** - use gRPC instead

### Why gRPC for Godot?

- **Efficient**: Binary protocol (faster than JSON/REST)
- **Streaming**: Native support for real-time updates
- **Type-Safe**: Strongly typed messages
- **Bi-directional**: Can handle complex communication patterns

## gRPC in Godot

### Available gRPC Plugins for Godot

#### Option 1: godot-grpc (Recommended)
- **Repository**: https://github.com/jgillich/godot-grpc
- **Godot Version**: 4.x
- **Features**: Full gRPC support including streaming
- **Installation**: Via GDExtension

#### Option 2: GDScript HTTP/2 (Fallback)
- Use Godot's built-in HTTP client with manual protobuf encoding
- More complex but doesn't require plugins
- Consider using REST API instead if this route is needed

#### Option 3: Proxy Pattern
- Create a local WebSocket proxy that translates to gRPC
- Godot connects via WebSocket
- Good for web exports

### Recommended Approach

**Use godot-grpc with Godot 4.x** for native gRPC support:

```gdscript
# Example connection setup
var client = GrpcClient.new()
client.connect_to_server("localhost:50051")
```

### Protobuf Definition Location

The protobuf definition is available at:
```
proto/leaderboard/v1/leaderboard.proto
```

You'll need to generate Godot-compatible code from this file using the gRPC plugin's code generator.

## API Reference

### Package and Service

```protobuf
package leaderboard.v1;

service LeaderboardService {
  rpc SubmitScore(SubmitScoreRequest) returns (SubmitScoreResponse);
  rpc GetTopScores(GetTopScoresRequest) returns (GetTopScoresResponse);
  rpc GetPlayerRank(GetPlayerRankRequest) returns (GetPlayerRankResponse);
  rpc StreamLeaderboard(SubscribeRequest) returns (stream LeaderboardUpdate);
}
```

### Common Message: ScoreEntry

Used across all responses to represent a player's score.

```protobuf
message ScoreEntry {
  string player_name = 1;  // 1-20 characters
  int64  score = 2;        // non-negative integer
  string updated_at = 3;   // RFC3339 timestamp (e.g., "2025-11-02T10:30:00Z")
}
```

**GDScript representation:**
```gdscript
class ScoreEntry:
    var player_name: String
    var score: int
    var updated_at: String
```

---

### RPC 1: SubmitScore (Unary)

Submit or update a player's score. Only applies if the new score is higher than the current best.

#### Request

```protobuf
message SubmitScoreRequest {
  string player_name = 1;  // 1-20 characters, required
  int64  score = 2;        // non-negative, required
}
```

**GDScript example:**
```gdscript
var request = SubmitScoreRequest.new()
request.player_name = "Alice"
request.score = 1000

var response = await client.submit_score(request)
```

#### Response

```protobuf
message SubmitScoreResponse {
  bool       applied = 1;  // true if score improved or was created
  ScoreEntry entry = 2;    // current best score for this player
}
```

**Response fields:**
- `applied = true`: Score was a new best (either new player or improved score)
- `applied = false`: Score was lower than current best, not saved
- `entry`: Always contains the player's current best score

**Example response:**
```json
{
  "applied": true,
  "entry": {
    "player_name": "Alice",
    "score": 1000,
    "updated_at": "2025-11-02T10:30:00Z"
  }
}
```

#### Error Codes

| Code | Description | Reason |
|------|-------------|--------|
| `INVALID_ARGUMENT` | Validation failed | Name empty/too long, score negative |
| `INTERNAL` | Server error | Database issue |

**GDScript error handling:**
```gdscript
var response = await client.submit_score(request)
if response.is_error():
    if response.error_code == grpc.StatusCode.INVALID_ARGUMENT:
        print("Validation error: ", response.error_message)
    else:
        print("Server error: ", response.error_message)
else:
    if response.applied:
        print("New best score!")
    else:
        print("Score not high enough")
```

---

### RPC 2: GetTopScores (Unary)

Retrieve the top N players, sorted by score (descending).

#### Request

```protobuf
message GetTopScoresRequest {
  int32 limit = 1;   // number of entries to return (default: 10, max: 100)
  int32 offset = 2;  // pagination offset (default: 0)
}
```

**GDScript example:**
```gdscript
var request = GetTopScoresRequest.new()
request.limit = 10
request.offset = 0

var response = await client.get_top_scores(request)
```

**Pagination example:**
```gdscript
# Get entries 11-20 (second page)
request.limit = 10
request.offset = 10
```

#### Response

```protobuf
message GetTopScoresResponse {
  repeated ScoreEntry entries = 1;
}
```

**GDScript handling:**
```gdscript
var response = await client.get_top_scores(request)
for entry in response.entries:
    print("%s: %d" % [entry.player_name, entry.score])
```

**Example response:**
```json
{
  "entries": [
    {
      "player_name": "Charlie",
      "score": 10000,
      "updated_at": "2025-11-02T10:35:00Z"
    },
    {
      "player_name": "Alice",
      "score": 5000,
      "updated_at": "2025-11-02T10:30:00Z"
    }
  ]
}
```

#### Sorting Rules

- **Primary**: Score (descending - highest first)
- **Tie-breaker**: Player name (lexicographical ascending)

Example order:
1. Charlie: 10000
2. Alice: 5000
3. Bob: 5000 (same score as Alice, sorted alphabetically)

---

### RPC 3: GetPlayerRank (Unary)

Get a specific player's rank and score.

#### Request

```protobuf
message GetPlayerRankRequest {
  string player_name = 1;  // required
}
```

**GDScript example:**
```gdscript
var request = GetPlayerRankRequest.new()
request.player_name = "Alice"

var response = await client.get_player_rank(request)
```

#### Response

```protobuf
message GetPlayerRankResponse {
  bool       not_found = 1;  // true if player doesn't exist
  int64      rank = 2;       // 1-based rank (1 = best)
  ScoreEntry entry = 3;      // player's score entry
}
```

**GDScript handling:**
```gdscript
var response = await client.get_player_rank(request)
if response.not_found:
    print("Player not found")
else:
    print("Rank: #%d" % response.rank)
    print("Score: %d" % response.entry.score)
```

**Example response (player found):**
```json
{
  "not_found": false,
  "rank": 2,
  "entry": {
    "player_name": "Alice",
    "score": 5000,
    "updated_at": "2025-11-02T10:30:00Z"
  }
}
```

**Example response (player not found):**
```json
{
  "not_found": true
}
```

**Rank calculation:**
- Rank 1 = highest score
- Ties receive the same rank
- Example: If 3 players have score 1000, they all get rank 1

---

### RPC 4: StreamLeaderboard (Server-Streaming)

Real-time leaderboard updates. This is the **most important RPC** for creating a live leaderboard experience.

#### Request

```protobuf
message SubscribeRequest {
  int32 initial_limit = 1;  // top N to include in initial snapshot (default: 10)
}
```

**GDScript example:**
```gdscript
var request = SubscribeRequest.new()
request.initial_limit = 10

var stream = client.stream_leaderboard(request)
```

#### Response Stream

```protobuf
message LeaderboardUpdate {
  enum Kind {
    KIND_UNSPECIFIED = 0;
    SNAPSHOT = 1;  // initial full leaderboard
    UPSERT   = 2;  // player score created/updated
    DELETE   = 3;  // player removed (admin action)
  }

  Kind                 kind = 1;
  repeated ScoreEntry  snapshot = 2;  // populated when kind == SNAPSHOT
  ScoreEntry           changed = 3;   // populated when kind == UPSERT or DELETE
}
```

#### Stream Behavior

**Step 1: Initial Snapshot**
- Immediately after subscribing, you receive ONE message with `kind = SNAPSHOT`
- Contains the current top N players
- Use this to populate your initial UI

**Step 2: Live Updates**
- After the snapshot, you receive `UPSERT` or `DELETE` messages
- Each message represents a single change
- Updates arrive in real-time (typically <100ms from database change)

**Step 3: Stream Lifetime**
- Stream stays open until client disconnects or server shuts down
- Automatically receives all leaderboard changes
- No polling needed - push-based updates

#### Update Types

##### SNAPSHOT (Initial State)

```json
{
  "kind": "SNAPSHOT",
  "snapshot": [
    {"player_name": "Charlie", "score": 10000, "updated_at": "..."},
    {"player_name": "Alice", "score": 5000, "updated_at": "..."},
    {"player_name": "Bob", "score": 3000, "updated_at": "..."}
  ]
}
```

**When to expect:**
- Immediately after calling `StreamLeaderboard`
- Exactly once per subscription

##### UPSERT (New or Updated Score)

```json
{
  "kind": "UPSERT",
  "changed": {
    "player_name": "Dave",
    "score": 7500,
    "updated_at": "2025-11-02T10:40:00Z"
  }
}
```

**When to expect:**
- Player submits a new best score via `SubmitScore`
- Admin updates a score via direct database modification

**Client action:**
- If player doesn't exist in your list: Insert
- If player exists: Update their score and re-sort

##### DELETE (Player Removed)

```json
{
  "kind": "DELETE",
  "changed": {
    "player_name": "Bob",
    "score": 3000,
    "updated_at": "2025-11-02T10:38:00Z"
  }
}
```

**When to expect:**
- Admin deletes a player via REST API or database

**Client action:**
- Remove player from your leaderboard display

#### GDScript Implementation Example

```gdscript
extends Node

var grpc_client
var leaderboard: Array[ScoreEntry] = []

func _ready():
    grpc_client = GrpcClient.new()
    grpc_client.connect_to_server("localhost:50051")
    subscribe_to_leaderboard()

func subscribe_to_leaderboard():
    var request = SubscribeRequest.new()
    request.initial_limit = 10

    var stream = grpc_client.stream_leaderboard(request)

    # Process stream messages
    while true:
        var update = await stream.receive()

        if update.is_error():
            print("Stream error: ", update.error_message)
            break

        match update.kind:
            LeaderboardUpdate.Kind.SNAPSHOT:
                handle_snapshot(update.snapshot)
            LeaderboardUpdate.Kind.UPSERT:
                handle_upsert(update.changed)
            LeaderboardUpdate.Kind.DELETE:
                handle_delete(update.changed)

func handle_snapshot(entries: Array):
    leaderboard = entries.duplicate()
    update_ui()
    print("Initial leaderboard loaded: %d players" % leaderboard.size())

func handle_upsert(entry: ScoreEntry):
    # Find existing player
    var index = -1
    for i in range(leaderboard.size()):
        if leaderboard[i].player_name == entry.player_name:
            index = i
            break

    if index >= 0:
        # Update existing
        leaderboard[index] = entry
        print("Updated: %s -> %d" % [entry.player_name, entry.score])
    else:
        # Insert new
        leaderboard.append(entry)
        print("New player: %s with %d" % [entry.player_name, entry.score])

    # Re-sort leaderboard
    leaderboard.sort_custom(func(a, b):
        if a.score != b.score:
            return a.score > b.score  # Descending by score
        return a.player_name < b.player_name  # Ascending by name
    )

    update_ui()

func handle_delete(entry: ScoreEntry):
    # Remove player
    leaderboard = leaderboard.filter(func(e):
        return e.player_name != entry.player_name
    )
    print("Removed: %s" % entry.player_name)
    update_ui()

func update_ui():
    # Update your UI here
    # Example: Update a list of labels
    for i in range(min(10, leaderboard.size())):
        var entry = leaderboard[i]
        var label = get_node("Leaderboard/Rank%d" % (i + 1))
        label.text = "%d. %s: %d" % [i + 1, entry.player_name, entry.score]
```

---

## Implementation Guide

### Recommended Architecture

```
godot_project/
├── scenes/
│   ├── main_menu.tscn
│   ├── game.tscn
│   └── leaderboard.tscn
├── scripts/
│   ├── grpc_client.gd          # gRPC connection manager
│   ├── leaderboard_service.gd  # Leaderboard business logic
│   └── leaderboard_ui.gd       # UI controller
└── proto/
    └── leaderboard/
        └── v1/
            └── leaderboard.proto  # Copy from backend
```

### Core Components

#### 1. GrpcClient (Singleton/Autoload)

Manages the gRPC connection and provides a simple API.

```gdscript
# scripts/grpc_client.gd
extends Node

const SERVER_ADDRESS = "localhost:50051"

var client: GrpcClient
var connected: bool = false

signal connection_established
signal connection_lost

func _ready():
    connect_to_server()

func connect_to_server():
    client = GrpcClient.new()
    var result = client.connect_to_server(SERVER_ADDRESS)

    if result.is_ok():
        connected = true
        connection_established.emit()
        print("Connected to leaderboard server")
    else:
        connected = false
        print("Failed to connect: ", result.error_message)
        # Retry logic
        await get_tree().create_timer(5.0).timeout
        connect_to_server()

func submit_score(player_name: String, score: int):
    if not connected:
        return {"error": "Not connected"}

    var request = SubmitScoreRequest.new()
    request.player_name = player_name
    request.score = score

    return await client.submit_score(request)

func get_top_scores(limit: int = 10, offset: int = 0):
    if not connected:
        return {"error": "Not connected"}

    var request = GetTopScoresRequest.new()
    request.limit = limit
    request.offset = offset

    return await client.get_top_scores(request)

func get_player_rank(player_name: String):
    if not connected:
        return {"error": "Not connected"}

    var request = GetPlayerRankRequest.new()
    request.player_name = player_name

    return await client.get_player_rank(request)

func subscribe_to_leaderboard(limit: int = 10):
    if not connected:
        return null

    var request = SubscribeRequest.new()
    request.initial_limit = limit

    return client.stream_leaderboard(request)
```

#### 2. LeaderboardService

Business logic for managing leaderboard state.

```gdscript
# scripts/leaderboard_service.gd
extends Node

var leaderboard: Array[Dictionary] = []
var streaming: bool = false

signal leaderboard_updated(entries: Array)
signal player_score_changed(player_name: String, score: int, rank: int)

func start_streaming():
    if streaming:
        return

    streaming = true
    var stream = GrpcClient.subscribe_to_leaderboard(10)

    if stream == null:
        print("Failed to start streaming")
        return

    _process_stream(stream)

func _process_stream(stream):
    while streaming:
        var update = await stream.receive()

        if update.is_error():
            print("Stream error: ", update.error_message)
            streaming = false
            # Attempt reconnect
            await get_tree().create_timer(3.0).timeout
            start_streaming()
            break

        match update.kind:
            0:  # SNAPSHOT
                _handle_snapshot(update.snapshot)
            1:  # UPSERT
                _handle_upsert(update.changed)
            2:  # DELETE
                _handle_delete(update.changed)

func _handle_snapshot(entries: Array):
    leaderboard.clear()
    for entry in entries:
        leaderboard.append({
            "player_name": entry.player_name,
            "score": entry.score,
            "updated_at": entry.updated_at
        })
    leaderboard_updated.emit(leaderboard)

func _handle_upsert(entry):
    var player_name = entry.player_name
    var score = entry.score

    # Find and update or insert
    var found = false
    for i in range(leaderboard.size()):
        if leaderboard[i].player_name == player_name:
            leaderboard[i].score = score
            leaderboard[i].updated_at = entry.updated_at
            found = true
            break

    if not found:
        leaderboard.append({
            "player_name": player_name,
            "score": score,
            "updated_at": entry.updated_at
        })

    # Re-sort
    _sort_leaderboard()

    # Calculate rank
    var rank = _get_rank(player_name)
    player_score_changed.emit(player_name, score, rank)
    leaderboard_updated.emit(leaderboard)

func _handle_delete(entry):
    leaderboard = leaderboard.filter(func(e):
        return e.player_name != entry.player_name
    )
    leaderboard_updated.emit(leaderboard)

func _sort_leaderboard():
    leaderboard.sort_custom(func(a, b):
        if a.score != b.score:
            return a.score > b.score
        return a.player_name < b.player_name
    )

func _get_rank(player_name: String) -> int:
    for i in range(leaderboard.size()):
        if leaderboard[i].player_name == player_name:
            return i + 1
    return -1

func submit_score(player_name: String, score: int):
    var response = await GrpcClient.submit_score(player_name, score)

    if response.is_error():
        print("Error submitting score: ", response.error_message)
        return false

    if response.applied:
        print("New best score for %s: %d" % [player_name, score])
    else:
        print("Score not high enough for %s" % player_name)

    return response.applied
```

#### 3. LeaderboardUI

UI controller for displaying the leaderboard.

```gdscript
# scripts/leaderboard_ui.gd
extends Control

@onready var leaderboard_list = $VBoxContainer/LeaderboardList
@onready var player_rank_label = $PlayerRank

var player_name: String = ""

func _ready():
    LeaderboardService.leaderboard_updated.connect(_on_leaderboard_updated)
    LeaderboardService.player_score_changed.connect(_on_player_score_changed)
    LeaderboardService.start_streaming()

func _on_leaderboard_updated(entries: Array):
    # Clear existing entries
    for child in leaderboard_list.get_children():
        child.queue_free()

    # Create new entries
    for i in range(entries.size()):
        var entry = entries[i]
        var label = Label.new()
        label.text = "%d. %s: %,d" % [i + 1, entry.player_name, entry.score]

        # Highlight current player
        if entry.player_name == player_name:
            label.add_theme_color_override("font_color", Color.YELLOW)

        leaderboard_list.add_child(label)

func _on_player_score_changed(p_name: String, score: int, rank: int):
    if p_name == player_name:
        player_rank_label.text = "Your Rank: #%d - Score: %,d" % [rank, score]

        # Play animation or sound effect
        _animate_rank_change(rank)

func _animate_rank_change(rank: int):
    # Animate the player's rank label
    var tween = create_tween()
    tween.tween_property(player_rank_label, "scale", Vector2(1.2, 1.2), 0.2)
    tween.tween_property(player_rank_label, "scale", Vector2(1.0, 1.0), 0.2)
```

---

## Real-Time Streaming

### Stream Lifecycle

```
1. Client calls StreamLeaderboard
   ↓
2. Server sends SNAPSHOT immediately
   ↓
3. Client renders initial UI
   ↓
4. Stream stays open
   ↓
5. Server sends UPSERT/DELETE as they occur
   ↓
6. Client updates UI in real-time
   ↓
7. Stream closes on disconnect
```

### Handling Reconnection

Always implement reconnection logic for robust streaming:

```gdscript
var reconnect_attempts = 0
const MAX_RECONNECT_ATTEMPTS = 5
const RECONNECT_DELAY = 3.0

func _process_stream(stream):
    while streaming:
        var update = await stream.receive()

        if update.is_error():
            print("Stream error: ", update.error_message)
            reconnect_attempts += 1

            if reconnect_attempts < MAX_RECONNECT_ATTEMPTS:
                print("Reconnecting in %d seconds..." % RECONNECT_DELAY)
                await get_tree().create_timer(RECONNECT_DELAY).timeout
                start_streaming()
            else:
                print("Max reconnect attempts reached")
                streaming = false
            break

        # Reset reconnect counter on successful message
        reconnect_attempts = 0

        # Process update...
```

### Performance Optimization

**Batch UI Updates:**
```gdscript
var update_queue: Array = []
var update_timer: Timer

func _ready():
    update_timer = Timer.new()
    update_timer.wait_time = 0.1  # Update UI every 100ms
    update_timer.timeout.connect(_flush_updates)
    add_child(update_timer)
    update_timer.start()

func queue_update(entry):
    update_queue.append(entry)

func _flush_updates():
    if update_queue.is_empty():
        return

    for entry in update_queue:
        _apply_update_to_ui(entry)

    update_queue.clear()
```

---

## UI/UX Recommendations

### Leaderboard Display

**Essential Information:**
- Rank (1, 2, 3, ...)
- Player name
- Score (formatted with commas: 1,000,000)
- Optional: Last update timestamp

**Visual Hierarchy:**
```
┌─────────────────────────────────┐
│      LEADERBOARD                │
├─────────────────────────────────┤
│  🥇 1. Charlie      10,000      │
│  🥈 2. Alice         5,000      │
│  🥉 3. Bob           3,000      │
│     4. Dave          2,500      │
│     5. Eve           2,000      │
│                                 │
│  Your Rank: #12 - Score: 850   │
└─────────────────────────────────┘
```

### Real-Time Feedback

**Score Update Animation:**
```gdscript
func animate_score_update(player_name: String):
    var entry_node = find_entry_node(player_name)

    # Flash animation
    var tween = create_tween()
    tween.tween_property(entry_node, "modulate", Color.YELLOW, 0.3)
    tween.tween_property(entry_node, "modulate", Color.WHITE, 0.3)

    # Sound effect
    $AudioStreamPlayer.play()
```

**New High Score Celebration:**
```gdscript
func celebrate_high_score(player_name: String, rank: int):
    if rank == 1:
        # Show "NEW #1!" banner
        $HighScoreBanner.show()
        $ParticleEffect.emitting = true
        $VictorySound.play()
```

### Loading States

**Initial Connection:**
```gdscript
func show_loading_state():
    $LoadingSpinner.show()
    $LeaderboardList.hide()
    $StatusLabel.text = "Connecting to server..."

func show_connected_state():
    $LoadingSpinner.hide()
    $LeaderboardList.show()
    $StatusLabel.text = ""
```

---

## Testing

### Testing with grpcurl

Before implementing in Godot, test the backend:

```bash
# Test SubmitScore
grpcurl -plaintext -d '{
  "player_name": "TestPlayer",
  "score": 1000
}' localhost:50051 leaderboard.v1.LeaderboardService/SubmitScore

# Test GetTopScores
grpcurl -plaintext -d '{
  "limit": 5
}' localhost:50051 leaderboard.v1.LeaderboardService/GetTopScores

# Test StreamLeaderboard
grpcurl -plaintext -d '{
  "initial_limit": 10
}' localhost:50051 leaderboard.v1.LeaderboardService/StreamLeaderboard
```

### Testing Real-Time Updates

**Terminal 1 - Subscribe:**
```bash
grpcurl -plaintext -d '{"initial_limit": 5}' \
  localhost:50051 leaderboard.v1.LeaderboardService/StreamLeaderboard
```

**Terminal 2 - Submit scores:**
```bash
grpcurl -plaintext -d '{"player_name": "Alice", "score": 1000}' \
  localhost:50051 leaderboard.v1.LeaderboardService/SubmitScore
```

Watch Terminal 1 receive the update in real-time!

### Unit Testing in Godot

```gdscript
# test_leaderboard_service.gd
extends GutTest

func test_leaderboard_sorting():
    var service = LeaderboardService.new()

    # Add entries in random order
    service._handle_upsert({"player_name": "Bob", "score": 500})
    service._handle_upsert({"player_name": "Alice", "score": 1000})
    service._handle_upsert({"player_name": "Charlie", "score": 750})

    # Verify sorting
    assert_eq(service.leaderboard[0].player_name, "Alice")
    assert_eq(service.leaderboard[1].player_name, "Charlie")
    assert_eq(service.leaderboard[2].player_name, "Bob")

func test_rank_calculation():
    var service = LeaderboardService.new()

    service._handle_upsert({"player_name": "Alice", "score": 1000})
    service._handle_upsert({"player_name": "Bob", "score": 500})

    assert_eq(service._get_rank("Alice"), 1)
    assert_eq(service._get_rank("Bob"), 2)
```

---

## Common Patterns

### Pattern 1: Submit Score After Game

```gdscript
# In your game scene
extends Node2D

var player_name: String = "Player1"
var score: int = 0

func _on_game_over():
    # Submit final score
    var success = await LeaderboardService.submit_score(player_name, score)

    if success:
        # Show leaderboard with player's new rank
        get_tree().change_scene_to_file("res://scenes/leaderboard.tscn")
    else:
        # Show error or retry
        $ErrorDialog.show()
```

### Pattern 2: Live Leaderboard During Gameplay

```gdscript
# In your game scene
extends Node2D

@onready var mini_leaderboard = $HUD/MiniLeaderboard

func _ready():
    # Start streaming for live updates
    LeaderboardService.leaderboard_updated.connect(_update_mini_leaderboard)
    LeaderboardService.start_streaming()

func _update_mini_leaderboard(entries: Array):
    # Show top 3
    for i in range(min(3, entries.size())):
        var label = mini_leaderboard.get_node("Rank%d" % (i + 1))
        label.text = "%s: %d" % [entries[i].player_name, entries[i].score]
```

### Pattern 3: Player Profile/Stats

```gdscript
# Player profile scene
extends Control

@export var player_name: String = ""

func _ready():
    load_player_stats()

func load_player_stats():
    var response = await GrpcClient.get_player_rank(player_name)

    if response.not_found:
        $RankLabel.text = "Unranked"
        $ScoreLabel.text = "No scores yet"
    else:
        $RankLabel.text = "Rank: #%d" % response.rank
        $ScoreLabel.text = "Best Score: %,d" % response.entry.score
        $LastPlayedLabel.text = "Last played: %s" % response.entry.updated_at
```

### Pattern 4: Pagination (Infinite Scroll)

```gdscript
extends ScrollContainer

var current_offset = 0
const PAGE_SIZE = 20
var loading = false

func _on_scroll_reached_bottom():
    if loading:
        return

    load_next_page()

func load_next_page():
    loading = true
    var response = await GrpcClient.get_top_scores(PAGE_SIZE, current_offset)

    for entry in response.entries:
        add_leaderboard_entry(entry)

    current_offset += PAGE_SIZE
    loading = false
```

---

## Troubleshooting

### Issue 1: Connection Refused

**Symptom:** `Failed to connect to localhost:50051`

**Solutions:**
1. Verify backend is running: `docker compose ps`
2. Check port is exposed: `docker compose logs app | grep 50051`
3. Test with grpcurl: `grpcurl -plaintext localhost:50051 list`
4. Check firewall settings

### Issue 2: No Stream Updates

**Symptom:** Initial snapshot works, but no updates arrive

**Solutions:**
1. Verify trigger is installed:
   ```bash
   docker compose exec postgres psql -U leaderboard -d leaderboard -c "\d+ scores"
   ```
2. Check migration version:
   ```bash
   make migrate-version  # Should be 2
   ```
3. Monitor backend logs:
   ```bash
   docker compose logs -f app | grep -E "(📨|📡)"
   ```
4. Ensure stream connection is still alive (check for timeout errors)

### Issue 3: Invalid Argument Errors

**Symptom:** `INVALID_ARGUMENT` errors when submitting scores

**Solutions:**
1. Validate player name: 1-20 characters, non-empty
   ```gdscript
   func validate_player_name(name: String) -> bool:
       return name.length() >= 1 and name.length() <= 20
   ```
2. Validate score: non-negative
   ```gdscript
   func validate_score(score: int) -> bool:
       return score >= 0
   ```

### Issue 4: Leaderboard Not Sorted

**Symptom:** Entries appear in wrong order

**Solution:**
```gdscript
func _sort_leaderboard():
    leaderboard.sort_custom(func(a, b):
        # First by score (descending)
        if a.score != b.score:
            return a.score > b.score  # Note: > for descending
        # Then by name (ascending)
        return a.player_name < b.player_name
    )
```

### Issue 5: Memory Leaks with Streaming

**Symptom:** Memory usage grows over time

**Solution:** Properly clean up streams:
```gdscript
var current_stream = null

func start_streaming():
    # Cancel old stream
    if current_stream:
        current_stream.cancel()

    current_stream = GrpcClient.subscribe_to_leaderboard(10)
    _process_stream(current_stream)

func _exit_tree():
    if current_stream:
        current_stream.cancel()
```

---

## Advanced Topics

### Handling Network Interruptions

```gdscript
extends Node

var connection_status = ConnectionStatus.CONNECTED

enum ConnectionStatus {
    CONNECTED,
    RECONNECTING,
    DISCONNECTED
}

signal connection_status_changed(status: ConnectionStatus)

func monitor_connection():
    while true:
        await get_tree().create_timer(5.0).timeout

        if not GrpcClient.connected:
            if connection_status == ConnectionStatus.CONNECTED:
                connection_status = ConnectionStatus.RECONNECTING
                connection_status_changed.emit(connection_status)
                _attempt_reconnect()

func _attempt_reconnect():
    while connection_status == ConnectionStatus.RECONNECTING:
        var success = await GrpcClient.connect_to_server()

        if success:
            connection_status = ConnectionStatus.CONNECTED
            connection_status_changed.emit(connection_status)
            # Restart streaming
            LeaderboardService.start_streaming()
            break

        await get_tree().create_timer(3.0).timeout
```

### Offline Mode Support

```gdscript
var offline_scores: Array = []

func submit_score_with_offline_support(player_name: String, score: int):
    if GrpcClient.connected:
        return await GrpcClient.submit_score(player_name, score)
    else:
        # Queue for later
        offline_scores.append({
            "player_name": player_name,
            "score": score,
            "timestamp": Time.get_unix_time_from_system()
        })

        # Save to disk
        _save_offline_scores()

        return {"offline": true}

func sync_offline_scores():
    if not GrpcClient.connected:
        return

    for entry in offline_scores:
        await GrpcClient.submit_score(entry.player_name, entry.score)

    offline_scores.clear()
    _save_offline_scores()
```

### Analytics and Metrics

```gdscript
func track_leaderboard_view():
    var metrics = {
        "event": "leaderboard_viewed",
        "timestamp": Time.get_unix_time_from_system(),
        "entries_count": LeaderboardService.leaderboard.size()
    }
    # Send to analytics service

func track_score_submission(player_name: String, score: int, applied: bool):
    var metrics = {
        "event": "score_submitted",
        "player_name": player_name,
        "score": score,
        "applied": applied,
        "timestamp": Time.get_unix_time_from_system()
    }
    # Send to analytics service
```

---

## Performance Tips

1. **Limit Leaderboard Size**: Don't render 1000+ entries at once
   - Use pagination or virtual scrolling
   - Only show top 10-50 in main view

2. **Debounce UI Updates**: Don't update on every tiny change
   - Batch updates over 100ms window
   - Use timers to coalesce rapid changes

3. **Cache Player Ranks**: Don't recalculate on every frame
   - Store rank with each entry
   - Only recalculate when leaderboard changes

4. **Use Object Pooling**: For list items
   - Reuse label nodes instead of creating/destroying
   - Improves performance for long lists

5. **Async Loading**: Don't block the main thread
   - Use `await` for all gRPC calls
   - Show loading indicators

---

## Security Notes

1. **No Authentication**: Current backend has no auth
   - Anyone can submit scores for any player name
   - Suitable for single-player or trusted environments
   - For production, implement player verification

2. **Input Validation**: Always validate on client
   ```gdscript
   func validate_input(name: String, score: int) -> Dictionary:
       if name.is_empty():
           return {"valid": false, "error": "Name is required"}
       if name.length() > 20:
           return {"valid": false, "error": "Name too long (max 20)"}
       if score < 0:
           return {"valid": false, "error": "Score must be non-negative"}
       return {"valid": true}
   ```

3. **Rate Limiting**: Consider client-side rate limiting
   ```gdscript
   var last_submit_time = 0.0
   const SUBMIT_COOLDOWN = 1.0  # seconds

   func can_submit_score() -> bool:
       var now = Time.get_ticks_msec() / 1000.0
       if now - last_submit_time < SUBMIT_COOLDOWN:
           return false
       last_submit_time = now
       return true
   ```

---

## Complete Example Project Structure

```
godot_leaderboard/
├── project.godot
├── scenes/
│   ├── main.tscn
│   ├── game.tscn
│   ├── leaderboard.tscn
│   ├── submit_score.tscn
│   └── components/
│       ├── leaderboard_entry.tscn
│       └── loading_spinner.tscn
├── scripts/
│   ├── autoload/
│   │   ├── grpc_client.gd
│   │   ├── leaderboard_service.gd
│   │   └── connection_manager.gd
│   ├── ui/
│   │   ├── leaderboard_ui.gd
│   │   ├── submit_score_ui.gd
│   │   └── player_rank_widget.gd
│   └── utils/
│       ├── validators.gd
│       └── formatters.gd
├── proto/
│   └── leaderboard/
│       └── v1/
│           ├── leaderboard.proto
│           └── leaderboard_pb.gd  # Generated
├── assets/
│   ├── fonts/
│   ├── icons/
│   └── sounds/
│       ├── score_submit.wav
│       ├── rank_up.wav
│       └── high_score.wav
└── tests/
    ├── test_leaderboard_service.gd
    └── test_validators.gd
```

---

## Quick Reference

### gRPC Connection
```gdscript
var client = GrpcClient.new()
client.connect_to_server("localhost:50051")
```

### Submit Score
```gdscript
var response = await client.submit_score(player_name, score)
if response.applied:
    print("New best!")
```

### Get Top Scores
```gdscript
var response = await client.get_top_scores(10, 0)
for entry in response.entries:
    print("%s: %d" % [entry.player_name, entry.score])
```

### Stream Leaderboard
```gdscript
var stream = client.stream_leaderboard(10)
while true:
    var update = await stream.receive()
    match update.kind:
        SNAPSHOT: handle_snapshot(update.snapshot)
        UPSERT: handle_upsert(update.changed)
        DELETE: handle_delete(update.changed)
```

---

## Backend Endpoints Summary

| Endpoint | Type | Purpose | Response Time |
|----------|------|---------|---------------|
| SubmitScore | Unary | Submit player score | ~10ms |
| GetTopScores | Unary | Fetch leaderboard | ~20ms |
| GetPlayerRank | Unary | Get player stats | ~15ms |
| StreamLeaderboard | Streaming | Real-time updates | Instant push |

---

## Support and Resources

### Backend Documentation
- Main README: `README.md`
- Swagger UI: http://localhost:8080/swagger/index.html
- Protobuf: `proto/leaderboard/v1/leaderboard.proto`

### Testing the Backend
```bash
# Check if backend is running
docker compose ps

# View backend logs
docker compose logs -f app

# Test with grpcurl
grpcurl -plaintext localhost:50051 list
```

### Debugging Tips

1. **Enable Verbose Logging**:
   ```gdscript
   GrpcClient.enable_debug_logging()
   ```

2. **Monitor Backend Logs**:
   ```bash
   docker compose logs -f app | grep -E "(📨|🔔|📡)"
   ```

3. **Test Outside Godot First**: Use grpcurl to verify backend works

4. **Check Network Tab**: Monitor gRPC traffic in debugger

---

## Next Steps

1. **Setup gRPC in Godot**
   - Install godot-grpc plugin
   - Copy protobuf definition
   - Generate Godot bindings

2. **Implement Core Services**
   - Create GrpcClient autoload
   - Create LeaderboardService
   - Test connection

3. **Build UI**
   - Create leaderboard scene
   - Implement real-time updates
   - Add animations

4. **Test Integration**
   - Test submit score flow
   - Test real-time streaming
   - Handle error cases

5. **Polish**
   - Add sound effects
   - Improve animations
   - Optimize performance

Good luck building your Godot leaderboard frontend! 🎮🏆
