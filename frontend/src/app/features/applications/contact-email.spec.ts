import { contactEmailControl } from './contact-email';

describe('contactEmailControl', () => {
  const valid = (value: string) => {
    const c = contactEmailControl();
    c.setValue(value);
    return c.valid;
  };

  it('akzeptiert leere und gültige Adressen, auch mit Leerzeichen außen', () => {
    for (const v of ['', '   ', 'jobs@acme.example', '  jobs@acme.example  ', 'vorname.nachname+bewerbung@firma.de']) {
      expect(valid(v), v).toBe(true);
    }
  });

  it('lehnt ungültige Adressen und mailto-Parameter ab', () => {
    const invalid = [
      'kein-mail', 'a@b', 'a b@c.de', 'a@b.de?subject=x', 'a@b.de?cc=x', 'a@b.de%3Fcc', 'a@b.de#x', 'a@b.de&cc=c@d.de',
      'a@b.de=x', '<a@b.de>', '"a"@b.de', 'a,b@c.de', 'a;b@c.de', 'a:b@c.de', 'a/b@c.de',
    ];
    for (const v of invalid) {
      expect(valid(v), v).toBe(false);
    }
  });
});
