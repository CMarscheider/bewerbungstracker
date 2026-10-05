import { Component, computed, effect, inject, input, signal, untracked } from '@angular/core';
import { FormControl, FormGroup, ReactiveFormsModule, Validators } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog } from '@angular/material/dialog';
import { MatExpansionModule } from '@angular/material/expansion';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { Router, RouterLink } from '@angular/router';
import { finalize, forkJoin } from 'rxjs';
import { Application, EventType, NewEvent } from '../../api/models';
import { Api } from '../../core/api';
import { formatDate } from '../../core/dates';
import { DEADLINE_LABELS, EVENT_LABELS, STATUS_TONE, eventLabel } from '../../shared/labels';
import { Icon } from '../../shared/icon';
import { FitScore } from '../../shared/fit-score';
import { StatusBadge } from '../../shared/status-badge';
import { ApplicationDocuments } from './application-documents';
import { CONTACT_EMAIL_ERROR, contactEmailControl } from './contact-email';
import { EventDialog, EventDialogData } from './event-dialog';
import { OUTLINED_FIELDS } from '../../shared/form-field-defaults';

// `/u/0/` ist das erste im Browser angemeldete Gmail-Konto; bei mehreren Konten ggf. das falsche.
const GMAIL_THREAD_URL = 'https://mail.google.com/mail/u/0/#all/';

@Component({
  selector: 'app-application-detail',
  providers: [OUTLINED_FIELDS],
  imports: [ReactiveFormsModule, RouterLink, MatButtonModule, MatExpansionModule, MatFormFieldModule, MatInputModule, FitScore, StatusBadge, ApplicationDocuments, Icon],
  templateUrl: './application-detail.html',
  styleUrl: './application-detail.scss',
})
export class ApplicationDetail {
  /** Aus der Route (:id) über withComponentInputBinding. */
  readonly id = input.required<string>();

  private readonly api = inject(Api);
  private readonly dialog = inject(MatDialog);
  private readonly router = inject(Router);

  protected readonly application = signal<Application | null>(null);
  protected readonly allowed = signal<EventType[]>([]);
  protected readonly editing = signal(false);
  protected readonly loadError = signal(false);
  protected readonly savingDetails = signal(false);
  protected readonly eventsNewestFirst = computed(() => [...(this.application()?.events ?? [])].reverse());
  protected readonly canUndo = computed(() => (this.application()?.events.length ?? 0) > 1);

  protected readonly labels = EVENT_LABELS;
  protected readonly tones = STATUS_TONE;
  protected readonly deadlineLabels = DEADLINE_LABELS;
  protected readonly eventLabel = eventLabel;
  protected readonly formatDate = formatDate;
  protected readonly contactEmailError = CONTACT_EMAIL_ERROR;
  /** Adresse kodiert, damit sie keine mailto-Parameter (?cc=, &body=) einschleusen kann. */
  protected readonly mailto = (address: string) => 'mailto:' + encodeURIComponent(address);

  protected readonly form = new FormGroup({
    position_title: new FormControl('', { nonNullable: true, validators: [Validators.required, Validators.pattern(/\S/)] }),
    job_url: new FormControl('', { nonNullable: true }),
    contact_email: contactEmailControl(),
    location: new FormControl('', { nonNullable: true }),
    source: new FormControl('', { nonNullable: true }),
    notes: new FormControl('', { nonNullable: true }),
  });

  constructor() {
    // Lädt neu, sobald sich die Route (:id) ändert.
    effect(() => {
      const id = this.id();
      untracked(() => {
        this.editing.set(false);
        this.load(id);
      });
    });
  }

  protected startEdit(): void {
    const a = this.application();
    if (!a) {
      return;
    }
    this.form.setValue({
      position_title: a.position_title,
      job_url: a.job_url ?? '',
      contact_email: a.contact_email ?? '',
      location: a.location ?? '',
      source: a.source ?? '',
      notes: a.notes ?? '',
    });
    this.editing.set(true);
  }

  protected saveDetails(): void {
    if (this.form.invalid || this.savingDetails()) {
      return;
    }
    // Leere Strings löschen optionale Felder (PATCH-Semantik des Backends).
    const v = this.form.getRawValue();
    const body = {
      position_title: v.position_title.trim(),
      job_url: v.job_url.trim(),
      contact_email: v.contact_email.trim(),
      location: v.location.trim(),
      source: v.source.trim(),
      notes: v.notes.trim(),
    };
    this.savingDetails.set(true);
    this.api
      .updateApplication(this.id(), body)
      .pipe(finalize(() => this.savingDetails.set(false)))
      .subscribe({
        next: (app) => {
          this.application.set(app);
          this.editing.set(false);
        },
        error: () => undefined, // Meldung zeigt der Interceptor
      });
  }

  protected openEventDialog(type: EventType): void {
    this.dialog
      .open<EventDialog, EventDialogData, NewEvent>(EventDialog, { data: { type } })
      .afterClosed()
      .subscribe((event) => {
        if (event) {
          this.api.addEvent(this.id(), event).subscribe({ next: () => this.load(this.id()), error: () => undefined });
        }
      });
  }

  /** Neu geladene Bewerbung aus dem Bereich Unterlagen; veraltete Antworten verwerfen. */
  protected documentsChanged(app: Application): void {
    if (app.id === this.id()) {
      this.application.set(app);
    }
  }

  /** Gmail-Link zum verknüpften Thread; die ID ist kodiert, obwohl der Server sie schon prüft. */
  protected readonly threadUrl = (threadId: string) => GMAIL_THREAD_URL + encodeURIComponent(threadId);

  protected unlinkThread(): void {
    if (window.confirm('Falls die Mail falsch zugeordnet wurde: Verknüpfung lösen?')) {
      const id = this.id();
      this.api.clearGmailThread(id).subscribe({ next: () => this.load(id), error: () => undefined });
    }
  }

  protected undo(): void {
    if (window.confirm('Letztes Ereignis wirklich rückgängig machen?')) {
      this.api.undoLastEvent(this.id()).subscribe({ next: () => this.load(this.id()), error: () => undefined });
    }
  }

  protected remove(): void {
    if (window.confirm('Bewerbung wirklich löschen?')) {
      this.api.deleteApplication(this.id()).subscribe({ next: () => void this.router.navigate(['/bewerbungen']), error: () => undefined });
    }
  }

  private load(id: string): void {
    this.loadError.set(false);
    forkJoin({ app: this.api.getApplication(id), allowed: this.api.listAllowedEvents(id) }).subscribe({
      next: ({ app, allowed }) => {
        // Veraltete Antworten (Route wurde inzwischen gewechselt) verwerfen.
        if (id === this.id()) {
          this.application.set(app);
          this.allowed.set(allowed);
        }
      },
      error: () => {
        if (id === this.id()) {
          this.application.set(null);
          this.loadError.set(true);
        }
      },
    });
  }
}
