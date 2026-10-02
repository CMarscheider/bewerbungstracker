import { PercentPipe } from '@angular/common';
import { Component, computed, inject } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { MatCardModule } from '@angular/material/card';
import { RouterLink } from '@angular/router';
import { catchError, map, of } from 'rxjs';
import { Appointment, Deadline } from '../../api/models';
import { Api } from '../../core/api';
import { formatDate } from '../../core/dates';
import { DEADLINE_LABELS, eventLabel } from '../../shared/labels';

@Component({
  selector: 'app-dashboard',
  imports: [PercentPipe, RouterLink, MatCardModule],
  templateUrl: './dashboard.html',
  styleUrl: './dashboard.scss',
})
export class Dashboard {
  private readonly api = inject(Api);

  protected readonly deadlines = toSignal(this.api.listDeadlines(7).pipe(catchError(() => of([] as Deadline[]))), { initialValue: [] as Deadline[] });
  protected readonly appointments = toSignal(this.api.listAppointments(14).pipe(catchError(() => of([] as Appointment[]))), { initialValue: [] as Appointment[] });
  protected readonly summary = toSignal(this.api.getSummary().pipe(catchError(() => of(undefined))));
  protected readonly activeCount = toSignal(this.api.listApplications({ phase: 'Aktiv' }).pipe(
      map((l) => l.length),
      catchError(() => of(undefined)),
    ));
  protected readonly openOffers = toSignal(this.api.listApplications({ status: 'AngebotErhalten' }).pipe(
      map((l) => l.length),
      catchError(() => of(undefined)),
    ));

  protected readonly overdue = computed(() => this.deadlines().filter((d) => d.overdue));
  protected readonly upcoming = computed(() => this.deadlines().filter((d) => !d.overdue));

  protected readonly formatDate = formatDate;
  protected readonly eventLabel = eventLabel;
  protected readonly deadlineLabels = DEADLINE_LABELS;
}
