package httpapi

import (
	"log/slog"
	"net/http"
	"strings"
	"time"
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

func logRequests(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		args := []any{"method", r.Method, "path", r.URL.Path, "status", rec.status, "duration", time.Since(start)}
		logger.Info("request", append(args, xffArgs(r)...)...)
	})
}

// maxXFFLen begrenzt den mitgeloggten X-Forwarded-For-Wert.
const maxXFFLen = 200

// xffArgs liefert "xff_untrusted" mit dem X-Forwarded-For-Header, falls vorhanden. Der Wert ist
// vom Client fälschbar und dient nur der Einordnung (hinter Funnel/nginx ist RemoteAddr immer nginx).
func xffArgs(r *http.Request) []any {
	v := strings.Join(r.Header.Values("X-Forwarded-For"), ", ")
	if v == "" {
		return nil
	}
	if len(v) > maxXFFLen {
		v = strings.ToValidUTF8(v[:maxXFFLen], "")
	}
	return []any{"xff_untrusted", v}
}
