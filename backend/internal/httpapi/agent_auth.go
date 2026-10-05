package httpapi

import (
	"crypto/subtle"
	"log/slog"
	"net/http"
	"strings"
)

// agentPrefix ist der Pfadpräfix der Agent-API.
const agentPrefix = "/api/agent/"

// requireAgentToken schützt die Agent-API: Ohne konfiguriertes Token gibt es sie nicht (404),
// sonst ist "Authorization: Bearer <token>" Pflicht (401). Andere Pfade bleiben unberührt.
func requireAgentToken(token string, logger *slog.Logger, next http.Handler) http.Handler {
	want := []byte("Bearer " + token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, agentPrefix) {
			next.ServeHTTP(w, r)
			return
		}
		if token == "" {
			writeProblem(w, problem{Type: problemBase + "not-found", Title: "Nicht gefunden",
				Status: http.StatusNotFound, Detail: "Die Agent-API ist nicht eingerichtet"})
			return
		}
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), want) != 1 {
			logger.Warn("agent-api: ungültiges oder fehlendes token", "method", r.Method, "path", r.URL.Path, "remote", r.RemoteAddr)
			w.Header().Set("WWW-Authenticate", `Bearer realm="agent"`)
			writeProblem(w, problem{Type: problemBase + "unauthorized", Title: "Nicht angemeldet",
				Status: http.StatusUnauthorized, Detail: "Gültiges Agent-Token erforderlich"})
			return
		}
		next.ServeHTTP(w, r)
	})
}
