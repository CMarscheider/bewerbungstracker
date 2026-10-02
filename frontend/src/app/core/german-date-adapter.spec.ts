import { TestBed } from '@angular/core/testing';
import { DateAdapter } from '@angular/material/core';
import { provideGermanDates } from './german-date-adapter';

describe('GermanDateAdapter', () => {
  let adapter: DateAdapter<Date>;

  beforeEach(() => {
    TestBed.configureTestingModule({ providers: [provideGermanDates()] });
    adapter = TestBed.inject(DateAdapter<Date>);
  });

  it('versteht deutsche Eingaben', () => {
    const d = adapter.parse('2.10.2026', null) as Date;
    expect([d.getFullYear(), d.getMonth(), d.getDate()]).toEqual([2026, 9, 2]);
  });

  it('erkennt ungültige Tage', () => {
    expect(adapter.isValid(adapter.parse('31.02.2026', null) as Date)).toBe(false);
  });

  it('liefert null für leere Eingaben', () => {
    expect(adapter.parse('', null)).toBeNull();
  });
});
