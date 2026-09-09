package webui

import (
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

const (
	indexBody = "<!doctype html><title>rss-reader</title><div id=app></div>"
	assetBody = "console.log('hi')"
)

// built mimics a real Vite build: an index.html plus content-hashed files under assets/.
func built() fstest.MapFS {
	return fstest.MapFS{
		"index.html":                &fstest.MapFile{Data: []byte(indexBody)},
		"assets/index-upzk593E.js":  &fstest.MapFile{Data: []byte(assetBody)},
		"assets/index-C48EtEa9.css": &fstest.MapFile{Data: []byte("body{}")},
		"favicon.ico":               &fstest.MapFile{Data: []byte("\x00")},
	}
}

func mustHandler(t *testing.T, fsys fs.FS) http.Handler {
	t.Helper()
	h, err := NewHandler(fsys)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	return h
}

func get(t *testing.T, h http.Handler, target string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil))
	return rec
}

func TestServesIndexAtRoot(t *testing.T) {
	rec := get(t, mustHandler(t, built()), "/")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Body.String(); got != indexBody {
		t.Errorf("body = %q, want %q", got, indexBody)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
	// The document must be revalidated: its asset references change on every build.
	if cc := rec.Header().Get("Cache-Control"); cc != noCache {
		t.Errorf("Cache-Control = %q, want %q", cc, noCache)
	}
}

func TestServesHashedAssetImmutably(t *testing.T) {
	rec := get(t, mustHandler(t, built()), "/assets/index-upzk593E.js")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Body.String(); got != assetBody {
		t.Errorf("body = %q, want %q", got, assetBody)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/javascript") &&
		!strings.HasPrefix(ct, "application/javascript") {
		t.Errorf("Content-Type = %q, want a JavaScript type", ct)
	}
	// Vite fingerprints these filenames, so the bytes behind a URL never change.
	if cc := rec.Header().Get("Cache-Control"); cc != immutableCache {
		t.Errorf("Cache-Control = %q, want %q", cc, immutableCache)
	}
}

func TestServesNonHashedFileWithoutImmutableCache(t *testing.T) {
	rec := get(t, mustHandler(t, built()), "/favicon.ico")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != noCache {
		t.Errorf("Cache-Control = %q, want %q", cc, noCache)
	}
}

// An unknown path is a client-side route, not a missing file: hand back the shell and let
// the app decide. Nothing routes today, but this is what a history-mode router would need.
func TestUnknownPathFallsBackToIndex(t *testing.T) {
	h := mustHandler(t, built())

	for _, target := range []string{"/feeds/12", "/deep/nested/route", "/assets"} {
		t.Run(target, func(t *testing.T) {
			rec := get(t, h, target)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
			}
			if got := rec.Body.String(); got != indexBody {
				t.Errorf("body = %q, want the index shell", got)
			}
		})
	}
}

// A missing asset is a broken build, not a route — serving the HTML shell for a .js request
// would surface as a confusing MIME-type error in the browser instead of a plain 404.
func TestMissingAssetIs404(t *testing.T) {
	rec := get(t, mustHandler(t, built()), "/assets/index-deadbeef.js")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if strings.Contains(rec.Body.String(), "<div id=app>") {
		t.Error("served the index shell for a missing asset")
	}
}

func TestRejectsTraversal(t *testing.T) {
	// Nest the served tree so there is something real to escape to.
	parent := fstest.MapFS{
		"dist/index.html": &fstest.MapFile{Data: []byte(indexBody)},
		"secret.txt":      &fstest.MapFile{Data: []byte("nope")},
	}
	sub, err := fs.Sub(parent, "dist")
	if err != nil {
		t.Fatalf("fs.Sub: %v", err)
	}
	h := mustHandler(t, sub)

	// http.NewRequest would clean this away, so drive the handler with a raw target.
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.URL.Path = "/../secret.txt"
	h.ServeHTTP(rec, req)

	if strings.Contains(rec.Body.String(), "nope") {
		t.Fatalf("traversal escaped the dist root: %q", rec.Body.String())
	}
}

func TestNewHandlerReportsUnbuiltUI(t *testing.T) {
	// What a clean checkout looks like: the embed placeholder and nothing else.
	_, err := NewHandler(fstest.MapFS{".gitkeep": &fstest.MapFile{}})

	if !errors.Is(err, ErrNotBuilt) {
		t.Fatalf("err = %v, want ErrNotBuilt", err)
	}
}

// Dist must stay usable even when only the placeholder is embedded — that is the state
// `go build ./...` sees on a fresh clone.
func TestDistIsAlwaysReadable(t *testing.T) {
	fsys, err := Dist()
	if err != nil {
		t.Fatalf("Dist: %v", err)
	}
	if _, err := fs.ReadDir(fsys, "."); err != nil {
		t.Fatalf("read embedded dist: %v", err)
	}
}
