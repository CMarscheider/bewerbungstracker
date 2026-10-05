package mail

import (
	"bytes"
	"encoding/base64"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	netmail "net/mail"
	"regexp"
	"strings"
	"testing"
	"time"
)

func testDraft() Draft {
	return Draft{
		From:     "Christian Marscheider <c@example.com>",
		To:       "jobs@acme.example",
		Subject:  "Bewerbung als Junior Frontend-Entwickler (m/w/d) – Christian Marscheider",
		Body:     "Sehr geehrte Damen und Herren,\n\nanbei meine Unterlagen. Größe: ü ö ä ß.\n\nViele Grüße",
		FileName: "Bewerbung_Marscheider_Acme.pdf",
		PDF:      []byte("%PDF-1.7 test"),
		Date:     time.Date(2026, 10, 5, 9, 30, 0, 0, time.UTC),
	}
}

func TestBuildDraft(t *testing.T) {
	d := testDraft()
	raw, err := BuildDraft(d)
	if err != nil {
		t.Fatalf("BuildDraft: %v", err)
	}
	msg, err := netmail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}

	subj := msg.Header.Get("Subject")
	if !strings.HasPrefix(subj, "=?utf-8?") {
		t.Errorf("Subject nicht RFC-2047-kodiert: %q", subj)
	}
	dec := new(mime.WordDecoder)
	got, err := dec.DecodeHeader(subj)
	if err != nil || got != d.Subject {
		t.Errorf("Subject = %q (%v), want %q", got, err, d.Subject)
	}

	if to, err := msg.Header.AddressList("To"); err != nil || len(to) != 1 || to[0].Address != "jobs@acme.example" {
		t.Errorf("To = %v (%v)", to, err)
	}
	if from, err := msg.Header.AddressList("From"); err != nil || len(from) != 1 ||
		from[0].Address != "c@example.com" || from[0].Name != "Christian Marscheider" {
		t.Errorf("From = %v (%v)", from, err)
	}
	if date, err := msg.Header.Date(); err != nil || !date.Equal(d.Date) {
		t.Errorf("Date = %v (%v)", date, err)
	}
	if v := msg.Header.Get("MIME-Version"); v != "1.0" {
		t.Errorf("MIME-Version = %q", v)
	}
	if id := msg.Header.Get("Message-Id"); !regexp.MustCompile(`^<[0-9a-f]{32}@example\.com>$`).MatchString(id) {
		t.Errorf("Message-Id = %q", id)
	}

	mediaType, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	if err != nil || mediaType != "multipart/mixed" {
		t.Fatalf("Content-Type = %q (%v)", mediaType, err)
	}
	mr := multipart.NewReader(msg.Body, params["boundary"])

	// Teil 1: Text
	text, err := mr.NextRawPart()
	if err != nil {
		t.Fatalf("Textteil: %v", err)
	}
	if ct := text.Header.Get("Content-Type"); ct != "text/plain; charset=utf-8" {
		t.Errorf("Text Content-Type = %q", ct)
	}
	if cte := text.Header.Get("Content-Transfer-Encoding"); cte != "quoted-printable" {
		t.Errorf("Text Content-Transfer-Encoding = %q", cte)
	}
	body, err := io.ReadAll(quotedprintable.NewReader(text))
	if err != nil {
		t.Fatalf("Text dekodieren: %v", err)
	}
	if want := strings.ReplaceAll(d.Body, "\n", "\r\n"); string(body) != want {
		t.Errorf("Body = %q, want %q", body, want)
	}

	// Teil 2: PDF
	att, err := mr.NextRawPart()
	if err != nil {
		t.Fatalf("Anhang: %v", err)
	}
	if ct := att.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/pdf") {
		t.Errorf("Anhang Content-Type = %q", ct)
	}
	if cd := att.Header.Get("Content-Disposition"); cd != `attachment; filename="Bewerbung_Marscheider_Acme.pdf"` {
		t.Errorf("Content-Disposition = %q", cd)
	}
	if cte := att.Header.Get("Content-Transfer-Encoding"); cte != "base64" {
		t.Errorf("Anhang Content-Transfer-Encoding = %q", cte)
	}
	encoded, err := io.ReadAll(att)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimRight(string(encoded), "\r\n"), "\r\n") {
		if len(line) > 76 {
			t.Errorf("base64-Zeile länger als 76 Zeichen: %d", len(line))
		}
	}
	pdf, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(string(encoded), "\r\n", ""))
	if err != nil || !bytes.Equal(pdf, d.PDF) {
		t.Errorf("PDF = %q (%v)", pdf, err)
	}

	if _, err := mr.NextRawPart(); err != io.EOF {
		t.Errorf("mehr als zwei Teile: %v", err)
	}
}

func TestBuildDraftLongPDFLines(t *testing.T) {
	d := testDraft()
	d.PDF = bytes.Repeat([]byte("%PDF-1.7 0123456789"), 100)
	raw, err := BuildDraft(d)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(raw), "\r\n") {
		if len(line) > 998 {
			t.Fatalf("Zeile länger als 998 Zeichen")
		}
	}
	if !bytes.Contains(raw, []byte("\r\n"+base64.StdEncoding.EncodeToString(d.PDF)[:76]+"\r\n")) {
		t.Error("base64 nicht in 76er-Zeilen umbrochen")
	}
}

func TestBuildDraftRejectsHeaderInjection(t *testing.T) {
	cases := map[string]func(*Draft){
		"To mit CRLF":      func(d *Draft) { d.To = "a@b.de\r\nBcc: x@y.de" },
		"To mit LF":        func(d *Draft) { d.To = "a@b.de\nBcc: x@y.de" },
		"Subject mit CR":   func(d *Draft) { d.Subject = "Hallo\rBcc: x@y.de" },
		"Subject mit LF":   func(d *Draft) { d.Subject = "Hallo\nBcc: x@y.de" },
		"From mit LF":      func(d *Draft) { d.From = "Christian <c@example.com>\nBcc: x@y.de" },
		"Dateiname mit LF": func(d *Draft) { d.FileName = "a.pdf\nBcc: x@y.de" },
		"To ungültig":      func(d *Draft) { d.To = "keine adresse" },
		"To mehrfach":      func(d *Draft) { d.To = "a@b.de, x@y.de" },
		"From ungültig":    func(d *Draft) { d.From = "" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			d := testDraft()
			mutate(&d)
			if _, err := BuildDraft(d); err == nil {
				t.Error("kein Fehler")
			}
		})
	}
}

func TestBuildDraftFoldsSubject(t *testing.T) {
	d := testDraft()
	d.Subject = strings.Repeat("Bewerbung für Müller & Söhne – ", 6)
	raw, err := BuildDraft(d)
	if err != nil {
		t.Fatal(err)
	}
	head, _, _ := strings.Cut(string(raw), "\r\n\r\n")
	start := strings.Index(head, "\r\nSubject: ")
	if start < 0 {
		t.Fatal("Subject fehlt")
	}
	subjRaw := head[start+2:]
	if end := strings.Index(subjRaw, "\r\nDate:"); end >= 0 {
		subjRaw = subjRaw[:end]
	}
	lines := strings.Split(subjRaw, "\r\n")
	if len(lines) < 2 {
		t.Fatalf("Subject nicht gefaltet: %q", subjRaw)
	}
	for i, line := range lines {
		if i > 0 && !strings.HasPrefix(line, " =?utf-8?") {
			t.Errorf("Folgezeile %d beginnt nicht mit Leerzeichen + encoded-word: %q", i, line)
		}
		if len(line) > 100 {
			t.Errorf("Zeile %d zu lang: %d", i, len(line))
		}
	}

	msg, err := netmail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	got, err := new(mime.WordDecoder).DecodeHeader(msg.Header.Get("Subject"))
	if err != nil || got != d.Subject {
		t.Errorf("Subject = %q (%v), want %q", got, err, d.Subject)
	}
}

func TestBuildDraftSubjectLength(t *testing.T) {
	d := testDraft()
	d.Subject = strings.Repeat("ä", 200)
	if _, err := BuildDraft(d); err != nil {
		t.Errorf("200 Zeichen abgelehnt: %v", err)
	}
	d.Subject = strings.Repeat("ä", 201)
	if _, err := BuildDraft(d); err == nil {
		t.Error("201 Zeichen nicht abgelehnt")
	}
}

func TestBuildDraftRejectsEmptyPDF(t *testing.T) {
	for _, pdf := range [][]byte{nil, {}} {
		d := testDraft()
		d.PDF = pdf
		if _, err := BuildDraft(d); err == nil {
			t.Errorf("leeres PDF (%v) nicht abgelehnt", pdf)
		}
	}
}
