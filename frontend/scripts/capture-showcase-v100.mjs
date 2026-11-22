/**
 * Capture PulseCore v1.0.0 showcase assets into assets/showcase/.
 * Headless Chromium 1366x820, dark theme, no Dev badges.
 */
import { chromium } from 'playwright'
import { mkdir, copyFile, writeFile, rm, readdir } from 'node:fs/promises'
import { createWriteStream, existsSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { spawn } from 'node:child_process'
import { createServer } from 'node:http'
import { readFileSync, statSync } from 'node:fs'
import { extname } from 'node:path'
import { pipeline } from 'node:stream/promises'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const FRONTEND = path.resolve(__dirname, '..')
const ROOT = path.resolve(FRONTEND, '..')
const OUT = path.join(ROOT, 'assets/showcase')
const TMP = path.join(OUT, '_tmp')
const PORT = 5191
const VW = 1366
const VH = 820

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

function run(cmd, args, opts = {}) {
  return new Promise((resolve, reject) => {
    const p = spawn(cmd, args, { stdio: ['ignore', 'pipe', 'pipe'], ...opts })
    let err = ''
    p.stderr.on('data', (d) => { err += d.toString() })
    p.on('exit', (code) => {
      if (code === 0) resolve()
      else reject(new Error(`${cmd} exited ${code}: ${err.slice(-800)}`))
    })
  })
}

async function preparePage(page) {
  await page.goto(`http://127.0.0.1:${PORT}/demo.html`, { waitUntil: 'load', timeout: 60000 })
  await page.waitForSelector('#tab-overview.tab.active', { timeout: 25000 })
  await page.evaluate(() => {
    document.documentElement.style.colorScheme = 'dark'
    document.body.style.background = '#0b0f19'
    document.getElementById('dev-mode-badge')?.classList.add('hidden')
    document.querySelectorAll('.dev-mode-nav').forEach((n) => n.classList.add('hidden'))
    // Force visible version strings
    document.querySelectorAll('.brand-ver, #about-app-ver, #settings-app-ver').forEach((el) => {
      if (el) el.textContent = 'v1.0.0'
    })
  })
  await page.waitForFunction(() => {
    const el = document.getElementById('ov-cpu')
    return el && el.textContent && el.textContent.trim() !== '-'
  }, { timeout: 20000 }).catch(() => {})
  await page.waitForTimeout(600)
}

async function openTab(page, tab) {
  await page.evaluate((t) => {
    document.querySelector(`.nav-item[data-tab="${t}"]`)?.click()
  }, tab)
  await page.waitForSelector(`#tab-${tab}.tab.active`, { timeout: 10000 })
  await page.waitForTimeout(550)
}

async function shellShot(page, file) {
  const shell = page.locator('.app-shell, .shell, body > .app, #app, .layout').first()
  const hasShell = await shell.count().catch(() => 0)
  if (hasShell) {
    await shell.screenshot({ path: file })
  } else {
    await page.screenshot({ path: file, fullPage: false })
  }
}

async function captureFrames(page, dir, seconds, fps) {
  await mkdir(dir, { recursive: true })
  const total = Math.round(seconds * fps)
  const delay = Math.round(1000 / fps)
  for (let i = 0; i < total; i++) {
    const name = path.join(dir, `f${String(i).padStart(4, '0')}.png`)
    await page.screenshot({ path: name, fullPage: false })
    await page.waitForTimeout(delay)
  }
}

async function framesToGif(frameDir, outGif, width = 1200, fps = 16) {
  // palette + gif for decent size/quality balance; then optionally scale if too large
  const pattern = path.join(frameDir, 'f%04d.png')
  const palette = path.join(frameDir, 'palette.png')
  await run('ffmpeg', [
    '-y', '-framerate', String(fps), '-i', pattern,
    '-vf', `scale=${width}:-1:flags=lanczos,palettegen=stats_mode=diff`,
    palette,
  ])
  await run('ffmpeg', [
    '-y', '-framerate', String(fps), '-i', pattern, '-i', palette,
    '-lavfi', `scale=${width}:-1:flags=lanczos[x];[x][1:v]paletteuse=dither=bayer:bayer_scale=3:diff_mode=rectangle`,
    '-loop', '0',
    outGif,
  ])
  const st = statSync(outGif)
  const mb = st.size / (1024 * 1024)
  console.log(`gif ${path.basename(outGif)} ${(mb).toFixed(2)} MB`)
  // If over 5.5MB, recompress harder
  if (st.size > 5.5 * 1024 * 1024) {
    const tmp = outGif + '.tmp.gif'
    await run('ffmpeg', [
      '-y', '-framerate', String(fps), '-i', pattern, '-i', palette,
      '-lavfi', `scale=${Math.min(width, 1000)}:-1:flags=lanczos,fps=${Math.min(fps, 14)}[x];[x][1:v]paletteuse=dither=bayer:bayer_scale=5`,
      '-loop', '0',
      tmp,
    ])
    await copyFile(tmp, outGif)
    await rm(tmp, { force: true })
    console.log(`gif recompressed ${path.basename(outGif)} ${(statSync(outGif).size / 1024 / 1024).toFixed(2)} MB`)
  }
}

async function stitchVertical(top, bottom, out) {
  // Use ffmpeg vstack
  await run('ffmpeg', [
    '-y', '-i', top, '-i', bottom,
    '-filter_complex', '[0:v]scale=1366:-1[a];[1:v]scale=1366:-1[b];[a][b]vstack=inputs=2',
    out,
  ])
}

async function makeLogo() {
  // Render icon on transparent/dark-neutral square via Playwright canvas-less: just copy PNG.
  // Prefer docs appicon (RGBA). Also write a dark-padded variant without white matte.
  const src = path.join(ROOT, 'docs/assets/appicon.png')
  const browser = await chromium.launch({ headless: true, args: ['--no-sandbox', '--disable-dev-shm-usage'] })
  const page = await browser.newPage({ viewport: { width: 320, height: 320 }, deviceScaleFactor: 2 })
  const b64 = readFileSync(src).toString('base64')
  await page.setContent(`<!doctype html><html><body style="margin:0;background:transparent">
    <canvas id="c" width="256" height="256"></canvas>
    <script>
      const img = new Image();
      img.onload = () => {
        const c = document.getElementById('c');
        const ctx = c.getContext('2d');
        ctx.clearRect(0,0,256,256);
        // draw icon full-bleed (no white padding matte)
        ctx.drawImage(img, 0, 0, 256, 256);
        window.__done = true;
      };
      img.src = 'data:image/png;base64,${b64}';
    </script></body></html>`, { waitUntil: 'load' })
  await page.waitForFunction(() => window.__done === true, { timeout: 10000 })
  const buf = await page.locator('#c').screenshot({ omitBackground: true, type: 'png' })
  await writeFile(path.join(OUT, 'logo.png'), buf)
  await browser.close()
  console.log('wrote logo.png')
}

async function main() {
  await mkdir(OUT, { recursive: true })
  await mkdir(TMP, { recursive: true })
  await makeLogo()

  const vite = startVite()
  let browser
  try {
    await waitForServer(`http://127.0.0.1:${PORT}/demo.html`)
    browser = await chromium.launch({
      headless: true,
      args: ['--no-sandbox', '--disable-dev-shm-usage'],
    })

    const page = await browser.newPage({
      viewport: { width: VW, height: VH },
      deviceScaleFactor: 1,
      colorScheme: 'dark',
    })
    await preparePage(page)
    await page.evaluate(() => window.__PULSECORE_MOCK__?.setProfile?.('B'))
    await page.waitForTimeout(700)

    // overview
    await openTab(page, 'overview')
    await page.screenshot({ path: path.join(OUT, 'overview.png'), fullPage: false })
    console.log('overview.png')

    // lags + eco
    await openTab(page, 'lag')
    await page.evaluate(() => {
      const btn = [...document.querySelectorAll('button')].find((b) => /eco|эко/i.test(b.textContent || ''))
      btn?.click()
    })
    await page.waitForTimeout(500)
    await page.screenshot({ path: path.join(OUT, 'lags-eco.png'), fullPage: false })
    console.log('lags-eco.png')

    // hardware cores + SSD
    await openTab(page, 'hardware')
    await page.waitForSelector('#core-grid .core-phys, #core-grid .core', { timeout: 10000 }).catch(() => {})
    await page.evaluate(() => {
      const list = document.getElementById('hw-ssd-list')
      const first = list?.querySelector('.hw-ssd-acc')
      if (first) {
        first.open = true
        first.dispatchEvent(new Event('toggle', { bubbles: true }))
      }
    })
    await page.waitForTimeout(900)
    await page.screenshot({ path: path.join(OUT, 'hardware.png'), fullPage: false })
    console.log('hardware.png')

    // upgrade + uninstall stitch
    await openTab(page, 'upgrade')
    const upTmp = path.join(TMP, 'upgrade.png')
    await page.screenshot({ path: upTmp, fullPage: false })
    await openTab(page, 'uninstall')
    await page.waitForTimeout(400)
    const unTmp = path.join(TMP, 'uninstall.png')
    await page.screenshot({ path: unTmp, fullPage: false })
    await stitchVertical(upTmp, unTmp, path.join(OUT, 'upgrade-uninstaller.png'))
    console.log('upgrade-uninstaller.png')

    // gamepulse
    await openTab(page, 'gamelab')
    await page.waitForTimeout(700)
    await page.screenshot({ path: path.join(OUT, 'gamepulse.png'), fullPage: false })
    console.log('gamepulse.png')

    // demo-monitoring.gif
    await openTab(page, 'overview')
    const monDir = path.join(TMP, 'mon')
    await captureFrames(page, monDir, 3.2, 16)
    await framesToGif(monDir, path.join(OUT, 'demo-monitoring.gif'), 1200, 16)

    // demo-uninstaller.gif
    await openTab(page, 'uninstall')
    const unDir = path.join(TMP, 'un')
    await mkdir(unDir, { recursive: true })
    // Kick uninstall demo interactions if buttons exist
    const killPromise = (async () => {
      await page.waitForTimeout(400)
      await page.evaluate(() => {
        const kill = document.getElementById('btn-uninstall-kill')
          || [...document.querySelectorAll('button')].find((b) => /kill|заверш|удал/i.test(b.textContent || ''))
        kill?.click()
      })
    })()
    await Promise.all([
      killPromise,
      captureFrames(page, unDir, 3.6, 16),
    ])
    await framesToGif(unDir, path.join(OUT, 'demo-uninstaller.gif'), 1200, 16)

    await page.close()

    // README rendered preview helper note
    const listing = (await readdir(OUT)).filter((f) => !f.startsWith('_'))
    await writeFile(path.join(OUT, 'MANIFEST.txt'), listing.sort().join('\n') + '\n')
    console.log('showcase assets:', listing.join(', '))
  } finally {
    if (browser) await browser.close().catch(() => {})
    killChild(vite)
    await rm(TMP, { recursive: true, force: true }).catch(() => {})
  }
}

main().catch((err) => {
  console.error(err)
  process.exit(1)
})
