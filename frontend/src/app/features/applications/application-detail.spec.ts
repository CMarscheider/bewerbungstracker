import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { of, throwError } from 'rxjs';
import { Application, EventType } from '../../api/models';
import { Api } from '../../core/api';
import { ApplicationDetail } from './application-detail';

const application: Application = {
  id: 'a1',
  company_id: 'c1',
  company_name: 'Acme',
  position_title: 'Go-Entwickler',
  status: 'Interview',
  phase: 'Aktiv',
  created_by_agent: false,
  created_at: '2026-09-01T10:00:00+02:00',
  updated_at: '2026-09-10T10:00:00+02:00',
  events: [
    { id: 'e1', type: 'Beworben', occurred_on: '2026-09-01', created_at: '2026-09-01T10:00:00+02:00' },
    { id: 'e2', type: 'Interview', occurred_on: '2026-09-10', interview_round: 1, note: 'Mit CTO', created_at: '2026-09-05T10:00:00+02:00' },
  ],
};

async function render(allowed: EventType[]) {
  const api = { getApplication: vi.fn(() => of(application)), listAllowedEvents: vi.fn(() => of(allowed)) };
  TestBed.configureTestingModule({ imports: [ApplicationDetail], providers: [provideRouter([]), { provide: Api, useValue: api }] });
  const fixture = TestBed.createComponent(ApplicationDetail);
  fixture.componentRef.setInput('id', 'a1');
  fixture.detectChanges();
  await fixture.whenStable();
  fixture.detectChanges();
  return { api, el: fixture.nativeElement as HTMLElement };
}

describe('ApplicationDetail', () => {
  it('zeigt nur die erlaubten Ereignisse als Buttons', async () => {
    const { api, el } = await render(['Interview', 'Absage']);
    const labels = [...el.querySelectorAll('[data-testid="event-button"]')].map((b) => b.textContent?.trim());
    expect(labels).toEqual(['Interview', 'Absage']);
    expect(api.getApplication).toHaveBeenCalledWith('a1');
  });

  it('zeigt die Timeline neueste zuerst mit Runde und Notiz', async () => {
    const { el } = await render([]);
    const items = [...el.querySelectorAll('.timeline li')].map((li) => li.textContent ?? '');
    expect(items[0]).toContain('Interview (Runde 1)');
    expect(items[0]).toContain('Mit CTO');
    expect(items[1]).toContain('Beworben');
  });

  it('meldet abgeschlossene Bewerbungen', async () => {
    const { el } = await render([]);
    expect(el.textContent).toContain('Diese Bewerbung ist abgeschlossen.');
  });

  it('zeigt eine Fehlermeldung, wenn die Bewerbung nicht geladen werden kann', async () => {
    const api = { getApplication: vi.fn(() => throwError(() => new Error('404'))), listAllowedEvents: vi.fn(() => of([])) };
    TestBed.configureTestingModule({ imports: [ApplicationDetail], providers: [provideRouter([]), { provide: Api, useValue: api }] });
    const fixture = TestBed.createComponent(ApplicationDetail);
    fixture.componentRef.setInput('id', 'x');
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
    expect((fixture.nativeElement as HTMLElement).textContent).toContain('Bewerbung nicht gefunden.');
  });
});
