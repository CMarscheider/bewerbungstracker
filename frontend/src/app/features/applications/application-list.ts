import { Component, inject } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { FormControl, FormGroup, ReactiveFormsModule } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { MatTableModule } from '@angular/material/table';
import { Router, RouterLink } from '@angular/router';
import { debounceTime, distinctUntilChanged, map, startWith, switchMap } from 'rxjs';
import { ApplicationSummary, EventType, Phase } from '../../api/models';
import { Api, ApplicationFilter } from '../../core/api';
import { formatDate } from '../../core/dates';
import { loaded } from '../../core/loaded';
import { ALL_EVENT_TYPES, ALL_PHASES, EVENT_LABELS, PHASE_LABELS } from '../../shared/labels';
import { StatusBadge } from '../../shared/status-badge';

@Component({
  selector: 'app-application-list',
  imports: [ReactiveFormsModule, RouterLink, MatButtonModule, MatFormFieldModule, MatInputModule, MatSelectModule, MatTableModule, StatusBadge],
  templateUrl: './application-list.html',
  styleUrl: './application-list.scss',
})
export class ApplicationList {
  private readonly api = inject(Api);
  private readonly router = inject(Router);

  protected readonly phases = ALL_PHASES;
  protected readonly statuses = ALL_EVENT_TYPES;
  protected readonly phaseLabels = PHASE_LABELS;
  protected readonly eventLabels = EVENT_LABELS;
  protected readonly formatDate = formatDate;
  protected readonly columns = ['company', 'position', 'status', 'last', 'due'];

  protected readonly filter = new FormGroup({
    phase: new FormControl<Phase | null>(null),
    status: new FormControl<EventType | null>(null),
    q: new FormControl('', { nonNullable: true }),
  });

  /** undefined, bis die erste Antwort da ist; sonst Daten oder `error: true`. */
  protected readonly applications = toSignal(
    this.filter.valueChanges.pipe(
      debounceTime(250),
      startWith(null),
      map(() => this.currentFilter()),
      distinctUntilChanged((a, b) => JSON.stringify(a) === JSON.stringify(b)),
      switchMap((f) => loaded(this.api.listApplications(f))),
    ),
  );

  protected open(a: ApplicationSummary): void {
    void this.router.navigate(['/bewerbungen', a.id]);
  }

  private currentFilter(): ApplicationFilter {
    const v = this.filter.getRawValue();
    return { phase: v.phase ?? undefined, status: v.status ?? undefined, q: v.q.trim() || undefined };
  }
}
