package documents_test

import (
	"testing"

	"bewerbungsmanager/internal/documents"
)

func TestStripGenderTags(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"Junior Frontend-Entwickler (m/w/d)", "Junior Frontend-Entwickler"},
		{"Junior Frontend-Entwickler (w/m/d)", "Junior Frontend-Entwickler"},
		{"Frontend Developer (m/f/d)", "Frontend Developer"},
		{"Frontend Developer (M/F/D)", "Frontend Developer"},
		{"Entwickler (d/m/w)", "Entwickler"},
		{"Entwickler (m/w/x)", "Entwickler"},
		{"Entwickler (m/w/i)", "Entwickler"},
		{"Entwickler (w/m/div.)", "Entwickler"},
		{"Entwickler (m/w/divers)", "Entwickler"},
		{"Entwickler (m|w|d)", "Entwickler"},
		{"Entwickler (m / w / d)", "Entwickler"},
		{"Entwickler ( m/w/d )", "Entwickler"},
		{"Entwickler [m/w/d]", "Entwickler"},
		{"Entwickler (m,w,d)", "Entwickler"},
		{"Entwickler (all genders)", "Entwickler"},
		{"Entwickler (All Genders)", "Entwickler"},
		{"Entwickler (all gender)", "Entwickler"},
		{"Entwickler (alle Geschlechter)", "Entwickler"},
		{"Entwickler (gn)", "Entwickler"},
		{"Entwickler (gn*)", "Entwickler"},
		{"Entwickler m/w/d", "Entwickler"},
		{"Entwickler M/W/D", "Entwickler"},
		{"Entwickler – (m/w/d)", "Entwickler"},
		{"Entwickler - m/w/d", "Entwickler"},
		{"Entwickler-(m/w/d)", "Entwickler"},
		{"Entwickler (m/w/d) in Berlin", "Entwickler in Berlin"},
		{"Entwickler (m/w/d) – Berlin", "Entwickler – Berlin"},
		{"als Entwickler (m/w/d), weil", "als Entwickler, weil"},
		{"als Entwickler (m/w/d).", "als Entwickler."},
		{"(m/w/d) Entwickler", "Entwickler"},
		{"Bewerbung als Entwickler m/w/d bei Acme", "Bewerbung als Entwickler bei Acme"},
		{"Zeile eins (m/w/d)\n\nZeile zwei", "Zeile eins\n\nZeile zwei"},
		// keine Geschlechterangaben
		{"Web/Mobile Entwickler", "Web/Mobile Entwickler"},
		{"Frontend (Angular)", "Frontend (Angular)"},
		{"Entwickler (w)", "Entwickler (w)"},
		{"Entwickler (Teilzeit)", "Entwickler (Teilzeit)"},
		{"UI/UX Designer", "UI/UX Designer"},
		{"Ein/Ausgabe", "Ein/Ausgabe"},
		{"m/w", "m/w"},
		{"Absatz eins.\n\nAbsatz  zwei", "Absatz eins.\n\nAbsatz  zwei"},
		{"Acme GmbH", "Acme GmbH"},
		{"", ""},
	} {
		if got := documents.StripGenderTags(tc.in); got != tc.want {
			t.Errorf("StripGenderTags(%q) = %q, erwartet %q", tc.in, got, tc.want)
		}
	}
}
