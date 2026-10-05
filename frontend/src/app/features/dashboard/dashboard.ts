import { PercentPipe } from '@angular/common';
import { Component, computed, inject } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { MatCardModule } from '@angular/material/card';
import { RouterLink } from '@angular/router';
import { map } from 'rxjs';
import { Api } from '../../core/api';
import { formatDayMonth, weekdayShort } from '../../core/dates';
import { loaded } from '../../core/loaded';
import { DEADLINE_LABELS, STATUS_TONE, eventLabel } from '../../shared/labels';
import { AgentSuggestions } from './agent-suggestions';

@Component({
  selector: 'app-dashboard',
  imports: [PercentPipe, RouterLink, MatCardModule, AgentSuggestions],
  templateUrl: './dashboard.html',
  styleUrl: './dashboard.scss',
})
export class Dashboard {
  private readonly api = inject(Api);

  /** Alle Quellen sind undefined, solange sie laden; bei Fehler `error: true` (Snackbar kommt vom Interceptor). */
  protected readonly deadlinesResult = toSignal(loaded(this.api.listDeadlines(7)));
  protected readonly appointmentsResult = toSignal(loaded(this.api.listAppointments(14)));
  protected readonly summary = toSignal(loaded(this.api.getSummary()).pipe(map((r) => r.data)));
  protected readonly activeCount = toSignal(loaded(this.api.listApplications({ phase: 'Aktiv' })).pipe(map((r) => r.data?.length)));
  protected readonly openOffers = toSignal(loaded(this.api.listApplications({ status: 'AngebotErhalten' })).pipe(map((r) => r.data?.length)));

  /** Überfällige zuerst, sonst in der Reihenfolge der API (nach Datum). */
  protected readonly deadlines = computed(() => {
    const all = this.deadlinesResult()?.data ?? [];
    return [...all.filter((d) => d.overdue), ...all.filter((d) => !d.overdue)];
  });
  protected readonly appointments = computed(() => this.appointmentsResult()?.data ?? []);

  protected readonly formatDayMonth = formatDayMonth;
  protected readonly weekdayShort = weekdayShort;
  protected readonly tones = STATUS_TONE;
  protected readonly eventLabel = eventLabel;
  protected readonly deadlineLabels = DEADLINE_LABELS;
}
