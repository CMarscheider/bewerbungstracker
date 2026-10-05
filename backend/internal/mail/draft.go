// Package mail baut Bewerbungsentwürfe und legt sie per IMAP im Entwurfsordner ab. Es sendet nie.
package mail

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	netmail "net/mail"
	"net/textproto"
	"strings"
	"time"
	"unicode/utf8"
)

// Draft enthält alles für einen Bewerbungsentwurf: Mailtext und PDF-Anhang.
type Draft struct {
	From, To, Subject, Body string
	FileName                string
	PDF                     []byte
	Date                    time.Time
}

// base64LineLen: maximale Zeilenlänge im base64-Anhang (RFC 2045).
const base64LineLen = 76

// maxSubjectRunes: längster erlaubter Betreff (die API begrenzt ebenso).
const maxSubjectRunes = 200

// BuildDraft erzeugt eine RFC-5322-Nachricht (multipart/mixed: Text + PDF).
// Header-Werte mit Zeilenumbruch werden abgelehnt (Header-Injection).
func BuildDraft(d Draft) ([]byte, error) {
	for name, v := range map[string]string{"From": d.From, "To": d.To, "Subject": d.Subject, "Dateiname": d.FileName} {
		if strings.ContainsAny(v, "\r\n") {
			return nil, fmt.Errorf("mail: %s enthält einen zeilenumbruch", name)
		}
	}
	from, err := netmail.ParseAddress(d.From)
	if err != nil {
		return nil, fmt.Errorf("mail: absender ungültig: %w", err)
	}
	to, err := netmail.ParseAddress(d.To)
	if err != nil {
		return nil, fmt.Errorf("mail: empfänger ungültig: %w", err)
	}
	if strings.TrimSpace(d.FileName) == "" {
		return nil, errors.New("mail: dateiname fehlt")
	}
	if n := utf8.RuneCountInString(d.Subject); n > maxSubjectRunes {
		return nil, fmt.Errorf("mail: betreff länger als %d zeichen (%d)", maxSubjectRunes, n)
	}
	if len(d.PDF) == 0 {
		return nil, errors.New("mail: pdf fehlt")
	}
	msgID, err := messageID(from.Address)
	if err != nil {
		return nil, err
	}
	date := d.Date
	if date.IsZero() {
		date = time.Now()
	}

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	header := func(k, v string) { fmt.Fprintf(&buf, "%s: %s\r\n", k, v) }
	header("From", from.String())
	header("To", to.String())
	header("Subject", foldSubject(d.Subject))
	header("Date", date.Format(time.RFC1123Z))
	header("Message-Id", msgID)
	header("MIME-Version", "1.0")
	header("Content-Type", mime.FormatMediaType("multipart/mixed", map[string]string{"boundary": mw.Boundary()}))
	buf.WriteString("\r\n")

	// Text-Teil: quoted-printable, Zeilenumbrüche werden zu CRLF.
	text, err := mw.CreatePart(textproto.MIMEHeader{
		"Content-Type":              {"text/plain; charset=utf-8"},
		"Content-Transfer-Encoding": {"quoted-printable"},
	})
	if err != nil {
		return nil, err
	}
	qp := quotedprintable.NewWriter(text)
	body := strings.ReplaceAll(strings.ReplaceAll(d.Body, "\r\n", "\n"), "\r", "\n")
	if _, err := qp.Write([]byte(body)); err != nil {
		return nil, err
	}
	if err := qp.Close(); err != nil {
		return nil, err
	}

	// PDF-Teil: base64 in Zeilen à 76 Zeichen.
	att, err := mw.CreatePart(textproto.MIMEHeader{
		"Content-Type":              {withFileName("application/pdf", "name", d.FileName)},
		"Content-Disposition":       {withFileName("attachment", "filename", d.FileName)},
		"Content-Transfer-Encoding": {"base64"},
	})
	if err != nil {
		return nil, err
	}
	enc := base64.StdEncoding.EncodeToString(d.PDF)
	for len(enc) > 0 {
		n := min(base64LineLen, len(enc))
		if _, err := fmt.Fprintf(att, "%s\r\n", enc[:n]); err != nil {
			return nil, err
		}
		enc = enc[n:]
	}
	if err := mw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// withFileName hängt den Dateinamen als Parameter an. Reine ASCII-Namen stehen
// in Anführungszeichen, alle anderen kodiert mime.FormatMediaType nach RFC 2231.
func withFileName(value, param, name string) string {
	plain := true
	for _, r := range name {
		if r < 0x20 || r > 0x7e || r == '"' || r == '\\' {
			plain = false
			break
		}
	}
	if plain {
		return fmt.Sprintf("%s; %s=%q", value, param, name)
	}
	return mime.FormatMediaType(value, map[string]string{param: name})
}

// messageID erzeugt <zufällige 16 Byte hex @ Domain des Absenders>.
func messageID(addr string) (string, error) {
	domain := addr[strings.LastIndex(addr, "@")+1:]
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("mail: message-id: %w", err)
	}
	return "<" + hex.EncodeToString(b) + "@" + domain + ">", nil
}

// foldSubject kodiert den Betreff nach RFC 2047 und faltet zwischen den
// encoded-words (CRLF + Leerzeichen), damit keine Zeile zu lang wird.
func foldSubject(s string) string {
	enc := mime.QEncoding.Encode("utf-8", s)
	if enc == s {
		return s // reines ASCII, nicht kodiert
	}
	return strings.ReplaceAll(enc, "?= =?", "?=\r\n =?")
}
