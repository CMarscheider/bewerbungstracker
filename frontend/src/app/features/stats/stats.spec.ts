import { TestBed } from '@angular/core/testing';
import { of, throwError } from 'rxjs';
import { Api } from '../../core/api';
import { Stats } from './stats';

const summary = {
  applied: 4,
  responded: 2,
  response_rate: 0.5,
  median_days_to_response: 3,
  avg_days_to_response: 4,
  rejections_after: [],
  by_source: [],
};

async function render(api: object) {
  TestBed.configureTestingModule({ imports: [Stats], providers: [{ provide: Api, useValue: api }] });
  const fixture = TestBed.createComponent(Stats);
  fixture.detectChanges();
  await fixture.whenStable();
  fixture.detectChanges();
  return fixture.nativeElement as HTMLElement;
}

describe('Stats', () => {
  it('rendert Funnel-Zeilen und Zusammenfassung', async () => {
    const el = await render({
      getFunnel: () => of([{ stage: 'Beworben', reached: 4, rate: 1 }, { stage: 'Interview', reached: 2, rate: 0.5 }]),
      getSummary: () => of(summary),
    });
    expect(el.querySelectorAll('.funnel .stage').length).toBe(2);
    expect(el.textContent).toContain('2 von 4 Bewerbungen beantwortet');
    expect(el.textContent).toContain('Noch keine Absagen.');
    expect(el.textContent).toContain('Noch keine Bewerbungen mit Quelle.');
  });

  it('zeigt Fehlertexte, wenn die Daten nicht geladen werden', async () => {
    const el = await render({
      getFunnel: () => throwError(() => new Error('x')),
      getSummary: () => throwError(() => new Error('x')),
    });
    expect(el.textContent).toContain('Konnte nicht geladen werden.');
    expect(el.textContent).not.toContain('Noch keine Absagen.');
  });
});
