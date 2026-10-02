package stats

import (
	"cmp"
	"slices"
	"strings"

	"bewerbungsmanager/internal/domain"
)

// UnknownSource ist der Schlüssel für Bewerbungen ohne Quelle.
const UnknownSource = "Unbekannt"

// Count ist eine Anzahl je Schlüssel.
type Count struct {
	Key   string
	Count int
}

// SourceStats fasst den Erfolg je Quelle zusammen.
type SourceStats struct {
	Source           string
	Applied          int
	ReachedInterview int
	ReachedOffer     int
}

// Summary enthält die Kennzahlen über alle Bewerbungen mit "Beworben".
type Summary struct {
	Applied              int
	Responded            int
	ResponseRate         float64
	MedianDaysToResponse *float64
	AvgDaysToResponse    *float64
	RejectionsAfter      []Count // Absagen nach Typ des vorherigen Ereignisses
	BySource             []SourceStats
}

// Summarize berechnet Antwortzeiten, Absagen je Phase und Erfolg je Quelle.
func Summarize(apps []Application) Summary {
	s := Summary{RejectionsAfter: []Count{}, BySource: []SourceStats{}}
	var days []float64
	rejections := map[domain.EventType]int{}
	bySource := map[string]*SourceStats{}

	for _, a := range apps {
		idx, ok := appliedIndex(a.Events)
		if !ok {
			continue
		}
		s.Applied++
		if n, ok := daysToFirstResponse(a.Events, idx); ok {
			s.Responded++
			days = append(days, n)
		}
		for i := 1; i < len(a.Events); i++ {
			if a.Events[i].Type == domain.Absage {
				rejections[precedingType(a.Events, i)]++
			}
		}

		src := UnknownSource
		if a.Source != nil && strings.TrimSpace(*a.Source) != "" {
			src = strings.TrimSpace(*a.Source)
		}
		st := bySource[src]
		if st == nil {
			st = &SourceStats{Source: src}
			bySource[src] = st
		}
		st.Applied++
		m := maxOrder(a.Events)
		if m >= domain.Interview.Order() {
			st.ReachedInterview++
		}
		if m >= domain.AngebotErhalten.Order() {
			st.ReachedOffer++
		}
	}

	s.ResponseRate = rate(s.Responded, s.Applied)
	if len(days) > 0 {
		med, avg := median(days), mean(days)
		s.MedianDaysToResponse, s.AvgDaysToResponse = &med, &avg
	}
	for _, t := range domain.AllEventTypes() {
		if n := rejections[t]; n > 0 {
			s.RejectionsAfter = append(s.RejectionsAfter, Count{Key: string(t), Count: n})
		}
	}
	for _, st := range bySource {
		s.BySource = append(s.BySource, *st)
	}
	slices.SortFunc(s.BySource, func(a, b SourceStats) int {
		if c := cmp.Compare(b.Applied, a.Applied); c != 0 {
			return c
		}
		return cmp.Compare(a.Source, b.Source)
	})
	return s
}

// precedingType liefert den Typ des letzten Ereignisses vor index, das nicht
// KeineRueckmeldung ist; gibt es keines, KeineRueckmeldung.
func precedingType(events []domain.Event, index int) domain.EventType {
	for i := index - 1; i >= 0; i-- {
		if events[i].Type != domain.KeineRueckmeldung {
			return events[i].Type
		}
	}
	return domain.KeineRueckmeldung
}

// daysToFirstResponse misst die Tage von "Beworben" bis zum ersten Ereignis, das eine
// Reaktion der Firma ist (alles außer Zurückziehen und Ghosting). Maßgeblich ist das
// frühere von OccurredOn und RecordedOn: Die Einladung zählt, nicht der Termin.
func daysToFirstResponse(events []domain.Event, appliedIdx int) (float64, bool) {
	start := domain.DateOf(events[appliedIdx].OccurredOn)
	for _, e := range events[appliedIdx+1:] {
		if e.Type == domain.Zurueckgezogen || e.Type == domain.KeineRueckmeldung {
			continue
		}
		answered := domain.DateOf(e.OccurredOn)
		if rec := domain.DateOf(e.RecordedOn); rec.Before(answered) {
			answered = rec
		}
		return answered.Sub(start).Hours() / 24, true
	}
	return 0, false
}

func median(values []float64) float64 {
	v := slices.Clone(values)
	slices.Sort(v)
	mid := len(v) / 2
	if len(v)%2 == 1 {
		return v[mid]
	}
	return (v[mid-1] + v[mid]) / 2
}

func mean(values []float64) float64 {
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}
