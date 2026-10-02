import { ALL_EVENT_TYPES, DEADLINE_TYPES, FUTURE_DATE_TYPES, eventLabel } from './labels';

describe('labels', () => {
  it('kennt alle 13 Ereignistypen', () => {
    expect(ALL_EVENT_TYPES.length).toBe(13);
  });

  it('kennt die Typen mit Frist und mit Zukunftsdatum wie das Backend', () => {
    expect([...DEADLINE_TYPES]).toEqual(['Vorgemerkt', 'ChallengeErhalten', 'AngebotErhalten']);
    expect([...FUTURE_DATE_TYPES]).toEqual(['ScreeningGespraech', 'Interview', 'Kennenlerntag']);
  });

  it('beschriftet Interviews mit Runde', () => {
    expect(eventLabel('Interview', 2)).toBe('Interview (Runde 2)');
    expect(eventLabel('Interview')).toBe('Interview');
    expect(eventLabel('KeineRueckmeldung')).toBe('Keine Rückmeldung');
  });
});
