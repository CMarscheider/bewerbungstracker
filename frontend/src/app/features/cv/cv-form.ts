import { FormArray, FormControl, FormGroup, Validators } from '@angular/forms';
import { Cv, CvEducation, CvExperience, CvLanguage, CvLink, CvProject, CvSkillGroup } from '../../api/models';

const PERIOD = /^\d{4}(-(0[1-9]|1[0-2]))?$/;

const text = (value = '') => new FormControl(value, { nonNullable: true });
const required = (value = '') => new FormControl(value, { nonNullable: true, validators: [Validators.required] });
const period = (value = '', mandatory = false) =>
  new FormControl(value, { nonNullable: true, validators: mandatory ? [Validators.required, Validators.pattern(PERIOD)] : [Validators.pattern(PERIOD)] });

/** Eine Zeile je Eintrag; leere Zeilen fallen weg. */
export function fromLines(value: string): string[] {
  return value
    .split(/\r?\n/)
    .map((s) => s.trim())
    .filter((s) => s !== '');
}

/** Kommagetrennte Liste; leere Einträge fallen weg. */
export function fromList(value: string): string[] {
  return value
    .split(',')
    .map((s) => s.trim())
    .filter((s) => s !== '');
}

/** Getrimmter Text oder undefined, damit leere optionale Felder im JSON fehlen. */
function opt(value: string): string | undefined {
  const t = value.trim();
  return t === '' ? undefined : t;
}

/** Entfernt Schlüssel mit undefined, damit das Ergebnis dem gespeicherten JSON entspricht. */
function compact<T extends object>(obj: T): T {
  return Object.fromEntries(Object.entries(obj).filter(([, v]) => v !== undefined)) as T;
}

export const linkGroup = (l?: CvLink) => new FormGroup({ label: required(l?.label), url: required(l?.url) });

export const experienceGroup = (e?: CvExperience) =>
  new FormGroup({
    role: required(e?.role),
    organization: required(e?.organization),
    location: text(e?.location),
    start: period(e?.start, true),
    end: period(e?.end),
    highlights: text(e?.highlights.join('\n')),
  });

export const educationGroup = (e?: CvEducation) =>
  new FormGroup({
    degree: required(e?.degree),
    institution: required(e?.institution),
    start: period(e?.start, true),
    end: period(e?.end),
    details: text(e?.details),
  });

export const skillGroup = (s?: CvSkillGroup) => new FormGroup({ category: required(s?.category), items: text(s?.items.join(', ')) });

export const projectGroup = (p?: CvProject) =>
  new FormGroup({
    name: required(p?.name),
    url: text(p?.url),
    description: text(p?.description),
    technologies: text(p?.technologies.join(', ')),
  });

export const languageGroup = (l?: CvLanguage) => new FormGroup({ language: required(l?.language), level: required(l?.level) });

export function cvForm(cv?: Cv) {
  return new FormGroup({
    person: new FormGroup({
      name: required(cv?.person.name),
      headline: text(cv?.person.headline),
      email: text(cv?.person.email),
      phone: text(cv?.person.phone),
      location: text(cv?.person.location),
      links: new FormArray((cv?.person.links ?? []).map(linkGroup)),
    }),
    summary: text(cv?.summary),
    experience: new FormArray((cv?.experience ?? []).map(experienceGroup)),
    education: new FormArray((cv?.education ?? []).map(educationGroup)),
    skills: new FormArray((cv?.skills ?? []).map(skillGroup)),
    projects: new FormArray((cv?.projects ?? []).map(projectGroup)),
    languages: new FormArray((cv?.languages ?? []).map(languageGroup)),
  });
}

export type CvForm = ReturnType<typeof cvForm>;

export function formToCv(form: CvForm): Cv {
  const v = form.getRawValue();
  return compact({
    person: compact({
      name: v.person.name.trim(),
      headline: opt(v.person.headline),
      email: opt(v.person.email),
      phone: opt(v.person.phone),
      location: opt(v.person.location),
      links: v.person.links.map((l) => ({ label: l.label.trim(), url: l.url.trim() })),
    }),
    summary: opt(v.summary),
    experience: v.experience.map((e) =>
      compact({
        role: e.role.trim(),
        organization: e.organization.trim(),
        location: opt(e.location),
        start: e.start.trim(),
        end: opt(e.end),
        highlights: fromLines(e.highlights),
      }),
    ),
    education: v.education.map((e) =>
      compact({ degree: e.degree.trim(), institution: e.institution.trim(), start: e.start.trim(), end: opt(e.end), details: opt(e.details) }),
    ),
    skills: v.skills.map((s) => ({ category: s.category.trim(), items: fromList(s.items) })),
    projects: v.projects.map((p) =>
      compact({ name: p.name.trim(), url: opt(p.url), description: opt(p.description), technologies: fromList(p.technologies) }),
    ),
    languages: v.languages.map((l) => ({ language: l.language.trim(), level: l.level.trim() })),
  });
}
