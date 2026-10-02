import { Component, computed, inject, signal } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { FormControl, FormGroup, ReactiveFormsModule, Validators } from '@angular/forms';
import { MatAutocompleteModule } from '@angular/material/autocomplete';
import { MatButtonModule } from '@angular/material/button';
import { MatButtonToggleModule } from '@angular/material/button-toggle';
import { MatDatepickerModule } from '@angular/material/datepicker';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { Router, RouterLink } from '@angular/router';
import { finalize, map, of, switchMap, tap } from 'rxjs';
import { Company, NewEvent } from '../../api/models';
import { Api } from '../../core/api';
import { toIsoDate } from '../../core/dates';

type FirstType = 'Beworben' | 'Vorgemerkt';

@Component({
  selector: 'app-application-new',
  imports: [ReactiveFormsModule, RouterLink, MatAutocompleteModule, MatButtonModule, MatButtonToggleModule, MatDatepickerModule, MatFormFieldModule, MatInputModule],
  templateUrl: './application-new.html',
  styleUrl: './application-new.scss',
})
export class ApplicationNew {
  private readonly api = inject(Api);
  private readonly router = inject(Router);

  protected readonly today = new Date();
  protected readonly saving = signal(false);
  protected readonly companies = signal<Company[]>([]);

  protected readonly form = new FormGroup({
    company: new FormControl('', { nonNullable: true, validators: [Validators.required] }),
    position_title: new FormControl('', { nonNullable: true, validators: [Validators.required] }),
    job_url: new FormControl('', { nonNullable: true }),
    location: new FormControl('', { nonNullable: true }),
    source: new FormControl('', { nonNullable: true }),
    notes: new FormControl('', { nonNullable: true }),
    first_type: new FormControl<FirstType>('Beworben', { nonNullable: true }),
    occurred_on: new FormControl<Date>(new Date(), { nonNullable: true, validators: [Validators.required] }),
    due_on: new FormControl<Date | null>(null),
  });

  constructor() {
    this.api.listCompanies().subscribe((list) => this.companies.set(list));
  }

  private readonly companyName = toSignal(this.form.controls.company.valueChanges, { initialValue: '' });
  protected readonly firstType = toSignal(this.form.controls.first_type.valueChanges, { initialValue: 'Beworben' as FirstType });

  protected readonly suggestions = computed(() => {
    const q = this.companyName().trim().toLowerCase();
    return this.companies()
      .filter((c) => c.name.toLowerCase().includes(q))
      .slice(0, 8);
  });

  protected readonly isNewCompany = computed(() => {
    const name = this.companyName().trim().toLowerCase();
    return name !== '' && !this.companies().some((c) => c.name.toLowerCase() === name);
  });

  protected submit(): void {
    if (this.form.invalid || this.saving()) {
      this.form.markAllAsTouched();
      return;
    }
    const v = this.form.getRawValue();
    const name = v.company.trim();
    const existing = this.companies().find((c) => c.name.toLowerCase() === name.toLowerCase());
    const companyId$ = existing ? of(existing.id) : this.api.createCompany({ name }).pipe(
          tap((c) => this.companies.update((list) => [...list, c])),
          map((c) => c.id),
        );

    const firstEvent: NewEvent = { type: v.first_type, occurred_on: toIsoDate(v.occurred_on) };
    if (v.first_type === 'Vorgemerkt' && v.due_on) {
      firstEvent.due_on = toIsoDate(v.due_on);
    }

    this.saving.set(true);
    companyId$
      .pipe(
        switchMap((company_id) =>
          this.api.createApplication({
            company_id,
            position_title: v.position_title.trim(),
            job_url: optional(v.job_url),
            location: optional(v.location),
            source: optional(v.source),
            notes: optional(v.notes),
            first_event: firstEvent,
          }),
        ),
        finalize(() => this.saving.set(false)),
      )
      .subscribe({
        next: (app) => void this.router.navigate(['/bewerbungen', app.id]),
        error: () => undefined, // Meldung zeigt der Interceptor; das Formular bleibt für den nächsten Versuch.
      });
  }
}

function optional(value: string): string | undefined {
  const trimmed = value.trim();
  return trimmed ? trimmed : undefined;
}
