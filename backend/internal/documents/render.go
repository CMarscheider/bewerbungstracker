package documents

import (
	"bytes"
	"embed"
	"encoding/base64"
	"fmt"
	"html/template"
	"strings"
)

//go:embed templates/*.tmpl
var templateFS embed.FS

//go:embed fonts/*.ttf
var fontFS embed.FS

var cvTemplate = template.Must(template.New("cv.html.tmpl").Funcs(template.FuncMap{
	"period":   period,
	"safeURL":  safeURL,
	"linkText": linkText,
}).ParseFS(templateFS, "templates/cv.html.tmpl"))

type cvView struct {
	CV
	Photo template.URL
}

// RenderCV erzeugt die HTML-Seite des Lebenslaufs; photo ist ein JPEG oder nil.
func RenderCV(cv CV, photo []byte) ([]byte, error) {
	view := cvView{CV: cv}
	if len(photo) > 0 {
		view.Photo = template.URL("data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(photo))
	}
	var buf bytes.Buffer
	if err := cvTemplate.Execute(&buf, view); err != nil {
		return nil, fmt.Errorf("lebenslauf-vorlage: %w", err)
	}
	return buf.Bytes(), nil
}

// fontNames sind die Schriftdateien, die die Vorlage per relativer URL einbindet.
var fontNames = []string{"Carlito-Regular.ttf", "Carlito-Bold.ttf", "Carlito-Italic.ttf"}

// Fonts liefert die Schriftdateien, die die Vorlage per relativer URL einbindet.
// Die Dateien sind eingebettet; fehlt eine, ist das ein Programmierfehler und Fonts löst panic aus.
func Fonts() map[string][]byte {
	out := make(map[string][]byte, len(fontNames))
	for _, name := range fontNames {
		data, err := fontFS.ReadFile("fonts/" + name)
		if err != nil {
			panic(fmt.Sprintf("eingebettete Schrift %s: %v", name, err))
		}
		out[name] = data
	}
	return out
}

// period formatiert "2020-09" → "09/2020", "2024" → "2024"; leeres Ende → "heute".
func period(start, end string) string {
	return formatMonth(start) + " – " + formatMonth(end)
}

func formatMonth(p string) string {
	switch {
	case p == "":
		return "heute"
	case len(p) == 7 && p[4] == '-':
		return p[5:] + "/" + p[:4]
	default:
		return p
	}
}

// safeURL gibt nur http(s)-Links frei; alles andere wird nicht verlinkt.
func safeURL(u string) template.URL {
	l := strings.ToLower(strings.TrimSpace(u))
	if strings.HasPrefix(l, "https://") || strings.HasPrefix(l, "http://") {
		return template.URL(strings.TrimSpace(u))
	}
	return ""
}

// linkText zeigt eine URL ohne Schema (Groß-/Kleinschreibung egal) und ohne abschließenden
// Schrägstrich, z. B. "github.com/erika"; bleibt nichts übrig, erscheint die Bezeichnung.
func linkText(u template.URL, label string) string {
	s := string(u)
	for _, scheme := range []string{"https://", "http://"} {
		if len(s) >= len(scheme) && strings.EqualFold(s[:len(scheme)], scheme) {
			s = s[len(scheme):]
			break
		}
	}
	s = strings.TrimSuffix(s, "/")
	if s == "" {
		return label
	}
	return s
}
