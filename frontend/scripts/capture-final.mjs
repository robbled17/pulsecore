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

async function waitReady(page) {
  await page.waitForSelector('#tab-overview.tab.active', { timeout: 15000 })
  await page.waitForFunction(() => {
    const el = document.getElementById('ov-cpu')
    return el && el.textContent && el.textContent.trim() !== '-'
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

async function enableDev(page) {
  await page.click('#btn-about')
  await page.waitForSelector('#about-modal:not(.hidden)')
  for (let i = 0; i < 7; i++) {
    await page.click('#about-app-ver')
    await page.waitForTimeout(80)
  }
  await page.waitForSelector('#tab-dev.tab.active, .dev-mode-nav:not(.hidden)', { timeout: 5000 }).catch(() => {})
  await page.click('#about-close').catch(() => {})
  await page.waitForTimeout(400)
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
  await page.evaluate(() => {
    localStorage.setItem('zp_lang', 'ru')
    const sel = document.getElementById('lang-select-settings')
    if (sel) {
      sel.value = 'ru'
      sel.dispatchEvent(new Event('change', { bubbles: true }))
    }
  })
  await page.waitForTimeout(400)

  // Overview with compact critical alert
  await page.evaluate(() => window.__PULSECORE_MOCK__?.setProfile?.('A'))
  await page.waitForTimeout(1100)
  await openTab(page, 'overview')
  await page.waitForSelector('#insight-list .insight-card', { timeout: 8000 }).catch(() => {})
  await saveShot(page, 'overview.png')

  // Hardware Dev OFF: hierarchical physical cores (8c/16t)
  await page.evaluate(() => {
    const mock = window.__PULSECORE_MOCK__
    if (mock?.profiles?.B) {
      mock.profiles.B.physicalCores = 8
      mock.profiles.B.logicalCores = 16
      mock.profiles.B.cpuName = 'AMD Ryzen 7 5700X'
      mock.setProfile('B')
    }
  })
  await page.waitForTimeout(800)
  await openTab(page, 'hardware')
  await page.waitForSelector('#hw-cpu-name', { timeout: 5000 })
  await page.waitForSelector('#core-grid .core-phys', { timeout: 5000 }).catch(() => {})
  await saveShot(page, 'hardware.png')
  await copyFile(path.join(MEDIA, 'hardware.png'), path.join(DOCS, 'hardware-16threads.png')).catch(() => {})

  // Dev mode open
  await enableDev(page)
  await openTab(page, 'dev')
  await page.waitForSelector('#tab-dev.tab.active', { timeout: 5000 })
  await page.check('#dev-show-percore').catch(() => {})
  await page.waitForTimeout(300)
  await saveShot(page, 'dev-mode.png')

  // Hardware still shows core/thread grid with Dev on
  await openTab(page, 'hardware')
  await page.waitForSelector('#core-grid .core-phys', { timeout: 5000 }).catch(() => {})
  await saveShot(page, 'hardware-dev-percore.png')

  await openTab(page, 'settings')
  await saveShot(page, 'minihud-settings-ru.png')
  await openTab(page, 'lag')
  await saveShot(page, 'lags.png')

  await browser.close()
  console.log('done')
}

main().catch((err) => {
  console.error(err)
  process.exit(1)
})
