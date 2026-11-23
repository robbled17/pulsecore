/**
 * Capture full-audit-v1-polish screenshots into the Project media folder.
 */
import { chromium } from 'playwright'
import { mkdir } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { spawn } from 'node:child_process'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const FRONTEND = path.resolve(__dirname, '..')
const OUT = '/cursor/stores/bc-01a1f952-18e1-7825-8cd8-3e318e381534/media/full-audit-v1-polish'
const PORT = 5197
const VW = 1280
const VH = 900

function startVite() {
  return spawn(
    process.platform === 'win32' ? 'npx.cmd' : 'npx',
    ['vite', '--host', '127.0.0.1', '--port', String(PORT), '--strictPort'],
    {
      cwd: FRONTEND,
      stdio: ['ignore', 'pipe', 'pipe'],
      shell: process.platform === 'win32',
      windowsHide: true,
      env: { ...process.env },
    },
  )
}

async function waitForServer(url, timeoutMs = 60000) {
  const start = Date.now()
  let lastErr = ''
  while (Date.now() - start < timeoutMs) {
    try {
      const res = await fetch(url, { signal: AbortSignal.timeout(2000) })
      if (res.ok || res.status === 404) return
      lastErr = `status ${res.status}`
    } catch (err) {
      lastErr = String(err?.message || err)
    }
    await new Promise((r) => setTimeout(r, 300))
  }
  throw new Error(`vite not ready: ${lastErr}`)
}

function killChild(child) {
  if (!child || child.killed) return
  try {
    child.kill('SIGTERM')
    setTimeout(() => { try { child.kill('SIGKILL') } catch (_) {} }, 1200)
  } catch (_) {}
}

async function preparePage(page) {
  await page.goto(`http://127.0.0.1:${PORT}/demo.html`, { waitUntil: 'load', timeout: 60000 })
  await page.waitForSelector('#tab-overview.tab.active', { timeout: 25000 })
  await page.evaluate(() => {
    document.documentElement.style.colorScheme = 'dark'
    document.body.style.background = '#0b0f19'
    document.getElementById('dev-mode-badge')?.classList.add('hidden')
    document.querySelectorAll('.dev-mode-nav').forEach((n) => n.classList.add('hidden'))
    // Prefer a mid-tier discrete profile when mock profile buttons exist.
    document.querySelector('.mock-profile-btn[data-profile="C"]')?.click()
    document.querySelector('.mock-profile-btn[data-profile="B"]')?.click()
  })
  await page.waitForFunction(() => {
    const el = document.getElementById('ov-cpu')
    return el && el.textContent && el.textContent.trim() !== '-'
  }, { timeout: 20000 }).catch(() => {})
  await page.waitForTimeout(700)
}

async function openTab(page, tab) {
  await page.evaluate((t) => {
    document.querySelector(`.nav-item[data-tab="${t}"]`)?.click()
  }, tab)
  await page.waitForSelector(`#tab-${tab}.tab.active`, { timeout: 10000 })
  await page.waitForTimeout(600)
}

async function shellShot(page, file) {
  const shell = page.locator('.app-shell, .shell, #app, .layout').first()
  if (await shell.count()) {
    await shell.screenshot({ path: file })
  } else {
    await page.screenshot({ path: file, fullPage: false })
  }
}

async function main() {
  await mkdir(OUT, { recursive: true })
  const child = startVite()
  let browser
  try {
    await waitForServer(`http://127.0.0.1:${PORT}/demo.html`)
    browser = await chromium.launch({ headless: true })
    const page = await browser.newPage({ viewport: { width: VW, height: VH } })
    await preparePage(page)

    await openTab(page, 'overview')
    await page.waitForTimeout(500)
    await shellShot(page, path.join(OUT, 'overview.png'))

    await openTab(page, 'lag')
    await page.evaluate(async () => {
      const btn = document.getElementById('btn-net-check') || document.querySelector('[data-i18n="netCheck"]')
      btn?.click()
    })
    await page.waitForTimeout(1200)
    await shellShot(page, path.join(OUT, 'lags.png'))

    await openTab(page, 'uninstall')
    await page.waitForTimeout(500)
    await shellShot(page, path.join(OUT, 'uninstall.png'))

    await openTab(page, 'gamelab')
    await page.waitForTimeout(400)
    await page.fill('#game-search', 'rdr2')
    await page.waitForTimeout(900)
    await page.evaluate(() => {
      const card = document.querySelector('.game-card[data-game="rdr2"], .game-card[data-game*="rdr"]')
        || document.querySelector('.game-card')
      card?.click()
    })
    await page.waitForTimeout(1200)
    await shellShot(page, path.join(OUT, 'gamepulse-rdr2.png'))

    console.log('captured:', OUT)
  } finally {
    if (browser) await browser.close().catch(() => {})
    killChild(child)
  }
}

main().catch((err) => {
  console.error(err)
  process.exit(1)
})
