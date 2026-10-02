import { Title } from '@angular/platform-browser';
import { RouterStateSnapshot, TitleStrategy } from '@angular/router';
import { TestBed } from '@angular/core/testing';
import { PageTitleStrategy } from './title-strategy';

describe('PageTitleStrategy', () => {
  function setup() {
    TestBed.configureTestingModule({ providers: [{ provide: TitleStrategy, useClass: PageTitleStrategy }] });
    return { strategy: TestBed.inject(TitleStrategy), title: TestBed.inject(Title) };
  }

  it('hängt den App-Namen an den Routen-Titel', () => {
    const { strategy, title } = setup();
    vi.spyOn(strategy, 'buildTitle').mockReturnValue('Firmen');
    strategy.updateTitle({} as RouterStateSnapshot);
    expect(title.getTitle()).toBe('Firmen · Bewerbungs-Tracker');
  });

  it('nutzt nur den App-Namen ohne Routen-Titel', () => {
    const { strategy, title } = setup();
    vi.spyOn(strategy, 'buildTitle').mockReturnValue(undefined);
    strategy.updateTitle({} as RouterStateSnapshot);
    expect(title.getTitle()).toBe('Bewerbungs-Tracker');
  });
});
