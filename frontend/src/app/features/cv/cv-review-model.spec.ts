import { Cv } from '../../api/models';
import { changedSections, sectionLines } from './cv-review-model';

const base: Cv = {
  person: { name: 'Erika', headline: 'Entwicklerin', links: [] },
  summary: 'Alt.',
  experience: [{ role: 'Agentin', organization: 'Acme', start: '2020-09', highlights: ['Kundenservice'] }],
  education: [{ degree: 'Ausbildung', institution: 'JET', start: '2012-08', end: '2014-07' }],
  skills: [{ category: 'Frontend', items: ['Angular', 'TypeScript'] }],
  projects: [{ name: 'Join', description: 'Kanban', technologies: ['JS'] }],
  languages: [{ language: 'Englisch', level: 'gut' }],
};

describe('cv-review-model', () => {
  it('meldet nur geänderte Abschnitte', () => {
    const proposal: Cv = { ...base, summary: 'Neu.', skills: [{ category: 'Frontend', items: ['Angular', 'TypeScript', 'RxJS'] }] };
    expect(changedSections(base, proposal)).toEqual(['summary', 'skills']);
  });

  it('ignoriert die Reihenfolge der Schlüssel', () => {
    const reordered = JSON.parse(
      '{"languages":[{"level":"gut","language":"Englisch"}],"projects":[{"technologies":["JS"],"name":"Join","description":"Kanban"}],' +
        '"skills":[{"items":["Angular","TypeScript"],"category":"Frontend"}],"education":[{"end":"2014-07","start":"2012-08","institution":"JET","degree":"Ausbildung"}],' +
        '"experience":[{"highlights":["Kundenservice"],"start":"2020-09","organization":"Acme","role":"Agentin"}],"summary":"Alt.","person":{"links":[],"headline":"Entwicklerin","name":"Erika"}}',
    ) as Cv;
    expect(changedSections(base, reordered)).toEqual([]);
  });

  it('erkennt eine geänderte Berufsbezeichnung', () => {
    expect(changedSections(base, { ...base, person: { ...base.person, headline: 'Junior Frontend-Entwicklerin' } })).toEqual(['headline']);
  });

  it('formatiert Abschnitte als lesbare Zeilen', () => {
    expect(sectionLines(base, 'experience')).toEqual(['09/2020 – heute: Agentin, Acme', '• Kundenservice']);
    expect(sectionLines(base, 'education')).toEqual(['08/2012 – 07/2014: Ausbildung, JET']);
    expect(sectionLines(base, 'skills')).toEqual(['Frontend: Angular, TypeScript']);
    expect(sectionLines(base, 'projects')).toEqual(['Join: Kanban', 'Technologien: JS']);
    expect(sectionLines(base, 'languages')).toEqual(['Englisch: gut']);
    expect(sectionLines(base, 'summary')).toEqual(['Alt.']);
    expect(sectionLines(base, 'headline')).toEqual(['Entwicklerin']);
  });
});
