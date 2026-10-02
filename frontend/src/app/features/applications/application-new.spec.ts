import { TestBed } from '@angular/core/testing';
import { FormGroup } from '@angular/forms';
import { Router, provideRouter } from '@angular/router';
import { of } from 'rxjs';
import { Api } from '../../core/api';
import { toIsoDate } from '../../core/dates';
import { provideGermanDates } from '../../core/german-date-adapter';
import { ApplicationNew } from './application-new';

function setup() {
  const api = {
    listCompanies: vi.fn(() => of([{ id: 'c1', name: 'Acme', created_at: '', application_count: 0 }])),
    createCompany: vi.fn(() => of({ id: 'c2', name: 'Neu GmbH', created_at: '', application_count: 0 })),
    createApplication: vi.fn(() => of({ id: 'a9' })),
  };
  TestBed.configureTestingModule({
    imports: [ApplicationNew],
    providers: [provideRouter([]), provideGermanDates(), { provide: Api, useValue: api }],
  });
  const navigate = vi.spyOn(TestBed.inject(Router), 'navigate').mockResolvedValue(true);
  const fixture = TestBed.createComponent(ApplicationNew);
  fixture.detectChanges();
  // Zugriff auf geschützte Member nur im Test.
  const component = fixture.componentInstance as unknown as { form: FormGroup; submit(): void };
  const form = component.form;
  const submit = () => component.submit();
  return { api, navigate, form, submit };
}

describe('ApplicationNew', () => {
  it('nutzt eine vorhandene Firma (Groß-/Kleinschreibung egal)', () => {
    const { api, navigate, form, submit } = setup();
    form.patchValue({ company: ' acme ', position_title: 'Go-Entwickler' });
    submit();

    expect(api.createCompany).not.toHaveBeenCalled();
    expect(api.createApplication).toHaveBeenCalledWith(
      expect.objectContaining({ company_id: 'c1', position_title: 'Go-Entwickler', first_event: { type: 'Beworben', occurred_on: toIsoDate(new Date()) } }),
    );
    expect(navigate).toHaveBeenCalledWith(['/bewerbungen', 'a9']);
  });

  it('legt eine unbekannte Firma an und übernimmt die Frist nur bei Vorgemerkt', () => {
    const { api, form, submit } = setup();
    form.patchValue({
      company: 'Neu GmbH',
      position_title: 'Backend',
      first_type: 'Vorgemerkt',
      occurred_on: new Date(2026, 9, 1),
      due_on: new Date(2026, 9, 15),
    });
    submit();

    expect(api.createCompany).toHaveBeenCalledWith({ name: 'Neu GmbH' });
    expect(api.createApplication).toHaveBeenCalledWith(
      expect.objectContaining({ company_id: 'c2', first_event: { type: 'Vorgemerkt', occurred_on: '2026-10-01', due_on: '2026-10-15' } }),
    );
  });

  it('sendet nichts, solange Pflichtfelder fehlen', () => {
    const { api, submit } = setup();
    submit();
    expect(api.createApplication).not.toHaveBeenCalled();
  });
});
