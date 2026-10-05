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
		auth: newAgentLimiter(rate.Every(time.Hour), authBurst),
		anon: newAgentLimiter(rate.Every(time.Hour), anonBurst),
	}
	return requireAgentToken(testToken, limits, newWarnThrottle(slog.New(slog.DiscardHandler), authWarnInterval), okHandler())
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
	warner := newWarnThrottle(slog.New(slog.NewJSONHandler(&logs, nil)), authWarnInterval)
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
	limits := agentLimits{auth: newAgentLimiter(rate.Every(time.Hour), 1), anon: newAgentLimiter(rate.Every(time.Hour), 1)}
	warner := newWarnThrottle(slog.New(slog.NewJSONHandler(&logs, nil)), authWarnInterval)
	h := requireAgentToken(testToken, limits, warner, okHandler())
	agentReq(h, "/api/agent/cv", "Bearer falsch")
	expectTooMany(t, agentReq(h, "/api/agent/cv", "Bearer falsch"))
	expectTooMany(t, agentReq(h, "/api/agent/cv", "Bearer falsch"))
	if n := len(logLines(t, &logs)); n != 1 {
		t.Fatalf("%d Logzeilen, erwartet 1: %s", n, logs.String())
	}
	if warner.suppressed != 0 {
		t.Errorf("suppressed = %d, gedrosselte Anfragen sollen nicht zählen", warner.suppressed)
	}
}

func TestLogRequestsXForwardedFor(t *testing.T) {
	var logs bytes.Buffer
	h := logRequests(slog.New(slog.NewJSONHandler(&logs, nil)), okHandler())

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/cv", nil))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/cv", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.7, 10.0.0.1")
	h.ServeHTTP(httptest.NewRecorder(), req)
	long := httptest.NewRequest(http.MethodGet, "/api/v1/cv", nil)
	long.Header.Set("X-Forwarded-For", strings.Repeat("a", 500))
	h.ServeHTTP(httptest.NewRecorder(), long)

	lines := logLines(t, &logs)
	if len(lines) != 3 {
		t.Fatalf("%d Logzeilen: %s", len(lines), logs.String())
	}
	if _, ok := lines[0]["xff_untrusted"]; ok {
		t.Errorf("ohne Header: xff_untrusted vorhanden: %v", lines[0])
	}
	if lines[1]["xff_untrusted"] != "203.0.113.7, 10.0.0.1" {
		t.Errorf("xff_untrusted = %v", lines[1]["xff_untrusted"])
	}
	if got, _ := lines[2]["xff_untrusted"].(string); got != strings.Repeat("a", 200) {
		t.Errorf("xff_untrusted nicht auf 200 Zeichen gekürzt: Länge %d", len(got))
	}
}
