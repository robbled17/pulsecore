/**
 * Final release v1.5.0 screenshots (mock/test data only).
 * App tabs + docs landing desktop/mobile.
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
const MEDIA = '/cursor/stores/bc-01a1f952-18e1-7825-8cd8-3e318e381534/media/final-release-v150'
const ARTIFACTS = '/opt/cursor/artifacts'
const APP_TABS = [
  { id: 'overview', file: 'app-overview' },
  { id: 'lag', file: 'app-lags' },
  { id: 'hardware', file: 'app-hardware-ssd' },
  { id: 'upgrade', file: 'app-upgrade' },
  { id: 'uninstall', file: 'app-uninstall' },
  { id: 'gamelab', file: 'app-gamepulse' },
  { id: 'settings', file: 'app-settings' },
]

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
  await page.waitForTimeout(500)
}

async function expandSSD(page) {
  await page.evaluate(async () => {
    const list = document.getElementById('hw-ssd-list')
    const first = list?.querySelector('.hw-ssd-acc')
    if (first) {
      first.open = true
      first.dispatchEvent(new Event('toggle', { bubbles: true }))
    }
  })
  await page.waitForTimeout(900)
  // wait for SMART rows
  await page.waitForFunction(() => {
    const body = document.querySelector('.hw-ssd-acc[open] [data-ssd-body] .row strong')
    return !!body
  }, { timeout: 8000 }).catch(() => {})
  await page.waitForTimeout(400)
}

async function main() {
  await mkdir(MEDIA, { recursive: true })
  await mkdir(ARTIFACTS, { recursive: true })

  // Use Vite mock for app (demo.html). Docs via static server.
  const docsServer = await staticServer(path.join(ROOT, 'docs'), 4177)
  const appBase = process.argv[2] || 'http://127.0.0.1:5173/demo.html'
  const docsURL = 'http://127.0.0.1:4177/'

  const browser = await chromium.launch({
    headless: true,
    args: ['--no-sandbox', '--disable-dev-shm-usage'],
  })
  const manifest = []

  // App screenshots
  {
    const page = await browser.newPage({ viewport: { width: 1280, height: 900 }, deviceScaleFactor: 1 })
    await page.goto(appBase, { waitUntil: 'networkidle', timeout: 60000 })
    await waitApp(page)
    await page.evaluate(() => window.__PULSECORE_MOCK__?.setProfile?.('B'))
    await page.waitForTimeout(800)
    await page.evaluate(() => {
      localStorage.setItem('zp_lang', 'ru')
      const sel = document.getElementById('lang-select-settings')
      if (sel) { sel.value = 'ru'; sel.dispatchEvent(new Event('change', { bubbles: true })) }
    })
    await page.waitForTimeout(500)

    for (const tab of APP_TABS) {
      await openTab(page, tab.id)
      if (tab.id === 'hardware') await expandSSD(page)
      if (tab.id === 'upgrade') {
        await page.waitForTimeout(600)
        await page.evaluate(() => document.getElementById('btn-upgrade-refresh')?.click())
        await page.waitForTimeout(800)
      }
      const name = `${tab.file}-testdata-1280x900.png`
      const out = path.join(MEDIA, name)
      const art = path.join(ARTIFACTS, name)
      await page.screenshot({ path: out, fullPage: false })
      await copyFile(out, art)
      manifest.push({ kind: 'app', tab: tab.id, path: out })
      console.log('saved', out)
    }
    await page.close()
  }

  // Docs landing desktop + mobile
  for (const vp of [
    { name: 'docs-landing-desktop', width: 1440, height: 900 },
    { name: 'docs-landing-mobile', width: 390, height: 844 },
  ]) {
    const page = await browser.newPage({
      viewport: { width: vp.width, height: vp.height },
      deviceScaleFactor: 1,
    })
    await page.goto(docsURL, { waitUntil: 'networkidle', timeout: 60000 })
    await page.waitForTimeout(1200)
    // exercise live demo tabs briefly so content is painted
    await page.evaluate(() => {
      const tabs = [...document.querySelectorAll('.demo-tabs [data-tab], .live-demo [data-tab], [role="tab"][data-tab]')]
      tabs.slice(0, 6).forEach((b, i) => setTimeout(() => b.click(), i * 80))
    })
    await page.waitForTimeout(900)
    // back to overview for the shot
    await page.evaluate(() => {
      document.querySelector('[data-tab="overview"]')?.click()
    })
    await page.waitForTimeout(500)
    const name = `${vp.name}-testdata-${vp.width}x${vp.height}.png`
    const out = path.join(MEDIA, name)
    const art = path.join(ARTIFACTS, name)
    await page.screenshot({ path: out, fullPage: false })
    await copyFile(out, art)
    manifest.push({ kind: 'docs', path: out })
    console.log('saved', out)
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
