/**
 * Capture required v1.6.2 screenshots (mock/test data labeled).
 * - Docs features grid desktop
 * - App Hardware tab cores 1..8
 */
import { chromium } from 'playwright'
import { createServer } from 'node:http'
import { readFile, mkdir, copyFile, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { spawn } from 'node:child_process'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const ROOT = path.resolve(__dirname, '..')
const REPO = path.resolve(ROOT, '..')
const MEDIA = '/cursor/stores/bc-01a1f952-18e1-7825-8cd8-3e318e381534/media/embedded-helper-v162'
const ARTIFACTS = '/opt/cursor/artifacts'

async function serveDocs(port) {
  const docsRoot = path.join(REPO, 'docs')
  const server = createServer(async (req, res) => {
    let urlPath = decodeURIComponent((req.url || '/').split('?')[0])
    if (urlPath === '/') urlPath = '/index.html'
    const file = path.join(docsRoot, urlPath.replace(/^\//, ''))
    if (!file.startsWith(docsRoot)) {
      res.writeHead(403); res.end(); return
    }
    try {
      const data = await readFile(file)
      const ext = path.extname(file)
      const type = ext === '.html' ? 'text/html' : ext === '.css' ? 'text/css' : ext === '.js' ? 'text/javascript' : 'application/octet-stream'
      res.writeHead(200, { 'Content-Type': type })
      res.end(data)
    } catch {
      res.writeHead(404); res.end('missing')
    }
  })
  await new Promise((resolve) => server.listen(port, '127.0.0.1', resolve))
  return server
}

async function main() {
  await mkdir(MEDIA, { recursive: true })
  await mkdir(ARTIFACTS, { recursive: true })

  const docsPort = 8765
  const docsServer = await serveDocs(docsPort)

  // Start Vite mock demo for Hardware tab
  const vite = spawn('npm', ['run', 'dev', '--', '--host', '127.0.0.1', '--port', '5173'], {
    cwd: ROOT,
    stdio: ['ignore', 'pipe', 'pipe'],
    env: { ...process.env },
  })
  let viteReady = false
  const waitVite = new Promise((resolve, reject) => {
    const t = setTimeout(() => reject(new Error('vite timeout')), 45000)
    const onData = (buf) => {
      const s = buf.toString()
      if (s.includes('Local:') || s.includes('5173')) {
        viteReady = true
        clearTimeout(t)
        resolve()
      }
    }
    vite.stdout.on('data', onData)
    vite.stderr.on('data', onData)
    vite.on('exit', (code) => {
      if (!viteReady) reject(new Error(`vite exited ${code}`))
    })
  })
  await waitVite

  const browser = await chromium.launch({
    headless: true,
    args: ['--no-sandbox', '--disable-dev-shm-usage'],
  })

  // 1) Docs features grid
  {
    const page = await browser.newPage({ viewport: { width: 1440, height: 900 }, deviceScaleFactor: 1 })
    await page.goto(`http://127.0.0.1:${docsPort}/index.html`, { waitUntil: 'domcontentloaded', timeout: 30000 })
    await page.waitForSelector('#features .bento', { timeout: 15000 })
    await page.evaluate(() => {
      document.getElementById('features')?.scrollIntoView({ block: 'start' })
      document.querySelectorAll('#features .reveal').forEach((el) => el.classList.add('is-in'))
    })
    await page.waitForTimeout(500)
    const feat = page.locator('#features')
    const out = path.join(MEDIA, 'docs-features-grid-desktop.png')
    await feat.screenshot({ path: out })
    await copyFile(out, path.join(ARTIFACTS, 'docs-features-grid-desktop.png'))
    console.log('saved', out)
    await page.close()
  }

  // 2) App Hardware tab with 8 physical cores (profile B)
  {
    const page = await browser.newPage({ viewport: { width: 1280, height: 860 }, deviceScaleFactor: 1 })
    await page.goto('http://127.0.0.1:5173/demo.html', { waitUntil: 'networkidle', timeout: 45000 })
    await page.waitForSelector('#tab-overview.tab.active', { timeout: 20000 })
    await page.evaluate(() => window.__PULSECORE_MOCK__?.setProfile?.('B'))
    await page.waitForTimeout(800)
    await page.click('.nav-item[data-tab="hardware"]')
    await page.waitForSelector('#tab-hardware.tab.active', { timeout: 8000 })
    await page.waitForSelector('#core-grid .core-phys', { timeout: 10000 })
    await page.waitForTimeout(600)
    // Label mock/test data in shot
    await page.evaluate(() => {
      const badge = document.createElement('div')
      badge.textContent = 'MOCK / test data · cores 1..8'
      badge.style.cssText = 'position:fixed;left:12px;bottom:12px;z-index:9999;padding:6px 10px;border-radius:8px;background:rgba(0,0,0,0.65);color:#9ecbff;font:12px/1.2 sans-serif;border:1px solid rgba(110,168,255,0.35)'
      document.body.appendChild(badge)
    })
    const out = path.join(MEDIA, 'app-hardware-cores-1-to-8.png')
    await page.screenshot({ path: out, fullPage: false })
    await copyFile(out, path.join(ARTIFACTS, 'app-hardware-cores-1-to-8.png'))
    console.log('saved', out)

    // Verify labels are 1..8 not 0..7
    const labels = await page.$$eval('#core-grid .core-phys .id', (els) => els.map((e) => e.textContent.trim()))
    console.log('core labels:', labels)
    await writeFile(path.join(MEDIA, 'README.md'), `# Embedded helper v1.6.2 screenshots

Mock / test data captures for PulseCore v1.6.2.

- \`docs-features-grid-desktop.png\` - landing \`#features\` dense 3-col grid
- \`app-hardware-cores-1-to-8.png\` - Hardware tab physical cores labeled 1..8

Core labels observed: ${labels.join(', ')}
`)
    await page.close()
  }

  await browser.close()
  docsServer.close()
  vite.kill('SIGTERM')
}

main().catch((err) => {
  console.error(err)
  process.exit(1)
})
