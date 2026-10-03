import { Injectable } from '@angular/core';

/** Schneidet ein Bild mittig auf 3:4 zu und verkleinert es auf 600×800 Pixel (JPEG). */
@Injectable({ providedIn: 'root' })
export class ImageResizer {
  async toPortraitJpeg(file: Blob, width = 600, height = 800, quality = 0.85): Promise<Blob> {
    const bitmap = await createImageBitmap(file);
    const target = width / height;
    const source = bitmap.width / bitmap.height;
    const sw = source > target ? bitmap.height * target : bitmap.width;
    const sh = source > target ? bitmap.height : bitmap.width / target;
    const canvas = document.createElement('canvas');
    canvas.width = width;
    canvas.height = height;
    const ctx = canvas.getContext('2d');
    if (!ctx) {
      throw new Error('Canvas nicht verfügbar');
    }
    ctx.drawImage(bitmap, (bitmap.width - sw) / 2, (bitmap.height - sh) / 2, sw, sh, 0, 0, width, height);
    bitmap.close();
    return new Promise((resolve, reject) =>
      canvas.toBlob((blob) => (blob ? resolve(blob) : reject(new Error('Bild konnte nicht umgewandelt werden'))), 'image/jpeg', quality),
    );
  }
}
