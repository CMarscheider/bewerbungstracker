package documents

import (
	"bytes"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
)

// Letter ist der stellenbezogene Teil der Bewerbung.
type Letter struct {
	Language      string   // "de" oder "en"
	CompanyName   string   // Empfänger
	PositionTitle string   // Betreff: "Bewerbung als …"
	CoverLetter   string   // Anrede und Absätze, getrennt durch Leerzeilen; Gruß und Name ergänzt die Vorlage
	ProfileLine   string   // ersetzt das Profil im Lebenslauf, wenn gesetzt
	Highlights    []string // ordnen die Kenntnisse; nur vorhandene Einträge zählen
	Date          time.Time
}

// letterView sind die Daten des Anschreibens.
type letterView struct {
	cvView
	Letter     Letter
	Dateline   string     // "Espelkamp, 5. Oktober 2026"
	Paragraphs [][]string // Absätze, je Absatz die Zeilen
}

// ApplicationHTML sind die beiden Teile der Bewerbung. Sie werden getrennt in PDF umgewandelt und
// danach zusammengefügt (Anschreiben zuerst), weil die Seitenleiste des Lebenslaufs per
// position: fixed auf jeder Seite ihres Dokuments erscheint – das Anschreiben soll keine haben.
type ApplicationHTML struct {
	Letter []byte // genau eine A4-Seite, einspaltig, ohne Foto
	CV     []byte // Lebenslauf mit Seitenleiste in der Sprache des Anschreibens
}

// RenderApplication erzeugt Anschreiben und angepassten Lebenslauf als zwei HTML-Dokumente.
// photo ist ein JPEG oder nil. Die Anschreiben-Seite bricht nie um; zu langer Text wird abgeschnitten.
func RenderApplication(cv CV, photo []byte, l Letter) (ApplicationHTML, error) {
	if _, ok := labels[l.Language]; !ok {
		return ApplicationHTML{}, fmt.Errorf("bewerbung: unbekannte sprache %q", l.Language)
	}
	if l.ProfileLine != "" {
		cv.Summary = l.ProfileLine
	}
	cv.Skills = orderSkills(cv.Skills, l.Highlights)
	view := letterView{
		cvView:     newCVView(cv, photo, l.Language),
		Letter:     l,
		Dateline:   formatDate(l.Date, l.Language, labels[l.Language]),
		Paragraphs: paragraphs(l.CoverLetter),
	}
	if loc := strings.TrimSpace(cv.Person.Location); loc != "" {
		view.Dateline = loc + ", " + view.Dateline
	}
	var letter, cvHTML bytes.Buffer
	if err := templates.ExecuteTemplate(&letter, "letter.html.tmpl", view); err != nil {
		return ApplicationHTML{}, fmt.Errorf("anschreiben-vorlage: %w", err)
	}
	if err := templates.ExecuteTemplate(&cvHTML, "cv.html.tmpl", view.cvView); err != nil {
		return ApplicationHTML{}, fmt.Errorf("lebenslauf-vorlage: %w", err)
	}
	return ApplicationHTML{Letter: letter.Bytes(), CV: cvHTML.Bytes()}, nil
}

func formatDate(d time.Time, lang string, lbl Labels) string {
	month := lbl.Months[d.Month()-1]
	if lang == "de" {
		return fmt.Sprintf("%d. %s %d", d.Day(), month, d.Year())
	}
	return fmt.Sprintf("%d %s %d", d.Day(), month, d.Year())
}

var blankLine = regexp.MustCompile(`\n\s*\n`)

// paragraphs trennt an Leerzeilen; einzelne Zeilenumbrüche bleiben als Zeilen eines Absatzes.
func paragraphs(text string) [][]string {
	var out [][]string
	for _, p := range blankLine.Split(strings.ReplaceAll(text, "\r\n", "\n"), -1) {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		lines := strings.Split(p, "\n")
		for i := range lines {
			lines[i] = strings.TrimSpace(lines[i])
		}
		out = append(out, lines)
	}
	return out
}

// orderSkills liefert eine Kopie, in der Einträge aus highlights (ohne Groß-/Kleinschreibung) in
// deren Reihenfolge vorn in ihrer Gruppe stehen und Gruppen mit Treffern vor den übrigen.
// Unbekannte highlights werden ignoriert; es kommt nie ein Eintrag hinzu.
func orderSkills(groups []SkillGroup, highlights []string) []SkillGroup {
	rank := make(map[string]int, len(highlights))
	for i, h := range highlights {
		key := strings.ToLower(strings.TrimSpace(h))
		if _, seen := rank[key]; !seen && key != "" {
			rank[key] = i
		}
	}
	rankOf := func(item string) (int, bool) {
		r, ok := rank[strings.ToLower(strings.TrimSpace(item))]
		return r, ok
	}
	var hit, rest []SkillGroup
	for _, g := range groups {
		items := slices.Clone(g.Items)
		slices.SortStableFunc(items, func(a, b string) int {
			ra, oka := rankOf(a)
			rb, okb := rankOf(b)
			switch {
			case oka && okb:
				return ra - rb
			case oka:
				return -1
			case okb:
				return 1
			default:
				return 0
			}
		})
		g.Items = items
		if len(items) > 0 {
			if _, ok := rankOf(items[0]); ok {
				hit = append(hit, g)
				continue
			}
		}
		rest = append(rest, g)
	}
	return append(hit, rest...)
}

var nonFileChars = regexp.MustCompile(`[^A-Za-z0-9-]+`)

// fileNamePart macht aus s einen ASCII-Teil für Dateinamen, z. B. "Müller & Söhne" → "Mueller_Soehne".
func fileNamePart(s string) string {
	return strings.Trim(nonFileChars.ReplaceAllString(fileNameReplacer.Replace(s), "_"), "_-")
}

// ApplicationFileName liefert z. B. "Bewerbung_Marscheider_Acme_GmbH.pdf" (nur ASCII, für
// Content-Disposition und Mail-Anhang); fehlende Teile entfallen.
func ApplicationFileName(cv CV, company string) string {
	parts := []string{"Bewerbung"}
	if names := strings.Fields(cv.Person.Name); len(names) > 0 {
		parts = append(parts, fileNamePart(names[len(names)-1]))
	}
	parts = append(parts, fileNamePart(company))
	parts = slices.DeleteFunc(parts, func(p string) bool { return p == "" })
	return strings.Join(parts, "_") + ".pdf"
}
