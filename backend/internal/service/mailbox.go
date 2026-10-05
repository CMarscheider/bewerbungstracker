package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"bewerbungsmanager/internal/domain"
	"bewerbungsmanager/internal/store"
)

// Zustände eines Vorschlags (status_suggestions.state).
const (
	SuggestionOpen      = "offen"
	SuggestionAccepted  = "angenommen"
	SuggestionDismissed = "verworfen"
)

// agentNotePrefix kennzeichnet Ereignisse, die der Agent angelegt hat.
const agentNotePrefix = "Agent: "

const (
	maxReasonLen  = 1000
	maxOutcomeLen = 200
	maxThreadLen  = 100
)

// defaultProcessedMailCap begrenzt neu gemerkte Mails je 24 Stunden – Schutz davor, dass eine
// manipulierte Mail den Agenten dazu bringt, massenhaft Mails als erledigt zu verstecken.
const defaultProcessedMailCap = 200

// withProcessedMailCap setzt die Obergrenze (nur für Tests, siehe export_test.go).
func withProcessedMailCap(n int) Option { return func(s *Service) { s.mailCap = n } }

// agentBookable sind die Ereignisse, die der Agent direkt anlegen darf. Alles andere (z. B. Zusagen,
// Rückzug) entscheidet der Nutzer über einen Vorschlag.
var agentBookable = []domain.EventType{
	domain.Beworben, domain.ScreeningGespraech, domain.ChallengeErhalten, domain.Interview,
	domain.Kennenlerntag, domain.AngebotErhalten, domain.Absage,
}

// gmailIDPattern entspricht GmailMessageId in der API-Spec.
var gmailIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,200}$`)

// Unicode-Zeilen- und Absatztrenner (Kategorie Zl/Zp, nicht Cc/Cf).
const (
	lineSeparator      = 0x2028
	paragraphSeparator = 0x2029
)

// rejectInvisible lehnt Steuer- und Formatzeichen (Cc, Cf, U+2028/U+2029) ab; sie können in Mails
// eingeschleuste Inhalte verbergen. Auch Tab und Zeilenumbruch sind nicht erlaubt.
func rejectInvisible(field, value string) error {
	for _, r := range value {
		if unicode.Is(unicode.Cc, r) || unicode.Is(unicode.Cf, r) || r == lineSeparator || r == paragraphSeparator {
			return &domain.ValidationError{Field: field, Detail: fmt.Sprintf("enthält unsichtbares Zeichen %U", r)}
		}
	}
	return nil
}

func rejectInvisibleOptional(field string, value *string) error {
	if value == nil {
		return nil
	}
	return rejectInvisible(field, *value)
}

// OpenApplication ist eine laufende Bewerbung, wie sie die Postfach-Auswertung braucht.
type OpenApplication struct {
	ID             uuid.UUID
	CompanyName    string
	CompanyWebsite *string
	PositionTitle  string
	Status         domain.EventType
	ContactEmail   *string
	GmailThreadID  *string
	DocumentsState string
	GmailDraftAt   *time.Time
	MailSubject    *string // Betreff der Bewerbungsmail aus den Unterlagen, falls vorhanden
	UpdatedAt      time.Time
	AllowedEvents  []domain.EventType
}

// NewSuggestion ist ein Vorschlag des Agenten; ApplicationID nil = nicht zugeordnet.
type NewSuggestion struct {
	ApplicationID *uuid.UUID
	SuggestedType domain.EventType
	OccurredOn    time.Time
	DueOn         *time.Time
	Reason        string
	MailSubject   *string
	MailFrom      *string
	MailURL       *string
	// GmailMessageID ist die Mail, aus der der Vorschlag stammt; pro Mail gibt es höchstens einen Vorschlag.
	GmailMessageID *string
}

// Suggestion ist ein gespeicherter Vorschlag; Firma und Stelle fehlen bei nicht zugeordneten.
type Suggestion struct {
	ID             uuid.UUID
	ApplicationID  *uuid.UUID
	CompanyName    *string
	PositionTitle  *string
	SuggestedType  domain.EventType
	OccurredOn     time.Time
	DueOn          *time.Time
	Reason         string
	MailSubject    *string
	MailFrom       *string
	MailURL        *string
	GmailMessageID *string
	State          string
	CreatedAt      time.Time
	DecidedAt      *time.Time
}

// ProcessedMail ist eine bereits ausgewertete Mail.
type ProcessedMail struct {
	GmailMessageID string
	ApplicationID  *uuid.UUID
	CompanyName    *string // nur in ListProcessedMails
	PositionTitle  *string // nur in ListProcessedMails
	Outcome        string
	ProcessedAt    time.Time
}

// ListOpenAgentApplications liefert alle nicht abgeschlossenen Bewerbungen sowie solche mit
// "Keine Rückmeldung" (späte Antworten nach Ghosting), sortiert nach Firma.
func (s *Service) ListOpenAgentApplications(ctx context.Context) ([]OpenApplication, error) {
	statuses := []string{string(domain.KeineRueckmeldung)}
	for _, p := range []domain.Phase{domain.PhaseVorbereitung, domain.PhaseAktiv} {
		for _, t := range domain.StatusesInPhase(p) {
			statuses = append(statuses, string(t))
		}
	}
	var out []OpenApplication
	err := s.inReadTx(ctx, func(q *store.Queries) error {
		rows, err := q.ListOpenAgentApplications(ctx, statuses)
		if err != nil {
			return err
		}
		out = make([]OpenApplication, 0, len(rows))
		for _, r := range rows {
			status := domain.EventType(r.CurrentStatus)
			allowed, err := openAllowedEvents(ctx, q, r.ID, status)
			if err != nil {
				return err
			}
			out = append(out, OpenApplication{
				ID: r.ID, CompanyName: r.CompanyName, CompanyWebsite: r.CompanyWebsite, PositionTitle: r.PositionTitle,
				Status: status, ContactEmail: r.ContactEmail, GmailThreadID: r.GmailThreadID,
				DocumentsState: r.DocumentsState, GmailDraftAt: r.GmailDraftAt, MailSubject: r.MailSubject,
				UpdatedAt: r.UpdatedAt, AllowedEvents: allowed,
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// openAllowedEvents liefert die erlaubten nächsten Ereignisse. Nach KeineRueckmeldung hängen sie vom
// Verlauf davor ab und werden wie in AllowedEvents aus der ganzen Historie bestimmt; in den Phasen
// Vorbereitung und Aktiv genügt der letzte Status.
func openAllowedEvents(ctx context.Context, q *store.Queries, appID uuid.UUID, status domain.EventType) ([]domain.EventType, error) {
	if status != domain.KeineRueckmeldung {
		return domain.AllowedNext([]domain.EventType{status}), nil
	}
	rows, err := q.ListEvents(ctx, appID)
	if err != nil {
		return nil, err
	}
	return domain.AllowedNext(domain.Types(toHistory(rows))), nil
}

// AgentAddEvent legt ein Ereignis wie AddEvent an und kennzeichnet die Notiz mit "Agent: ".
func (s *Service) AgentAddEvent(ctx context.Context, appID uuid.UUID, next domain.NewEvent) (Event, error) {
	if !slices.Contains(agentBookable, next.Type) {
		return Event{}, &domain.ValidationError{Field: "type",
			Detail: fmt.Sprintf("%s darf der Agent nicht direkt anlegen – stattdessen einen Vorschlag ablegen", next.Type)}
	}
	if err := rejectInvisibleOptional("note", next.Note); err != nil {
		return Event{}, err
	}
	next.Note = agentNote(next.Note)
	return s.AddEvent(ctx, appID, next)
}

// agentNote ergänzt das Präfix "Agent: ", ohne es zu verdoppeln.
func agentNote(note *string) *string {
	text := ""
	if n := cleanOptional(note); n != nil {
		text = *n
	}
	if strings.HasPrefix(text, agentNotePrefix) {
		return &text
	}
	if text == "" {
		text = "automatisch erfasst"
	}
	text = agentNotePrefix + text
	return &text
}

// SetGmailThread merkt sich den Gmail-Thread der gesendeten Bewerbung. Dieselbe ID erneut ist ein No-op;
// eine andere ID für dieselbe Bewerbung oder eine schon vergebene ID ergibt einen ConflictError.
func (s *Service) SetGmailThread(ctx context.Context, appID uuid.UUID, threadID string) error {
	threadID, err := requireText("gmail_thread_id", threadID)
	if err != nil {
		return err
	}
	if utf8.RuneCountInString(threadID) > maxThreadLen {
		return &domain.ValidationError{Field: "gmail_thread_id", Detail: fmt.Sprintf("höchstens %d Zeichen", maxThreadLen)}
	}
	return s.inTx(ctx, func(q *store.Queries) error {
		a, err := q.LockApplication(ctx, appID)
		if err != nil {
			return notFoundIfNoRows(err, "Bewerbung")
		}
		if a.GmailThreadID != nil {
			if *a.GmailThreadID == threadID {
				return nil
			}
			return &ConflictError{Detail: "Für diese Bewerbung ist bereits ein anderer Gmail-Thread hinterlegt"}
		}
		err = q.SetGmailThread(ctx, store.SetGmailThreadParams{ID: appID, GmailThreadID: &threadID})
		if isUniqueViolation(err) {
			return &ConflictError{Detail: "Dieser Gmail-Thread gehört schon zu einer anderen Bewerbung"}
		}
		return err
	})
}

// CreateSuggestion legt einen offenen Vorschlag an. Gibt es zur selben GmailMessageID schon einen
// (egal in welchem Zustand), wird dieser unverändert zurückgegeben und created ist false.
func (s *Service) CreateSuggestion(ctx context.Context, in NewSuggestion) (Suggestion, bool, error) {
	if err := rejectInvisible("reason", in.Reason); err != nil {
		return Suggestion{}, false, err
	}
	mailFields := []struct {
		name  string
		value *string
	}{{"mail_subject", in.MailSubject}, {"mail_from", in.MailFrom}, {"mail_url", in.MailURL}}
	for _, f := range mailFields {
		if err := rejectInvisibleOptional(f.name, f.value); err != nil {
			return Suggestion{}, false, err
		}
	}
	if in.GmailMessageID != nil && !gmailIDPattern.MatchString(*in.GmailMessageID) {
		return Suggestion{}, false, &domain.ValidationError{Field: "gmail_message_id", Detail: "nur A–Z, a–z, 0–9, _ und -, höchstens 200 Zeichen"}
	}
	if !in.SuggestedType.Valid() {
		return Suggestion{}, false, &domain.ValidationError{Field: "suggested_type", Detail: fmt.Sprintf("unbekannter Ereignistyp %q", in.SuggestedType)}
	}
	if in.OccurredOn.IsZero() {
		return Suggestion{}, false, &domain.ValidationError{Field: "occurred_on", Detail: "fehlt"}
	}
	if in.DueOn != nil && !in.SuggestedType.AllowsDeadline() {
		return Suggestion{}, false, &domain.ValidationError{Field: "due_on", Detail: fmt.Sprintf("%s hat keine Frist", in.SuggestedType)}
	}
	reason, err := requireText("reason", in.Reason)
	if err != nil {
		return Suggestion{}, false, err
	}
	if utf8.RuneCountInString(reason) > maxReasonLen {
		return Suggestion{}, false, &domain.ValidationError{Field: "reason", Detail: fmt.Sprintf("höchstens %d Zeichen", maxReasonLen)}
	}
	var due *time.Time
	if in.DueOn != nil {
		d := domain.DateOf(*in.DueOn)
		due = &d
	}
	var (
		row     store.GetSuggestionRow
		created bool
	)
	err = s.inTx(ctx, func(q *store.Queries) error {
		inserted, err := q.InsertSuggestion(ctx, store.InsertSuggestionParams{
			ApplicationID:  in.ApplicationID,
			SuggestedType:  string(in.SuggestedType),
			OccurredOn:     domain.DateOf(in.OccurredOn),
			DueOn:          due,
			Reason:         reason,
			MailSubject:    cleanOptional(in.MailSubject),
			MailFrom:       cleanOptional(in.MailFrom),
			MailUrl:        cleanOptional(in.MailURL),
			GmailMessageID: in.GmailMessageID,
		})
		id := inserted.ID
		switch {
		case isForeignKeyViolation(err):
			return &NotFoundError{Resource: "Bewerbung"}
		case errors.Is(err, pgx.ErrNoRows): // ON CONFLICT DO NOTHING: zur Mail gibt es schon einen Vorschlag
			if id, err = q.GetSuggestionIDByMessage(ctx, in.GmailMessageID); err != nil {
				return err
			}
		case err != nil:
			return err
		default:
			created = true
		}
		row, err = q.GetSuggestion(ctx, id)
		return err
	})
	if err != nil {
		return Suggestion{}, false, err
	}
	return toSuggestion(row), created, nil
}

// ListOpenSuggestions liefert die offenen Vorschläge, älteste zuerst.
func (s *Service) ListOpenSuggestions(ctx context.Context) ([]Suggestion, error) {
	rows, err := s.queries().ListOpenSuggestions(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Suggestion, 0, len(rows))
	for _, r := range rows {
		out = append(out, toSuggestion(store.GetSuggestionRow(r)))
	}
	return out, nil
}

// AcceptSuggestion legt das vorgeschlagene Ereignis an und markiert den Vorschlag als angenommen.
// appID ordnet einen nicht zugeordneten Vorschlag zu; bei zugeordneten hat sie Vorrang.
// Scheitert das Ereignis, bleibt der Vorschlag offen.
func (s *Service) AcceptSuggestion(ctx context.Context, id uuid.UUID, appID *uuid.UUID) (Application, error) {
	var target uuid.UUID
	err := s.inTx(ctx, func(q *store.Queries) error {
		sg, err := lockOpenSuggestion(ctx, q, id)
		if err != nil {
			return err
		}
		switch {
		case appID != nil:
			target = *appID
		case sg.ApplicationID != nil:
			target = *sg.ApplicationID
		default:
			return &domain.ValidationError{Field: "application_id", Detail: "Vorschlag ist keiner Bewerbung zugeordnet"}
		}
		if _, err := s.addEvent(ctx, q, target, domain.NewEvent{
			Type: domain.EventType(sg.SuggestedType), OccurredOn: sg.OccurredOn, DueOn: sg.DueOn, Note: agentNote(&sg.Reason),
		}); err != nil {
			return err
		}
		return q.DecideSuggestion(ctx, store.DecideSuggestionParams{ID: id, State: SuggestionAccepted, ApplicationID: &target})
	})
	if err != nil {
		return Application{}, err
	}
	return s.GetApplication(ctx, target)
}

// DismissSuggestion verwirft einen offenen Vorschlag.
func (s *Service) DismissSuggestion(ctx context.Context, id uuid.UUID) error {
	return s.inTx(ctx, func(q *store.Queries) error {
		if _, err := lockOpenSuggestion(ctx, q, id); err != nil {
			return err
		}
		return q.DecideSuggestion(ctx, store.DecideSuggestionParams{ID: id, State: SuggestionDismissed})
	})
}

// lockOpenSuggestion sperrt einen Vorschlag; bereits entschiedene ergeben einen ConflictError.
func lockOpenSuggestion(ctx context.Context, q *store.Queries, id uuid.UUID) (store.StatusSuggestion, error) {
	sg, err := q.LockSuggestion(ctx, id)
	if err != nil {
		return sg, notFoundIfNoRows(err, "Vorschlag")
	}
	if sg.State != SuggestionOpen {
		return sg, &ConflictError{Detail: "Der Vorschlag wurde schon " + sg.State}
	}
	return sg, nil
}

// GetProcessedMail liefert eine bereits ausgewertete Mail oder NotFoundError.
func (s *Service) GetProcessedMail(ctx context.Context, messageID string) (ProcessedMail, error) {
	m, err := s.queries().GetProcessedMail(ctx, messageID)
	if err != nil {
		return ProcessedMail{}, notFoundIfNoRows(err, "Verarbeitete Mail")
	}
	return toProcessedMail(m), nil
}

// MarkMailProcessed merkt eine Mail als ausgewertet. created ist false, wenn sie schon gemerkt war;
// dann bleibt der vorhandene Eintrag unverändert und wird zurückgegeben.
func (s *Service) MarkMailProcessed(ctx context.Context, messageID string, appID *uuid.UUID, outcome string) (ProcessedMail, bool, error) {
	messageID, err := requireText("gmail_message_id", messageID)
	if err != nil {
		return ProcessedMail{}, false, err
	}
	outcome, err = requireText("outcome", outcome)
	if err != nil {
		return ProcessedMail{}, false, err
	}
	if err := rejectInvisible("outcome", outcome); err != nil {
		return ProcessedMail{}, false, err
	}
	if utf8.RuneCountInString(outcome) > maxOutcomeLen {
		return ProcessedMail{}, false, &domain.ValidationError{Field: "outcome", Detail: fmt.Sprintf("höchstens %d Zeichen", maxOutcomeLen)}
	}
	var (
		m       store.ProcessedMail
		created bool
	)
	err = s.inTx(ctx, func(q *store.Queries) error {
		if err := q.LockProcessedMails(ctx); err != nil {
			return err
		}
		existing, err := q.GetProcessedMail(ctx, messageID)
		switch {
		case err == nil:
			m = existing
			return nil
		case !errors.Is(err, pgx.ErrNoRows):
			return err
		}
		n, err := q.CountRecentProcessedMails(ctx)
		if err != nil {
			return err
		}
		if n >= int64(s.mailCap) {
			return &ConflictError{Detail: fmt.Sprintf("Obergrenze von %d neu ausgewerteten Mails in 24 Stunden erreicht – bitte in der Oberfläche prüfen", s.mailCap)}
		}
		m, err = q.InsertProcessedMail(ctx, store.InsertProcessedMailParams{
			GmailMessageID: messageID, ApplicationID: appID, Outcome: outcome,
		})
		switch {
		case isForeignKeyViolation(err):
			return &NotFoundError{Resource: "Bewerbung"}
		case errors.Is(err, pgx.ErrNoRows): // ON CONFLICT DO NOTHING: schon vorhanden
			m, err = q.GetProcessedMail(ctx, messageID)
			return err
		case err != nil:
			return err
		}
		created = true
		return nil
	})
	if err != nil {
		return ProcessedMail{}, false, err
	}
	return toProcessedMail(m), created, nil
}

func toSuggestion(r store.GetSuggestionRow) Suggestion {
	return Suggestion{
		ID: r.ID, ApplicationID: r.ApplicationID, CompanyName: r.CompanyName, PositionTitle: r.PositionTitle,
		SuggestedType: domain.EventType(r.SuggestedType), OccurredOn: r.OccurredOn, DueOn: r.DueOn,
		Reason: r.Reason, MailSubject: r.MailSubject, MailFrom: r.MailFrom, MailURL: r.MailUrl,
		GmailMessageID: r.GmailMessageID, State: r.State, CreatedAt: r.CreatedAt, DecidedAt: r.DecidedAt,
	}
}

func toProcessedMail(m store.ProcessedMail) ProcessedMail {
	return ProcessedMail{
		GmailMessageID: m.GmailMessageID, ApplicationID: m.ApplicationID, Outcome: m.Outcome, ProcessedAt: m.ProcessedAt,
	}
}

// ClearGmailThread löst eine falsche Thread-Zuordnung; ohne Zuordnung ist es ein No-op.
func (s *Service) ClearGmailThread(ctx context.Context, appID uuid.UUID) error {
	return s.inTx(ctx, func(q *store.Queries) error {
		a, err := q.LockApplication(ctx, appID)
		if err != nil {
			return notFoundIfNoRows(err, "Bewerbung")
		}
		if a.GmailThreadID == nil {
			return nil
		}
		return q.SetGmailThread(ctx, store.SetGmailThreadParams{ID: appID})
	})
}

// maxProcessedMailList begrenzt ListProcessedMails.
const maxProcessedMailList = 200

// ListProcessedMails liefert die zuletzt ausgewerteten Mails, neueste zuerst, mit Firma und Stelle.
func (s *Service) ListProcessedMails(ctx context.Context, limit int) ([]ProcessedMail, error) {
	limit = min(max(limit, 1), maxProcessedMailList)
	rows, err := s.queries().ListProcessedMails(ctx, int32(limit)) //nolint:gosec // auf 1–200 begrenzt
	if err != nil {
		return nil, err
	}
	out := make([]ProcessedMail, 0, len(rows))
	for _, r := range rows {
		out = append(out, ProcessedMail{
			GmailMessageID: r.GmailMessageID, ApplicationID: r.ApplicationID, CompanyName: r.CompanyName,
			PositionTitle: r.PositionTitle, Outcome: r.Outcome, ProcessedAt: r.ProcessedAt,
		})
	}
	return out, nil
}

// DeleteProcessedMail vergisst eine ausgewertete Mail, damit der Agent sie erneut auswertet; idempotent.
func (s *Service) DeleteProcessedMail(ctx context.Context, messageID string) error {
	return s.queries().DeleteProcessedMail(ctx, messageID)
}
