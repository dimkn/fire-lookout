// Package webui is the driving adapter that serves the built Vue single-page app.
//
// The assets are compiled into the binary, so a release is a single file. They arrive here
// through the Docker build (infra/Dockerfile), which drops frontend/dist into this
// package's dist/ directory before compiling — Go's embed cannot reach outside the module,
// and the frontend pillar must not be reachable from Go source (AGENTS.md §3, rule 6).
// A clean checkout carries only dist/.gitkeep; NewHandler then reports ErrNotBuilt and the
// caller keeps serving the API alone, which is what the Vite dev flow needs.
package webui

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"
)

// The "all:" prefix is what lets this compile on a fresh clone: without it embed rejects a
// directory whose only member is a dotfile.
//
//go:embed all:dist
var distFS embed.FS

// ErrNotBuilt reports that the embedded dist/ holds no index.html — the frontend was never
// built into this binary.
var ErrNotBuilt = errors.New("webui: frontend assets are not built into this binary")

const (
	indexFile = "index.html"

	// assetDir holds Vite's content-hashed output. A URL under it always denotes one exact
	// build of one exact file, so it can be cached forever.
	assetDir = "assets/"

	immutableCache = "public, max-age=31536000, immutable"
	noCache        = "no-cache"
)

// Dist returns the embedded frontend build, rooted at the directory index.html sits in.
func Dist() (fs.FS, error) {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		return nil, fmt.Errorf("locate embedded frontend: %w", err)
	}
	return sub, nil
}

// NewHandler serves fsys as a single-page app. It returns ErrNotBuilt if fsys carries no
// index.html, so callers can distinguish "no UI in this binary" from a real failure.
func NewHandler(fsys fs.FS) (http.Handler, error) {
	index, err := fs.ReadFile(fsys, indexFile)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, ErrNotBuilt
		}
		return nil, fmt.Errorf("read %s: %w", indexFile, err)
	}
	return &handler{fsys: fsys, index: index}, nil
}

type handler struct {
	fsys  fs.FS
	index []byte
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")

	// A bare "/" cleans to "." — and anything that tried to climb out of the root has been
	// flattened by now, so what is left either names a file in fsys or does not exist.
	if name == "" || name == "." {
		h.serveIndex(w, r)
		return
	}

	file, err := h.fsys.Open(name)
	if err != nil {
		// A request for a concrete file that is missing is a broken build, not a route:
		// answering with the HTML shell would surface as a MIME-type error in the browser.
		if isAssetRequest(name) {
			http.NotFound(w, r)
			return
		}
		h.serveIndex(w, r)
		return
	}
	defer func() { _ = file.Close() }()

	info, err := file.Stat()
	if err != nil || info.IsDir() {
		h.serveIndex(w, r)
		return
	}

	seeker, ok := file.(io.ReadSeeker)
	if !ok {
		// Every fs.FS in play (embed.FS, fstest.MapFS) returns seekable files; this is a
		// guard rather than a real path.
		h.serveIndex(w, r)
		return
	}

	if strings.HasPrefix(name, assetDir) {
		w.Header().Set("Cache-Control", immutableCache)
	} else {
		w.Header().Set("Cache-Control", noCache)
	}
	// Embedded files carry a zero ModTime, which ServeContent reads as "unknown" and simply
	// omits from the response.
	http.ServeContent(w, r, info.Name(), time.Time{}, seeker)
}

// serveIndex hands back the app shell. Unknown paths land here so a client-side route
// survives a reload; the shell itself must always be revalidated, because each build
// rewrites the hashed asset URLs it points at.
func (h *handler) serveIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", noCache)
	http.ServeContent(w, r, indexFile, time.Time{}, bytes.NewReader(h.index))
}

// isAssetRequest reports whether name asks for a concrete file rather than a client-side
// route: anything Vite fingerprinted, or any path carrying a file extension.
func isAssetRequest(name string) bool {
	return strings.HasPrefix(name, assetDir) || path.Ext(name) != ""
}
