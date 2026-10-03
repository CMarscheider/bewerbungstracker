// Erstellt README-Screenshots aus dem laufenden Stack mit Demo-Daten.
//   BASE_URL=http://localhost:4200 npm run shoot
import { mkdir } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import { chromium } from 'playwright';

const base = process.env.BASE_URL ?? 'http://localhost:4200';
const outDir = fileURLToPath(new URL('../../docs/screenshots/', import.meta.url));
await mkdir(outDir, { recursive: true });

const offers = await (await fetch(`${base}/api/v1/applications?status=AngebotErhalten`)).json();
if (!Array.isArray(offers) || offers.length === 0) {
  throw new Error('Keine Demo-Daten gefunden – vorher `task seed` ausführen.');
}

const pages = [
  { name: 'dashboard', path: '/' },
  { name: 'bewerbungen', path: '/bewerbungen' },
  { name: 'detail', path: `/bewerbungen/${offers[0].id}` },
  { name: 'statistik', path: '/statistik' },
  { name: 'neu', path: '/bewerbungen/neu' },
];
const viewports = [
  { suffix: 'desktop', viewport: { width: 1280, height: 860 }, scale: 1, scheme: 'light', only: null },
  { suffix: 'mobile', viewport: { width: 390, height: 844 }, scale: 2, scheme: 'light', only: ['dashboard', 'detail'] },
  { suffix: 'desktop-dark', viewport: { width: 1280, height: 860 }, scale: 1, scheme: 'dark', only: ['dashboard', 'detail'] },
];

const browser = await chromium.launch();
try {
  for (const vp of viewports) {
    const context = await browser.newContext({
      viewport: vp.viewport,
      deviceScaleFactor: vp.scale,
      locale: 'de-DE',
      timezoneId: 'Europe/Berlin',
      colorScheme: vp.scheme,
    });
    const page = await context.newPage();
    for (const p of pages) {
      if (vp.only && !vp.only.includes(p.name)) continue;
      await page.goto(base + p.path, { waitUntil: 'networkidle' });
      await page.waitForTimeout(400); // Animationen von Material abklingen lassen
      const file = `${outDir}${p.name}-${vp.suffix}.png`;
      await page.screenshot({ path: file, fullPage: true });
      console.log('gespeichert:', file);
    }
    await context.close();
  }
} finally {
  await browser.close();
}
