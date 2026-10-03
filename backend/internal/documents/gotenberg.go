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

// Gotenberg wandelt HTML über den Chromium-Endpunkt von Gotenberg in PDF um.
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
	for k, v := range map[string]string{
		"printBackground": "true", "preferCssPageSize": "true",
		"marginTop": "0", "marginBottom": "0", "marginLeft": "0", "marginRight": "0",
	} {
		if err := w.WriteField(k, v); err != nil {
			return nil, err
		}
	}
	if err := w.Close(); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.url+"/forms/chromium/convert/html", &body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	res, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err) //nolint:errorlint // Ursache nur als Text
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(res.Body, 500))
		if res.StatusCode >= 500 || res.StatusCode == http.StatusTooManyRequests {
			return nil, fmt.Errorf("%w: status %d: %s", ErrUnavailable, res.StatusCode, msg)
		}
		return nil, fmt.Errorf("gotenberg: status %d: %s", res.StatusCode, msg)
	}
	return io.ReadAll(io.LimitReader(res.Body, maxPDFBytes))
}

func addFile(w *multipart.Writer, name string, data []byte) error {
	part, err := w.CreateFormFile("files", name)
	if err != nil {
		return err
	}
	_, err = part.Write(data)
	return err
}
