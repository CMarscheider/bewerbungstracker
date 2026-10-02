import { TestBed } from '@angular/core/testing';
import { FormControl } from '@angular/forms';
import { of } from 'rxjs';
import { Company } from '../../api/models';
import { Api } from '../../core/api';
import { Companies } from './companies';

const list: Company[] = [
  { id: 'c1', name: 'Acme', application_count: 2, created_at: '2026-09-01T10:00:00+02:00' },
  { id: 'c2', name: 'Leer GmbH', application_count: 0, created_at: '2026-09-01T10:00:00+02:00' },
];

async function render() {
  const api = {
    listCompanies: vi.fn(() => of(list)),
    updateCompany: vi.fn(() => of(list[0])),
    deleteCompany: vi.fn(() => of(undefined)),
  };
  TestBed.configureTestingModule({ imports: [Companies], providers: [{ provide: Api, useValue: api }] });
  const fixture = TestBed.createComponent(Companies);
  fixture.detectChanges();
  await fixture.whenStable();
  fixture.detectChanges();
  return { api, fixture, el: fixture.nativeElement as HTMLElement };
}

describe('Companies', () => {
  it('zeigt Firmen mit Anzahl und sperrt Löschen bei Bewerbungen', async () => {
    const { el } = await render();
    const rows = [...el.querySelectorAll('.list li')];
    expect(rows.length).toBe(2);
    expect(rows[0].textContent).toContain('Acme');
    expect(rows[0].textContent).toContain('2 Bewerbung(en)');
    expect(rows[1].textContent).toContain('0 Bewerbung(en)');
    const del = rows.map((r) => r.querySelector<HTMLButtonElement>('button.danger')!);
    expect(del[0].disabled).toBe(true);
    expect(del[1].disabled).toBe(false);
  });

  it('benennt eine Firma um', async () => {
    const { api, fixture } = await render();
    const cmp = fixture.componentInstance as unknown as { name: FormControl<string>; startRename(c: Company): void; saveRename(c: Company): void };
    cmp.startRename(list[0]);
    cmp.name.setValue('Neu');
    cmp.saveRename(list[0]);
    expect(api.updateCompany).toHaveBeenCalledWith('c1', { name: 'Neu' });
  });
});
