import { Component, inject, signal } from '@angular/core';
import { FormControl, ReactiveFormsModule, Validators } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { Company } from '../../api/models';
import { Api } from '../../core/api';

@Component({
  selector: 'app-companies',
  imports: [ReactiveFormsModule, MatButtonModule, MatFormFieldModule, MatInputModule],
  templateUrl: './companies.html',
  styleUrl: './companies.scss',
})
export class Companies {
  private readonly api = inject(Api);

  protected readonly companies = signal<Company[]>([]);
  protected readonly editingId = signal<string | null>(null);
  protected readonly name = new FormControl('', { nonNullable: true, validators: [Validators.required] });

  constructor() {
    this.reload();
  }

  protected startRename(c: Company): void {
    this.name.setValue(c.name);
    this.editingId.set(c.id);
  }

  protected saveRename(c: Company): void {
    const name = this.name.value.trim();
    if (!name) {
      return;
    }
    this.api.updateCompany(c.id, { name }).subscribe(() => {
      this.editingId.set(null);
      this.reload();
    });
  }

  protected remove(c: Company): void {
    if (window.confirm(`Firma „${c.name}“ löschen?`)) {
      this.api.deleteCompany(c.id).subscribe(() => this.reload());
    }
  }

  private reload(): void {
    this.api.listCompanies().subscribe((list) => this.companies.set(list));
  }
}
