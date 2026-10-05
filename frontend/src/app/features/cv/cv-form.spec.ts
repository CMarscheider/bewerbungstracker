import { Cv } from '../../api/models';
import { applySection, cvForm, formToCv, fromLines, fromList } from './cv-form';

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

  it('übernimmt einen Abschnitt aus einem Vorschlag und markiert das Formular als geändert', () => {
    const form = cvForm(cv);
    const proposal = { ...cv, summary: 'Neues Profil', experience: [{ role: 'Entwickler', organization: 'Neu GmbH', start: '2025', highlights: ['A', 'B'] }] };
    applySection(form, 'experience', proposal);
    applySection(form, 'summary', proposal);
    const out = formToCv(form);
    expect(out.experience).toEqual(proposal.experience);
    expect(out.summary).toBe('Neues Profil');
    expect(out.education).toEqual(cv.education);
    expect(form.dirty).toBe(true);
  });

  it('akzeptiert nur http(s)-Links', () => {
    const form = cvForm({ ...cv, projects: [{ name: 'Tracker', technologies: [] }] });
    const link = form.controls.person.controls.links.at(0).controls.url;
    const project = form.controls.projects.at(0).controls.url;
    for (const bad of ['javascript:alert(1)', 'github.com/x']) {
      link.setValue(bad);
      project.setValue(bad);
      expect(link.invalid).toBe(true);
      expect(project.invalid).toBe(true);
    }
    link.setValue('https://github.com/x');
    project.setValue('https://github.com/x');
    expect(link.valid).toBe(true);
    expect(project.valid).toBe(true);
    project.setValue('');
    expect(project.valid).toBe(true);
  });
});
