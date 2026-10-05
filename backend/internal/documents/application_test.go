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

// renderApp liefert Anschreiben und Lebenslauf der Bewerbung als Text.
func renderApp(t *testing.T, cv documents.CV, photo []byte, l documents.Letter) (letter, cvHTML string) {
	t.Helper()
	app, err := documents.RenderApplication(cv, photo, l)
	if err != nil {
		t.Fatal(err)
	}
	return string(app.Letter), string(app.CV)
}

func TestRenderApplicationContainsLetterAndCV(t *testing.T) {
	letter, cv := renderApp(t, sample(t), samplePhoto(t), sampleLetter())
	for _, want := range []string{
		"Bewerbung als Junior Frontend-Entwickler", "Acme GmbH", "Espelkamp, 5. Oktober 2026",
		"<p>Sehr geehrte Damen und Herren,</p>", "<p>erster Absatz.</p>", "<p>zweiter Absatz<br>mit zweiter Zeile.</p>",
		"Mit freundlichen Grüßen", `class="letter"`,
		// Absenderzeile mit Kontaktdaten statt Seitenleiste
		"Erika Mustermann", "Junior Frontend-Entwicklerin", "0170 1234567", "erika@example.com", "github.com/erika",
	} {
		if !strings.Contains(letter, want) {
			t.Errorf("Anschreiben: fehlt %q", want)
		}
	}
	for _, unwanted := range []string{"<aside", "html::before", "data:image/jpeg", "Kenntnisse", "Berufserfahrung", "javascript:"} {
		if strings.Contains(letter, unwanted) {
			t.Errorf("Anschreiben enthält %q (keine Seitenleiste, kein Foto)", unwanted)
		}
	}
	for _, want := range []string{
		"<aside", "html::before", "data:image/jpeg", "Kenntnisse", "Berufserfahrung", "09/2020 – heute",
		"Junior-Frontend-Entwickler mit Angular-Projekten.", // ersetzt das Profil
	} {
		if !strings.Contains(cv, want) {
			t.Errorf("Lebenslauf: fehlt %q", want)
		}
	}
	if strings.Contains(cv, "Oberflächen") {
		t.Error("Profil-Satz muss das Profil ersetzen")
	}
	if strings.Contains(cv, "Erfunden") {
		t.Error("Schwerpunkt, der nicht im Lebenslauf steht, darf nicht erscheinen")
	}
}

func TestRenderApplicationKeepsSummaryWithoutProfileLine(t *testing.T) {
	l := sampleLetter()
	l.ProfileLine = ""
	if _, cv := renderApp(t, sample(t), nil, l); !strings.Contains(cv, "Oberflächen") {
		t.Error("ohne Profil-Satz bleibt das Profil")
	}
}

func TestRenderApplicationOrdersSkillsByHighlights(t *testing.T) {
	cv := sample(t)
	cv.Skills = append([]documents.SkillGroup{{Category: "Werkzeuge", Items: []string{"Git"}}}, cv.Skills...)
	l := sampleLetter()
	l.Highlights = []string{" typescript ", "unbekannt"}
	_, html := renderApp(t, cv, nil, l)
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
	letter, cv := renderApp(t, sample(t), nil, l)
	html := letter + cv
	for _, want := range []string{"Application for", "Espelkamp, 5 October 2026", "Kind regards", "Experience", "Skills", "09/2020 – present", "<title>Curriculum Vitae"} {
		if !strings.Contains(html, want) {
			t.Errorf("fehlt: %q", want)
		}
	}
	for _, doc := range []string{letter, cv} {
		if !strings.Contains(doc, `lang="en"`) {
			t.Error(`lang="en" fehlt`)
		}
	}
	for _, unwanted := range []string{"Berufserfahrung", "heute", "Lebenslauf"} {
		if strings.Contains(html, unwanted) {
			t.Errorf("deutscher Text %q im englischen Dokument", unwanted)
		}
	}
}

func TestRenderApplicationWithoutLocation(t *testing.T) {
	cv := sample(t)
	cv.Person.Location = ""
	if letter, _ := renderApp(t, cv, nil, sampleLetter()); !strings.Contains(letter, ">5. Oktober 2026<") {
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
	letter, cv := renderApp(t, sample(t), nil, l)
	s := letter + cv
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
	if got := documents.ApplicationFileName(cv, "Acme GmbH (m/w/d)"); got != "Bewerbung_Mustermann_Acme_GmbH.pdf" {
		t.Errorf("mit Geschlechterangabe: got %q", got)
	}
	if got := documents.ApplicationFileName(cv, " !! "); got != "Bewerbung_Mustermann.pdf" {
		t.Errorf("ohne Firma: got %q", got)
	}
	cv.Person.Name = ""
	if got := documents.ApplicationFileName(cv, ""); got != "Bewerbung.pdf" {
		t.Errorf("leer: got %q", got)
	}
}

func TestRenderApplicationStripsGenderTags(t *testing.T) {
	l := sampleLetter()
	l.PositionTitle = "Junior Frontend-Entwickler (m/w/d)"
	html, _ := renderApp(t, sample(t), nil, l)
	if !strings.Contains(html, "Bewerbung als Junior Frontend-Entwickler</p>") {
		t.Error("Betreff ohne Geschlechterangabe fehlt")
	}
	if strings.Contains(html, "m/w/d") {
		t.Error("Geschlechterangabe im Anschreiben")
	}
}
