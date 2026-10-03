import { Component, DestroyRef, inject, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatSnackBar } from '@angular/material/snack-bar';
import { finalize } from 'rxjs';
import { Api } from '../../core/api';
import { ImageResizer } from '../../core/image-resizer';

@Component({
  selector: 'app-cv-photo',
  imports: [MatButtonModule],
  templateUrl: './cv-photo.html',
  styleUrl: './cv-photo.scss',
})
export class CvPhoto {
  private readonly api = inject(Api);
  private readonly resizer = inject(ImageResizer);
  private readonly snackBar = inject(MatSnackBar);

  protected readonly url = signal<string | null>(null);
  protected readonly busy = signal(false);

  constructor() {
    inject(DestroyRef).onDestroy(() => this.setUrl(null));
    this.api.getCvPhoto().subscribe({
      next: (blob) => this.setUrl(URL.createObjectURL(blob)),
      error: () => undefined, // 404 = noch kein Foto; andere Fehler meldet der Interceptor.
    });
  }

  protected async choose(event: Event): Promise<void> {
    const input = event.target as HTMLInputElement;
    const file = input.files?.[0];
    input.value = '';
    if (!file) {
      return;
    }
    if (!['image/jpeg', 'image/png', 'image/webp'].includes(file.type)) {
      this.snackBar.open('Bitte ein Bild (JPG oder PNG) wählen', 'OK', { duration: 4000 });
      return;
    }
    this.busy.set(true);
    let jpeg: Blob;
    try {
      jpeg = await this.resizer.toPortraitJpeg(file);
    } catch {
      this.busy.set(false);
      this.snackBar.open('Das Bild konnte nicht gelesen werden', 'OK', { duration: 4000 });
      return;
    }
    this.api
      .saveCvPhoto(jpeg)
      .pipe(finalize(() => this.busy.set(false)))
      .subscribe({
        next: () => this.setUrl(URL.createObjectURL(jpeg)),
        error: () => undefined,
      });
  }

  protected remove(): void {
    if (!window.confirm('Foto entfernen?')) {
      return;
    }
    this.api.deleteCvPhoto().subscribe({ next: () => this.setUrl(null), error: () => undefined });
  }

  private setUrl(next: string | null): void {
    const old = this.url();
    if (old) {
      URL.revokeObjectURL(old);
    }
    this.url.set(next);
  }
}
