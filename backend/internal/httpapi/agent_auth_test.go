package httpapi_test

import (
	"net/http"
	"testing"
)

func TestAgentAPIDisabledWithoutToken(t *testing.T) {
	srv := newAgentTestServer(t, "")
	expectProblem(t, callWith(t, srv, http.MethodGet, "/api/agent/cv", agentToken, nil), http.StatusNotFound, "/problems/not-found")
}

func TestAgentAPIRequiresToken(t *testing.T) {
	srv := newAgentTestServer(t, agentToken)
	r := callWith(t, srv, http.MethodGet, "/api/agent/cv", "", nil)
	expectProblem(t, r, http.StatusUnauthorized, "/problems/unauthorized")
	if r.Header.Get("WWW-Authenticate") == "" {
		t.Error("WWW-Authenticate fehlt")
	}
	expectProblem(t, callWith(t, srv, http.MethodGet, "/api/agent/cv", "falsch-falsch-falsch-falsch-falsch!", nil), http.StatusUnauthorized, "/problems/unauthorized")
}

func TestAgentGetCV(t *testing.T) {
	srv := newAgentTestServer(t, agentToken)
	expectProblem(t, callWith(t, srv, http.MethodGet, "/api/agent/cv", agentToken, nil), http.StatusNotFound, "/problems/not-found")

	expectStatus(t, call(t, srv, http.MethodPut, "/api/v1/cv", sampleCV()), http.StatusOK)
	r := callWith(t, srv, http.MethodGet, "/api/agent/cv", agentToken, nil)
	expectStatus(t, r, http.StatusOK)
	body := r.object(t)
	if body["person"].(map[string]any)["name"] != "Erika Muster" || body["updated_at"] == nil {
		t.Errorf("GET /api/agent/cv = %v", body)
	}
}

func TestUIAPIStaysWithoutToken(t *testing.T) {
	srv := newAgentTestServer(t, agentToken)
	expectStatus(t, call(t, srv, http.MethodGet, "/api/v1/cv", nil), http.StatusOK)
}

// rawGet schickt GET ohne Redirect-Folgen und mit beliebigem Authorization-Header (leer = keiner).
func rawGet(t *testing.T, url, auth string) (int, http.Header) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	return res.StatusCode, res.Header
}

func TestAgentAuthHeaderParsing(t *testing.T) {
	srv := newAgentTestServer(t, agentToken)
	expectStatus(t, call(t, srv, http.MethodPut, "/api/v1/cv", sampleCV()), http.StatusOK)
	cases := []struct {
		name, auth string
		want       int
	}{
		{"Bearer ohne Token", "Bearer", http.StatusUnauthorized},
		{"Bearer mit leerem Token", "Bearer ", http.StatusUnauthorized},
		{"Basic", "Basic xyz", http.StatusUnauthorized},
		{"Token ohne Schema", agentToken, http.StatusUnauthorized},
		{"kleingeschriebenes Schema", "bearer " + agentToken, http.StatusOK},
		{"korrekt", "Bearer " + agentToken, http.StatusOK},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status, header := rawGet(t, srv.URL+"/api/agent/cv", c.auth)
			if status != c.want {
				t.Errorf("Status %d, erwartet %d", status, c.want)
			}
			if c.want == http.StatusOK && header.Get("Cache-Control") != "no-store" {
				t.Errorf("Cache-Control = %q, erwartet no-store", header.Get("Cache-Control"))
			}
		})
	}
}

func TestAgentAuthUnknownAgentPathNeedsToken(t *testing.T) {
	srv := newAgentTestServer(t, agentToken)
	expectProblem(t, callWith(t, srv, http.MethodGet, "/api/agent/x", "", nil), http.StatusUnauthorized, "/problems/unauthorized")
}

func TestAgentAuthPathVariants(t *testing.T) {
	srv := newAgentTestServer(t, agentToken)
	expectStatus(t, call(t, srv, http.MethodPut, "/api/v1/cv", sampleCV()), http.StatusOK)
	if status, _ := rawGet(t, srv.URL+"/api//agent/cv", ""); status == http.StatusOK {
		t.Errorf("/api//agent/cv ohne Token: Status %d", status)
	}
	if status, _ := rawGet(t, srv.URL+"/api/agent/../v1/cv", ""); status != http.StatusUnauthorized {
		t.Errorf("/api/agent/../v1/cv ohne Token: Status %d, erwartet 401", status)
	}
}
