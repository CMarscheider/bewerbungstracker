import { TestBed } from '@angular/core/testing';
import { MatSnackBar } from '@angular/material/snack-bar';
import { Observable, of, throwError } from 'rxjs';
import { Application, Documents, DocumentsState } from '../../api/models';
import { Api } from '../../core/api';
import { ApplicationDocuments } from './application-documents';

const base: Application = {
  id: 'a1',
  company_id: 'c1',
  company_name: 'Acme',
  position_title: 'Frontend-Entwickler',
  status: 'Vorgemerkt',
  phase: 'Vorbereitung',
  created_by_agent: true,
  created_at: '2026-10-01T10:00:00+02:00',
  updated_at: '2026-10-05T10:00:00+02:00',
  events: [],
  documents_state: 'keine',
  contact_email: 'jobs@acme.de',
  job_url: 'https://acme.de/jobs/1',
};

const docs: Documents = {
  version: 1,
  language: 'de',
  cover_letter: 'Sehr geehrte Damen und Herren,\nich bewerbe mich.',
  profile_line: 'Frontend-Entwickler mit Angular.',
  highlights: ['Angular', 'TypeScript'],
  mail_subject: 'Bewerbung als Frontend-Entwickler',
  mail_body: 'Hallo,\nanbei meine Unterlagen.',
  file_name: 'Bewerbung_Acme.pdf',
  rendered_at: '2026-10-05T11:00:00+02:00',
  updated_at: '2026-10-05T11:00:00+02:00',
};

const app = (state: DocumentsState, overrides: Partial<Application> = {}): Application => ({ ...base, documents_state: state, ...overrides });

interface ApiMock {
  requestDocuments: ReturnType<typeof vi.fn>;
  getDocuments: ReturnType<typeof vi.fn>;
  updateDocuments: ReturnType<typeof vi.fn>;
  createDraft: ReturnType<typeof vi.fn>;
  getApplication: ReturnType<typeof vi.fn>;
}

async function render(application: Application, overrides: Partial<ApiMock> = {}) {
  const api: ApiMock = {
    requestDocuments: vi.fn(() => of(app('angefordert'))),
    getDocuments: vi.fn(() => of(docs)),
    updateDocuments: vi.fn((_id: string, body: object) => of({ ...docs, ...body, version: 2 })),
    createDraft: vi.fn(() => of(app('entwurf_angelegt', { gmail_draft_at: '2026-10-05T12:00:00+02:00' }))),
    getApplication: vi.fn(() => of(application)),
    ...overrides,
  };
  TestBed.configureTestingModule({ imports: [ApplicationDocuments], providers: [{ provide: Api, useValue: api }] });
  const snack = vi.spyOn(TestBed.inject(MatSnackBar), 'open');
  const fixture = TestBed.createComponent(ApplicationDocuments);
  fixture.componentRef.setInput('application', application);
  const changed: Application[] = [];
  fixture.componentInstance.changed.subscribe((a) => {
    changed.push(a);
    // Wie die Detailseite: die neue Bewerbung wird wieder hineingereicht.
    fixture.componentRef.setInput('application', a);
  });
  const settle = async () => {
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
  };
  await settle();
  const el = fixture.nativeElement as HTMLElement;
  const button = (cls: string) => el.querySelector<HTMLButtonElement>(`button.${cls}`);
  const field = <T extends HTMLElement>(name: string) => el.querySelector<T>(`[formcontrolname="${name}"]`);
  const status = () => el.querySelector('[role="status"]')?.textContent ?? '';
  return { api, fixture, el, settle, button, field, status, changed, snack };
}

function type(input: HTMLInputElement | HTMLTextAreaElement, value: string): void {
  input.value = value;
  input.dispatchEvent(new Event('input'));
}

describe('ApplicationDocuments', () => {
  afterEach(() => vi.restoreAllMocks());

  it('keine: fordert Unterlagen an und meldet die geänderte Bewerbung', async () => {
    const { api, button, settle, changed, snack, status } = await render(app('keine'));
    expect(api.getDocuments).not.toHaveBeenCalled();
    button('request')!.click();
    await settle();
    expect(api.requestDocuments).toHaveBeenCalledWith('a1');
    expect(changed.map((a) => a.documents_state)).toEqual(['angefordert']);
    expect(snack).toHaveBeenCalledWith('Unterlagen angefordert', undefined, expect.anything());
    expect(status()).toContain('Claude erstellt die Unterlagen beim nächsten Lauf.');
  });

  it('angefordert: zeigt den Hinweis und aktualisiert den Status', async () => {
    const getApplication = vi.fn(() => of(app('erstellt')));
    const { api, button, settle, changed, status, el } = await render(app('angefordert'), { getApplication });
    expect(status()).toContain('Claude erstellt die Unterlagen beim nächsten Lauf.');
    expect(api.getDocuments).not.toHaveBeenCalled();
    button('refresh')!.click();
    await settle();
    expect(getApplication).toHaveBeenCalledWith('a1');
    expect(changed.map((a) => a.documents_state)).toEqual(['erstellt']);
    expect(api.getDocuments).toHaveBeenCalledWith('a1');
    expect(el.querySelector('a.pdf')).not.toBeNull();
  });

  for (const state of ['erstellt', 'entwurf_angelegt', 'portal'] as const) {
    it(`${state}: lädt die Unterlagen und zeigt PDF-Link und Formular`, async () => {
      const { api, el, field } = await render(app(state));
      expect(api.getDocuments).toHaveBeenCalledWith('a1');
      const pdf = el.querySelector<HTMLAnchorElement>('a.pdf')!;
      expect(pdf.textContent).toContain('PDF ansehen');
      expect(pdf.getAttribute('href')).toBe('/api/v1/applications/a1/documents/pdf');
      expect(pdf.getAttribute('target')).toBe('_blank');
      expect(pdf.getAttribute('rel')).toBe('noopener');
      expect(field<HTMLTextAreaElement>('cover_letter')!.tagName).toBe('TEXTAREA');
      expect(field<HTMLTextAreaElement>('cover_letter')!.value).toBe(docs.cover_letter);
      expect(field<HTMLInputElement>('profile_line')!.value).toBe(docs.profile_line);
      expect(field<HTMLInputElement>('mail_subject')!.value).toBe(docs.mail_subject);
      expect(field<HTMLTextAreaElement>('mail_body')!.tagName).toBe('TEXTAREA');
      expect(field('language')).not.toBeNull();
    });
  }

  it('speichert und rendert neu – Schwerpunkte unverändert', async () => {
    const { api, field, button, settle, snack } = await render(app('erstellt'));
    type(field<HTMLTextAreaElement>('cover_letter')!, '  Neues Anschreiben.  ');
    type(field<HTMLInputElement>('mail_subject')!, 'Neuer Betreff');
    await settle();
    button('save')!.click();
    await settle();
    expect(api.updateDocuments).toHaveBeenCalledWith('a1', {
      language: 'de',
      cover_letter: 'Neues Anschreiben.',
      profile_line: docs.profile_line,
      highlights: ['Angular', 'TypeScript'],
      mail_subject: 'Neuer Betreff',
      mail_body: docs.mail_body,
    });
    expect(snack).toHaveBeenCalledWith('Gespeichert und neu gerendert', undefined, expect.anything());
  });

  it('speichert nicht mit leerem Anschreiben', async () => {
    const { api, field, button, settle } = await render(app('erstellt'));
    type(field<HTMLTextAreaElement>('cover_letter')!, '   ');
    await settle();
    expect(button('save')!.disabled).toBe(true);
    expect(api.updateDocuments).not.toHaveBeenCalled();
  });

  it('entwurf_angelegt: zeigt den Gmail-Hinweis mit Link und Datum', async () => {
    const { el, status } = await render(app('entwurf_angelegt', { gmail_draft_at: '2026-10-05T12:30:00+02:00' }));
    expect(status()).toContain('Entwurf liegt in Gmail');
    const gmail = el.querySelector<HTMLAnchorElement>('a.gmail')!;
    expect(gmail.getAttribute('href')).toBe('https://mail.google.com/mail/u/0/#drafts');
    expect(gmail.getAttribute('target')).toBe('_blank');
    expect(gmail.getAttribute('rel')).toBe('noopener');
    expect(el.textContent).toContain('05.10.2026');
  });

  it('entwurf_angelegt: nach dem Speichern veraltet der Entwurf und lässt sich neu anlegen', async () => {
    const { api, el, button, settle, changed } = await render(app('entwurf_angelegt', { gmail_draft_at: '2026-10-05T12:00:00+02:00' }));
    expect(el.textContent).not.toContain('Der Gmail-Entwurf enthält noch die alte Fassung');
    expect(button('draft')).toBeNull();
    button('save')!.click();
    await settle();
    expect(el.textContent).toContain('Der Gmail-Entwurf enthält noch die alte Fassung');
    const draft = button('draft')!;
    expect(draft.textContent).toContain('Gmail-Entwurf neu anlegen');
    draft.click();
    await settle();
    expect(api.createDraft).toHaveBeenCalledWith('a1');
    expect(changed.at(-1)?.documents_state).toBe('entwurf_angelegt');
    expect(el.textContent).not.toContain('Der Gmail-Entwurf enthält noch die alte Fassung');
  });

  it('erstellt mit Bewerbungsadresse: legt einen Gmail-Entwurf an', async () => {
    const { api, button, settle, changed, snack } = await render(app('erstellt'));
    const draft = button('draft')!;
    expect(draft.textContent).toContain('Gmail-Entwurf anlegen');
    draft.click();
    await settle();
    expect(api.createDraft).toHaveBeenCalledWith('a1');
    expect(changed.map((a) => a.documents_state)).toEqual(['entwurf_angelegt']);
    expect(snack).toHaveBeenCalledWith('Gmail-Entwurf angelegt', undefined, expect.anything());
  });

  it('sperrt den Entwurf bei ungespeicherten Änderungen', async () => {
    const { el, field, button, settle } = await render(app('erstellt'));
    type(field<HTMLInputElement>('mail_subject')!, 'Geändert');
    await settle();
    expect(button('draft')!.disabled).toBe(true);
    expect(el.textContent).toContain('Erst speichern.');
  });

  it('erstellt ohne Bewerbungsadresse: kein Entwurfsknopf', async () => {
    const { button } = await render(app('erstellt', { contact_email: undefined }));
    expect(button('draft')).toBeNull();
  });

  it('portal: verweist auf das Portal', async () => {
    const { el, status } = await render(app('portal', { contact_email: undefined }));
    expect(status()).toContain('Keine Bewerbungsadresse – über das Portal bewerben');
    const link = el.querySelector<HTMLAnchorElement>('a.portal')!;
    expect(link.getAttribute('href')).toBe('https://acme.de/jobs/1');
    expect(link.getAttribute('target')).toBe('_blank');
    expect(link.getAttribute('rel')).toBe('noopener');
  });

  it('fehler: zeigt die Meldung als Alert und fordert erneut an', async () => {
    const confirm = vi.spyOn(window, 'confirm').mockReturnValue(true);
    const { api, el, button, settle, changed } = await render(app('fehler', { documents_error: 'PDF konnte nicht erzeugt werden: kaputt' }), {
      getDocuments: vi.fn(() => throwError(() => ({ status: 404 }))),
    });
    const alert = el.querySelector('[role="alert"]')!;
    expect(alert.textContent).toContain('PDF konnte nicht erzeugt werden: kaputt');
    button('request')!.click();
    await settle();
    expect(confirm).not.toHaveBeenCalled();
    expect(api.requestDocuments).toHaveBeenCalledWith('a1');
    expect(changed.map((a) => a.documents_state)).toEqual(['angefordert']);
  });

  it('zeigt documents_error auch bei erstellt (Entwurf gescheitert) als reinen Text', async () => {
    const evil = '<img src=x onerror=alert(1)>';
    const { el } = await render(app('erstellt', { documents_error: evil }));
    const alert = el.querySelector('[role="alert"]')!;
    expect(alert.textContent).toContain(evil);
    expect(el.querySelector('img')).toBeNull();
  });

  it('fragt vor dem erneuten Anfordern bei vorhandenen Unterlagen nach', async () => {
    const confirm = vi.spyOn(window, 'confirm').mockReturnValue(false);
    const { api, button, settle } = await render(app('erstellt'));
    button('request')!.click();
    await settle();
    expect(confirm).toHaveBeenCalledWith('Claude schreibt die Unterlagen neu. Fortfahren?');
    expect(api.requestDocuments).not.toHaveBeenCalled();
    confirm.mockReturnValue(true);
    button('request')!.click();
    await settle();
    expect(api.requestDocuments).toHaveBeenCalledWith('a1');
  });

  it('setzt busy nach einem Fehler zurück', async () => {
    const { button, settle, changed } = await render(app('keine'), {
      requestDocuments: vi.fn(() => throwError(() => ({ status: 500 }))),
    });
    button('request')!.click();
    await settle();
    expect(changed).toEqual([]);
    expect(button('request')!.disabled).toBe(false);
  });

  it('zeigt einen Ladefehler der Unterlagen mit erneutem Versuch', async () => {
    const getDocuments = vi.fn<() => Observable<Documents>>(() => throwError(() => ({ status: 500 })));
    const { el, button, settle } = await render(app('erstellt'), { getDocuments });
    expect(el.textContent).toContain('Die Unterlagen konnten nicht geladen werden.');
    getDocuments.mockReturnValue(of(docs));
    button('retry')!.click();
    await settle();
    expect(el.querySelector('a.pdf')).not.toBeNull();
  });
});
