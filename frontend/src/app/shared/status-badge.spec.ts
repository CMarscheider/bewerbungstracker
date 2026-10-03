import { TestBed } from '@angular/core/testing';
import { StatusBadge } from './status-badge';

describe('StatusBadge', () => {
  it('zeigt das Label und setzt die Farbton-Klasse des Status', async () => {
    const fixture = TestBed.createComponent(StatusBadge);
    fixture.componentRef.setInput('status', 'AngebotErhalten');
    await fixture.whenStable();

    const badge = (fixture.nativeElement as HTMLElement).querySelector('.badge') as HTMLElement;
    expect(badge.textContent?.trim()).toBe('Angebot erhalten');
    expect(badge.classList).toContain('tone-green');
  });

  it('färbt Status desselben Prozessschritts gleich', async () => {
    const tones = await Promise.all(
      (['ScreeningGespraech', 'Interview', 'Kennenlerntag'] as const).map(async (status) => {
        const fixture = TestBed.createComponent(StatusBadge);
        fixture.componentRef.setInput('status', status);
        await fixture.whenStable();
        return (fixture.nativeElement as HTMLElement).querySelector('.badge')!.className;
      }),
    );
    expect(new Set(tones).size).toBe(1);
    expect(tones[0]).toContain('tone-violet');
  });
});
