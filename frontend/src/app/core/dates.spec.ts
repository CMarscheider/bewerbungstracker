import { formatDate, fromIsoDate, toIsoDate } from './dates';

describe('dates', () => {
  it('wandelt ein lokales Datum ohne Zeitzonenverschiebung in YYYY-MM-DD', () => {
    expect(toIsoDate(new Date(2026, 0, 5, 23, 59))).toBe('2026-01-05');
    expect(toIsoDate(new Date(2026, 9, 2, 0, 1))).toBe('2026-10-02');
  });

  it('liest YYYY-MM-DD als lokales Datum', () => {
    const d = fromIsoDate('2026-10-02');
    expect([d.getFullYear(), d.getMonth(), d.getDate()]).toEqual([2026, 9, 2]);
  });

  it('formatiert für die Anzeige auf Deutsch', () => {
    expect(formatDate('2026-10-02')).toBe('02.10.2026');
    expect(formatDate('2026-10-02T23:34:56+02:00')).toBe('02.10.2026');
    expect(formatDate(undefined)).toBe('–');
  });
});
