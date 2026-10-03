import { Cv } from '../../api/models';
import { cvForm, formToCv, fromLines, fromList } from './cv-form';

const cv: Cv = {
  person: { name: 'Erika Muster', email: 'erika@example.com', links: [{ label: 'GitHub', url: 'https://github.com/erika' }] },
  summary: 'Baut gern Oberflächen.',
  experience: [{ role: 'Werkstudentin', organization: 'Acme', start: '2024-03', highlights: ['Komponenten gebaut', 'Tests geschrieben'] }],
  education: [{ degree: 'B.Sc. Informatik', institution: 'FH Bielefeld', start: '2021', end: '2025' }],
  skills: [{ category: 'Frontend', items: ['Angular', 'TypeScript'] }],
  projects: [{ name: 'Tracker', technologies: ['Go', 'Angular'] }],
  languages: [{ language: 'Deutsch', level: 'Muttersprache' }],
  updated_at: '2026-10-03T12:00:00+02:00',
};

describe('cv-form', () => {
  it('fromLines trennt Zeilen und verwirft leere', () => {
    expect(fromLines(' a \n\n b\r\n')).toEqual(['a', 'b']);
  });

  it('fromList trennt an Kommas und verwirft leere', () => {
    expect(fromList('Go, Angular ,, ')).toEqual(['Go', 'Angular']);
  });

  it('übernimmt einen Lebenslauf verlustfrei hin und zurück', () => {
    const { updated_at: _ignored, ...expected } = cv;
    expect(formToCv(cvForm(cv))).toEqual(expected);
  });

  it('lässt leere optionale Felder weg', () => {
    const form = cvForm();
    form.controls.person.controls.name.setValue('  Erika  ');
    form.controls.person.controls.phone.setValue('   ');
    const out = formToCv(form);
    expect(out.person).toEqual({ name: 'Erika', links: [] });
    expect(out.summary).toBeUndefined();
  });

  it('meldet ungültige Zeiträume', () => {
    const form = cvForm(cv);
    form.controls.experience.at(0).controls.start.setValue('März 2024');
    expect(form.invalid).toBe(true);
  });

  it('lehnt Pflichtfelder aus Leerzeichen ab', () => {
    const form = cvForm();
    form.controls.person.controls.name.setValue('   ');
    expect(form.controls.person.controls.name.invalid).toBe(true);
  });

  it('toleriert Leerzeichen um Zeiträume', () => {
    const form = cvForm(cv);
    const start = form.controls.experience.at(0).controls.start;
    start.setValue(' 2024-03 ');
    expect(start.valid).toBe(true);
    expect(formToCv(form).experience[0].start).toBe('2024-03');
  });
});
