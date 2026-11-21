/**
 * Capture Upgrade Advisor demo (docs) + app mock at desktop and 390 mobile.
 */
import { chromium } from '../frontend/node_modules/playwright/index.mjs'
import { mkdir, copyFile } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const MEDIA = '/cursor/stores/bc-01a1f952-18e1-7825-8cd8-3e318e381534/media'
const ARTIFACTS = '/opt/cursor/artifacts'
const LOCAL = path.join(__dirname, 'screenshots')
const DOCS = process.argv[2] || 'http://127.0.0.1:8765/index.html'
const APP = process.argv[3] || 'http://127.0.0.1:5173/demo.html'

const shots = [
  {
    file: 'upgrade-advisor-desktop-1440x900.png',
    w: 1440, h: 900, kind: 'docs', device: 'pc',
  },
  {
    file: 'upgrade-advisor-mobile-390x844.png',
    w: 390, h: 844, kind: 'docs', device: 'pc',
  },
  {
    file: 'upgrade-advisor-laptop-desktop-1440x900.png',
    w: 1440, h: 900, kind: 'docs', device: 'laptop',
  },
  {
    file: 'upgrade-advisor-app-desktop-1280x800.png',
    w: 1280, h: 800, kind: 'app',
  },
  {
    file: 'upgrade-advisor-app-mobile-390x844.png',
    w: 390, h: 844, kind: 'app',
  },
]

async function prepDocs(page, device) {
  await page.waitForSelector('.preview-tabs button[data-tab="upgrade"]', { timeout: 15000 })
  await page.click('.preview-tabs button[data-tab="upgrade"]')
  await page.waitForTimeout(500)
  await page.evaluate(() => {
    const btn = document.querySelector('.preview-tabs button[data-tab="upgrade"]')
    btn?.scrollIntoView({ inline: 'center', block: 'nearest', behavior: 'auto' })
  })
  await page.waitForTimeout(200)
  if (device === 'laptop') {
    await page.click('.demo-device[data-device="laptop"]')
    await page.waitForTimeout(300)
  } else {
    await page.click('.demo-device[data-device="pc"]').catch(() => {})
    await page.waitForTimeout(200)
  }
  await page.waitForSelector('#demo-upgrade-recs .demo-upgrade-rec', { timeout: 8000 })
  const count = await page.locator('#demo-upgrade-recs .demo-upgrade-rec').count()
  if (count < 2) throw new Error(`expected 2+ upgrade cards, got ${count}`)
  const text = await page.locator('#demo-upgrade-recs').innerText()
  if (/длину корпуса|длина корпуса|case length/i.test(text)) {
    throw new Error('broken case-length tip still present')
  }
  if (device === 'pc' && !/4070 Super/.test(text)) {
    throw new Error('expected RTX 4070 Super on PC FPS goal')
  }
  if (device === 'laptop' && (/4070 Super|5800X3D/.test(text))) {
    throw new Error('laptop should not suggest desktop GPU/CPU upgrades')
  }
  await page.evaluate(() => {
    document.getElementById('preview')?.scrollIntoView({ block: 'start' })
  })
  await page.waitForTimeout(400)
  // Ensure tabs not clipped on mobile: scroll to end of tab row
  await page.evaluate(() => {
    const tabs = document.querySelector('.preview-tabs')
    if (tabs) tabs.scrollLeft = tabs.scrollWidth
  })
  await page.waitForTimeout(200)
  await page.evaluate(() => {
    const tabs = document.querySelector('.preview-tabs')
    if (tabs) tabs.scrollLeft = 0
  })
}

async function prepApp(page) {
  await page.waitForSelector('.nav-item[data-tab="upgrade"]', { timeout: 20000 })
  await page.click('.nav-item[data-tab="upgrade"]')
  await page.waitForSelector('#tab-upgrade.tab.active', { timeout: 8000 })
  await page.waitForSelector('.upgrade-rec', { timeout: 10000 })
  const count = await page.locator('.upgrade-rec').count()
  if (count < 2) throw new Error(`app expected 2+ recs, got ${count}`)
  const text = await page.locator('#upgrade-recs').innerText()
  if (/длину корпуса|длина корпуса/i.test(text)) throw new Error('broken pitfall in app')
  if (!/4070 Super/.test(text) && !/dual-channel|NVMe|undervolt/i.test(text)) {
    throw new Error('app recs missing concrete models')
  }
  await page.waitForTimeout(500)
}

async function main() {
  await mkdir(MEDIA, { recursive: true })
  await mkdir(ARTIFACTS, { recursive: true })
  await mkdir(LOCAL, { recursive: true })
  const browser = await chromium.launch({
    headless: true,
    args: ['--no-sandbox', '--disable-dev-shm-usage'],
  })

  for (const shot of shots) {
    const page = await browser.newPage({
      viewport: { width: shot.w, height: shot.h },
      deviceScaleFactor: 1,
    })
    await page.addInitScript(() => {
      try { localStorage.setItem('zp_lang', 'ru') } catch (_) {}
      try { localStorage.setItem('zp_upgrade_goal', 'max_fps') } catch (_) {}
    })
    if (shot.kind === 'docs') {
      await page.goto(DOCS, { waitUntil: 'networkidle', timeout: 45000 })
      await prepDocs(page, shot.device)
    } else {
      await page.goto(APP, { waitUntil: 'networkidle', timeout: 45000 })
      await prepApp(page)
    }
    const outMedia = path.join(MEDIA, shot.file)
    const outLocal = path.join(LOCAL, shot.file)
    const outArt = path.join(ARTIFACTS, shot.file)
    await page.screenshot({ path: outMedia, fullPage: false })
    await copyFile(outMedia, outLocal)
    await copyFile(outMedia, outArt)
    console.log('saved', shot.file)
    await page.close()
  }

  // Also refresh the live-demo-tab-upgrade screenshot used by verify script
  const page = await browser.newPage({ viewport: { width: 1440, height: 900 } })
  await page.addInitScript(() => { try { localStorage.setItem('zp_lang', 'ru') } catch (_) {} })
  await page.goto(DOCS, { waitUntil: 'networkidle', timeout: 45000 })
  await prepDocs(page, 'pc')
  const tabFile = 'live-demo-tab-upgrade-1440x900.png'
  for (const dir of [LOCAL, MEDIA, ARTIFACTS]) {
    await page.screenshot({ path: path.join(dir, tabFile), fullPage: false })
  }
  console.log('saved', tabFile)

  await browser.close()
  console.log('done')
}

main().catch((err) => {
  console.error(err)
  process.exit(1)
})
