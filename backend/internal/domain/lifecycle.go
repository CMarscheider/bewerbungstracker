package domain

import (
	"fmt"
	"slices"
	"time"
)

// AllowedNext liefert die Ereignistypen, die auf den bisherigen Verlauf folgen dürfen,
// in kanonischer Reihenfolge. Ein leeres Ergebnis bedeutet: Bewerbung ist abgeschlossen.
func AllowedNext(history []EventType) []EventType {
	if len(history) == 0 {
		return []EventType{Vorgemerkt, Beworben}
	}
	last := history[len(history)-1]
	allowed := map[EventType]bool{}

	switch {
	case last == KeineRueckmeldung:
		// Späte Antwort: alles, was vor dem Ghosting erlaubt war – außer erneutem Ghosting.
		for _, t := range AllowedNext(history[:len(history)-1]) {
			if t != KeineRueckmeldung {
				allowed[t] = true
			}
		}
	case last == Vorgemerkt:
		allowed[Beworben] = true
		allowed[Zurueckgezogen] = true
	case last == AngebotErhalten:
		allowed[AngebotAngenommen] = true
		allowed[AngebotAbgelehnt] = true
		allowed[Absage] = true
		allowed[Zurueckgezogen] = true
	case last.Phase() == PhaseAktiv:
		for _, t := range allEventTypes {
			if t.Phase() == PhaseAktiv && t.Order() > last.Order() {
				allowed[t] = true
			}
		}
		if last != ChallengeErhalten {
			delete(allowed, ChallengeAbgegeben)
		}
		if last == Interview {
			allowed[Interview] = true
		}
		allowed[Absage] = true
		allowed[Zurueckgezogen] = true
		allowed[KeineRueckmeldung] = true
	}
	return inCanonicalOrder(allowed)
}

func inCanonicalOrder(set map[EventType]bool) []EventType {
	var out []EventType
	for _, t := range allEventTypes {
		if set[t] {
			out = append(out, t)
		}
	}
	return out
}

// Event ist ein gespeichertes Ereignis, reduziert auf das, was die Regeln brauchen.
type Event struct {
	Type       EventType
	OccurredOn time.Time // fachliches Datum
	RecordedOn time.Time // Erfassungsdatum
}

// NewEvent ist ein Ereignis, das angelegt werden soll.
type NewEvent struct {
	Type       EventType
	OccurredOn time.Time
	DueOn      *time.Time
	Note       *string
}

// DateOf schneidet die Uhrzeit ab und liefert Mitternacht UTC desselben Kalendertags.
func DateOf(t time.Time) time.Time {
	y, m, day := t.Date()
	return time.Date(y, m, day, 0, 0, 0, 0, time.UTC)
}

// Types extrahiert die Ereignistypen eines Verlaufs.
func Types(history []Event) []EventType {
	out := make([]EventType, 0, len(history))
	for _, e := range history {
		out = append(out, e.Type)
	}
	return out
}

// CanApply prüft, ob next an den Verlauf angehängt werden darf.
func CanApply(history []Event, next NewEvent, today time.Time) error {
	if !next.Type.Valid() {
		return &ValidationError{Field: "type", Detail: fmt.Sprintf("unbekannter Ereignistyp %q", next.Type)}
	}
	if next.DueOn != nil && !next.Type.AllowsDeadline() {
		return &ValidationError{Field: "due_on", Detail: fmt.Sprintf("%s hat keine Frist", next.Type)}
	}

	types := Types(history)
	if !slices.Contains(AllowedNext(types), next.Type) {
		var from EventType
		if len(types) > 0 {
			from = types[len(types)-1]
		}
		return &TransitionError{From: from, Attempted: next.Type}
	}

	occurred := DateOf(next.OccurredOn)
	if occurred.After(DateOf(today)) && !next.Type.AllowsFutureDate() {
		return &RuleError{Code: CodeDateInFuture, Detail: fmt.Sprintf("%s darf nicht in der Zukunft liegen", next.Type)}
	}
	if len(history) > 0 {
		prev := history[len(history)-1]
		earliest := DateOf(prev.OccurredOn)
		if rec := DateOf(prev.RecordedOn); rec.Before(earliest) {
			earliest = rec
		}
		if occurred.Before(earliest) {
			return &RuleError{
				Code:   CodeDateBeforePrevious,
				Detail: fmt.Sprintf("Datum darf nicht vor dem %s liegen", earliest.Format("02.01.2006")),
			}
		}
	}
	return nil
}

// InterviewRound liefert die Rundennummer für ein neues Interview.
func InterviewRound(history []Event) int {
	round := 1
	for _, e := range history {
		if e.Type == Interview {
			round++
		}
	}
	return round
}

// CanUndo prüft, ob das letzte Ereignis gelöscht werden darf.
func CanUndo(history []Event) error {
	if len(history) <= 1 {
		return &RuleError{
			Code:   CodeCannotUndoFirst,
			Detail: "Das erste Ereignis kann nicht gelöscht werden; lösche stattdessen die Bewerbung",
		}
	}
	return nil
}
