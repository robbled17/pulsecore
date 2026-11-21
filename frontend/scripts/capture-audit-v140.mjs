/**
 * Audit v1.4.0 screenshots: all tabs × RU+DE × desktop+narrow (test data).
 * Labels files with -testdata- suffix.
 */
import { chromium } from 'playwright'
import { mkdir, copyFile, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const BASE = process.argv[2] || 'http://127.0.0.1:5173/demo.html'
const MEDIA = '/cursor/stores/bc-01a1f952-18e1-7825-8cd8-3e318e381534/media'
const ARTIFACTS = '/opt/cursor/artifacts'
const TABS = ['overview', 'lag', 'hardware', 'upgrade', 'uninstall', 'gamelab', 'settings']
const LANGS = ['ru', 'de']
const VIEWPORTS = [
  { name: 'desktop', width: 1440, height: 900 },
  { name: 'narrow', width: 390, height: 844 },
]

async function waitReady(page) {
  await page.waitForSelector('#tab-overview.tab.active', { timeout: 20000 })
  await page.waitForFunction(() => {
    const el = document.getElementById('ov-cpu')
    return el && el.textContent && el.textContent.trim() !== '-'
  }, { timeout: 15000 }).catch(() => {})
  await page.waitForTimeout(900)
}

async function setLang(page, lang) {
  await page.evaluate((l) => {
    localStorage.setItem('zp_lang', l)
    const sel = document.getElementById('lang-select-settings')
    if (sel) {
      sel.value = l
      sel.dispatchEvent(new Event('change', { bubbles: true }))
    }
  }, lang)
  await page.waitForTimeout(400)
}

async function openTab(page, tab) {
  // Prefer JS activation so Mini-HUD / narrow layout cannot block clicks.
  const ok = await page.evaluate((t) => {
    const btn = document.querySelector(`.nav-item[data-tab="${t}"]`)
    if (!btn) return false
    btn.click()
    return true
  }, tab)
  if (!ok) return false
  await page.waitForSelector(`#tab-${tab}.tab.active`, { timeout: 8000 })
  await page.waitForTimeout(450)
  return true
}

async function main() {
  await mkdir(MEDIA, { recursive: true })
  await mkdir(ARTIFACTS, { recursive: true })

  const browser = await chromium.launch({
    headless: true,
    args: ['--no-sandbox', '--disable-dev-shm-usage'],
  })

  const manifest = []

  for (const vp of VIEWPORTS) {
    const page = await browser.newPage({
      viewport: { width: vp.width, height: vp.height },
      deviceScaleFactor: 1,
    })
    await page.goto(BASE, { waitUntil: 'networkidle', timeout: 45000 })
    await waitReady(page)
    await page.evaluate(() => window.__PULSECORE_MOCK__?.setProfile?.('B'))
    await page.waitForTimeout(700)

    for (const lang of LANGS) {
      await setLang(page, lang)
      for (const tab of TABS) {
        const ok = await openTab(page, tab)
        if (!ok) continue
        // Expand first SMART/disk accordion if present on overview/hardware
        if (tab === 'overview' || tab === 'hardware') {
          await page.locator('details.expand, .disk-card, .periph-group-title').first().click({ timeout: 1500 }).catch(() => {})
        }
        if (tab === 'settings') {
          await page.click('#btn-hud-title').catch(() => {})
          await page.waitForTimeout(300)
        }
        const file = `audit-v140-testdata-${tab}-${lang}-${vp.name}-${vp.width}x${vp.height}.png`
        const outMedia = path.join(MEDIA, file)
        const outArt = path.join(ARTIFACTS, file)
        await page.screenshot({ path: outMedia, fullPage: false })
        await copyFile(outMedia, outArt)
        manifest.push({ file, tab, lang, viewport: vp.name, label: 'test data' })
        console.log('saved', file)
      }
      // Mini-HUD shot then dismiss so next language can navigate
      await page.evaluate(async () => {
        try { await window.go?.main?.App?.ToggleMiniHUD?.(true) } catch (_) {}
      })
      await page.waitForTimeout(400)
      const hudFile = `audit-v140-testdata-minihud-${lang}-${vp.name}-${vp.width}x${vp.height}.png`
      await page.screenshot({ path: path.join(MEDIA, hudFile), fullPage: false })
      await copyFile(path.join(MEDIA, hudFile), path.join(ARTIFACTS, hudFile))
      manifest.push({ file: hudFile, tab: 'minihud', lang, viewport: vp.name, label: 'test data' })
      console.log('saved', hudFile)
      await page.evaluate(async () => {
        try { await window.go?.main?.App?.ToggleMiniHUD?.(false) } catch (_) {}
        document.getElementById('minihud')?.classList.add('hidden')
      })
      await page.waitForTimeout(200)
    }
    await page.close()
  }

  await writeFile(path.join(MEDIA, 'audit-v140-screenshots.json'), JSON.stringify(manifest, null, 2))
  await writeFile(path.join(ARTIFACTS, 'audit-v140-screenshots.json'), JSON.stringify(manifest, null, 2))
  await browser.close()
  console.log('done', manifest.length, 'shots')
}

main().catch((err) => {
  console.error(err)
  process.exit(1)
})
