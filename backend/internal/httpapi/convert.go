package httpapi

import (
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"bewerbungsmanager/internal/domain"
	"bewerbungsmanager/internal/service"
)

func toDate(t time.Time) openapi_types.Date { return openapi_types.Date{Time: t} }

func toDatePtr(t *time.Time) *openapi_types.Date {
	if t == nil {
		return nil
	}
	d := toDate(*t)
	return &d
}

func fromDatePtr(d *openapi_types.Date) *time.Time {
	if d == nil {
		return nil
	}
	t := d.Time
	return &t
}

func companyDTO(c service.Company) Company {
	return Company{
		Id: c.ID, Name: c.Name, Website: c.Website, Notes: c.Notes,
		CreatedAt: c.CreatedAt, ApplicationCount: c.ApplicationCount,
	}
}

func eventDTO(e service.Event) Event {
	return Event{
		Id: e.ID, Type: EventType(e.Type), OccurredOn: toDate(e.OccurredOn), DueOn: toDatePtr(e.DueOn),
		InterviewRound: e.InterviewRound, Note: e.Note, CreatedAt: e.CreatedAt,
	}
}

func applicationDTO(a service.Application) Application {
	events := make([]Event, 0, len(a.Events))
	for _, e := range a.Events {
		events = append(events, eventDTO(e))
	}
	return Application{
		Id: a.ID, CompanyId: a.CompanyID, CompanyName: a.CompanyName, PositionTitle: a.PositionTitle,
		JobUrl: a.JobURL, Location: a.Location, Source: a.Source, Notes: a.Notes,
		ContactEmail: a.ContactEmail, PostingText: a.PostingText, FitScore: a.FitScore, FitReason: a.FitReason,
		Status: EventType(a.Status), Phase: Phase(a.Phase), CreatedByAgent: a.CreatedByAgent,
		DocumentsState: DocumentsState(a.DocumentsState), DocumentsError: a.DocumentsError, GmailDraftAt: a.GmailDraftAt,
		GmailThreadId: a.GmailThreadID, CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt, Events: events,
	}
}

func summaryDTO(a service.ApplicationSummary) ApplicationSummary {
	return ApplicationSummary{
		Id: a.ID, CompanyId: a.CompanyID, CompanyName: a.CompanyName, PositionTitle: a.PositionTitle,
		Status: EventType(a.Status), Phase: Phase(a.Phase), UpdatedAt: a.UpdatedAt,
		LastEventOn: toDate(a.LastEventOn), OpenDueOn: toDatePtr(a.OpenDueOn),
		FitScore: a.FitScore, CreatedByAgent: a.CreatedByAgent,
	}
}

func newEventFromDTO(n NewEvent) domain.NewEvent {
	return domain.NewEvent{
		Type: domain.EventType(n.Type), OccurredOn: n.OccurredOn.Time,
		DueOn: fromDatePtr(n.DueOn), Note: n.Note,
	}
}

func documentsDTO(d service.Documents) Documents {
	highlights := d.Highlights
	if highlights == nil {
		highlights = []string{}
	}
	return Documents{
		Version: d.Version, Language: DocumentsLanguage(d.Language), CoverLetter: d.CoverLetter,
		ProfileLine: d.ProfileLine, Highlights: highlights, MailSubject: d.MailSubject, MailBody: d.MailBody,
		FileName: d.FileName, RenderedAt: d.RenderedAt, UpdatedAt: d.UpdatedAt,
	}
}

func agentApplicationDTO(a service.AgentApplication) AgentApplication {
	return AgentApplication{
		Id: a.ID, CompanyName: a.CompanyName, CompanyWebsite: a.CompanyWebsite, PositionTitle: a.PositionTitle,
		JobUrl: a.JobURL, Location: a.Location, ContactEmail: a.ContactEmail, PostingText: a.PostingText,
		FitReason: a.FitReason, DocumentsState: DocumentsState(a.DocumentsState), DocumentsVersion: a.DocumentsVersion,
	}
}

func eventTypesDTO(ts []domain.EventType) []EventType {
	out := make([]EventType, 0, len(ts))
	for _, t := range ts {
		out = append(out, EventType(t))
	}
	return out
}

func openApplicationDTO(a service.OpenApplication) OpenApplication {
	return OpenApplication{
		Id: a.ID, CompanyName: a.CompanyName, CompanyWebsite: a.CompanyWebsite, PositionTitle: a.PositionTitle,
		Status: EventType(a.Status), ContactEmail: a.ContactEmail, GmailThreadId: a.GmailThreadID,
		DocumentsState: DocumentsState(a.DocumentsState), GmailDraftAt: a.GmailDraftAt, MailSubject: a.MailSubject,
		AllowedEvents: eventTypesDTO(a.AllowedEvents),
	}
}

func suggestionDTO(s service.Suggestion) Suggestion {
	return Suggestion{
		Id: s.ID, ApplicationId: s.ApplicationID, CompanyName: s.CompanyName, PositionTitle: s.PositionTitle,
		SuggestedType: EventType(s.SuggestedType), OccurredOn: toDate(s.OccurredOn), DueOn: toDatePtr(s.DueOn),
		Reason: s.Reason, MailSubject: s.MailSubject, MailFrom: s.MailFrom, MailUrl: s.MailURL,
		State: SuggestionState(s.State), CreatedAt: s.CreatedAt, DecidedAt: s.DecidedAt,
	}
}

func processedMailDTO(m service.ProcessedMail) ProcessedMail {
	return ProcessedMail{
		GmailMessageId: m.GmailMessageID, ApplicationId: m.ApplicationID, Outcome: m.Outcome, ProcessedAt: m.ProcessedAt,
	}
}

func derefStrings(p *[]string) []string {
	if p == nil {
		return nil
	}
	return *p
}
