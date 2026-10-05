import { Provider } from '@angular/core';
import { MAT_FORM_FIELD_DEFAULT_OPTIONS, MatFormFieldDefaultOptions } from '@angular/material/form-field';

/**
 * Umrandete Formularfelder passend zum flachen Kartenstil. Wird je Seite (lazy) bereitgestellt statt in
 * app.config, damit das Form-Field-Modul nicht ins Start-Bundle wandert.
 */
export const OUTLINED_FIELDS: Provider = {
  provide: MAT_FORM_FIELD_DEFAULT_OPTIONS,
  useValue: { appearance: 'outline' } satisfies MatFormFieldDefaultOptions,
};
