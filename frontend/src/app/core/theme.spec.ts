import { TestBed } from '@angular/core/testing';
import { THEME_STORAGE_KEY, ThemeService } from './theme';

describe('ThemeService', () => {
  afterEach(() => {
    localStorage.clear();
    document.documentElement.removeAttribute('data-theme');
    vi.restoreAllMocks();
  });

  it('startet ohne gespeicherte Wahl im System-Modus und setzt kein data-theme', () => {
    const theme = TestBed.inject(ThemeService);
    expect(theme.mode()).toBe('system');
    expect(document.documentElement.hasAttribute('data-theme')).toBe(false);
  });

  it('setzt data-theme und merkt sich die Wahl', () => {
    const theme = TestBed.inject(ThemeService);
    theme.set('dark');
    expect(document.documentElement.getAttribute('data-theme')).toBe('dark');
    expect(localStorage.getItem(THEME_STORAGE_KEY)).toBe('dark');

    theme.set('system');
    expect(document.documentElement.hasAttribute('data-theme')).toBe(false);
    expect(localStorage.getItem(THEME_STORAGE_KEY)).toBeNull();
  });

  it('übernimmt eine gespeicherte Wahl und ignoriert ungültige Werte', () => {
    localStorage.setItem(THEME_STORAGE_KEY, 'light');
    expect(TestBed.inject(ThemeService).mode()).toBe('light');

    TestBed.resetTestingModule();
    localStorage.setItem(THEME_STORAGE_KEY, 'lila');
    expect(TestBed.inject(ThemeService).mode()).toBe('system');
  });

  it('funktioniert weiter, wenn der Browser-Speicher gesperrt ist', () => {
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
      throw new Error('blockiert');
    });
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('blockiert');
    });
    const theme = TestBed.inject(ThemeService);
    expect(theme.mode()).toBe('system');
    theme.set('dark');
    expect(theme.mode()).toBe('dark');
    expect(document.documentElement.getAttribute('data-theme')).toBe('dark');
  });
});
