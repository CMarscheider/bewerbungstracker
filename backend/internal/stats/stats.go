// Package stats berechnet Kennzahlen aus Ereignisverläufen, ohne Datenbankzugriff.
package stats

import (
	"slices"
	"sort"
	"strings"

	"bewerbungsmanager/internal/domain"
)

// Application ist der Verlauf einer Bewerbung, Ereignisse in Erfassungsreihenfolge.
type Application struct {
	Source *string
	Events []domain.Event
}

// FunnelStep ist eine Stufe im Bewerbungs-Funnel.
type FunnelStep struct {
	Stage   string
	Reached int
	Rate    float64 // Anteil an allen Bewerbungen mit "Beworben"
}

var funnelStages = []struct {
	name     string
	minOrder int
}{
	{"Beworben", domain.Beworben.Order()},
	{"Screening", domain.ScreeningGespraech.Order()},
	{"Challenge", domain.ChallengeErhalten.Order()},
	{"Interview", domain.Interview.Order()},
	{"Kennenlerntag", domain.Kennenlerntag.Order()},
	{"Angebot", domain.AngebotErhalten.Order()},
}

// Funnel zählt, wie viele Bewerbungen mindestens bis zu jeder Stufe gekommen sind.
func Funnel(apps []Application) []FunnelStep {
	counts := make([]int, len(funnelStages))
	accepted, base := 0, 0
	for _, a := range apps {
		m := maxOrder(a.Events)
		if m < domain.Beworben.Order() {
			continue
		}
		base++
		for i, s := range funnelStages {
			if m >= s.minOrder {
				counts[i]++
			}
		}
		if hasType(a.Events, domain.AngebotAngenommen) {
			accepted++
		}
	}
	steps := make([]FunnelStep, 0, len(funnelStages)+1)
	for i, s := range funnelStages {
		steps = append(steps, FunnelStep{Stage: s.name, Reached: counts[i], Rate: rate(counts[i], base)})
	}
	return append(steps, FunnelStep{Stage: "Angenommen", Reached: accepted, Rate: rate(accepted, base)})
}

// maxOrder liefert die höchste erreichte Position in der aktiven Phase, sonst -1.
func maxOrder(events []domain.Event) int {
	m := -1
	for _, e := range events {
		if e.Type.Phase() == domain.PhaseAktiv && e.Type.Order() > m {
			m = e.Type.Order()
		}
	}
	return m
}

func hasType(events []domain.Event, t domain.EventType) bool {
	for _, e := range events {
		if e.Type == t {
			return true
		}
	}
	return false
}

func rate(n, base int) float64 {
	if base == 0 {
		return 0
	}
	return float64(n) / float64(base)
}

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
		idx := slices.IndexFunc(a.Events, func(e domain.Event) bool { return e.Type == domain.Beworben })
		if idx < 0 {
			continue
		}
		s.Applied++
		if n, ok := daysToFirstResponse(a.Events, idx); ok {
			s.Responded++
			days = append(days, n)
		}
		for i := 1; i < len(a.Events); i++ {
			if a.Events[i].Type == domain.Absage {
				rejections[a.Events[i-1].Type]++
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
	sort.Slice(s.BySource, func(i, j int) bool {
		if s.BySource[i].Applied != s.BySource[j].Applied {
			return s.BySource[i].Applied > s.BySource[j].Applied
		}
		return s.BySource[i].Source < s.BySource[j].Source
	})
	return s
}

// daysToFirstResponse misst die Tage von "Beworben" bis zum ersten Ereignis, das eine
// Reaktion der Firma ist (alles außer Zurückziehen und Ghosting).
func daysToFirstResponse(events []domain.Event, appliedIdx int) (float64, bool) {
	start := domain.DateOf(events[appliedIdx].OccurredOn)
	for _, e := range events[appliedIdx+1:] {
		if e.Type == domain.Zurueckgezogen || e.Type == domain.KeineRueckmeldung {
			continue
		}
		return domain.DateOf(e.OccurredOn).Sub(start).Hours() / 24, true
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
