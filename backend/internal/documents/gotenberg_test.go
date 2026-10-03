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

// TestGotenbergRendersCV erzeugt mit einem echten Gotenberg-Container ein PDF.
// Mit CV_PDF_OUT=<pfad> wird das Ergebnis zum Ansehen gespeichert.
// gotenbergImage ist die festgelegte Gotenberg-Version (auch in docker-compose.yml).
const gotenbergImage = "gotenberg/gotenberg:8.37.0"

func TestGotenbergRendersCV(t *testing.T) {
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

	html, err := documents.RenderCV(sample(t), samplePhoto(t))
	if err != nil {
		t.Fatal(err)
	}
	pdf, err := documents.NewGotenberg(url).Convert(ctx, html, documents.Fonts())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) || len(pdf) < 10_000 {
		t.Fatalf("kein plausibles PDF (%d Bytes)", len(pdf))
	}
	if out := os.Getenv("CV_PDF_OUT"); out != "" {
		if err := os.WriteFile(out, pdf, 0o600); err != nil {
			t.Fatal(err)
		}
	}
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
