package main

import (
	"time"

	"bewerbungsmanager/internal/domain"
)

// step ist ein Ereignis relativ zu heute: Day -3 = vor drei Tagen, +2 = übermorgen.
type step struct {
	Type   domain.EventType
	Day    int
	DueDay *int
	Note   string
}

type scenario struct {
	Company  string
	Position string
	Location string
	Source   string
	Steps    []step
}

func due(day int) *int { return &day }

func (s step) event(today time.Time) domain.NewEvent {
	e := domain.NewEvent{Type: s.Type, OccurredOn: today.AddDate(0, 0, s.Day)}
	if s.DueDay != nil {
		d := today.AddDate(0, 0, *s.DueDay)
		e.DueOn = &d
	}
	if s.Note != "" {
		note := s.Note
		e.Note = &note
	}
	return e
}

// scenarios liefert fiktive Bewerbungen. Firmen und Personen sind frei erfunden.
func scenarios() []scenario {
	const (
		b  = domain.Beworben
		v  = domain.Vorgemerkt
		sc = domain.ScreeningGespraech
		ce = domain.ChallengeErhalten
		ca = domain.ChallengeAbgegeben
		iv = domain.Interview
		kt = domain.Kennenlerntag
		ae = domain.AngebotErhalten
		an = domain.AngebotAngenommen
		al = domain.AngebotAbgelehnt
		ab = domain.Absage
		zu = domain.Zurueckgezogen
		kr = domain.KeineRueckmeldung
	)
	return []scenario{
		{"Acme GmbH", "Senior Go-Entwickler", "Berlin", "LinkedIn", []step{
			{Type: b, Day: -40}, {Type: sc, Day: -33, Note: "Telefonat mit HR, 30 Minuten"},
			{Type: iv, Day: -25, Note: "Fachgespräch mit Teamlead"}, {Type: iv, Day: -18, Note: "Systemdesign mit CTO"},
			{Type: kt, Day: -10}, {Type: ae, Day: -3, DueDay: due(4), Note: "Angebot schriftlich erhalten"},
		}},
		{"Acme GmbH", "Platform Engineer", "Berlin", "LinkedIn", []step{
			{Type: b, Day: -70}, {Type: ab, Day: -60, Note: "Stelle intern besetzt"},
		}},
		{"Nordlicht Software", "Backend Engineer", "Hamburg", "StepStone", []step{
			{Type: b, Day: -30}, {Type: ce, Day: -24, DueDay: due(-20), Note: "REST-API in Go mit Tests"},
			{Type: ca, Day: -21}, {Type: iv, Day: 3, Note: "Code-Review der Challenge"},
		}},
		{"Datenwerk Solutions", "Go Developer", "Remote", "LinkedIn", []step{
			{Type: b, Day: -20}, {Type: ce, Day: -2, DueDay: due(5), Note: "Take-Home: CLI-Tool"},
		}},
		{"Bergblick IT", "Softwareentwickler Java/Go", "München", "Empfehlung", []step{
			{Type: b, Day: -50}, {Type: iv, Day: -40}, {Type: ae, Day: -30, DueDay: due(-25)},
			{Type: an, Day: -26, Note: "Start zum Quartalsbeginn"},
		}},
		{"Hanse Logistik", "Plattform-Entwickler", "Bremen", "Firmenwebsite", []step{
			{Type: b, Day: -35}, {Type: ab, Day: -28},
		}},
		{"Pixelhaus Studio", "Fullstack-Entwickler Angular/Go", "Köln", "LinkedIn", []step{
			{Type: b, Day: -45}, {Type: sc, Day: -38}, {Type: ab, Day: -36, Note: "Mehr Frontend-Erfahrung gesucht"},
		}},
		{"Cloudkraft", "DevOps Engineer", "Remote", "StepStone", []step{
			{Type: b, Day: -60}, {Type: kr, Day: -20},
		}},
		{"Finanzblick Bank", "Backend Developer", "Frankfurt am Main", "Indeed", []step{
			{Type: v, Day: -5, DueDay: due(-1), Note: "Anschreiben noch anpassen"},
		}},
		{"Solarwerk Energie", "Go-Entwickler", "Freiburg", "Firmenwebsite", []step{
			{Type: v, Day: -3, DueDay: due(10)},
		}},
		{"Medicus Health", "Software Engineer", "Leipzig", "LinkedIn", []step{
			{Type: b, Day: -14}, {Type: sc, Day: 2, Note: "Videocall 10:00 Uhr"},
		}},
		{"Logiq Systems", "Backend-Entwickler", "Stuttgart", "StepStone", []step{
			{Type: b, Day: -25}, {Type: iv, Day: -15}, {Type: ab, Day: -8},
		}},
		{"Byteschmiede", "Junior Go-Entwickler", "Dresden", "Empfehlung", []step{
			{Type: b, Day: -12}, {Type: kt, Day: 6, Note: "Probetag vor Ort"},
		}},
		{"Mittelland Versicherung", "Anwendungsentwickler", "Nürnberg", "Indeed", []step{
			{Type: b, Day: -40}, {Type: ce, Day: -32, DueDay: due(-25)}, {Type: ca, Day: -26}, {Type: ab, Day: -18},
		}},
		{"Stadtwerke Nordheide", "IT-Entwickler", "Kiel", "Firmenwebsite", []step{
			{Type: b, Day: -9},
		}},
		{"Quantum Retail", "Go Backend Engineer", "Berlin", "LinkedIn", []step{
			{Type: b, Day: -7},
		}},
		{"Greenfield Mobility", "Software Developer", "München", "StepStone", []step{
			{Type: b, Day: -21}, {Type: sc, Day: -16}, {Type: kr, Day: -2},
		}},
		{"Kanzlei Digital", "Legal-Tech-Entwickler", "Hamburg", "Empfehlung", []step{
			{Type: v, Day: -10}, {Type: zu, Day: -8, Note: "Passt fachlich nicht"},
		}},
		{"Orbit Games", "Backend Programmer", "Remote", "LinkedIn", []step{
			{Type: b, Day: -28}, {Type: iv, Day: -20}, {Type: ae, Day: -12, DueDay: due(-5)},
			{Type: al, Day: -6, Note: "Gehalt unter Erwartung"},
		}},
		{"Rheinwerk Consulting", "Go Consultant", "Düsseldorf", "Indeed", []step{
			{Type: b, Day: -16}, {Type: ab, Day: -15},
		}},
		{"Tierisch Gut", "Webentwickler", "Hannover", "Firmenwebsite", []step{
			{Type: b, Day: -4},
		}},
		{"Wolkenlos SaaS", "Senior Backend Engineer", "Remote", "LinkedIn", []step{
			{Type: b, Day: -33}, {Type: sc, Day: -27}, {Type: ce, Day: -22, DueDay: due(-15)},
			{Type: ca, Day: -16}, {Type: iv, Day: -9}, {Type: kr, Day: -1},
		}},
		{"Fahrrad Digital", "Software Engineer", "Münster", "StepStone", []step{
			{Type: b, Day: -19}, {Type: iv, Day: 9},
		}},
		{"Konsum IT", "Entwickler E-Commerce", "Bielefeld", "Indeed", []step{
			{Type: b, Day: -55}, {Type: ab, Day: -50},
		}},
		{"Eisvogel Labs", "Go-Entwickler", "Potsdam", "Empfehlung", []step{
			{Type: b, Day: -11}, {Type: ce, Day: -6, DueDay: due(1)},
		}},
		{"Hafen Data", "Data Engineer (Go)", "Hamburg", "LinkedIn", []step{
			{Type: b, Day: -26}, {Type: zu, Day: -10, Note: "Anderes Angebot angenommen"},
		}},
	}
}
