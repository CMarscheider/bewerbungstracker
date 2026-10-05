package httpapi

import (
	"crypto/sha256"
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
	want := sha256.Sum256([]byte(token))
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
		if !validBearer(r.Header.Get("Authorization"), want) {
			logger.Warn("agent-api: ungültiges oder fehlendes token", "method", r.Method, "path", r.URL.Path, "remote", r.RemoteAddr)
			w.Header().Set("WWW-Authenticate", `Bearer realm="agent"`)
			writeProblem(w, problem{Type: problemBase + "unauthorized", Title: "Nicht angemeldet",
				Status: http.StatusUnauthorized, Detail: "Gültiges Agent-Token erforderlich"})
			return
		}
		// Antworten enthalten personenbezogene Daten.
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

// validBearer prüft "Bearer <token>" (Schema ohne Beachtung der Groß-/Kleinschreibung) gegen den
// SHA-256-Hash des konfigurierten Tokens; der Hash-Vergleich verrät die Token-Länge nicht.
func validBearer(header string, want [sha256.Size]byte) bool {
	scheme, presented, ok := strings.Cut(header, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return false
	}
	got := sha256.Sum256([]byte(presented))
	return subtle.ConstantTimeCompare(got[:], want[:]) == 1
}
