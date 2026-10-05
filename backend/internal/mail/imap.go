package mail

import (
	"context"
	"errors"
	"fmt"
	"net"
	"slices"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

// IMAPConfig beschreibt das Postfach, in dem Entwürfe abgelegt werden.
type IMAPConfig struct {
	Addr               string // z. B. "imap.gmail.com:993"
	Username, Password string
	Insecure           bool // nur für Tests: ohne TLS
}

var (
	// ErrAuth: Benutzername oder (App-)Passwort falsch.
	ErrAuth = errors.New("imap: anmeldung fehlgeschlagen")
	// ErrUnavailable: Server nicht erreichbar, Verbindung abgebrochen oder Zeit überschritten.
	ErrUnavailable = errors.New("imap: server nicht erreichbar")
	// ErrNoDrafts: weder ein Ordner mit \Drafts noch einer der bekannten Namen existiert.
	ErrNoDrafts = errors.New("imap: kein entwurfsordner gefunden")
)

const (
	// dialTimeout begrenzt den Verbindungsaufbau.
	dialTimeout = 15 * time.Second
	// saveTimeout begrenzt den gesamten Vorgang, falls der Aufrufer keine Frist setzt.
	saveTimeout = 2 * time.Minute
)

// fallbackDrafts: Ordnernamen, falls der Server keine Special-Use-Attribute liefert.
var fallbackDrafts = []string{"[Gmail]/Drafts", "[Gmail]/Entwürfe", "Drafts", "Entwürfe"}

// IMAPDrafter legt Nachrichten per IMAP APPEND im Entwurfsordner ab.
// Er kennt kein SMTP und kann nichts senden.
type IMAPDrafter struct{ cfg IMAPConfig }

func NewIMAPDrafter(cfg IMAPConfig) *IMAPDrafter {
	return &IMAPDrafter{cfg: cfg}
}

// Save legt msg mit Flag \Draft im Ordner mit Special-Use \Drafts ab
// (Fallback: "[Gmail]/Drafts", "[Gmail]/Entwürfe", "Drafts", "Entwürfe").
func (d *IMAPDrafter) Save(ctx context.Context, msg []byte) (err error) {
	ctx, cancel := context.WithTimeout(ctx, saveTimeout)
	defer cancel()

	opts := &imapclient.Options{Dialer: &net.Dialer{Timeout: dialTimeout}}
	var c *imapclient.Client
	if d.cfg.Insecure {
		c, err = imapclient.DialInsecure(d.cfg.Addr, opts)
	} else {
		c, err = imapclient.DialTLS(d.cfg.Addr, opts)
	}
	if err != nil {
		return fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	// Kontext-Abbruch schließt die Verbindung; laufende Befehle kehren dann mit Fehler zurück.
	stop := context.AfterFunc(ctx, func() { _ = c.Close() })
	defer func() {
		stop()
		_ = c.Close()
		if ctxErr := ctx.Err(); ctxErr != nil && err != nil {
			err = fmt.Errorf("%w: %w", ErrUnavailable, ctxErr)
		}
	}()

	if err := c.WaitGreeting(); err != nil {
		return fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	if err := c.Login(d.cfg.Username, d.cfg.Password).Wait(); err != nil {
		var imapErr *imap.Error
		if errors.As(err, &imapErr) && imapErr.Type == imap.StatusResponseTypeNo {
			return fmt.Errorf("%w: %w", ErrAuth, err)
		}
		return fmt.Errorf("%w: %w", ErrUnavailable, err)
	}

	mailbox, err := draftsMailbox(c)
	if err != nil {
		return err
	}

	app := c.Append(mailbox, int64(len(msg)), &imap.AppendOptions{Flags: []imap.Flag{imap.FlagDraft}})
	if _, err := app.Write(msg); err != nil {
		_ = app.Close()
		return fmt.Errorf("imap: entwurf übertragen: %w", err)
	}
	if err := app.Close(); err != nil {
		return fmt.Errorf("imap: entwurf übertragen: %w", err)
	}
	if _, err := app.Wait(); err != nil {
		return fmt.Errorf("imap: entwurf ablegen in %q: %w", mailbox, err)
	}
	// Der Entwurf liegt; ein Fehler beim Abmelden ändert daran nichts.
	_ = c.Logout().Wait()
	return nil
}

// draftsMailbox sucht den Ordner mit Attribut \Drafts, sonst einen der bekannten Namen.
func draftsMailbox(c *imapclient.Client) (string, error) {
	var opts *imap.ListOptions
	if c.Caps().Has(imap.CapSpecialUse) {
		opts = &imap.ListOptions{ReturnSpecialUse: true}
	}
	list, err := c.List("", "*", opts).Collect()
	if err != nil {
		return "", fmt.Errorf("imap: ordner auflisten: %w", err)
	}
	names := make([]string, 0, len(list))
	for _, mb := range list {
		if slices.Contains(mb.Attrs, imap.MailboxAttrDrafts) {
			return mb.Mailbox, nil
		}
		names = append(names, mb.Mailbox)
	}
	for _, name := range fallbackDrafts {
		if slices.Contains(names, name) {
			return name, nil
		}
	}
	return "", ErrNoDrafts
}
