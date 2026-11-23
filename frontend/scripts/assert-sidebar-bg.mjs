/**
 * Assert sidebar and content share the same computed backgroundColor.
 * Walks parents on Uninstall + Overview; samples painted pixels in gaps.
 * Usage: node scripts/assert-sidebar-bg.mjs [baseUrl] [screenshotPath]
 */
import { chromium } from 'playwright'
import { mkdir } from 'node:fs/promises'
import path from 'node:path'

const BASE = process.argv[2] || 'http://127.0.0.1:5173/demo.html'
const SHOT =
  process.argv[3] ||
  '/cursor/stores/bc-01a1f952-18e1-7825-8cd8-3e318e381534/media/match-sidebar-bg/uninstall.png'

const TABS = ['overview', 'lag', 'hardware', 'upgrade', 'uninstall', 'gamelab', 'settings']

async function openTab(page, tab) {
  const dataTab = tab === 'lag' ? 'lag' : tab
  await page.click(`.nav-item[data-tab="${dataTab}"]`)
  await page.waitForSelector(`#tab-${dataTab}.tab.active`, { timeout: 8000 })
  await page.waitForTimeout(500)
}

async function probeTab(page, tab) {
  return page.evaluate((tabName) => {
    const sidebar = document.querySelector('.sidebar')
    const content = document.querySelector('main.content, .content')
    if (!sidebar || !content) throw new Error('missing sidebar/content')

    const sidebarBg = getComputedStyle(sidebar).backgroundColor
    const contentBg = getComputedStyle(content).backgroundColor
    const contentImg = getComputedStyle(content).backgroundImage
    const sidebarImg = getComputedStyle(sidebar).backgroundImage

    const tabEl = document.querySelector(`#tab-${tabName}`)
    const card =
      tabEl?.querySelector('.panel, .metric-card, .status-strip, .game-card') ||
      content.querySelector('.panel, .metric-card')

    const chain = []
    let el = card
    while (el && el.nodeType === 1) {
      const cs = getComputedStyle(el)
      const before = getComputedStyle(el, '::before')
      const after = getComputedStyle(el, '::after')
      chain.push({
        node: `${el.tagName.toLowerCase()}${el.id ? '#' + el.id : ''}${
          el.className && typeof el.className === 'string'
            ? '.' + el.className.trim().split(/\s+/).slice(0, 3).join('.')
            : ''
        }`,
        backgroundColor: cs.backgroundColor,
        backgroundImage: cs.backgroundImage.slice(0, 100),
        beforeBg: before.backgroundColor,
        beforeImg: before.backgroundImage !== 'none' ? before.backgroundImage.slice(0, 80) : 'none',
        afterBg: after.backgroundColor,
        afterImg: after.backgroundImage !== 'none' ? after.backgroundImage.slice(0, 80) : 'none',
      })
      el = el.parentElement
    }

    // First opaque ancestor behind gaps (excluding the card itself)
    let gapPainter = null
    for (const row of chain.slice(1)) {
      const opaque =
        row.backgroundColor &&
        row.backgroundColor !== 'rgba(0, 0, 0, 0)' &&
        row.backgroundColor !== 'transparent'
      const hasImg = row.backgroundImage && row.backgroundImage !== 'none'
      if (opaque || hasImg) {
        gapPainter = row
        break
      }
    }

    const gap = getComputedStyle(tabEl || content).gap

    return {
      tab: tabName,
      sidebarBg,
      contentBg,
      sidebarImg,
      contentImg,
      match: sidebarBg === contentBg,
      gap,
      gapPainter,
      chain,
    }
  }, tab)
}

async function paintedMatch(page) {
  const coords = await page.evaluate(() => {
    const sidebar = document.querySelector('.sidebar')
    const cards = [...document.querySelectorAll('#tab-uninstall.tab.active .panel')].filter(
      (el) => {
        const r = el.getBoundingClientRect()
        return r.height > 8 && r.width > 8
      },
    )
    if (!sidebar || cards.length < 2) return null
    const sb = sidebar.getBoundingClientRect()
    const a = cards[0].getBoundingClientRect()
    const b = cards[1].getBoundingClientRect()
    let gx, gy
    if (b.top > a.bottom + 1) {
      gx = (a.left + a.right) / 2
      gy = (a.bottom + b.top) / 2
    } else {
      gx = (a.right + b.left) / 2
      gy = (a.top + a.bottom) / 2
    }
    // Prefer empty sidebar chrome (not nav text / buttons)
    const sideX = sb.left + 20
    const sideY = sb.top + 10
    const pad = document.querySelector('main.content')?.getBoundingClientRect()
    return {
      sidebar: { x: sideX, y: sideY },
      gap: { x: gx, y: gy, size: b.top - a.bottom },
      contentPad: pad ? { x: pad.right - 24, y: a.top - 8 } : null,
      hit: document.elementFromPoint(gx, gy)?.className || null,
    }
  })
  if (!coords) return { error: 'no coords' }

  const cdp = await page.context().newCDPSession(page)
  async function pixelAt(x, y) {
    const { data } = await cdp.send('Page.captureScreenshot', {
      format: 'png',
      clip: { x: Math.floor(x), y: Math.floor(y), width: 1, height: 1, scale: 1 },
      fromSurface: true,
    })
    return page.evaluate(async (b64) => {
      const img = new Image()
      img.src = 'data:image/png;base64,' + b64
      await img.decode()
      const c = document.createElement('canvas')
      c.width = 1
      c.height = 1
      const ctx = c.getContext('2d')
      ctx.drawImage(img, 0, 0)
      const [r, g, b, a] = ctx.getImageData(0, 0, 1, 1).data
      return { r, g, b, a, css: `rgb(${r}, ${g}, ${b})` }
    }, data)
  }

  const sidebarPixel = await pixelAt(coords.sidebar.x, coords.sidebar.y)
  const gapPixel = await pixelAt(coords.gap.x, coords.gap.y)
  const contentPadPixel = coords.contentPad
    ? await pixelAt(coords.contentPad.x, coords.contentPad.y)
    : null
  return {
    coords,
    sidebarPixel,
    gapPixel,
    contentPadPixel,
    pixelMatch:
      sidebarPixel.r === gapPixel.r &&
      sidebarPixel.g === gapPixel.g &&
      sidebarPixel.b === gapPixel.b,
    contentPadMatch: contentPadPixel
      ? sidebarPixel.r === contentPadPixel.r &&
        sidebarPixel.g === contentPadPixel.g &&
        sidebarPixel.b === contentPadPixel.b
      : null,
  }
}

async function main() {
  await mkdir(path.dirname(SHOT), { recursive: true })

  const browser = await chromium.launch({
    headless: true,
    args: ['--no-sandbox', '--disable-dev-shm-usage'],
  })
  const page = await browser.newPage({
    viewport: { width: 1280, height: 800 },
    deviceScaleFactor: 1,
  })

  console.log('Opening', BASE)
  await page.goto(BASE, { waitUntil: 'networkidle', timeout: 30000 })
  await page.waitForSelector('#tab-overview.tab.active', { timeout: 15000 })
  await page.waitForTimeout(1200)

  const results = []
  for (const tab of TABS) {
    await openTab(page, tab)
    const probe = await probeTab(page, tab === 'lag' ? 'lag' : tab)
    results.push(probe)
    console.log(
      `[${tab}] sidebar=${probe.sidebarBg} content=${probe.contentBg} match=${probe.match} gap=${probe.gap} painter=${probe.gapPainter?.node || 'n/a'}`,
    )
    if (!probe.match) {
      console.error('FAIL computed backgroundColor mismatch on', tab)
      console.error(JSON.stringify({ sidebar: probe.sidebarBg, content: probe.contentBg }, null, 2))
      await browser.close()
      process.exit(1)
    }
  }

  // Deep walk dump for Uninstall + Overview
  await openTab(page, 'uninstall')
  const uninstall = results.find((r) => r.tab === 'uninstall')
  console.log('\n=== UNINSTALL parent walk (gap painter) ===')
  console.log(JSON.stringify(uninstall.chain, null, 2))

  await openTab(page, 'overview')
  const overview = results.find((r) => r.tab === 'overview')
  console.log('\n=== OVERVIEW parent walk (gap painter) ===')
  console.log(JSON.stringify(overview.chain, null, 2))

  await openTab(page, 'uninstall')
  const pixels = await paintedMatch(page)
  console.log('\n=== PAINTED PIXELS (uninstall gap vs sidebar) ===')
  console.log(JSON.stringify(pixels, null, 2))

  await page.screenshot({ path: SHOT, fullPage: false })
  console.log('\nScreenshot saved:', SHOT)

  const summary = {
    before: {
      note: 'Prior: sidebar rgba(14, 18, 32, 0.66) translucent; content transparent over body blue gradient + ambient',
      sidebarBg: 'rgba(14, 18, 32, 0.66)',
      contentBg: 'rgba(0, 0, 0, 0)',
    },
    after: {
      sidebarBg: uninstall.sidebarBg,
      contentBg: uninstall.contentBg,
      match: uninstall.sidebarBg === uninstall.contentBg,
      tabGap: uninstall.gap,
      painted: pixels,
    },
    allTabsMatch: results.every((r) => r.match),
  }
  console.log('\n=== ASSERTION SUMMARY ===')
  console.log(JSON.stringify(summary, null, 2))

  const pixelOk = Boolean(pixels?.pixelMatch)
  if (!summary.allTabsMatch || !pixelOk) {
    console.error('FAIL painted gap pixel must match sidebar (pixelMatch=true)')
    console.error(JSON.stringify({
      allTabsMatch: summary.allTabsMatch,
      pixelMatch: pixels?.pixelMatch,
      sidebarPixel: pixels?.sidebarPixel,
      gapPixel: pixels?.gapPixel,
    }, null, 2))
    await browser.close()
    process.exit(1)
  }

  await browser.close()
  console.log('\nPASS: computed + painted gap pixels match sidebar on all tabs')
}

main().catch((e) => {
  console.error(e)
  process.exit(1)
})
