# OpenAPI/Swagger Integration

This document describes the OpenAPI/Swagger documentation added to the Leaderboard REST API.

## Overview

The REST Admin API now includes interactive OpenAPI 3.0 documentation via Swagger UI, providing a user-friendly interface to explore and test the API endpoints.

## Features

### Swagger UI
- **URL**: http://localhost:8080/swagger/index.html
- Interactive API exploration
- Try-it-out functionality for all endpoints
- Complete request/response schemas
- Example payloads
- Response status codes documentation

### OpenAPI Specification Files
- **JSON**: http://localhost:8080/swagger/doc.json
- **YAML**: `docs/swagger.yaml` (local file)
- **Go Code**: `docs/docs.go` (embedded in binary)

## API Documentation

### Endpoints Documented

#### Health Check
- **GET** `/health`
- Check if the API server is running

#### Score Management

1. **POST** `/scores`
   - Create or update a player score
   - Implements upsert logic with best score retention
   - Request body: `CreateScoreRequest`
   - Response: `ScoreResponse`

2. **PUT** `/scores/{player_name}`
   - Update a specific player's score
   - Only applies if new score is higher
   - Path parameter: `player_name` (1-20 characters)
   - Request body: `UpdateScoreRequest`
   - Response: `ScoreResponse`

3. **DELETE** `/scores/{player_name}`
   - Remove a player's score entry entirely
   - Path parameter: `player_name` (1-20 characters)
   - Response: 204 No Content

### Data Models

#### CreateScoreRequest
```json
{
  "player_name": "Alice",
  "score": 1000
}
```
- `player_name`: string (1-20 characters, required)
- `score`: integer (≥0, required)

#### UpdateScoreRequest
```json
{
  "score": 1500
}
```
- `score`: integer (≥0, required)

#### ScoreResponse
```json
{
  "player_name": "Alice",
  "score": 1000,
  "updated_at": "2025-01-15T10:30:00Z",
  "applied": true
}
```
- `player_name`: string
- `score`: integer
- `updated_at`: RFC3339 timestamp
- `applied`: boolean (indicates if score was improved/created)

#### ErrorResponse
```json
{
  "error": "validation_error",
  "message": "player_name is required"
}
```
- `error`: error type string
- `message`: human-readable error description

## Development Workflow

### Generating Documentation

After modifying REST endpoints or request/response models:

```bash
make swagger
```

This command:
1. Scans Go code for Swagger annotations
2. Generates OpenAPI 3.0 specification
3. Creates `docs/` directory with:
   - `docs.go` (embedded Go code)
   - `swagger.json` (OpenAPI JSON spec)
   - `swagger.yaml` (OpenAPI YAML spec)

### Adding New Endpoints

When adding new REST endpoints:

1. Add Swagger annotations to the handler function:
```go
// createScore godoc
//
//	@Summary		Create a new score
//	@Description	Submit a new player score
//	@Tags			Scores
//	@Accept			json
//	@Produce		json
//	@Param			request	body		CreateScoreRequest	true	"Score data"
//	@Success		200		{object}	ScoreResponse		"Score created"
//	@Failure		400		{object}	ErrorResponse		"Validation error"
//	@Router			/scores [post]
func (s *Server) createScore(c echo.Context) error {
    // handler code
}
```

2. Regenerate documentation:
```bash
make swagger
```

3. Rebuild the application:
```bash
make build
```

### Annotation Reference

Common Swagger annotations used:

- `@Summary`: Brief description (1 line)
- `@Description`: Detailed description (multi-line)
- `@Tags`: Group endpoints by category
- `@Accept`: Content-Type accepted (e.g., json)
- `@Produce`: Content-Type produced (e.g., json)
- `@Param`: Request parameter (path, query, body, header)
- `@Success`: Success response with status code and schema
- `@Failure`: Error response with status code and schema
- `@Router`: Route path and HTTP method

## Integration Details

### Dependencies

The following packages were added:

```go
github.com/swaggo/echo-swagger  // Swagger UI for Echo
github.com/swaggo/files         // Embedded Swagger UI files
github.com/swaggo/swag          // Code generation tool
```

### Server Configuration

The REST server automatically registers the Swagger route:

```go
func (s *Server) registerRoutes() {
    // Swagger documentation
    s.echo.GET("/swagger/*", echoSwagger.WrapHandler)

    // Other routes...
}
```

### Main Application

The generated documentation is imported in `cmd/server/main.go`:

```go
import (
    _ "github.com/yourorg/leaderboard/docs" // Import swagger docs
    // other imports...
)
```

This ensures the Swagger documentation is compiled into the binary.

## Makefile Integration

### New Targets

- `make swagger`: Generate OpenAPI documentation
- `make generate`: Generate all code (proto + sqlc + swagger)

### Updated Targets

- `make clean`: Now removes `docs/` directory
- `make install-tools`: Now installs `swag` CLI tool

## Best Practices

1. **Keep annotations up-to-date**: Update Swagger comments when modifying endpoints
2. **Use examples**: Provide example values in struct tags for better documentation
3. **Document errors**: Include all possible error responses
4. **Regenerate regularly**: Run `make swagger` after API changes
5. **Review the output**: Check Swagger UI to ensure documentation is correct

## Troubleshooting

### Documentation not updating

If changes aren't reflected in Swagger UI:

1. Regenerate documentation: `make swagger`
2. Rebuild the application: `make build`
3. Restart the server
4. Hard refresh the browser (Ctrl+F5 or Cmd+Shift+R)

### Swagger UI not accessible

Ensure:
1. The REST server is running on port 8080
2. The `/swagger/*` route is registered
3. The docs package is imported in `cmd/server/main.go`

### Generation errors

Common issues:
- Missing Swagger annotations on handlers
- Syntax errors in annotations
- Missing `@Router` annotation
- Invalid Go struct tags

Run with verbose output:
```bash
swag init -g internal/transport/rest/server.go -o docs --parseDependency --parseInternal -v
```

## Additional Resources

- [Swaggo Documentation](https://github.com/swaggo/swag)
- [OpenAPI Specification](https://swagger.io/specification/)
- [Swagger UI](https://swagger.io/tools/swagger-ui/)
- [Echo Swagger Integration](https://github.com/swaggo/echo-swagger)
