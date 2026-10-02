// Package domain enthält die Fachlogik des Bewerbungsverlaufs ohne Abhängigkeiten zu DB oder HTTP.
package domain

import "slices"

// EventType ist die Art eines Ereignisses im Bewerbungsverlauf.
type EventType string

const (
	Vorgemerkt         EventType = "Vorgemerkt"
	Beworben           EventType = "Beworben"
	ScreeningGespraech EventType = "ScreeningGespraech"
	ChallengeErhalten  EventType = "ChallengeErhalten"
	ChallengeAbgegeben EventType = "ChallengeAbgegeben"
	Interview          EventType = "Interview"
	Kennenlerntag      EventType = "Kennenlerntag"
	AngebotErhalten    EventType = "AngebotErhalten"
	AngebotAngenommen  EventType = "AngebotAngenommen"
	AngebotAbgelehnt   EventType = "AngebotAbgelehnt"
	Absage             EventType = "Absage"
	Zurueckgezogen     EventType = "Zurueckgezogen"
	KeineRueckmeldung  EventType = "KeineRueckmeldung"
)

// Phase fasst Ereignistypen zu groben Abschnitten zusammen.
type Phase string

const (
	PhaseVorbereitung  Phase = "Vorbereitung"
	PhaseAktiv         Phase = "Aktiv"
	PhaseAbgeschlossen Phase = "Abgeschlossen"
)

type eventInfo struct {
	phase      Phase
	order      int // Reihenfolge im Ablauf; -1 für Abschluss-Ereignisse
	deadline   bool
	futureDate bool
}

var allEventTypes = []EventType{
	Vorgemerkt, Beworben, ScreeningGespraech, ChallengeErhalten, ChallengeAbgegeben,
	Interview, Kennenlerntag, AngebotErhalten,
	AngebotAngenommen, AngebotAbgelehnt, Absage, Zurueckgezogen, KeineRueckmeldung,
}

var eventInfos = map[EventType]eventInfo{
	Vorgemerkt:         {PhaseVorbereitung, 0, true, false},
	Beworben:           {PhaseAktiv, 1, false, false},
	ScreeningGespraech: {PhaseAktiv, 2, false, true},
	ChallengeErhalten:  {PhaseAktiv, 3, true, false},
	ChallengeAbgegeben: {PhaseAktiv, 4, false, false},
	Interview:          {PhaseAktiv, 5, false, true},
	Kennenlerntag:      {PhaseAktiv, 6, false, true},
	AngebotErhalten:    {PhaseAktiv, 7, true, false},
	AngebotAngenommen:  {PhaseAbgeschlossen, -1, false, false},
	AngebotAbgelehnt:   {PhaseAbgeschlossen, -1, false, false},
	Absage:             {PhaseAbgeschlossen, -1, false, false},
	Zurueckgezogen:     {PhaseAbgeschlossen, -1, false, false},
	KeineRueckmeldung:  {PhaseAbgeschlossen, -1, false, false},
}

// AllEventTypes liefert alle Ereignistypen in kanonischer Reihenfolge.
func AllEventTypes() []EventType { return slices.Clone(allEventTypes) }

// Valid meldet, ob t ein bekannter Ereignistyp ist.
func (t EventType) Valid() bool {
	_, ok := eventInfos[t]
	return ok
}

// Phase liefert die Phase des Typs; leer für unbekannte Typen.
func (t EventType) Phase() Phase { return eventInfos[t].phase }

// Order liefert die Position im Ablauf (0 = Vorgemerkt … 7 = AngebotErhalten), sonst -1.
func (t EventType) Order() int {
	info, ok := eventInfos[t]
	if !ok {
		return -1
	}
	return info.order
}

// AllowsDeadline meldet, ob ein Ereignis dieses Typs eine Frist tragen darf.
func (t EventType) AllowsDeadline() bool { return eventInfos[t].deadline }

// AllowsFutureDate meldet, ob das Datum in der Zukunft liegen darf (geplanter Termin).
func (t EventType) AllowsFutureDate() bool { return eventInfos[t].futureDate }

// StatusesInPhase liefert alle Typen einer Phase in kanonischer Reihenfolge.
func StatusesInPhase(p Phase) []EventType {
	var out []EventType
	for _, t := range allEventTypes {
		if t.Phase() == p {
			out = append(out, t)
		}
	}
	return out
}
