/**
 * Headless Playwright capture of redesigned PulseCore screens (mock data).
 * Usage: node scripts/capture-ui.mjs [baseUrl]
 */
import { chromium } from 'playwright'
import { mkdir, copyFile } from 'node:fs/promises'
import path from 'node:path'

const BASE = process.argv[2] || 'http://127.0.0.1:5173/demo.html'
const MEDIA = '/cursor/stores/bc-01a1f952-18e1-7825-8cd8-3e318e381534/media'
const ARTIFACTS = '/opt/cursor/artifacts'

const shots = [
  { file: 'ui-home-overview.png', label: 'Home / Главная', tab: 'overview' },
  { file: 'ui-lags.png', label: 'Lags / Лаги', tab: 'lag' },
  { file: 'ui-games-gamepulse.png', label: 'Games / GamePulse', tab: 'gamelab', extra: 'game' },
  { file: 'ui-settings.png', label: 'Settings / Настройки', tab: 'settings' },
  { file: 'ui-mini-hud.png', label: 'Mini-HUD', tab: null, hud: true },
]

async function waitReady(page) {
  await page.waitForSelector('#tab-overview.tab.active', { timeout: 15000 })
  await page.waitForFunction(() => {
    const el = document.getElementById('ov-cpu')
    return el && el.textContent && !el.textContent.includes('—') && !el.textContent.includes('Н/Д')
  }, { timeout: 15000 }).catch(() => {})
  // give sparklines a couple of frames
  await page.waitForTimeout(1200)
}

async function openTab(page, tab) {
  await page.click(`.nav-item[data-tab="${tab}"]`)
  await page.waitForSelector(`#tab-${tab}.tab.active`, { timeout: 8000 })
  await page.waitForTimeout(600)
}

async function main() {
  await mkdir(MEDIA, { recursive: true })
  await mkdir(ARTIFACTS, { recursive: true })

  const browser = await chromium.launch({
    headless: true,
    args: ['--no-sandbox', '--disable-dev-shm-usage'],
  })
  const page = await browser.newPage({
    viewport: { width: 1280, height: 800 },
    deviceScaleFactor: 1,
  })

  console.log('Opening', BASE)
  await page.goto(BASE, { waitUntil: 'networkidle', timeout: 30000 })
  await waitReady(page)

  for (const shot of shots) {
    if (shot.hud) {
      // return to overview chrome first if needed
      const hudBtn = page.locator('#btn-hud-title')
      if (await hudBtn.count()) {
        await hudBtn.click()
      } else {
        await page.evaluate(() => window.go?.main?.App?.ToggleMiniHUD?.(true))
      }
      await page.waitForSelector('#minihud:not(.hidden)', { timeout: 8000 })
      await page.waitForTimeout(500)
      // Mini-HUD is full-window overlay; capture viewport
      const out = path.join(MEDIA, shot.file)
      await page.screenshot({ path: out, fullPage: false })
      await copyFile(out, path.join(ARTIFACTS, shot.file))
      console.log('saved', out)
      // restore shell for cleanliness
      await page.click('#mh-expand')
      await page.waitForSelector('#shell:not(.hidden)', { timeout: 8000 })
      continue
    }

    await openTab(page, shot.tab)

    if (shot.extra === 'game') {
      // pick first game card for FPS panel
      const card = page.locator('#game-grid .game-card').first()
      if (await card.count()) {
        await card.click()
        await page.waitForSelector('#fps-result .fps-grid', { timeout: 8000 }).catch(() => {})
        await page.waitForTimeout(500)
      }
    }

    const out = path.join(MEDIA, shot.file)
    await page.screenshot({ path: out, fullPage: false })
    await copyFile(out, path.join(ARTIFACTS, shot.file))
    console.log('saved', out)
  }

  await browser.close()
  console.log('done')
}

main().catch((err) => {
  console.error(err)
  process.exit(1)
})
