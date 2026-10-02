import { Component, computed, input } from '@angular/core';
import { EventType, Phase } from '../api/models';
import { EVENT_LABELS } from './labels';

@Component({
  selector: 'app-status-badge',
  template: `<span class="badge" [class]="'phase-' + phase() + ' status-' + status()">{{ label() }}</span>`,
  styles: `
    .badge { border-radius: 999px; display: inline-block; font-size: 0.8rem; padding: 2px 10px; white-space: nowrap; }
    .phase-Vorbereitung { background: #eef0f3; color: #374151; }
    .phase-Aktiv { background: #e0ecff; color: #1d4ed8; }
    .phase-Abgeschlossen { background: #f1f1f1; color: #4b5563; }
    .status-AngebotErhalten, .status-AngebotAngenommen { background: #dcfce7; color: #166534; }
    .status-Absage { background: #fee2e2; color: #991b1b; }
  `,
})
export class StatusBadge {
  readonly status = input.required<EventType>();
  readonly phase = input<Phase>('Aktiv');
  protected readonly label = computed(() => EVENT_LABELS[this.status()]);
}
