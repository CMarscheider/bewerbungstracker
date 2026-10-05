package documents_test

import (
	"bytes"
	"os"
	"testing"

	"bewerbungsmanager/internal/documents"
)

// TestRenderCVUnchanged schützt das Lebenslauf-HTML vor ungewollten Änderungen.
// Mit UPDATE_GOLDEN=1 wird testdata/cv_golden.html neu geschrieben (nur bei gewollter Layout-Änderung).
func TestRenderCVUnchanged(t *testing.T) {
	const golden = "testdata/cv_golden.html"
	got := must(documents.RenderCV(sample(t), samplePhoto(t)))
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(golden, got, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("Lebenslauf-HTML weicht von %s ab", golden)
	}
}
