// Package stats berechnet Kennzahlen aus Ereignisverläufen, ohne Datenbankzugriff.
package stats

import "bewerbungsmanager/internal/domain"

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
