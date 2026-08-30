// Package httpapi is the driving adapter: it exposes the application's read use cases
// over HTTP using the server stubs generated from api/openapi.yaml. Handlers stay thin —
// translate, delegate, translate back.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"fire-lookout/backend/internal/domain"
)

// StatusProvider is the slice of the application layer this adapter needs. Declaring it
// here (rather than depending on the concrete service) keeps the handlers testable with
// a fake.
type StatusProvider interface {
	Overview(ctx context.Context) ([]domain.SystemOverview, error)
	FeedItems(ctx context.Context, feedID int64, since *time.Time, limit int) ([]domain.StatusItem, error)
}

// Server implements the generated ServerInterface.
type Server struct {
	status StatusProvider
}

var _ ServerInterface = (*Server)(nil)

// NewServer wires the handlers to the application.
func NewServer(status StatusProvider) *Server {
	return &Server{status: status}
}

// NewRouter mounts the generated routes under baseURL (e.g. "/api") and reports
// parameter-validation failures using the contract's Error schema.
func NewRouter(si ServerInterface, baseURL string) http.Handler {
	return HandlerWithOptions(si, StdHTTPServerOptions{
		BaseURL: baseURL,
		ErrorHandlerFunc: func(w http.ResponseWriter, _ *http.Request, err error) {
			writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		},
	})
}

// GetOverview serves the main page's read model: one row per subscribed system.
func (s *Server) GetOverview(w http.ResponseWriter, r *http.Request) {
	overviews, err := s.status.Overview(r.Context())
	if err != nil {
		fail(w, r, err)
		return
	}

	out := make([]SystemOverview, 0, len(overviews))
	for _, o := range overviews {
		out = append(out, toSystemOverview(o))
	}
	writeJSON(w, http.StatusOK, out)
}

// ListFeedItems serves one system's incidents, newest first. An absent limit is passed
// through as zero: clamping to the published bounds is the application's rule.
func (s *Server) ListFeedItems(w http.ResponseWriter, r *http.Request, feedID FeedId, params ListFeedItemsParams) {
	limit := 0
	if params.Limit != nil {
		limit = *params.Limit
	}

	items, err := s.status.FeedItems(r.Context(), feedID, params.Since, limit)
	if err != nil {
		fail(w, r, err)
		return
	}

	out := make([]StatusItem, 0, len(items))
	for _, item := range items {
		out = append(out, toStatusItem(item))
	}
	writeJSON(w, http.StatusOK, out)
}

func toSystemOverview(o domain.SystemOverview) SystemOverview {
	feed := toFeed(o.Feed)
	if o.CurrentStatus != nil {
		status := Status(*o.CurrentStatus)
		feed.CurrentStatus = &status
	}

	return SystemOverview{
		Feed:          feed,
		Indicator:     Indicator(o.Indicator),
		LastUpdatedAt: o.LastUpdatedAt,
	}
}

func toFeed(f domain.Feed) Feed {
	out := Feed{
		Id:                 f.ID,
		Url:                f.URL,
		Title:              f.Title,
		GroupId:            f.GroupID,
		Enabled:            f.Enabled,
		RefreshIntervalSec: int(f.RefreshInterval.Seconds()),
		LastFetchedAt:      f.LastFetchedAt,
		LastSuccessAt:      f.LastSuccessAt,
		CreatedAt:          f.CreatedAt,
		UpdatedAt:          f.UpdatedAt,
	}
	if f.LastError != "" {
		out.LastError = &f.LastError
	}
	return out
}

func toStatusItem(i domain.StatusItem) StatusItem {
	out := StatusItem{
		Id:        i.ID,
		FeedId:    i.FeedID,
		Guid:      i.GUID,
		Title:     i.Title,
		Status:    Status(i.Status),
		FetchedAt: i.FetchedAt,
	}
	// A zero time means the provider shipped no such timestamp; report that as null
	// rather than as the year 1.
	if !i.PublishedAt.IsZero() {
		out.PublishedAt = &i.PublishedAt
	}
	if !i.UpdatedAt.IsZero() {
		out.UpdatedAt = &i.UpdatedAt
	}
	if i.Link != "" {
		out.Link = &i.Link
	}
	if i.ContentHTML != "" {
		out.ContentHtml = &i.ContentHTML
	}
	if i.ContentText != "" {
		out.ContentText = &i.ContentText
	}
	return out
}

// fail maps a use-case error onto the contract's error responses. Internal failures are
// logged in full and reported vaguely — the client can do nothing with our stack.
func fail(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, domain.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "the requested resource does not exist")
		return
	}

	slog.Error("request failed", "method", r.Method, "path", r.URL.Path, "error", err)
	writeError(w, http.StatusInternalServerError, "internal_error", "the request could not be completed")
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, Error{Code: code, Message: message})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Error("write response body", "error", err)
	}
}
