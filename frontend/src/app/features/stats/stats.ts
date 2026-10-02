import { DecimalPipe, PercentPipe } from '@angular/common';
import { Component, inject } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { catchError, of } from 'rxjs';
import { FunnelStep } from '../../api/models';
import { Api } from '../../core/api';
import { EVENT_LABELS } from '../../shared/labels';

@Component({
  selector: 'app-stats',
  imports: [DecimalPipe, PercentPipe],
  templateUrl: './stats.html',
  styleUrl: './stats.scss',
})
export class Stats {
  private readonly api = inject(Api);

  protected readonly funnel = toSignal(this.api.getFunnel().pipe(catchError(() => of([] as FunnelStep[]))), { initialValue: [] as FunnelStep[] });
  protected readonly summary = toSignal(this.api.getSummary().pipe(catchError(() => of(undefined))));
  protected readonly labels = EVENT_LABELS;
}
