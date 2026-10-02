import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';
import { EventType } from '../../api/models';
import { toIsoDate } from '../../core/dates';
import { provideGermanDates } from '../../core/german-date-adapter';
import { EventDialog } from './event-dialog';

function setup(type: EventType) {
  const close = vi.fn();
  TestBed.configureTestingModule({
    imports: [EventDialog],
    providers: [provideGermanDates(), { provide: MAT_DIALOG_DATA, useValue: { type } }, { provide: MatDialogRef, useValue: { close } }],
  });
  const fixture = TestBed.createComponent(EventDialog);
  fixture.detectChanges();
  return { fixture, close, el: fixture.nativeElement as HTMLElement };
}

describe('EventDialog', () => {
  it('zeigt das Fristfeld bei Typen mit Frist', () => {
    const { el } = setup('ChallengeErhalten');
    expect(el.querySelector('[data-testid="due-field"]')).not.toBeNull();
    expect(el.textContent).toContain('Abgabe');
  });

  it('blendet das Fristfeld bei anderen Typen aus', () => {
    const { el } = setup('Interview');
    expect(el.querySelector('[data-testid="due-field"]')).toBeNull();
  });

  it('liefert das Ereignis im API-Format', () => {
    const { el, close } = setup('Absage');
    const note = el.querySelector('textarea') as HTMLTextAreaElement;
    note.value = '  Absage per Mail ';
    note.dispatchEvent(new Event('input'));
    (el.querySelector('[data-testid="save"]') as HTMLButtonElement).click();

    expect(close).toHaveBeenCalledWith({ type: 'Absage', occurred_on: toIsoDate(new Date()), note: 'Absage per Mail' });
  });
});
