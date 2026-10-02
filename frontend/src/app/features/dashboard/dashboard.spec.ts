import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { of, throwError } from 'rxjs';
import { Api } from '../../core/api';
import { Dashboard } from './dashboard';

describe('Dashboard', () => {
  it('rendert Fristen, Termine und Kacheln, auch wenn die Zusammenfassung fehlschlägt', async () => {
    const api = {
      listDeadlines: vi.fn(() =>
        of([
          { application_id: 'a1', company_name: 'Acme', position_title: 'Dev', due_on: '2026-09-30', event_type: 'ChallengeErhalten', overdue: true },
          { application_id: 'a2', company_name: 'Beta', position_title: 'Ops', due_on: '2026-10-05', event_type: 'Vorgemerkt', overdue: false },
        ]),
      ),
      listAppointments: vi.fn(() =>
        of([{ application_id: 'a3', company_name: 'Gamma', position_title: 'QA', date: '2026-10-08', event_type: 'Interview', interview_round: 2 }]),
      ),
      getSummary: vi.fn(() => throwError(() => new Error('x'))),
      listApplications: vi.fn(() => of([{ id: '1' }, { id: '2' }, { id: '3' }])),
    };
    TestBed.configureTestingModule({ imports: [Dashboard], providers: [provideRouter([]), { provide: Api, useValue: api }] });
    const fixture = TestBed.createComponent(Dashboard);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    expect(el.textContent).toContain('Überfällige Fristen');
    expect(el.textContent).toContain('Interview (Runde 2)');
    expect(el.querySelector('.tile .value')?.textContent?.trim()).toBe('3');
    expect(el.querySelectorAll('.tile .value')[2].textContent?.trim()).toBe('–');
  });

  it('zeigt Fehlertexte statt Leerzuständen, wenn Fristen und Termine nicht laden', async () => {
    const fail = vi.fn(() => throwError(() => new Error('x')));
    const api = { listDeadlines: fail, listAppointments: fail, getSummary: fail, listApplications: fail };
    TestBed.configureTestingModule({ imports: [Dashboard], providers: [provideRouter([]), { provide: Api, useValue: api }] });
    const fixture = TestBed.createComponent(Dashboard);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    expect(el.textContent).toContain('Konnte nicht geladen werden.');
    expect(el.textContent).not.toContain('Keine anstehenden Fristen.');
    expect(el.textContent).not.toContain('Keine Termine');
  });
});
