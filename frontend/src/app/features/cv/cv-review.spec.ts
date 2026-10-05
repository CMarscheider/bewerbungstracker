import { TestBed } from '@angular/core/testing';
import { MatSnackBar } from '@angular/material/snack-bar';
import { of, throwError } from 'rxjs';
import { Cv, CvReview } from '../../api/models';
import { Api } from '../../core/api';
import { cvForm, formToCv } from './cv-form';
import { CvReviewPanel } from './cv-review';

const saved: Cv = {
  person: { name: 'Erika', headline: 'Entwicklerin', links: [] },
  summary: 'Alt.',
  experience: [],
  education: [],
  skills: [],
  projects: [],
  languages: [],
};
const savedAt = '2026-10-05T10:00:00+02:00';

function ready(overrides: Partial<CvReview> = {}): CvReview {
  return {
    id: 'r1',
    state: 'fertig',
    based_on_updated_at: savedAt,
    requested_at: '2026-10-05T10:01:00+02:00',
    completed_at: '2026-10-05T11:00:00+02:00',
    notes: ['Kennzahlen ergänzen'],
    proposal: { ...saved, summary: 'Neu und geschärft.' },
    ...overrides,
  };
}

const pending = (): CvReview => ready({ state: 'angefordert', proposal: undefined, notes: [], completed_at: undefined });

interface ApiMock {
  getCvReview: ReturnType<typeof vi.fn>;
  requestCvReview: ReturnType<typeof vi.fn>;
  closeCvReview: ReturnType<typeof vi.fn>;
}

async function render(review: CvReview | null, opts: { dirty?: boolean; savedAt?: string; api?: Partial<ApiMock> } = {}) {
  const api: ApiMock = {
    getCvReview: vi.fn(() => (review ? of(review) : throwError(() => ({ status: 404 })))),
    requestCvReview: vi.fn(() => of(pending())),
    closeCvReview: vi.fn(() => of(undefined)),
    ...opts.api,
  };
  TestBed.configureTestingModule({ imports: [CvReviewPanel], providers: [{ provide: Api, useValue: api }] });
  const snackBar = TestBed.inject(MatSnackBar);
  const snack = vi.spyOn(snackBar, 'open');
  const fixture = TestBed.createComponent(CvReviewPanel);
  const form = cvForm(saved);
  fixture.componentRef.setInput('form', form);
  fixture.componentRef.setInput('savedAt', 'savedAt' in opts ? opts.savedAt : savedAt);
  fixture.componentRef.setInput('dirty', opts.dirty ?? false);
  const settle = async () => {
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
  };
  await settle();
  const el = fixture.nativeElement as HTMLElement;
  const button = (cls: string) => el.querySelector<HTMLButtonElement>(`button.${cls}`);
  return { api, form, settle, el, button, snack, fixture };
}

describe('CvReviewPanel', () => {
  afterEach(() => vi.restoreAllMocks());

  it('fordert eine Optimierung an', async () => {
    const { el, api, settle, button } = await render(null);
    expect(button('request')!.disabled).toBe(false);
    button('request')!.click();
    await settle();
    expect(api.requestCvReview).toHaveBeenCalled();
    expect(el.querySelector('[role="status"]')!.textContent).toContain('Angefordert am');
  });

  it('sperrt die Anforderung bei ungespeicherten Änderungen oder ohne gespeicherten Lebenslauf', async () => {
    let r = await render(null, { dirty: true });
    expect(r.button('request')!.disabled).toBe(true);
    TestBed.resetTestingModule();
    r = await render(null, { savedAt: undefined });
    expect(r.button('request')!.disabled).toBe(true);
  });

  it('setzt busy nach einem Fehler zurück und lädt den Stand neu', async () => {
    const { api, settle, button } = await render(null, {
      api: { requestCvReview: vi.fn(() => throwError(() => ({ status: 500 }))) },
    });
    button('request')!.click();
    await settle();
    expect(api.getCvReview).toHaveBeenCalledTimes(2);
    expect(button('request')!.disabled).toBe(false);
  });

  it('zeigt bei einem Ladefehler eine Meldung mit erneutem Versuch', async () => {
    const getCvReview = vi.fn(() => throwError(() => ({ status: 500 })));
    const { el, settle, button } = await render(null, { api: { getCvReview } });
    expect(el.textContent).toContain('konnte nicht geladen werden');
    expect(button('request')).toBeNull();
    getCvReview.mockReturnValue(throwError(() => ({ status: 404 })));
    button('retry')!.click();
    await settle();
    expect(button('request')).not.toBeNull();
  });

  it('aktualisiert den Status einer angeforderten Optimierung', async () => {
    const getCvReview = vi.fn(() => of(pending()));
    const { el, settle, button } = await render(null, { api: { getCvReview } });
    getCvReview.mockReturnValue(of(ready()));
    button('refresh')!.click();
    await settle();
    expect(getCvReview).toHaveBeenCalledTimes(2);
    expect(el.querySelector('.change')).not.toBeNull();
  });

  it('zieht eine Anfrage zurück', async () => {
    const { el, api, settle, button, snack } = await render(pending());
    button('withdraw')!.click();
    await settle();
    expect(api.closeCvReview).toHaveBeenCalled();
    expect(snack).toHaveBeenCalledWith('Anfrage zurückgezogen', undefined, expect.anything());
    expect(button('request')).not.toBeNull();
    expect(document.activeElement).toBe(el.querySelector('h2'));
  });

  it('zeigt Hinweise und Vorher/Nachher und übernimmt einen Abschnitt', async () => {
    const { el, form, settle } = await render(ready());
    expect(el.textContent).toContain('Kennzahlen ergänzen');
    const change = el.querySelector('.change')!;
    expect(change.textContent).toContain('Profil');
    expect(change.textContent).toContain('Alt.');
    expect(change.textContent).toContain('Neu und geschärft.');
    const apply = change.querySelector<HTMLButtonElement>('button.apply')!;
    expect(apply.getAttribute('aria-label')).toBe('Profil übernehmen');
    apply.click();
    await settle();
    expect(formToCv(form).summary).toBe('Neu und geschärft.');
    expect(form.dirty).toBe(true);
    expect(apply.textContent).toContain('Übernommen');
    expect(document.activeElement).toBe(change.querySelector('h3'));
  });

  it('übernimmt alle Abschnitte mit einer Meldung', async () => {
    const { form, settle, button, snack } = await render(
      ready({ proposal: { ...saved, summary: 'Neu.', person: { ...saved.person, headline: 'Frontend-Entwicklerin' } } }),
    );
    button('apply-all')!.click();
    await settle();
    expect(formToCv(form).summary).toBe('Neu.');
    expect(formToCv(form).person.headline).toBe('Frontend-Entwicklerin');
    expect(snack).toHaveBeenCalledTimes(1);
    expect(snack).toHaveBeenCalledWith('Alle Vorschläge übernommen – zum Speichern unten auf „Speichern“ klicken', undefined, expect.anything());
    expect(button('apply-all')!.disabled).toBe(true);
  });

  it('warnt, wenn der Lebenslauf seit der Anforderung geändert wurde', async () => {
    const { el } = await render(ready({ based_on_updated_at: '2026-10-04T09:00:00+02:00' }));
    expect(el.querySelector('.stale')).not.toBeNull();
  });

  it('warnt nicht, wenn erst nach dem Übernehmen gespeichert wurde', async () => {
    const { el, settle, fixture } = await render(ready());
    el.querySelector<HTMLButtonElement>('button.apply')!.click();
    await settle();
    fixture.componentRef.setInput('savedAt', '2026-10-05T12:00:00+02:00');
    await settle();
    expect(el.querySelector('.stale')).toBeNull();
  });

  it('weist auf ungespeicherte Änderungen hin', async () => {
    const { el } = await render(ready(), { dirty: true });
    expect(el.querySelector('.dirty-hint')!.textContent).toContain('Ungespeicherte Änderungen werden beim Übernehmen überschrieben.');
  });

  it('meldet, wenn es keine Änderungen gibt', async () => {
    const { el } = await render(ready({ proposal: saved }));
    expect(el.querySelector('[role="status"]')!.textContent).toContain('keine Änderungen');
  });

  it('fragt vor dem Verwerfen nach und bricht ab, wenn abgelehnt', async () => {
    const confirm = vi.spyOn(window, 'confirm').mockReturnValue(false);
    const { api, settle, button } = await render(ready());
    expect(button('close')!.textContent).toContain('Vorschläge verwerfen');
    button('close')!.click();
    await settle();
    expect(confirm).toHaveBeenCalled();
    expect(api.closeCvReview).not.toHaveBeenCalled();
  });

  it('verwirft die Vorschläge nach Bestätigung', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    const { api, settle, button } = await render(ready());
    button('close')!.click();
    await settle();
    expect(api.closeCvReview).toHaveBeenCalled();
    expect(button('request')).not.toBeNull();
  });

  it('schließt die Optimierung ab', async () => {
    const confirm = vi.spyOn(window, 'confirm');
    const { el, api, settle, button, snack } = await render(ready());
    button('apply')!.click();
    await settle();
    expect(button('close')!.textContent).toContain('Abschließen');
    button('close')!.click();
    await settle();
    expect(confirm).not.toHaveBeenCalled();
    expect(api.closeCvReview).toHaveBeenCalled();
    expect(snack).toHaveBeenCalledWith('Optimierung abgeschlossen', undefined, expect.anything());
    expect(button('request')).not.toBeNull();
    expect(document.activeElement).toBe(el.querySelector('h2'));
  });
});
