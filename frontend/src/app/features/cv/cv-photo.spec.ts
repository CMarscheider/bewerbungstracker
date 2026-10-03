import { TestBed } from '@angular/core/testing';
import { MatSnackBar } from '@angular/material/snack-bar';
import { of, throwError } from 'rxjs';
import { Api } from '../../core/api';
import { ImageResizer } from '../../core/image-resizer';
import { CvPhoto } from './cv-photo';

async function render(photo: Blob | null) {
  const api = {
    getCvPhoto: vi.fn(() => (photo ? of(photo) : throwError(() => ({ status: 404 })))),
    saveCvPhoto: vi.fn(() => of(undefined)),
    deleteCvPhoto: vi.fn(() => of(undefined)),
  };
  const resizer = { toPortraitJpeg: vi.fn(async () => new Blob(['jpeg'], { type: 'image/jpeg' })) };
  let n = 0;
  const create = vi.spyOn(URL, 'createObjectURL').mockImplementation(() => (n++ === 0 ? 'blob:foto' : 'blob:foto' + n));
  const revoke = vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => undefined);
  TestBed.configureTestingModule({
    imports: [CvPhoto],
    providers: [
      { provide: Api, useValue: api },
      { provide: ImageResizer, useValue: resizer },
    ],
  });
  const snackBar = TestBed.inject(MatSnackBar);
  vi.spyOn(snackBar, 'open');
  const fixture = TestBed.createComponent(CvPhoto);
  const settle = async () => {
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
  };
  await settle();
  return { api, resizer, snackBar, create, revoke, settle, el: fixture.nativeElement as HTMLElement };
}

function choose(el: HTMLElement, file: File) {
  const input = el.querySelector<HTMLInputElement>('input[type="file"]')!;
  Object.defineProperty(input, 'files', { value: [file] });
  input.dispatchEvent(new Event('change'));
}

describe('CvPhoto', () => {
  afterEach(() => vi.restoreAllMocks());

  it('zeigt das vorhandene Foto', async () => {
    const { el } = await render(new Blob(['x'], { type: 'image/jpeg' }));
    expect(el.querySelector<HTMLImageElement>('img')!.src).toBe('blob:foto');
    expect(el.querySelector('button.remove')).not.toBeNull();
  });

  it('zeigt einen Platzhalter ohne Foto', async () => {
    const { el } = await render(null);
    expect(el.querySelector('img')).toBeNull();
    expect(el.textContent).toContain('Noch kein Foto');
  });

  it('verkleinert und lädt ein gewähltes Bild hoch', async () => {
    const { el, api, resizer, settle } = await render(null);
    choose(el, new File(['raw'], 'foto.png', { type: 'image/png' }));
    await settle();
    await settle();
    expect(resizer.toPortraitJpeg).toHaveBeenCalled();
    expect(api.saveCvPhoto).toHaveBeenCalledTimes(1);
    expect(el.querySelector<HTMLImageElement>('img')!.src).toBe('blob:foto');
  });

  it('lehnt Nicht-Bilder ab', async () => {
    const { el, api, snackBar, settle } = await render(null);
    choose(el, new File(['%PDF'], 'lebenslauf.pdf', { type: 'application/pdf' }));
    await settle();
    expect(api.saveCvPhoto).not.toHaveBeenCalled();
    expect(snackBar.open).toHaveBeenCalledWith('Bitte ein Bild (JPG, PNG oder WebP) wählen', 'OK', { duration: 4000 });
  });

  it('entfernt das Foto', async () => {
    const { el, api, settle } = await render(new Blob(['x'], { type: 'image/jpeg' }));
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    el.querySelector<HTMLButtonElement>('button.remove')!.click();
    await settle();
    expect(api.deleteCvPhoto).toHaveBeenCalled();
    expect(el.querySelector('img')).toBeNull();
  });

  it('gibt die alte Bild-URL frei, wenn das Foto ersetzt wird', async () => {
    const { el, revoke, settle } = await render(new Blob(['x'], { type: 'image/jpeg' }));
    choose(el, new File(['raw'], 'neu.png', { type: 'image/png' }));
    await settle();
    await settle();
    expect(revoke).toHaveBeenCalledWith('blob:foto');
    expect(el.querySelector<HTMLImageElement>('img')!.src).not.toBe('blob:foto');
  });

  it('löscht nicht, wenn die Rückfrage abgelehnt wird', async () => {
    const { el, api, settle } = await render(new Blob(['x'], { type: 'image/jpeg' }));
    vi.spyOn(window, 'confirm').mockReturnValue(false);
    el.querySelector<HTMLButtonElement>('button.remove')!.click();
    await settle();
    expect(api.deleteCvPhoto).not.toHaveBeenCalled();
    expect(el.querySelector('img')).not.toBeNull();
  });

  it('meldet Fehler beim Verkleinern und gibt den Button wieder frei', async () => {
    const { el, api, resizer, snackBar, settle } = await render(null);
    resizer.toPortraitJpeg.mockRejectedValueOnce(new Error('kaputt'));
    choose(el, new File(['raw'], 'foto.png', { type: 'image/png' }));
    await settle();
    await settle();
    expect(api.saveCvPhoto).not.toHaveBeenCalled();
    expect(snackBar.open).toHaveBeenCalledWith('Das Bild konnte nicht gelesen werden', 'OK', { duration: 4000 });
    expect(el.querySelector<HTMLButtonElement>('button.upload')!.disabled).toBe(false);
  });
});
