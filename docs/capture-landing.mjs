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
  { file: 'landing-polish-v2-1440x900.png', w: 1440, h: 900 },
  { file: 'landing-polish-v2-390x844.png', w: 390, h: 844 },
]

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
    await page.goto(BASE, { waitUntil: 'networkidle', timeout: 30000 })
    await page.waitForSelector('.hero h1', { timeout: 10000 })
    await page.waitForSelector('.preview-tab-pill', { timeout: 10000 })
    await page.waitForTimeout(1100)
    // Exercise tab switch once on desktop for richer state (pill + fade)
    if (shot.w >= 1000) {
      await page.click('.preview-tabs button[data-tab="gamepulse"]').catch(() => {})
      await page.waitForTimeout(520)
      await page.click('.preview-tabs button[data-tab="overview"]').catch(() => {})
      await page.waitForTimeout(520)
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

  await browser.close()
  console.log('done')
}

main().catch((err) => {
  console.error(err)
  process.exit(1)
})
