package documents_test

import (
	"strings"
	"testing"

	"bewerbungsmanager/internal/documents"
)

const sampleJSON = `{
  "person": {"name": "Erika Mustermann", "headline": "Junior Frontend-Entwicklerin", "email": "erika@example.com",
             "phone": "0170 1234567", "location": "Espelkamp",
             "links": [{"label": "GitHub", "url": "https://github.com/erika"}, {"label": "Böse", "url": "javascript:alert(1)"}]},
  "summary": "Baut gern <b>Oberflächen</b>.",
  "experience": [
    {"role": "Call Center Agent", "organization": "Acme GmbH", "location": "Bremen", "start": "2020-09", "highlights": ["Kundenservice für Samsung"]},
    {"role": "Mechanikerin", "organization": "Rad & Co", "start": "2015", "end": "2020-08", "highlights": []}
  ],
  "education": [{"degree": "Ausbildung", "institution": "JET GmbH", "start": "2012-08", "end": "2014-07"}],
  "skills": [{"category": "Frontend", "items": ["Angular", "TypeScript"]}],
  "projects": [{"name": "Ring of Fire", "url": "https://ring.example", "description": "Kartenspiel", "technologies": ["Angular", "Firebase"]}],
  "languages": [{"language": "Englisch", "level": "gut"}]
}`

func sample(t *testing.T) documents.CV {
	t.Helper()
	cv, err := documents.ParseCV([]byte(sampleJSON))
	if err != nil {
		t.Fatal(err)
	}
	return cv
}

func TestRenderCVContainsContent(t *testing.T) {
	html, err := documents.RenderCV(sample(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	s := string(html)
	for _, want := range []string{
		"Erika Mustermann", "Junior Frontend-Entwicklerin", "Espelkamp", "erika@example.com",
		"Call Center Agent", "Acme GmbH", "Kundenservice für Samsung", "Ring of Fire", "Firebase",
		"Englisch", "gut", "font-family: Carlito", ">github.com/erika</a>",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("HTML enthält %q nicht", want)
		}
	}
}

func TestRenderCVFormatsPeriods(t *testing.T) {
	s := string(must(documents.RenderCV(sample(t), nil)))
	for _, want := range []string{"09/2020 – heute", "2015 – 08/2020", "08/2012 – 07/2014"} {
		if !strings.Contains(s, want) {
			t.Errorf("Zeitraum %q fehlt", want)
		}
	}
}

func TestRenderCVEscapesAndFiltersLinks(t *testing.T) {
	s := string(must(documents.RenderCV(sample(t), nil)))
	if strings.Contains(s, "<b>Oberflächen</b>") {
		t.Error("HTML aus den Daten wurde nicht maskiert")
	}
	if strings.Contains(s, "javascript:") {
		t.Error("javascript:-Link darf nicht im HTML landen")
	}
	if !strings.Contains(s, `href="https://github.com/erika"`) || !strings.Contains(s, `href="https://ring.example"`) {
		t.Error("https-Links fehlen")
	}
	if !strings.Contains(s, "Böse") {
		t.Error("Bezeichnung eines unsicheren Links soll als Text bleiben")
	}
}

func TestRenderCVEmbedsPhoto(t *testing.T) {
	without := string(must(documents.RenderCV(sample(t), nil)))
	if strings.Contains(without, "data:image/jpeg") {
		t.Error("ohne Foto darf kein Bild eingebettet sein")
	}
	with := string(must(documents.RenderCV(sample(t), []byte("\xff\xd8\xff"))))
	if !strings.Contains(with, `src="data:image/jpeg;base64,/9j/`) {
		t.Error("Foto ist nicht als data-URI eingebettet")
	}
}

func TestCVFileName(t *testing.T) {
	cv := sample(t)
	cv.Person.Name = "Jürgen  Groß-Müller"
	if got := documents.CVFileName(cv); got != "Lebenslauf_Juergen_Gross-Mueller.pdf" {
		t.Errorf("CVFileName = %q", got)
	}
}

func TestFontsAreEmbedded(t *testing.T) {
	fonts := documents.Fonts()
	for _, name := range []string{"Carlito-Regular.ttf", "Carlito-Bold.ttf", "Carlito-Italic.ttf"} {
		if len(fonts[name]) < 100_000 {
			t.Errorf("Schrift %s fehlt oder ist zu klein (%d Bytes)", name, len(fonts[name]))
		}
	}
}

func TestCVFileNameEmpty(t *testing.T) {
	cv := sample(t)
	for _, name := range []string{"", "  ", "!!!"} {
		cv.Person.Name = name
		if got := documents.CVFileName(cv); got != "Lebenslauf.pdf" {
			t.Errorf("CVFileName(%q) = %q", name, got)
		}
	}
}

func TestRenderCVLinkText(t *testing.T) {
	cv := sample(t)
	cv.Person.Links = []documents.Link{
		{Label: "Seite", URL: "HTTP://Example.de/"},
		{Label: "Leer", URL: "HTTPS://"},
	}
	s := string(must(documents.RenderCV(cv, nil)))
	if !strings.Contains(s, ">Example.de</a>") {
		t.Error("Schema soll ohne Rücksicht auf Groß-/Kleinschreibung entfernt werden")
	}
	if !strings.Contains(s, ">Leer</a>") {
		t.Error("leerer Linktext soll auf die Bezeichnung zurückfallen")
	}
}

func TestRenderCVQuoteInURL(t *testing.T) {
	cv := sample(t)
	cv.Person.Links = []documents.Link{{Label: "X", URL: `https://x.de/"><script>alert(1)</script>`}}
	cv.Projects[0].URL = `https://x.de/"><script>alert(1)</script>`
	s := string(must(documents.RenderCV(cv, nil)))
	if strings.Contains(s, "<script>") {
		t.Error("<script> aus der URL landet roh im HTML")
	}
	if strings.Contains(s, `href="https://x.de/"`) {
		t.Error("Anführungszeichen in der URL beendet das href-Attribut")
	}
}

func must(b []byte, err error) []byte {
	if err != nil {
		panic(err)
	}
	return b
}
