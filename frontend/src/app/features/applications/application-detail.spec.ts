import { TestBed } from '@angular/core/testing';
import { FormGroup } from '@angular/forms';
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
  documents_state: 'keine',
  events: [
    { id: 'e1', type: 'Beworben', occurred_on: '2026-09-01', created_at: '2026-09-01T10:00:00+02:00' },
    { id: 'e2', type: 'Interview', occurred_on: '2026-09-10', interview_round: 1, note: 'Mit CTO', created_at: '2026-09-05T10:00:00+02:00' },
  ],
};

async function render(allowed: EventType[], app: Application = application) {
  const api = {
    getApplication: vi.fn(() => of(app)),
    listAllowedEvents: vi.fn(() => of(allowed)),
    updateApplication: vi.fn((_id: string, body: object) => of({ ...app, ...body })),
    requestDocuments: vi.fn(() => of({ ...app, documents_state: 'angefordert' })),
    getDocuments: vi.fn(() => throwError(() => ({ status: 404 }))),
    clearGmailThread: vi.fn(() => of(undefined)),
  };
  TestBed.configureTestingModule({ imports: [ApplicationDetail], providers: [provideRouter([]), { provide: Api, useValue: api }] });
  const fixture = TestBed.createComponent(ApplicationDetail);
  fixture.componentRef.setInput('id', 'a1');
  fixture.detectChanges();
  await fixture.whenStable();
  fixture.detectChanges();
  return { api, fixture, el: fixture.nativeElement as HTMLElement };
}

const agentJob: Application = {
  ...application,
  fit_score: 82,
  fit_reason: 'Passt gut zu Angular und Go.',
  posting_text: 'Wir suchen\nAngular-Entwickler',
  contact_email: 'jobs@acme.de',
  created_by_agent: true,
};

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

  it('zeigt die Passung mit Score und Begründung', async () => {
    const { el } = await render([], agentJob);
    const fit = el.querySelector('[data-testid="fit"]') as HTMLElement;
    expect(fit.classList).toContain('fit-section');
    expect(fit.textContent).toContain('Passung');
    expect(fit.querySelector('[role="img"]')?.textContent?.trim()).toBe('82');
    expect(fit.textContent).toContain('Passt gut zu Angular und Go.');
  });

  it('zeigt ohne Score keinen Bereich Passung', async () => {
    const { el } = await render([]);
    expect(el.querySelector('[data-testid="fit"]')).toBeNull();
    expect(el.querySelector('mat-expansion-panel')).toBeNull();
  });

  it('zeigt den Anzeigentext in einem zugeklappten Bereich', async () => {
    const { el } = await render([], agentJob);
    const panel = el.querySelector('mat-expansion-panel') as HTMLElement;
    expect(panel.textContent).toContain('Anzeigentext');
    expect(panel.classList).not.toContain('mat-expanded');
  });

  it('zeigt die Bewerbungs-E-Mail als mailto-Link', async () => {
    const { el } = await render([], agentJob);
    const link = el.querySelector('a[href^="mailto:"]');
    expect(link?.getAttribute('href')).toBe('mailto:' + encodeURIComponent('jobs@acme.de'));
    expect(el.querySelector('dl')?.textContent).toContain('Bewerbungs-E-Mail');
  });

  it('kodiert die Adresse im mailto-Link', async () => {
    const { el } = await render([], { ...agentJob, contact_email: 'a+b@acme.de?cc=x' });
    expect(el.querySelector('a[href^="mailto:"]')?.getAttribute('href')).toBe('mailto:a%2Bb%40acme.de%3Fcc%3Dx');
  });

  it('zeigt Anzeigentext und Begründung als reinen Text', async () => {
    const evil = '<img src=x onerror=alert(1)>';
    const { fixture, el } = await render([], { ...agentJob, fit_reason: evil, posting_text: evil });
    (el.querySelector('mat-expansion-panel-header') as HTMLElement).click();
    fixture.detectChanges();
    await fixture.whenStable();
    expect(el.querySelector('img')).toBeNull();
    expect(el.querySelector('[data-testid="fit"]')?.textContent).toContain(evil);
    expect(el.querySelector('.posting .pre')?.textContent).toBe(evil);
  });

  it('sendet die Bewerbungs-E-Mail beim Speichern mit (leer löscht)', async () => {
    const { api, fixture } = await render([], agentJob);
    const component = fixture.componentInstance as unknown as { form: FormGroup; startEdit(): void; saveDetails(): void };
    component.startEdit();
    expect(component.form.value.contact_email).toBe('jobs@acme.de');
    component.form.patchValue({ contact_email: ' ' });
    component.saveDetails();
    expect(api.updateApplication).toHaveBeenCalledWith('a1', expect.objectContaining({ contact_email: '' }));
  });

  it('prüft das Format der Bewerbungs-E-Mail', async () => {
    const { api, fixture, el } = await render([], agentJob);
    const component = fixture.componentInstance as unknown as { form: FormGroup; startEdit(): void; saveDetails(): void };
    component.startEdit();
    component.form.patchValue({ contact_email: 'keine-mail' });
    component.form.markAllAsTouched();
    fixture.detectChanges();
    await fixture.whenStable();
    component.saveDetails();
    expect(api.updateApplication).not.toHaveBeenCalled();
    expect(el.textContent).toContain('Bitte eine gültige E-Mail-Adresse eingeben');
  });

  it('zeigt den Bereich Unterlagen unter der Passung', async () => {
    const { el } = await render([], agentJob);
    const sections = [...el.querySelectorAll('[data-testid="fit"], app-application-documents')];
    expect(sections.map((s) => s.tagName.toLowerCase())).toEqual(['section', 'app-application-documents']);
    expect(el.querySelector('app-application-documents')?.textContent).toContain('Unterlagen');
  });

  it('übernimmt die geänderte Bewerbung aus dem Bereich Unterlagen', async () => {
    const { api, fixture, el } = await render([], agentJob);
    el.querySelector<HTMLButtonElement>('app-application-documents button.request')!.click();
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
    expect(api.requestDocuments).toHaveBeenCalledWith('a1');
    expect(el.querySelector('app-application-documents [role="status"]')?.textContent).toContain('Claude erstellt die Unterlagen beim nächsten Lauf.');
  });

  describe('Gmail-Thread', () => {
    afterEach(() => vi.restoreAllMocks());
    const threaded: Application = { ...application, gmail_thread_id: '18f2a_B-3' };

    it('zeigt ohne Thread keinen Hinweis', async () => {
      const { el } = await render([]);
      expect(el.querySelector('[data-testid="gmail-thread"]')).toBeNull();
    });

    it('zeigt den verknüpften Thread mit Link in Gmail', async () => {
      const { el } = await render([], threaded);
      const line = el.querySelector('[data-testid="gmail-thread"]') as HTMLElement;
      expect(line.textContent).toContain('Gmail-Thread verknüpft');
      const link = line.querySelector<HTMLAnchorElement>('a.thread-link')!;
      expect(link.getAttribute('href')).toBe('https://mail.google.com/mail/u/0/#all/18f2a_B-3');
      expect(link.getAttribute('target')).toBe('_blank');
      expect(link.getAttribute('rel')).toBe('noopener');
      expect(link.getAttribute('aria-label')).toContain('(öffnet in neuem Tab)');
    });

    it('löst die Verknüpfung nach Bestätigung und lädt neu', async () => {
      const confirm = vi.spyOn(window, 'confirm').mockReturnValue(true);
      const { api, fixture, el } = await render([], threaded);
      api.getApplication.mockReturnValue(of(application));
      el.querySelector<HTMLButtonElement>('button.unlink')!.click();
      fixture.detectChanges();
      await fixture.whenStable();
      fixture.detectChanges();
      expect(confirm).toHaveBeenCalledWith('Falls die Mail falsch zugeordnet wurde: Verknüpfung lösen?');
      expect(api.clearGmailThread).toHaveBeenCalledWith('a1');
      expect(api.getApplication).toHaveBeenCalledTimes(2);
      expect(el.querySelector('[data-testid="gmail-thread"]')).toBeNull();
    });

    it('lässt die Verknüpfung ohne Bestätigung bestehen', async () => {
      vi.spyOn(window, 'confirm').mockReturnValue(false);
      const { api, el } = await render([], threaded);
      el.querySelector<HTMLButtonElement>('button.unlink')!.click();
      expect(api.clearGmailThread).not.toHaveBeenCalled();
    });
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
