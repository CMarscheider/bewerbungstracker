import { Component, DestroyRef, ElementRef, Injector, afterNextRender, computed, inject, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { MatButtonModule } from '@angular/material/button';
import { MatCardModule } from '@angular/material/card';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatSnackBar } from '@angular/material/snack-bar';
import { RouterLink } from '@angular/router';
import { Observable, finalize } from 'rxjs';
import { ApplicationSummary, EventType, Suggestion } from '../../api/models';
import { Api } from '../../core/api';
import { formatDate } from '../../core/dates';
import { DEADLINE_LABELS } from '../../shared/labels';
import { StatusBadge } from '../../shared/status-badge';

@Component({
  selector: 'app-agent-suggestions',
  imports: [RouterLink, MatButtonModule, MatCardModule, MatFormFieldModule, MatInputModule, StatusBadge],
  templateUrl: './agent-suggestions.html',
  styleUrl: './agent-suggestions.scss',
})
export class AgentSuggestions {
  private readonly api = inject(Api);
  private readonly snackBar = inject(MatSnackBar);
  private readonly destroyRef = inject(DestroyRef);
  private readonly injector = inject(Injector);
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);

  /** null, solange geladen wird oder wenn das Laden fehlschlug (Meldung kommt vom Interceptor). */
  protected readonly suggestions = signal<Suggestion[] | null>(null);
  /** Nach der ersten Entscheidung bleibt die Karte sichtbar, damit Meldung und Fokus nicht ins Leere gehen. */
  private readonly decided = signal(false);
  protected readonly visible = computed(() => !!this.suggestions()?.length || this.decided());

  /** Vorschläge, für die gerade eine Anfrage läuft. */
  protected readonly busy = signal<ReadonlySet<string>>(new Set());
  /** Gewählte Bewerbung je nicht zugeordnetem Vorschlag. */
  protected readonly chosen = signal<Readonly<Record<string, string>>>({});
  protected readonly message = signal('');
  /** Nicht abgeschlossene Bewerbungen für die Zuordnung. */
  protected readonly openApplications = signal<ApplicationSummary[]>([]);

  protected readonly formatDate = formatDate;

  constructor() {
    this.api
      .listSuggestions()
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: (list) => {
          this.suggestions.set(list);
          if (list.some((s) => !s.application_id)) {
            this.loadApplications();
          }
        },
        error: () => undefined,
      });
  }

  protected dueLabel(type: EventType): string {
    return DEADLINE_LABELS[type] ?? 'Frist';
  }

  protected choose(id: string, applicationId: string): void {
    this.chosen.update((c) => ({ ...c, [id]: applicationId }));
  }

  protected canAccept(s: Suggestion): boolean {
    return !this.busy().has(s.id) && (!!s.application_id || !!this.chosen()[s.id]);
  }

  protected accept(s: Suggestion): void {
    if (!this.canAccept(s)) {
      return;
    }
    const applicationId = s.application_id ? undefined : this.chosen()[s.id];
    this.run(s.id, this.api.acceptSuggestion(s.id, applicationId ? { application_id: applicationId } : {}), () => {
      this.snackBar.open('Übernommen', undefined, { duration: 3000, politeness: 'off' });
      return 'Übernommen.';
    });
  }

  protected dismiss(s: Suggestion): void {
    this.run(s.id, this.api.dismissSuggestion(s.id), () => 'Verworfen.');
  }

  /**
   * Führt eine Aktion für genau einen Vorschlag aus; die Antwort entfernt diesen Vorschlag (per id),
   * auch wenn sich die Liste inzwischen geändert hat. Fehlermeldungen zeigt der Interceptor.
   */
  private run(id: string, call: Observable<unknown>, done: () => string): void {
    if (this.busy().has(id)) {
      return;
    }
    this.setBusy(id, true);
    call
      .pipe(
        finalize(() => this.setBusy(id, false)),
        takeUntilDestroyed(this.destroyRef),
      )
      .subscribe({
        next: () => {
          this.message.set(done());
          this.remove(id);
        },
        error: () => undefined,
      });
  }

  private remove(id: string): void {
    const list = this.suggestions() ?? [];
    const index = list.findIndex((s) => s.id === id);
    if (index < 0) {
      return;
    }
    const next = list[index + 1]?.id;
    this.suggestions.set(list.filter((s) => s.id !== id));
    this.decided.set(true);
    this.chosen.update(({ [id]: _, ...rest }) => rest);
    afterNextRender(() => this.focus(next), { injector: this.injector });
  }

  private focus(id: string | undefined): void {
    const root = this.host.nativeElement;
    const item = id ? [...root.querySelectorAll<HTMLElement>('li.suggestion')].find((li) => li.dataset['id'] === id) : undefined;
    (item ?? root.querySelector<HTMLElement>('h2'))?.focus();
  }

  private setBusy(id: string, on: boolean): void {
    this.busy.update((b) => {
      const copy = new Set(b);
      if (on) {
        copy.add(id);
      } else {
        copy.delete(id);
      }
      return copy;
    });
  }

  private loadApplications(): void {
    this.api
      .listApplications()
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({ next: (apps) => this.openApplications.set(apps.filter((a) => a.phase !== 'Abgeschlossen')), error: () => undefined });
  }
}
