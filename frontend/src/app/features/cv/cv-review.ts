import { DatePipe } from '@angular/common';
import { Component, DestroyRef, OnInit, computed, inject, input, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { MatButtonModule } from '@angular/material/button';
import { MatSnackBar } from '@angular/material/snack-bar';
import { finalize } from 'rxjs';
import { Cv, CvReview } from '../../api/models';
import { Api } from '../../core/api';
import { applySection, CvForm, formToCv } from './cv-form';
import { changedSections, CvSection, SECTION_LABELS, sectionLines } from './cv-review-model';

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

  readonly form = input.required<CvForm>();
  /** updated_at des gespeicherten Lebenslaufs; undefined = noch nie gespeichert. */
  readonly savedAt = input<string | undefined>();
  readonly dirty = input(false);

  protected readonly review = signal<CvReview | null>(null);
  protected readonly loading = signal(true);
  protected readonly busy = signal(false);
  protected readonly applied = signal<ReadonlySet<CvSection>>(new Set());
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

  protected readonly stale = computed(() => {
    const r = this.review();
    const saved = this.savedAt();
    return r?.state === 'fertig' && !!saved && Date.parse(saved) !== Date.parse(r.based_on_updated_at);
  });

  /** Erst in ngOnInit laden: show() liest das Pflicht-Input `form`, das im Konstruktor noch fehlt. */
  ngOnInit(): void {
    this.api
      .getCvReview()
      .pipe(
        finalize(() => this.loading.set(false)),
        takeUntilDestroyed(this.destroyRef),
      )
      .subscribe({ next: (r) => this.show(r), error: () => undefined }); // 404 = keine offene Optimierung.
  }

  protected request(): void {
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
        error: () => undefined,
      });
  }

  protected close(): void {
    this.busy.set(true);
    this.api
      .closeCvReview()
      .pipe(
        finalize(() => this.busy.set(false)),
        takeUntilDestroyed(this.destroyRef),
      )
      .subscribe({ next: () => this.show(null), error: () => undefined });
  }

  protected apply(section: CvSection): void {
    const proposal = this.review()?.proposal;
    if (!proposal || this.applied().has(section)) {
      return;
    }
    applySection(this.form(), section, proposal);
    this.applied.update((s) => new Set([...s, section]));
    this.snackBar.open('Übernommen – zum Speichern unten auf „Speichern“ klicken', undefined, { duration: 4000 });
  }

  protected applyAll(): void {
    this.changes().forEach((c) => this.apply(c.section));
  }

  private show(r: CvReview | null): void {
    this.review.set(r);
    this.applied.set(new Set());
    this.baseline.set(r?.proposal ? formToCv(this.form()) : null);
  }
}
