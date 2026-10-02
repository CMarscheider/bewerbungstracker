import { Component, computed, effect, inject, input, signal, untracked } from '@angular/core';
import { FormControl, FormGroup, ReactiveFormsModule, Validators } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog } from '@angular/material/dialog';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { Router } from '@angular/router';
import { forkJoin } from 'rxjs';
import { Application, EventType, NewEvent } from '../../api/models';
import { Api } from '../../core/api';
import { formatDate } from '../../core/dates';
import { DEADLINE_LABELS, EVENT_LABELS, eventLabel } from '../../shared/labels';
import { StatusBadge } from '../../shared/status-badge';
import { EventDialog, EventDialogData } from './event-dialog';

@Component({
  selector: 'app-application-detail',
  imports: [ReactiveFormsModule, MatButtonModule, MatFormFieldModule, MatInputModule, StatusBadge],
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
  protected readonly eventsNewestFirst = computed(() => [...(this.application()?.events ?? [])].reverse());
  protected readonly canUndo = computed(() => (this.application()?.events.length ?? 0) > 1);

  protected readonly labels = EVENT_LABELS;
  protected readonly deadlineLabels = DEADLINE_LABELS;
  protected readonly eventLabel = eventLabel;
  protected readonly formatDate = formatDate;

  protected readonly form = new FormGroup({
    position_title: new FormControl('', { nonNullable: true, validators: [Validators.required] }),
    job_url: new FormControl('', { nonNullable: true }),
    location: new FormControl('', { nonNullable: true }),
    source: new FormControl('', { nonNullable: true }),
    notes: new FormControl('', { nonNullable: true }),
  });

  constructor() {
    // Lädt neu, sobald sich die Route (:id) ändert.
    effect(() => {
      const id = this.id();
      untracked(() => this.load(id));
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
      location: a.location ?? '',
      source: a.source ?? '',
      notes: a.notes ?? '',
    });
    this.editing.set(true);
  }

  protected saveDetails(): void {
    if (this.form.invalid) {
      return;
    }
    // Leere Strings löschen optionale Felder (PATCH-Semantik des Backends).
    this.api.updateApplication(this.id(), this.form.getRawValue()).subscribe((app) => {
      this.application.set(app);
      this.editing.set(false);
    });
  }

  protected openEventDialog(type: EventType): void {
    this.dialog
      .open<EventDialog, EventDialogData, NewEvent>(EventDialog, { data: { type } })
      .afterClosed()
      .subscribe((event) => {
        if (event) {
          this.api.addEvent(this.id(), event).subscribe(() => this.load(this.id()));
        }
      });
  }

  protected undo(): void {
    if (window.confirm('Letztes Ereignis wirklich rückgängig machen?')) {
      this.api.undoLastEvent(this.id()).subscribe(() => this.load(this.id()));
    }
  }

  protected remove(): void {
    if (window.confirm('Bewerbung wirklich löschen?')) {
      this.api.deleteApplication(this.id()).subscribe(() => void this.router.navigate(['/bewerbungen']));
    }
  }

  private load(id: string): void {
    forkJoin({ app: this.api.getApplication(id), allowed: this.api.listAllowedEvents(id) }).subscribe(({ app, allowed }) => {
      this.application.set(app);
      this.allowed.set(allowed);
    });
  }
}
