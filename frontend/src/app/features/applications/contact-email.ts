import { FormControl, Validators } from '@angular/forms';

/**
 * Wie das Muster in api/openapi.yaml; Leerzeichen außen toleriert, da beim Speichern getrimmt wird.
 * Leer (auch nur Leerzeichen) ist erlaubt – im PATCH löscht das die Adresse.
 */
const CONTACT_EMAIL = /^\s*([^@\s]+@[^@\s]+\.[^@\s]+)?\s*$/;

export const CONTACT_EMAIL_ERROR = 'Bitte eine gültige E-Mail-Adresse eingeben';

export const contactEmailControl = () => new FormControl('', { nonNullable: true, validators: [Validators.pattern(CONTACT_EMAIL)] });
