import { Injectable, Provider } from '@angular/core';
import { DateAdapter, MAT_DATE_LOCALE, NativeDateAdapter, provideNativeDateAdapter } from '@angular/material/core';

const GERMAN_DATE = /^\s*(\d{1,2})\.(\d{1,2})\.(\d{4})\s*$/;

/** NativeDateAdapter, der zusätzlich Eingaben wie "02.10.2026" versteht. */
@Injectable()
export class GermanDateAdapter extends NativeDateAdapter {
  override parse(value: unknown, parseFormat?: unknown): Date | null {
    if (typeof value === 'string') {
      const match = GERMAN_DATE.exec(value);
      if (match) {
        const [day, month, year] = [Number(match[1]), Number(match[2]), Number(match[3])];
        const date = new Date(year, month - 1, day);
        const valid = date.getFullYear() === year && date.getMonth() === month - 1 && date.getDate() === day;
        return valid ? date : this.invalid();
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
