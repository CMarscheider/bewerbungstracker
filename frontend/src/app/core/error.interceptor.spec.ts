import { HttpClient, HttpErrorResponse, provideHttpClient, withInterceptors } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { MatSnackBar } from '@angular/material/snack-bar';
import { errorInterceptor, problemMessage } from './error.interceptor';

describe('problemMessage', () => {
  it('nimmt detail aus Problem-JSON', () => {
    const err = new HttpErrorResponse({ status: 422, error: { title: 'Statuswechsel nicht erlaubt', detail: 'Übergang von Absage nach Interview ist nicht erlaubt' } });
    expect(problemMessage(err)).toBe('Übergang von Absage nach Interview ist nicht erlaubt');
  });

  it('liest Problem-JSON auch aus Text', () => {
    const err = new HttpErrorResponse({ status: 409, error: JSON.stringify({ title: 'Konflikt', detail: 'Firma existiert bereits' }) });
    expect(problemMessage(err)).toBe('Firma existiert bereits');
  });

  it('fällt auf title zurück', () => {
    expect(problemMessage(new HttpErrorResponse({ status: 500, error: { title: 'Interner Fehler' } }))).toBe('Interner Fehler');
  });

  it('meldet fehlende Verbindung', () => {
    expect(problemMessage(new HttpErrorResponse({ status: 0 }))).toBe('Server nicht erreichbar');
  });

  it('meldet den Status ohne Body', () => {
    expect(problemMessage(new HttpErrorResponse({ status: 502, error: null }))).toBe('Fehler 502');
  });
});

describe('errorInterceptor', () => {
  it('zeigt Fehler als Snackbar und reicht sie weiter', () => {
    const open = vi.fn();
    TestBed.configureTestingModule({
      providers: [
        provideHttpClient(withInterceptors([errorInterceptor])),
        provideHttpClientTesting(),
        { provide: MatSnackBar, useValue: { open } },
      ],
    });
    const http = TestBed.inject(HttpClient);
    const ctrl = TestBed.inject(HttpTestingController);

    let failed = false;
    http.get('/api/v1/companies').subscribe({ error: () => (failed = true) });
    ctrl.expectOne('/api/v1/companies').flush(
      { title: 'Konflikt', detail: 'Firma existiert bereits' },
      { status: 409, statusText: 'Conflict' },
    );

    expect(open).toHaveBeenCalledWith('Firma existiert bereits', 'OK', { duration: 6000 });
    expect(failed).toBe(true);
  });
});
