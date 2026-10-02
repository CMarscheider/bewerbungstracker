package domain

import "fmt"

// Codes für RuleError; sie erscheinen im Problem-JSON als Typ-URI.
const (
	CodeDateInFuture       = "date-in-future"
	CodeDateBeforePrevious = "date-before-previous"
	CodeCannotUndoFirst    = "cannot-undo-first-event"
)

// ValidationError: Eingabe formal ungültig (HTTP 400).
type ValidationError struct {
	Field  string
	Detail string
}

func (e *ValidationError) Error() string { return e.Field + ": " + e.Detail }

// TransitionError: Ereignis ist im aktuellen Zustand nicht erlaubt (HTTP 422).
type TransitionError struct {
	From      EventType // leer, wenn es noch kein Ereignis gibt
	Attempted EventType
}

func (e *TransitionError) Error() string {
	from := string(e.From)
	if from == "" {
		from = "(kein Ereignis)"
	}
	return fmt.Sprintf("Übergang von %s nach %s ist nicht erlaubt", from, e.Attempted)
}

// RuleError: sonstige fachliche Regel verletzt (HTTP 422).
type RuleError struct {
	Code   string
	Detail string
}

func (e *RuleError) Error() string { return e.Detail }
