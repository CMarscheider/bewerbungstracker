package mail

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-imap/v2/imapserver"
	"github.com/emersion/go-imap/v2/imapserver/imapmemserver"
)

// gmailDrafts: imapmemserver kann keine Special-Use-Attribute setzen,
// daher wird der Fallback über den Gmail-Ordnernamen getestet.
const gmailDrafts = "[Gmail]/Drafts"

// startServer startet einen In-Memory-IMAP-Server auf 127.0.0.1 ohne TLS.
func startServer(t *testing.T) string {
	t.Helper()
	user := imapmemserver.NewUser("user", "pass")
	for _, name := range []string{"INBOX", "Entwürfe", gmailDrafts} {
		if err := user.Create(name, nil); err != nil {
			t.Fatal(err)
		}
	}
	mem := imapmemserver.New()
	mem.AddUser(user)
	srv := imapserver.New(&imapserver.Options{
		NewSession: func(*imapserver.Conn) (imapserver.Session, *imapserver.GreetingData, error) {
			return mem.NewSession(), nil, nil
		},
		Caps:         imap.CapSet{imap.CapIMAP4rev1: {}},
		InsecureAuth: true,
		Logger:       log.New(io.Discard, "", 0),
	})
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	return ln.Addr().String()
}

// messages liefert die Flags aller Nachrichten im Postfach.
func messages(t *testing.T, addr, mailbox string) [][]imap.Flag {
	t.Helper()
	c, err := imapclient.DialInsecure(addr, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	if err := c.Login("user", "pass").Wait(); err != nil {
		t.Fatal(err)
	}
	sel, err := c.Select(mailbox, nil).Wait()
	if err != nil {
		t.Fatal(err)
	}
	if sel.NumMessages == 0 {
		return nil
	}
	msgs, err := c.Fetch(imap.SeqSetNum(1, sel.NumMessages), &imap.FetchOptions{Flags: true}).Collect()
	if err != nil {
		t.Fatal(err)
	}
	var out [][]imap.Flag
	for _, m := range msgs {
		out = append(out, m.Flags)
	}
	return out
}

func TestIMAPDrafterSave(t *testing.T) {
	addr := startServer(t)
	raw, err := BuildDraft(testDraft())
	if err != nil {
		t.Fatal(err)
	}
	d := NewIMAPDrafter(IMAPConfig{Addr: addr, Username: "user", Password: "pass", Insecure: true})
	if err := d.Save(context.Background(), raw); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got := messages(t, addr, gmailDrafts)
	if len(got) != 1 {
		t.Fatalf("%d Nachrichten in %s, want 1", len(got), gmailDrafts)
	}
	hasDraft := false
	for _, f := range got[0] {
		if f == imap.FlagDraft {
			hasDraft = true
		}
	}
	if !hasDraft {
		t.Errorf("Flags = %v, \\Draft fehlt", got[0])
	}
	for _, other := range []string{"INBOX", "Entwürfe"} {
		if n := len(messages(t, addr, other)); n != 0 {
			t.Errorf("%d Nachrichten in %s", n, other)
		}
	}
}

func TestIMAPDrafterWrongPassword(t *testing.T) {
	addr := startServer(t)
	d := NewIMAPDrafter(IMAPConfig{Addr: addr, Username: "user", Password: "falsch", Insecure: true})
	err := d.Save(context.Background(), []byte("Subject: x\r\n\r\nx"))
	if !errors.Is(err, ErrAuth) {
		t.Fatalf("err = %v, want ErrAuth", err)
	}
}

func TestIMAPDrafterUnavailable(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	d := NewIMAPDrafter(IMAPConfig{Addr: addr, Username: "user", Password: "pass", Insecure: true})
	err = d.Save(context.Background(), []byte("Subject: x\r\n\r\nx"))
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("err = %v, want ErrUnavailable", err)
	}
}

func TestIMAPDrafterNoDraftsMailbox(t *testing.T) {
	user := imapmemserver.NewUser("user", "pass")
	if err := user.Create("INBOX", nil); err != nil {
		t.Fatal(err)
	}
	mem := imapmemserver.New()
	mem.AddUser(user)
	srv := imapserver.New(&imapserver.Options{
		NewSession: func(*imapserver.Conn) (imapserver.Session, *imapserver.GreetingData, error) {
			return mem.NewSession(), nil, nil
		},
		InsecureAuth: true,
		Logger:       log.New(io.Discard, "", 0),
	})
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })

	d := NewIMAPDrafter(IMAPConfig{Addr: ln.Addr().String(), Username: "user", Password: "pass", Insecure: true})
	if err := d.Save(context.Background(), []byte("Subject: x\r\n\r\nx")); !errors.Is(err, ErrNoDrafts) {
		t.Fatalf("err = %v, want ErrNoDrafts", err)
	}
}

func TestIMAPDrafterContextCanceled(t *testing.T) {
	// Server nimmt die Verbindung an, schickt aber nie eine Begrüßung.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				_, _ = io.Copy(io.Discard, conn)
				_ = conn.Close()
			}()
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	d := NewIMAPDrafter(IMAPConfig{Addr: ln.Addr().String(), Username: "user", Password: "pass", Insecure: true})
	start := time.Now()
	err = d.Save(ctx, []byte("Subject: x\r\n\r\nx"))
	if !errors.Is(err, ErrUnavailable) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want ErrUnavailable und DeadlineExceeded", err)
	}
	if time.Since(start) > 5*time.Second {
		t.Errorf("Abbruch dauerte %v", time.Since(start))
	}
}

func TestPickDrafts(t *testing.T) {
	list := func(entries ...*imap.ListData) []*imap.ListData { return entries }
	box := func(name string, attrs ...imap.MailboxAttr) *imap.ListData {
		return &imap.ListData{Mailbox: name, Attrs: attrs}
	}
	cases := []struct {
		name string
		in   []*imap.ListData
		want string
	}{
		{"Special-Use schlägt Namen", list(box("INBOX"), box("[Gmail]/Drafts"), box("Drafts"),
			box("Meine Vorlagen", imap.MailboxAttrHasNoChildren, imap.MailboxAttrDrafts)), "Meine Vorlagen"},
		{"Fallback in fester Reihenfolge", list(box("INBOX"), box("Entwürfe"), box("Drafts")), "Drafts"},
		{"Gmail-Fallback zuerst", list(box("Drafts"), box("[Gmail]/Entwürfe")), "[Gmail]/Entwürfe"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := pickDrafts(tc.in)
			if err != nil || got != tc.want {
				t.Errorf("pickDrafts = %q (%v), want %q", got, err, tc.want)
			}
		})
	}
	for _, in := range [][]*imap.ListData{nil, list(box("INBOX"), box("Sent"))} {
		if _, err := pickDrafts(in); !errors.Is(err, ErrNoDrafts) {
			t.Errorf("err = %v, want ErrNoDrafts", err)
		}
	}
}

func TestServerErrClassification(t *testing.T) {
	imapErr := &imap.Error{Type: imap.StatusResponseTypeNo, Text: "quota"}
	if err := classify("append", imapErr); errors.Is(err, ErrUnavailable) || !errors.Is(err, imapErr) {
		t.Errorf("Serverantwort: %v", err)
	}
	if err := classify("append", io.ErrUnexpectedEOF); !errors.Is(err, ErrUnavailable) || !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Errorf("Netzfehler: %v", err)
	}
	if classify("append", nil) != nil {
		t.Error("nil nicht durchgereicht")
	}
}

// TestIMAPDrafterConnectionDropsAfterLogin: der Server bestätigt LOGIN und
// trennt dann beim nächsten Befehl die Verbindung.
func TestIMAPDrafterConnectionDropsAfterLogin(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		r := bufio.NewReader(conn)
		_, _ = io.WriteString(conn, "* OK [CAPABILITY IMAP4rev1] bereit\r\n")
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			tag, rest, _ := strings.Cut(strings.TrimSpace(line), " ")
			cmd, _, _ := strings.Cut(rest, " ")
			switch strings.ToUpper(cmd) {
			case "CAPABILITY":
				_, _ = io.WriteString(conn, "* CAPABILITY IMAP4rev1\r\n"+tag+" OK fertig\r\n")
			case "LOGIN":
				_, _ = io.WriteString(conn, tag+" OK angemeldet\r\n")
			default:
				return // Verbindung abbrechen
			}
		}
	}()

	d := NewIMAPDrafter(IMAPConfig{Addr: ln.Addr().String(), Username: "user", Password: "pass", Insecure: true})
	err = d.Save(context.Background(), []byte("Subject: x\r\n\r\nx"))
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("err = %v, want ErrUnavailable", err)
	}
}

func TestIMAPConfigRedactsPassword(t *testing.T) {
	cfg := IMAPConfig{Addr: "imap.example.com:993", Username: "user", Password: "geheim123"}
	var logBuf strings.Builder
	slog.New(slog.NewTextHandler(&logBuf, nil)).Info("imap", "cfg", cfg)
	for _, out := range []string{
		cfg.String(), fmt.Sprintf("%v", cfg), fmt.Sprintf("%+v", cfg), (&cfg).String(), logBuf.String(),
	} {
		if strings.Contains(out, "geheim123") {
			t.Errorf("Passwort sichtbar: %s", out)
		}
		if !strings.Contains(out, "imap.example.com:993") || !strings.Contains(out, "user") {
			t.Errorf("Adresse/Benutzer fehlen: %s", out)
		}
	}
}
