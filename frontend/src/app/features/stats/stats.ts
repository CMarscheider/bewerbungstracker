import { DecimalPipe, PercentPipe } from '@angular/common';
import { Component, inject } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { Api } from '../../core/api';
import { loaded } from '../../core/loaded';
import { EVENT_LABELS } from '../../shared/labels';

@Component({
  selector: 'app-stats',
  imports: [DecimalPipe, PercentPipe],
  templateUrl: './stats.html',
  styleUrl: './stats.scss',
})
export class Stats {
  private readonly api = inject(Api);

  protected readonly funnel = toSignal(loaded(this.api.getFunnel()));
  protected readonly summary = toSignal(loaded(this.api.getSummary()));
  protected readonly labels = EVENT_LABELS;
}
