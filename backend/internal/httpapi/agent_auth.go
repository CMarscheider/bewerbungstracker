package httpapi

import (
	"crypto/sha256"
	"crypto/subtle"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

// agentPrefix ist der Pfadpräfix der Agent-API.
const agentPrefix = "/api/agent/"

// requireAgentToken schützt die Agent-API: Ohne konfiguriertes Token gibt es sie nicht (404),
// sonst ist "Authorization: Bearer <token>" Pflicht (401, Warnung gedrosselt). Andere Pfade bleiben unberührt.
func requireAgentToken(token string, warner *warnThrottle, next http.Handler) http.Handler {
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
			warner.warn("agent-api: ungültiges oder fehlendes token",
				append([]any{"method", r.Method, "path", r.URL.Path, "remote", r.RemoteAddr}, xffArgs(r)...)...)
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

// authWarnInterval ist der Mindestabstand zwischen zwei Warnungen über ungültige Tokens.
const authWarnInterval = time.Minute

// warnThrottle schreibt höchstens eine Warnung je Intervall und zählt die unterdrückten mit,
// damit Fehlversuche aus dem Internet das Log nicht fluten.
type warnThrottle struct {
	logger     *slog.Logger
	interval   time.Duration
	now        func() time.Time
	mu         sync.Mutex
	last       time.Time
	suppressed int
}

func newWarnThrottle(logger *slog.Logger, interval time.Duration) *warnThrottle {
	return &warnThrottle{logger: logger, interval: interval, now: time.Now}
}

func (t *warnThrottle) warn(msg string, args ...any) {
	t.mu.Lock()
	now := t.now()
	if !t.last.IsZero() && now.Sub(t.last) < t.interval {
		t.suppressed++
		t.mu.Unlock()
		return
	}
	suppressed := t.suppressed
	t.last, t.suppressed = now, 0
	t.mu.Unlock()
	if suppressed > 0 {
		args = append(args, "suppressed", suppressed)
	}
	t.logger.Warn(msg, args...)
}
