import { Observable, catchError, map, of } from 'rxjs';

/** Ergebnis einer Abfrage: Daten oder Fehler (die Meldung zeigt der Interceptor). */
export type Loaded<T> = { readonly error: false; readonly data: T } | { readonly error: true; readonly data?: undefined };

/** Macht aus einer Abfrage ein Ergebnis, das nie fehlschlägt – so lassen sich Laden, Fehler und Leer unterscheiden. */
export function loaded<T>(source: Observable<T>): Observable<Loaded<T>> {
  return source.pipe(
    map((data): Loaded<T> => ({ error: false, data })),
    catchError(() => of<Loaded<T>>({ error: true })),
  );
}
