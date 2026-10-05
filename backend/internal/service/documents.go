package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	netmail "net/mail"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"bewerbungsmanager/internal/documents"
	"bewerbungsmanager/internal/domain"
	"bewerbungsmanager/internal/mail"
	"bewerbungsmanager/internal/store"
)

const (
	// draftTimeout begrenzt das Ablegen eines Entwurfs per IMAP.
	draftTimeout = 30 * time.Second
	// bookkeepingTimeout begrenzt Zustandsänderungen, die auch nach Abbruch des Aufrufers laufen.
	bookkeepingTimeout = 5 * time.Second
	// maxHighlights: so viele Schwerpunkte ordnen die Kenntnisse höchstens.
	maxHighlights = 8
)

// DocumentsInput sind die stellenbezogenen Texte der Unterlagen.
type DocumentsInput struct {
	Version     int // nur Agent; 0 bei der Oberfläche (= aktuelle Version + 1)
	Language    string
	CoverLetter string
	ProfileLine *string
	Highlights  []string
	MailSubject string
	MailBody    string
}

// Documents ist die aktuelle Fassung der Unterlagen einer Bewerbung.
type Documents struct {
	Version     int
	Language    string
	CoverLetter string
	ProfileLine *string
	Highlights  []string
	MailSubject string
	MailBody    string
	FileName    *string
	RenderedAt  *time.Time
	UpdatedAt   time.Time
}

// renderFailedError: Vorlage oder PDF-Dienst sind gescheitert (nicht: Dienst nicht erreichbar).
type renderFailedError struct{ err error }

func (e *renderFailedError) Error() string { return e.err.Error() }
func (e *renderFailedError) Unwrap() error { return e.err }

// rendered ist das Ergebnis von renderDocuments.
type rendered struct {
	pdf      []byte
	fileName string
	cvName   string
}

// Zustände der Bewerbungsunterlagen (Spalte applications.documents_state).
const (
	DocsNone      = "keine"
	DocsRequested = "angefordert"
	DocsCreated   = "erstellt"
	DocsDrafted   = "entwurf_angelegt"
	DocsPortal    = "portal"
	DocsFailed    = "fehler"
)

var docsStates = []string{DocsNone, DocsRequested, DocsCreated, DocsDrafted, DocsPortal, DocsFailed}

// AgentApplication ist eine Stelle, wie sie der Agent für die Unterlagen braucht.
type AgentApplication struct {
	ID               uuid.UUID
	CompanyName      string
	CompanyWebsite   *string
	PositionTitle    string
	JobURL           *string
	Location         *string
	ContactEmail     *string
	PostingText      *string
	FitReason        *string
	DocumentsState   string
	DocumentsVersion int // 0 = noch keine Unterlagen; der Agent liefert Version DocumentsVersion+1
}

// RequestDocuments gibt eine Stelle für die Unterlagen frei (auch erneut, z. B. nach einem Fehler).
func (s *Service) RequestDocuments(ctx context.Context, id uuid.UUID) (Application, error) {
	err := s.inTx(ctx, func(q *store.Queries) error {
		a, err := q.LockApplication(ctx, id)
		if err != nil {
			return notFoundIfNoRows(err, "Bewerbung")
		}
		if a.DocumentsState == DocsRequested {
			return nil // unverändert lassen: updated_at bestimmt die Reihenfolge der Agent-Liste
		}
		return q.SetDocumentsState(ctx, store.SetDocumentsStateParams{ID: id, DocumentsState: DocsRequested})
	})
	if err != nil {
		return Application{}, err
	}
	return s.GetApplication(ctx, id)
}

// ListAgentApplications liefert Stellen in einem Unterlagen-Zustand, älteste Änderung zuerst.
func (s *Service) ListAgentApplications(ctx context.Context, state string) ([]AgentApplication, error) {
	if !slices.Contains(docsStates, state) {
		return nil, &domain.ValidationError{Field: "documents_state", Detail: "unbekannter Zustand"}
	}
	rows, err := s.queries().ListAgentApplicationsByDocumentsState(ctx, state)
	if err != nil {
		return nil, err
	}
	out := make([]AgentApplication, 0, len(rows))
	for _, r := range rows {
		out = append(out, AgentApplication{
			ID: r.ID, CompanyName: r.CompanyName, CompanyWebsite: r.CompanyWebsite, PositionTitle: r.PositionTitle,
			JobURL: r.JobUrl, Location: r.Location, ContactEmail: r.ContactEmail, PostingText: r.PostingText,
			FitReason: r.FitReason, DocumentsState: r.DocumentsState, DocumentsVersion: int(r.DocumentsVersion),
		})
	}
	return out, nil
}

// SaveAgentDocuments speichert die vom Agenten gelieferten Unterlagen, rendert das PDF und legt –
// mit Bewerbungsadresse und eingerichtetem Drafter – einen Mail-Entwurf an. Eine erneute, gleiche
// Lieferung derselben Version ist ein No-op.
func (s *Service) SaveAgentDocuments(ctx context.Context, id uuid.UUID, in DocumentsInput) (Documents, error) {
	in, err := normalizeDocuments(in, true)
	if err != nil {
		return Documents{}, err
	}
	q := s.queries()
	app, err := q.GetApplication(ctx, id)
	if err != nil {
		return Documents{}, notFoundIfNoRows(err, "Bewerbung")
	}
	stored, has, err := findDocuments(ctx, q, id)
	if err != nil {
		return Documents{}, err
	}
	if has && int(stored.Version) == in.Version && sameDocuments(stored, in) {
		return toDocuments(stored)
	}
	if app.DocumentsState != DocsRequested {
		return Documents{}, &ConflictError{Detail: "Für diese Stelle sind keine Unterlagen angefordert"}
	}
	if has && in.Version <= int(stored.Version) {
		return Documents{}, &ConflictError{Detail: "Version ist veraltet"}
	}

	r, err := s.renderDocuments(ctx, app.CompanyName, app.PositionTitle, in)
	if err != nil {
		return Documents{}, s.handleRenderError(ctx, id, err)
	}

	var (
		saved     store.ApplicationDocument
		contact   *string
		duplicate bool // eine gleiche Lieferung kam während des Renderns zuvor
	)
	err = s.inTx(ctx, func(q *store.Queries) error {
		a, err := q.LockApplication(ctx, id)
		if err != nil {
			return notFoundIfNoRows(err, "Bewerbung")
		}
		cur, has, err := findDocuments(ctx, q, id)
		if err != nil {
			return err
		}
		if has && int(cur.Version) == in.Version && sameDocuments(cur, in) {
			saved, duplicate = cur, true
			return nil
		}
		if a.DocumentsState != DocsRequested {
			return &ConflictError{Detail: "Für diese Stelle sind keine Unterlagen angefordert"}
		}
		if has && in.Version <= int(cur.Version) {
			return &ConflictError{Detail: "Version ist veraltet"}
		}
		if saved, err = q.UpsertDocuments(ctx, s.upsertParams(id, in.Version, in, r)); err != nil {
			return err
		}
		contact = a.ContactEmail
		return q.SetDocumentsState(ctx, store.SetDocumentsStateParams{ID: id, DocumentsState: stateAfterRender(a.ContactEmail)})
	})
	if err != nil {
		return Documents{}, err
	}

	if !duplicate && contact != nil && s.drafter != nil {
		// Die Unterlagen sind gespeichert; ein gescheiterter Entwurf ist kein Fehler des Aufrufs.
		if err := s.saveDraft(ctx, id, r.cvName, *contact, saved); err != nil {
			msg := "Gmail-Entwurf konnte nicht angelegt werden: " + err.Error()
			if err := s.setStateIf(ctx, id, DocsCreated, DocsCreated, &msg); err != nil {
				return Documents{}, err
			}
		}
	}
	return toDocuments(saved)
}

// UpdateDocuments speichert von Hand bearbeitete Unterlagen als neue Version und rendert neu.
// Der Zustand bleibt, nur aus "fehler" wird wieder "erstellt" (ohne Adresse "portal"); ein Entwurf
// wird nicht automatisch angelegt.
func (s *Service) UpdateDocuments(ctx context.Context, id uuid.UUID, in DocumentsInput) (Documents, error) {
	in, err := normalizeDocuments(in, false)
	if err != nil {
		return Documents{}, err
	}
	q := s.queries()
	app, err := q.GetApplication(ctx, id)
	if err != nil {
		return Documents{}, notFoundIfNoRows(err, "Bewerbung")
	}
	stored, has, err := findDocuments(ctx, q, id)
	if err != nil {
		return Documents{}, err
	}
	if !has {
		return Documents{}, &NotFoundError{Resource: "Unterlagen"}
	}

	// Ein Renderfehler beim Bearbeiten ändert nichts: alte Fassung, PDF und Zustand bleiben.
	r, err := s.renderDocuments(ctx, app.CompanyName, app.PositionTitle, in)
	var rf *renderFailedError
	if errors.As(err, &rf) {
		return Documents{}, rf.err
	}
	if err != nil {
		return Documents{}, err
	}

	var saved store.ApplicationDocument
	err = s.inTx(ctx, func(q *store.Queries) error {
		a, err := q.LockApplication(ctx, id)
		if err != nil {
			return notFoundIfNoRows(err, "Bewerbung")
		}
		cur, has, err := findDocuments(ctx, q, id)
		if err != nil {
			return err
		}
		if !has || cur.Version != stored.Version {
			return &ConflictError{Detail: "Die Unterlagen wurden inzwischen geändert"}
		}
		if saved, err = q.UpsertDocuments(ctx, s.upsertParams(id, int(stored.Version)+1, in, r)); err != nil {
			return err
		}
		if a.DocumentsState == DocsFailed {
			return q.SetDocumentsState(ctx, store.SetDocumentsStateParams{ID: id, DocumentsState: stateAfterRender(a.ContactEmail)})
		}
		return nil
	})
	if err != nil {
		return Documents{}, err
	}
	return toDocuments(saved)
}

// GetDocuments liefert die aktuelle Fassung der Unterlagen.
func (s *Service) GetDocuments(ctx context.Context, id uuid.UUID) (Documents, error) {
	d, err := s.queries().GetDocuments(ctx, id)
	if err != nil {
		return Documents{}, notFoundIfNoRows(err, "Unterlagen")
	}
	return toDocuments(d)
}

// DocumentsPDF liefert das gespeicherte PDF (Anschreiben + Lebenslauf) und den Dateinamen.
func (s *Service) DocumentsPDF(ctx context.Context, id uuid.UUID) ([]byte, string, error) {
	d, err := s.queries().GetDocuments(ctx, id)
	if err != nil {
		return nil, "", notFoundIfNoRows(err, "Unterlagen")
	}
	if d.Pdf == nil || d.FileName == nil {
		return nil, "", &NotFoundError{Resource: "PDF"}
	}
	return d.Pdf, *d.FileName, nil
}

// CreateDraft legt (erneut) einen Mail-Entwurf mit den gespeicherten Unterlagen an.
func (s *Service) CreateDraft(ctx context.Context, id uuid.UUID) (Application, error) {
	q := s.queries()
	app, err := q.GetApplication(ctx, id)
	if err != nil {
		return Application{}, notFoundIfNoRows(err, "Bewerbung")
	}
	d, err := q.GetDocuments(ctx, id)
	if err != nil {
		return Application{}, notFoundIfNoRows(err, "Unterlagen")
	}
	if d.Pdf == nil || d.FileName == nil {
		return Application{}, &NotFoundError{Resource: "PDF"}
	}
	if app.DocumentsState == DocsRequested {
		return Application{}, &ConflictError{Detail: "Unterlagen werden gerade erstellt"}
	}
	if app.ContactEmail == nil {
		return Application{}, &domain.ValidationError{Field: "contact_email", Detail: "keine Bewerbungsadresse hinterlegt"}
	}
	if _, err := netmail.ParseAddress(*app.ContactEmail); err != nil {
		return Application{}, &domain.ValidationError{Field: "contact_email", Detail: "ist keine gültige Mail-Adresse"}
	}
	if s.drafter == nil {
		return Application{}, &UnavailableError{Detail: "Gmail ist nicht eingerichtet (GMAIL_ADDRESS fehlt)"}
	}
	name, err := s.cvName(ctx)
	if err != nil {
		return Application{}, err
	}
	if err := s.saveDraft(ctx, id, name, *app.ContactEmail, d); err != nil {
		switch {
		case errors.Is(err, mail.ErrAuth):
			return Application{}, &ConflictError{Detail: "Gmail-Anmeldung fehlgeschlagen – App-Passwort prüfen"}
		case errors.Is(err, mail.ErrNoDrafts):
			return Application{}, &ConflictError{Detail: "Im Postfach gibt es keinen Entwurfsordner"}
		case errors.Is(err, mail.ErrUnavailable), errors.Is(err, context.DeadlineExceeded):
			return Application{}, &UnavailableError{Detail: "Gmail ist gerade nicht erreichbar", Err: err}
		}
		return Application{}, err
	}
	return s.GetApplication(ctx, id)
}

// renderDocuments rendert Anschreiben + Lebenslauf als PDF. Ohne Lebenslauf: ConflictError;
// PDF-Dienst fehlt oder ist nicht erreichbar: UnavailableError; sonst renderFailedError.
func (s *Service) renderDocuments(ctx context.Context, company, position string, in DocumentsInput) (rendered, error) {
	cv, photo, err := s.loadCV(ctx)
	var nf *NotFoundError
	if errors.As(err, &nf) {
		return rendered{}, &ConflictError{Detail: "Erst den Lebenslauf speichern"}
	}
	if err != nil {
		return rendered{}, err
	}
	letter := documents.Letter{
		Language: in.Language, CompanyName: company, PositionTitle: position,
		CoverLetter: in.CoverLetter, Highlights: in.Highlights, Date: s.now().In(time.Local),
	}
	if in.ProfileLine != nil {
		letter.ProfileLine = *in.ProfileLine
	}
	html, err := documents.RenderApplication(cv, photo, letter)
	if err != nil {
		return rendered{}, &renderFailedError{err: err}
	}
	pdf, err := s.convertPDF(ctx, html)
	var ue *UnavailableError
	if errors.As(err, &ue) {
		return rendered{}, err
	}
	if err != nil {
		return rendered{}, &renderFailedError{err: err}
	}
	return rendered{pdf: pdf, fileName: documents.ApplicationFileName(cv, company), cvName: strings.TrimSpace(cv.Person.Name)}, nil
}

// handleRenderError hält einen echten Renderfehler im Zustand "fehler" fest (nur wenn die Stelle
// noch "angefordert" ist) und gibt ihn zurück.
func (s *Service) handleRenderError(ctx context.Context, id uuid.UUID, err error) error {
	var rf *renderFailedError
	if !errors.As(err, &rf) {
		return err
	}
	msg := "PDF konnte nicht erzeugt werden: " + rf.err.Error()
	if serr := s.setStateIf(ctx, id, DocsRequested, DocsFailed, &msg); serr != nil {
		return errors.Join(rf.err, serr)
	}
	return rf.err
}

// setStateIf setzt den Zustand nur, wenn er noch expected ist – ein inzwischen neuerer Zustand
// (z. B. eine neue Anforderung) bleibt. Läuft auch nach Abbruch des Aufrufers.
func (s *Service) setStateIf(ctx context.Context, id uuid.UUID, expected, state string, errMsg *string) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), bookkeepingTimeout)
	defer cancel()
	_, err := s.queries().SetDocumentsStateIf(ctx, store.SetDocumentsStateIfParams{
		ID: id, ExpectedState: expected, NewState: state, DocumentsError: errMsg,
	})
	return err
}

// saveDraft baut die Nachricht, legt sie per Drafter ab und vermerkt den Entwurf.
// Läuft auch nach Abbruch des Aufrufers weiter, damit ein abgelegter Entwurf vermerkt wird.
func (s *Service) saveDraft(ctx context.Context, id uuid.UUID, name, to string, d store.ApplicationDocument) error {
	if d.FileName == nil {
		return errors.New("unterlagen ohne dateiname")
	}
	msg, err := mail.BuildDraft(mail.Draft{
		From:     (&netmail.Address{Name: name, Address: s.draftFrom}).String(),
		To:       to,
		Subject:  d.MailSubject,
		Body:     d.MailBody,
		FileName: *d.FileName,
		PDF:      d.Pdf,
		Date:     s.now(),
	})
	if err != nil {
		return err
	}
	imapCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), draftTimeout)
	defer cancel()
	if err := s.drafter.Save(imapCtx, msg); err != nil {
		return err
	}
	dbCtx, cancelDB := context.WithTimeout(context.WithoutCancel(ctx), bookkeepingTimeout)
	defer cancelDB()
	return s.queries().SetDraftCreated(dbCtx, id)
}

// cvName liefert den Namen aus dem gespeicherten Lebenslauf ("" ohne Lebenslauf).
func (s *Service) cvName(ctx context.Context) (string, error) {
	cv, err := s.GetCV(ctx)
	if err != nil || cv.Data == nil {
		return "", err
	}
	doc, err := documents.ParseCV(cv.Data)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(doc.Person.Name), nil
}

func (s *Service) upsertParams(id uuid.UUID, version int, in DocumentsInput, r rendered) store.UpsertDocumentsParams {
	highlights, _ := json.Marshal(in.Highlights) // []string lässt sich immer serialisieren
	now := s.now()
	return store.UpsertDocumentsParams{
		ApplicationID: id, Version: int32(version), Language: in.Language, CoverLetter: in.CoverLetter,
		ProfileLine: in.ProfileLine, Highlights: highlights, MailSubject: in.MailSubject, MailBody: in.MailBody,
		Pdf: r.pdf, FileName: &r.fileName, RenderedAt: &now,
	}
}

// stateAfterRender: mit Bewerbungsadresse "erstellt", sonst "portal".
func stateAfterRender(contact *string) string {
	if contact == nil {
		return DocsPortal
	}
	return DocsCreated
}

func findDocuments(ctx context.Context, q *store.Queries, id uuid.UUID) (store.ApplicationDocument, bool, error) {
	d, err := q.GetDocuments(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.ApplicationDocument{}, false, nil
	}
	return d, err == nil, err
}

// normalizeDocuments prüft und bereinigt die Eingabe; agent verlangt eine Version ≥ 1.
func normalizeDocuments(in DocumentsInput, agent bool) (DocumentsInput, error) {
	if agent && in.Version < 1 {
		return in, &domain.ValidationError{Field: "version", Detail: "muss mindestens 1 sein"}
	}
	if err := rejectNUL(in); err != nil {
		return in, err
	}
	in.Language = strings.TrimSpace(in.Language)
	if in.Language != "de" && in.Language != "en" {
		return in, &domain.ValidationError{Field: "language", Detail: "muss de oder en sein"}
	}
	var err error
	if in.CoverLetter, err = requireText("cover_letter", in.CoverLetter); err != nil {
		return in, err
	}
	if in.MailSubject, err = requireText("mail_subject", in.MailSubject); err != nil {
		return in, err
	}
	if strings.ContainsAny(in.MailSubject, "\r\n") {
		return in, &domain.ValidationError{Field: "mail_subject", Detail: "darf keinen Zeilenumbruch enthalten"}
	}
	if in.MailBody, err = requireText("mail_body", in.MailBody); err != nil {
		return in, err
	}
	in.ProfileLine = cleanOptional(in.ProfileLine)
	if in.ProfileLine != nil && strings.ContainsAny(*in.ProfileLine, "\r\n") {
		return in, &domain.ValidationError{Field: "profile_line", Detail: "muss eine Zeile sein"}
	}
	highlights := make([]string, 0, len(in.Highlights))
	for _, h := range in.Highlights {
		if h = strings.TrimSpace(h); h != "" {
			highlights = append(highlights, h)
		}
	}
	if len(highlights) > maxHighlights {
		return in, &domain.ValidationError{Field: "highlights", Detail: fmt.Sprintf("höchstens %d Einträge", maxHighlights)}
	}
	in.Highlights = highlights
	return in, nil
}

// sameDocuments vergleicht den Inhalt (ohne Version und PDF).
func sameDocuments(d store.ApplicationDocument, in DocumentsInput) bool {
	var highlights []string
	if err := json.Unmarshal(d.Highlights, &highlights); err != nil {
		return false
	}
	profileEqual := (d.ProfileLine == nil) == (in.ProfileLine == nil) &&
		(d.ProfileLine == nil || *d.ProfileLine == *in.ProfileLine)
	return d.Language == in.Language && d.CoverLetter == in.CoverLetter && profileEqual &&
		slices.Equal(highlights, in.Highlights) && d.MailSubject == in.MailSubject && d.MailBody == in.MailBody
}

func toDocuments(d store.ApplicationDocument) (Documents, error) {
	highlights := []string{}
	if err := json.Unmarshal(d.Highlights, &highlights); err != nil {
		return Documents{}, fmt.Errorf("schwerpunkte lesen: %w", err)
	}
	return Documents{
		Version: int(d.Version), Language: d.Language, CoverLetter: d.CoverLetter, ProfileLine: d.ProfileLine,
		Highlights: highlights, MailSubject: d.MailSubject, MailBody: d.MailBody,
		FileName: d.FileName, RenderedAt: d.RenderedAt, UpdatedAt: d.UpdatedAt,
	}, nil
}

// rejectNUL weist NUL-Zeichen ab; PostgreSQL kann sie in text/jsonb nicht speichern.
func rejectNUL(in DocumentsInput) error {
	fields := []struct{ name, value string }{
		{"language", in.Language}, {"cover_letter", in.CoverLetter},
		{"mail_subject", in.MailSubject}, {"mail_body", in.MailBody},
	}
	if in.ProfileLine != nil {
		fields = append(fields, struct{ name, value string }{"profile_line", *in.ProfileLine})
	}
	for _, h := range in.Highlights {
		fields = append(fields, struct{ name, value string }{"highlights", h})
	}
	for _, f := range fields {
		if strings.ContainsRune(f.value, 0) {
			return &domain.ValidationError{Field: f.name, Detail: "darf kein NUL-Zeichen enthalten"}
		}
	}
	return nil
}
