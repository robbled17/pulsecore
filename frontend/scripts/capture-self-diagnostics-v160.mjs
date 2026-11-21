/**
 * Self-diagnostics + Mini-HUD hotkey screenshots (mock/test data).
 * Saves under Project media/self-diagnostics-v160 and /opt/cursor/artifacts.
 */
import { chromium } from 'playwright'
import { mkdir, copyFile, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { spawn } from 'node:child_process'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const ROOT = path.resolve(__dirname, '..')
const MEDIA = '/cursor/stores/bc-01a1f952-18e1-7825-8cd8-3e318e381534/media/self-diagnostics-v160'
const ARTIFACTS = '/opt/cursor/artifacts'
const PORT = 4188
const BASE = `http://127.0.0.1:${PORT}`

function startVite() {
  const child = spawn('npx', ['vite', '--host', '127.0.0.1', '--port', String(PORT), '--strictPort'], {
    cwd: ROOT, stdio: ['ignore', 'pipe', 'pipe'],
  })
  const ready = new Promise((resolve, reject) => {
    const t = setTimeout(() => reject(new Error('vite timeout')), 45000)
    const on = (b) => {
      if (/Local:|ready in/i.test(String(b))) { clearTimeout(t); resolve() }
    }
    child.stdout.on('data', on)
    child.stderr.on('data', on)
  })
  return { child, ready }
}

async function shot(page, name) {
  const file = `${name}.png`
  const art = path.join(ARTIFACTS, file)
  const med = path.join(MEDIA, file)
  await page.screenshot({ path: art, fullPage: false })
  await copyFile(art, med)
  console.log('saved', med)
}

async function main() {
  await mkdir(MEDIA, { recursive: true })
  await mkdir(ARTIFACTS, { recursive: true })
  const { child, ready } = startVite()
  try {
    await ready
    const browser = await chromium.launch({ headless: true })
    const page = await browser.newPage({ viewport: { width: 1180, height: 760 } })
    await page.goto(`${BASE}/demo.html`, { waitUntil: 'networkidle' })
    await page.waitForSelector('#tab-overview.tab.active', { timeout: 25000 })
    await page.waitForTimeout(500)

    // Label mock data
    await page.evaluate(() => {
      const badge = document.createElement('div')
      badge.textContent = 'MOCK / TEST DATA'
      badge.style.cssText = 'position:fixed;top:8px;right:12px;z-index:9999;background:rgba(0,0,0,.55);color:#fde68a;padding:4px 8px;border-radius:8px;font:650 11px Inter,sans-serif;pointer-events:none'
      document.body.appendChild(badge)
    })

    await page.locator('#btn-about').click()
    await page.waitForSelector('#about-modal:not(.hidden)')
    await page.waitForTimeout(200)
    await shot(page, 'about-check-sensors')

    await page.locator('#btn-check-sensors').click()
    await page.waitForSelector('#diag-modal:not(.hidden)')
    await page.waitForSelector('#diag-list .diag-item')
    await page.waitForTimeout(250)
    await shot(page, 'diagnostics-modal')

    await page.keyboard.press('Escape')
    await page.waitForFunction(() => document.getElementById('diag-modal')?.classList.contains('hidden'))
    await page.keyboard.press('Escape')

    await page.locator('.nav-item[data-tab="settings"]').click()
    await page.waitForSelector('#tab-settings.tab.active')
    await page.locator('#minihud-settings').scrollIntoViewIfNeeded()
    await page.waitForTimeout(200)
    await shot(page, 'settings-minihud-hotkey')

    await writeFile(path.join(MEDIA, 'README.md'), `# Self-diagnostics v1.6.0 screenshots

Mock/test data only (demo.html harness).

- about-check-sensors.png - About dialog with Check sensors
- diagnostics-modal.png - Self-diagnostics results modal
- settings-minihud-hotkey.png - Mini-HUD hotkey field in Settings
`)
    await browser.close()
  } finally {
    child.kill('SIGTERM')
  }
}

main().catch((e) => { console.error(e); process.exit(1) })
