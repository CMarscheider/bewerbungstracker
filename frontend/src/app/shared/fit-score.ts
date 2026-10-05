import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';

export type FitLevel = 'hoch' | 'mittel' | 'niedrig';

export function fitLevel(score: number): FitLevel {
  return score >= 70 ? 'hoch' : score >= 40 ? 'mittel' : 'niedrig';
}

/** Passungs-Score des Agenten als farbiger Chip (0–100). Farben in styles.scss (--fit-*). */
@Component({
  selector: 'app-fit-score',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `<span role="img" [class]="'fit ' + level()" [attr.aria-label]="'Passung ' + score() + ' von 100'">{{ score() }}</span>`,
  styles: `
    .fit {
      border-radius: 999px;
      display: inline-block;
      font-variant-numeric: tabular-nums;
      font-weight: 600;
      min-width: 2.25rem;
      padding: 0.125rem 0.5rem;
      text-align: center;
    }
    .hoch { background: var(--fit-high-bg); color: var(--fit-high-fg); }
    .mittel { background: var(--fit-mid-bg); color: var(--fit-mid-fg); }
    .niedrig { background: var(--fit-low-bg); color: var(--fit-low-fg); }
  `,
})
export class FitScore {
  readonly score = input.required<number>();
  protected readonly level = computed(() => fitLevel(this.score()));
}
