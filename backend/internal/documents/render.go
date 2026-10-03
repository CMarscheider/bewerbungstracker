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
	"period":  period,
	"safeURL": safeURL,
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

// Fonts liefert die Schriftdateien, die die Vorlage per relativer URL einbindet.
func Fonts() map[string][]byte {
	out := map[string][]byte{}
	entries, _ := fontFS.ReadDir("fonts")
	for _, e := range entries {
		data, err := fontFS.ReadFile("fonts/" + e.Name())
		if err == nil {
			out[e.Name()] = data
		}
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

// LinkText zeigt eine URL ohne Schema und abschließenden Schrägstrich, z. B. "github.com/erika".
func (cvView) LinkText(u template.URL) string {
	s := string(u)
	s = strings.TrimPrefix(strings.TrimPrefix(s, "https://"), "http://")
	return strings.TrimSuffix(s, "/")
}
