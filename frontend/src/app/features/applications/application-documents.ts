import { DatePipe } from '@angular/common';
import { Component, DestroyRef, ElementRef, computed, effect, inject, input, output, signal, untracked } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { FormControl, FormGroup, ReactiveFormsModule, Validators } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatButtonToggleModule } from '@angular/material/button-toggle';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatSnackBar } from '@angular/material/snack-bar';
import { Observable, Subscription, finalize } from 'rxjs';
import { Application, Documents, DocumentsInput, DocumentsState } from '../../api/models';
import { Api } from '../../core/api';

/** Zustände, in denen es (vielleicht) schon Unterlagen gibt. */
const WITH_DOCUMENTS: ReadonlySet<DocumentsState> = new Set(['erstellt', 'entwurf_angelegt', 'portal', 'fehler']);

export const GMAIL_DRAFTS_URL = 'https://mail.google.com/mail/u/0/#drafts';

const notBlank = Validators.pattern(/\S/);

@Component({
  selector: 'app-application-documents',
  imports: [DatePipe, ReactiveFormsModule, MatButtonModule, MatButtonToggleModule, MatFormFieldModule, MatInputModule],
  templateUrl: './application-documents.html',
  styleUrl: './application-documents.scss',
})
export class ApplicationDocuments {
  private readonly api = inject(Api);
  private readonly snackBar = inject(MatSnackBar);
  private readonly destroyRef = inject(DestroyRef);
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);

  readonly application = input.required<Application>();
  /** Neu geladene Bewerbung nach einer Zustandsänderung. */
  readonly changed = output<Application>();

  protected readonly docs = signal<Documents | null>(null);
  protected readonly loading = signal(false);
  protected readonly loadError = signal(false);
  protected readonly busy = signal(false);
  /** Nach dem Speichern enthält ein bereits angelegter Gmail-Entwurf noch die alte Fassung. */
  protected readonly draftStale = signal(false);

  protected readonly state = computed(() => this.application().documents_state);
  protected readonly pdfUrl = computed(() => `/api/v1/applications/${encodeURIComponent(this.application().id)}/documents/pdf`);
  protected readonly gmailUrl = GMAIL_DRAFTS_URL;
  protected readonly canDraft = computed(() => {
    const state = this.state();
    if (!this.application().contact_email || !this.docs()) {
      return false;
    }
    return state === 'erstellt' || state === 'portal' || (state === 'entwurf_angelegt' && this.draftStale());
  });

  /** Bewerbung, deren Unterlagen gezeigt werden; null = Zustand ohne Unterlagen. */
  private readonly docsKey = computed(() => (WITH_DOCUMENTS.has(this.state()) ? this.application().id : null));
  private docsLoad?: Subscription;

  protected readonly form = new FormGroup({
    language: new FormControl<'de' | 'en'>('de', { nonNullable: true }),
    cover_letter: new FormControl('', { nonNullable: true, validators: [Validators.required, notBlank, Validators.maxLength(3000)] }),
    profile_line: new FormControl('', { nonNullable: true, validators: [Validators.maxLength(400)] }),
    mail_subject: new FormControl('', {
      nonNullable: true,
      validators: [Validators.required, Validators.pattern(/^[^\r\n]*\S[^\r\n]*$/), Validators.maxLength(200)],
    }),
    mail_body: new FormControl('', { nonNullable: true, validators: [Validators.required, notBlank, Validators.maxLength(3000)] }),
  });

  constructor() {
    effect(() => {
      const id = this.docsKey();
      untracked(() => {
        this.draftStale.set(false);
        if (id) {
          this.loadDocuments();
        } else {
          this.docsLoad?.unsubscribe();
          this.loading.set(false);
          this.loadError.set(false);
          this.docs.set(null);
        }
      });
    });
  }

  protected loadDocuments(): void {
    const id = this.application().id;
    this.docsLoad?.unsubscribe();
    this.loading.set(true);
    this.loadError.set(false);
    this.docsLoad = this.api
      .getDocuments(id)
      .pipe(
        finalize(() => this.loading.set(false)),
        takeUntilDestroyed(this.destroyRef),
      )
      .subscribe({
        next: (d) => this.show(d),
        // 404 = noch keine Unterlagen (z. B. Fehler beim ersten Rendern).
        error: (e: { status?: number }) => (e?.status === 404 ? this.show(null) : this.loadError.set(true)),
      });
  }

  protected request(): void {
    if (this.busy() || (this.docs() && !window.confirm('Claude schreibt die Unterlagen neu. Fortfahren?'))) {
      return;
    }
    this.run(this.api.requestDocuments(this.application().id), (app) => {
      this.changed.emit(app);
      this.snackBar.open('Unterlagen angefordert', undefined, { duration: 3000 });
      this.focus('h2');
    });
  }

  protected refresh(): void {
    this.run(this.api.getApplication(this.application().id), (app) => {
      this.changed.emit(app);
      if (app.documents_state === 'angefordert') {
        this.snackBar.open('Die Unterlagen sind noch nicht fertig', undefined, { duration: 3000 });
      }
    });
  }

  protected save(): void {
    if (this.form.invalid) {
      return;
    }
    const v = this.form.getRawValue();
    const body: DocumentsInput = {
      language: v.language,
      cover_letter: v.cover_letter.trim(),
      profile_line: v.profile_line.trim(),
      // Schwerpunkte bearbeitet nur Claude; unverändert mitsenden.
      highlights: this.docs()?.highlights ?? [],
      mail_subject: v.mail_subject.trim(),
      mail_body: v.mail_body.trim(),
    };
    const id = this.application().id;
    const wasFailed = this.state() === 'fehler';
    this.run(this.api.updateDocuments(id, body), (d) => {
      this.show(d);
      if (this.state() === 'entwurf_angelegt') {
        this.draftStale.set(true);
      }
      this.snackBar.open('Gespeichert und neu gerendert', undefined, { duration: 3000 });
      if (wasFailed) {
        // Aus „fehler“ wird wieder „erstellt“ bzw. „portal“.
        this.api
          .getApplication(id)
          .pipe(takeUntilDestroyed(this.destroyRef))
          .subscribe({ next: (app) => this.changed.emit(app), error: () => undefined });
      }
    });
  }

  protected createDraft(): void {
    if (this.form.dirty) {
      return;
    }
    this.run(this.api.createDraft(this.application().id), (app) => {
      this.changed.emit(app);
      this.draftStale.set(false);
      this.snackBar.open('Gmail-Entwurf angelegt', undefined, { duration: 3000 });
    });
  }

  /** Führt eine Aktion mit busy-Sperre aus; Fehlermeldungen zeigt der Interceptor. */
  private run<T>(call: Observable<T>, next: (value: T) => void): void {
    if (this.busy()) {
      return;
    }
    this.busy.set(true);
    call
      .pipe(
        finalize(() => this.busy.set(false)),
        takeUntilDestroyed(this.destroyRef),
      )
      .subscribe({ next, error: () => undefined });
  }

  private show(d: Documents | null): void {
    this.docs.set(d);
    this.form.reset({
      language: d?.language ?? 'de',
      cover_letter: d?.cover_letter ?? '',
      profile_line: d?.profile_line ?? '',
      mail_subject: d?.mail_subject ?? '',
      mail_body: d?.mail_body ?? '',
    });
  }

  private focus(selector: string): void {
    this.host.nativeElement.querySelector<HTMLElement>(selector)?.focus();
  }
}
