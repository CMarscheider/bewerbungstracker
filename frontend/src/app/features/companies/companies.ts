import { Component, ElementRef, inject, signal, viewChild } from '@angular/core';
import { FormControl, FormGroup, ReactiveFormsModule, Validators } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { finalize } from 'rxjs';
import { Company } from '../../api/models';
import { Api } from '../../core/api';
import { Icon } from '../../shared/icon';
import { OUTLINED_FIELDS } from '../../shared/form-field-defaults';

@Component({
  selector: 'app-companies',
  providers: [OUTLINED_FIELDS],
  imports: [ReactiveFormsModule, MatButtonModule, MatFormFieldModule, MatInputModule, Icon],
  templateUrl: './companies.html',
  styleUrl: './companies.scss',
})
export class Companies {
  private readonly api = inject(Api);

  /** null, bis die erste Antwort da ist. */
  protected readonly companies = signal<Company[] | null>(null);
  protected readonly loadError = signal(false);
  protected readonly editingId = signal<string | null>(null);
  protected readonly saving = signal(false);
  protected readonly renameForm = new FormGroup({
    name: new FormControl('', { nonNullable: true, validators: [Validators.required, Validators.maxLength(200)] }),
  });
  private readonly renameInput = viewChild<ElementRef<HTMLInputElement>>('renameInput');

  constructor() {
    this.reload();
  }

  protected startRename(c: Company): void {
    this.renameForm.setValue({ name: c.name });
    this.editingId.set(c.id);
    setTimeout(() => this.renameInput()?.nativeElement.focus());
  }

  protected cancelRename(): void {
    this.editingId.set(null);
  }

  protected saveRename(c: Company): void {
    const name = this.renameForm.controls.name.value.trim();
    if (!name || this.renameForm.invalid || this.saving()) {
      return;
    }
    this.saving.set(true);
    this.api
      .updateCompany(c.id, { name })
      .pipe(finalize(() => this.saving.set(false)))
      .subscribe({
        next: () => {
          this.editingId.set(null);
          this.reload();
        },
        error: () => undefined, // Meldung zeigt der Interceptor; das Formular bleibt offen.
      });
  }

  protected remove(c: Company): void {
    if (c.application_count > 0) {
      return;
    }
    if (window.confirm(`Firma „${c.name}“ löschen?`)) {
      this.api.deleteCompany(c.id).subscribe({ next: () => this.reload(), error: () => undefined });
    }
  }

  private reload(): void {
    this.api.listCompanies().subscribe({
      next: (list) => {
        this.loadError.set(false);
        this.companies.set(list);
      },
      error: () => this.loadError.set(true),
    });
  }
}
