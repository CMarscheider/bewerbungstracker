package service

import (
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"bewerbungsmanager/internal/domain"
)

// NotFoundError: angefragte Ressource existiert nicht (HTTP 404).
type NotFoundError struct{ Resource string }

func (e *NotFoundError) Error() string { return e.Resource + " nicht gefunden" }

// ConflictError: Aktion widerspricht dem aktuellen Datenbestand (HTTP 409).
type ConflictError struct{ Detail string }

func (e *ConflictError) Error() string { return e.Detail }

// UnavailableError: ein benötigter Dienst fehlt oder ist nicht erreichbar (HTTP 503).
type UnavailableError struct{ Detail string }

func (e *UnavailableError) Error() string { return e.Detail }

func notFoundIfNoRows(err error, resource string) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return &NotFoundError{Resource: resource}
	}
	return err
}

func hasPgCode(err error, code string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == code
}

func isUniqueViolation(err error) bool     { return hasPgCode(err, "23505") }
func isForeignKeyViolation(err error) bool { return hasPgCode(err, "23503") }

// cleanOptional trimmt und macht aus leeren Strings nil.
func cleanOptional(s *string) *string {
	if s == nil {
		return nil
	}
	t := strings.TrimSpace(*s)
	if t == "" {
		return nil
	}
	return &t
}

// applyOptional: nil im Patch = unverändert, "" = löschen.
func applyOptional(current, patch *string) *string {
	if patch == nil {
		return current
	}
	return cleanOptional(patch)
}

func requireText(field, value string) (string, error) {
	t := strings.TrimSpace(value)
	if t == "" {
		return "", &domain.ValidationError{Field: field, Detail: "darf nicht leer sein"}
	}
	return t, nil
}
