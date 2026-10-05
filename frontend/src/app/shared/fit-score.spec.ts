import { TestBed } from '@angular/core/testing';
import { FitScore, fitLevel } from './fit-score';

describe('fitLevel', () => {
  it('stuft ein', () => {
    expect(fitLevel(85)).toBe('hoch');
    expect(fitLevel(70)).toBe('hoch');
    expect(fitLevel(69)).toBe('mittel');
    expect(fitLevel(40)).toBe('mittel');
    expect(fitLevel(39)).toBe('niedrig');
  });
});

describe('FitScore', () => {
  it('zeigt Zahl, Stufe und zugängliche Beschriftung', async () => {
    const fixture = TestBed.createComponent(FitScore);
    fixture.componentRef.setInput('score', 82);
    await fixture.whenStable();
    const el: HTMLElement = fixture.nativeElement.querySelector('.fit');
    expect(el.textContent?.trim()).toBe('82');
    expect(el.classList).toContain('hoch');
    expect(el.getAttribute('aria-label')).toBe('Passung 82 von 100');
  });
});
