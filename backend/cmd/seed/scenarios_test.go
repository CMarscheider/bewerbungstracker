package main

import (
	"testing"
	"time"

	"bewerbungsmanager/internal/domain"
)

var testToday = domain.DateOf(time.Date(2026, 10, 3, 12, 0, 0, 0, time.Local))

// Jedes Szenario muss die echten Domainregeln erfüllen, sonst lehnt die API es ab.
func TestScenariosAreValid(t *testing.T) {
	for _, s := range scenarios() {
		var history []domain.Event
		for i, st := range s.Steps {
			next := st.event(testToday)
			if err := domain.CanApply(history, next, testToday); err != nil {
				t.Errorf("%s / %s, Schritt %d (%s): %v", s.Company, s.Position, i, st.Type, err)
				break
			}
			// Der Seed erfasst alles heute: Erfassungsdatum = heute.
			history = append(history, domain.Event{Type: next.Type, OccurredOn: next.OccurredOn, RecordedOn: testToday})
		}
	}
}

// Die Demo-Daten sollen jede Ansicht sinnvoll füllen.
func TestScenariosCoverAllViews(t *testing.T) {
	all := scenarios()
	if len(all) < 25 {
		t.Errorf("nur %d Szenarien, erwartet mindestens 25", len(all))
	}

	has := map[string]bool{}
	sources := map[string]bool{}
	perCompany := map[string]int{}
	for _, s := range all {
		sources[s.Source] = true
		perCompany[s.Company]++
		last := s.Steps[len(s.Steps)-1]
		has[string(last.Type)] = true
		if last.DueDay != nil && *last.DueDay < 0 {
			has["überfällige Frist"] = true
		}
		if last.DueDay != nil && *last.DueDay >= 0 && *last.DueDay <= 7 {
			has["Frist in 7 Tagen"] = true
		}
		if last.Type.AllowsFutureDate() && last.Day >= 0 && last.Day <= 14 {
			has["Termin in 14 Tagen"] = true
		}
	}
	for _, want := range []string{
		"überfällige Frist", "Frist in 7 Tagen", "Termin in 14 Tagen",
		"Vorgemerkt", "Beworben", "AngebotErhalten", "AngebotAngenommen", "AngebotAbgelehnt",
		"Absage", "Zurueckgezogen", "KeineRueckmeldung",
	} {
		if !has[want] {
			t.Errorf("Demo-Daten decken %q nicht ab", want)
		}
	}
	if len(sources) < 4 {
		t.Errorf("nur %d Quellen, erwartet mindestens 4", len(sources))
	}
	multi := false
	for _, n := range perCompany {
		if n > 1 {
			multi = true
		}
	}
	if !multi {
		t.Error("mindestens eine Firma soll mehrere Bewerbungen haben")
	}
}
