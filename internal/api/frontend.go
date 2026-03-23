package api

import (
	"io/fs"
	"net/http"
	"strings"

	"arb/web"
)

// frontendHandler returns an http.Handler that serves the embedded React
// frontend. For SPA routing any path that does not resolve to a real file
// falls back to index.html so the React router can handle it client-side.
func frontendHandler() http.Handler {
	sub, err := fs.Sub(web.Dist, "dist")
	if err != nil {
		panic("embedded dist directory missing: " + err.Error())
	}
	fsys := &spaFS{fs: http.FS(sub)}
	return http.FileServer(fsys)
}

// spaFS wraps an http.FileSystem and falls back to index.html for missing
// paths so the React SPA router works on all routes.
type spaFS struct {
	fs http.FileSystem
}

func (s *spaFS) Open(name string) (http.File, error) {
	f, err := s.fs.Open(name)
	if err != nil {
		// If the path looks like a real file request (has an extension in the
		// last segment), return the actual 404 so browsers don't get a false
		// 200 for missing assets like .js, .css, etc.
		last := name
		if idx := strings.LastIndex(name, "/"); idx >= 0 {
			last = name[idx:]
		}
		if strings.Contains(last, ".") {
			return nil, err
		}
		// SPA fallback — serve index.html for route paths.
		return s.fs.Open("index.html")
	}
	return f, nil
}
