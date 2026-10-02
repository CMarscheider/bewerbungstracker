import { TestBed } from '@angular/core/testing';
import { StatusBadge } from './status-badge';

describe('StatusBadge', () => {
  it('zeigt das Label und setzt Phase- und Status-Klassen', async () => {
    const fixture = TestBed.createComponent(StatusBadge);
    fixture.componentRef.setInput('status', 'AngebotErhalten');
    fixture.componentRef.setInput('phase', 'Aktiv');
    await fixture.whenStable();

    const badge = (fixture.nativeElement as HTMLElement).querySelector('.badge') as HTMLElement;
    expect(badge.textContent?.trim()).toBe('Angebot erhalten');
    expect(badge.classList).toContain('badge');
    expect(badge.classList).toContain('phase-Aktiv');
    expect(badge.classList).toContain('status-AngebotErhalten');
  });
});
