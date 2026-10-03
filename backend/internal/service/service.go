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

// PDFConverter wandelt HTML (mit Zusatzdateien wie Schriften) in ein PDF um.
type PDFConverter interface {
	Convert(ctx context.Context, html []byte, assets map[string][]byte) ([]byte, error)
}

// Service bündelt alle Anwendungsfälle.
type Service struct {
	pool *pgxpool.Pool
	now  func() time.Time
	pdf  PDFConverter
}

// Option konfiguriert optionale Abhängigkeiten.
type Option func(*Service)

// WithPDFConverter aktiviert die PDF-Erzeugung.
func WithPDFConverter(c PDFConverter) Option { return func(s *Service) { s.pdf = c } }

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
