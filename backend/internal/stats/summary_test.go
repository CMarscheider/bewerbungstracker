package stats

import (
	"math"
	"slices"
	"testing"
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
