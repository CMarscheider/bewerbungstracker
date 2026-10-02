// Command seed legt Demo-Daten über die REST-API an, damit alle Fachregeln greifen.
//
//	go run ./cmd/seed -url http://localhost:4200
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"bewerbungsmanager/internal/domain"
)

func main() {
	baseURL := flag.String("url", "http://localhost:4200", "Basis-URL der App (Frontend-Proxy oder Backend)")
	flag.Parse()

	if err := run(*baseURL, domain.DateOf(time.Now())); err != nil {
		fmt.Fprintln(os.Stderr, "seed:", err)
		os.Exit(1)
	}
}

func run(baseURL string, today time.Time) error {
	api := &apiClient{base: baseURL, http: &http.Client{Timeout: 10 * time.Second}}

	var existing []struct{ ID string }
	if err := api.do(http.MethodGet, "/api/v1/companies", nil, &existing); err != nil {
		return fmt.Errorf("API nicht erreichbar (läuft der Stack? `task up`): %w", err)
	}
	if len(existing) > 0 {
		return errors.New("es gibt bereits Daten; Demo-Daten nur in eine leere Datenbank laden (`docker compose down -v` setzt zurück)")
	}

	companyIDs := map[string]string{}
	events := 0
	for _, s := range scenarios() {
		companyID, ok := companyIDs[s.Company]
		if !ok {
			var created struct {
				ID string `json:"id"`
			}
			if err := api.do(http.MethodPost, "/api/v1/companies", map[string]any{"name": s.Company}, &created); err != nil {
				return fmt.Errorf("Firma %s: %w", s.Company, err)
			}
			companyID = created.ID
			companyIDs[s.Company] = companyID
		}

		var app struct {
			ID string `json:"id"`
		}
		body := map[string]any{
			"company_id":     companyID,
			"position_title": s.Position,
			"location":       s.Location,
			"source":         s.Source,
			"first_event":    eventPayload(s.Steps[0], today),
		}
		if err := api.do(http.MethodPost, "/api/v1/applications", body, &app); err != nil {
			return fmt.Errorf("Bewerbung %s / %s: %w", s.Company, s.Position, err)
		}
		events++

		for _, st := range s.Steps[1:] {
			if err := api.do(http.MethodPost, "/api/v1/applications/"+app.ID+"/events", eventPayload(st, today), nil); err != nil {
				return fmt.Errorf("Ereignis %s bei %s / %s: %w", st.Type, s.Company, s.Position, err)
			}
			events++
		}
	}

	fmt.Printf("Demo-Daten angelegt: %d Firmen, %d Bewerbungen, %d Ereignisse.\n", len(companyIDs), len(scenarios()), events)
	return nil
}

func eventPayload(s step, today time.Time) map[string]any {
	e := s.event(today)
	p := map[string]any{"type": string(e.Type), "occurred_on": e.OccurredOn.Format(time.DateOnly)}
	if e.DueOn != nil {
		p["due_on"] = e.DueOn.Format(time.DateOnly)
	}
	if e.Note != nil {
		p["note"] = *e.Note
	}
	return p
}

type apiClient struct {
	base string
	http *http.Client
}

// do sendet body als JSON und dekodiert die Antwort nach out (falls nicht nil).
func (c *apiClient) do(method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, c.base+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	data, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if res.StatusCode >= 300 {
		return fmt.Errorf("%s %s: HTTP %d: %s", method, path, res.StatusCode, bytes.TrimSpace(data))
	}
	if out != nil {
		return json.Unmarshal(data, out)
	}
	return nil
}
