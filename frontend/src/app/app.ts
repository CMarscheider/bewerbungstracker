import { Component } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatToolbarModule } from '@angular/material/toolbar';
import { RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';
import { ThemeToggle } from './shared/theme-toggle';

@Component({
  selector: 'app-root',
  imports: [RouterOutlet, RouterLink, RouterLinkActive, MatToolbarModule, MatButtonModule, ThemeToggle],
  templateUrl: './app.html',
  styleUrl: './app.scss',
})
export class App {
  protected readonly links = [
    { path: '/', label: 'Übersicht', exact: true },
    { path: '/bewerbungen', label: 'Bewerbungen', exact: false },
    { path: '/statistik', label: 'Statistik', exact: false },
    { path: '/firmen', label: 'Firmen', exact: false },
  ];
}
