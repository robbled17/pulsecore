/**
 * Verify live-demo tab switching + capture each panel screenshot.
 * Usage: node docs/verify-live-demo-tabs.mjs [baseUrl]
 */
import { chromium } from '../frontend/node_modules/playwright/index.mjs'
import { mkdir, copyFile, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const BASE = process.argv[2] || 'http://127.0.0.1:8765/index.html'
const MEDIA = '/cursor/stores/bc-01a1f952-18e1-7825-8cd8-3e318e381534/media'
const ARTIFACTS = '/opt/cursor/artifacts'
const LOCAL = path.join(__dirname, 'screenshots')

const TABS = [
  {
    id: 'overview',
    file: 'live-demo-tab-overview-1440x900.png',
    expectVisible: ['#demo-metrics', '#demo-alert', '#demo-trend'],
    expectHidden: ['#demo-procs', '#demo-games', '#demo-hw-ssd'],
    forbidText: ['SMART'],
  },
  {
    id: 'lags',
    file: 'live-demo-tab-lags-1440x900.png',
    expectVisible: ['#demo-procs', '#panel-lags .btn-eco'],
    expectHidden: ['#demo-metrics', '#demo-games', '#demo-hw-ssd'],
  },
  {
    id: 'gamepulse',
    file: 'live-demo-tab-gamepulse-1440x900.png',
    expectVisible: ['#demo-games'],
    expectHidden: ['#demo-metrics', '#demo-procs', '#demo-hw-ssd'],
    clickGame: true,
  },
  {
    id: 'hardware',
    file: 'live-demo-tab-hardware-1440x900.png',
    expectVisible: ['#demo-hw-ssd', '#demo-core-grid', '#panel-hardware .demo-core'],
    expectHidden: ['#demo-metrics', '#demo-procs', '#demo-games'],
    openSsd: true,
  },
  {
    id: 'upgrade',
    file: 'live-demo-tab-upgrade-1440x900.png',
    expectVisible: ['#demo-upgrade-goals', '#demo-upgrade-device', '#demo-upgrade-recs', '.demo-upgrade-rec'],
    expectHidden: ['#demo-metrics', '#demo-procs', '#demo-games', '#demo-hw-ssd'],
  },
  {
    id: 'uninstall',
    file: 'live-demo-tab-uninstall-1440x900.png',
    expectVisible: ['#demo-residual', '#demo-kill-remove'],
    expectHidden: ['#demo-metrics', '#demo-procs', '#demo-games', '#demo-hw-ssd'],
  },
]

async function assertDisplay(page, sel, wantVisible, label) {
  const info = await page.evaluate((selector) => {
    const el = document.querySelector(selector)
    if (!el) return { missing: true }
    const pane = el.closest('.demo-pane') || el
    const cs = getComputedStyle(pane)
    return {
      missing: false,
      display: cs.display,
      opacity: cs.opacity,
      hidden: pane.hidden,
      active: pane.classList.contains('active'),
      panel: pane.dataset.panel || null,
      id: pane.id || null,
    }
  }, sel)
  if (info.missing) throw new Error(`[${label}] missing ${sel}`)
  const visible = info.display !== 'none' && !info.hidden
  if (wantVisible && !visible) {
    throw new Error(`[${label}] expected visible ${sel}: ${JSON.stringify(info)}`)
  }
  if (!wantVisible && visible && info.active) {
    throw new Error(`[${label}] expected hidden ${sel}: ${JSON.stringify(info)}`)
  }
  return info
}

async function main() {
  await mkdir(MEDIA, { recursive: true })
  await mkdir(ARTIFACTS, { recursive: true })
  await mkdir(LOCAL, { recursive: true })

  const browser = await chromium.launch({ headless: true })
  const page = await browser.newPage({ viewport: { width: 1440, height: 900 } })
  const results = []

  await page.goto(BASE, { waitUntil: 'networkidle', timeout: 30000 })
  await page.waitForSelector('.preview-tabs button[data-tab="overview"]', { timeout: 10000 })
  await page.waitForTimeout(600)

  for (const tab of TABS) {
    await page.click(`.preview-tabs button[data-tab="${tab.id}"]`)
    await page.waitForTimeout(500)
    await page.evaluate(() => {
      document.getElementById('preview')?.scrollIntoView({ block: 'start' })
    })
    await page.waitForTimeout(300)

    const panel = await page.evaluate((id) => {
      const pane = document.querySelector(`.demo-pane[data-panel="${id}"], #panel-${id}`)
      const btn = document.querySelector(`.preview-tabs button[data-tab="${id}"]`)
      if (!pane || !btn) return null
      const cs = getComputedStyle(pane)
      return {
        btnActive: btn.classList.contains('active'),
        paneActive: pane.classList.contains('active'),
        display: cs.display,
        opacity: cs.opacity,
        transform: cs.transform,
        hidden: pane.hidden,
        panel: pane.dataset.panel,
        id: pane.id,
        innerHTMLSnippet: pane.innerText.slice(0, 120).replace(/\s+/g, ' '),
      }
    }, tab.id)

    if (!panel) throw new Error(`panel not found for ${tab.id}`)
    if (!panel.btnActive) throw new Error(`${tab.id}: button not active`)
    if (!panel.paneActive) throw new Error(`${tab.id}: pane not active`)
    if (panel.display === 'none' || panel.hidden) throw new Error(`${tab.id}: pane still hidden ${JSON.stringify(panel)}`)
    if (Number(panel.opacity) < 0.99) throw new Error(`${tab.id}: opacity not visible ${panel.opacity}`)

    for (const sel of tab.expectVisible || []) {
      await assertDisplay(page, sel, true, tab.id)
    }
    for (const sel of tab.expectHidden || []) {
      const el = await page.$(sel)
      if (!el) continue
      await assertDisplay(page, sel, false, tab.id)
    }

    if (tab.forbidText) {
      const text = await page.locator('#panel-overview').innerText()
      for (const bad of tab.forbidText) {
        if (text.includes(bad)) throw new Error(`overview contains forbidden text: ${bad}`)
      }
      // green SMART banner specifically shouldn't exist
      const smartBanner = await page.$('#panel-overview .demo-smart, #panel-overview [data-smart], #demo-smart')
      if (smartBanner) throw new Error('overview has SMART banner element')
    }

    if (tab.clickGame) {
      const game = page.locator('#demo-games .demo-game').first()
      await game.click()
      await page.waitForTimeout(400)
      const fpsVisible = await page.evaluate(() => {
        const fps = document.getElementById('demo-fps')
        return !!(fps && fps.children.length > 0 && getComputedStyle(fps.closest('.demo-pane')).display !== 'none')
      })
      if (!fpsVisible) throw new Error('gamepulse: FPS presets not shown after click')
    }

    if (tab.openSsd) {
      await page.evaluate(() => {
        const acc = document.getElementById('demo-hw-ssd')
        if (acc) acc.open = true
      })
      await page.waitForTimeout(300)
      const cores = await page.evaluate(() => {
        const cards = [...document.querySelectorAll('#demo-core-grid .demo-core')]
        return cards.map((c) => c.innerText.replace(/\s+/g, ' ').slice(0, 80))
      })
      if (cores.length < 1) throw new Error('hardware: no core cards')
      const joined = cores.join(' | ')
      if (!/T0/i.test(joined) || !/T1/i.test(joined)) {
        throw new Error(`hardware: expected T0/T1 labels in cores: ${joined}`)
      }
      const ssdOpen = await page.evaluate(() => !!document.getElementById('demo-hw-ssd')?.open)
      if (!ssdOpen) throw new Error('hardware: SSD accordion not open')
    }

    const shotPath = path.join(LOCAL, tab.file)
    await page.screenshot({ path: shotPath, fullPage: false })
    await copyFile(shotPath, path.join(MEDIA, tab.file))
    await copyFile(shotPath, path.join(ARTIFACTS, tab.file))
    results.push({ tab: tab.id, ok: true, panel, file: tab.file })
    console.log('OK', tab.id, panel.display, 'opacity=' + panel.opacity, '->', tab.file)
  }

  // Also verify inactive panes are display:none when on the last exercised tab
  const lastTabId = TABS[TABS.length - 1].id
  const inactive = await page.evaluate(() => {
    return [...document.querySelectorAll('.demo-pane')].map((p) => ({
      panel: p.dataset.panel,
      active: p.classList.contains('active'),
      display: getComputedStyle(p).display,
      hidden: p.hidden,
    }))
  })
  for (const p of inactive) {
    if (p.panel === lastTabId) {
      if (p.display === 'none') throw new Error(`active ${lastTabId} hidden`)
    } else if (p.display !== 'none') {
      throw new Error(`inactive panel ${p.panel} display=${p.display}`)
    }
  }

  await writeFile(path.join(ARTIFACTS, 'live-demo-tabs-verify.json'), JSON.stringify({ results, inactive }, null, 2))
  await browser.close()
  console.log(JSON.stringify({ pass: true, results, inactive }, null, 2))
}

main().catch((err) => {
  console.error(err)
  process.exit(1)
})
