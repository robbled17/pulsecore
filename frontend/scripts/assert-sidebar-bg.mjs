/**
 * Assert shared full-window ambient layer: transparent content/wrappers,
 * glass sidebar + capsules, continuous deep-blue gaps (no flat scrap seams).
 * Usage: node scripts/assert-sidebar-bg.mjs [baseUrl] [screenshotPath]
 */
import { chromium } from 'playwright'
import { mkdir } from 'node:fs/promises'
import path from 'node:path'

const BASE = process.argv[2] || 'http://127.0.0.1:5173/demo.html'
const SHOT =
  process.argv[3] ||
  '/cursor/stores/bc-01a1f952-18e1-7825-8cd8-3e318e381534/media/parallax-window-bg/uninstall.png'

const TABS = ['overview', 'lag', 'hardware', 'upgrade', 'uninstall', 'gamelab', 'settings']
const GLASS = 'rgba(13, 20, 36, 0.72)'

async function openTab(page, tab) {
  const dataTab = tab === 'lag' ? 'lag' : tab
  await page.click(`.nav-item[data-tab="${dataTab}"]`)
  await page.waitForSelector(`#tab-${dataTab}.tab.active`, { timeout: 8000 })
  await page.waitForTimeout(500)
}

async function probeTab(page, tab) {
  return page.evaluate(
    ({ tabName, glass }) => {
      const sidebar = document.querySelector('.sidebar')
      const content = document.querySelector('main.content, .content')
      const ambient = document.querySelector('.ambient')
      if (!sidebar || !content) throw new Error('missing sidebar/content')
      if (!ambient) throw new Error('missing .ambient layer')

      const sidebarBg = getComputedStyle(sidebar).backgroundColor
      const contentBg = getComputedStyle(content).backgroundColor
      const ambientPos = getComputedStyle(ambient).position
      const ambientZ = getComputedStyle(ambient).zIndex
      const ambientPe = getComputedStyle(ambient).pointerEvents
      const ambientBgImg = getComputedStyle(ambient).backgroundImage

      const tabEl = document.querySelector(`#tab-${tabName}`)
      const card =
        tabEl?.querySelector('.panel, .metric-card, .status-strip, .game-card') ||
        content.querySelector('.panel, .metric-card')

      const cardBg = card ? getComputedStyle(card).backgroundColor : null
      const cardBorder = card ? getComputedStyle(card).borderTopColor : null
      const cardBackdrop = card ? getComputedStyle(card).backdropFilter : null

      const wrappers = []
      let el = card?.parentElement
      while (el && el !== content && el !== document.body) {
        const cs = getComputedStyle(el)
        wrappers.push({
          node: `${el.tagName.toLowerCase()}${el.id ? '#' + el.id : ''}`,
          backgroundColor: cs.backgroundColor,
          backgroundImage: cs.backgroundImage.slice(0, 80),
        })
        el = el.parentElement
      }

      const opaqueWrapper = wrappers.find((w) => {
        const opaque =
          w.backgroundColor &&
          w.backgroundColor !== 'rgba(0, 0, 0, 0)' &&
          w.backgroundColor !== 'transparent'
        const hasImg = w.backgroundImage && w.backgroundImage !== 'none'
        return opaque || hasImg
      })

      return {
        tab: tabName,
        sidebarBg,
        contentBg,
        contentTransparent:
          !contentBg ||
          contentBg === 'transparent' ||
          contentBg === 'rgba(0, 0, 0, 0)',
        sidebarGlass: sidebarBg === glass || sidebarBg.replace(/\s+/g, '') === glass.replace(/\s+/g, ''),
        cardBg,
        cardBorder,
        cardBackdrop,
        ambient: {
          position: ambientPos,
          zIndex: ambientZ,
          pointerEvents: ambientPe,
          hasGradient: ambientBgImg.includes('linear-gradient') || ambientBgImg.includes('radial-gradient'),
        },
        gap: getComputedStyle(tabEl || content).gap,
        opaqueWrapper: opaqueWrapper || null,
        wrappers,
      }
    },
    { tabName: tab, glass: GLASS },
  )
}

async function gapPixelSample(page) {
  const coords = await page.evaluate(() => {
    const cards = [...document.querySelectorAll('#tab-uninstall.tab.active .panel')].filter((el) => {
      const r = el.getBoundingClientRect()
      return r.height > 8 && r.width > 8
    })
    if (cards.length < 2) return null
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
    return {
      gap: { x: gx, y: gy, size: b.top - a.bottom },
      hit: document.elementFromPoint(gx, gy)?.className || null,
    }
  })
  if (!coords) return { error: 'no coords' }

  const cdp = await page.context().newCDPSession(page)
  const { data } = await cdp.send('Page.captureScreenshot', {
    format: 'png',
    clip: {
      x: Math.floor(coords.gap.x),
      y: Math.floor(coords.gap.y),
      width: 1,
      height: 1,
      scale: 1,
    },
    fromSurface: true,
  })
  const gapPixel = await page.evaluate(async (b64) => {
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

  // Deep blue volume: dark navy, not flat black scrap (#000) or bright white.
  const deepBlue =
    gapPixel.b > gapPixel.r &&
    gapPixel.b > gapPixel.g &&
    gapPixel.r < 40 &&
    gapPixel.g < 50 &&
    gapPixel.b < 80 &&
    gapPixel.r + gapPixel.g + gapPixel.b > 12

  return { coords, gapPixel, deepBlue }
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
      `[${tab}] sidebar=${probe.sidebarBg} contentTransparent=${probe.contentTransparent} glass=${probe.sidebarGlass} gap=${probe.gap} ambient=${probe.ambient.hasGradient}`,
    )
    if (!probe.contentTransparent) {
      console.error('FAIL content must be transparent on', tab)
      await browser.close()
      process.exit(1)
    }
    if (!probe.sidebarGlass) {
      console.error('FAIL sidebar glass fill expected', GLASS, 'got', probe.sidebarBg)
      await browser.close()
      process.exit(1)
    }
    if (!probe.ambient.hasGradient || probe.ambient.position !== 'fixed') {
      console.error('FAIL ambient layer missing fixed gradient', probe.ambient)
      await browser.close()
      process.exit(1)
    }
    if (probe.opaqueWrapper) {
      console.error('FAIL opaque wrapper under card', probe.opaqueWrapper)
      await browser.close()
      process.exit(1)
    }
    if (probe.gap !== '12px') {
      console.error('FAIL tab gap expected 12px got', probe.gap)
      await browser.close()
      process.exit(1)
    }
  }

  await openTab(page, 'uninstall')
  const pixels = await gapPixelSample(page)
  console.log('\n=== GAP PIXEL (should be deep blue ambient) ===')
  console.log(JSON.stringify(pixels, null, 2))

  await page.screenshot({ path: SHOT, fullPage: false })
  console.log('\nScreenshot saved:', SHOT)

  if (!pixels?.deepBlue) {
    console.error('FAIL gap pixel is not deep blue volumetric ambient')
    await browser.close()
    process.exit(1)
  }

  await browser.close()
  console.log('\nPASS: continuous ambient + glass sidebar/capsules + transparent wrappers')
}

main().catch((e) => {
  console.error(e)
  process.exit(1)
})
