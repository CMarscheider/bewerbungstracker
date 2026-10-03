import { HttpContext } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';
import {
  Application,
  ApplicationInput,
  ApplicationPatch,
  ApplicationSummary,
  Appointment,
  Company,
  CompanyInput,
  CompanyPatch,
  Cv,
  Deadline,
  Event,
  EventType,
  FunnelStep,
  NewEvent,
  Phase,
  Summary,
} from '../api/models';
import { SILENT_NOT_FOUND } from './error.interceptor';
import { ApplicationsService, CompaniesService, CvService, DashboardService } from '../api/services';

export interface ApplicationFilter {
  phase?: Phase;
  status?: EventType;
  q?: string;
}

/** Einzige Stelle, an der Komponenten die API sehen – leicht zu mocken. */
@Injectable({ providedIn: 'root' })
export class Api {
  private readonly companies = inject(CompaniesService);
  private readonly applications = inject(ApplicationsService);
  private readonly dashboard = inject(DashboardService);
  private readonly cv = inject(CvService);

  listCompanies(): Observable<Company[]> {
    return this.companies.listCompanies();
  }
  createCompany(body: CompanyInput): Observable<Company> {
    return this.companies.createCompany({ body });
  }
  updateCompany(id: string, body: CompanyPatch): Observable<Company> {
    return this.companies.updateCompany({ id, body });
  }
  deleteCompany(id: string): Observable<void> {
    return this.companies.deleteCompany({ id });
  }

  listApplications(filter: ApplicationFilter = {}): Observable<ApplicationSummary[]> {
    return this.applications.listApplications(filter);
  }
  createApplication(body: ApplicationInput): Observable<Application> {
    return this.applications.createApplication({ body });
  }
  getApplication(id: string): Observable<Application> {
    return this.applications.getApplication({ id });
  }
  updateApplication(id: string, body: ApplicationPatch): Observable<Application> {
    return this.applications.updateApplication({ id, body });
  }
  deleteApplication(id: string): Observable<void> {
    return this.applications.deleteApplication({ id });
  }
  addEvent(id: string, body: NewEvent): Observable<Event> {
    return this.applications.addEvent({ id, body });
  }
  undoLastEvent(id: string): Observable<Application> {
    return this.applications.undoLastEvent({ id });
  }
  listAllowedEvents(id: string): Observable<EventType[]> {
    return this.applications.listAllowedEvents({ id });
  }

  listDeadlines(withinDays = 7): Observable<Deadline[]> {
    return this.dashboard.listDeadlines({ within_days: withinDays });
  }
  listAppointments(withinDays = 14): Observable<Appointment[]> {
    return this.dashboard.listAppointments({ within_days: withinDays });
  }
  getFunnel(): Observable<FunnelStep[]> {
    return this.dashboard.getFunnel();
  }
  getSummary(): Observable<Summary> {
    return this.dashboard.getSummary();
  }

  getCv(): Observable<Cv> {
    return this.cv.getCv();
  }
  saveCv(body: Cv): Observable<Cv> {
    return this.cv.saveCv({ body });
  }
  /** 404 heißt „noch kein Foto“ und wird nicht als Fehler gemeldet. */
  getCvPhoto(): Observable<Blob> {
    return this.cv.getCvPhoto(undefined, new HttpContext().set(SILENT_NOT_FOUND, true));
  }
  saveCvPhoto(image: Blob): Observable<void> {
    return this.cv.saveCvPhoto({ body: image });
  }
  deleteCvPhoto(): Observable<void> {
    return this.cv.deleteCvPhoto();
  }
}
