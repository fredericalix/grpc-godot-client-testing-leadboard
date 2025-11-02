# Godot Leaderboard Display

A real-time leaderboard display application built with **Godot 4.5** that connects to a gRPC backend to show the top 5 players with live updates.

![Godot Version](https://img.shields.io/badge/Godot-4.5-blue)
![License](https://img.shields.io/badge/license-MIT-green)

## Features

- **Real-time Updates**: Displays top 5 players with live score updates via gRPC streaming
- **Auto-reconnection**: Automatically reconnects with exponential backoff if connection drops
- **Clean UI**: Dark-themed interface with gold/silver/bronze medals for top 3
- **Score Formatting**: Thousands separators for better readability (e.g., "99,999")
- **Connection Status**: Visual feedback for connection state (Connected/Disconnected/Reconnecting)

## Screenshots

```
┌─────────────────────────────────────┐
│        LEADERBOARD - TOP 5          │
│  Status: Connected                  │
├─────────────────────────────────────┤
│  #1  TestPlayer      99,999         │
│  #2  Alice           10,500         │
│  #3  Bob              8,200         │
│  #4  Diana            6,400         │
│  #5  Eve              5,900         │
└─────────────────────────────────────┘
```

## Prerequisites

- **Godot 4.5+** ([Download](https://godotengine.org/download))
- **Backend Server** running at `localhost:50051`
  - See `../../backend/` for the gRPC leaderboard service

## Project Structure

```
leadboard-grpc/
├── addons/
│   ├── godot_grpc/          # gRPC GDExtension for Godot
│   │   ├── bin/             # Platform-specific binaries (.dylib, .so, .dll)
│   │   └── godot_grpc.gdextension
│   └── protobuf/            # Godobuf - Protocol Buffer implementation
│       └── protobuf_*.gd
├── proto/
│   └── leaderboard/
│       └── v1/
│           ├── leaderboard.proto           # Original proto file (from backend)
│           ├── leaderboard_messages.proto  # Messages-only (for godobuf)
│           └── leaderboard_pb.gd          # Generated GDScript classes
├── main_screen.tscn         # Main UI scene
├── main.gd                  # gRPC client implementation
├── project.godot            # Godot project configuration
├── CLAUDE.md                # Documentation for Claude Code
└── README.md                # This file
```

## Installation & Setup

### 1. Clone/Copy the Project

This project is part of the `grpc-testing` monorepo. The frontend is located at:
```
grpc-testing/front/leadboard-grpc/
```

### 2. Install Dependencies

The required addons are already included:
- ✅ **godot_grpc** (v1.0.0) - gRPC client GDExtension
- ✅ **Godobuf** (v0.6.1) - Protocol buffer serialization

No additional installation needed!

### 3. Start the Backend

The app requires the gRPC backend to be running:

```bash
cd ../../backend
make quickstart
```

Verify the backend is running:
```bash
curl http://localhost:8080/health
# Expected: {"status":"ok"}
```

### 4. Open in Godot

1. Launch **Godot 4.5**
2. Click **Import** and select this directory
3. The project should load with no errors

## Running the App

### Method 1: From Godot Editor

1. Open the project in Godot
2. Press **F5** (or click the Play button)
3. The app will launch and connect to `localhost:50051`

### Method 2: From Command Line

```bash
# macOS
/Applications/Godot.app/Contents/MacOS/Godot --path . &

# Linux
godot --path . &

# Windows
Godot.exe --path .
```

## Testing Real-time Updates

With the app running, submit new scores using the backend client:

```bash
cd ../../backend

# Submit a high score
./bin/client -cmd submit -player "TestPlayer" -score 99999

# Submit another score
./bin/client -cmd submit -player "Alice" -score 50000

# Delete a player (via REST API)
curl -X DELETE http://localhost:8080/scores/TestPlayer
```

The Godot app will **immediately update** to reflect the changes!

## Testing Reconnection

1. **Stop the backend:**
   ```bash
   cd ../../backend
   make compose-down
   ```

2. **Observe:** App shows "Disconnected" and "Reconnecting in Xs..."

3. **Restart the backend:**
   ```bash
   make compose-up
   ```

4. **Observe:** App reconnects and displays the leaderboard again

## How It Works

### gRPC Streaming

The app uses **server-streaming RPC** to receive real-time updates:

1. **Connection**: App connects to `localhost:50051` (insecure gRPC)
2. **Subscribe**: Calls `StreamLeaderboard` with `initial_limit = 5`
3. **Initial Snapshot**: Server sends top 5 scores
4. **Live Updates**: Server streams `UPSERT` or `DELETE` messages when scores change
5. **UI Updates**: App immediately reflects changes in the display

### Message Types

The app handles three types of `LeaderboardUpdate` messages:

| Type | Kind | Description |
|------|------|-------------|
| **SNAPSHOT** | 1 | Initial full list of top N scores |
| **UPSERT** | 2 | A player's score was added or updated |
| **DELETE** | 3 | A player was removed from the leaderboard |

### Reconnection Logic

If the connection drops:
- **Initial retry**: 2 seconds
- **Exponential backoff**: 2s → 4s → 8s → 16s → 30s (max)
- **Reset on success**: Backoff resets to 2s after successful connection

## Technical Details

### Addons Used

#### godot_grpc (GDExtension)
- **Type**: Native C++ extension via GDExtension
- **Purpose**: Provides `GrpcClient` class for gRPC communication
- **Platform Support**: macOS (ARM64/x86_64), Linux (x86_64), Windows (x86_64)
- **Repository**: https://github.com/fredericalix/godot_grpc

**Key Classes:**
```gdscript
GrpcClient.new()                    # Create gRPC client
client.connect(endpoint, options)   # Connect to server
client.server_stream_start(...)     # Start server-streaming RPC
client.server_stream_cancel(id)     # Cancel stream
```

**Signals:**
```gdscript
client.message.connect(callback)    # Emitted when message received
client.finished.connect(callback)   # Emitted when stream ends
client.error.connect(callback)      # Emitted on error
```

#### Godobuf (Protobuf)
- **Type**: Pure GDScript protobuf implementation
- **Purpose**: Encode/decode protobuf messages
- **Repository**: https://github.com/oniksan/godobuf

**Usage:**
```gdscript
# Compile .proto to GDScript
godot --headless --script res://addons/protobuf/protobuf_cmdln.gd \
  --input=proto/file.proto --output=proto/file.gd

# Use generated classes
var request = SubscribeRequest.new()
request.set_initial_limit(5)
var bytes = request.to_bytes()
```

### Proto File Compilation

The proto file is compiled to GDScript using godobuf:

```bash
/Applications/Godot.app/Contents/MacOS/Godot --headless \
  --script res://addons/protobuf/protobuf_cmdln.gd \
  --input=proto/leaderboard/v1/leaderboard_messages.proto \
  --output=proto/leaderboard/v1/leaderboard_pb.gd
```

**Note**: Godobuf doesn't support `service` definitions, so we use a messages-only version of the proto file.

### GDScript 2.0 Features

The code uses modern GDScript 2.0 syntax:
- **Typed variables**: `var grpc_client: GrpcClient`
- **@onready**: `@onready var status_label: Label = $StatusLabel`
- **Lambda functions**: `leaderboard.filter(func(e): return e.name != "Bob")`
- **await**: `await get_tree().create_timer(2.0).timeout`

## Configuration

### Server Address

To change the backend server address, edit `main.gd`:

```gdscript
# Line ~40
if grpc_client.connect("dns:///localhost:50051"):
    # Change to your server address
```

### Top Players Limit

To change from top 5 to top 10, edit `main.gd`:

```gdscript
# Line ~58
request.set_initial_limit(10)  # Change from 5 to 10

# Line ~157
if leaderboard.size() > 10:    # Change from 5 to 10
    leaderboard.resize(10)
```

### Reconnection Timing

To adjust reconnection behavior, edit constants in `main.gd`:

```gdscript
# Line ~14
var reconnect_delay: float = 5.0            # Initial delay (default: 2s)
const MAX_RECONNECT_DELAY: float = 60.0    # Max delay (default: 30s)
```

## Troubleshooting

### "Cannot connect to localhost:50051"

**Cause**: Backend is not running

**Solution**:
```bash
cd ../../backend
make quickstart
curl http://localhost:8080/health  # Verify it's running
```

### "GDExtension not loading" / "GrpcClient not found"

**Cause**: Platform-specific binary is missing or incompatible

**Solution**:
1. Check `addons/godot_grpc/bin/` for your platform's binary
2. Verify Godot version is 4.3+ (extension requires 4.3 minimum)
3. Rebuild the extension for your platform if needed

### "Failed to parse LeaderboardUpdate"

**Cause**: Proto file mismatch between frontend and backend

**Solution**:
1. Copy latest proto file from backend:
   ```bash
   cp ../../backend/proto/leaderboard/v1/leaderboard.proto proto/leaderboard/v1/
   ```
2. Recompile the proto file (see "Proto File Compilation" above)

### Stream disconnects frequently

**Cause**: Network issues or backend instability

**Solution**:
- Check backend logs: `docker logs -f grpc-testing-backend-backend-1`
- Increase keepalive timeout in `main.gd` connection options
- Verify network connectivity

## Development

### Adding New Message Types

1. **Update proto file**: Edit `proto/leaderboard/v1/leaderboard_messages.proto`
2. **Recompile**: Run the protobuf compiler command
3. **Update handlers**: Modify `main.gd` to handle new message types

### Customizing UI

The UI is defined in `main_screen.tscn`. You can:
- Edit it directly in Godot's Scene editor
- Modify colors, fonts, and layout
- Add animations or visual effects

### Debugging

Enable verbose gRPC logging in `main.gd`:

```gdscript
# Line ~35
grpc_client.set_log_level(5)  # TRACE level (0=NONE, 5=TRACE)
```

Check Godot console output for detailed logs:
- Connection events
- Message reception
- Parsing results
- UI updates

## Performance

- **Memory**: ~50MB (including Godot runtime)
- **CPU**: <1% idle, <5% during updates
- **Network**: ~10KB initial, ~1KB per update

## License

This project is part of the gRPC testing monorepo. See the parent repository for license information.

## Related Documentation

- **Backend API**: `../../backend/DOC_FOR_DEV.md`
- **Implementation Guide**: `PROMPT_FOR_GODOT_APP.md`
- **Claude Code Guide**: `CLAUDE.md`
- **Proto File**: `../../backend/proto/leaderboard/v1/leaderboard.proto`

## Support

For issues or questions:
1. Check the troubleshooting section above
2. Review backend documentation in `../../backend/DOC_FOR_DEV.md`
3. Check addon documentation:
   - godot_grpc: https://github.com/fredericalix/godot_grpc
   - godobuf: https://github.com/oniksan/godobuf

## Acknowledgments

- **godot_grpc**: Contributors to the godot_grpc GDExtension
- **godobuf**: Oleg Malyavkin (@oniksan) for the protobuf implementation
- **Godot Engine**: The Godot development team

---

**Built with Godot 4.5 | gRPC | Protocol Buffers**
