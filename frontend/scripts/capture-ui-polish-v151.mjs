/**
 * UI polish v1.5.1 screenshots (mock/test data only).
 * Required: app Lags / GamePulse / Settings + docs Dynamics / Upgrade pitfalls / Uninstall buttons.
 */
import { chromium } from 'playwright'
import { mkdir, copyFile, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { createServer } from 'node:http'
import { readFileSync, existsSync, statSync } from 'node:fs'
import { extname } from 'node:path'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const ROOT = path.resolve(__dirname, '../..')
const MEDIA = '/cursor/stores/bc-01a1f952-18e1-7825-8cd8-3e318e381534/media/ui-polish-v151'
const ARTIFACTS = '/opt/cursor/artifacts'

const MIME = {
  '.html': 'text/html; charset=utf-8',
  '.js': 'text/javascript; charset=utf-8',
  '.css': 'text/css; charset=utf-8',
  '.svg': 'image/svg+xml',
  '.png': 'image/png',
  '.webp': 'image/webp',
  '.json': 'application/json',
  '.ico': 'image/x-icon',
}

function staticServer(dir, port) {
  return new Promise((resolve) => {
    const server = createServer((req, res) => {
      let urlPath = decodeURIComponent((req.url || '/').split('?')[0])
      if (urlPath.endsWith('/')) urlPath += 'index.html'
      const file = path.join(dir, urlPath.replace(/^\//, ''))
      if (!file.startsWith(dir) || !existsSync(file) || statSync(file).isDirectory()) {
        res.writeHead(404); res.end('not found'); return
      }
      const ext = extname(file)
      res.writeHead(200, { 'Content-Type': MIME[ext] || 'application/octet-stream' })
      res.end(readFileSync(file))
    })
    server.listen(port, '127.0.0.1', () => resolve(server))
  })
}

async function waitApp(page) {
  await page.waitForSelector('#tab-overview.tab.active', { timeout: 25000 })
  await page.waitForFunction(() => {
    const el = document.getElementById('ov-cpu')
    return el && el.textContent && el.textContent.trim() !== '-'
  }, { timeout: 15000 }).catch(() => {})
  await page.waitForTimeout(700)
}

async function openTab(page, tab) {
  await page.evaluate((t) => {
    document.querySelector(`.nav-item[data-tab="${t}"]`)?.click()
  }, tab)
  await page.waitForSelector(`#tab-${tab}.tab.active`, { timeout: 8000 })
  await page.waitForTimeout(600)
}

async function saveShot(page, file, manifest, kind) {
  const out = path.join(MEDIA, file)
  const art = path.join(ARTIFACTS, file)
  await page.screenshot({ path: out, fullPage: false })
  await copyFile(out, art)
  manifest.push({ kind, path: out, note: 'mock/test data' })
  console.log('saved', out)
}

async function main() {
  await mkdir(MEDIA, { recursive: true })
  await mkdir(ARTIFACTS, { recursive: true })

  const docsServer = await staticServer(path.join(ROOT, 'docs'), 4178)
  const appBase = process.argv[2] || 'http://127.0.0.1:5173/demo.html'
  const docsURL = 'http://127.0.0.1:4178/'

  const browser = await chromium.launch({
    headless: true,
    args: ['--no-sandbox', '--disable-dev-shm-usage'],
  })
  const manifest = []

  {
    const page = await browser.newPage({ viewport: { width: 1280, height: 900 }, deviceScaleFactor: 1 })
    await page.goto(appBase, { waitUntil: 'networkidle', timeout: 60000 })
    await waitApp(page)
    await page.evaluate(() => window.__PULSECORE_MOCK__?.setProfile?.('B'))
    await page.waitForTimeout(600)
    await page.evaluate(() => {
      localStorage.setItem('zp_lang', 'ru')
      const sel = document.getElementById('lang-select-settings')
      if (sel) { sel.value = 'ru'; sel.dispatchEvent(new Event('change', { bubbles: true })) }
    })
    await page.waitForTimeout(400)

    await openTab(page, 'lag')
    await saveShot(page, 'app-lags-testdata-1280x900.png', manifest, 'app-lags')

    await openTab(page, 'gamelab')
    await page.waitForFunction(() => {
      const grid = document.getElementById('game-grid')
      return grid && grid.querySelectorAll('.game-card').length >= 12
    }, { timeout: 12000 }).catch(() => {})
    await page.waitForTimeout(800)
    await saveShot(page, 'app-gamepulse-full-grid-testdata-1280x900.png', manifest, 'app-gamepulse')

    await openTab(page, 'settings')
    await page.waitForTimeout(400)
    await saveShot(page, 'app-settings-reset-header-testdata-1280x900.png', manifest, 'app-settings')
    await page.close()
  }

  {
    const page = await browser.newPage({ viewport: { width: 1440, height: 900 }, deviceScaleFactor: 1 })
    await page.goto(docsURL, { waitUntil: 'networkidle', timeout: 60000 })
    await page.waitForTimeout(1000)
    await page.evaluate(() => document.querySelector('[data-tab="overview"]')?.click())
    await page.waitForTimeout(900)
    const dyn = await page.$('#panel-overview .demo-chart')
    if (dyn) {
      await dyn.scrollIntoViewIfNeeded()
      await page.waitForTimeout(300)
      const box = await dyn.boundingBox()
      if (box) {
        const out = path.join(MEDIA, 'docs-dynamics-legend-scale-testdata-1440x900.png')
        const art = path.join(ARTIFACTS, 'docs-dynamics-legend-scale-testdata-1440x900.png')
        await page.screenshot({
          path: out,
          clip: {
            x: Math.max(0, box.x - 12),
            y: Math.max(0, box.y - 48),
            width: Math.min(1400, box.width + 24),
            height: Math.min(420, box.height + 72),
          },
        })
        await copyFile(out, art)
        manifest.push({ kind: 'docs-dynamics', path: out, note: 'mock/test data' })
        console.log('saved', out)
      }
    } else {
      await saveShot(page, 'docs-dynamics-legend-scale-testdata-1440x900.png', manifest, 'docs-dynamics')
    }

    await page.evaluate(() => document.querySelector('[data-tab="upgrade"]')?.click())
    await page.waitForTimeout(700)
    const pit = await page.$('#panel-upgrade .demo-upgrade-recs')
    if (pit) {
      await pit.scrollIntoViewIfNeeded()
      await page.waitForTimeout(300)
      const box = await pit.boundingBox()
      if (box) {
        const out = path.join(MEDIA, 'docs-upgrade-pitfalls-testdata-1440x900.png')
        const art = path.join(ARTIFACTS, 'docs-upgrade-pitfalls-testdata-1440x900.png')
        await page.screenshot({
          path: out,
          clip: {
            x: Math.max(0, box.x - 8),
            y: Math.max(0, box.y - 80),
            width: Math.min(1200, box.width + 16),
            height: Math.min(520, box.height + 100),
          },
        })
        await copyFile(out, art)
        manifest.push({ kind: 'docs-upgrade', path: out, note: 'mock/test data' })
        console.log('saved', out)
      }
    }

    await page.evaluate(() => document.querySelector('[data-tab="uninstall"]')?.click())
    await page.waitForTimeout(500)
    // Capture both styled buttons: primary action + replay after demo completes.
    await page.evaluate(() => document.getElementById('demo-kill-remove')?.click())
    await page.waitForTimeout(2200)
    const panel = await page.$('#panel-uninstall')
    if (panel) {
      await panel.scrollIntoViewIfNeeded()
      const box = await panel.boundingBox()
      const out = path.join(MEDIA, 'docs-uninstall-buttons-testdata-1440x900.png')
      const art = path.join(ARTIFACTS, 'docs-uninstall-buttons-testdata-1440x900.png')
      if (box) {
        await page.screenshot({
          path: out,
          clip: {
            x: Math.max(0, box.x - 8),
            y: Math.max(0, box.y - 8),
            width: Math.min(900, box.width + 16),
            height: Math.min(520, box.height + 16),
          },
        })
      } else {
        await page.screenshot({ path: out, fullPage: false })
      }
      await copyFile(out, art)
      manifest.push({ kind: 'docs-uninstall', path: out, note: 'mock/test data' })
      console.log('saved', out)
    }
    await page.close()
  }

  await writeFile(path.join(MEDIA, 'manifest.json'), JSON.stringify(manifest, null, 2))
  await browser.close()
  docsServer.close()
  console.log('done', manifest.length)
}

main().catch((err) => {
  console.error(err)
  process.exit(1)
})
