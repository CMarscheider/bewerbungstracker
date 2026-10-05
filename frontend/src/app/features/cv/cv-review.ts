import { DatePipe } from '@angular/common';
import { Component, DestroyRef, ElementRef, OnInit, computed, inject, input, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { MatButtonModule } from '@angular/material/button';
import { MatSnackBar } from '@angular/material/snack-bar';
import { finalize } from 'rxjs';
import { Cv, CvReview } from '../../api/models';
import { Api } from '../../core/api';
import { applySection, CvForm, formToCv } from './cv-form';
import { changedSections, CvSection, SECTION_LABELS, sectionLines } from './cv-review-model';

const SAVE_HINT = 'zum Speichern unten auf „Speichern“ klicken';

@Component({
  selector: 'app-cv-review',
  imports: [DatePipe, MatButtonModule],
  templateUrl: './cv-review.html',
  styleUrl: './cv-review.scss',
})
export class CvReviewPanel implements OnInit {
  private readonly api = inject(Api);
  private readonly snackBar = inject(MatSnackBar);
  private readonly destroyRef = inject(DestroyRef);
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);

  readonly form = input.required<CvForm>();
  /** updated_at des gespeicherten Lebenslaufs; undefined = noch nie gespeichert. */
  readonly savedAt = input<string | undefined>();
  readonly dirty = input(false);

  protected readonly review = signal<CvReview | null>(null);
  protected readonly loading = signal(true);
  protected readonly loadError = signal(false);
  protected readonly busy = signal(false);
  protected readonly applied = signal<ReadonlySet<CvSection>>(new Set());
  /** Beim Laden festgestellt – späteres Speichern übernommener Abschnitte darf die Warnung nicht auslösen. */
  protected readonly stale = signal(false);
  /** Formularstand beim Laden des Vorschlags – Grundlage für „Bisher“. */
  private readonly baseline = signal<Cv | null>(null);

  protected readonly changes = computed(() => {
    const proposal = this.review()?.proposal;
    const before = this.baseline();
    if (!proposal || !before) {
      return [];
    }
    return changedSections(before, proposal).map((section) => ({
      section,
      label: SECTION_LABELS[section],
      before: sectionLines(before, section),
      after: sectionLines(proposal, section),
    }));
  });

  protected readonly allApplied = computed(() => this.changes().every((c) => this.applied().has(c.section)));

  /** show() liest das Pflicht-Input `form`, das im Konstruktor noch fehlt – daher erst hier laden. */
  ngOnInit(): void {
    this.load();
  }

  protected load(): void {
    this.loading.set(true);
    this.loadError.set(false);
    this.api
      .getCvReview()
      .pipe(
        finalize(() => this.loading.set(false)),
        takeUntilDestroyed(this.destroyRef),
      )
      .subscribe({
        next: (r) => this.show(r),
        // 404 = keine offene Optimierung.
        error: (e: { status?: number }) => (e?.status === 404 ? this.show(null) : this.loadError.set(true)),
      });
  }

  protected request(): void {
    if (this.busy()) {
      return;
    }
    this.busy.set(true);
    this.api
      .requestCvReview()
      .pipe(
        finalize(() => this.busy.set(false)),
        takeUntilDestroyed(this.destroyRef),
      )
      .subscribe({
        next: (r) => {
          this.show(r);
          this.snackBar.open('Optimierung angefordert', undefined, { duration: 3000 });
        },
        error: () => this.load(),
      });
  }

  protected withdraw(): void {
    this.finish('Anfrage zurückgezogen');
  }

  protected close(): void {
    if (this.applied().size === 0 && !window.confirm('Alle Vorschläge von Claude verwerfen?')) {
      return;
    }
    this.finish('Optimierung abgeschlossen');
  }

  protected apply(section: CvSection): void {
    if (this.applyOne(section)) {
      this.snackBar.open(`Übernommen – ${SAVE_HINT}`, undefined, { duration: 4000 });
      this.focus(`[data-head="${section}"]`);
    }
  }

  protected applyAll(): void {
    const any = this.changes().filter((c) => this.applyOne(c.section)).length > 0;
    if (any) {
      this.snackBar.open(`Alle Vorschläge übernommen – ${SAVE_HINT}`, undefined, { duration: 4000 });
      this.focus('h2');
    }
  }

  private applyOne(section: CvSection): boolean {
    const proposal = this.review()?.proposal;
    if (!proposal || this.applied().has(section)) {
      return false;
    }
    applySection(this.form(), section, proposal);
    this.applied.update((s) => new Set([...s, section]));
    return true;
  }

  private finish(message: string): void {
    if (this.busy()) {
      return;
    }
    this.busy.set(true);
    this.api
      .closeCvReview()
      .pipe(
        finalize(() => this.busy.set(false)),
        takeUntilDestroyed(this.destroyRef),
      )
      .subscribe({
        next: () => {
          this.show(null);
          this.snackBar.open(message, undefined, { duration: 3000 });
          this.focus('h2');
        },
        error: () => this.load(),
      });
  }

  private focus(selector: string): void {
    this.host.nativeElement.querySelector<HTMLElement>(selector)?.focus();
  }

  private show(r: CvReview | null): void {
    const saved = this.savedAt();
    this.review.set(r);
    this.applied.set(new Set());
    this.baseline.set(r?.proposal ? formToCv(this.form()) : null);
    this.stale.set(r?.state === 'fertig' && !!saved && Date.parse(saved) !== Date.parse(r.based_on_updated_at));
  }
}
