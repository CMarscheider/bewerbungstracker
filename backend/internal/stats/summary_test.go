package stats

import (
	"math"
	"slices"
	"testing"

	"bewerbungsmanager/internal/domain"
)

func TestSummarize(t *testing.T) {
	s := Summarize(sampleApps())

	if s.Applied != 4 || s.Responded != 3 {
		t.Fatalf("Applied=%d Responded=%d, erwartet 4/3", s.Applied, s.Responded)
	}
	if math.Abs(s.ResponseRate-0.75) > 1e-9 {
		t.Errorf("ResponseRate = %v", s.ResponseRate)
	}
	// Antwortzeiten: 10 (Absage), 4 (Interview), 2 (Challenge)
	if s.MedianDaysToResponse == nil || *s.MedianDaysToResponse != 4 {
		t.Errorf("Median = %v, erwartet 4", s.MedianDaysToResponse)
	}
	if s.AvgDaysToResponse == nil || math.Abs(*s.AvgDaysToResponse-16.0/3.0) > 1e-9 {
		t.Errorf("Durchschnitt = %v, erwartet 5.33", s.AvgDaysToResponse)
	}

	wantRejections := []Count{{Key: "Beworben", Count: 1}, {Key: "ChallengeAbgegeben", Count: 1}}
	if !slices.Equal(s.RejectionsAfter, wantRejections) {
		t.Errorf("RejectionsAfter = %v, erwartet %v", s.RejectionsAfter, wantRejections)
	}

	wantSources := []SourceStats{
		{Source: "LinkedIn", Applied: 2, ReachedInterview: 1, ReachedOffer: 1},
		{Source: "Empfehlung", Applied: 1},
		{Source: UnknownSource, Applied: 1},
	}
	if !slices.Equal(s.BySource, wantSources) {
		t.Errorf("BySource = %+v, erwartet %+v", s.BySource, wantSources)
	}
}

func TestSummarizeEmpty(t *testing.T) {
	s := Summarize(nil)
	if s.Applied != 0 || s.ResponseRate != 0 || s.MedianDaysToResponse != nil || s.AvgDaysToResponse != nil {
		t.Errorf("leere Zusammenfassung: %+v", s)
	}
	if s.RejectionsAfter == nil || s.BySource == nil {
		t.Error("Listen müssen leer, aber nicht nil sein (JSON: [] statt null)")
	}
}

func TestMedianEven(t *testing.T) {
	if got := median([]float64{4, 1, 3, 2}); got != 2.5 {
		t.Errorf("median = %v, erwartet 2.5", got)
	}
}

func TestSummarizeResponseTime(t *testing.T) {
	tests := []struct {
		name     string
		events   []domain.Event
		responds bool
		days     float64
	}{
		{"Einladung zählt, nicht der Termin", []domain.Event{
			e(domain.Beworben, "2026-09-01"),
			eRec(domain.Interview, "2026-10-20", "2026-10-01"),
		}, true, 30},
		{"späte Antwort nach Ghosting", []domain.Event{
			e(domain.Beworben, "2026-09-01"),
			e(domain.KeineRueckmeldung, "2026-09-20"),
			e(domain.Absage, "2026-09-25"),
		}, true, 24},
		{"Zurückgezogen ist keine Antwort", []domain.Event{
			e(domain.Beworben, "2026-09-01"),
			e(domain.Zurueckgezogen, "2026-09-05"),
		}, false, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Summarize([]Application{app(nil, tt.events...)})
			if s.Applied != 1 {
				t.Fatalf("Applied = %d", s.Applied)
			}
			if !tt.responds {
				if s.Responded != 0 || s.MedianDaysToResponse != nil {
					t.Errorf("keine Antwort erwartet: %+v", s)
				}
				return
			}
			if s.Responded != 1 || s.MedianDaysToResponse == nil || *s.MedianDaysToResponse != tt.days {
				t.Errorf("Median = %v, erwartet %v", s.MedianDaysToResponse, tt.days)
			}
		})
	}
}

func TestSummarizeRejectionAfterGhosting(t *testing.T) {
	s := Summarize([]Application{app(nil,
		e(domain.Beworben, "2026-09-01"), e(domain.Interview, "2026-09-05"),
		e(domain.KeineRueckmeldung, "2026-09-20"), e(domain.Absage, "2026-09-25"))})
	want := []Count{{Key: "Interview", Count: 1}}
	if !slices.Equal(s.RejectionsAfter, want) {
		t.Errorf("RejectionsAfter = %v, erwartet %v", s.RejectionsAfter, want)
	}
}

func TestSummarizeSourceNormalization(t *testing.T) {
	b := e(domain.Beworben, "2026-09-01")
	s := Summarize([]Application{
		app(str("  "), b), app(str(""), b), app(str(" LinkedIn "), b), app(str("LinkedIn"), b),
	})
	want := []SourceStats{
		{Source: "LinkedIn", Applied: 2},
		{Source: UnknownSource, Applied: 2},
	}
	if !slices.Equal(s.BySource, want) {
		t.Errorf("BySource = %+v, erwartet %+v", s.BySource, want)
	}
}

func TestSummarizeExcludesVorgemerktThenZurueckgezogen(t *testing.T) {
	s := Summarize([]Application{app(nil, e(domain.Vorgemerkt, "2026-09-01"), e(domain.Zurueckgezogen, "2026-09-02"))})
	if s.Applied != 0 || len(s.BySource) != 0 {
		t.Errorf("nicht in der Basis erwartet: %+v", s)
	}
}
