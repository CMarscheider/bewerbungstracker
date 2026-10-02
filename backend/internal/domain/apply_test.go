package domain

import (
	"errors"
	"testing"
	"time"
)

func d(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func ev(t EventType, occurred, recorded string) Event {
	return Event{Type: t, OccurredOn: d(occurred), RecordedOn: d(recorded)}
}

func ptrTime(t time.Time) *time.Time { return &t }

var today = d("2026-10-02")

func TestCanApplyAcceptsValidFirstEvent(t *testing.T) {
	err := CanApply(nil, NewEvent{Type: Beworben, OccurredOn: d("2026-09-30")}, today)
	if err != nil {
		t.Fatalf("unerwarteter Fehler: %v", err)
	}
}

func TestCanApplyRejectsInvalidTransition(t *testing.T) {
	history := []Event{ev(Beworben, "2026-09-01", "2026-09-01"), ev(Absage, "2026-09-10", "2026-09-10")}
	err := CanApply(history, NewEvent{Type: Interview, OccurredOn: d("2026-09-20")}, today)
	var te *TransitionError
	if !errors.As(err, &te) {
		t.Fatalf("erwartet TransitionError, bekommen %v", err)
	}
	if te.From != Absage || te.Attempted != Interview {
		t.Errorf("TransitionError = %+v", te)
	}
}

func TestCanApplyRejectsInvalidFirstEvent(t *testing.T) {
	err := CanApply(nil, NewEvent{Type: Interview, OccurredOn: d("2026-09-20")}, today)
	var te *TransitionError
	if !errors.As(err, &te) || te.From != "" {
		t.Fatalf("erwartet TransitionError ohne From, bekommen %v", err)
	}
}

func TestCanApplyValidation(t *testing.T) {
	tests := []struct {
		name  string
		next  NewEvent
		field string
	}{
		{"Frist bei Beworben", NewEvent{Type: Beworben, OccurredOn: d("2026-09-30"), DueOn: ptrTime(d("2026-10-10"))}, "due_on"},
		{"unbekannter Typ", NewEvent{Type: "Quatsch", OccurredOn: d("2026-09-30")}, "type"},
		{"Datum fehlt", NewEvent{Type: Beworben}, "occurred_on"},
		{"Frist bei unerlaubtem Erstereignis", NewEvent{Type: Interview, OccurredOn: d("2026-09-30"), DueOn: ptrTime(d("2026-10-10"))}, "due_on"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CanApply(nil, tt.next, today)
			var ve *ValidationError
			if !errors.As(err, &ve) || ve.Field != tt.field {
				t.Fatalf("erwartet ValidationError für %s, bekommen %v", tt.field, err)
			}
		})
	}
}

func TestCanApplyAllowsDeadlineOnVorgemerkt(t *testing.T) {
	next := NewEvent{Type: Vorgemerkt, OccurredOn: d("2026-10-01"), DueOn: ptrTime(d("2026-10-15"))}
	if err := CanApply(nil, next, today); err != nil {
		t.Fatalf("unerwarteter Fehler: %v", err)
	}
}

func TestCanApplyDateRules(t *testing.T) {
	applied := []Event{ev(Beworben, "2026-09-01", "2026-09-01")}
	scheduled := []Event{
		ev(Beworben, "2026-09-01", "2026-09-01"),
		ev(Interview, "2026-10-10", "2026-09-25"), // im Voraus eingetragener Termin
	}
	tests := []struct {
		name     string
		history  []Event
		next     NewEvent
		wantCode string // leer = kein Fehler
	}{
		{"Absage in der Zukunft", applied, NewEvent{Type: Absage, OccurredOn: d("2026-10-03")}, CodeDateInFuture},
		{"Interview in der Zukunft", applied, NewEvent{Type: Interview, OccurredOn: d("2026-10-10")}, ""},
		{"vor vorherigem Ereignis", applied, NewEvent{Type: Absage, OccurredOn: d("2026-08-31")}, CodeDateBeforePrevious},
		{"gleicher Tag wie vorheriges", applied, NewEvent{Type: Absage, OccurredOn: d("2026-09-01")}, ""},
		{"Absage vor geplantem Interview", scheduled, NewEvent{Type: Absage, OccurredOn: d("2026-10-01")}, ""},
		{"vor Erfassung des Termins", scheduled, NewEvent{Type: Absage, OccurredOn: d("2026-09-20")}, CodeDateBeforePrevious},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CanApply(tt.history, tt.next, today)
			if tt.wantCode == "" {
				if err != nil {
					t.Fatalf("unerwarteter Fehler: %v", err)
				}
				return
			}
			var re *RuleError
			if !errors.As(err, &re) || re.Code != tt.wantCode {
				t.Fatalf("erwartet RuleError %s, bekommen %v", tt.wantCode, err)
			}
		})
	}
}

func TestInterviewRound(t *testing.T) {
	if got := InterviewRound(nil); got != 1 {
		t.Errorf("InterviewRound(nil) = %d, erwartet 1", got)
	}
	history := []Event{
		ev(Beworben, "2026-09-01", "2026-09-01"),
		ev(Interview, "2026-09-05", "2026-09-05"),
		ev(Interview, "2026-09-12", "2026-09-12"),
	}
	if got := InterviewRound(history); got != 3 {
		t.Errorf("InterviewRound = %d, erwartet 3", got)
	}
}

func TestCanUndo(t *testing.T) {
	one := []Event{ev(Beworben, "2026-09-01", "2026-09-01")}
	var re *RuleError
	if err := CanUndo(one); !errors.As(err, &re) || re.Code != CodeCannotUndoFirst {
		t.Fatalf("erwartet RuleError %s, bekommen %v", CodeCannotUndoFirst, err)
	}
	two := append(one, ev(Absage, "2026-09-02", "2026-09-02"))
	if err := CanUndo(two); err != nil {
		t.Fatalf("unerwarteter Fehler: %v", err)
	}
}

func TestDateOfDropsTime(t *testing.T) {
	in := time.Date(2026, 10, 2, 23, 30, 0, 0, time.FixedZone("CEST", 2*3600))
	if got := DateOf(in); !got.Equal(d("2026-10-02")) {
		t.Errorf("DateOf = %v", got)
	}
}

func TestCanApplyAfterKeineRueckmeldung(t *testing.T) {
	history := []Event{
		ev(Beworben, "2026-09-01", "2026-09-01"),
		ev(KeineRueckmeldung, "2026-09-20", "2026-09-20"),
	}
	if err := CanApply(history, NewEvent{Type: Interview, OccurredOn: d("2026-09-25")}, today); err != nil {
		t.Fatalf("unerwarteter Fehler: %v", err)
	}
	var re *RuleError
	err := CanApply(history, NewEvent{Type: Interview, OccurredOn: d("2026-09-19")}, today)
	if !errors.As(err, &re) || re.Code != CodeDateBeforePrevious {
		t.Fatalf("erwartet RuleError %s, bekommen %v", CodeDateBeforePrevious, err)
	}
	var te *TransitionError
	err = CanApply(history, NewEvent{Type: KeineRueckmeldung, OccurredOn: d("2026-09-25")}, today)
	if !errors.As(err, &te) {
		t.Fatalf("erwartet TransitionError, bekommen %v", err)
	}
}
