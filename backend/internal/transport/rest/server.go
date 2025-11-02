// Package rest implements the REST API using Echo
//
//	@title						Leaderboard Admin API
//	@version					1.0
//	@description				REST API for managing videogame leaderboard scores (admin/ops use only)
//	@description				This API provides endpoints to create, update, and delete player scores.
//	@description				The backend enforces "best score" logic: only the highest score per player is kept.
//
//	@contact.name				API Support
//	@contact.email				support@example.com
//
//	@license.name				BSD 3-Clause
//	@license.url				https://opensource.org/licenses/BSD-3-Clause
//
//	@host						localhost:8080
//	@BasePath					/
//
//	@schemes					http
//	@produce					json
//	@consumes					json
//
//	@tag.name					Health
//	@tag.description			Health check endpoints
//	@tag.name					Scores
//	@tag.description			Score management operations
package rest

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/rs/zerolog"
	echoSwagger "github.com/swaggo/echo-swagger"
	"github.com/yourorg/leaderboard/internal/service"
)

// Server implements the REST API using Echo
type Server struct {
	echo   *echo.Echo
	svc    *service.Service
	logger *zerolog.Logger
}

// NewServer creates a new REST server
func NewServer(svc *service.Service, logger *zerolog.Logger) *Server {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	// Middleware
	e.Use(middleware.Recover())
	e.Use(middleware.RequestID())
	e.Use(middleware.CORS())
	e.Use(loggingMiddleware(logger))

	s := &Server{
		echo:   e,
		svc:    svc,
		logger: logger,
	}

	s.registerRoutes()
	return s
}

func (s *Server) registerRoutes() {
	// Swagger documentation
	s.echo.GET("/swagger/*", echoSwagger.WrapHandler)

	// Health check
	s.echo.GET("/health", s.healthCheck)

	// Score management endpoints
	s.echo.POST("/scores", s.createOrUpdateScore)
	s.echo.PUT("/scores/:player_name", s.updateScore)
	s.echo.DELETE("/scores/:player_name", s.deleteScore)
}

// Start starts the REST server
func (s *Server) Start(addr string) error {
	s.logger.Info().Str("addr", addr).Msg("starting REST server")
	return s.echo.Start(addr)
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown() error {
	return s.echo.Close()
}

// Request/Response types

// CreateScoreRequest represents the request body for creating or updating a score
type CreateScoreRequest struct {
	PlayerName string `json:"player_name" validate:"required,min=1,max=20" example:"Alice" minLength:"1" maxLength:"20"`
	Score      int64  `json:"score" validate:"required,min=0" example:"1000" minimum:"0"`
}

// UpdateScoreRequest represents the request body for updating a score
type UpdateScoreRequest struct {
	Score int64 `json:"score" validate:"required,min=0" example:"1500" minimum:"0"`
}

// ScoreResponse represents a score entry in the response
type ScoreResponse struct {
	PlayerName string `json:"player_name" example:"Alice"`
	Score      int64  `json:"score" example:"1000"`
	UpdatedAt  string `json:"updated_at" example:"2025-01-15T10:30:00Z"`
	Applied    bool   `json:"applied,omitempty" example:"true"` // Only for create/update responses
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error" example:"validation_error"`
	Message string `json:"message,omitempty" example:"player_name is required"`
}

// Handlers

// healthCheck godoc
//
//	@Summary		Health check
//	@Description	Check if the API server is running
//	@Tags			Health
//	@Produce		json
//	@Success		200	{object}	map[string]string	"API is healthy"
//	@Router			/health [get]
func (s *Server) healthCheck(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{
		"status": "ok",
	})
}

// createOrUpdateScore godoc
//
//	@Summary		Create or update a player score
//	@Description	Submit a new score for a player. If the player exists, only applies if the new score is higher than the current best.
//	@Description	This endpoint uses "upsert" logic with best score retention.
//	@Tags			Scores
//	@Accept			json
//	@Produce		json
//	@Param			request	body		CreateScoreRequest	true	"Player name and score"
//	@Success		200		{object}	ScoreResponse		"Score created or updated"
//	@Failure		400		{object}	ErrorResponse		"Validation error"
//	@Failure		500		{object}	ErrorResponse		"Internal server error"
//	@Router			/scores [post]
func (s *Server) createOrUpdateScore(c echo.Context) error {
	var req CreateScoreRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "bad_request",
			Message: "invalid request body",
		})
	}

	// Validate
	if req.PlayerName == "" {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "validation_error",
			Message: "player_name is required",
		})
	}
	if req.Score < 0 {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "validation_error",
			Message: "score must be non-negative",
		})
	}

	result, err := s.svc.SubmitScore(c.Request().Context(), req.PlayerName, req.Score)
	if err != nil {
		return s.handleServiceError(c, err)
	}

	return c.JSON(http.StatusOK, ScoreResponse{
		PlayerName: result.PlayerName,
		Score:      result.Score,
		UpdatedAt:  result.UpdatedAt,
		Applied:    result.Applied,
	})
}

// updateScore godoc
//
//	@Summary		Update a player's score
//	@Description	Update a specific player's score by name. Only applies if the new score is higher than the current best.
//	@Tags			Scores
//	@Accept			json
//	@Produce		json
//	@Param			player_name	path		string				true	"Player name (1-20 characters)"	minlength(1)	maxlength(20)
//	@Param			request		body		UpdateScoreRequest	true	"New score value"
//	@Success		200			{object}	ScoreResponse		"Score updated"
//	@Failure		400			{object}	ErrorResponse		"Validation error"
//	@Failure		500			{object}	ErrorResponse		"Internal server error"
//	@Router			/scores/{player_name} [put]
func (s *Server) updateScore(c echo.Context) error {
	playerName := c.Param("player_name")
	if playerName == "" {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "validation_error",
			Message: "player_name is required",
		})
	}

	var req UpdateScoreRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "bad_request",
			Message: "invalid request body",
		})
	}

	if req.Score < 0 {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "validation_error",
			Message: "score must be non-negative",
		})
	}

	result, err := s.svc.SubmitScore(c.Request().Context(), playerName, req.Score)
	if err != nil {
		return s.handleServiceError(c, err)
	}

	return c.JSON(http.StatusOK, ScoreResponse{
		PlayerName: result.PlayerName,
		Score:      result.Score,
		UpdatedAt:  result.UpdatedAt,
		Applied:    result.Applied,
	})
}

// deleteScore godoc
//
//	@Summary		Delete a player's score
//	@Description	Remove a player's score entry from the leaderboard entirely
//	@Tags			Scores
//	@Produce		json
//	@Param			player_name	path	string	true	"Player name (1-20 characters)"	minlength(1)	maxlength(20)
//	@Success		204			"Score deleted successfully"
//	@Failure		400			{object}	ErrorResponse	"Validation error"
//	@Failure		404			{object}	ErrorResponse	"Player not found"
//	@Failure		500			{object}	ErrorResponse	"Internal server error"
//	@Router			/scores/{player_name} [delete]
func (s *Server) deleteScore(c echo.Context) error {
	playerName := c.Param("player_name")
	if playerName == "" {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "validation_error",
			Message: "player_name is required",
		})
	}

	if err := s.svc.DeleteScore(c.Request().Context(), playerName); err != nil {
		return s.handleServiceError(c, err)
	}

	return c.NoContent(http.StatusNoContent)
}

func (s *Server) handleServiceError(c echo.Context, err error) error {
	if errors.Is(err, service.ErrInvalidPlayerName) {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "validation_error",
			Message: err.Error(),
		})
	}
	if errors.Is(err, service.ErrInvalidScore) {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "validation_error",
			Message: err.Error(),
		})
	}
	if errors.Is(err, service.ErrPlayerNotFound) {
		return c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "player not found",
		})
	}

	s.logger.Error().Err(err).Msg("internal server error")
	return c.JSON(http.StatusInternalServerError, ErrorResponse{
		Error:   "internal_error",
		Message: "an internal error occurred",
	})
}

// loggingMiddleware creates a logging middleware using zerolog
func loggingMiddleware(logger *zerolog.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()
			res := c.Response()

			err := next(c)

			logger.Info().
				Str("method", req.Method).
				Str("uri", req.RequestURI).
				Int("status", res.Status).
				Str("remote_ip", c.RealIP()).
				Str("request_id", c.Response().Header().Get(echo.HeaderXRequestID)).
				Err(err).
				Msg("http request")

			return err
		}
	}
}
