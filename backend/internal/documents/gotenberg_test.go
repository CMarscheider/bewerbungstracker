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
			if err == io.EOF {
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
	if fields["printBackground"] != "true" || fields["preferCssPageSize"] != "true" {
		t.Errorf("Felder = %v", fields)
	}
}

func TestGotenbergUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "kaputt", http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	_, err := documents.NewGotenberg(srv.URL).Convert(context.Background(), []byte("x"), nil)
	if !errors.Is(err, documents.ErrUnavailable) {
		t.Fatalf("erwartet ErrUnavailable, bekommen %v", err)
	}

	_, err = documents.NewGotenberg("http://127.0.0.1:1").Convert(context.Background(), []byte("x"), nil)
	if !errors.Is(err, documents.ErrUnavailable) {
		t.Fatalf("nicht erreichbar: erwartet ErrUnavailable, bekommen %v", err)
	}
}

// TestGotenbergRendersCV erzeugt mit einem echten Gotenberg-Container ein PDF.
// Mit CV_PDF_OUT=<pfad> wird das Ergebnis zum Ansehen gespeichert.
func TestGotenbergRendersCV(t *testing.T) {
	if testing.Short() {
		t.Skip("braucht Docker")
	}
	if err := testdb.EnsureDockerHost(); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	ctr, err := testcontainers.Run(ctx, "gotenberg/gotenberg:8",
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
