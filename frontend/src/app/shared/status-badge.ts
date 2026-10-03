import { Component, computed, input } from '@angular/core';
import { EventType } from '../api/models';
import { EVENT_LABELS, STATUS_TONE } from './labels';

@Component({
  selector: 'app-status-badge',
  template: `<span class="badge" [class]="'tone-' + tone()">{{ label() }}</span>`,
  styles: `
    .badge {
      background: var(--tone-bg);
      border-radius: 999px;
      color: var(--tone-fg);
      display: inline-block;
      font-size: 0.8rem;
      font-weight: 500;
      padding: 2px 10px;
      white-space: nowrap;
    }
  `,
})
export class StatusBadge {
  readonly status = input.required<EventType>();
  protected readonly label = computed(() => EVENT_LABELS[this.status()]);
  protected readonly tone = computed(() => STATUS_TONE[this.status()]);
}
