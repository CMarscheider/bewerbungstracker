// Datumswerte der API sind Kalendertage ("YYYY-MM-DD"). Bewusst ohne toISOString(),
// das in UTC umrechnet und um Mitternacht den Tag verschieben würde.

export function toIsoDate(d: Date): string {
  const month = String(d.getMonth() + 1).padStart(2, '0');
  const day = String(d.getDate()).padStart(2, '0');
  return `${d.getFullYear()}-${month}-${day}`;
}

export function fromIsoDate(s: string): Date {
  const [y, m, d] = s.slice(0, 10).split('-').map(Number);
  return new Date(y, m - 1, d);
}

/** Nur für Kalendertag-Felder (YYYY-MM-DD); für Zeitstempel DatePipe verwenden. */
export function formatDate(s: string | null | undefined): string {
  if (!s) {
    return '–';
  }
  const [y, m, d] = s.slice(0, 10).split('-');
  return `${d}.${m}.${y}`;
}

/** "07.10." – für schmale Datumsspalten. */
export function formatDayMonth(s: string): string {
  const [, m, d] = s.slice(0, 10).split('-');
  return `${d}.${m}.`;
}

const WEEKDAYS = ['So', 'Mo', 'Di', 'Mi', 'Do', 'Fr', 'Sa'];

/** Kurzer deutscher Wochentag eines Kalendertags, z. B. "Mi". */
export function weekdayShort(s: string): string {
  return WEEKDAYS[fromIsoDate(s).getDay()];
}
