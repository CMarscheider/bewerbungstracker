import { NgTemplateOutlet } from '@angular/common';
import { Component, computed, inject } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatMenuModule } from '@angular/material/menu';
import { ThemeMode, ThemeService } from '../core/theme';

const OPTIONS: { mode: ThemeMode; label: string }[] = [
  { mode: 'system', label: 'System' },
  { mode: 'light', label: 'Hell' },
  { mode: 'dark', label: 'Dunkel' },
];

/** Icon-Button mit Menü für Hell/Dunkel/System. Icons als Inline-SVG, damit keine Icon-Schrift nötig ist. */
@Component({
  selector: 'app-theme-toggle',
  imports: [NgTemplateOutlet, MatButtonModule, MatMenuModule],
  template: `
    <button mat-icon-button [matMenuTriggerFor]="menu" [attr.aria-label]="'Darstellung: ' + currentLabel()" [title]="'Darstellung: ' + currentLabel()">
      <ng-container *ngTemplateOutlet="icon; context: { $implicit: theme.mode() }" />
    </button>
    <mat-menu #menu="matMenu">
      @for (o of options; track o.mode) {
        <button mat-menu-item (click)="theme.set(o.mode)" [attr.aria-checked]="theme.mode() === o.mode" role="menuitemradio">
          <span class="item">
            <ng-container *ngTemplateOutlet="icon; context: { $implicit: o.mode }" />
            {{ o.label }}
          </span>
        </button>
      }
    </mat-menu>

    <ng-template #icon let-mode>
      <svg viewBox="0 0 24 24" width="20" height="20" aria-hidden="true" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        @switch (mode) {
          @case ('light') {
            <circle cx="12" cy="12" r="4" />
            <path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4" />
          }
          @case ('dark') {
            <path d="M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8z" />
          }
          @default {
            <rect x="2" y="4" width="20" height="13" rx="2" />
            <path d="M8 21h8M12 17v4" />
          }
        }
      </svg>
    </ng-template>
  `,
  styles: `
    .item { align-items: center; display: inline-flex; gap: 12px; }
  `,
})
export class ThemeToggle {
  protected readonly theme = inject(ThemeService);
  protected readonly options = OPTIONS;
  protected readonly currentLabel = computed(() => OPTIONS.find((o) => o.mode === this.theme.mode())!.label);
}
