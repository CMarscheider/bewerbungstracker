import { DOCUMENT, Injectable, inject, signal } from '@angular/core';

export type ThemeMode = 'system' | 'light' | 'dark';

// Gleicher Schlüssel im Inline-Skript in index.html, das das Theme vor dem App-Start setzt.
export const THEME_STORAGE_KEY = 'theme';

/** Hell/Dunkel/System; setzt data-theme auf <html> und merkt sich die Wahl im Browser. */
@Injectable({ providedIn: 'root' })
export class ThemeService {
  private readonly root = inject(DOCUMENT).documentElement;
  private readonly current = signal<ThemeMode>(readStored());
  readonly mode = this.current.asReadonly();

  constructor() {
    this.apply(this.current());
  }

  set(mode: ThemeMode): void {
    this.current.set(mode);
    this.apply(mode);
    try {
      if (mode === 'system') {
        localStorage.removeItem(THEME_STORAGE_KEY);
      } else {
        localStorage.setItem(THEME_STORAGE_KEY, mode);
      }
    } catch {
      // Gesperrter Speicher: Wahl gilt nur bis zum Neuladen.
    }
  }

  private apply(mode: ThemeMode): void {
    if (mode === 'system') {
      this.root.removeAttribute('data-theme');
    } else {
      this.root.setAttribute('data-theme', mode);
    }
  }
}

function readStored(): ThemeMode {
  try {
    const v = localStorage.getItem(THEME_STORAGE_KEY);
    return v === 'light' || v === 'dark' ? v : 'system';
  } catch {
    return 'system';
  }
}
