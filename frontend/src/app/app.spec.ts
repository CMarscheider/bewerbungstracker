import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { App } from './app';

describe('App', () => {
  it('zeigt die Hauptnavigation', async () => {
    TestBed.configureTestingModule({ imports: [App], providers: [provideRouter([])] });
    const fixture = TestBed.createComponent(App);
    fixture.detectChanges();
    await fixture.whenStable();

    const links = [...(fixture.nativeElement as HTMLElement).querySelectorAll('nav a')].map((a) => a.textContent?.trim());
    expect((fixture.nativeElement as HTMLElement).querySelector('main')).not.toBeNull();
    expect(links).toEqual(['Übersicht', 'Bewerbungen', 'Statistik', 'Firmen', 'Lebenslauf']);
  });
});
