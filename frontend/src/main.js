import {
  GetMetrics,
  FreezeProcess,
  ResumeProcess,
  SetPowerProfile,
  SetPollInterval,
  SetLanguage,
  ToggleMiniHUD,
  ToggleGameOverlay,
  HideToTray,
  ToggleFullscreen,
  CheckDiskHealth,
  GetMonthlyAppReport,
  DiagnoseLagOrigin,
  CheckSleepBlockers,
  EcoModeByProcessName,
  GetConnectedPeripherals,
  ListGames,
  SearchGames,
  SearchGamesOnline,
  PredictGameFPS,
} from '../wailsjs/go/main/App.js'
import { EventsOn, WindowMinimise } from '../wailsjs/runtime/runtime.js'
import { i18n } from './i18n.js'

const MIT_LICENSE = `PulseCore v1.0

MIT License

Copyright (c) 2026 Robert Sharafutdinov

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.`

const HISTORY_LEN = 60
const BUFFER_DELAY_MS = 650
const LERP = 0.08
const EMA_ALPHA = 0.25

const chartThemes = {
  cpu: { stroke: '#8ec0ff', glow: 'rgba(142, 192, 255, 0.95)', top: 'rgba(110, 168, 255, 0.55)', bottom: 'rgba(110, 168, 255, 0.04)' },
  gpu: { stroke: '#5ff0d4', glow: 'rgba(95, 240, 212, 0.95)', top: 'rgba(61, 231, 200, 0.5)', bottom: 'rgba(61, 231, 200, 0.04)' },
  ram: { stroke: '#b49bff', glow: 'rgba(180, 155, 255, 0.95)', top: 'rgba(155, 123, 255, 0.5)', bottom: 'rgba(155, 123, 255, 0.04)' },
  battery: { stroke: '#ffd166', glow: 'rgba(255, 209, 102, 0.9)', top: 'rgba(251, 191, 36, 0.45)', bottom: 'rgba(251, 191, 36, 0.04)' },
}

const APP_VERSION = 'v1.0'
const GITHUB_URL = 'https://github.com/robbled17/pulsecore'

let lang = localStorage.getItem('zp_lang') || 'ru'
let uiTheme = localStorage.getItem('zp_theme') || 'dark'
let frozen = new Set()
let activeMetric = 'cpu'
let selectedDrive = 'C'
let hudEnabled = false
let gameOverlayOn = false
let latestDisks = []
let gameCatalog = []
let selectedGameId = ''
let lastMetrics = null
const periphCollapsed = new Set(JSON.parse(localStorage.getItem('zp_periph_collapsed') || '[]'))
const cachedDiskReport = Object.create(null)
const whitelist = new Set(JSON.parse(localStorage.getItem('zp_whitelist') || '[]'))
const chartScroll = { pos: 0, target: 0 }
const frameSamples = []

function loadHudPrefs() {
  try {
    const raw = JSON.parse(localStorage.getItem('zp_minihud') || '{}')
    return {
      cpu: raw.cpu !== false,
      gpu: raw.gpu !== false,
      ram: raw.ram !== false,
      temps: raw.temps !== false,
      opacity: Number.isFinite(Number(raw.opacity)) ? Math.min(95, Math.max(35, Number(raw.opacity))) : 78,
    }
  } catch (_) {
    return { cpu: true, gpu: true, ram: true, temps: true, opacity: 78 }
  }
}
let hudPrefs = loadHudPrefs()

function saveHudPrefs() {
  localStorage.setItem('zp_minihud', JSON.stringify(hudPrefs))
}

const series = {
  cpu: makeSeries(),
  gpu: makeSeries(),
  ram: makeSeries(),
  battery: makeSeries(),
}

const $ = (id) => document.getElementById(id)

function makeSeries() {
  return {
    target: Array(HISTORY_LEN).fill(0),
    current: Array(HISTORY_LEN).fill(0),
    queue: [],
  }
}

function t(key) {
  return (i18n[lang] && i18n[lang][key]) || i18n.ru[key] || key
}

function applyI18n() {
  document.querySelectorAll('[data-i18n]').forEach((el) => {
    el.textContent = t(el.getAttribute('data-i18n'))
  })
  document.querySelectorAll('[data-i18n-tip]').forEach((el) => {
    el.setAttribute('data-tip', t(el.getAttribute('data-i18n-tip')))
  })
  document.querySelectorAll('[data-i18n-placeholder]').forEach((el) => {
    el.setAttribute('placeholder', t(el.getAttribute('data-i18n-placeholder')))
  })
  updateChartTitle()
  syncLangSelects()
  syncOverlayPill(gameOverlayOn)
  renderGameGrid()
  if (selectedGameId) predictSelectedGame()
}

function syncLangSelects() {
  const el = $('lang-select-settings')
  if (el) el.value = lang
}

function applyTheme(theme) {
  uiTheme = theme === 'light' ? 'light' : 'dark'
  localStorage.setItem('zp_theme', uiTheme)
  document.documentElement.setAttribute('data-theme', uiTheme)
  document.querySelectorAll('.theme-btn').forEach((btn) => {
    btn.classList.toggle('active', btn.dataset.theme === uiTheme)
  })
}

function na() {
  return lang === 'ru' ? 'Н/Д' : 'N/A'
}

function dash() {
  return '—'
}

async function changeLanguage(next) {
  lang = next || 'ru'
  localStorage.setItem('zp_lang', lang)
  try { await SetLanguage(lang) } catch (_) {}
  applyI18n()
  refresh()
  refreshWeekly()
  refreshPeripherals()
}

function setActiveTab(name) {
  document.querySelectorAll('.nav-item').forEach((btn) => {
    btn.classList.toggle('active', btn.dataset.tab === name)
  })
  document.querySelectorAll('.tab').forEach((tab) => {
    tab.classList.toggle('active', tab.id === `tab-${name}`)
  })
  if (name === 'hardware') refreshPeripherals()
}

function updateChartTitle() {
  const map = { cpu: 'chartCPU', gpu: 'chartGPU', ram: 'chartRAM', battery: 'chartBattery' }
  const el = $('chart-title')
  if (el) el.textContent = t(map[activeMetric] || 'chartCPU')
}

function pushSample(metric, value) {
  const s = series[metric]
  if (!s) return
  let v = Number.isFinite(value) ? Math.max(0, Math.min(100, value)) : 0
  const lastTarget = s.target.length ? s.target[s.target.length - 1] : v
  v = EMA_ALPHA * v + (1 - EMA_ALPHA) * lastTarget
  s.queue.push({ value: v, at: performance.now() + BUFFER_DELAY_MS })
}

function drainQueues(now) {
  Object.keys(series).forEach((key) => {
    const s = series[key]
    while (s.queue.length && s.queue[0].at <= now) {
      const sample = s.queue.shift()
      s.target.push(sample.value)
      if (s.target.length > HISTORY_LEN) s.target.shift()
      while (s.current.length < s.target.length) s.current.push(s.current[s.current.length - 1] || 0)
      if (s.current.length > HISTORY_LEN) s.current.shift()
    }
  })
}

function lerpSeries() {
  Object.keys(series).forEach((key) => {
    const s = series[key]
    const n = Math.min(s.target.length, s.current.length)
    for (let i = 0; i < n; i++) {
      s.current[i] += (s.target[i] - s.current[i]) * LERP
    }
  })
}

function buildPoints(arr, cssW, cssH, padL = 36, padR = 8, padT = 10, padB = 14) {
  const n = Math.max(HISTORY_LEN - 1, 1)
  const plotW = cssW - padL - padR
  const plotH = cssH - padT - padB
  return arr.map((v, i) => ({
    x: padL + (i / n) * plotW,
    y: padT + plotH - (Math.min(100, Math.max(0, v)) / 100) * plotH,
  }))
}

function batteryTheme(pct) {
  if (pct < 20) return { stroke: '#fb7185', glow: 'rgba(251, 113, 133, 0.85)', top: 'rgba(251, 113, 133, 0.34)', bottom: 'rgba(251, 113, 133, 0.02)' }
  if (pct < 40) return { stroke: '#fbbf24', glow: 'rgba(251, 191, 36, 0.8)', top: 'rgba(251, 191, 36, 0.3)', bottom: 'rgba(251, 191, 36, 0.02)' }
  return chartThemes.battery
}

function strokeSmooth(ctx, points) {
  if (points.length < 2) return
  ctx.beginPath()
  ctx.moveTo(points[0].x, points[0].y)
  for (let i = 0; i < points.length - 1; i++) {
    const p0 = points[i === 0 ? i : i - 1]
    const p1 = points[i]
    const p2 = points[i + 1]
    const p3 = points[i + 2] || p2
    ctx.bezierCurveTo(
      p1.x + (p2.x - p0.x) / 6,
      p1.y + (p2.y - p0.y) / 6,
      p2.x - (p3.x - p1.x) / 6,
      p2.y - (p3.y - p1.y) / 6,
      p2.x,
      p2.y,
    )
  }
  ctx.stroke()
}

function fillSmooth(ctx, points, cssH, colorTop, colorBottom) {
  if (points.length < 2) return
  ctx.beginPath()
  ctx.moveTo(points[0].x, points[0].y)
  for (let i = 0; i < points.length - 1; i++) {
    const p0 = points[i === 0 ? i : i - 1]
    const p1 = points[i]
    const p2 = points[i + 1]
    const p3 = points[i + 2] || p2
    ctx.bezierCurveTo(
      p1.x + (p2.x - p0.x) / 6,
      p1.y + (p2.y - p0.y) / 6,
      p2.x - (p3.x - p1.x) / 6,
      p2.y - (p3.y - p1.y) / 6,
      p2.x,
      p2.y,
    )
  }
  const last = points[points.length - 1]
  const first = points[0]
  ctx.lineTo(last.x, cssH - 4)
  ctx.lineTo(first.x, cssH - 4)
  ctx.closePath()
  const grad = ctx.createLinearGradient(0, 0, 0, cssH)
  grad.addColorStop(0, colorTop)
  grad.addColorStop(1, colorBottom)
  ctx.fillStyle = grad
  ctx.fill()
}

function drawTrendFrame() {
  const canvas = $('trend-canvas')
  if (!canvas) return
  const ctx = canvas.getContext('2d')
  const dpr = window.devicePixelRatio || 1
  const cssW = canvas.clientWidth || 900
  const cssH = 190
  if (canvas.width !== Math.floor(cssW * dpr) || canvas.height !== Math.floor(cssH * dpr)) {
    canvas.width = Math.floor(cssW * dpr)
    canvas.height = Math.floor(cssH * dpr)
  }
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
  ctx.clearRect(0, 0, cssW, cssH)

  const padL = 40
  const padR = 36
  const padT = 16
  const padB = 30
  const plotW = cssW - padL - padR
  const plotH = cssH - padT - padB
  const marks = [
    { pct: 100, label: '100%' },
    { pct: 50, label: '50%' },
    { pct: 0, label: '0%' },
  ]
  ctx.setLineDash([3, 5])
  ctx.lineWidth = 1
  ctx.font = '600 11px Inter, Segoe UI, sans-serif'
  marks.forEach((m) => {
    const y = padT + plotH - (m.pct / 100) * plotH
    ctx.strokeStyle = 'rgba(160, 190, 255, 0.12)'
    ctx.beginPath()
    ctx.moveTo(padL, y)
    ctx.lineTo(cssW - padR, y)
    ctx.stroke()
    ctx.fillStyle = 'rgba(200, 210, 230, 0.7)'
    ctx.fillText(m.label, 4, y + 3)
  })
  ctx.setLineDash([])

  const timeLabels = [
    { t: 0, label: '-30s' },
    { t: 1 / 3, label: '-20s' },
    { t: 2 / 3, label: '-10s' },
    { t: 1, label: t('nowLabel') },
  ]
  ctx.fillStyle = 'rgba(200, 210, 230, 0.7)'
  timeLabels.forEach((m) => {
    const x = padL + m.t * plotW
    const label = m.label
    const tw = ctx.measureText(label).width
    const tx = m.t === 1 ? x - tw : x - (m.t === 0 ? 0 : tw / 2)
    ctx.fillText(label, Math.max(padL - 2, Math.min(tx, cssW - padR - tw + 2)), cssH - 8)
  })

  const values = series[activeMetric].current
  const last = values.length ? values[values.length - 1] : 0
  let theme = chartThemes[activeMetric] || chartThemes.cpu
  if (activeMetric === 'battery') theme = batteryTheme(last)

  chartScroll.target = Math.max(0, values.length - 1)
  chartScroll.pos += (chartScroll.target - chartScroll.pos) * 0.1
  const scrollFrac = chartScroll.target > 0 ? (chartScroll.pos / chartScroll.target) : 1
  const pts = buildPoints(values, cssW, cssH, padL, padR, padT, padB).map((p, i) => {
    const ideal = padL + (i / Math.max(HISTORY_LEN - 1, 1)) * plotW
    return { x: ideal - (1 - scrollFrac) * 8, y: p.y }
  })

  fillSmooth(ctx, pts, cssH, theme.top, theme.bottom)

  // Neon glow pass
  ctx.save()
  ctx.lineWidth = 4.5
  ctx.lineJoin = 'round'
  ctx.lineCap = 'round'
  ctx.strokeStyle = theme.glow || theme.stroke
  ctx.shadowColor = theme.glow || theme.stroke
  ctx.shadowBlur = 18
  ctx.globalAlpha = 0.55
  strokeSmooth(ctx, pts)
  ctx.restore()

  ctx.lineWidth = 2.4
  ctx.lineJoin = 'round'
  ctx.lineCap = 'round'
  ctx.strokeStyle = theme.stroke
  ctx.shadowColor = theme.glow || theme.stroke
  ctx.shadowBlur = 10
  strokeSmooth(ctx, pts)
  ctx.shadowBlur = 0

  const cur = $('chart-current')
  if (cur) cur.textContent = `${t('currentPct')}: ${fmtPct(last, 1)}`
}

function animateChart(now) {
  drainQueues(now)
  lerpSeries()
  drawTrendFrame()
  renderSparklines()
  sampleFrametime(now)
  requestAnimationFrame(animateChart)
}

function fmt(n, digits = 1) {
  if (n === null || n === undefined || n === '') return na()
  const v = typeof n === 'number' ? n : Number(n)
  if (!Number.isFinite(v)) return na()
  return v.toFixed(digits)
}

function fmtPct(n, digits = 1) {
  const s = fmt(n, digits)
  return s === na() ? s : `${s}%`
}

function safeNum(n, fallback = 0) {
  const v = Number(n)
  return Number.isFinite(v) ? v : fallback
}

/** Reject null/NaN/0/-1 and absurd temps — always Н/Д or —. */
function fmtTemp(n, digits = 1) {
  const v = Number(n)
  if (!Number.isFinite(v) || v <= 0 || v > 150) return na()
  return v.toFixed(digits)
}

function drawSparkline(canvasId, values, theme) {
  const canvas = $(canvasId)
  if (!canvas) return
  const ctx = canvas.getContext('2d')
  if (!ctx) return
  const dpr = window.devicePixelRatio || 1
  const cssW = canvas.clientWidth || 160
  const cssH = 44
  if (canvas.width !== Math.floor(cssW * dpr) || canvas.height !== Math.floor(cssH * dpr)) {
    canvas.width = Math.floor(cssW * dpr)
    canvas.height = Math.floor(cssH * dpr)
  }
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
  ctx.clearRect(0, 0, cssW, cssH)
  const arr = values || []
  if (arr.length < 2) return
  const padX = 2
  const padY = 3
  const n = Math.max(arr.length - 1, 1)
  const pts = arr.map((raw, i) => {
    const v = Math.min(100, Math.max(0, safeNum(raw, 0)))
    return {
      x: padX + (i / n) * (cssW - padX * 2),
      y: padY + (cssH - padY * 2) - (v / 100) * (cssH - padY * 2),
    }
  })
  const stroke = theme?.stroke || '#6ea8ff'
  const glow = theme?.glow || stroke
  const top = theme?.top || 'rgba(110,168,255,0.35)'
  const bottom = theme?.bottom || 'rgba(110,168,255,0.02)'

  // Translucent fill
  ctx.beginPath()
  pts.forEach((p, i) => { if (i === 0) ctx.moveTo(p.x, p.y); else ctx.lineTo(p.x, p.y) })
  const last = pts[pts.length - 1]
  const first = pts[0]
  ctx.lineTo(last.x, cssH)
  ctx.lineTo(first.x, cssH)
  ctx.closePath()
  const grad = ctx.createLinearGradient(0, 0, 0, cssH)
  grad.addColorStop(0, top)
  grad.addColorStop(1, bottom)
  ctx.fillStyle = grad
  ctx.fill()

  // Glow stroke
  ctx.beginPath()
  pts.forEach((p, i) => { if (i === 0) ctx.moveTo(p.x, p.y); else ctx.lineTo(p.x, p.y) })
  ctx.strokeStyle = glow
  ctx.lineWidth = 3.2
  ctx.lineJoin = 'round'
  ctx.lineCap = 'round'
  ctx.shadowColor = glow
  ctx.shadowBlur = 12
  ctx.globalAlpha = 0.55
  ctx.stroke()

  // Crisp neon stroke
  ctx.globalAlpha = 1
  ctx.shadowBlur = 8
  ctx.strokeStyle = stroke
  ctx.lineWidth = 1.8
  ctx.stroke()
  ctx.shadowBlur = 0
}

function renderSparklines() {
  drawSparkline('spark-cpu', series.cpu.current, chartThemes.cpu)
  drawSparkline('spark-gpu', series.gpu.current, chartThemes.gpu)
  drawSparkline('spark-ram', series.ram.current, chartThemes.ram)
  drawSparkline('spark-battery', series.battery.current, chartThemes.battery)
}

function validTemp(n, available) {
  if (available === false) return NaN
  const v = Number(n)
  return Number.isFinite(v) && v > 0 && v <= 150 ? v : NaN
}

function renderStatusStrip(m) {
  const st = $('st-status')
  const ramEl = $('st-ram')
  const procEl = $('st-procs')
  if (!st || !ramEl || !procEl) return

  const temp = validTemp(m.temperatureC, m.tempAvailable)
  const cpu = safeNum(m.cpuPercent, 0)
  const stutters = (m.stutterEvents || []).filter((e) => safeNum(e.jitterMs, 0) >= 300)
  const vamps = m.vampires || []
  const bgCount = vamps.reduce((sum, v) => sum + (v.count || (v.pids ? v.pids.length : 1) || 1), 0)

  let state = 'ok'
  let label = t('statusOk')
  if (Number.isFinite(temp) && temp >= 90) {
    state = 'hot'
    label = t('statusHot')
  } else if ((Number.isFinite(temp) && temp >= 80) || cpu >= 92 || stutters.length || bgCount >= 5) {
    state = 'warn'
    label = t('statusWarn')
  }
  st.textContent = label
  st.dataset.state = state

  const total = safeNum(m.ramTotalMB, 0)
  const used = safeNum(m.ramUsedMB, 0)
  if (total > 0) {
    const free = Math.max(0, total - used)
    ramEl.textContent = `${fmt(free / 1024, 1)} GB`
  } else {
    ramEl.textContent = na()
  }
  procEl.textContent = String(bgCount)
}

function collectInsights(m) {
  const items = []
  const temp = validTemp(m.temperatureC, m.tempAvailable)
  const ramPct = safeNum(m.ramPercent, 0)
  const disks = m.disks || []
  const sysDisk = disks.find((d) => /^C/i.test(String(d.mount || ''))) || disks[0]
  const diskPct = safeNum(sysDisk?.percent, 0)
  const lags = m.lagEvents || []
  const stutters = (m.stutterEvents || []).filter((e) => safeNum(e.jitterMs, 0) >= 300)
  const vamps = (m.vampires || []).filter((v) => !v.ecoMode && !v.frozen)

  if (Number.isFinite(temp) && temp >= 85) {
    items.push({
      id: 'thermal',
      level: temp >= 90 ? 'hot' : 'warn',
      title: t('insightThermalTitle'),
      body: t('insightThermalBody').replace('{temp}', fmtTemp(temp, 0)),
      actions: [
        { id: 'power-silent', label: t('insightActSilent') },
        { id: 'eco-vampires', label: t('insightActEcoBg') },
        { id: 'tab-lag', label: t('insightActOpenLags') },
      ],
    })
  }

  if (ramPct >= 85) {
    items.push({
      id: 'ram',
      level: ramPct >= 90 ? 'hot' : 'warn',
      title: t('insightRamTitle'),
      body: t('insightRamBody').replace('{pct}', fmt(ramPct, 0)),
      actions: [
        { id: 'tab-weekly', label: t('insightActTopProcs') },
        { id: 'eco-vampires', label: t('ecoAppetite') },
      ],
    })
  }

  if (sysDisk && diskPct >= 90) {
    items.push({
      id: 'disk',
      level: 'warn',
      title: t('insightDiskTitle'),
      body: t('insightDiskBody')
        .replace('{pct}', fmt(diskPct, 0))
        .replace('{mount}', sysDisk.mount || 'C:\\'),
      actions: [
        { id: 'disk-cleanup', label: t('insightActCleanup') },
        { id: 'disk-smart', label: t('diskHealthBtn') },
      ],
    })
  }

  if (lags.length) {
    const e = lags[0]
    const name = e.culpritFriendly || e.culpritProcess || t('unknownProc')
    items.push({
      id: 'lag',
      level: 'warn',
      title: t('insightLagTitle'),
      body: t('insightLagBody').replace('{name}', name).replace('{core}', e.coreID ?? '—'),
      actions: [
        ...(e.culpritPID ? [{ id: 'eco-pid', label: t('ecoLower'), pid: e.culpritPID, name: e.culpritProcess }] : []),
        ...(e.culpritProcess ? [{ id: 'eco-name', label: t('ecoCulprit'), name: e.culpritProcess }] : []),
        { id: 'tab-lag', label: t('insightActOpenLags') },
      ],
    })
  } else if (stutters.length) {
    const e = stutters[0]
    items.push({
      id: 'stutter',
      level: 'warn',
      title: t('insightStutterTitle'),
      body: e.cause || t('insightStutterBody'),
      actions: [
        ...(e.culprit ? [{ id: 'eco-name', label: t('ecoCulprit'), name: e.culprit }] : []),
        { id: 'tab-lag', label: t('insightActOpenLags') },
      ],
    })
  }

  if (vamps.length >= 3 && !items.some((i) => i.id === 'ram' || i.id === 'thermal')) {
    items.push({
      id: 'vampires',
      level: 'warn',
      title: t('insightVampTitle'),
      body: t('insightVampBody').replace('{n}', String(vamps.length)),
      actions: [
        { id: 'eco-vampires', label: t('insightActEcoBg') },
        { id: 'tab-lag', label: t('insightActOpenLags') },
      ],
    })
  }

  return items
}

function renderInsightHost(hostId, items) {
  const host = $(hostId)
  if (!host) return
  if (!items.length) {
    host.innerHTML = ''
    host.classList.add('hidden')
    return
  }
  host.classList.remove('hidden')
  host.innerHTML = items.map((it) => `
    <div class="insight-card ${it.level || 'warn'}" data-insight="${it.id}">
      <div class="insight-text">
        <div class="insight-title">${it.title}</div>
        <div class="insight-body">${it.body}</div>
      </div>
      <div class="insight-actions">
        ${(it.actions || []).map((a) => `
          <button type="button" class="btn compact ${a.id.startsWith('eco') ? 'danger' : ''}"
            data-insight-act="${a.id}"
            ${a.pid != null ? `data-pid="${a.pid}"` : ''}
            ${a.name ? `data-eco-name="${a.name}"` : ''}>${a.label}</button>
        `).join('')}
      </div>
    </div>
  `).join('')
}

function renderInsights(m) {
  const items = collectInsights(m)
  renderInsightHost('insight-list', items)
  // Lags tab: emphasize lag/stutter/thermal actions
  const lagFocus = items.filter((i) => ['lag', 'stutter', 'thermal', 'vampires'].includes(i.id))
  renderInsightHost('lag-insight-list', lagFocus.length ? lagFocus : items.slice(0, 2))
}

function batterySeriesValue(m) {
  const bat = m.battery || {}
  if (bat.available) return Math.max(0, Math.min(100, bat.percent || 0))
  return 100
}

function driveLetter(mount) {
  return String(mount || 'C').replace(':\\', '').replace(':', '').toUpperCase()
}

function renderOverview(m) {
  $('ov-cpu').textContent = fmtPct(m.cpuPercent)
  $('ov-cpu-sub').textContent = m.cpuName || dash()
  $('ov-cpu-bar').style.width = `${Math.min(100, safeNum(m.cpuPercent, 0))}%`

  $('ov-gpu').textContent = fmtPct(m.gpuPercent)
  $('ov-gpu-sub').textContent = m.gpuName || dash()
  $('ov-gpu-bar').style.width = `${Math.min(100, safeNum(m.gpuPercent, 0))}%`

  $('ov-ram').textContent = fmtPct(m.ramPercent)
  const used = safeNum(m.ramUsedMB, NaN)
  const total = safeNum(m.ramTotalMB, NaN)
  $('ov-ram-sub').textContent = (Number.isFinite(used) && Number.isFinite(total) && total > 0)
    ? `${fmt(used, 0)} / ${fmt(total, 0)} MB`
    : na()
  $('ov-ram-bar').style.width = `${Math.min(100, safeNum(m.ramPercent, 0))}%`

  const bat = m.battery || {}
  if (bat.available) {
    $('ov-bat').textContent = fmtPct(bat.percent, 0)
    const wear = bat.wearLabel ? ` · ${bat.wearLabel}` : ''
    $('ov-bat-sub').textContent = `${bat.statusText || dash()} · ${bat.deviceName || t('batteryPack')}${wear}`
    $('ov-bat-bar').style.width = `${Math.min(100, safeNum(bat.percent, 0))}%`
  } else {
    $('ov-bat').textContent = 'AC'
    $('ov-bat-sub').textContent = bat.wearLabel || bat.statusText || t('desktopPC')
    $('ov-bat-bar').style.width = '100%'
  }

  const temp = m.tempAvailable === false ? na() : fmtTemp(m.temperatureC)
  $('temp-chip').textContent = temp === na() ? `${na()} °C` : `${temp} °C`
  $('cpu-name').textContent = m.cpuName || 'CPU'

  renderStatusStrip(m)
  renderInsights(m)

  latestDisks = m.disks || []
  renderDiskTabs(latestDisks)
  const box = $('ov-disks')
  const disks = latestDisks
  if (!disks.length) {
    box.innerHTML = `<div class="empty">${dash()}</div>`
  } else {
    if (!selectedDrive && disks[0]) selectedDrive = driveLetter(disks[0].mount)
    const activeDisk = disks.find((d) => driveLetter(d.mount) === selectedDrive.toUpperCase()) || disks[0]
    const d = activeDisk
    box.innerHTML = `
      <div class="row disk-row active" data-drive="${driveLetter(d.mount)}">
        <div>
          <div class="name">${t('diskPrefix')} ${d.mount}</div>
          <div class="meta">${fmt(d.usedGB)} / ${fmt(d.totalGB)} GB · ${t('free')} ${fmt(d.freeGB)} GB</div>
          <div class="meta">${t('read')} ${fmt(d.readMBps)} MB/s · ${t('write')} ${fmt(d.writeMBps)} MB/s</div>
        </div>
        <div class="name metric">${fmtPct(d.percent, 0)}</div>
      </div>
    `
  }
}

function renderDiskTabs(disks, targetId = 'disk-tabs') {
  const host = $(targetId)
  if (!host) return
  if (!disks.length) {
    host.innerHTML = ''
    return
  }
  host.innerHTML = disks.map((d) => {
    const letter = driveLetter(d.mount)
    const active = letter === selectedDrive.toUpperCase()
    return `
      <button type="button" class="disk-tab ${active ? 'active' : ''}" data-drive="${letter}">
        ${t('diskPrefix')} ${letter}:
        <div class="fill"><i style="width:${Math.min(100, d.percent || 0)}%"></i></div>
      </button>
    `
  }).join('')
}

function lagExplanation(e) {
  const raw = String(e.explanation || '')
  if (raw.startsWith('lag_core_spike|')) {
    const parts = raw.split('|')
    const core = parts[1] ?? e.coreID
    const load = parts[2] ?? fmt(e.coreLoad, 0)
    return t('lagExplain').replace('{core}', core).replace('{load}', load)
  }
  return t('lagExplain').replace('{core}', e.coreID).replace('{load}', fmt(e.coreLoad, 0))
}

function renderLag(m) {
  const stutters = (m.stutterEvents || []).filter((e) => safeNum(e.jitterMs, 0) >= 300)
  const stutterBox = $('stutter-log')
  if (stutterBox) {
    if (!stutters.length) {
      stutterBox.innerHTML = `<div class="status-banner ok">${t('noStutters')}</div>`
    } else {
      stutterBox.innerHTML = stutters.map((e) => `
        <div class="row">
          <div>
            <div class="name metric">${e.timestamp || dash()} · ${fmt(e.jitterMs, 0)} ms</div>
            <div class="meta stutter-cause">${e.cause || dash()}</div>
          </div>
          <div class="row-actions">
            ${e.culprit ? `<button type="button" class="btn compact danger" data-eco-name="${e.culprit}">${t('ecoCulprit')}</button>` : ''}
          </div>
        </div>
      `).join('')
    }
  }

  const lags = m.lagEvents || []
  const lagBox = $('lag-log')
  if (!lags.length) {
    lagBox.innerHTML = `<div class="empty">${t('noLags')}</div>`
  } else {
    lagBox.innerHTML = lags.map((e) => `
      <div class="incident-card">
        <div>
          <div class="incident-title">${e.culpritFriendly || e.culpritProcess || t('unknownProc')}</div>
          <div class="incident-explain">${lagExplanation(e)}</div>
          <div class="meta">${e.timestamp || dash()} · ${e.culpritProcess || dash()} · PID ${e.culpritPID || dash()}</div>
        </div>
        <div class="row-actions">
          ${e.culpritPID ? `<button type="button" class="btn compact danger" data-act="freeze" data-pid="${e.culpritPID}">${t('ecoLower')}</button>` : ''}
        </div>
      </div>
    `).join('')
  }

  const vamps = m.vampires || []
  const vampBox = $('vampire-list')
  if (!vamps.length) {
    vampBox.innerHTML = `<div class="empty">${t('noVampires')}</div>`
  } else {
    vampBox.innerHTML = vamps.map((v) => {
      const isEco = frozen.has(v.pid) || v.ecoMode || v.frozen
      const count = v.count || (v.pids ? v.pids.length : 1) || 1
      const badge = count > 1 ? `<span class="badge-count">${count} ${t('procCount')}</span>` : ''
      const activeTag = isEco ? '' : ` <span class="badge-count">${t('statusActive')}</span>`
      return `
        <div class="row">
          <div>
            <div class="name">${v.friendlyName || v.name}${badge}${activeTag}</div>
            <div class="meta">${v.name || dash()}</div>
            <div class="meta">CPU ${fmtPct(v.cpuPercent)} · disk ~${fmt(v.diskMBps)} MB/s</div>
          </div>
          <div class="row-actions">
            <button type="button" class="btn compact ${isEco ? 'eco-on' : 'danger'}" data-vamp-eco="${v.name}" data-act="${isEco ? 'resume' : 'freeze'}" data-pid="${v.pid}">
              ${isEco ? t('ecoLimited') : (count > 1 ? t('ecoAll') : t('freeze'))}
            </button>
          </div>
        </div>
      `
    }).join('')
  }
}

function renderHardware(m) {
  const hw = m.hardware || {}
  const cpu = hw.cpu || {}
  const gpu = hw.gpu || {}
  const ram = hw.ram || {}
  const pwr = hw.power || {}

  $('hw-cpu-name').textContent = cpu.name || m.cpuName || na()
  const phys = cpu.physicalCores
  const logic = cpu.logicalCores
  $('hw-cpu-cores').textContent = (phys || logic)
    ? `${phys || na()} / ${logic || na()}`
    : na()
  $('hw-cpu-freq').textContent = cpu.frequencyMHz
    ? `${fmt(cpu.frequencyMHz / 1000, 2)} GHz`
    : na()
  $('hw-cpu-load').textContent = fmtPct(cpu.percent ?? m.cpuPercent)

  const cores = cpu.perCore || m.cpuPerCore || []
  $('core-grid').innerHTML = cores.length
    ? cores.map((c, i) => {
      const load = Number.isFinite(Number(c)) ? safeNum(c, 0) : 0
      const val = Number.isFinite(Number(c)) ? fmtPct(c, 0) : na()
      return `
      <div class="core-cell">
        <div class="id">C${i}</div>
        <div class="val">${val}</div>
        <div class="mini-bar"><i style="width:${Math.min(100, load)}%"></i></div>
      </div>`
    }).join('')
    : `<div class="empty">${na()}</div>`

  $('hw-gpu-name').textContent = gpu.name || m.gpuName || na()
  $('hw-gpu-driver').textContent = gpu.driverVersion || na()
  const ded = gpu.dedicatedMB ?? m.vramTotalMB
  const shared = gpu.sharedMB ?? m.vramSharedMB
  $('hw-gpu-ded').textContent = Number.isFinite(Number(ded)) && Number(ded) > 0 ? `${fmt(ded, 0)} MB` : na()
  $('hw-gpu-shared').textContent = Number.isFinite(Number(shared)) ? `${fmt(shared, 0)} MB` : na()

  const ramTotal = ram.totalMB || m.ramTotalMB
  $('hw-ram-total').textContent = ramTotal ? `${ramTotal} MB` : na()
  const ramType = ram.type || ''
  const ramSpeed = ram.speedMHz ? `${ram.speedMHz} MHz` : ''
  $('hw-ram-type').textContent = (ramType || ramSpeed)
    ? [ramType || na(), ramSpeed].filter(Boolean).join(' · ')
    : na()
  const slotsUsed = ram.slotsUsed
  const slotsTotal = ram.slotsTotal
  $('hw-ram-slots').textContent = (slotsUsed != null || slotsTotal != null)
    ? `${slotsUsed ?? na()}/${slotsTotal ?? na()}`
    : na()
  const usedMB = ram.usedMB || m.ramUsedMB
  const freeMB = ram.freeMB
  $('hw-ram-use').textContent = (usedMB || freeMB)
    ? `${usedMB || 0} / ${freeMB ?? na()} MB`
    : na()
  const channel = $('hw-ram-channel')
  if (channel) {
    channel.textContent = ram.channelStatus || na()
    channel.classList.remove('warn', 'ok')
    if (ram.isSingleChannel) channel.classList.add('warn')
    else if ((ram.slotsUsed || 0) >= 2) channel.classList.add('ok')
  }

  if (pwr.hasBattery) {
    $('hw-pwr-name').textContent = pwr.deviceName || t('batteryPack')
    $('hw-pwr-status').textContent = pwr.statusText || na()
    $('hw-pwr-pct').textContent = fmtPct(pwr.percent, 0)
    $('hw-pwr-wear').textContent = pwr.wearLabel || fmtPct(pwr.wearPercent, 0)
    $('hw-pwr-cap').textContent = pwr.capacitymWh
      ? `${fmt(pwr.capacitymWh, 0)} / ${fmt(pwr.designCapacitymWh, 0)} mWh`
      : na()
  } else {
    $('hw-pwr-name').textContent = t('desktopPC')
    $('hw-pwr-status').textContent = 'AC'
    $('hw-pwr-pct').textContent = 'AC'
    $('hw-pwr-wear').textContent = pwr.wearLabel || t('desktopPC')
    $('hw-pwr-cap').textContent = na()
  }
}

function isDefenderName(name) {
  const n = String(name || '').toLowerCase()
  return /msmpeng|defender|защитник|антивирус|antivirus/.test(n)
}

function localizeWeeklyApp(a) {
  const n = String(a.name || '').toLowerCase()
  const friendlyKey = String(a.friendlyName || '')
  const detailKey = String(a.detail || '')

  if (friendlyKey.startsWith('proc') && i18n[lang]?.[friendlyKey]) {
    return {
      title: t(friendlyKey),
      detail: (detailKey.startsWith('proc') && i18n[lang]?.[detailKey]) ? t(detailKey) : t(detailKey),
    }
  }
  if (detailKey.startsWith('proc') && i18n[lang]?.[detailKey]) {
    // title still needs mapping from exe
  }

  if (isDefenderName(n) || /defender|защитник/.test(friendlyKey.toLowerCase())) {
    return { title: t('procDefender'), detail: t('procDefenderDetail') }
  }
  if (/searchindexer|searchprotocol|searchhost/.test(n) || /indexer|индекс/.test(friendlyKey.toLowerCase())) {
    return { title: t('procIndexer'), detail: t('procIndexerDetail') }
  }
  if (/chrome|msedge|firefox|brave|opera|browser|браузер/.test(n) || /browser|браузер/.test(friendlyKey.toLowerCase())) {
    return { title: t('procBrowser'), detail: t('procBrowserDetail') }
  }
  if (friendlyKey.startsWith('proc')) return { title: t(friendlyKey), detail: t(detailKey) }
  if (detailKey.startsWith('proc')) return { title: a.friendlyName || a.name, detail: t(detailKey) }
  return { title: a.friendlyName || a.name, detail: a.detail || a.name }
}

async function refreshWeekly() {
  const box = $('weekly-list')
  if (!box) return
  box.innerHTML = `<div class="empty">${t('weeklyLoading')}</div>`
  try {
    const r = await GetMonthlyAppReport()
    const apps = (r.topApps || []).filter((a) => !whitelist.has(String(a.name || '').toLowerCase()))
    if (!apps.length) {
      box.innerHTML = `<div class="empty">${t('weeklyEmpty')}</div>`
      return
    }
    box.innerHTML = apps.map((a, i) => {
      const loc = localizeWeeklyApp(a)
      return `
      <div class="row">
        <div>
          <div class="name">#${i + 1} ${loc.title}</div>
          <div class="meta">${loc.detail}</div>
        </div>
        <div class="row-actions">
          <button type="button" class="btn compact danger" data-weekly-eco="${a.name}">${t('ecoAppetite')}</button>
          <button type="button" class="btn compact" data-weekly-hide="${a.name}">${t('whitelist')}</button>
        </div>
      </div>`
    }).join('')
  } catch (err) {
    box.innerHTML = `<div class="empty">${String(err)}</div>`
  }
}

function periphGroupMeta(group) {
  switch (group) {
    case 'input': return { title: t('groupInput'), ico: '🖱' }
    case 'audio': return { title: t('groupAudio'), ico: '🎧' }
    case 'storage': return { title: t('groupStorage'), ico: '💾' }
    case 'display': return { title: t('groupDisplay'), ico: '🖥' }
    default: return { title: group || '—', ico: '🔌' }
  }
}

function renderPeripherals(items) {
  const box = $('periph-list')
  if (!box) return
  if (!items || !items.length) {
    box.innerHTML = `<div class="empty">—</div>`
    return
  }
  const order = ['input', 'audio', 'storage', 'display']
  const groups = {}
  items.forEach((p) => {
    const g = p.group || 'input'
    if (!groups[g]) groups[g] = []
    groups[g].push(p)
  })
  box.innerHTML = order.filter((g) => groups[g]?.length).map((g) => {
    const meta = periphGroupMeta(g)
    const collapsed = periphCollapsed.has(g)
    const rows = groups[g].map((p) => `
      <div class="periph-row">
        <div class="periph-ico">${p.icon || meta.ico}</div>
        <div>
          <div class="periph-name">${p.name || '-'}</div>
          <div class="periph-meta">${p.kind || p.class || ''}</div>
        </div>
        <div class="periph-status">${t('periphActive')}</div>
      </div>
    `).join('')
    return `
      <div class="periph-group ${collapsed ? 'collapsed' : ''}" data-group="${g}">
        <button type="button" class="periph-group-title" data-toggle-group="${g}">
          <span class="ico">${meta.ico}</span>${meta.title}
          <span class="arrow">${collapsed ? '►' : '▼'}</span>
        </button>
        <div class="periph-group-body">${rows}</div>
      </div>`
  }).join('')
}

async function refreshPeripherals() {
  const box = $('periph-list')
  if (!box) return
  box.innerHTML = `<div class="empty">${t('periphEmpty')}</div>`
  try {
    const items = await GetConnectedPeripherals()
    renderPeripherals(items)
  } catch (err) {
    box.innerHTML = `<div class="empty">${String(err)}</div>`
  }
}

function netBadge(ok, key) {
  return `<span class="net-badge ${ok ? 'ok' : 'bad'}">${t(key)}</span>`
}

async function runNetDiag() {
  const box = $('net-diag')
  box.innerHTML = `
    <div class="net-progress">
      <div class="label">${t('netProgress')}</div>
      <div class="track"><div class="fill"></div></div>
    </div>`
  try {
    const d = await DiagnoseLagOrigin()
    const verdict = t(d.verdictKey || d.verdict || 'netVerdictOK')
    box.innerHTML = `
      <div class="verdict-card ${d.level === 'ok' ? 'ok' : 'warn'}"><div class="v-title">${verdict}</div></div>
      <div class="net-card">
        <div>
          <div class="title">${t('netRouter')}</div>
          <div class="hint">${t('netRouterHint')}</div>
          ${netBadge(!!d.gatewayBadgeOK, d.gatewayBadge || 'netBadgeIdeal')}
        </div>
        <div class="net-ms">${d.gatewayMs < 0 ? '—' : fmt(d.gatewayMs, 0) + ' ms'}</div>
      </div>
      <div class="net-card">
        <div>
          <div class="title">${t('netInternet')}</div>
          <div class="hint">${t('netInternetHint')}</div>
          ${netBadge(!!d.publicBadgeOK, d.publicBadge || 'netBadgeFast')}
        </div>
        <div class="net-ms">${d.publicMs < 0 ? '—' : fmt(d.publicMs, 0) + ' ms'}</div>
      </div>
      <div class="net-card">
        <div>
          <div class="title">${t('netJitter')}</div>
          <div class="hint">${t(d.jitterBadgeOK ? 'netJitterHintOk' : 'netJitterHintBad')}</div>
          ${netBadge(!!d.jitterBadgeOK, d.jitterBadge || 'netBadgeStable')}
        </div>
        <div class="net-ms">${fmt(d.localJitterMs, 0)} ms</div>
      </div>
    `
  } catch (err) {
    box.innerHTML = `<div class="empty">${String(err)}</div>`
  }
}

async function runSleepCheck() {
  const box = $('sleep-report')
  box.innerHTML = `<div class="empty">...</div>`
  try {
    const r = await CheckSleepBlockers()
    const blockers = r.blockers || []
    if (!r.ok && r.summary === 'sleep_query_failed') {
      box.innerHTML = `<div class="verdict-card warn"><div class="v-title">${t('sleepFail')}</div></div>`
      return
    }
    if (r.sleepOK || !blockers.length || r.summary === 'sleep_ok') {
      box.innerHTML = `<div class="verdict-card ok"><div class="v-title">${t('sleepOk')}</div></div>`
      return
    }
    const name = blockers[0] || '—'
    const title = t('sleepBlocked').replace('{name}', name)
    box.innerHTML = `
      <div class="verdict-card warn">
        <div class="v-title">${title}</div>
        <div class="v-sub">${blockers.slice(1).join(' · ')}</div>
      </div>
      ${blockers.map((b) => `<div class="row"><div class="name">${b}</div></div>`).join('')}
    `
  } catch (err) {
    box.innerHTML = `<div class="verdict-card warn"><div class="v-title">${t('sleepFail')}</div><div class="v-sub">${String(err)}</div></div>`
  }
}

function applyMiniHudPrefs() {
  const keys = ['cpu', 'gpu', 'ram', 'temps']
  keys.forEach((k) => {
    const el = document.querySelector(`.mh-item[data-mh="${k}"]`)
    el?.classList.toggle('hidden', !hudPrefs[k])
    const opt = $(`mh-opt-${k}`)
    if (opt) opt.checked = !!hudPrefs[k]
  })
  const visible = [...document.querySelectorAll('.minihud-metrics .mh-item:not(.hidden)')]
  visible.forEach((el, i) => el.classList.toggle('is-last', i === visible.length - 1))
  const opacity = hudPrefs.opacity
  const capsule = document.querySelector('.minihud-capsule')
  if (capsule) capsule.style.setProperty('--hud-bg-opacity', (opacity / 100).toFixed(2))
  const slider = $('mh-opacity')
  const label = $('mh-opacity-val')
  if (slider) slider.value = String(opacity)
  if (label) label.textContent = `${opacity}%`
}

function renderMiniHUD(m) {
  $('mh-cpu').textContent = fmtPct(m.cpuPercent, 0)
  $('mh-ram').textContent = fmtPct(m.ramPercent, 0)
  $('mh-gpu').textContent = fmtPct(m.gpuPercent, 0)
  const tempEl = $('mh-temp')
  if (tempEl) {
    const temp = fmtTemp(m.temperatureC, 0)
    tempEl.textContent = temp === na() ? na() : `${temp}°`
  }
  applyMiniHudPrefs()
}

function syncSettings(m) {
  document.querySelectorAll('.btn.mode').forEach((btn) => {
    btn.classList.toggle('active', btn.dataset.mode === (m.powerMode || 'balanced'))
  })
  document.querySelectorAll('.btn.poll').forEach((btn) => {
    btn.classList.toggle('active', Number(btn.dataset.poll) === Number(m.pollIntervalMS || 1000))
  })
}

function syncMockProfileButtons(profileId) {
  const bar = $('mock-profile-bar')
  if (!bar || !window.__PULSECORE_MOCK__) return
  const cur = profileId || window.__PULSECORE_MOCK__.getProfile()
  bar.querySelectorAll('.mock-profile-btn').forEach((b) => {
    b.classList.toggle('active', b.dataset.profile === cur)
  })
}

function applyMetrics(m) {
  if (!m) return
  lastMetrics = m
  if (m.language && m.language !== lang && !localStorage.getItem('zp_lang')) {
    lang = m.language
    applyI18n()
  }
  pushSample('cpu', safeNum(m.cpuPercent, 0))
  pushSample('gpu', safeNum(m.gpuPercent, 0))
  pushSample('ram', safeNum(m.ramPercent, 0))
  pushSample('battery', batterySeriesValue(m))
  renderOverview(m)
  renderLag(m)
  renderHardware(m)
  renderMiniHUD(m)
  syncSettings(m)
  syncMockProfileButtons(m.mockProfile)
  setHudMode(!!m.miniHUD)
  const sleepPanel = $('sleep-panel')
  if (sleepPanel) {
    const hasBattery = !!(m.battery && m.battery.available)
    sleepPanel.classList.toggle('hidden', !hasBattery)
  }
}

function setHudMode(enabled) {
  hudEnabled = !!enabled
  if (enabled) setGameOverlayMode(false, true)
  document.body.classList.toggle('mini-hud-active', enabled)
  $('minihud').classList.toggle('hidden', !enabled)
  if (enabled || gameOverlayOn) {
    $('shell').classList.add('hidden')
    $('titlebar').classList.add('hidden')
  } else {
    $('shell').classList.remove('hidden')
    $('titlebar').classList.remove('hidden')
  }
}

function syncOverlayPill(enabled) {
  const btn = $('btn-toggle-overlay')
  if (!btn) return
  btn.classList.toggle('active', !!enabled)
  const label = btn.querySelector('[data-i18n="overlayPill"]') || btn.querySelector('span')
  if (label) {
    label.textContent = enabled ? t('overlayPillOn') : t('overlayPill')
  }
}

function setGameOverlayMode(enabled, skipBackend = false) {
  gameOverlayOn = !!enabled
  if (enabled) {
    document.body.classList.remove('mini-hud-active')
    $('minihud').classList.add('hidden')
  }
  document.body.classList.toggle('game-overlay-active', enabled)
  $('gameoverlay').classList.toggle('hidden', !enabled)
  syncOverlayPill(enabled)
  if (enabled || hudEnabled) {
    $('shell').classList.add('hidden')
    $('titlebar').classList.add('hidden')
  } else {
    $('shell').classList.remove('hidden')
    $('titlebar').classList.remove('hidden')
  }
}

async function enableGameOverlay() {
  try { await ToggleGameOverlay(true) } catch (_) {}
  setGameOverlayMode(true)
}

async function disableGameOverlay() {
  try { await ToggleGameOverlay(false) } catch (_) {}
  setGameOverlayMode(false)
}

async function toggleGameOverlayPill() {
  if (gameOverlayOn) await disableGameOverlay()
  else await enableGameOverlay()
}

function sampleFrametime(now) {
  if (!frameSamples.length) {
    frameSamples.push(now)
    return
  }
  const last = frameSamples[frameSamples.length - 1]
  const dt = now - last
  frameSamples.push(now)
  while (frameSamples.length > 120) frameSamples.shift()
  if (!gameOverlayOn) return
  const deltas = []
  for (let i = 1; i < frameSamples.length; i++) {
    deltas.push(frameSamples[i] - frameSamples[i - 1])
  }
  if (!deltas.length) return
  const avg = deltas.reduce((a, b) => a + b, 0) / deltas.length
  const fps = avg > 0 ? 1000 / avg : 0
  const sorted = [...deltas].sort((a, b) => b - a)
  const onePct = sorted[Math.max(0, Math.floor(sorted.length * 0.01))] || sorted[0]
  const low = onePct > 0 ? 1000 / onePct : 0
  $('go-fps').textContent = fmt(fps, 0)
  $('go-low').textContent = fmt(low, 0)
  $('go-ft').textContent = fmt(avg, 1)
  void dt
}

async function loadGames() {
  try {
    gameCatalog = await ListGames() || []
  } catch (_) {
    gameCatalog = []
  }
  renderGameGrid()
}

function setSearchSpinner(on) {
  $('game-search-spinner')?.classList.toggle('hidden', !on)
}

function mergeGameLists(localList, onlineList) {
  const out = []
  const seen = new Set()
  for (const g of [...(localList || []), ...(onlineList || [])]) {
    if (!g || !g.id) continue
    const key = String(g.steamAppId > 0 ? `app:${g.steamAppId}` : g.id).toLowerCase()
    if (seen.has(key)) continue
    seen.add(key)
    out.push(g)
  }
  return out
}

async function searchGamesLocal(q) {
  try {
    if (!q) {
      gameCatalog = await ListGames() || []
    } else {
      gameCatalog = await SearchGames(q) || []
    }
  } catch (_) {
    gameCatalog = gameCatalog || []
  }
  renderGameGrid()
}

async function searchGamesOnlineMerge(q) {
  if (!q) return
  setSearchSpinner(true)
  try {
    const online = await SearchGamesOnline(q) || []
    const local = await SearchGames(q) || []
    gameCatalog = mergeGameLists(local, online)
    if (!gameCatalog.length) {
      gameCatalog = []
    }
  } catch (_) {
    // keep current local results
  } finally {
    setSearchSpinner(false)
  }
  renderGameGrid()
  if (gameCatalog.length === 1) {
    selectedGameId = gameCatalog[0].id
    await predictSelectedGame()
  }
}

async function searchGamesLive(opts = {}) {
  const online = !!opts.online
  const q = String($('game-search')?.value || '').trim()
  if (!q) {
    setSearchSpinner(false)
    gameCatalog = await ListGames() || []
    renderGameGrid()
    return
  }
  await searchGamesLocal(q)
  if (online || gameCatalog.length === 0) {
    await searchGamesOnlineMerge(q)
  }
}

function posterFallbackHTML(name, cls) {
  const safe = String(name || 'Game').replace(/</g, '&lt;').replace(/>/g, '&gt;')
  return `<div class="${cls} fallback">${safe}</div>`
}

function steamHeaderFromPoster(url) {
  if (!url) return ''
  return String(url)
    .replace('/library_600x900.jpg', '/header.jpg')
    .replace('/library_600x900_2x.jpg', '/header.jpg')
}

function steamCdnUrls(steamAppId) {
  const id = Number(steamAppId)
  if (!Number.isFinite(id) || id <= 0) return { poster: '', header: '' }
  return {
    poster: `https://cdn.cloudflare.steamstatic.com/steam/apps/${id}/library_600x900.jpg`,
    header: `https://cdn.cloudflare.steamstatic.com/steam/apps/${id}/header.jpg`,
  }
}

function resolveGameArt(g) {
  const cdn = steamCdnUrls(g?.steamAppId)
  const poster = g?.posterUrl || cdn.poster || ''
  const header = g?.headerUrl || cdn.header || steamHeaderFromPoster(poster) || ''
  return { poster, header }
}

function gamePosterHTML(g) {
  const name = String(g.name || 'Game').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;')
  const safeName = String(g.name || 'Game').replace(/\\/g, '\\\\').replace(/'/g, "\\'")
  const { poster: primary, header: secondary } = resolveGameArt(g)
  if (!primary && !secondary) return posterFallbackHTML(g.name, 'game-poster')
  const src = primary || secondary
  const fb = secondary && secondary !== src ? secondary : ''
  return `<img class="game-poster" src="${src}" alt="${name}" loading="lazy" data-fb="${fb}" onerror="if(this.dataset.fb){const u=this.dataset.fb;this.dataset.fb='';this.src=u;}else{this.replaceWith(Object.assign(document.createElement('div'),{className:'game-poster fallback',textContent:'${safeName}'}))}" />`
}

function compactCoverHTML(p) {
  const name = String(p.gameName || 'Game')
  const safeName = name.replace(/\\/g, '\\\\').replace(/'/g, "\\'")
  const { poster: primary, header: secondary } = resolveGameArt(p)
  if (!primary && !secondary) return posterFallbackHTML(name, 'game-cover')
  const src = primary || secondary
  const fb = secondary && secondary !== src ? secondary : ''
  return `<img class="game-cover" src="${src}" alt="${name.replace(/"/g, '&quot;')}" data-fb="${fb}" onerror="if(this.dataset.fb){const u=this.dataset.fb;this.dataset.fb='';this.src=u;}else{this.replaceWith(Object.assign(document.createElement('div'),{className:'game-cover fallback',textContent:'${safeName}'}))}" />`
}

function renderGameGrid() {
  const host = $('game-grid')
  if (!host) return
  const list = gameCatalog || []
  const q = String($('game-search')?.value || '').trim()
  if (!list.length) {
    host.innerHTML = q
      ? `<div class="game-empty-card">${t('gameNotFound')}</div>`
      : `<div class="empty">—</div>`
    return
  }
  host.innerHTML = list.map((g) => `
    <button type="button" class="game-card ${g.id === selectedGameId ? 'active' : ''}" data-game="${g.id}" title="${String(g.name || '').replace(/"/g, '&quot;')}">
      ${gamePosterHTML(g)}
      <div class="game-card-body">
        <div class="g-name title">${g.name}</div>
      </div>
    </button>
  `).join('')
}

async function predictSelectedGame() {
  const box = $('fps-result')
  if (!box || !selectedGameId) return
  try {
    const p = await PredictGameFPS(selectedGameId)
    const recKey = ({ low: 'presetLow', med: 'presetMed', high: 'presetHigh', ultra: 'presetUltra' })[p.recommend] || 'presetMed'
    const warnKeys = Array.isArray(p.warnings) ? p.warnings : []
    const bottleneck = warnKeys.length ? t(warnKeys[0]) : ''
    const verdictKey = p.verdict || ''
    const verdictText = verdictKey ? t(verdictKey) : ''
    const verdictCls = p.verdictLevel === 'bad' ? 'bad' : (p.verdictLevel === 'ok' ? 'ok' : 'warn')
    const show = (range, avg) => {
      if (range === '<5') return '<5'
      return (range && String(range).includes('-')) ? range : avg
    }
    const vramLabel = p.isIntegrated ? t('fpsSharedVRAM') : `${fmt(p.vramGB, 1)} GB`
    const ramLabel = p.dualChannel ? t('fpsDualChannel') : t('fpsSingleChannel')
    const scores = `GPU <b>${fmt(p.gpuScore, 0)}</b> · CPU <b>${fmt(p.cpuScore, 0)}</b> · VRAM <b>${vramLabel}</b> · RAM <b>${ramLabel}</b>`
    const badge = p.badge || p.genre || p.category || ''
    const statusBits = [
      `${t('fpsRec')}: <b>${t(recKey)}</b>`,
      verdictText ? `<b>${verdictText}</b>` : '',
      bottleneck ? `⚠ ${bottleneck}` : '',
    ].filter(Boolean).join(' · ')
    box.className = 'fps-result'
    box.innerHTML = `
      <div class="game-compact">
        ${compactCoverHTML(p)}
        <div class="game-compact-body">
          <div class="game-compact-title">${p.gameName || ''}</div>
          <div class="game-compact-meta">
            <span class="game-badge era">${badge}</span>
            ${p.engine ? `<span class="game-badge">${p.engine}</span>` : ''}
            ${p.year ? `<span class="game-badge">${p.year}</span>` : ''}
          </div>
          <div class="game-compact-scores">${scores}</div>
        </div>
      </div>
      <div class="fps-grid">
        <div class="fps-tile ${p.recommend === 'low' ? 'rec' : ''}"><div class="v">${show(p.lowRange, p.lowAvg)}</div><div class="q">${t('fpsLow')}</div></div>
        <div class="fps-tile ${p.recommend === 'med' ? 'rec' : ''}"><div class="v">${show(p.medRange, p.medAvg)}</div><div class="q">${t('fpsMed')}</div></div>
        <div class="fps-tile ${p.recommend === 'high' ? 'rec' : ''}"><div class="v">${show(p.highRange, p.highAvg)}</div><div class="q">${t('fpsHigh')}</div></div>
        <div class="fps-tile ${p.recommend === 'ultra' ? 'rec' : ''}"><div class="v">${show(p.ultraRange, p.ultraAvg)}</div><div class="q">${t('fpsUltra')}</div></div>
      </div>
      <div class="fps-status-bar ${verdictCls}">${statusBits}</div>
    `
  } catch (err) {
    box.className = 'fps-result empty'
    box.textContent = String(err)
  }
}

function openAbout() {
  const name = $('about-app-name')
  const ver = $('about-app-ver')
  if (name) name.textContent = 'PulseCore'
  if (ver) ver.textContent = APP_VERSION
  const link = $('about-github')
  if (link) {
    link.href = GITHUB_URL
    link.textContent = 'github.com/robbled17/pulsecore'
  }
  const box = $('license-box')
  if (box) box.textContent = MIT_LICENSE
  $('about-modal')?.classList.remove('hidden')
}

function closeAbout() {
  $('about-modal')?.classList.add('hidden')
}

function buildPCConfigLine(m = lastMetrics) {
  const hw = m?.hardware || {}
  const cpu = hw.cpu?.name || m?.cpuName || na()
  const gpu = hw.gpu?.name || m?.gpuName || na()
  const ramMB = hw.ram?.totalMB || m?.ramTotalMB
  const ram = ramMB ? `${Math.round(Number(ramMB) / 1024)} GB RAM` : na()
  const os = m?.osName || m?.os || 'Windows'
  return `CPU: ${cpu} | GPU: ${gpu} | RAM: ${ram} | OS: ${os}`
}

async function copyPCConfig() {
  const line = buildPCConfigLine()
  try {
    await navigator.clipboard.writeText(line)
    showToast(t('aboutCopied'), true)
  } catch (_) {
    try {
      const ta = document.createElement('textarea')
      ta.value = line
      ta.style.position = 'fixed'
      ta.style.opacity = '0'
      document.body.appendChild(ta)
      ta.select()
      document.execCommand('copy')
      ta.remove()
      showToast(t('aboutCopied'), true)
    } catch (err) {
      showToast(String(err), false)
    }
  }
}

function showToast(text, ok = false) {
  const el = $('toast')
  el.textContent = text
  el.classList.toggle('ok', !!ok)
  el.classList.remove('hidden')
  clearTimeout(showToast._t)
  showToast._t = setTimeout(() => el.classList.add('hidden'), 4200)
}

function openModal(html) {
  $('modal-body').innerHTML = html
  $('modal').classList.remove('hidden')
}

function closeModal() {
  $('modal').classList.add('hidden')
}

function renderDiskHealthBody(r) {
  const ok = String(r.verdictBadge || r.verdict || '').includes('Отлич') || String(r.verdict || '').includes('Хорош') || String(r.verdict || '').toLowerCase().includes('good')
  const tbw = r.tbwGB > 1024 ? `${fmt(r.tbwGB / 1024, 2)} TB` : `${fmt(r.tbwGB || 0, 1)} GB`
  const hours = r.powerOnHours > 0 ? `${fmt(r.powerOnHours, 0)} h` : '-'
  return `
    <div class="health-badge ${ok ? 'ok' : 'warn'}">${r.verdictBadge || r.verdict || '-'}</div>
    <div class="modal-row"><span class="k">${t('lblModel')}</span><span class="v">${r.model || '-'}</span></div>
    <div class="modal-row"><span class="k">${t('serial')}</span><span class="v">${r.serialNumber || '-'}</span></div>
    <div class="modal-row"><span class="k">${t('iface')}</span><span class="v">${r.interfaceType || r.mediaType || '-'}</span></div>
    <div class="modal-row"><span class="k">${t('temp')}</span><span class="v">${(() => { const tt = fmtTemp(r.temperatureC, 0); return tt === na() ? t('noControllerTemp') : tt + ' °C' })()}</span></div>
    <div class="modal-row"><span class="k">${t('tbw')}</span><span class="v">${r.tbwGB > 0 ? tbw : t('noData')}</span></div>
    <div class="modal-row"><span class="k">${t('powerOn')}</span><span class="v">${hours}</span></div>
    <div class="modal-row"><span class="k">${t('wearLeft')}</span><span class="v">${r.wearRemaining >= 0 ? fmt(r.wearRemaining, 0) + '%' : '-'}</span></div>
    <div class="modal-row"><span class="k">${t('critErr')}</span><span class="v">${r.hasCriticalErrors ? t('yes') : t('no')}</span></div>
  `
}

async function runDiskHealth(drive, force = false) {
  if (drive) selectedDrive = driveLetter(drive)
  const key = (selectedDrive || 'C').toUpperCase()
  renderDiskTabs(latestDisks, 'modal-disk-tabs')

  if (!force && cachedDiskReport[key]) {
    openModal(renderDiskHealthBody(cachedDiskReport[key]))
    return
  }

  openModal(`<div class="empty">${t('checking')}</div>`)
  try {
    const r = await CheckDiskHealth(key)
    if (!r || r.ok === false) {
      openModal(`<div class="modal-row"><span class="k">${t('error')}</span><span class="v">${r?.errorMessage || t('noData')}</span></div>`)
      return
    }
    cachedDiskReport[key] = r
    openModal(renderDiskHealthBody(r))
  } catch (err) {
    openModal(`<div class="modal-row"><span class="k">${t('error')}</span><span class="v">${String(err)}</span></div>`)
  }
}

async function refresh() {
  try {
    const m = await GetMetrics()
    applyMetrics(m)
  } catch (err) {
    console.error(err)
  }
}

function wireUI() {
  document.querySelectorAll('.nav-item').forEach((btn) => {
    btn.addEventListener('click', () => setActiveTab(btn.dataset.tab))
  })

  wireTooltips()

  document.querySelectorAll('.chart-tab').forEach((card) => {
    card.addEventListener('click', () => {
      activeMetric = card.dataset.metric || 'cpu'
      document.querySelectorAll('.chart-tab').forEach((c) => c.classList.toggle('active', c === card))
      updateChartTitle()
    })
  })

  // Keep expand-panel action buttons from toggling <details>
  document.querySelectorAll('.expand-summary .btn').forEach((btn) => {
    btn.addEventListener('click', (ev) => ev.stopPropagation())
  })

  $('btn-min').addEventListener('click', () => WindowMinimise())
  $('btn-fs').addEventListener('click', async () => {
    try { await ToggleFullscreen() } catch (_) {}
  })
  $('btn-close').addEventListener('click', async () => {
    try { await HideToTray() } catch (_) {}
  })

  document.addEventListener('keydown', async (ev) => {
    if (ev.key === 'F11') {
      ev.preventDefault()
      try { await ToggleFullscreen() } catch (_) {}
    }
    if (ev.key === 'F12') {
      ev.preventDefault()
      if (gameOverlayOn) await disableGameOverlay()
      else await enableGameOverlay()
    }
  })

  $('lang-select-settings')?.addEventListener('change', (ev) => changeLanguage(ev.target.value))

  document.querySelectorAll('.theme-btn').forEach((btn) => {
    btn.addEventListener('click', () => applyTheme(btn.dataset.theme))
  })

  document.querySelectorAll('.btn.mode').forEach((btn) => {
    btn.addEventListener('click', async (ev) => {
      if (ev.target.closest('.tip')) return
      const mode = btn.dataset.mode
      try {
        await SetPowerProfile(mode)
        document.querySelectorAll('.btn.mode').forEach((b) => b.classList.toggle('active', b === btn))
        showToast(t('modeApplied'), true)
      } catch (err) {
        showToast(String(err), false)
      }
    })
  })

  $('btn-reset-profile')?.addEventListener('click', async () => {
    try {
      await SetPowerProfile('balanced')
      document.querySelectorAll('.btn.mode').forEach((b) => {
        b.classList.toggle('active', b.dataset.mode === 'balanced')
      })
      showToast(t('modeApplied'), true)
    } catch (err) {
      showToast(String(err), false)
    }
  })

  document.querySelectorAll('.btn.poll').forEach((btn) => {
    btn.addEventListener('click', async () => {
      await SetPollInterval(Number(btn.dataset.poll))
      document.querySelectorAll('.btn.poll').forEach((b) => b.classList.toggle('active', b === btn))
    })
  })

  const enableHud = async () => {
    try { await ToggleMiniHUD(true) } catch (_) {}
    setHudMode(true)
  }
  const disableHud = async () => {
    try { await ToggleMiniHUD(false) } catch (_) {}
    setHudMode(false)
  }

  $('btn-hud-title')?.addEventListener('click', enableHud)
  $('mh-expand').addEventListener('click', disableHud)
  $('btn-toggle-overlay')?.addEventListener('click', toggleGameOverlayPill)
  $('btn-overlay')?.addEventListener('click', enableGameOverlay)
  $('go-close')?.addEventListener('click', disableGameOverlay)
  syncOverlayPill(gameOverlayOn)
  $('btn-about')?.addEventListener('click', openAbout)
  $('about-close')?.addEventListener('click', closeAbout)
  $('about-modal')?.addEventListener('click', (ev) => {
    if (ev.target.id === 'about-modal') closeAbout()
  })
  $('btn-copy-pc')?.addEventListener('click', copyPCConfig)

  // Demo-only hardware matrix switcher (present only when mock bindings loaded)
  const mockBar = $('mock-profile-bar')
  if (mockBar && window.__PULSECORE_MOCK__) {
    mockBar.classList.remove('hidden')
    const syncMockBtns = () => {
      const cur = window.__PULSECORE_MOCK__.getProfile()
      mockBar.querySelectorAll('.mock-profile-btn').forEach((b) => {
        b.classList.toggle('active', b.dataset.profile === cur)
      })
    }
    mockBar.querySelectorAll('.mock-profile-btn').forEach((btn) => {
      btn.addEventListener('click', async () => {
        window.__PULSECORE_MOCK__.setProfile(btn.dataset.profile)
        syncMockBtns()
        if (selectedGameId) {
          try { await predictSelectedGame() } catch (_) {}
        }
      })
    })
    syncMockBtns()
  }

  const onInsightAct = async (ev) => {
    const btn = ev.target.closest('[data-insight-act]')
    if (!btn) return
    const act = btn.dataset.insightAct
    try {
      if (act === 'power-silent') {
        await SetPowerProfile('silent')
        document.querySelectorAll('.btn.mode').forEach((b) => {
          b.classList.toggle('active', b.dataset.mode === 'silent')
        })
        showToast(t('modeApplied'), true)
      } else if (act === 'eco-vampires') {
        const vamps = (lastMetrics?.vampires || []).filter((v) => !v.ecoMode && !v.frozen)
        for (const v of vamps.slice(0, 4)) {
          try {
            if (v.name) await EcoModeByProcessName(v.name, true)
            else if (v.pid) await FreezeProcess(v.pid)
            if (v.pid) frozen.add(v.pid)
          } catch (_) {}
        }
        showToast(t('insightEcoDone'), true)
        refresh()
      } else if (act === 'eco-pid') {
        const pid = Number(btn.dataset.pid)
        if (pid) {
          await FreezeProcess(pid)
          frozen.add(pid)
          showToast(t('insightEcoDone'), true)
          refresh()
        }
      } else if (act === 'eco-name') {
        const name = btn.dataset.ecoName
        if (name) {
          await EcoModeByProcessName(name, true)
          showToast(t('insightEcoDone'), true)
          refresh()
        }
      } else if (act === 'tab-lag') {
        setActiveTab('lag')
      } else if (act === 'tab-weekly') {
        setActiveTab('overview')
        $('weekly-list')?.scrollIntoView({ behavior: 'smooth', block: 'center' })
      } else if (act === 'disk-smart') {
        runDiskHealth(selectedDrive || 'C', false)
      } else if (act === 'disk-cleanup') {
        setActiveTab('overview')
        $('ov-disks')?.scrollIntoView({ behavior: 'smooth', block: 'center' })
        showToast(t('insightCleanupHint'), true)
      }
    } catch (err) {
      showToast(String(err), false)
    }
  }
  $('insight-list')?.addEventListener('click', onInsightAct)
  $('lag-insight-list')?.addEventListener('click', onInsightAct)

  ;['cpu', 'gpu', 'ram', 'temps'].forEach((key) => {
    $(`mh-opt-${key}`)?.addEventListener('change', (ev) => {
      hudPrefs[key] = !!ev.target.checked
      // keep at least one metric visible
      if (!hudPrefs.cpu && !hudPrefs.gpu && !hudPrefs.ram && !hudPrefs.temps) {
        hudPrefs[key] = true
        ev.target.checked = true
      }
      saveHudPrefs()
      applyMiniHudPrefs()
    })
  })
  $('mh-opacity')?.addEventListener('input', (ev) => {
    hudPrefs.opacity = Number(ev.target.value) || 78
    saveHudPrefs()
    applyMiniHudPrefs()
  })
  applyMiniHudPrefs()

  $('btn-disk-health').addEventListener('click', () => runDiskHealth(selectedDrive, false))
  $('modal-smart-refresh')?.addEventListener('click', () => runDiskHealth(selectedDrive, true))
  $('btn-weekly-refresh')?.addEventListener('click', refreshWeekly)
  $('btn-periph-refresh')?.addEventListener('click', refreshPeripherals)
  $('btn-net-diag')?.addEventListener('click', runNetDiag)
  $('btn-sleep')?.addEventListener('click', runSleepCheck)
  $('modal-close').addEventListener('click', closeModal)
  $('modal').addEventListener('click', (ev) => {
    if (ev.target.id === 'modal') closeModal()
  })

  $('periph-list')?.addEventListener('click', (ev) => {
    const btn = ev.target.closest('[data-toggle-group]')
    if (!btn) return
    const g = btn.dataset.toggleGroup
    if (periphCollapsed.has(g)) periphCollapsed.delete(g)
    else periphCollapsed.add(g)
    localStorage.setItem('zp_periph_collapsed', JSON.stringify([...periphCollapsed]))
    const group = btn.closest('.periph-group')
    group?.classList.toggle('collapsed', periphCollapsed.has(g))
    const arrow = btn.querySelector('.arrow')
    if (arrow) arrow.textContent = periphCollapsed.has(g) ? '►' : '▼'
  })

  let searchTimer = null
  const searchInput = $('game-search')
  searchInput?.addEventListener('input', () => {
    clearTimeout(searchTimer)
    const q = String(searchInput.value || '').trim()
    if (!q) {
      setSearchSpinner(false)
      searchGamesLive({ online: false })
      return
    }
    // Local filter immediately, Steam online after 300ms pause.
    searchGamesLocal(q)
    searchTimer = setTimeout(() => searchGamesLive({ online: true }), 300)
  })
  searchInput?.addEventListener('keydown', async (ev) => {
    if (ev.key !== 'Enter') return
    clearTimeout(searchTimer)
    await searchGamesLive({ online: true })
  })

  const carousel = $('games-carousel-wrapper') || document.querySelector('.games-carousel-wrapper')
  if (carousel) {
    carousel.addEventListener('wheel', (e) => {
      e.preventDefault()
      const step = 206
      carousel.scrollBy({
        top: e.deltaY > 0 ? step : -step,
        behavior: 'smooth',
      })
    }, { passive: false })
  }

  $('game-grid')?.addEventListener('click', async (ev) => {
    const card = ev.target.closest('[data-game]')
    if (!card) return
    selectedGameId = card.dataset.game
    renderGameGrid()
    await predictSelectedGame()
  })

  $('lag-log')?.addEventListener('click', async (ev) => {
    const btn = ev.target.closest('button[data-pid]')
    if (!btn) return
    const pid = Number(btn.dataset.pid)
    try {
      await FreezeProcess(pid)
      frozen.add(pid)
      btn.textContent = t('ecoLimited')
      btn.classList.remove('danger')
      btn.classList.add('eco-on')
    } catch (_) {}
  })

  const onDiskTabClick = async (ev) => {
    const tab = ev.target.closest('[data-drive]')
    if (!tab) return
    selectedDrive = tab.dataset.drive
    renderDiskTabs(latestDisks, 'disk-tabs')
    renderDiskTabs(latestDisks, 'modal-disk-tabs')
    if (!$('modal').classList.contains('hidden')) {
      await runDiskHealth(selectedDrive, false)
    } else {
      refresh()
    }
  }
  $('disk-tabs')?.addEventListener('click', onDiskTabClick)
  $('modal-disk-tabs')?.addEventListener('click', onDiskTabClick)

  $('weekly-list')?.addEventListener('click', async (ev) => {
    const eco = ev.target.closest('[data-weekly-eco]')
    const hide = ev.target.closest('[data-weekly-hide]')
    if (eco) {
      const name = eco.dataset.weeklyEco
      try {
        await EcoModeByProcessName(name, true)
      } catch (_) {
        // No kernel/PPL error toasts — button still reflects intended outcome for Defender.
      }
      eco.textContent = isDefenderName(name) ? t('ecoLimited15') : t('ecoLimited')
      eco.classList.remove('danger')
      eco.classList.add('eco-on')
    }
    if (hide) {
      whitelist.add(String(hide.dataset.weeklyHide || '').toLowerCase())
      localStorage.setItem('zp_whitelist', JSON.stringify([...whitelist]))
      refreshWeekly()
    }
  })

  $('stutter-log')?.addEventListener('click', async (ev) => {
    const btn = ev.target.closest('[data-eco-name]')
    if (!btn) return
    const name = btn.dataset.ecoName
    try {
      await EcoModeByProcessName(name, true)
    } catch (_) {}
    btn.textContent = isDefenderName(name) ? t('ecoLimited15') : t('ecoLimited')
    btn.classList.remove('danger')
    btn.classList.add('eco-on')
  })

  $('vampire-list').addEventListener('click', async (ev) => {
    const btn = ev.target.closest('button[data-vamp-eco], button[data-pid]')
    if (!btn) return
    const name = btn.dataset.vampEco
    const pid = Number(btn.dataset.pid)
    try {
      if (btn.dataset.act === 'freeze') {
        if (name) await EcoModeByProcessName(name, true)
        else await FreezeProcess(pid)
        if (pid) frozen.add(pid)
        btn.textContent = t('ecoLimited')
        btn.classList.remove('danger')
        btn.classList.add('eco-on')
      } else {
        if (name) await EcoModeByProcessName(name, false)
        else await ResumeProcess(pid)
        if (pid) frozen.delete(pid)
      }
      refresh()
    } catch (_) {}
  })
}

function wireTooltips() {
  const floatEl = $('tip-float')
  const place = (tip) => {
    const text = tip.getAttribute('data-tip') || ''
    if (!text || !floatEl) return
    floatEl.textContent = text
    floatEl.classList.remove('hidden')
    const rect = tip.getBoundingClientRect()
    const width = Math.min(280, window.innerWidth - 24)
    let left = rect.left
    if (left < 12) left = 12
    if (left + width > window.innerWidth - 12) left = window.innerWidth - width - 12
    // Near left edge / sidebar — nudge right.
    if (rect.left < 200) left = Math.max(left, 12)
    floatEl.style.width = `${width}px`
    floatEl.style.left = `${left}px`
    floatEl.style.transform = rect.left < 24 ? 'translateX(10px)' : 'none'
    requestAnimationFrame(() => {
      const h = floatEl.offsetHeight || 60
      let top = rect.top - h - 8
      if (top < 8) top = rect.bottom + 8
      floatEl.style.top = `${top}px`
    })
  }

  document.addEventListener('mouseover', (ev) => {
    const tip = ev.target.closest('.tip')
    if (!tip) return
    place(tip)
  })
  document.addEventListener('mouseout', (ev) => {
    const tip = ev.target.closest('.tip')
    if (!tip) return
    if (ev.relatedTarget && tip.contains(ev.relatedTarget)) return
    floatEl?.classList.add('hidden')
  })
}

function wireAmbientParallax() {
  const root = document.documentElement
  let raf = 0
  let latestX = 0
  let latestY = 0
  const publish = () => {
    raf = 0
    root.style.setProperty('--mouse-x', latestX.toFixed(4))
    root.style.setProperty('--mouse-y', latestY.toFixed(4))
  }
  window.addEventListener('pointermove', (ev) => {
    const w = window.innerWidth || 1
    const h = window.innerHeight || 1
    latestX = (ev.clientX / w) * 2 - 1
    latestY = (ev.clientY / h) * 2 - 1
    if (!raf) raf = requestAnimationFrame(publish)
  }, { passive: true })
  window.addEventListener('pointerleave', () => {
    latestX = 0
    latestY = 0
    if (!raf) raf = requestAnimationFrame(publish)
  }, { passive: true })
}

function boot() {
  applyTheme(uiTheme)
  applyI18n()
  wireUI()
  wireAmbientParallax()
  SetLanguage(lang).catch(() => {})
  refresh()
  refreshWeekly()
  refreshPeripherals()
  loadGames()
  setInterval(refresh, 1000)
  requestAnimationFrame(animateChart)

  try {
    EventsOn('metrics:update', (m) => applyMetrics(m))
    EventsOn('thermal:alarm', () => showToast(t('thermal')))
    EventsOn('ui:minihud', (enabled) => setHudMode(!!enabled))
    EventsOn('ui:gameoverlay', (enabled) => setGameOverlayMode(!!enabled, true))
    EventsOn('peripherals:update', (items) => renderPeripherals(items || []))
  } catch (_) {}
}

boot()
