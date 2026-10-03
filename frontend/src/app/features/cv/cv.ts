import { Component, inject, signal } from '@angular/core';
import { FormArray, FormGroup, ReactiveFormsModule } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatSnackBar } from '@angular/material/snack-bar';
import { finalize } from 'rxjs';
import { Api } from '../../core/api';
import { cvForm, CvForm, educationGroup, experienceGroup, formToCv, languageGroup, linkGroup, projectGroup, skillGroup } from './cv-form';

@Component({
  selector: 'app-cv',
  imports: [ReactiveFormsModule, MatButtonModule, MatFormFieldModule, MatInputModule],
  templateUrl: './cv.html',
  styleUrl: './cv.scss',
})
export class CvPage {
  private readonly api = inject(Api);
  private readonly snackBar = inject(MatSnackBar);

  protected readonly form = signal<CvForm | null>(null);
  protected readonly loadError = signal(false);
  protected readonly saving = signal(false);
  protected readonly updatedAt = signal<string | undefined>(undefined);

  protected readonly newLink = linkGroup;
  protected readonly newExperience = experienceGroup;
  protected readonly newEducation = educationGroup;
  protected readonly newSkill = skillGroup;
  protected readonly newProject = projectGroup;
  protected readonly newLanguage = languageGroup;

  constructor() {
    this.api.getCv().subscribe({
      next: (cv) => {
        this.form.set(cvForm(cv));
        this.updatedAt.set(cv.updated_at);
      },
      error: () => this.loadError.set(true),
    });
  }

  protected add<T extends FormGroup>(list: FormArray<T>, create: () => T): void {
    list.push(create());
  }

  protected remove(list: FormArray, index: number): void {
    list.removeAt(index);
  }

  protected save(): void {
    const form = this.form();
    if (!form || this.saving()) {
      return;
    }
    if (form.invalid) {
      form.markAllAsTouched();
      return;
    }
    this.saving.set(true);
    this.api
      .saveCv(formToCv(form))
      .pipe(finalize(() => this.saving.set(false)))
      .subscribe({
        next: (cv) => {
          this.updatedAt.set(cv.updated_at);
          form.markAsPristine();
          this.snackBar.open('Lebenslauf gespeichert', undefined, { duration: 3000 });
        },
        error: () => undefined, // Meldung zeigt der Interceptor.
      });
  }
}
