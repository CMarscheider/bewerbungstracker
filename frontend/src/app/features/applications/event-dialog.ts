import { Component, inject } from '@angular/core';
import { FormControl, FormGroup, ReactiveFormsModule, Validators } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatDatepickerModule } from '@angular/material/datepicker';
import { MAT_DIALOG_DATA, MatDialogModule, MatDialogRef } from '@angular/material/dialog';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { EventType, NewEvent } from '../../api/models';
import { toIsoDate } from '../../core/dates';
import { DEADLINE_LABELS, DEADLINE_TYPES, EVENT_LABELS, FUTURE_DATE_TYPES } from '../../shared/labels';
import { OUTLINED_FIELDS } from '../../shared/form-field-defaults';

export interface EventDialogData {
  type: EventType;
}

@Component({
  selector: 'app-event-dialog',
  providers: [OUTLINED_FIELDS],
  imports: [ReactiveFormsModule, MatButtonModule, MatDatepickerModule, MatDialogModule, MatFormFieldModule, MatInputModule],
  templateUrl: './event-dialog.html',
  styles: `.fields { display: flex; flex-direction: column; min-width: min(360px, 80vw); padding-top: 8px; }`,
})
export class EventDialog {
  private readonly data = inject<EventDialogData>(MAT_DIALOG_DATA);
  private readonly dialogRef = inject<MatDialogRef<EventDialog, NewEvent>>(MatDialogRef);

  protected readonly title = EVENT_LABELS[this.data.type];
  protected readonly showDeadline = DEADLINE_TYPES.has(this.data.type);
  protected readonly deadlineLabel = DEADLINE_LABELS[this.data.type] ?? 'Frist';
  // Termine dürfen in der Zukunft liegen, alles andere nicht (Backend prüft dasselbe).
  protected readonly maxDate = FUTURE_DATE_TYPES.has(this.data.type) ? null : new Date();

  protected readonly form = new FormGroup({
    occurred_on: new FormControl<Date>(new Date(), { nonNullable: true, validators: [Validators.required] }),
    due_on: new FormControl<Date | null>(null),
    note: new FormControl('', { nonNullable: true }),
  });

  protected save(): void {
    if (this.form.invalid) {
      return;
    }
    const v = this.form.getRawValue();
    const event: NewEvent = { type: this.data.type, occurred_on: toIsoDate(v.occurred_on) };
    if (this.showDeadline && v.due_on) {
      event.due_on = toIsoDate(v.due_on);
    }
    if (v.note.trim()) {
      event.note = v.note.trim();
    }
    this.dialogRef.close(event);
  }
}
