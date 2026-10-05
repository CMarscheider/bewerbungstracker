package documents_test

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"bewerbungsmanager/internal/documents"
	"bewerbungsmanager/internal/testdb"
)

func TestGotenbergSendsFilesAndOptions(t *testing.T) {
	var files []string
	var fields = map[string]string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/forms/chromium/convert/html" {
			t.Errorf("Pfad = %s", r.URL.Path)
		}
		_, params, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
		mr := multipart.NewReader(r.Body, params["boundary"])
		for {
			p, err := mr.NextPart()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				t.Errorf("multipart: %v", err)
				break
			}
			if p.FileName() != "" {
				files = append(files, p.FileName())
			} else {
				v, _ := io.ReadAll(p)
				fields[p.FormName()] = string(v)
			}
		}
		_, _ = w.Write([]byte("%PDF-1.7 fake"))
	}))
	defer srv.Close()

	pdf, err := documents.NewGotenberg(srv.URL).Convert(context.Background(), []byte("<html></html>"), map[string][]byte{"a.ttf": {1}})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Errorf("Antwort = %q", pdf)
	}
	if len(files) != 2 || files[0] != "index.html" || files[1] != "a.ttf" {
		t.Errorf("Dateien = %v", files)
	}
	want := map[string]string{
		"printBackground": "true", "preferCssPageSize": "true",
		"marginTop": "0", "marginBottom": "0", "marginLeft": "0", "marginRight": "0",
	}
	for k, v := range want {
		if fields[k] != v {
			t.Errorf("Feld %s = %q, erwartet %q", k, fields[k], v)
		}
	}
}

func TestGotenbergErrors(t *testing.T) {
	for _, tc := range []struct {
		name        string
		status      int
		body        string
		unavailable bool
	}{
		{"503", http.StatusServiceUnavailable, "kaputt", true},
		{"429", http.StatusTooManyRequests, "zu viel", true},
		{"400", http.StatusBadRequest, "falsch", false},
		{"200 ohne PDF", http.StatusOK, "<html>kein pdf</html>", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			_, err := documents.NewGotenberg(srv.URL).Convert(context.Background(), []byte("x"), nil)
			if err == nil {
				t.Fatal("Fehler erwartet")
			}
			if got := errors.Is(err, documents.ErrUnavailable); got != tc.unavailable {
				t.Errorf("errors.Is(ErrUnavailable) = %v, erwartet %v (%v)", got, tc.unavailable, err)
			}
		})
	}
}

func TestGotenbergUnreachable(t *testing.T) {
	_, err := documents.NewGotenberg("http://127.0.0.1:1").Convert(context.Background(), []byte("x"), nil)
	if !errors.Is(err, documents.ErrUnavailable) {
		t.Fatalf("erwartet ErrUnavailable, bekommen %v", err)
	}
}

func TestGotenbergRejectsOversizedPDF(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("%PDF-1.7 "))
		_, _ = w.Write(make([]byte, 20<<20))
	}))
	defer srv.Close()
	_, err := documents.NewGotenberg(srv.URL).Convert(context.Background(), []byte("x"), nil)
	if err == nil {
		t.Fatal("zu großes PDF muss abgelehnt werden")
	}
}

// gotenbergImage ist die festgelegte Gotenberg-Version (auch in docker-compose.yml).
const gotenbergImage = "gotenberg/gotenberg:8.37.0"

// startGotenberg startet einen echten Gotenberg-Container und liefert dessen URL.
func startGotenberg(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("braucht Docker")
	}
	if err := testdb.EnsureDockerHost(); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	ctr, err := testcontainers.Run(ctx, gotenbergImage,
		testcontainers.WithExposedPorts("3000/tcp"),
		testcontainers.WithWaitStrategy(wait.ForHTTP("/health").WithPort("3000/tcp")),
	)
	testcontainers.CleanupContainer(t, ctr)
	if err != nil {
		t.Fatal(err)
	}
	url, err := ctr.PortEndpoint(ctx, "3000/tcp", "http")
	if err != nil {
		t.Fatal(err)
	}
	return url
}

// TestGotenbergRendersCV erzeugt mit einem echten Gotenberg-Container ein PDF.
// Mit CV_PDF_OUT=<pfad> wird das Ergebnis zum Ansehen gespeichert.
func TestGotenbergRendersCV(t *testing.T) {
	url := startGotenberg(t)
	html, err := documents.RenderCV(sample(t), samplePhoto(t))
	if err != nil {
		t.Fatal(err)
	}
	pdf := convert(t, url, html)
	if out := os.Getenv("CV_PDF_OUT"); out != "" {
		if err := os.WriteFile(out, pdf, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// TestGotenbergRendersApplication prüft, dass das Anschreiben genau eine Seite bleibt – auch mit
// zu langem Text – und der Lebenslauf danach folgt. Mit APPLICATION_PDF_OUT=<pfad> wird das
// Ergebnis mit normalem Anschreiben zum Ansehen gespeichert.
func TestGotenbergRendersApplication(t *testing.T) {
	url := startGotenberg(t)
	letter := sampleLetter()
	letter.CoverLetter = "Sehr geehrte Damen und Herren,\n\n" +
		strings.Repeat("mit großem Interesse habe ich Ihre Stellenanzeige gelesen und bewerbe mich hiermit. ", 4) +
		"\n\n" + strings.Repeat("In meinen Projekten habe ich Angular, TypeScript und Firebase eingesetzt. ", 5) +
		"\n\nÜber eine Einladung zu einem Gespräch freue ich mich."
	tooLong := sampleLetter()
	tooLong.CoverLetter = strings.Repeat(strings.Repeat("Viel zu langer Text. ", 30)+"\n\n", 8)

	for name, l := range map[string]documents.Letter{"normal": letter, "zu lang": tooLong} {
		t.Run(name, func(t *testing.T) {
			html, err := documents.RenderApplication(sample(t), samplePhoto(t), l)
			if err != nil {
				t.Fatal(err)
			}
			pdf := convert(t, url, html)
			if n := pageCount(pdf); n != 2 {
				t.Errorf("Seiten = %d, erwartet 2 (Anschreiben + Lebenslauf)", n)
			}
			if out := os.Getenv("APPLICATION_PDF_OUT"); out != "" && name == "normal" {
				if err := os.WriteFile(out, pdf, 0o600); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func convert(t *testing.T, url string, html []byte) []byte {
	t.Helper()
	pdf, err := documents.NewGotenberg(url).Convert(context.Background(), html, documents.Fonts())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) || len(pdf) < 10_000 {
		t.Fatalf("kein plausibles PDF (%d Bytes)", len(pdf))
	}
	return pdf
}

var pageObject = regexp.MustCompile(`/Type\s*/Page\b`)

// pageCount zählt die Seitenobjekte (/Type /Page, nicht /Pages).
func pageCount(pdf []byte) int {
	return len(pageObject.FindAll(pdf, -1))
}

func samplePhoto(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 60, 80))
	for y := range 80 {
		for x := range 60 {
			img.Set(x, y, color.RGBA{R: uint8(x * 4), G: 120, B: uint8(y * 3), A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
