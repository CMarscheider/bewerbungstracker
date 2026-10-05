package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
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
)

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
}

// Suggestion ist ein gespeicherter Vorschlag; Firma und Stelle fehlen bei nicht zugeordneten.
type Suggestion struct {
	ID            uuid.UUID
	ApplicationID *uuid.UUID
	CompanyName   *string
	PositionTitle *string
	SuggestedType domain.EventType
	OccurredOn    time.Time
	DueOn         *time.Time
	Reason        string
	MailSubject   *string
	MailFrom      *string
	MailURL       *string
	State         string
	CreatedAt     time.Time
	DecidedAt     *time.Time
}

// ProcessedMail ist eine bereits ausgewertete Mail.
type ProcessedMail struct {
	GmailMessageID string
	ApplicationID  *uuid.UUID
	Outcome        string
	ProcessedAt    time.Time
}

// ListOpenAgentApplications liefert alle nicht abgeschlossenen Bewerbungen, sortiert nach Firma.
func (s *Service) ListOpenAgentApplications(ctx context.Context) ([]OpenApplication, error) {
	var statuses []string
	for _, p := range []domain.Phase{domain.PhaseVorbereitung, domain.PhaseAktiv} {
		for _, t := range domain.StatusesInPhase(p) {
			statuses = append(statuses, string(t))
		}
	}
	rows, err := s.queries().ListOpenAgentApplications(ctx, statuses)
	if err != nil {
		return nil, err
	}
	out := make([]OpenApplication, 0, len(rows))
	for _, r := range rows {
		status := domain.EventType(r.CurrentStatus)
		out = append(out, OpenApplication{
			ID: r.ID, CompanyName: r.CompanyName, CompanyWebsite: r.CompanyWebsite, PositionTitle: r.PositionTitle,
			Status: status, ContactEmail: r.ContactEmail, GmailThreadID: r.GmailThreadID,
			DocumentsState: r.DocumentsState, GmailDraftAt: r.GmailDraftAt, MailSubject: r.MailSubject,
			UpdatedAt: r.UpdatedAt,
			// Außerhalb der Phase Abgeschlossen hängen die erlaubten Ereignisse nur vom letzten ab
			// (die Sonderregel für KeineRueckmeldung betrifft abgeschlossene Bewerbungen).
			AllowedEvents: domain.AllowedNext([]domain.EventType{status}),
		})
	}
	return out, nil
}

// AgentAddEvent legt ein Ereignis wie AddEvent an und kennzeichnet die Notiz mit "Agent: ".
func (s *Service) AgentAddEvent(ctx context.Context, appID uuid.UUID, next domain.NewEvent) (Event, error) {
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

// SetGmailThread merkt sich den Gmail-Thread der gesendeten Bewerbung; erneutes Setzen ist idempotent.
func (s *Service) SetGmailThread(ctx context.Context, appID uuid.UUID, threadID string) error {
	threadID, err := requireText("gmail_thread_id", threadID)
	if err != nil {
		return err
	}
	return s.inTx(ctx, func(q *store.Queries) error {
		a, err := q.LockApplication(ctx, appID)
		if err != nil {
			return notFoundIfNoRows(err, "Bewerbung")
		}
		if a.GmailThreadID != nil && *a.GmailThreadID == threadID {
			return nil
		}
		err = q.SetGmailThread(ctx, store.SetGmailThreadParams{ID: appID, GmailThreadID: &threadID})
		if isUniqueViolation(err) {
			return &ConflictError{Detail: "Dieser Gmail-Thread gehört schon zu einer anderen Bewerbung"}
		}
		return err
	})
}

// CreateSuggestion legt einen offenen Vorschlag an.
func (s *Service) CreateSuggestion(ctx context.Context, in NewSuggestion) (Suggestion, error) {
	if !in.SuggestedType.Valid() {
		return Suggestion{}, &domain.ValidationError{Field: "suggested_type", Detail: fmt.Sprintf("unbekannter Ereignistyp %q", in.SuggestedType)}
	}
	if in.OccurredOn.IsZero() {
		return Suggestion{}, &domain.ValidationError{Field: "occurred_on", Detail: "fehlt"}
	}
	if in.DueOn != nil && !in.SuggestedType.AllowsDeadline() {
		return Suggestion{}, &domain.ValidationError{Field: "due_on", Detail: fmt.Sprintf("%s hat keine Frist", in.SuggestedType)}
	}
	reason, err := requireText("reason", in.Reason)
	if err != nil {
		return Suggestion{}, err
	}
	if utf8.RuneCountInString(reason) > maxReasonLen {
		return Suggestion{}, &domain.ValidationError{Field: "reason", Detail: fmt.Sprintf("höchstens %d Zeichen", maxReasonLen)}
	}
	var due *time.Time
	if in.DueOn != nil {
		d := domain.DateOf(*in.DueOn)
		due = &d
	}
	var row store.GetSuggestionRow
	err = s.inTx(ctx, func(q *store.Queries) error {
		created, err := q.InsertSuggestion(ctx, store.InsertSuggestionParams{
			ApplicationID: in.ApplicationID,
			SuggestedType: string(in.SuggestedType),
			OccurredOn:    domain.DateOf(in.OccurredOn),
			DueOn:         due,
			Reason:        reason,
			MailSubject:   cleanOptional(in.MailSubject),
			MailFrom:      cleanOptional(in.MailFrom),
			MailUrl:       cleanOptional(in.MailURL),
		})
		if isForeignKeyViolation(err) {
			return &NotFoundError{Resource: "Bewerbung"}
		}
		if err != nil {
			return err
		}
		row, err = q.GetSuggestion(ctx, created.ID)
		return err
	})
	if err != nil {
		return Suggestion{}, err
	}
	return toSuggestion(row), nil
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
		note := agentNotePrefix + sg.Reason
		if _, err := s.addEvent(ctx, q, target, domain.NewEvent{
			Type: domain.EventType(sg.SuggestedType), OccurredOn: sg.OccurredOn, DueOn: sg.DueOn, Note: &note,
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
	if utf8.RuneCountInString(outcome) > maxOutcomeLen {
		return ProcessedMail{}, false, &domain.ValidationError{Field: "outcome", Detail: fmt.Sprintf("höchstens %d Zeichen", maxOutcomeLen)}
	}
	var (
		m       store.ProcessedMail
		created bool
	)
	err = s.inTx(ctx, func(q *store.Queries) error {
		var err error
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
		State: r.State, CreatedAt: r.CreatedAt, DecidedAt: r.DecidedAt,
	}
}

func toProcessedMail(m store.ProcessedMail) ProcessedMail {
	return ProcessedMail{
		GmailMessageID: m.GmailMessageID, ApplicationID: m.ApplicationID, Outcome: m.Outcome, ProcessedAt: m.ProcessedAt,
	}
}
