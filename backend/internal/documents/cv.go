// Package documents erzeugt Bewerbungsunterlagen: HTML aus Vorlagen, PDF über Gotenberg.
package documents

import (
	"encoding/json"
	"fmt"
	"strings"
)

// CV entspricht dem OpenAPI-Schema Cv (ohne updated_at); optionale Felder sind leer statt nil.
type CV struct {
	Person     Person       `json:"person"`
	Summary    string       `json:"summary"`
	Experience []Experience `json:"experience"`
	Education  []Education  `json:"education"`
	Skills     []SkillGroup `json:"skills"`
	Projects   []Project    `json:"projects"`
	Languages  []Language   `json:"languages"`
}

type Person struct {
	Name     string `json:"name"`
	Headline string `json:"headline"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Location string `json:"location"`
	Links    []Link `json:"links"`
}

type Link struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

type Experience struct {
	Role         string   `json:"role"`
	Organization string   `json:"organization"`
	Location     string   `json:"location"`
	Start        string   `json:"start"`
	End          string   `json:"end"`
	Highlights   []string `json:"highlights"`
}

type Education struct {
	Degree      string `json:"degree"`
	Institution string `json:"institution"`
	Start       string `json:"start"`
	End         string `json:"end"`
	Details     string `json:"details"`
}

type SkillGroup struct {
	Category string   `json:"category"`
	Items    []string `json:"items"`
}

type Project struct {
	Name         string   `json:"name"`
	URL          string   `json:"url"`
	Description  string   `json:"description"`
	Technologies []string `json:"technologies"`
}

type Language struct {
	Language string `json:"language"`
	Level    string `json:"level"`
}

// ParseCV liest den gespeicherten Lebenslauf (JSON im Schema Cv).
func ParseCV(data []byte) (CV, error) {
	var cv CV
	if err := json.Unmarshal(data, &cv); err != nil {
		return CV{}, fmt.Errorf("lebenslauf lesen: %w", err)
	}
	return cv, nil
}

var fileNameReplacer = strings.NewReplacer("ä", "ae", "ö", "oe", "ü", "ue", "Ä", "Ae", "Ö", "Oe", "Ü", "Ue", "ß", "ss")

// CVFileName liefert z. B. "Lebenslauf_Christian_Marscheider.pdf" (nur ASCII, für Content-Disposition);
// ohne verwertbaren Namen "Lebenslauf.pdf".
func CVFileName(cv CV) string {
	name := fileNameReplacer.Replace(strings.Join(strings.Fields(cv.Person.Name), "_"))
	var b strings.Builder
	for _, r := range name {
		if r < 128 && (r == '_' || r == '-' || r >= '0' && r <= '9' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z') {
			b.WriteRune(r)
		}
	}
	if strings.Trim(b.String(), "_-") == "" {
		return "Lebenslauf.pdf"
	}
	return "Lebenslauf_" + b.String() + ".pdf"
}
