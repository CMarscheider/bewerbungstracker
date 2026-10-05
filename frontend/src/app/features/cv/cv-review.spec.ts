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

async function render(review: CvReview | null, opts: { dirty?: boolean; savedAt?: string } = {}) {
  const api = {
    getCvReview: vi.fn(() => (review ? of(review) : throwError(() => ({ status: 404 })))),
    requestCvReview: vi.fn(() => of(ready({ state: 'angefordert', proposal: undefined, notes: [], completed_at: undefined }))),
    closeCvReview: vi.fn(() => of(undefined)),
  };
  TestBed.configureTestingModule({ imports: [CvReviewPanel], providers: [{ provide: Api, useValue: api }] });
  const snackBar = TestBed.inject(MatSnackBar);
  vi.spyOn(snackBar, 'open');
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
  return { api, form, settle, el: fixture.nativeElement as HTMLElement };
}

describe('CvReviewPanel', () => {
  afterEach(() => vi.restoreAllMocks());

  it('fordert eine Optimierung an', async () => {
    const { el, api, settle } = await render(null);
    el.querySelector<HTMLButtonElement>('button.request')!.click();
    await settle();
    expect(api.requestCvReview).toHaveBeenCalled();
    expect(el.textContent).toContain('Angefordert am');
  });

  it('sperrt die Anforderung bei ungespeicherten Änderungen oder ohne gespeicherten Lebenslauf', async () => {
    let r = await render(null, { dirty: true });
    expect(r.el.querySelector<HTMLButtonElement>('button.request')!.disabled).toBe(true);
    TestBed.resetTestingModule();
    r = await render(null, { savedAt: undefined });
    expect(r.el.querySelector<HTMLButtonElement>('button.request')!.disabled).toBe(true);
  });

  it('zeigt Hinweise und Vorher/Nachher und übernimmt einen Abschnitt', async () => {
    const { el, form, settle } = await render(ready());
    expect(el.textContent).toContain('Kennzahlen ergänzen');
    const change = el.querySelector('.change')!;
    expect(change.textContent).toContain('Profil');
    expect(change.textContent).toContain('Alt.');
    expect(change.textContent).toContain('Neu und geschärft.');
    change.querySelector<HTMLButtonElement>('button.apply')!.click();
    await settle();
    expect(formToCv(form).summary).toBe('Neu und geschärft.');
    expect(form.dirty).toBe(true);
    expect(change.querySelector<HTMLButtonElement>('button.apply')!.textContent).toContain('Übernommen');
  });

  it('warnt, wenn der Lebenslauf seit der Anforderung geändert wurde', async () => {
    const { el } = await render(ready({ based_on_updated_at: '2026-10-04T09:00:00+02:00' }));
    expect(el.querySelector('.stale')).not.toBeNull();
  });

  it('meldet, wenn es keine Änderungen gibt', async () => {
    const { el } = await render(ready({ proposal: saved }));
    expect(el.textContent).toContain('keine Änderungen');
  });

  it('schließt die Optimierung ab', async () => {
    const { el, api, settle } = await render(ready());
    el.querySelector<HTMLButtonElement>('button.close')!.click();
    await settle();
    expect(api.closeCvReview).toHaveBeenCalled();
    expect(el.querySelector('button.request')).not.toBeNull();
  });
});
