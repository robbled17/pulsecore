/**
 * Headless UI smoke against demo.html mock harness (Vite).
 * Opens every tab, game search, accordion, self-diagnostics, Esc, 7 langs.
 * Fails on console errors or missing critical nodes / obvious overflow.
 */
import { spawn } from 'node:child_process'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { chromium } from 'playwright'

const __dirname = dirname(fileURLToPath(import.meta.url))
const root = join(__dirname, '..')
const PORT = Number(process.env.SMOKE_PORT || 4177)
const BASE = `http://127.0.0.1:${PORT}`
const LANGS = ['ru', 'en', 'kk', 'tt', 'de', 'ja', 'zh']
const TABS = ['overview', 'lag', 'hardware', 'upgrade', 'uninstall', 'gamelab', 'settings']

function startVite() {
  const child = spawn(
    process.platform === 'win32' ? 'npx.cmd' : 'npx',
    ['vite', '--host', '127.0.0.1', '--port', String(PORT), '--strictPort'],
    { cwd: root, stdio: ['ignore', 'pipe', 'pipe'], shell: process.platform === 'win32' },
  )
  let ready = false
  const readyPromise = new Promise((resolve, reject) => {
    const timer = setTimeout(() => reject(new Error('vite start timeout')), 45000)
    const onData = (buf) => {
      const s = String(buf)
      if (/Local:|ready in/i.test(s)) {
        ready = true
        clearTimeout(timer)
        resolve()
      }
    }
    child.stdout.on('data', onData)
    child.stderr.on('data', onData)
    child.on('exit', (code) => {
      if (!ready) {
        clearTimeout(timer)
        reject(new Error(`vite exited early: ${code}`))
      }
    })
  })
  return { child, readyPromise }
}

function overflowSuspects(page) {
  return page.evaluate(() => {
    const bad = []
    const nodes = document.querySelectorAll('.nav, .brand, .about-card, .diag-card, .diag-item')
    for (const el of nodes) {
      if (!(el instanceof HTMLElement)) continue
      if (el.scrollWidth > el.clientWidth + 4) {
        bad.push({ tag: el.tagName, id: el.id, cls: String(el.className), sw: el.scrollWidth, cw: el.clientWidth })
      }
    }
    return bad.slice(0, 8)
  })
}

async function openTab(page, tab) {
  await page.locator(`.nav-item[data-tab="${tab}"]`).click()
  await page.waitForSelector(`#tab-${tab}.tab.active`, { timeout: 8000 })
}

async function main() {
  const { child, readyPromise } = startVite()
  try {
    await readyPromise
    const browser = await chromium.launch({ headless: true })
    const page = await browser.newPage({ viewport: { width: 1180, height: 760 } })
    const consoleErrors = []
    page.on('pageerror', (err) => consoleErrors.push(String(err)))
    page.on('console', (msg) => {
      if (msg.type() === 'error') consoleErrors.push(msg.text())
    })

    try {
      await page.goto(`${BASE}/demo.html`, { waitUntil: 'networkidle', timeout: 60000 })
      await page.waitForSelector('#tab-overview.tab.active', { timeout: 25000 })
      await page.waitForTimeout(400)

      for (const tab of TABS) {
        await openTab(page, tab)
        await page.waitForTimeout(80)
      }

      await openTab(page, 'gamelab')
      const search = page.locator('#game-search')
      if (await search.count()) {
        await search.fill('cyber')
        await page.waitForTimeout(300)
      }

      await openTab(page, 'hardware')
      const details = page.locator('#tab-hardware details.expand-panel, #tab-hardware details').first()
      if (await details.count()) {
        await details.locator('summary').first().click({ force: true }).catch(() => details.click())
        await page.waitForTimeout(120)
      }

      await page.locator('#btn-about').click()
      await page.waitForSelector('#about-modal:not(.hidden)')
      await page.locator('#btn-check-sensors').click()
      await page.waitForSelector('#diag-modal:not(.hidden)')
      await page.waitForSelector('#diag-list .diag-item')
      await page.keyboard.press('Escape')
      await page.waitForFunction(() => document.getElementById('diag-modal')?.classList.contains('hidden'))
      await page.keyboard.press('Escape')
      await page.waitForFunction(() => document.getElementById('about-modal')?.classList.contains('hidden'))
      await page.waitForTimeout(100)

      await openTab(page, 'settings')
      const hotkey = page.locator('#mh-hotkey')
      if (!(await hotkey.count())) throw new Error('Mini-HUD hotkey field missing')
      await hotkey.fill('Ctrl+Shift+H')
      await hotkey.press('Enter')
      await page.waitForTimeout(120)

      for (const lang of LANGS) {
        await page.locator('#lang-select-settings').selectOption(lang)
        await page.waitForTimeout(60)
        if (!(await page.locator('#btn-about').isVisible())) {
          throw new Error(`about btn hidden after lang=${lang}`)
        }
      }

      const overflows = await overflowSuspects(page)
      if (overflows.length) {
        throw new Error(`overflow suspects: ${JSON.stringify(overflows)}`)
      }

      const realErrors = consoleErrors.filter((e) =>
        !/Failed to load resource/i.test(e) &&
        !/net::ERR_/i.test(e) &&
        !/favicon/i.test(e) &&
        !/steamstatic/i.test(e)
      )
      if (realErrors.length) {
        throw new Error(`console errors:\n${realErrors.join('\n')}`)
      }

      console.log('smoke-ui: OK')
    } finally {
      await browser.close()
    }
  } finally {
    child.kill('SIGTERM')
  }
}

main().catch((err) => {
  console.error('smoke-ui: FAIL', err)
  process.exit(1)
})
