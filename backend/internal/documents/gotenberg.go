package documents

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"slices"
	"strings"
	"time"
)

// ErrUnavailable: Gotenberg ist nicht erreichbar oder überlastet.
var ErrUnavailable = errors.New("pdf-dienst nicht erreichbar")

// maxPDFBytes begrenzt die gelesene Antwort.
const maxPDFBytes = 20 << 20

// Gotenberg wandelt HTML über den Chromium-Endpunkt von Gotenberg in PDF um und fügt PDFs zusammen.
type Gotenberg struct {
	url    string
	client *http.Client
}

func NewGotenberg(url string) *Gotenberg {
	return &Gotenberg{url: strings.TrimRight(url, "/"), client: &http.Client{Timeout: 60 * time.Second}}
}

// Convert schickt index.html und die Zusatzdateien (z. B. Schriften) und liefert das PDF.
// Seitengröße und Ränder legt das CSS fest (@page).
func (g *Gotenberg) Convert(ctx context.Context, html []byte, assets map[string][]byte) ([]byte, error) {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	if err := addFile(w, "index.html", html); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(assets))
	for name := range assets {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		if err := addFile(w, name, assets[name]); err != nil {
			return nil, err
		}
	}
	for _, f := range [][2]string{
		{"printBackground", "true"}, {"preferCssPageSize", "true"},
		{"marginTop", "0"}, {"marginBottom", "0"}, {"marginLeft", "0"}, {"marginRight", "0"},
	} {
		if err := w.WriteField(f[0], f[1]); err != nil {
			return nil, err
		}
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return g.post(ctx, "/forms/chromium/convert/html", w.FormDataContentType(), &body)
}

// Merge fügt PDFs in der übergebenen Reihenfolge zu einem zusammen. Gotenberg sortiert die
// Dateien nach Namen, daher heißen sie 001.pdf, 002.pdf, …
func (g *Gotenberg) Merge(ctx context.Context, pdfs ...[]byte) ([]byte, error) {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	for i, pdf := range pdfs {
		if err := addFile(w, fmt.Sprintf("%03d.pdf", i+1), pdf); err != nil {
			return nil, err
		}
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return g.post(ctx, "/forms/pdfengines/merge", w.FormDataContentType(), &body)
}

// post schickt das Formular an Gotenberg und liefert das PDF der Antwort.
func (g *Gotenberg) post(ctx context.Context, path, contentType string, body io.Reader) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.url+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	res, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(res.Body, 500))
		if res.StatusCode >= 500 || res.StatusCode == http.StatusTooManyRequests {
			return nil, fmt.Errorf("%w: status %d: %s", ErrUnavailable, res.StatusCode, msg)
		}
		return nil, fmt.Errorf("gotenberg: status %d: %s", res.StatusCode, msg)
	}
	pdf, err := io.ReadAll(io.LimitReader(res.Body, maxPDFBytes+1))
	if err != nil {
		return nil, fmt.Errorf("gotenberg: antwort lesen: %w", err)
	}
	if len(pdf) > maxPDFBytes {
		return nil, fmt.Errorf("gotenberg: pdf größer als %d bytes", maxPDFBytes)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		return nil, errors.New("gotenberg: antwort ist kein pdf")
	}
	return pdf, nil
}

func addFile(w *multipart.Writer, name string, data []byte) error {
	part, err := w.CreateFormFile("files", name)
	if err != nil {
		return err
	}
	_, err = part.Write(data)
	return err
}
