import { Routes } from '@angular/router';

export const routes: Routes = [
  { path: '', title: 'Übersicht', loadComponent: () => import('./features/dashboard/dashboard').then((m) => m.Dashboard) },
  { path: 'bewerbungen', title: 'Bewerbungen', loadComponent: () => import('./features/applications/application-list').then((m) => m.ApplicationList) },
  { path: 'bewerbungen/neu', title: 'Neue Bewerbung', loadComponent: () => import('./features/applications/application-new').then((m) => m.ApplicationNew) },
  { path: 'bewerbungen/:id', title: 'Bewerbung', loadComponent: () => import('./features/applications/application-detail').then((m) => m.ApplicationDetail) },
  { path: 'statistik', title: 'Statistik', loadComponent: () => import('./features/stats/stats').then((m) => m.Stats) },
  { path: 'firmen', title: 'Firmen', loadComponent: () => import('./features/companies/companies').then((m) => m.Companies) },
  { path: '**', redirectTo: '' },
];
