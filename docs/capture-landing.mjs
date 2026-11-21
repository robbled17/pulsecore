/**
 * Headless capture of docs/index.html landing page.
 * Usage: node docs/capture-landing.mjs [baseUrl]
 */
import { chromium } from '../frontend/node_modules/playwright/index.mjs'
import { mkdir, copyFile } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const BASE = process.argv[2] || 'http://127.0.0.1:8765/index.html'
const MEDIA = '/cursor/stores/bc-01a1f952-18e1-7825-8cd8-3e318e381534/media'
const ARTIFACTS = '/opt/cursor/artifacts'
const LOCAL = path.join(__dirname, 'screenshots')

const shots = [
  { file: 'landing-live-demo-1440x900.png', w: 1440, h: 900, prep: 'desktop' },
  { file: 'landing-live-demo-390x844.png', w: 390, h: 844, prep: 'mobile' },
  { file: 'landing-gamepulse-covers-1440x900.png', w: 1440, h: 900, prep: 'gamepulse' },
  { file: 'landing-lang-switcher-1440x900.png', w: 1440, h: 900, prep: 'lang' },
  { file: 'landing-icon-hero-1440x900.png', w: 1440, h: 900, prep: 'icon' },
]

async function prep(page, kind) {
  await page.waitForSelector('.hero h1', { timeout: 10000 })
  await page.waitForSelector('.preview-tab-pill', { timeout: 10000 })
  await page.waitForTimeout(900)
  if (kind === 'gamepulse') {
    await page.click('.preview-tabs button[data-tab="gamepulse"]').catch(() => {})
    await page.waitForTimeout(700)
    await page.evaluate(() => {
      document.getElementById('preview')?.scrollIntoView({ block: 'start' })
    })
    await page.waitForTimeout(400)
  } else if (kind === 'lang') {
    await page.selectOption('#lang-switch', 'en').catch(() => {})
    await page.waitForTimeout(400)
    await page.selectOption('#lang-switch', 'de').catch(() => {})
    await page.waitForTimeout(350)
    await page.evaluate(() => window.scrollTo(0, 0))
  } else if (kind === 'icon') {
    await page.evaluate(() => window.scrollTo(0, 0))
    await page.waitForTimeout(300)
  } else if (kind === 'desktop') {
    await page.click('.preview-tabs button[data-tab="hardware"]').catch(() => {})
    await page.waitForTimeout(500)
    await page.click('.preview-tabs button[data-tab="overview"]').catch(() => {})
    await page.waitForTimeout(500)
  } else if (kind === 'mobile') {
    await page.click('.preview-tabs button[data-tab="gamepulse"]').catch(() => {})
    await page.waitForTimeout(500)
    await page.click('.preview-tabs button[data-tab="overview"]').catch(() => {})
    await page.waitForTimeout(400)
  }
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
    await page.goto(BASE, { waitUntil: 'networkidle', timeout: 45000 })
    await prep(page, shot.prep)
    const outMedia = path.join(MEDIA, shot.file)
    const outLocal = path.join(LOCAL, shot.file)
    const outArt = path.join(ARTIFACTS, shot.file)
    await page.screenshot({ path: outMedia, fullPage: false })
    await copyFile(outMedia, outLocal)
    await copyFile(outMedia, outArt)
    console.log('saved', shot.file)
    await page.close()
  }

  await browser.close()
  console.log('done')
}

main().catch((err) => {
  console.error(err)
  process.exit(1)
})
