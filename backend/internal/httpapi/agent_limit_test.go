package httpapi

import (
	"bufio"
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"golang.org/x/time/rate"
)

// logLines zerlegt JSON-Logausgaben in einzelne Einträge.
func logLines(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var out []map[string]any
	sc := bufio.NewScanner(bytes.NewReader(buf.Bytes()))
	for sc.Scan() {
		var m map[string]any
		if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
			t.Fatalf("kein JSON: %s", sc.Bytes())
		}
		out = append(out, m)
	}
	return out
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
}

const testToken = "richtiges-token"

// agentReq schickt eine Anfrage an h, optional mit Authorization-Header.
func agentReq(h http.Handler, path, auth string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func limitedAgent(authBurst, anonBurst int) http.Handler {
	limits := agentLimits{
		auth: rate.NewLimiter(rate.Every(time.Hour), authBurst),
		anon: rate.NewLimiter(rate.Every(time.Hour), anonBurst),
	}
	return requireAgentToken(testToken, limits, newLogThrottle(slog.New(slog.DiscardHandler), authWarnInterval), okHandler())
}

func expectTooMany(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("Status %d, erwartet 429", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Errorf("Content-Type = %q", ct)
	}
	if ra := rec.Header().Get("Retry-After"); ra != "1" {
		t.Errorf("Retry-After = %q, erwartet 1", ra)
	}
	var p map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if p["type"] != problemBase+"too-many-requests" || p["title"] != "Zu viele Anfragen" {
		t.Errorf("Problem = %v", p)
	}
}

func TestAgentAnonFloodDoesNotLockOutValidToken(t *testing.T) {
	h := limitedAgent(3, 3)
	for _, auth := range []string{"", "Bearer falsch", "Basic xyz"} {
		if rec := agentReq(h, "/api/agent/cv", auth); rec.Code != http.StatusUnauthorized {
			t.Fatalf("%q: Status %d, erwartet 401", auth, rec.Code)
		}
	}
	// Fremde Anfragen über dem Limit: 429 statt 401.
	expectTooMany(t, agentReq(h, "/api/agent/cv", "Bearer falsch"))
	expectTooMany(t, agentReq(h, "/api/agent/cv", ""))
	if rec := agentReq(h, "/api/agent/cv", "Bearer "+testToken); rec.Code != http.StatusOK {
		t.Fatalf("gültiges Token nach Flut: Status %d, erwartet 200", rec.Code)
	}
}

func TestAgentAuthLimitRejectsAfterBurst(t *testing.T) {
	h := limitedAgent(3, 3)
	for i := range 3 {
		if rec := agentReq(h, "/api/agent/cv", "Bearer "+testToken); rec.Code != http.StatusOK {
			t.Fatalf("Anfrage %d: Status %d, erwartet 200", i+1, rec.Code)
		}
	}
	expectTooMany(t, agentReq(h, "/api/agent/cv", "Bearer "+testToken))
	// Das fremde Kontingent ist davon unberührt.
	if rec := agentReq(h, "/api/agent/cv", "Bearer falsch"); rec.Code != http.StatusUnauthorized {
		t.Errorf("fremde Anfrage: Status %d, erwartet 401", rec.Code)
	}
}

func TestAgentLimitIgnoresOtherPaths(t *testing.T) {
	h := limitedAgent(1, 1)
	for i := range 5 {
		if rec := agentReq(h, "/api/v1/cv", ""); rec.Code != http.StatusOK {
			t.Fatalf("Anfrage %d: Status %d, erwartet 200", i+1, rec.Code)
		}
	}
}

func TestAgentLimitDefaults(t *testing.T) {
	if agentRate != rate.Every(500*time.Millisecond) || agentBurst != 20 {
		t.Errorf("agentRate = %v, agentBurst = %d", agentRate, agentBurst)
	}
	if agentAnonRate != rate.Every(time.Second) || agentAnonBurst != 10 {
		t.Errorf("agentAnonRate = %v, agentAnonBurst = %d", agentAnonRate, agentAnonBurst)
	}
	if authWarnInterval != time.Minute {
		t.Errorf("authWarnInterval = %v", authWarnInterval)
	}
}

// fakeClock ist eine von Hand weitergestellte Uhr.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

func TestAgentAuthWarningIsThrottled(t *testing.T) {
	var logs bytes.Buffer
	clock := &fakeClock{t: time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)}
	warner := newLogThrottle(slog.New(slog.NewJSONHandler(&logs, nil)), authWarnInterval)
	warner.now = clock.now
	// Ungedrosselt, damit alle 50 Fehlversuche die Token-Prüfung erreichen.
	unlimited := agentLimits{auth: rate.NewLimiter(rate.Inf, 0), anon: rate.NewLimiter(rate.Inf, 0)}
	h := requireAgentToken(testToken, unlimited, warner, okHandler())

	bad := func() {
		req := httptest.NewRequest(http.MethodGet, "/api/agent/cv", nil)
		req.Header.Set("Authorization", "Bearer falsch")
		req.Header.Set("X-Forwarded-For", "203.0.113.7")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("Status %d, erwartet 401", rec.Code)
		}
	}
	for range 50 {
		bad()
	}
	lines := logLines(t, &logs)
	if len(lines) != 1 {
		t.Fatalf("%d Warnungen, erwartet 1: %s", len(lines), logs.String())
	}
	if lines[0]["level"] != "WARN" || lines[0]["xff_untrusted"] != "203.0.113.7" {
		t.Errorf("Warnung = %v", lines[0])
	}
	if _, ok := lines[0]["suppressed"]; ok {
		t.Errorf("erste Warnung enthält suppressed: %v", lines[0])
	}

	clock.advance(authWarnInterval)
	bad()
	lines = logLines(t, &logs)
	if len(lines) != 2 {
		t.Fatalf("%d Warnungen, erwartet 2: %s", len(lines), logs.String())
	}
	if lines[1]["suppressed"] != float64(49) {
		t.Errorf("suppressed = %v, erwartet 49", lines[1]["suppressed"])
	}
}

func TestAgentThrottledRequestsDoNotWarn(t *testing.T) {
	var logs bytes.Buffer
	limits := agentLimits{auth: rate.NewLimiter(rate.Every(time.Hour), 1), anon: rate.NewLimiter(rate.Every(time.Hour), 1)}
	warner := newLogThrottle(slog.New(slog.NewJSONHandler(&logs, nil)), authWarnInterval)
	h := requireAgentToken(testToken, limits, warner, okHandler())
	agentReq(h, "/api/agent/cv", "Bearer falsch")
	expectTooMany(t, agentReq(h, "/api/agent/cv", "Bearer falsch"))
	expectTooMany(t, agentReq(h, "/api/agent/cv", "Bearer falsch"))
	if n := len(logLines(t, &logs)); n != 1 {
		t.Fatalf("%d Logzeilen, erwartet 1: %s", n, logs.String())
	}
	if n := warner.pending(warnKey); n != 0 {
		t.Errorf("unterdrückt = %d, gedrosselte Anfragen sollen nicht zählen", n)
	}
}

// pending liest einen Zähler unter dem Mutex (nur für Tests).
func (t *logThrottle) pending(key string) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.counts[key]
}

func newTestLogRequests(logs *bytes.Buffer, clock *fakeClock, next http.Handler) http.Handler {
	logger := slog.New(slog.NewJSONHandler(logs, nil))
	rejects := newLogThrottle(logger, agentRejectInterval)
	if clock != nil {
		rejects.now = clock.now
	}
	return logRequests(logger, rejects, next)
}

func TestLogRequestsXForwardedFor(t *testing.T) {
	var logs bytes.Buffer
	h := newTestLogRequests(&logs, nil, okHandler())

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/cv", nil))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/cv", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.7, 10.0.0.1")
	h.ServeHTTP(httptest.NewRecorder(), req)
	long := httptest.NewRequest(http.MethodGet, "/api/v1/cv", nil)
	long.Header.Set("X-Forwarded-For", strings.Repeat("a", 500)+", 198.51.100.9")
	h.ServeHTTP(httptest.NewRecorder(), long)
	// Mehrbyte-Zeichen, damit der Schnitt mitten in einem Zeichen liegt.
	multi := httptest.NewRequest(http.MethodGet, "/api/v1/cv", nil)
	multi.Header.Set("X-Forwarded-For", strings.Repeat("ä", 150)+"x")
	h.ServeHTTP(httptest.NewRecorder(), multi)

	lines := logLines(t, &logs)
	if len(lines) != 4 {
		t.Fatalf("%d Logzeilen: %s", len(lines), logs.String())
	}
	if _, ok := lines[0]["xff_untrusted"]; ok {
		t.Errorf("ohne Header: xff_untrusted vorhanden: %v", lines[0])
	}
	if lines[1]["xff_untrusted"] != "203.0.113.7, 10.0.0.1" {
		t.Errorf("xff_untrusted = %v", lines[1]["xff_untrusted"])
	}
	got, _ := lines[2]["xff_untrusted"].(string)
	if want := "…" + (strings.Repeat("a", 500) + ", 198.51.100.9")[514-200:]; got != want {
		t.Errorf("xff_untrusted = %q, erwartet Ende mit Präfix …: %q", got, want)
	}
	got, _ = lines[3]["xff_untrusted"].(string)
	if !strings.HasPrefix(got, "…") || !strings.HasSuffix(got, "äx") || len(got) > len("…")+200 {
		t.Errorf("xff_untrusted = %q", got)
	}
	if !utf8.ValidString(got) || strings.ContainsRune(got, utf8.RuneError) {
		t.Errorf("xff_untrusted kein gültiges UTF-8: %q", got)
	}
}

func TestXFFTailIsValidUTF8(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Forwarded-For", strings.Repeat("ä", 150)+"x")
	v := xffArgs(r)[1].(string)
	if !utf8.ValidString(v) || v != "…"+strings.Repeat("ä", 99)+"x" {
		t.Errorf("xffArgs = %q", v)
	}
}

func TestLogRequestsAggregatesAgentRejections(t *testing.T) {
	var logs bytes.Buffer
	clock := &fakeClock{t: time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)}
	status := http.StatusUnauthorized
	h := newTestLogRequests(&logs, clock, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
	}))
	send := func(path string) { h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, path, nil)) }

	for range 30 {
		send("/api/agent/cv")
	}
	status = http.StatusTooManyRequests
	for range 20 {
		send("/api/agent/cv")
	}
	// Andere Pfade und Status bleiben einzeln geloggt.
	send("/api/v1/cv")
	status = http.StatusOK
	send("/api/agent/cv")

	lines := logLines(t, &logs)
	if len(lines) != 3 {
		t.Fatalf("%d Logzeilen, erwartet 3: %s", len(lines), logs.String())
	}
	if lines[0]["msg"] != "agent-api: abgewiesene Anfragen" || lines[0]["unauthorized"] != float64(1) || lines[0]["throttled"] != float64(0) {
		t.Errorf("erste Zusammenfassung = %v", lines[0])
	}
	if lines[1]["msg"] != "request" || lines[1]["path"] != "/api/v1/cv" || lines[1]["status"] != float64(429) {
		t.Errorf("Zeile 2 = %v", lines[1])
	}
	if lines[2]["msg"] != "request" || lines[2]["status"] != float64(200) {
		t.Errorf("Zeile 3 = %v", lines[2])
	}

	clock.advance(agentRejectInterval)
	status = http.StatusTooManyRequests
	send("/api/agent/cv")
	lines = logLines(t, &logs)
	if len(lines) != 4 {
		t.Fatalf("%d Logzeilen, erwartet 4: %s", len(lines), logs.String())
	}
	if lines[3]["unauthorized"] != float64(29) || lines[3]["throttled"] != float64(21) {
		t.Errorf("zweite Zusammenfassung = %v", lines[3])
	}
}

func TestAgentDisabledReturns404BeforeThrottling(t *testing.T) {
	limits := agentLimits{auth: rate.NewLimiter(rate.Every(time.Hour), 1), anon: rate.NewLimiter(rate.Every(time.Hour), 1)}
	h := requireAgentToken("", limits, newLogThrottle(slog.New(slog.DiscardHandler), authWarnInterval), okHandler())
	for i := range 5 {
		if rec := agentReq(h, "/api/agent/cv", "Bearer x"); rec.Code != http.StatusNotFound {
			t.Fatalf("Anfrage %d: Status %d, erwartet 404", i+1, rec.Code)
		}
	}
}

func TestAgentWarningOmitsPresentedToken(t *testing.T) {
	var logs bytes.Buffer
	unlimited := agentLimits{auth: rate.NewLimiter(rate.Inf, 0), anon: rate.NewLimiter(rate.Inf, 0)}
	h := requireAgentToken(testToken, unlimited, newLogThrottle(slog.New(slog.NewJSONHandler(&logs, nil)), authWarnInterval), okHandler())
	agentReq(h, "/api/agent/cv", "Bearer geheimes-falsches-token")
	if logs.Len() == 0 {
		t.Fatal("keine Warnung geschrieben")
	}
	if strings.Contains(logs.String(), "geheimes-falsches-token") {
		t.Errorf("Warnung enthält das Token: %s", logs.String())
	}
}
