package documents

import (
	"regexp"
	"strings"
)

// genderToken ist ein einzelnes Kürzel einer Geschlechterangabe: m, w, f, d, x, i, div., divers.
const genderToken = `(?:div(?:erse?)?\b\.?|[mwfdxi]\b)`

// genderTag erkennt Geschlechterangaben wie "(m/w/d)", "(all genders)", "(gn)" oder ein
// nacktes "m/w/d" (mindestens drei Kürzel) samt Leerraum und Gedankenstrich davor.
// Nur Leerzeichen/Tabs davor werden entfernt, nie Zeilenumbrüche.
var genderTag = regexp.MustCompile(`(?i)[ \t]*(?:[-–—][ \t]*)?(?:` +
	`[(\[]\s*(?:` + genderToken + `(?:\s*[/|,]\s*` + genderToken + `)+` +
	`|gn\*?|all[ -]?genders?|alle\s+geschlechter)\s*[)\]]` +
	`|\b` + genderToken + `(?:[ \t]*[/|][ \t]*` + genderToken + `){2,})`)

var (
	multiSpace = regexp.MustCompile(`[ \t]{2,}`)
	lineEdge   = regexp.MustCompile(`(?m)^[ \t]+|[ \t]+$`)
)

// StripGenderTags entfernt Geschlechterangaben wie "(m/w/d)" aus s (ohne Groß-/Kleinschreibung).
// Nur wenn etwas entfernt wurde, werden zurückbleibende Doppel-Leerzeichen zusammengefasst und
// Zeilen an den Rändern von Leerzeichen befreit; Absätze bleiben erhalten.
func StripGenderTags(s string) string {
	out := genderTag.ReplaceAllString(s, "")
	if out == s {
		return s
	}
	out = multiSpace.ReplaceAllString(out, " ")
	return strings.TrimSpace(lineEdge.ReplaceAllString(out, ""))
}
