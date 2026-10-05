import { TestBed } from '@angular/core/testing';
import { MatSnackBar } from '@angular/material/snack-bar';
import { provideRouter } from '@angular/router';
import { Subject, of, throwError } from 'rxjs';
import { ProcessedMail } from '../../api/models';
import { Api } from '../../core/api';
import { ProcessedMails } from './processed-mails';

const mails: ProcessedMail[] = [
  {
    gmail_message_id: 'm1',
    application_id: 'a1',
    company_name: 'Acme <b>AG</b>',
    position_title: 'Frontend-Entwickler',
    outcome: 'Ereignis <i>Absage</i> angelegt',
    processed_at: '2026-10-05T08:15:00+02:00',
  },
  { gmail_message_id: 'm2', outcome: 'Nicht relevant', processed_at: '2026-10-04T18:00:00+02:00' },
];

async function render(overrides: Record<string, ReturnType<typeof vi.fn>> = {}) {
  const api = {
    listProcessedMails: vi.fn(() => of(mails)),
    deleteProcessedMail: vi.fn(() => of(undefined)),
    ...overrides,
  };
  TestBed.configureTestingModule({ imports: [ProcessedMails], providers: [provideRouter([]), { provide: Api, useValue: api }] });
  const snack = vi.spyOn(TestBed.inject(MatSnackBar), 'open');
  const fixture = TestBed.createComponent(ProcessedMails);
  document.body.appendChild(fixture.nativeElement);
  const settle = async () => {
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
  };
  await settle();
  const el = fixture.nativeElement as HTMLElement;
  const open = async () => {
    el.querySelector<HTMLElement>('mat-expansion-panel-header')!.click();
    await settle();
  };
  const rows = () => [...el.querySelectorAll<HTMLElement>('li.mail')];
  return { api, fixture, el, settle, open, rows, snack };
}

describe('ProcessedMails', () => {
  afterEach(() => {
    vi.restoreAllMocks();
    document.body.innerHTML = '';
  });

  it('ist zugeklappt und lädt erst beim Öffnen', async () => {
    const { api, el, open } = await render();
    expect(el.querySelector('mat-expansion-panel-header')?.textContent).toContain('Vom Agenten verarbeitete Mails');
    expect(el.querySelector('mat-expansion-panel-header')?.getAttribute('aria-expanded')).toBe('false');
    expect(api.listProcessedMails).not.toHaveBeenCalled();
    await open();
    expect(api.listProcessedMails).toHaveBeenCalledWith(50);
    await open();
    await open();
    expect(api.listProcessedMails).toHaveBeenCalledTimes(1);
  });

  it('listet Datum, Ergebnis und Bewerbung – Texte nur als Text', async () => {
    const { open, rows } = await render();
    await open();
    const [first, second] = rows();
    expect(first.textContent).toContain('05.10.2026');
    expect(first.querySelector('.outcome')?.textContent).toContain('Ereignis <i>Absage</i> angelegt');
    expect(first.querySelector('i')).toBeNull();
    const link = first.querySelector<HTMLAnchorElement>('a.what')!;
    expect(link.getAttribute('href')).toBe('/bewerbungen/a1');
    expect(link.textContent).toContain('Acme <b>AG</b>');
    expect(link.textContent).toContain('Frontend-Entwickler');
    expect(first.querySelector('b')).toBeNull();
    expect(second.querySelector('a.what')).toBeNull();
    expect(second.textContent).toContain('nicht zugeordnet');
  });

  it('zeigt einen Leerzustand', async () => {
    const { open, el } = await render({ listProcessedMails: vi.fn(() => of([])) });
    await open();
    expect(el.textContent).toContain('Noch keine Mails verarbeitet.');
  });

  it('zeigt einen Fehler und lädt beim nächsten Öffnen erneut', async () => {
    const listProcessedMails = vi.fn(() => throwError(() => new Error('x')));
    const { open, el } = await render({ listProcessedMails });
    await open();
    expect(el.textContent).toContain('Konnte nicht geladen werden.');
    await open();
    await open();
    expect(listProcessedMails).toHaveBeenCalledTimes(2);
  });

  it('Erneut prüfen lassen vergisst die Mail und entfernt die Zeile', async () => {
    const { api, open, rows, settle, snack } = await render();
    await open();
    rows()[0].querySelector<HTMLButtonElement>('button.recheck')!.click();
    await settle();
    expect(api.deleteProcessedMail).toHaveBeenCalledWith('m1');
    expect(rows().map((r) => r.dataset['id'])).toEqual(['m2']);
    expect(document.activeElement).toBe(rows()[0]);
    expect(snack).toHaveBeenCalledWith('Die Mail wird beim nächsten Lauf erneut ausgewertet', undefined, expect.anything());
  });

  it('Fehler beim Vergessen: Zeile bleibt und ist wieder bedienbar', async () => {
    const { open, rows, settle } = await render({ deleteProcessedMail: vi.fn(() => throwError(() => new Error('x'))) });
    await open();
    rows()[0].querySelector<HTMLButtonElement>('button.recheck')!.click();
    await settle();
    expect(rows().length).toBe(2);
    expect(rows()[0].querySelector<HTMLButtonElement>('button.recheck')!.disabled).toBe(false);
  });

  it('Busy-Guard je Zeile', async () => {
    const pending = new Subject<void>();
    const deleteProcessedMail = vi.fn(() => pending);
    const { open, rows, settle } = await render({ deleteProcessedMail });
    await open();
    const button = () => rows()[0].querySelector<HTMLButtonElement>('button.recheck')!;
    button().click();
    await settle();
    button().click();
    expect(deleteProcessedMail).toHaveBeenCalledTimes(1);
    expect(button().disabled).toBe(true);
    expect(rows()[1].querySelector<HTMLButtonElement>('button.recheck')!.disabled).toBe(false);
    pending.next();
    pending.complete();
    await settle();
    expect(rows().map((r) => r.dataset['id'])).toEqual(['m2']);
  });
});
