package documents_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
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

func TestGotenbergMergeSendsPDFsInOrder(t *testing.T) {
	var files []string
	var contents []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/forms/pdfengines/merge" {
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
			v, _ := io.ReadAll(p)
			files, contents = append(files, p.FileName()), append(contents, string(v))
		}
		_, _ = w.Write([]byte("%PDF-1.7 merged"))
	}))
	defer srv.Close()

	pdfs := make([][]byte, 11)
	for i := range pdfs {
		pdfs[i] = []byte(fmt.Sprintf("%%PDF-%d", i))
	}
	pdf, err := documents.NewGotenberg(srv.URL).Merge(context.Background(), pdfs...)
	if err != nil || string(pdf) != "%PDF-1.7 merged" {
		t.Fatalf("Merge = %q, %v", pdf, err)
	}
	// Gotenberg fügt in Namensreihenfolge zusammen: die Namen müssen so sortieren wie die Eingabe.
	if !slices.IsSorted(files) || len(files) != 11 || files[0] != "001.pdf" {
		t.Errorf("Dateien = %v", files)
	}
	for i, c := range contents {
		if c != fmt.Sprintf("%%PDF-%d", i) {
			t.Errorf("Datei %d = %q", i, c)
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
			g := documents.NewGotenberg(srv.URL)
			_, convErr := g.Convert(context.Background(), []byte("x"), nil)
			_, mergeErr := g.Merge(context.Background(), []byte("%PDF-a"), []byte("%PDF-b"))
			for _, err := range []error{convErr, mergeErr} {
				if err == nil {
					t.Fatal("Fehler erwartet")
				}
				if got := errors.Is(err, documents.ErrUnavailable); got != tc.unavailable {
					t.Errorf("errors.Is(ErrUnavailable) = %v, erwartet %v (%v)", got, tc.unavailable, err)
				}
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

// TestGotenbergRendersApplication prüft, dass das Anschreiben genau eine Seite ohne Seitenleiste
// bleibt – auch mit zu langem Text – und der Lebenslauf mit Seitenleiste danach folgt.
// Mit APPLICATION_PDF_OUT=<pfad> wird das Ergebnis mit normalem Anschreiben zum Ansehen gespeichert.
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
			app, err := documents.RenderApplication(sample(t), samplePhoto(t), l)
			if err != nil {
				t.Fatal(err)
			}
			g := documents.NewGotenberg(url)
			pdf, err := g.Merge(context.Background(), convert(t, url, app.Letter), convert(t, url, app.CV))
			if err != nil {
				t.Fatal(err)
			}
			pages := pageContents(t, pdf)
			if len(pages) != 2 {
				t.Fatalf("Seiten = %d, erwartet 2 (Anschreiben + Lebenslauf)", len(pages))
			}
			if sidebarFilled(pages[0]) {
				t.Error("Seite 1 (Anschreiben) hat eine Seitenleiste")
			}
			if !sidebarFilled(pages[1]) {
				t.Error("Seite 2 (Lebenslauf) ohne Seitenleiste")
			}
			if out := os.Getenv("APPLICATION_PDF_OUT"); out != "" && name == "normal" {
				if err := os.WriteFile(out, pdf, 0o600); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

// TestGotenbergCVHasSidebarOnEveryPage: der Lebenslauf allein zeichnet die Seitenleiste auf jeder Seite.
func TestGotenbergCVHasSidebarOnEveryPage(t *testing.T) {
	url := startGotenberg(t)
	cv := sample(t)
	for range 4 { // genug Einträge für eine zweite Seite
		cv.Experience = append(cv.Experience, cv.Experience...)
	}
	html, err := documents.RenderCV(cv, samplePhoto(t))
	if err != nil {
		t.Fatal(err)
	}
	pages := pageContents(t, convert(t, url, html))
	if len(pages) < 2 {
		t.Fatalf("Seiten = %d, erwartet mindestens 2", len(pages))
	}
	for i, p := range pages {
		if !sidebarFilled(p) {
			t.Errorf("Seite %d ohne Seitenleiste", i+1)
		}
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
