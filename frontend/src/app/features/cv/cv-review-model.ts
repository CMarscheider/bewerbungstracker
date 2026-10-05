import { Cv } from '../../api/models';

/** Abschnitte, die eine Optimierung einzeln ändern kann (Kontaktdaten bleiben unberührt). */
export type CvSection = 'headline' | 'summary' | 'projects' | 'experience' | 'education' | 'skills' | 'languages';

export const SECTION_LABELS: Record<CvSection, string> = {
  headline: 'Berufsbezeichnung',
  summary: 'Profil',
  projects: 'Projekte',
  experience: 'Berufserfahrung',
  education: 'Ausbildung',
  skills: 'Kenntnisse',
  languages: 'Sprachen',
};

export const SECTIONS = Object.keys(SECTION_LABELS) as CvSection[];

export function sectionValue(cv: Cv, section: CvSection): unknown {
  switch (section) {
    case 'headline':
      return cv.person.headline ?? '';
    case 'summary':
      return cv.summary ?? '';
    default:
      return cv[section];
  }
}

/** JSON mit sortierten Schlüsseln, damit die Reihenfolge (Go vs. Browser) keine Rolle spielt; leere Felder zählen wie fehlende. */
function stable(value: unknown): string {
  if (Array.isArray(value)) {
    return `[${value.map(stable).join(',')}]`;
  }
  if (value && typeof value === 'object') {
    const entries = Object.entries(value as Record<string, unknown>)
      .filter(([, v]) => v !== undefined && !(typeof v === 'string' && v.trim() === ''))
      .sort(([a], [b]) => a.localeCompare(b));
    return `{${entries.map(([k, v]) => `${JSON.stringify(k)}:${stable(v)}`).join(',')}}`;
  }
  return JSON.stringify(value);
}

export function changedSections(current: Cv, proposal: Cv): CvSection[] {
  return SECTIONS.filter((s) => stable(sectionValue(current, s)) !== stable(sectionValue(proposal, s)));
}

function month(p: string | undefined): string {
  if (!p) {
    return 'heute';
  }
  return /^\d{4}-\d{2}$/.test(p) ? `${p.slice(5)}/${p.slice(0, 4)}` : p;
}

/** Lesbare Zeilen eines Abschnitts für die Vorher/Nachher-Ansicht. */
export function sectionLines(cv: Cv, section: CvSection): string[] {
  switch (section) {
    case 'headline':
      return [cv.person.headline ?? ''].filter(Boolean);
    case 'summary':
      return [cv.summary ?? ''].filter(Boolean);
    case 'projects':
      return cv.projects.flatMap((p) => [
        `${p.name}${p.description ? `: ${p.description}` : ''}`,
        ...(p.url ? [`Link: ${p.url}`] : []),
        ...(p.technologies.length ? [`Technologien: ${p.technologies.join(', ')}`] : []),
      ]);
    case 'experience':
      return cv.experience.flatMap((e) => [
        `${month(e.start)} – ${month(e.end)}: ${e.role}, ${e.organization}${e.location ? ` (${e.location})` : ''}`,
        ...e.highlights.map((h) => `• ${h}`),
      ]);
    case 'education':
      return cv.education.flatMap((e) => [`${month(e.start)} – ${month(e.end)}: ${e.degree}, ${e.institution}`, ...(e.details ? [e.details] : [])]);
    case 'skills':
      return cv.skills.map((s) => `${s.category}: ${s.items.join(', ')}`);
    case 'languages':
      return cv.languages.map((l) => `${l.language}: ${l.level}`);
  }
}
