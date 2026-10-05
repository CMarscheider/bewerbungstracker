package httpapi

import (
	"net/http"
	"time"
)

// slowWriteTimeout ersetzt für die Unterlagen-Routen das WriteTimeout des Servers (60 s):
// PDF (bis 45 s) + IMAP-Entwurf (bis 30 s) + Datenbank. nginx erlaubt dort 120 s.
const slowWriteTimeout = 100 * time.Second

// slowRoutes enthält die Routen, die rendern und ggf. einen Entwurf per IMAP anlegen.
var slowRoutes = func() *http.ServeMux {
	m := http.NewServeMux()
	noop := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	for _, p := range []string{
		"PUT /api/agent/applications/{id}/documents",
		"PUT /api/v1/applications/{id}/documents",
		"POST /api/v1/applications/{id}/documents/draft",
	} {
		m.Handle(p, noop)
	}
	return m
}()

func isSlowRoute(r *http.Request) bool {
	_, pattern := slowRoutes.Handler(r)
	return pattern != ""
}

// extendWriteDeadline verlängert auf den langsamen Unterlagen-Routen die Schreibfrist auf d.
func extendWriteDeadline(d time.Duration, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isSlowRoute(r) {
			// Fehler nur, wenn der Writer keine Fristen kennt (z. B. httptest.Recorder) – dann gilt die alte.
			_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(d))
		}
		next.ServeHTTP(w, r)
	})
}
