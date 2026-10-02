import { Injectable, Provider } from '@angular/core';
import { DateAdapter, MAT_DATE_LOCALE, NativeDateAdapter, provideNativeDateAdapter } from '@angular/material/core';

const GERMAN_DATE = /^\s*(\d{1,2})\.(\d{1,2})\.(\d{4}|\d{2})?\s*$/;

/** NativeDateAdapter, der zusätzlich "02.10.2026", "2.10.26" und "2.10." versteht. */
@Injectable()
export class GermanDateAdapter extends NativeDateAdapter {
  override parse(value: unknown, parseFormat?: unknown): Date | null {
    if (typeof value === 'string') {
      const match = GERMAN_DATE.exec(value);
      if (match) {
        const day = Number(match[1]);
        const month = Number(match[2]);
        let year = new Date().getFullYear();
        if (match[3]) {
          year = match[3].length === 2 ? 2000 + Number(match[3]) : Number(match[3]);
        }
        const date = new Date(year, month - 1, day);
        const valid = date.getFullYear() === year && date.getMonth() === month - 1 && date.getDate() === day;
        return valid ? date : this.invalid();
      }
      // Punkt-Eingaben nie an Date.parse geben (V8 liest "02.10.26" als 10. Februar).
      if (value.includes('.')) {
        return this.invalid();
      }
    }
    return super.parse(value, parseFormat);
  }
}

export function provideGermanDates(): Provider[] {
  return [
    provideNativeDateAdapter(),
    { provide: DateAdapter, useClass: GermanDateAdapter },
    { provide: MAT_DATE_LOCALE, useValue: 'de-DE' },
  ];
}
