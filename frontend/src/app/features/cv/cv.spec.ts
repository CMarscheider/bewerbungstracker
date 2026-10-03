import { TestBed } from '@angular/core/testing';
import { of } from 'rxjs';
import { Cv as CvModel } from '../../api/models';
import { Api } from '../../core/api';
import { CvPage } from './cv';

const stored: CvModel = {
  person: { name: 'Erika Muster', links: [] },
  experience: [{ role: 'Werkstudentin', organization: 'Acme', start: '2024-03', highlights: ['Komponenten gebaut'] }],
  education: [],
  skills: [],
  projects: [],
  languages: [],
  updated_at: '2026-10-03T12:00:00+02:00',
};

async function render(cv: CvModel = stored) {
  const api = { getCv: vi.fn(() => of(cv)), saveCv: vi.fn((body: CvModel) => of({ ...body, updated_at: '2026-10-03T13:00:00+02:00' })) };
  TestBed.configureTestingModule({ imports: [CvPage], providers: [{ provide: Api, useValue: api }] });
  const fixture = TestBed.createComponent(CvPage);
  const settle = async () => {
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
  };
  await settle();
  return { api, settle, el: fixture.nativeElement as HTMLElement };
}

describe('CvPage', () => {
  afterEach(() => vi.restoreAllMocks());

  it('füllt das Formular mit dem gespeicherten Lebenslauf', async () => {
    const { el } = await render();
    expect(el.querySelector<HTMLInputElement>('input[name="person-name"]')!.value).toBe('Erika Muster');
    expect(el.querySelectorAll('[data-section="experience"] .entry').length).toBe(1);
  });

  it('fügt Einträge hinzu und entfernt sie', async () => {
    const { el, settle } = await render();
    el.querySelector<HTMLButtonElement>('[data-section="experience"] button.add')!.click();
    await settle();
    expect(el.querySelectorAll('[data-section="experience"] .entry').length).toBe(2);
    el.querySelector<HTMLButtonElement>('[data-section="experience"] .entry button.remove')!.click();
    await settle();
    expect(el.querySelectorAll('[data-section="experience"] .entry').length).toBe(1);
  });

  it('speichert den umgewandelten Lebenslauf', async () => {
    const { el, api, settle } = await render();
    el.querySelector<HTMLFormElement>('form')!.dispatchEvent(new Event('submit'));
    await settle();
    expect(api.saveCv).toHaveBeenCalledTimes(1);
    const body = api.saveCv.mock.calls[0][0];
    expect(body.person.name).toBe('Erika Muster');
    expect(body.experience[0].highlights).toEqual(['Komponenten gebaut']);
  });

  it('speichert nicht, solange Pflichtfelder fehlen', async () => {
    const { el, api, settle } = await render({ ...stored, person: { name: '', links: [] } });
    el.querySelector<HTMLFormElement>('form')!.dispatchEvent(new Event('submit'));
    await settle();
    expect(api.saveCv).not.toHaveBeenCalled();
  });
});
