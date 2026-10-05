package service_test

import (
	"errors"
	"testing"
	"time"

	"bewerbungsmanager/internal/domain"
	"bewerbungsmanager/internal/service"
	"bewerbungsmanager/internal/store"
)

// Zwei gleichzeitige Läufe: Die Dublettensuche sieht die fremde, noch nicht festgeschriebene Stelle
// nicht, das Einfügen scheitert dann am eindeutigen Index. Auch dann soll DuplicateError mit der ID der
// anderen Bewerbung kommen. Ohne Zufall: Die fremde Transaktion wird erst festgeschrieben, wenn
// CreateAgentJob nachweislich auf ihre Sperre wartet.
func TestCreateAgentJobRaceOnSameURLReportsDuplicate(t *testing.T) {
	svc := newService(t)
	job := sampleJob()

	tx, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := store.New(tx)
	c, err := q.CreateCompany(ctx, store.CreateCompanyParams{Name: "Andere Firma"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := q.CreateApplication(ctx, store.CreateApplicationParams{
		CompanyID: c.ID, PositionTitle: "Anderer Titel", JobUrl: job.JobURL, CurrentStatus: string(domain.Vorgemerkt),
	})
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := svc.CreateAgentJob(ctx, job)
		done <- err
	}()

	// Warten, bis eine andere Sitzung auf eine Sperre wartet (das Einfügen hängt am Index).
	deadline := time.Now().Add(10 * time.Second)
	for {
		var waiting int
		if err := testPool.QueryRow(ctx,
			`SELECT count(*) FROM pg_stat_activity WHERE wait_event_type = 'Lock' AND datname = current_database()`,
		).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting > 0 {
			break
		}
		select {
		case err := <-done:
			t.Fatalf("CreateAgentJob kam ohne Warten zurück: %v", err)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("CreateAgentJob wartet nicht auf die Sperre")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	err = <-done
	var dup *service.DuplicateError
	if !errors.As(err, &dup) || dup.ExistingID != other.ID {
		t.Fatalf("erwartet DuplicateError mit %s, bekommen %v", other.ID, err)
	}
}
