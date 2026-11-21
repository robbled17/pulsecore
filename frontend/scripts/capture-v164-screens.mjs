/**
 * Capture required v1.6.4 screenshots:
 * - about-check-updates.png
 * - update-banner.png
 */
import { chromium } from 'playwright'
import { mkdir, copyFile, writeFile, access } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { spawn } from 'node:child_process'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const ROOT = path.resolve(__dirname, '..')
const MEDIA = '/cursor/stores/self/media/overflow-update-v164'
const ARTIFACTS = '/opt/cursor/artifacts'
const PORT = 5188

function startVite() {
  return spawn(
    process.platform === 'win32' ? 'npx.cmd' : 'npx',
    ['vite', '--host', '127.0.0.1', '--port', String(PORT), '--strictPort'],
    {
      cwd: ROOT,
      stdio: ['ignore', 'pipe', 'pipe'],
      shell: process.platform === 'win32',
      windowsHide: true,
    },
  )
}

async function waitForServer(url, timeoutMs = 45000) {
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

async function main() {
  await mkdir(MEDIA, { recursive: true })
  await mkdir(ARTIFACTS, { recursive: true })

  const child = startVite()
  let browser
  try {
    await waitForServer(`http://127.0.0.1:${PORT}/demo.html`)
    browser = await chromium.launch({
      headless: true,
      args: ['--no-sandbox', '--disable-dev-shm-usage'],
    })

    // 1) About modal with Check for updates
    {
      const page = await browser.newPage({ viewport: { width: 1180, height: 760 }, deviceScaleFactor: 1 })
      await page.goto(`http://127.0.0.1:${PORT}/demo.html`, { waitUntil: 'load', timeout: 60000 })
      await page.waitForSelector('#tab-overview.tab.active', { timeout: 25000 })
      await page.locator('#btn-about').click()
      await page.waitForSelector('#about-modal:not(.hidden)')
      await page.waitForSelector('#btn-check-updates')
      await page.waitForTimeout(250)
      const out = path.join(MEDIA, 'about-check-updates.png')
      await page.locator('#about-modal .about-card').screenshot({ path: out })
      await copyFile(out, path.join(ARTIFACTS, 'about-check-updates.png'))
      console.log('saved', out)
      await page.close()
    }

    // 2) Update banner with forced available update
    {
      const page = await browser.newPage({ viewport: { width: 1180, height: 760 }, deviceScaleFactor: 1 })
      await page.addInitScript(() => {
        window.__PULSECORE_FORCE_UPDATE__ = 'available'
      })
      await page.goto(`http://127.0.0.1:${PORT}/demo.html`, { waitUntil: 'load', timeout: 60000 })
      await page.waitForSelector('#tab-overview.tab.active', { timeout: 25000 })
      // Auto-check is ~3s; force immediately for capture reliability
      await page.evaluate(async () => {
        window.__PULSECORE_FORCE_UPDATE__ = 'available'
        const res = await window.go.main.App.CheckForUpdates()
        const banner = document.getElementById('update-banner')
        const text = document.getElementById('update-banner-text')
        if (banner && text && res?.status === 'update_available') {
          const ver = `v${String(res.latestVersion || '').replace(/^v/i, '')}`
          text.textContent = (window.__i18nBanner || `Доступна новая версия PulseCore ${ver}`)
          // Prefer live i18n if boot already applied
          const ru = document.documentElement.lang === 'ru'
          text.textContent = ru
            ? `Доступна новая версия PulseCore ${ver}`
            : `New PulseCore version available: ${ver}`
          banner.dataset.downloadUrl = res.downloadURL || ''
          banner.classList.remove('hidden')
        }
      })
      await page.waitForSelector('#update-banner:not(.hidden)', { timeout: 8000 })
      await page.waitForTimeout(200)
      const out = path.join(MEDIA, 'update-banner.png')
      await page.screenshot({ path: out, fullPage: false })
      await copyFile(out, path.join(ARTIFACTS, 'update-banner.png'))
      console.log('saved', out)
      await page.close()
    }

    await writeFile(path.join(MEDIA, 'README.md'), `# Overflow + update checker v1.6.4 screenshots

- \`about-check-updates.png\` - About modal with Check for updates
- \`update-banner.png\` - New-version notification banner
`)

    for (const name of ['about-check-updates.png', 'update-banner.png']) {
      await access(path.join(MEDIA, name))
      console.log('verified', path.join(MEDIA, name))
    }
  } finally {
    if (browser) await browser.close().catch(() => {})
    killChild(child)
  }
}

main().catch((err) => {
  console.error(err)
  process.exit(1)
})
