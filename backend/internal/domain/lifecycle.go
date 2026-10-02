package domain

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
