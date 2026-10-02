package domain

import (
	"slices"
	"testing"
)

func TestAllEventTypesAreValid(t *testing.T) {
	types := AllEventTypes()
	if len(types) != 13 {
		t.Fatalf("erwartet 13 Ereignistypen, bekommen %d", len(types))
	}
	for _, et := range types {
		if !et.Valid() {
			t.Errorf("%s sollte gültig sein", et)
		}
	}
	if EventType("Quatsch").Valid() {
		t.Error("unbekannter Typ darf nicht gültig sein")
	}
}

func TestPhase(t *testing.T) {
	tests := map[EventType]Phase{
		Vorgemerkt:        PhaseVorbereitung,
		Beworben:          PhaseAktiv,
		Interview:         PhaseAktiv,
		AngebotErhalten:   PhaseAktiv,
		AngebotAngenommen: PhaseAbgeschlossen,
		Absage:            PhaseAbgeschlossen,
		KeineRueckmeldung: PhaseAbgeschlossen,
	}
	for et, want := range tests {
		if got := et.Phase(); got != want {
			t.Errorf("%s.Phase() = %s, erwartet %s", et, got, want)
		}
	}
}

func TestOrder(t *testing.T) {
	if Beworben.Order() != 1 || AngebotErhalten.Order() != 7 {
		t.Errorf("Reihenfolge falsch: Beworben=%d AngebotErhalten=%d", Beworben.Order(), AngebotErhalten.Order())
	}
	if Absage.Order() != -1 || EventType("Quatsch").Order() != -1 {
		t.Error("Abschluss- und unbekannte Typen müssen Reihenfolge -1 haben")
	}
}

func TestAllowsDeadline(t *testing.T) {
	var got []EventType
	for _, et := range AllEventTypes() {
		if et.AllowsDeadline() {
			got = append(got, et)
		}
	}
	want := []EventType{Vorgemerkt, ChallengeErhalten, AngebotErhalten}
	if !slices.Equal(got, want) {
		t.Errorf("Typen mit Frist = %v, erwartet %v", got, want)
	}
}

func TestAllowsFutureDate(t *testing.T) {
	var got []EventType
	for _, et := range AllEventTypes() {
		if et.AllowsFutureDate() {
			got = append(got, et)
		}
	}
	want := []EventType{ScreeningGespraech, Interview, Kennenlerntag}
	if !slices.Equal(got, want) {
		t.Errorf("Typen mit Zukunftsdatum = %v, erwartet %v", got, want)
	}
}

func TestStatusesInPhase(t *testing.T) {
	got := StatusesInPhase(PhaseVorbereitung)
	if !slices.Equal(got, []EventType{Vorgemerkt}) {
		t.Errorf("Vorbereitung = %v", got)
	}
	if n := len(StatusesInPhase(PhaseAktiv)); n != 7 {
		t.Errorf("Aktiv hat %d Typen, erwartet 7", n)
	}
	if n := len(StatusesInPhase(PhaseAbgeschlossen)); n != 5 {
		t.Errorf("Abgeschlossen hat %d Typen, erwartet 5", n)
	}
}
