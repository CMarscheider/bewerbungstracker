package documents_test

import (
	"strings"
	"testing"
	"time"

	"bewerbungsmanager/internal/documents"
)

func sampleLetter() documents.Letter {
	return documents.Letter{
		Language: "de", CompanyName: "Acme GmbH", PositionTitle: "Junior Frontend-Entwickler",
		CoverLetter: "Sehr geehrte Damen und Herren,\n\nerster Absatz.\n \nzweiter Absatz\nmit zweiter Zeile.",
		ProfileLine: "Junior-Frontend-Entwickler mit Angular-Projekten.",
		Highlights:  []string{"TypeScript", "Erfunden"},
		Date:        time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC),
	}
}

func TestRenderApplicationContainsLetterAndCV(t *testing.T) {
	html := string(must(documents.RenderApplication(sample(t), nil, sampleLetter())))
	for _, want := range []string{
		"Bewerbung als Junior Frontend-Entwickler", "Acme GmbH", "Espelkamp, 5. Oktober 2026",
		"<p>Sehr geehrte Damen und Herren,</p>", "<p>erster Absatz.</p>", "<p>zweiter Absatz<br>mit zweiter Zeile.</p>",
		"Mit freundlichen Grüßen",
		"Junior-Frontend-Entwickler mit Angular-Projekten.", // ersetzt das Profil
		`class="page letter"`, `class="page cv"`, "Kenntnisse", "Berufserfahrung", "09/2020 – heute",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("fehlt: %q", want)
		}
	}
	if strings.Contains(html, "Oberflächen") {
		t.Error("Profil-Satz muss das Profil ersetzen")
	}
	if strings.Contains(html, "Erfunden") {
		t.Error("Schwerpunkt, der nicht im Lebenslauf steht, darf nicht erscheinen")
	}
}

func TestRenderApplicationKeepsSummaryWithoutProfileLine(t *testing.T) {
	l := sampleLetter()
	l.ProfileLine = ""
	if !strings.Contains(string(must(documents.RenderApplication(sample(t), nil, l))), "Oberflächen") {
		t.Error("ohne Profil-Satz bleibt das Profil")
	}
}

func TestRenderApplicationOrdersSkillsByHighlights(t *testing.T) {
	cv := sample(t)
	cv.Skills = append([]documents.SkillGroup{{Category: "Werkzeuge", Items: []string{"Git"}}}, cv.Skills...)
	l := sampleLetter()
	l.Highlights = []string{" typescript ", "unbekannt"}
	html := string(must(documents.RenderApplication(cv, nil, l)))
	if strings.Index(html, ">TypeScript<") > strings.Index(html, ">Angular<") {
		t.Error("TypeScript muss vor Angular stehen")
	}
	if strings.Index(html, ">Frontend<") > strings.Index(html, ">Werkzeuge<") {
		t.Error("Gruppe mit Treffer muss vorn stehen")
	}
	if cv.Skills[1].Items[0] != "Angular" {
		t.Error("Lebenslauf des Aufrufers darf nicht verändert werden")
	}
}

func TestRenderApplicationEnglish(t *testing.T) {
	l := sampleLetter()
	l.Language = "en"
	html := string(must(documents.RenderApplication(sample(t), nil, l)))
	for _, want := range []string{"Application for", "Espelkamp, 5 October 2026", "Kind regards", "Experience", "Skills", "09/2020 – present", `lang="en"`} {
		if !strings.Contains(html, want) {
			t.Errorf("fehlt: %q", want)
		}
	}
	for _, unwanted := range []string{"Berufserfahrung", "heute"} {
		if strings.Contains(html, unwanted) {
			t.Errorf("deutscher Text %q im englischen Dokument", unwanted)
		}
	}
}

func TestRenderApplicationWithoutLocation(t *testing.T) {
	cv := sample(t)
	cv.Person.Location = ""
	if !strings.Contains(string(must(documents.RenderApplication(cv, nil, sampleLetter()))), ">5. Oktober 2026<") {
		t.Error("ohne Ort steht nur das Datum")
	}
}

func TestRenderApplicationUnknownLanguage(t *testing.T) {
	l := sampleLetter()
	l.Language = "fr"
	if _, err := documents.RenderApplication(sample(t), nil, l); err == nil {
		t.Error("unbekannte Sprache muss ein Fehler sein")
	}
}

func TestRenderApplicationEscapes(t *testing.T) {
	l := sampleLetter()
	l.CoverLetter = "<script>alert(1)</script>"
	l.CompanyName = "<b>Acme</b>"
	l.PositionTitle = "<i>Dev</i>"
	l.ProfileLine = "<u>Profil</u>"
	s := string(must(documents.RenderApplication(sample(t), nil, l)))
	for _, raw := range []string{"<script>alert", "<b>Acme", "<i>Dev", "<u>Profil"} {
		if strings.Contains(s, raw) {
			t.Errorf("%q wurde nicht maskiert", raw)
		}
	}
}

func TestApplicationFileName(t *testing.T) {
	cv := sample(t)
	if got := documents.ApplicationFileName(cv, "Müller & Söhne GmbH"); got != "Bewerbung_Mustermann_Mueller_Soehne_GmbH.pdf" {
		t.Errorf("got %q", got)
	}
	if got := documents.ApplicationFileName(cv, " !! "); got != "Bewerbung_Mustermann.pdf" {
		t.Errorf("ohne Firma: got %q", got)
	}
	cv.Person.Name = ""
	if got := documents.ApplicationFileName(cv, ""); got != "Bewerbung.pdf" {
		t.Errorf("leer: got %q", got)
	}
}
