import { TestBed } from '@angular/core/testing';
import { of, throwError } from 'rxjs';
import { Company } from '../../api/models';
import { Api } from '../../core/api';
import { Companies } from './companies';

const list: Company[] = [
  { id: 'c1', name: 'Acme', application_count: 2, created_at: '2026-09-01T10:00:00+02:00' },
  { id: 'c2', name: 'Leer GmbH', application_count: 0, created_at: '2026-09-01T10:00:00+02:00' },
];

async function render(listCompanies = vi.fn(() => of(list))) {
  const api = {
    listCompanies,
    updateCompany: vi.fn(() => of(list[0])),
    deleteCompany: vi.fn(() => of(undefined)),
  };
  TestBed.configureTestingModule({ imports: [Companies], providers: [{ provide: Api, useValue: api }] });
  const fixture = TestBed.createComponent(Companies);
  const settle = async () => {
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
  };
  await settle();
  return { api, fixture, settle, el: fixture.nativeElement as HTMLElement };
}

describe('Companies', () => {
  afterEach(() => vi.restoreAllMocks());

  it('zeigt Firmen mit Anzahl und sperrt Löschen bei Bewerbungen', async () => {
    const { el } = await render();
    const rows = [...el.querySelectorAll('.list li')];
    expect(rows.length).toBe(2);
    expect(rows[0].textContent).toContain('Acme');
    expect(rows[0].textContent).toContain('2 Bewerbung(en)');
    const del = rows.map((r) => r.querySelector<HTMLButtonElement>('button.danger')!);
    expect(del[0].getAttribute('aria-disabled')).toBe('true');
    expect(del[1].getAttribute('aria-disabled')).not.toBe('true');
  });

  it('benennt eine Firma über das Formular um', async () => {
    const { api, el, settle } = await render();
    el.querySelector<HTMLButtonElement>('.list li button')!.click();
    await settle();
    const input = el.querySelector<HTMLInputElement>('form input')!;
    input.value = 'Neu';
    input.dispatchEvent(new Event('input'));
    el.querySelector<HTMLButtonElement>('form button[type="submit"]')!.click();
    await settle();
    expect(api.updateCompany).toHaveBeenCalledWith('c1', { name: 'Neu' });
  });

  it('löscht eine Firma ohne Bewerbungen und lädt neu', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    const { api, el, settle } = await render();
    const del = el.querySelectorAll<HTMLButtonElement>('button.danger')[1];
    del.click();
    await settle();
    expect(api.deleteCompany).toHaveBeenCalledWith('c2');
    expect(api.listCompanies).toHaveBeenCalledTimes(2);
  });

  it('löscht nicht, wenn Bewerbungen existieren', async () => {
    const confirm = vi.spyOn(window, 'confirm').mockReturnValue(true);
    const { api, el, settle } = await render();
    el.querySelectorAll<HTMLButtonElement>('button.danger')[0].click();
    await settle();
    expect(confirm).not.toHaveBeenCalled();
    expect(api.deleteCompany).not.toHaveBeenCalled();
  });

  it('zeigt einen Fehlertext statt des Leerzustands', async () => {
    const { el } = await render(vi.fn(() => throwError(() => new Error('x'))));
    expect(el.textContent).toContain('Konnte nicht geladen werden.');
    expect(el.textContent).not.toContain('Noch keine Firmen');
  });
});
