package httpapi

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"strings"
	"time"
)

// agentPrefix ist der Pfadpräfix der Agent-API.
const agentPrefix = "/api/agent/"

// mailScopePatterns sind die Agent-Endpunkte, die das eingeschränkte Postfach-Token (AGENT_TOKEN_MAIL)
// aufrufen darf. Die Routine liest fremde Mails; ein manipuliertes Prompt soll damit weder Stellen anlegen
// noch Unterlagen oder Lebenslauf-Vorschläge abliefern können.
var mailScopePatterns = []string{
	"GET /api/agent/cv",
	"GET /api/agent/applications/open",
	"POST /api/agent/applications/{id}/events",
	"PUT /api/agent/applications/{id}/gmail-thread",
	"POST /api/agent/suggestions",
	"GET /api/agent/processed-mails/{messageId}",
	"POST /api/agent/processed-mails",
}

// newMailScope liefert eine Prüfung, ob eine Anfrage im Umfang des Postfach-Tokens liegt. Sie nutzt
// ServeMux-Muster, damit Pfadbereinigung (z. B. "..") genauso greift wie beim eigentlichen Routing.
func newMailScope() func(*http.Request) bool {
	mux := http.NewServeMux()
	for _, p := range mailScopePatterns {
		mux.Handle(p, http.NotFoundHandler())
	}
	return func(r *http.Request) bool {
		_, pattern := mux.Handler(r)
		return pattern != ""
	}
}

// requireAgentToken ist requireAgentTokens ohne Postfach-Token.
func requireAgentToken(token string, limits agentLimits, warner *logThrottle, next http.Handler) http.Handler {
	return requireAgentTokens(token, "", limits, warner, next)
}

// requireAgentTokens schützt und drosselt die Agent-API: Ohne konfiguriertes Token gibt es sie nicht (404, ungedrosselt),
// sonst ist "Authorization: Bearer <token>" Pflicht (401, Warnung gedrosselt). Anfragen mit gültigem
// Token zählen gegen limits.auth, alle anderen gegen limits.anon; über dem Limit gibt es 429 ohne
// Warnung. Andere Pfade bleiben unberührt. mailToken (optional) ist gültig, darf aber nur die Endpunkte
// aus mailScopePatterns aufrufen, sonst 403; es teilt sich das Kontingent mit token.
func requireAgentTokens(token, mailToken string, limits agentLimits, warner *logThrottle, next http.Handler) http.Handler {
	want := sha256.Sum256([]byte(token))
	wantMail := sha256.Sum256([]byte(mailToken))
	inMailScope := newMailScope()
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
		auth := r.Header.Get("Authorization")
		full := validBearer(auth, want)
		mail := mailToken != "" && validBearer(auth, wantMail)
		valid := full || mail
		limiter := limits.anon
		if valid {
			limiter = limits.auth
		}
		if !limiter.Allow() {
			writeTooManyRequests(w)
			return
		}
		if !valid {
			warner.warn("agent-api: ungültiges oder fehlendes token",
				append([]any{"method", r.Method, "path", r.URL.Path, "remote", r.RemoteAddr}, xffArgs(r)...)...)
			w.Header().Set("WWW-Authenticate", `Bearer realm="agent"`)
			writeProblem(w, problem{Type: problemBase + "unauthorized", Title: "Nicht angemeldet",
				Status: http.StatusUnauthorized, Detail: "Gültiges Agent-Token erforderlich"})
			return
		}
		if !full && !inMailScope(r) {
			writeProblem(w, problem{Type: problemBase + "forbidden", Title: "Nicht erlaubt",
				Status: http.StatusForbidden, Detail: "Dieses Token darf diesen Endpunkt nicht aufrufen"})
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

// warnKey ist der Zählerschlüssel der Token-Warnung in logThrottle.
const warnKey = "warn"

// warn schreibt höchstens eine Warnung je Intervall; die Zahl der unterdrückten steht in "suppressed".
func (t *logThrottle) warn(msg string, args ...any) {
	counts, ok := t.tick(warnKey)
	if !ok {
		return
	}
	if n := counts[warnKey] - 1; n > 0 {
		args = append(args, "suppressed", n)
	}
	t.logger.Warn(msg, args...)
}
