package httpapi

import (
	"net/http"
	"strings"
	"time"

	"golang.org/x/time/rate"
)

// Drossel der Agent-API: global statt je IP, weil Funnel und nginx die Client-Adresse verdecken
// und es nur einen legitimen Client gibt.
// rate.Every ist eine Funktion, daher var statt const.
var agentRate = rate.Every(500 * time.Millisecond) // 2 Anfragen/s

const agentBurst = 20

// newAgentLimiter erzeugt den Token-Bucket für die Agent-API.
func newAgentLimiter(r rate.Limit, burst int) *rate.Limiter {
	return rate.NewLimiter(r, burst)
}

// limitAgent beantwortet Anfragen unter /api/agent/ über dem Limit mit 429; sie zählen auch ohne Token.
func limitAgent(l *rate.Limiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, agentPrefix) && !l.Allow() {
			// Bei 2 Anfragen/s ist nach spätestens einer Sekunde wieder Platz.
			w.Header().Set("Retry-After", "1")
			writeProblem(w, problem{Type: problemBase + "too-many-requests", Title: "Zu viele Anfragen",
				Status: http.StatusTooManyRequests, Detail: "Bitte später erneut versuchen"})
			return
		}
		next.ServeHTTP(w, r)
	})
}
