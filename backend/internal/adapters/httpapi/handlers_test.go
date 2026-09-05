package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"fire-lookout/backend/internal/domain"
)

type fakeStatus struct {
	overview []domain.SystemOverview
	items    []domain.StatusItem
	err      error

	gotFeedID int64
	gotSince  *time.Time
	gotLimit  int
}

func (f *fakeStatus) Overview(context.Context) ([]domain.SystemOverview, error) {
	return f.overview, f.err
}

func (f *fakeStatus) FeedItems(_ context.Context, feedID int64, since *time.Time, limit int) ([]domain.StatusItem, error) {
	f.gotFeedID, f.gotSince, f.gotLimit = feedID, since, limit
	return f.items, f.err
}

func ts(t *testing.T, s string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return parsed
}

func ptr[T any](v T) *T { return &v }

// fakeSubscriber stands in for the write-side use case.
type fakeSubscriber struct {
	feed domain.Feed
	err  error

	calls int
	got   domain.SubscribeInput
}

func (f *fakeSubscriber) SubscribeFeed(_ context.Context, in domain.SubscribeInput) (domain.Feed, error) {
	f.calls++
	f.got = in
	return f.feed, f.err
}

func do(t *testing.T, fake *fakeStatus, method, target string) *httptest.ResponseRecorder {
	t.Helper()
	return send(t, fake, &fakeSubscriber{}, method, target, "")
}

func send(t *testing.T, status *fakeStatus, subscriber *fakeSubscriber, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()

	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), method, target, reader)
	NewRouter(NewServer(status, subscriber), "/api").ServeHTTP(rec, req)
	return rec
}

func TestGetOverviewReturnsRows(t *testing.T) {
	status := domain.StatusInvestigating
	updated := ts(t, "2026-08-18T08:00:00Z")
	fake := &fakeStatus{overview: []domain.SystemOverview{{
		Feed: domain.Feed{
			ID: 1, URL: "https://githubstatus.test/history.atom", Title: "GitHub",
			GroupID: 0, Enabled: true, RefreshInterval: 5 * time.Minute,
			LastFetchedAt: ptr(ts(t, "2026-08-18T09:05:00Z")),
			LastSuccessAt: ptr(ts(t, "2026-08-18T09:00:00Z")),
			LastError:     "502 Bad Gateway",
			CreatedAt:     ts(t, "2026-08-01T00:00:00Z"),
			UpdatedAt:     ts(t, "2026-08-02T00:00:00Z"),
		},
		Indicator:     domain.IndicatorOutage,
		CurrentStatus: &status,
		LastUpdatedAt: &updated,
	}}}

	rec := do(t, fake, http.MethodGet, "/api/overview")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var got []SystemOverview
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal body %s: %v", rec.Body.String(), err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d rows, want 1", len(got))
	}

	row := got[0]
	if row.Indicator != IndicatorOutage {
		t.Errorf("indicator = %q, want %q", row.Indicator, IndicatorOutage)
	}
	if row.Feed.Title != "GitHub" || row.Feed.Id != 1 {
		t.Errorf("feed = %+v, want GitHub/1", row.Feed)
	}
	if row.Feed.RefreshIntervalSec != 300 {
		t.Errorf("refresh_interval_sec = %d, want 300", row.Feed.RefreshIntervalSec)
	}
	if row.Feed.CurrentStatus == nil || *row.Feed.CurrentStatus != StatusInvestigating {
		t.Errorf("feed.current_status = %v, want investigating", row.Feed.CurrentStatus)
	}
	if row.Feed.LastError == nil || *row.Feed.LastError != "502 Bad Gateway" {
		t.Errorf("feed.last_error = %v, want the message", row.Feed.LastError)
	}
	if row.LastUpdatedAt == nil || !row.LastUpdatedAt.Equal(updated) {
		t.Errorf("last_updated_at = %v, want %v", row.LastUpdatedAt, updated)
	}
}

func TestGetOverviewOmitsNullsForAPristineFeed(t *testing.T) {
	fake := &fakeStatus{overview: []domain.SystemOverview{{
		Feed:      domain.Feed{ID: 2, Title: "Never polled", Enabled: true},
		Indicator: domain.IndicatorUnknown,
	}}}

	rec := do(t, fake, http.MethodGet, "/api/overview")

	var got []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal body %s: %v", rec.Body.String(), err)
	}
	feed, ok := got[0]["feed"].(map[string]any)
	if !ok {
		t.Fatalf("row has no feed object: %v", got[0])
	}
	for _, key := range []string{"current_status", "last_fetched_at", "last_success_at", "last_error"} {
		if v, present := feed[key]; present {
			t.Errorf("feed.%s = %v, want it omitted for a never-polled feed", key, v)
		}
	}
	if v, present := got[0]["last_updated_at"]; present {
		t.Errorf("last_updated_at = %v, want it omitted", v)
	}
}

func TestGetOverviewWithNoSystemsIsAnEmptyArray(t *testing.T) {
	rec := do(t, &fakeStatus{}, http.MethodGet, "/api/overview")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	// Must be [] and never null: the UI iterates the response directly.
	if body := strings.TrimSpace(rec.Body.String()); body != "[]" {
		t.Errorf("body = %s, want []", body)
	}
}

func TestGetOverviewFailureIsFiveHundred(t *testing.T) {
	rec := do(t, &fakeStatus{err: errors.New("database on fire")}, http.MethodGet, "/api/overview")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}

	var body Error
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body %s: %v", rec.Body.String(), err)
	}
	if body.Code != "internal_error" {
		t.Errorf("code = %q, want internal_error", body.Code)
	}
	if body.Message == "" {
		t.Error("message is empty, want an explanation")
	}
	if strings.Contains(body.Message, "database on fire") {
		t.Error("message leaks the internal error text")
	}
}

func TestListFeedItemsReturnsItems(t *testing.T) {
	fake := &fakeStatus{items: []domain.StatusItem{
		{
			ID: 10, FeedID: 1, GUID: "guid-1", Title: "Elevated errors",
			Link:        "https://githubstatus.test/incidents/1",
			PublishedAt: ts(t, "2026-08-18T08:00:00Z"), UpdatedAt: ts(t, "2026-08-18T08:30:00Z"),
			ContentHTML: "<p>update</p>", ContentText: "update",
			Status: domain.StatusMonitoring, FetchedAt: ts(t, "2026-08-18T09:00:00Z"),
		},
		{
			ID: 11, FeedID: 1, GUID: "guid-2", Title: "Bare entry",
			Status: domain.StatusUnknown, FetchedAt: ts(t, "2026-08-18T09:00:00Z"),
		},
	}}

	rec := do(t, fake, http.MethodGet, "/api/feeds/1/items")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	if fake.gotFeedID != 1 {
		t.Errorf("feed id passed to the service = %d, want 1", fake.gotFeedID)
	}

	var got []StatusItem
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal body %s: %v", rec.Body.String(), err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d items, want 2", len(got))
	}
	if got[0].Status != StatusMonitoring {
		t.Errorf("status = %q, want monitoring", got[0].Status)
	}
	if got[0].Link == nil || *got[0].Link != "https://githubstatus.test/incidents/1" {
		t.Errorf("link = %v, want the permalink", got[0].Link)
	}
	if got[0].ContentText == nil || *got[0].ContentText != "update" {
		t.Errorf("content_text = %v, want %q", got[0].ContentText, "update")
	}
	if got[0].PublishedAt == nil || !got[0].PublishedAt.Equal(ts(t, "2026-08-18T08:00:00Z")) {
		t.Errorf("published_at = %v, want the provider's timestamp", got[0].PublishedAt)
	}
	if got[1].Link != nil || got[1].ContentText != nil || got[1].ContentHtml != nil {
		t.Errorf("bare item = %+v, want empty optionals omitted", got[1])
	}
	// The provider shipped no timestamps: null, not the year 1.
	if got[1].PublishedAt != nil || got[1].UpdatedAt != nil {
		t.Errorf("bare item timestamps = %v / %v, want both omitted", got[1].PublishedAt, got[1].UpdatedAt)
	}
	if !got[1].FetchedAt.Equal(ts(t, "2026-08-18T09:00:00Z")) {
		t.Errorf("fetched_at = %v, want it always present", got[1].FetchedAt)
	}
}

func TestListFeedItemsWithNoItemsIsAnEmptyArray(t *testing.T) {
	rec := do(t, &fakeStatus{}, http.MethodGet, "/api/feeds/1/items")

	if body := strings.TrimSpace(rec.Body.String()); body != "[]" {
		t.Errorf("body = %s, want []", body)
	}
}

func TestListFeedItemsForwardsQueryParameters(t *testing.T) {
	fake := &fakeStatus{}
	since := "2026-08-11T00:00:00Z"

	do(t, fake, http.MethodGet, fmt.Sprintf("/api/feeds/7/items?since=%s&limit=5", since))

	if fake.gotFeedID != 7 {
		t.Errorf("feed id = %d, want 7", fake.gotFeedID)
	}
	if fake.gotSince == nil || !fake.gotSince.Equal(ts(t, since)) {
		t.Errorf("since = %v, want %s", fake.gotSince, since)
	}
	if fake.gotLimit != 5 {
		t.Errorf("limit = %d, want 5", fake.gotLimit)
	}
}

// Clamping is the application's rule, so an absent limit must reach it as zero rather
// than being invented here.
func TestListFeedItemsWithoutParametersPassesZeroLimit(t *testing.T) {
	fake := &fakeStatus{}

	do(t, fake, http.MethodGet, "/api/feeds/7/items")

	if fake.gotLimit != 0 {
		t.Errorf("limit = %d, want 0", fake.gotLimit)
	}
	if fake.gotSince != nil {
		t.Errorf("since = %v, want nil", fake.gotSince)
	}
}

func TestListFeedItemsUnknownFeedIsNotFound(t *testing.T) {
	fake := &fakeStatus{err: fmt.Errorf("get feed 99: %w", domain.ErrNotFound)}

	rec := do(t, fake, http.MethodGet, "/api/feeds/99/items")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (body %s)", rec.Code, rec.Body.String())
	}

	var body Error
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body %s: %v", rec.Body.String(), err)
	}
	if body.Code != "not_found" {
		t.Errorf("code = %q, want not_found", body.Code)
	}
}

func TestMalformedFeedIdIsBadRequest(t *testing.T) {
	rec := do(t, &fakeStatus{}, http.MethodGet, "/api/feeds/not-a-number/items")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body %s)", rec.Code, rec.Body.String())
	}

	var body Error
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body %s: %v", rec.Body.String(), err)
	}
	if body.Code != "bad_request" {
		t.Errorf("code = %q, want bad_request", body.Code)
	}
}

func TestSubscribeFeedCreatesAndReturnsTheFeed(t *testing.T) {
	subscriber := &fakeSubscriber{feed: domain.Feed{
		ID: 7, URL: "https://a.test/feed.atom", Title: "GitHub",
		Enabled: true, RefreshInterval: 5 * time.Minute,
		CreatedAt: ts(t, "2026-08-24T12:00:00Z"), UpdatedAt: ts(t, "2026-08-24T12:00:00Z"),
	}}

	rec := send(t, &fakeStatus{}, subscriber, http.MethodPost, "/api/feeds",
		`{"url":"https://a.test/feed.atom","title":"GitHub"}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body %s)", rec.Code, rec.Body.String())
	}

	var got Feed
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal body %s: %v", rec.Body.String(), err)
	}
	if got.Id != 7 || got.Title != "GitHub" {
		t.Errorf("feed = %+v, want the created one", got)
	}
	if subscriber.got.URL != "https://a.test/feed.atom" || subscriber.got.Title != "GitHub" {
		t.Errorf("use case received %+v, want the body's url and title", subscriber.got)
	}
	// A brand-new feed has never been polled, so its light stays unknown.
	if got.CurrentStatus != nil || got.LastSuccessAt != nil {
		t.Errorf("feed = %+v, want no derived status or health yet", got)
	}
}

func TestSubscribeFeedPassesOptionalFieldsThrough(t *testing.T) {
	subscriber := &fakeSubscriber{}

	send(t, &fakeStatus{}, subscriber, http.MethodPost, "/api/feeds",
		`{"url":"https://a.test/feed.atom","title":"GitHub","group_id":3,"refresh_interval_sec":90,"enabled":false}`)

	if subscriber.got.GroupID != 3 {
		t.Errorf("GroupID = %d, want 3", subscriber.got.GroupID)
	}
	if subscriber.got.RefreshInterval != 90*time.Second {
		t.Errorf("RefreshInterval = %v, want 90s", subscriber.got.RefreshInterval)
	}
	if subscriber.got.Enabled == nil || *subscriber.got.Enabled {
		t.Errorf("Enabled = %v, want false", subscriber.got.Enabled)
	}
}

// Omitted optionals must stay nil/zero so the application applies its own defaults.
func TestSubscribeFeedLeavesOmittedOptionalsAlone(t *testing.T) {
	subscriber := &fakeSubscriber{}

	send(t, &fakeStatus{}, subscriber, http.MethodPost, "/api/feeds",
		`{"url":"https://a.test/feed.atom","title":"GitHub"}`)

	if subscriber.got.Enabled != nil {
		t.Errorf("Enabled = %v, want nil so the default applies", subscriber.got.Enabled)
	}
	if subscriber.got.RefreshInterval != 0 {
		t.Errorf("RefreshInterval = %v, want zero so the default applies", subscriber.got.RefreshInterval)
	}
	if subscriber.got.GroupID != 0 {
		t.Errorf("GroupID = %d, want 0", subscriber.got.GroupID)
	}
}

func TestSubscribeFeedErrorMapping(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
		wantInBody string
	}{
		{
			name:       "a rejected field is a bad request",
			err:        domain.ValidationError{Field: "title", Message: "Name must not be empty."},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_input",
			wantInBody: "Name must not be empty.",
		},
		{
			name:       "a duplicate url is a conflict",
			err:        fmt.Errorf("subscribe: %w", domain.ErrDuplicateURL),
			wantStatus: http.StatusConflict,
			wantCode:   "feed_exists",
		},
		{
			name:       "a duplicate name is a conflict",
			err:        fmt.Errorf("subscribe: %w", domain.ErrDuplicateTitle),
			wantStatus: http.StatusConflict,
			wantCode:   "name_exists",
		},
		{
			name:       "an unreadable feed is unprocessable",
			err:        fmt.Errorf("validate: %w", domain.ErrFeedUnreachable),
			wantStatus: http.StatusUnprocessableEntity,
			wantCode:   "feed_unreachable",
		},
		{
			name:       "anything else is a server error",
			err:        errors.New("database on fire"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   "internal_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := send(t, &fakeStatus{}, &fakeSubscriber{err: tt.err}, http.MethodPost, "/api/feeds",
				`{"url":"https://a.test/feed.atom","title":"GitHub"}`)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body %s)", rec.Code, tt.wantStatus, rec.Body.String())
			}

			var body Error
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("unmarshal body %s: %v", rec.Body.String(), err)
			}
			if body.Code != tt.wantCode {
				t.Errorf("code = %q, want %q", body.Code, tt.wantCode)
			}
			if body.Message == "" {
				t.Error("message is empty, want something showable to the user")
			}
			if tt.wantInBody != "" && body.Message != tt.wantInBody {
				t.Errorf("message = %q, want %q", body.Message, tt.wantInBody)
			}
			// Internal detail must never reach the client.
			if strings.Contains(body.Message, "database on fire") {
				t.Error("message leaks the internal error text")
			}
		})
	}
}

func TestSubscribeFeedRejectsMalformedJSON(t *testing.T) {
	subscriber := &fakeSubscriber{}

	rec := send(t, &fakeStatus{}, subscriber, http.MethodPost, "/api/feeds", `{"url":`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if subscriber.calls != 0 {
		t.Errorf("use case called %d times, want 0", subscriber.calls)
	}
}

func TestUnimplementedOperationsAreNotRouted(t *testing.T) {
	// The contract documents more than this slice implements; those paths must 404
	// rather than half-answer.
	rec := do(t, &fakeStatus{}, http.MethodGet, "/api/groups")

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 for an unimplemented path", rec.Code)
	}
}
