import { DatePipe } from '@angular/common';
import { Component, DestroyRef, ElementRef, Injector, afterNextRender, inject, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { MatButtonModule } from '@angular/material/button';
import { MatExpansionModule } from '@angular/material/expansion';
import { MatSnackBar } from '@angular/material/snack-bar';
import { RouterLink } from '@angular/router';
import { finalize } from 'rxjs';
import { Icon } from '../../shared/icon';
import { ProcessedMail } from '../../api/models';
import { Api } from '../../core/api';

const LIMIT = 50;

@Component({
  selector: 'app-processed-mails',
  imports: [DatePipe, RouterLink, MatButtonModule, MatExpansionModule, Icon],
  templateUrl: './processed-mails.html',
  styleUrl: './processed-mails.scss',
})
export class ProcessedMails {
  private readonly api = inject(Api);
  private readonly snackBar = inject(MatSnackBar);
  private readonly destroyRef = inject(DestroyRef);
  private readonly injector = inject(Injector);
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);

  /** null = noch nicht geladen (die Liste lädt erst beim ersten Aufklappen). */
  protected readonly mails = signal<ProcessedMail[] | null>(null);
  protected readonly loading = signal(false);
  protected readonly loadError = signal(false);
  /** Mails, für die gerade eine Anfrage läuft. */
  protected readonly busy = signal<ReadonlySet<string>>(new Set());

  /** Beim Aufklappen laden – nur beim ersten Mal oder nach einem Fehler. */
  protected opened(): void {
    if (this.loading() || (this.mails() && !this.loadError())) {
      return;
    }
    this.loading.set(true);
    this.loadError.set(false);
    this.api
      .listProcessedMails(LIMIT)
      .pipe(
        finalize(() => this.loading.set(false)),
        takeUntilDestroyed(this.destroyRef),
      )
      .subscribe({
        next: (list) => this.mails.set(list),
        error: () => this.loadError.set(true), // Meldung zeigt der Interceptor
      });
  }

  protected recheck(mail: ProcessedMail): void {
    const id = mail.gmail_message_id;
    if (this.busy().has(id)) {
      return;
    }
    this.setBusy(id, true);
    this.api
      .deleteProcessedMail(id)
      .pipe(
        finalize(() => this.setBusy(id, false)),
        takeUntilDestroyed(this.destroyRef),
      )
      .subscribe({
        next: () => {
          this.snackBar.open('Die Mail wird beim nächsten Lauf erneut ausgewertet', undefined, { duration: 3000 });
          this.remove(id);
        },
        error: () => undefined,
      });
  }

  private remove(id: string): void {
    const list = this.mails() ?? [];
    const index = list.findIndex((m) => m.gmail_message_id === id);
    if (index < 0) {
      return;
    }
    const next = list[index + 1]?.gmail_message_id;
    this.mails.set(list.filter((m) => m.gmail_message_id !== id));
    afterNextRender(() => this.focus(next), { injector: this.injector });
  }

  /** Fokus auf die nächste Zeile, sonst auf den Kopf des Bereichs. */
  private focus(id: string | undefined): void {
    const root = this.host.nativeElement;
    const row = id ? [...root.querySelectorAll<HTMLElement>('li.mail')].find((li) => li.dataset['id'] === id) : undefined;
    (row ?? root.querySelector<HTMLElement>('mat-expansion-panel-header'))?.focus();
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
}
