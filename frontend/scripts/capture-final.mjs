/**
 * Final polish screenshots → docs/screenshots + store media.
 * Usage: node scripts/capture-final.mjs [baseUrl]
 */
import { chromium } from 'playwright'
import { mkdir, copyFile } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const ROOT = path.resolve(__dirname, '../..')
const BASE = process.argv[2] || 'http://127.0.0.1:5173/demo.html'
const MEDIA = '/cursor/stores/bc-01a1f952-18e1-7825-8cd8-3e318e381534/media'
const DOCS = path.join(ROOT, 'docs/screenshots')
const ARTIFACTS = '/opt/cursor/artifacts'

const shots = [
  { file: 'overview.png', tab: 'overview', prep: 'alerts' },
  { file: 'hardware-16threads.png', tab: 'hardware', prep: 'threads16' },
  { file: 'about.png', tab: 'overview', prep: 'about' },
  { file: 'minihud-settings-ru.png', tab: 'settings', prep: 'minihud' },
  { file: 'lags.png', tab: 'lag' },
  { file: 'gamepulse.png', tab: 'gamelab', prep: 'game' },
]

async function waitReady(page) {
  await page.waitForSelector('#tab-overview.tab.active', { timeout: 15000 })
  await page.waitForFunction(() => {
    const el = document.getElementById('ov-cpu')
    return el && el.textContent && !el.textContent.includes('—')
  }, { timeout: 15000 }).catch(() => {})
  await page.waitForTimeout(1600)
}

async function openTab(page, tab) {
  await page.click(`.nav-item[data-tab="${tab}"]`)
  await page.waitForSelector(`#tab-${tab}.tab.active`, { timeout: 8000 })
  await page.waitForTimeout(500)
}

async function saveShot(page, file) {
  const outMedia = path.join(MEDIA, file)
  const outDocs = path.join(DOCS, file)
  const outArt = path.join(ARTIFACTS, file)
  await page.screenshot({ path: outMedia, fullPage: false })
  await copyFile(outMedia, outDocs)
  await copyFile(outMedia, outArt)
  console.log('saved', file)
}

async function main() {
  await mkdir(MEDIA, { recursive: true })
  await mkdir(DOCS, { recursive: true })
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

  // Force RU for polish shots
  await page.evaluate(() => {
    localStorage.setItem('zp_lang', 'ru')
    const sel = document.getElementById('lang-select')
    if (sel) {
      sel.value = 'ru'
      sel.dispatchEvent(new Event('change', { bubbles: true }))
    }
  })
  await page.waitForTimeout(400)

  for (const shot of shots) {
    if (shot.prep === 'alerts') {
      await page.evaluate(() => window.__PULSECORE_MOCK__?.setProfile?.('A'))
      await page.waitForTimeout(1200)
      await openTab(page, 'overview')
      await page.waitForSelector('#insight-list .insight-card', { timeout: 8000 }).catch(() => {})
      await saveShot(page, shot.file)
      continue
    }

    if (shot.prep === 'threads16') {
      await page.evaluate(() => {
        const mock = window.__PULSECORE_MOCK__
        if (!mock?.profiles?.B) return
        mock.profiles.B.physicalCores = 8
        mock.profiles.B.logicalCores = 16
        mock.profiles.B.cpuName = 'AMD Ryzen 7 5700X'
        mock.setProfile('B')
      })
      await page.waitForTimeout(900)
      await openTab(page, 'hardware')
      await page.waitForFunction(() => {
        const g = document.getElementById('core-grid')
        return g && g.querySelectorAll('.core-cell').length === 16
      }, { timeout: 8000 }).catch(() => {})
      await saveShot(page, shot.file)
      continue
    }

    if (shot.prep === 'about') {
      await page.evaluate(() => window.__PULSECORE_MOCK__?.setProfile?.('B'))
      await page.waitForTimeout(600)
      await openTab(page, 'overview')
      await page.click('#btn-about')
      await page.waitForSelector('#about-modal:not(.hidden)', { timeout: 5000 })
      await page.waitForSelector('#about-specs .about-line', { timeout: 5000 }).catch(() => {})
      await page.waitForTimeout(300)
      await saveShot(page, shot.file)
      await page.click('#about-close')
      await page.waitForSelector('#about-modal.hidden', { timeout: 5000 }).catch(() => {})
      continue
    }

    if (shot.prep === 'minihud') {
      await openTab(page, 'settings')
      await page.waitForSelector('#mh-opt-temps', { timeout: 5000 })
      await page.evaluate(() => {
        const el = document.querySelector('#tab-settings')
        const target = document.getElementById('mh-opt-temps')
        target?.scrollIntoView({ block: 'center' })
        el?.scrollIntoView?.()
      })
      await page.waitForTimeout(400)
      await saveShot(page, shot.file)
      continue
    }

    await openTab(page, shot.tab)
    if (shot.prep === 'game') {
      await page.evaluate(() => window.__PULSECORE_MOCK__?.setProfile?.('B'))
      await page.waitForSelector('#game-grid .game-card', { timeout: 10000 }).catch(() => {})
      const card = page.locator('#game-grid .game-card').first()
      if (await card.count()) {
        await card.click()
        await page.waitForSelector('#fps-result .fps-grid', { timeout: 8000 }).catch(() => {})
        await page.waitForTimeout(600)
      }
    }
    await saveShot(page, shot.file)
  }

  // Also capture compact overview at 1024×680
  await page.setViewportSize({ width: 1024, height: 680 })
  await page.evaluate(() => window.__PULSECORE_MOCK__?.setProfile?.('A'))
  await page.waitForTimeout(900)
  await openTab(page, 'overview')
  await page.waitForSelector('#insight-list .insight-card', { timeout: 8000 }).catch(() => {})
  await saveShot(page, 'overview-1024.png')

  await browser.close()
  console.log('done')
}

main().catch((err) => {
  console.error(err)
  process.exit(1)
})
