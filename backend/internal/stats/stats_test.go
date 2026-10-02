package stats

import (
	"math"
	"testing"
	"time"

	"bewerbungsmanager/internal/domain"
)

func d(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func app(source *string, events ...domain.Event) Application {
	return Application{Source: source, Events: events}
}

func e(t domain.EventType, occurred string) domain.Event {
	return domain.Event{Type: t, OccurredOn: d(occurred), RecordedOn: d(occurred)}
}

func str(s string) *string { return &s }

// sampleApps: 4 Bewerbungen in der Basis, 1 nur vorgemerkt.
func sampleApps() []Application {
	return []Application{
		app(str("LinkedIn"), e(domain.Beworben, "2026-09-01"), e(domain.Absage, "2026-09-11")),
		app(nil, e(domain.Vorgemerkt, "2026-09-01")),
		app(str("LinkedIn"),
			e(domain.Beworben, "2026-09-01"), e(domain.Interview, "2026-09-05"), e(domain.Interview, "2026-09-12"),
			e(domain.AngebotErhalten, "2026-09-20"), e(domain.AngebotAngenommen, "2026-09-22")),
		app(nil,
			e(domain.Beworben, "2026-09-01"), e(domain.ChallengeErhalten, "2026-09-03"),
			e(domain.ChallengeAbgegeben, "2026-09-08"), e(domain.Absage, "2026-09-15")),
		app(str("Empfehlung"),
			e(domain.Vorgemerkt, "2026-09-05"), e(domain.Beworben, "2026-09-10"),
			e(domain.KeineRueckmeldung, "2026-10-01")),
	}
}

func TestFunnel(t *testing.T) {
	got := Funnel(sampleApps())
	want := []FunnelStep{
		{"Beworben", 4, 1.0},
		{"Screening", 2, 0.5},
		{"Challenge", 2, 0.5},
		{"Interview", 1, 0.25},
		{"Kennenlerntag", 1, 0.25},
		{"Angebot", 1, 0.25},
		{"Angenommen", 1, 0.25},
	}
	if len(got) != len(want) {
		t.Fatalf("Funnel hat %d Stufen, erwartet %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Stage != want[i].Stage || got[i].Reached != want[i].Reached || math.Abs(got[i].Rate-want[i].Rate) > 1e-9 {
			t.Errorf("Stufe %d = %+v, erwartet %+v", i, got[i], want[i])
		}
	}
}

func TestFunnelEmpty(t *testing.T) {
	got := Funnel(nil)
	if len(got) != 7 {
		t.Fatalf("Funnel hat %d Stufen, erwartet 7", len(got))
	}
	for _, s := range got {
		if s.Reached != 0 || s.Rate != 0 {
			t.Errorf("leerer Funnel: %+v", s)
		}
	}
}
