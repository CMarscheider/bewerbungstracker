import { TestBed } from '@angular/core/testing';
import { FormGroup } from '@angular/forms';
import { Router, provideRouter } from '@angular/router';
import { of, throwError } from 'rxjs';
import { Api } from '../../core/api';
import { toIsoDate } from '../../core/dates';
import { provideGermanDates } from '../../core/german-date-adapter';
import { ApplicationNew } from './application-new';

function setup(createApplication: () => unknown = () => of({ id: 'a9' })) {
  const api = {
    listCompanies: vi.fn(() => of([{ id: 'c1', name: 'Acme', created_at: '', application_count: 0 }])),
    createCompany: vi.fn(() => of({ id: 'c2', name: 'Neu GmbH', created_at: '', application_count: 0 })),
    createApplication: vi.fn(createApplication),
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
  return { api, navigate, form, submit, fixture };
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

  it('sendet die Bewerbungs-E-Mail als contact_email, leer wird weggelassen', () => {
    const { api, form, submit } = setup();
    form.patchValue({ company: 'Acme', position_title: 'Go', contact_email: ' jobs@acme.de ' });
    submit();
    expect(api.createApplication).toHaveBeenLastCalledWith(expect.objectContaining({ contact_email: 'jobs@acme.de' }));

    form.patchValue({ contact_email: '' });
    submit();
    expect(api.createApplication).toHaveBeenLastCalledWith(expect.objectContaining({ contact_email: undefined }));
  });

  it('prüft das Format der Bewerbungs-E-Mail', () => {
    const { api, form, submit } = setup();
    form.patchValue({ company: 'Acme', position_title: 'Go', contact_email: 'keine-mail' });
    submit();
    expect(api.createApplication).not.toHaveBeenCalled();
    expect(form.get('contact_email')?.invalid).toBe(true);
  });

  it('zeigt den Formatfehler der Bewerbungs-E-Mail an', async () => {
    const { form, submit, fixture } = setup();
    form.patchValue({ contact_email: 'keine-mail' });
    submit();
    fixture.detectChanges();
    await fixture.whenStable();
    expect((fixture.nativeElement as HTMLElement).textContent).toContain('Bitte eine gültige E-Mail-Adresse eingeben');
  });

  it('sendet nichts, solange Pflichtfelder fehlen', () => {
    const { api, submit } = setup();
    submit();
    expect(api.createApplication).not.toHaveBeenCalled();
  });

  it('legt die Firma beim erneuten Absenden nach einem Fehler nicht noch einmal an', () => {
    let calls = 0;
    const { api, form, submit } = setup(() => (calls++ === 0 ? throwError(() => new Error('x')) : of({ id: 'a9' })));
    form.patchValue({ company: 'Neu GmbH', position_title: 'Backend' });
    submit();
    submit();

    expect(api.createCompany).toHaveBeenCalledTimes(1);
    expect(api.createApplication).toHaveBeenCalledTimes(2);
    expect(api.createApplication).toHaveBeenLastCalledWith(expect.objectContaining({ company_id: 'c2' }));
  });
});
