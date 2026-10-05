// Package service enthält die Anwendungsfälle und kapselt Transaktionen.
package service

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"bewerbungsmanager/internal/domain"
	"bewerbungsmanager/internal/store"
)

// PDFConverter wandelt HTML (mit Zusatzdateien wie Schriften) in ein PDF um und fügt PDFs
// in der übergebenen Reihenfolge zusammen.
type PDFConverter interface {
	Convert(ctx context.Context, html []byte, assets map[string][]byte) ([]byte, error)
	Merge(ctx context.Context, pdfs ...[]byte) ([]byte, error)
}

// Drafter legt eine fertige Nachricht als Entwurf ab (Produktion: mail.IMAPDrafter).
type Drafter interface {
	Save(ctx context.Context, msg []byte) error
}

// Service bündelt alle Anwendungsfälle.
type Service struct {
	pool      *pgxpool.Pool
	now       func() time.Time
	pdf       PDFConverter
	drafter   Drafter
	draftFrom string // Absenderadresse der Entwürfe; den Namen liefert der Lebenslauf
}

// Option konfiguriert optionale Abhängigkeiten.
type Option func(*Service)

// WithPDFConverter aktiviert die PDF-Erzeugung.
func WithPDFConverter(c PDFConverter) Option { return func(s *Service) { s.pdf = c } }

// WithDrafter aktiviert das Anlegen von Mail-Entwürfen; from ist die reine Absenderadresse,
// der Anzeigename kommt aus dem gespeicherten Lebenslauf.
func WithDrafter(d Drafter, from string) Option {
	return func(s *Service) { s.drafter, s.draftFrom = d, from }
}

// New erzeugt einen Service; now ist in Produktion time.Now.
func New(pool *pgxpool.Pool, now func() time.Time, opts ...Option) *Service {
	s := &Service{pool: pool, now: now}
	for _, o := range opts {
		o(s)
	}
	return s
}

func (s *Service) today() time.Time { return domain.DateOf(s.now().In(time.Local)) }

func (s *Service) queries() *store.Queries { return store.New(s.pool) }

func (s *Service) inTx(ctx context.Context, fn func(q *store.Queries) error) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		return fn(store.New(tx))
	})
}

// inReadTx liest in einer schreibgeschützten Transaktion mit konsistentem Snapshot.
func (s *Service) inReadTx(ctx context.Context, fn func(q *store.Queries) error) error {
	return pgx.BeginTxFunc(ctx, s.pool, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}, func(tx pgx.Tx) error {
		return fn(store.New(tx))
	})
}
