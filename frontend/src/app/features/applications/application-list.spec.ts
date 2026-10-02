import { TestBed } from '@angular/core/testing';
import { FormGroup } from '@angular/forms';
import { provideRouter } from '@angular/router';
import { of, throwError } from 'rxjs';
import { ApplicationSummary } from '../../api/models';
import { Api } from '../../core/api';
import { ApplicationList } from './application-list';

const summaries: ApplicationSummary[] = [
  { id: 'a1', company_id: 'c1', company_name: 'Acme', position_title: 'Go', status: 'Beworben', phase: 'Aktiv', last_event_on: '2026-09-01' } as ApplicationSummary,
  { id: 'a2', company_id: 'c2', company_name: 'Beta', position_title: 'Java', status: 'Absage', phase: 'Abgeschlossen', last_event_on: '2026-09-02' } as ApplicationSummary,
];

describe('ApplicationList', () => {
  afterEach(() => vi.useRealTimers());

  async function setup() {
    const api = { listApplications: vi.fn(() => of(summaries)) };
    TestBed.configureTestingModule({ imports: [ApplicationList], providers: [provideRouter([]), { provide: Api, useValue: api }] });
    const fixture = TestBed.createComponent(ApplicationList);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
    return { api, fixture, el: fixture.nativeElement as HTMLElement };
  }

  it('zeigt Zeilen mit Link zur Detailseite', async () => {
    const { el } = await setup();
    const rows = el.querySelectorAll('tr.row');
    expect(rows.length).toBe(2);
    expect(rows[0].querySelector('a')?.getAttribute('href')).toBe('/bewerbungen/a1');
  });

  it('lädt nach Filteränderung mit Phase neu (entprellt)', async () => {
    const { api, fixture } = await setup();
    vi.useFakeTimers();
    const component = fixture.componentInstance as unknown as { filter: FormGroup };
    component.filter.patchValue({ phase: 'Aktiv' });
    vi.advanceTimersByTime(300);

    expect(api.listApplications).toHaveBeenLastCalledWith({ phase: 'Aktiv', status: undefined, q: undefined });
  });

  it('zeigt einen Fehlertext statt "Keine Bewerbungen gefunden."', async () => {
    const api = { listApplications: vi.fn(() => throwError(() => new Error('x'))) };
    TestBed.configureTestingModule({ imports: [ApplicationList], providers: [provideRouter([]), { provide: Api, useValue: api }] });
    const fixture = TestBed.createComponent(ApplicationList);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
    const text = (fixture.nativeElement as HTMLElement).textContent;
    expect(text).toContain('Konnte nicht geladen werden.');
    expect(text).not.toContain('Keine Bewerbungen gefunden.');
  });
});
