import { TestBed } from '@angular/core/testing';
import { MatSnackBar } from '@angular/material/snack-bar';
import { provideRouter } from '@angular/router';
import { Subject, of, throwError } from 'rxjs';
import { Application, ApplicationSummary, Suggestion } from '../../api/models';
import { Api } from '../../core/api';
import { AgentSuggestions } from './agent-suggestions';

const assigned: Suggestion = {
  id: 's1',
  application_id: 'a1',
  company_name: 'Acme',
  position_title: 'Frontend-Entwickler',
  suggested_type: 'Interview',
  occurred_on: '2026-10-04',
  due_on: '2026-10-12',
  reason: '<b>Einladung</b> zum Gespräch',
  mail_subject: 'Einladung <script>x</script>',
  mail_from: 'HR <hr@acme.de>',
  mail_url: 'https://mail.google.com/mail/u/0/#inbox/abc',
  state: 'offen',
  created_at: '2026-10-05T08:00:00+02:00',
};

const second: Suggestion = {
  id: 's2',
  application_id: 'a2',
  company_name: 'Beta',
  position_title: 'Angular-Entwickler',
  suggested_type: 'Absage',
  occurred_on: '2026-10-03',
  reason: 'Absage per Mail',
  state: 'offen',
  created_at: '2026-10-05T09:00:00+02:00',
};

const unassigned: Suggestion = {
  id: 's3',
  suggested_type: 'Absage',
  occurred_on: '2026-10-02',
  reason: 'Firma unklar',
  mail_subject: 'Ihre Bewerbung',
  state: 'offen',
  created_at: '2026-10-05T10:00:00+02:00',
};

const summary = (id: string, company: string, phase: ApplicationSummary['phase'], status: ApplicationSummary['status'] = 'Beworben'): ApplicationSummary => ({
  id,
  company_id: 'c-' + id,
  company_name: company,
  position_title: 'Dev',
  status,
  phase,
  created_by_agent: false,
  last_event_on: '2026-10-01',
  updated_at: '2026-10-01T10:00:00+02:00',
});

async function render(suggestions: Suggestion[], overrides: Record<string, ReturnType<typeof vi.fn>> = {}) {
  const api = {
    listSuggestions: vi.fn(() => of(suggestions)),
    acceptSuggestion: vi.fn(() => of({ id: 'a1' } as Application)),
    dismissSuggestion: vi.fn(() => of(undefined)),
    listApplications: vi.fn(() =>
      of([
        summary('a1', 'Acme', 'Aktiv'),
        summary('a9', 'Alt GmbH', 'Abgeschlossen', 'Absage'),
        summary('a5', 'Gamma', 'Vorbereitung'),
        summary('a7', 'Still AG', 'Abgeschlossen', 'KeineRueckmeldung'),
      ]),
    ),
    ...overrides,
  };
  TestBed.configureTestingModule({ imports: [AgentSuggestions], providers: [provideRouter([]), { provide: Api, useValue: api }] });
  const snack = vi.spyOn(TestBed.inject(MatSnackBar), 'open');
  const fixture = TestBed.createComponent(AgentSuggestions);
  document.body.appendChild(fixture.nativeElement);
  const settle = async () => {
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
  };
  await settle();
  const el = fixture.nativeElement as HTMLElement;
  const items = () => [...el.querySelectorAll<HTMLElement>('li.suggestion')];
  const button = (item: HTMLElement, cls: string) => item.querySelector<HTMLButtonElement>(`button.${cls}`)!;
  const status = () => el.querySelector('[role="status"]')?.textContent?.trim() ?? '';
  /** Die Meldung erscheint kurz verzögert (siehe announce). */
  const announced = (text: string) =>
    vi.waitFor(async () => {
      await settle();
      expect(status()).toBe(text);
    });
  return { api, fixture, el, settle, items, button, status, announced, snack };
}

describe('AgentSuggestions', () => {
  afterEach(() => {
    vi.restoreAllMocks();
    document.body.innerHTML = '';
  });

  it('zeigt ohne offene Vorschläge keine Karte', async () => {
    const { el, api } = await render([]);
    expect(el.querySelector('mat-card')).toBeNull();
    expect(api.listApplications).not.toHaveBeenCalled();
  });

  it('zeigt keine Karte, wenn die Vorschläge nicht laden', async () => {
    const { el } = await render([], { listSuggestions: vi.fn(() => throwError(() => new Error('x'))) });
    expect(el.querySelector('mat-card')).toBeNull();
  });

  it('rendert einen zugeordneten Vorschlag mit allen Angaben – Texte nur als Text', async () => {
    const { el, items } = await render([assigned]);
    expect(el.querySelector('.card-title')?.textContent).toContain('Vorschläge des Agenten');
    expect(el.querySelector('.card-title')?.textContent).toContain('1');
    const [item] = items();
    const link = item.querySelector<HTMLAnchorElement>('a.what')!;
    expect(link.getAttribute('href')).toBe('/bewerbungen/a1');
    expect(link.textContent).toContain('Acme');
    expect(link.textContent).toContain('Frontend-Entwickler');
    expect(item.querySelector('app-status-badge')?.textContent).toContain('Interview');
    expect(item.textContent).toContain('04.10.2026');
    expect(item.textContent).toContain('Frist 12.10.2026');
    expect(item.querySelector('.reason')?.textContent).toContain('<b>Einladung</b> zum Gespräch');
    expect(item.querySelector('.reason b')).toBeNull();
    expect(item.querySelector('.subject')?.textContent).toContain('Einladung <script>x</script>');
    expect(item.querySelector('script')).toBeNull();
    expect(item.querySelector('.from')?.textContent).toContain('HR <hr@acme.de>');
    const mail = item.querySelector<HTMLAnchorElement>('a.mail')!;
    expect(mail.textContent).toContain('Mail öffnen');
    expect(mail.getAttribute('href')).toBe(assigned.mail_url);
    expect(mail.getAttribute('target')).toBe('_blank');
    expect(mail.getAttribute('rel')).toBe('noopener');
    expect(mail.getAttribute('aria-label')).toContain('(öffnet in neuem Tab)');
  });

  it('zeigt den Mail-Link nur mit mail_url und keine Frist ohne due_on', async () => {
    const { items } = await render([second]);
    const [item] = items();
    expect(item.querySelector('a.mail')).toBeNull();
    expect(item.textContent).not.toContain('Frist');
  });

  it('Übernehmen ruft acceptSuggestion, entfernt den Eintrag und fokussiert den nächsten', async () => {
    const { api, items, button, settle, snack, announced, el } = await render([assigned, second]);
    // Die Knöpfe nennen ihre Zeile.
    for (const cls of ['accept', 'dismiss']) {
      const describedBy = button(items()[0], cls).getAttribute('aria-describedby')!;
      expect(document.getElementById(describedBy)?.textContent).toContain('Acme');
    }
    button(items()[0], 'accept').click();
    await settle();
    expect(api.acceptSuggestion).toHaveBeenCalledWith('s1', {});
    expect(items().map((i) => i.dataset['id'])).toEqual(['s2']);
    expect(snack).toHaveBeenCalledWith('Übernommen', undefined, expect.anything());
    await announced('Übernommen: Acme.');
    expect(document.activeElement).toBe(items()[0]);
    expect(el.querySelector('.card-title')?.textContent).toContain('1');
  });

  it('Fehler beim Übernehmen: Eintrag bleibt und ist wieder bedienbar', async () => {
    const acceptSuggestion = vi.fn(() => throwError(() => new Error('422')));
    const { items, button, settle, snack } = await render([assigned], { acceptSuggestion });
    button(items()[0], 'accept').click();
    await settle();
    expect(items().length).toBe(1);
    expect(button(items()[0], 'accept').disabled).toBe(false);
    // Die Fehlermeldung zeigt der Interceptor.
    expect(snack).not.toHaveBeenCalledWith('Übernommen', undefined, expect.anything());
  });

  it('Verwerfen ruft dismissSuggestion, entfernt den Eintrag und fokussiert die Überschrift, wenn keiner bleibt', async () => {
    const { api, items, button, settle, el, announced } = await render([assigned]);
    button(items()[0], 'dismiss').click();
    await settle();
    expect(api.dismissSuggestion).toHaveBeenCalledWith('s1');
    expect(items().length).toBe(0);
    await announced('Verworfen: Acme.');
    expect(el.textContent).toContain('Keine offenen Vorschläge mehr.');
    expect(document.activeElement).toBe(el.querySelector('h2'));
  });

  it('meldet jede Entscheidung neu, auch bei gleichem Text', async () => {
    const twin: Suggestion = { ...assigned, id: 's9' };
    const { items, button, settle, status, announced } = await render([assigned, twin]);
    button(items()[0], 'dismiss').click();
    await announced('Verworfen: Acme.');
    // Zwischen zwei Meldungen wird die Live-Region kurz geleert, damit Screenreader erneut ansagen.
    button(items()[0], 'dismiss').click();
    await settle();
    expect(status()).toBe('');
    await announced('Verworfen: Acme.');
  });

  it('nicht zugeordnet: Auswahl der offenen Bewerbungen, Übernehmen erst nach Auswahl', async () => {
    const { api, items, button, settle, announced } = await render([unassigned]);
    const [item] = items();
    expect(item.textContent).toContain('Nicht zugeordnet');
    const describedBy = button(item, 'accept').getAttribute('aria-describedby')!;
    expect(document.getElementById(describedBy)?.textContent).toContain('Nicht zugeordnet');
    expect(item.querySelector('a.what')).toBeNull();
    expect(api.listApplications).toHaveBeenCalledWith();
    const select = item.querySelector<HTMLSelectElement>('select')!;
    const options = [...select.options].map((o) => o.textContent?.trim());
    expect(options).toContain('Acme – Dev');
    expect(options).toContain('Gamma – Dev');
    expect(options).not.toContain('Alt GmbH – Dev');
    // Späte Antwort nach „Keine Rückmeldung“ muss zuordenbar bleiben.
    expect(options).toContain('Still AG – Dev');
    expect(button(item, 'accept').disabled).toBe(true);
    select.value = 'a5';
    select.dispatchEvent(new Event('change'));
    await settle();
    expect(button(item, 'accept').disabled).toBe(false);
    button(item, 'accept').click();
    await settle();
    expect(api.acceptSuggestion).toHaveBeenCalledWith('s3', { application_id: 'a5' });
    expect(items().length).toBe(0);
    await announced('Übernommen: Gamma.');
  });

  it('Busy-Guard je Eintrag: kein Doppelklick, andere Einträge bleiben bedienbar', async () => {
    const pending = new Subject<Application>();
    const acceptSuggestion = vi.fn(() => pending);
    const { items, button, settle, api } = await render([assigned, second], { acceptSuggestion });
    button(items()[0], 'accept').click();
    await settle();
    button(items()[0], 'accept').click();
    expect(acceptSuggestion).toHaveBeenCalledTimes(1);
    expect(button(items()[0], 'accept').disabled).toBe(true);
    expect(button(items()[0], 'dismiss').disabled).toBe(true);
    expect(button(items()[1], 'dismiss').disabled).toBe(false);
    button(items()[1], 'dismiss').click();
    await settle();
    expect(api.dismissSuggestion).toHaveBeenCalledWith('s2');
    // Die späte Antwort entfernt genau ihren Eintrag.
    pending.next({ id: 'a1' } as Application);
    pending.complete();
    await settle();
    expect(items().length).toBe(0);
  });
});
