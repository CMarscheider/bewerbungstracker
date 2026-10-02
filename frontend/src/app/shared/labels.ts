import { EventType, Phase } from '../api/models';

export const EVENT_LABELS: Record<EventType, string> = {
  Vorgemerkt: 'Vorgemerkt',
  Beworben: 'Beworben',
  ScreeningGespraech: 'Screening-Gespräch',
  ChallengeErhalten: 'Challenge erhalten',
  ChallengeAbgegeben: 'Challenge abgegeben',
  Interview: 'Interview',
  Kennenlerntag: 'Kennenlerntag',
  AngebotErhalten: 'Angebot erhalten',
  AngebotAngenommen: 'Angebot angenommen',
  AngebotAbgelehnt: 'Angebot abgelehnt',
  Absage: 'Absage',
  Zurueckgezogen: 'Zurückgezogen',
  KeineRueckmeldung: 'Keine Rückmeldung',
};

export const ALL_EVENT_TYPES = Object.keys(EVENT_LABELS) as EventType[];

export const PHASE_LABELS: Record<Phase, string> = {
  Vorbereitung: 'Vorbereitung',
  Aktiv: 'Aktiv',
  Abgeschlossen: 'Abgeschlossen',
};

export const ALL_PHASES = Object.keys(PHASE_LABELS) as Phase[];

// Anzeige-Wissen, gespiegelt aus der Spec (Statusmodell). Die Regeln prüft allein das Backend.
export const DEADLINE_TYPES: ReadonlySet<EventType> = new Set<EventType>(['Vorgemerkt', 'ChallengeErhalten', 'AngebotErhalten']);
export const FUTURE_DATE_TYPES: ReadonlySet<EventType> = new Set<EventType>(['ScreeningGespraech', 'Interview', 'Kennenlerntag']);

export const DEADLINE_LABELS: Partial<Record<EventType, string>> = {
  Vorgemerkt: 'Bewerbungsschluss',
  ChallengeErhalten: 'Abgabe',
  AngebotErhalten: 'Antwortfrist',
};

export function eventLabel(type: EventType, round?: number | null): string {
  return type === 'Interview' && round ? `Interview (Runde ${round})` : EVENT_LABELS[type];
}
