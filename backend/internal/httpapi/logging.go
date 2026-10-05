package httpapi

import (
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// Unwrap erlaubt http.ResponseController den Zugriff auf den eigentlichen Writer.
func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

// agentRejectInterval ist der Mindestabstand zwischen zwei Zusammenfassungen abgewiesener Agent-Anfragen.
const agentRejectInterval = time.Minute

// logRequests loggt jede Anfrage. Abgewiesene Agent-Anfragen (401, 429) werden stattdessen über
// rejects zusammengefasst, damit Anfragen aus dem Internet das Log nicht fluten.
func logRequests(logger *slog.Logger, rejects *logThrottle, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		if strings.HasPrefix(r.URL.Path, agentPrefix) {
			switch rec.status {
			case http.StatusTooManyRequests:
				rejects.summarize("throttled")
				return
			case http.StatusUnauthorized:
				rejects.summarize("unauthorized")
				return
			}
		}
		args := []any{"method", r.Method, "path", r.URL.Path, "status", rec.status, "duration", time.Since(start)}
		logger.Info("request", append(args, xffArgs(r)...)...)
	})
}

// maxXFFLen begrenzt den mitgeloggten X-Forwarded-For-Wert (in Bytes, ohne Kürzungszeichen).
const maxXFFLen = 200

// xffArgs liefert "xff_untrusted" mit dem X-Forwarded-For-Header, falls vorhanden. Der Wert ist
// vom Client fälschbar und dient nur der Einordnung (hinter Funnel/nginx ist RemoteAddr immer nginx).
// Gekürzt wird vorn: Die rechten Einträge haben die Proxys angehängt.
func xffArgs(r *http.Request) []any {
	v := strings.Join(r.Header.Values("X-Forwarded-For"), ", ")
	if v == "" {
		return nil
	}
	if len(v) > maxXFFLen {
		tail := v[len(v)-maxXFFLen:]
		// Angeschnittenes Zeichen am Anfang überspringen.
		for len(tail) > 0 && !utf8.RuneStart(tail[0]) {
			tail = tail[1:]
		}
		v = "…" + strings.ToValidUTF8(tail, "")
	}
	return []any{"xff_untrusted", v}
}

// logThrottle schreibt höchstens einen Logeintrag je Intervall und zählt die Ereignisse dazwischen
// je Schlüssel mit.
type logThrottle struct {
	logger   *slog.Logger
	interval time.Duration
	now      func() time.Time
	mu       sync.Mutex
	last     time.Time
	counts   map[string]int
}

func newLogThrottle(logger *slog.Logger, interval time.Duration) *logThrottle {
	return &logThrottle{logger: logger, interval: interval, now: time.Now, counts: map[string]int{}}
}

// tick zählt ein Ereignis zu key. Ist ein Eintrag fällig, liefert es die Zähler seit dem letzten
// Eintrag (einschließlich dieses Ereignisses) und setzt sie zurück.
func (t *logThrottle) tick(key string) (map[string]int, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.counts[key]++
	now := t.now()
	if !t.last.IsZero() && now.Sub(t.last) < t.interval {
		return nil, false
	}
	counts := t.counts
	t.last, t.counts = now, map[string]int{}
	return counts, true
}

// summarize zählt eine abgewiesene Agent-Anfrage (key "throttled" oder "unauthorized").
func (t *logThrottle) summarize(key string) {
	if counts, ok := t.tick(key); ok {
		t.logger.Info("agent-api: abgewiesene Anfragen",
			"throttled", counts["throttled"], "unauthorized", counts["unauthorized"])
	}
}
