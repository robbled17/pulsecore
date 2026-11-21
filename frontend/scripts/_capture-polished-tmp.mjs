import { chromium } from 'playwright'
import { mkdir, copyFile } from 'node:fs/promises'
import { spawn } from 'node:child_process'
import path from 'node:path'

const root = '/agent/repos/pulsecore/frontend'
const MEDIA = '/cursor/stores/bc-01a1f952-18e1-7825-8cd8-3e318e381534/media/self-diagnostics-v160'
const ART = '/opt/cursor/artifacts'
const PORT = 4192
await mkdir(MEDIA, { recursive: true })
await mkdir(ART, { recursive: true })

function kill(child) {
  try {
    child.kill('SIGKILL')
  } catch (_) {}
}

const child = spawn('npx', ['vite', '--host', '127.0.0.1', '--port', String(PORT), '--strictPort'], {
  cwd: root, stdio: ['ignore', 'pipe', 'pipe'],
})
await new Promise((resolve, reject) => {
  const t = setTimeout(() => reject(new Error('vite timeout')), 45000)
  const on = (b) => {
    if (/Local:|ready in/i.test(String(b))) { clearTimeout(t); resolve() }
  }
  child.stdout.on('data', on)
  child.stderr.on('data', on)
  child.on('exit', (c) => reject(new Error('vite exit ' + c)))
})

// HTTP ready
for (let i = 0; i < 40; i++) {
  try {
    const r = await fetch(`http://127.0.0.1:${PORT}/demo.html`)
    if (r.ok) break
  } catch (_) {}
  await new Promise((r) => setTimeout(r, 250))
}

const browser = await chromium.launch({ headless: true })
try {
  const page = await browser.newPage({ viewport: { width: 1180, height: 760 } })
  await page.goto(`http://127.0.0.1:${PORT}/demo.html`, { waitUntil: 'load', timeout: 60000 })
  await page.waitForSelector('#tab-overview.tab.active', { timeout: 25000 })
  await page.evaluate(() => {
    const badge = document.createElement('div')
    badge.textContent = 'MOCK / TEST DATA'
    badge.style.cssText = 'position:fixed;top:8px;right:12px;z-index:9999;background:rgba(0,0,0,.55);color:#fde68a;padding:4px 8px;border-radius:8px;font:650 11px Inter,sans-serif;pointer-events:none'
    document.body.appendChild(badge)
  })
  await page.locator('#btn-about').click()
  await page.waitForSelector('#about-modal:not(.hidden)')
  await page.locator('#btn-check-sensors').click()
  await page.waitForSelector('#diag-modal:not(.hidden)')
  await page.waitForSelector('#diag-list .diag-item')
  await page.waitForTimeout(400)
  // Assert polish: no raw Working / model=
  const body = await page.locator('#diag-list').innerText()
  if (/\bWorking\b/.test(body)) throw new Error('still shows Working: ' + body.slice(0, 200))
  if (/model=/.test(body)) throw new Error('still shows model=: ' + body.slice(0, 200))
  const file = 'diagnostics-modal-polished.png'
  await page.screenshot({ path: path.join(ART, file) })
  await copyFile(path.join(ART, file), path.join(MEDIA, file))
  console.log('saved', path.join(MEDIA, file))
  console.log('preview text:\n', body.slice(0, 500))
} finally {
  await browser.close().catch(() => {})
  kill(child)
}
