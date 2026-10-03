import { HttpContextToken, HttpErrorResponse, HttpInterceptorFn } from '@angular/common/http';
import { inject } from '@angular/core';
import { MatSnackBar } from '@angular/material/snack-bar';
import { catchError, throwError } from 'rxjs';

/** Anfragen mit diesem Token melden ein 404 nicht als Snackbar (erwarteter Fall, z. B. „noch kein Foto“). */
export const SILENT_NOT_FOUND = new HttpContextToken<boolean>(() => false);

/** Zeigt jeden HTTP-Fehler als Snackbar; Aufrufer bekommen den Fehler trotzdem. */
export const errorInterceptor: HttpInterceptorFn = (req, next) => {
  const snackBar = inject(MatSnackBar);
  return next(req).pipe(
    catchError((err: unknown) => {
      if (err instanceof HttpErrorResponse && !(err.status === 404 && req.context.get(SILENT_NOT_FOUND))) {
        snackBar.open(problemMessage(err), 'OK', { duration: 6000 });
      }
      return throwError(() => err);
    }),
  );
};

/** Liest die Meldung aus einer RFC-9457-Antwort (application/problem+json). */
export function problemMessage(err: HttpErrorResponse): string {
  let body: unknown = err.error;
  if (typeof body === 'string') {
    try {
      body = JSON.parse(body);
    } catch {
      body = null;
    }
  }
  if (body && typeof body === 'object') {
    const { detail, title } = body as { detail?: unknown; title?: unknown };
    if (typeof detail === 'string' && detail) {
      return detail;
    }
    if (typeof title === 'string' && title) {
      return title;
    }
  }
  return err.status === 0 ? 'Server nicht erreichbar' : `Fehler ${err.status}`;
}
