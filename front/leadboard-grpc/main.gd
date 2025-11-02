extends Control

## Leaderboard Display - Real-time gRPC Streaming Client
## Connects to LeaderboardService and displays top 5 players

# Load protobuf message classes
const LeaderboardPB = preload("res://proto/leaderboard/v1/leaderboard_pb.gd")

# gRPC client
var grpc_client: GrpcClient
var stream_id: int = 0

# Connection state
var is_connected: bool = false
var reconnect_delay: float = 2.0
const MAX_RECONNECT_DELAY: float = 30.0

# Leaderboard state
var leaderboard: Array = []  # Array of Dictionary: {player_name: String, score: int, updated_at: String}

# UI references
@onready var status_label: Label = $MainContainer/VBoxContainer/TitlePanel/VBox/StatusLabel
@onready var player_list: VBoxContainer = $MainContainer/VBoxContainer/LeaderboardPanel/MarginContainer/PlayerList

func _ready():
	print("=== Leaderboard Client Starting ===")
	connect_to_server()

## Connect to the gRPC server
func connect_to_server():
	update_status("Connecting...", Color(0.7, 0.7, 0.7))

	# Create gRPC client
	grpc_client = GrpcClient.new()
	grpc_client.set_log_level(3)  # INFO level for debugging

	# Connect signals
	grpc_client.message.connect(_on_stream_message)
	grpc_client.finished.connect(_on_stream_finished)
	grpc_client.error.connect(_on_stream_error)

	# Connect to server (insecure connection)
	if grpc_client.connect("dns:///localhost:50051"):
		print("✓ Connected to gRPC server at localhost:50051")
		is_connected = true
		reconnect_delay = 2.0  # Reset backoff on successful connection
		update_status("Connected", Color(0.3, 1.0, 0.3))
		start_stream()
	else:
		print("✗ Failed to connect to gRPC server")
		update_status("Connection failed", Color(1.0, 0.3, 0.3))
		schedule_reconnect()

## Start the StreamLeaderboard RPC
func start_stream():
	# Create SubscribeRequest with initial_limit = 5
	var request = LeaderboardPB.SubscribeRequest.new()
	request.set_initial_limit(5)
	var request_bytes = request.to_bytes()

	print("Starting StreamLeaderboard RPC (requesting top 5)...")

	# Start server-streaming RPC
	stream_id = grpc_client.server_stream_start(
		"/leaderboard.v1.LeaderboardService/StreamLeaderboard",
		request_bytes
	)

	print("Stream started with ID: ", stream_id)

## Handle incoming LeaderboardUpdate messages
func _on_stream_message(sid: int, data: PackedByteArray):
	if sid != stream_id:
		return

	print("Received leaderboard update: ", data.size(), " bytes")

	# Parse the LeaderboardUpdate message
	var update = LeaderboardPB.LeaderboardUpdate.new()
	var parse_result = update.from_bytes(data)

	if parse_result != LeaderboardPB.PB_ERR.NO_ERRORS:
		print("Failed to parse LeaderboardUpdate, error code: ", parse_result)
		return

	# Handle based on update kind
	var kind = update.get_kind()

	match kind:
		LeaderboardPB.LeaderboardUpdate.Kind.SNAPSHOT:
			print("Received SNAPSHOT update")
			handle_snapshot(update.get_snapshot())
		LeaderboardPB.LeaderboardUpdate.Kind.UPSERT:
			print("Received UPSERT update")
			handle_upsert(update.get_changed())
		LeaderboardPB.LeaderboardUpdate.Kind.DELETE:
			print("Received DELETE update")
			handle_delete(update.get_changed())
		_:
			print("Unknown update kind: ", kind)

	# Update UI
	update_ui()

## Handle SNAPSHOT: replace entire leaderboard
func handle_snapshot(snapshot_entries):
	if snapshot_entries == null:
		print("Warning: snapshot is null")
		return

	leaderboard.clear()

	# Convert ScoreEntry objects to dictionaries
	for entry in snapshot_entries:
		if entry != null:
			var player_data = {
				"player_name": entry.get_player_name(),
				"score": entry.get_score(),
				"updated_at": entry.get_updated_at()
			}
			leaderboard.append(player_data)

	print("Loaded ", leaderboard.size(), " entries from snapshot")
	update_status("Connected - Live updates active", Color(0.3, 1.0, 0.3))

## Handle UPSERT: update or add player
func handle_upsert(entry):
	if entry == null:
		print("Warning: upsert entry is null")
		return

	var player_name = entry.get_player_name()
	var score = entry.get_score()

	print("UPSERT: ", player_name, " -> ", score)

	# Remove old entry if exists
	leaderboard = leaderboard.filter(
		func(e): return e.get("player_name") != player_name
	)

	# Add new entry
	var player_data = {
		"player_name": player_name,
		"score": score,
		"updated_at": entry.get_updated_at()
	}
	leaderboard.append(player_data)

	# Sort by score descending
	leaderboard.sort_custom(
		func(a, b): return a.get("score", 0) > b.get("score", 0)
	)

	# Keep only top 5 for display
	if leaderboard.size() > 5:
		leaderboard.resize(5)

## Handle DELETE: remove player
func handle_delete(entry):
	if entry == null:
		print("Warning: delete entry is null")
		return

	var player_name = entry.get_player_name()
	print("DELETE: ", player_name)

	leaderboard = leaderboard.filter(
		func(e): return e.get("player_name") != player_name
	)

## Update the UI with current leaderboard
func update_ui():
	# Clear current player rows
	for child in player_list.get_children():
		child.queue_free()

	# Add each player
	for i in range(leaderboard.size()):
		var entry = leaderboard[i]
		var rank = i + 1

		# Create player row
		var row = create_player_row(rank, entry.get("player_name", ""), entry.get("score", 0))
		player_list.add_child(row)

	# If empty, show a message
	if leaderboard.is_empty():
		var empty_label = Label.new()
		empty_label.text = "No players yet..."
		empty_label.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
		empty_label.add_theme_font_size_override("font_size", 24)
		empty_label.add_theme_color_override("font_color", Color(0.5, 0.5, 0.5))
		player_list.add_child(empty_label)

## Create a player row UI element
func create_player_row(rank: int, player_name: String, score: int) -> HBoxContainer:
	var row = HBoxContainer.new()
	row.add_theme_constant_override("separation", 20)

	# Rank label
	var rank_label = Label.new()
	rank_label.text = "#" + str(rank)
	rank_label.custom_minimum_size = Vector2(80, 0)
	rank_label.add_theme_font_size_override("font_size", 32)
	rank_label.add_theme_color_override("font_color", get_rank_color(rank))
	rank_label.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	row.add_child(rank_label)

	# Player name label
	var name_label = Label.new()
	name_label.text = player_name
	name_label.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	name_label.add_theme_font_size_override("font_size", 28)
	name_label.add_theme_color_override("font_color", Color(1, 1, 1))
	row.add_child(name_label)

	# Score label
	var score_label = Label.new()
	score_label.text = format_score(score)
	score_label.custom_minimum_size = Vector2(200, 0)
	score_label.add_theme_font_size_override("font_size", 32)
	score_label.add_theme_color_override("font_color", Color(1, 0.9, 0.4))
	score_label.horizontal_alignment = HORIZONTAL_ALIGNMENT_RIGHT
	row.add_child(score_label)

	return row

## Get color for rank (gold/silver/bronze for top 3)
func get_rank_color(rank: int) -> Color:
	match rank:
		1: return Color(1.0, 0.84, 0.0)    # Gold
		2: return Color(0.75, 0.75, 0.75)  # Silver
		3: return Color(0.8, 0.5, 0.2)     # Bronze
		_: return Color(0.7, 0.7, 0.7)     # Gray

## Format score with thousands separators
func format_score(score: int) -> String:
	var s = str(score)
	var result = ""
	var count = 0

	# Process from right to left
	for i in range(s.length() - 1, -1, -1):
		if count > 0 and count % 3 == 0:
			result = "," + result
		result = s[i] + result
		count += 1

	return result

## Update status label
func update_status(message: String, color: Color):
	status_label.text = message
	status_label.add_theme_color_override("font_color", color)

## Handle stream completion
func _on_stream_finished(sid: int, status_code: int):
	if sid != stream_id:
		return

	print("Stream finished with status code: ", status_code)
	is_connected = false

	if status_code == 0:
		update_status("Stream ended normally", Color(0.7, 0.7, 0.7))
	else:
		update_status("Stream ended with error", Color(1.0, 0.5, 0.3))

	schedule_reconnect()

## Handle stream errors
func _on_stream_error(sid: int, error_code: int, error_msg: String):
	if sid != stream_id:
		return

	print("Stream error [", error_code, "]: ", error_msg)
	is_connected = false
	update_status("Error: " + error_msg, Color(1.0, 0.3, 0.3))
	schedule_reconnect()

## Schedule reconnection with exponential backoff
func schedule_reconnect():
	var delay_text = "Reconnecting in " + str(int(reconnect_delay)) + "s..."
	update_status(delay_text, Color(1.0, 0.7, 0.3))

	print("Scheduling reconnect in ", reconnect_delay, " seconds...")

	await get_tree().create_timer(reconnect_delay).timeout

	# Exponential backoff
	reconnect_delay = min(reconnect_delay * 2.0, MAX_RECONNECT_DELAY)

	# Clean up old client
	if stream_id > 0:
		grpc_client.server_stream_cancel(stream_id)
		stream_id = 0

	if grpc_client:
		grpc_client.close()

	# Reconnect
	connect_to_server()

## Clean up on exit
func _exit_tree():
	print("Shutting down...")

	if stream_id > 0:
		grpc_client.server_stream_cancel(stream_id)

	if grpc_client and grpc_client.is_connected():
		grpc_client.close()
