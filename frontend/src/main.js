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
  GetUpgradeAdvice,
  ListInstalledApps,
  PrecheckUninstall,
  UninstallApp,
  ScanResiduals,
  DeleteResiduals,
  StartTracedInstall,
  FinishTracedInstall,
  ListInstallTraces,
  CleanupFromTrace,
  CleanSafeDiskCaches,
  PreviewSafeDiskCaches,
  OpenAppInstallFolder,
  ScanSystemClean,
  CleanSystemCategories,
  GetSensorLogs,
  ClearSensorLogs,
  ForceRescanHardware,
  CheckAdminStatus,
  ResetAppCache,
  GetDevTelemetry,
  TitlebarDragBegin,
  TitlebarToggleMaximise,
  WindowMaximisedState,
  RunSelfDiagnostics,
  SetMiniHUDHotkey,
  GetMiniHUDHotkey,
  IsElevated,
  RestartAsAdministrator,
  CheckForUpdates,
} from '../wailsjs/go/main/App.js'
import { BrowserOpenURL, EventsOn, WindowMinimise, WindowSetSize, WindowToggleMaximise, WindowIsMaximised } from '../wailsjs/runtime/runtime.js'
import { i18n } from './i18n.js'
import {
  initDevMode,
  getDevState,
  isDevModeActive,
} from './devMode.js'

const DEV_MODE_ENABLED = true

const MIT_LICENSE = `PulseCore v1.0.1

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

const HISTORY_LEN = 36
const BUFFER_DELAY_MS = 650
const LERP = 0.14
const EMA_ALPHA = 0.22

const chartThemes = {
  cpu: { stroke: '#8ec0ff', glow: 'rgba(142, 192, 255, 0.95)', top: 'rgba(110, 168, 255, 0.55)', bottom: 'rgba(110, 168, 255, 0.04)' },
  gpu: { stroke: '#5ff0d4', glow: 'rgba(95, 240, 212, 0.95)', top: 'rgba(61, 231, 200, 0.5)', bottom: 'rgba(61, 231, 200, 0.04)' },
  ram: { stroke: '#b49bff', glow: 'rgba(180, 155, 255, 0.95)', top: 'rgba(155, 123, 255, 0.5)', bottom: 'rgba(155, 123, 255, 0.04)' },
  battery: { stroke: '#ffd166', glow: 'rgba(255, 209, 102, 0.9)', top: 'rgba(251, 191, 36, 0.45)', bottom: 'rgba(251, 191, 36, 0.04)' },
}

const APP_VERSION = 'v1.0.0'
const GITHUB_URL = 'https://github.com/robbled17/pulsecore'
const DEFAULT_EXE_URL = 'https://github.com/robbled17/pulsecore/releases/latest/download/pulsecore.exe'

let lang = localStorage.getItem('zp_lang') || 'ru'
let uiTheme = localStorage.getItem('zp_theme') || 'dark'
let frozen = new Set()
let activeMetric = 'cpu'
let selectedDrive = 'C'
let hudEnabled = false
let gameOverlayOn = false
let latestDisks = []
let gameCatalog = []
let fullGameCatalog = []
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
      hotkey: typeof raw.hotkey === 'string' && raw.hotkey.trim() ? raw.hotkey.trim() : 'Ctrl+Shift+M',
    }
  } catch (_) {
    return { cpu: true, gpu: true, ram: true, temps: true, opacity: 78, hotkey: 'Ctrl+Shift+M' }
  }
}
let hudPrefs = loadHudPrefs()
let lastDiagReport = null

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
  try {
    const v = t('na')
    if (v && v !== 'na') return v
  } catch (_) {}
  return lang === 'ru' ? 'Н/Д' : 'N/A'
}

function sensorUnavailableLabel() {
  try {
    const v = t('sensorUnavailable')
    if (v && v !== 'sensorUnavailable') return v
  } catch (_) {}
  return na()
}

function dash() {
  return '-'
}

async function changeLanguage(next) {
  lang = next || 'ru'
  localStorage.setItem('zp_lang', lang)
  try { await SetLanguage(lang) } catch (_) {}
  applyI18n()
  hwSSDSignature = '' // force SSD accordion labels to rebuild for new lang
  refresh()
  refreshWeekly()
  refreshPeripherals()
  refreshUpgradeAdvisor()
}

let activeTabName = 'overview'

function setActiveTab(name) {
  if (name === 'dev' && !isDevModeActive()) return
  const prev = activeTabName
  activeTabName = name
  document.querySelectorAll('.nav-item').forEach((btn) => {
    btn.classList.toggle('active', btn.dataset.tab === name)
  })
  document.querySelectorAll('.tab').forEach((tab) => {
    tab.classList.toggle('active', tab.id === `tab-${name}`)
  })
  // Drop heavy GamePulse DOM while the tab is inactive (lazy posters reload on return).
  if (prev === 'gamelab' && name !== 'gamelab') {
    const host = $('game-grid')
    if (host) host.innerHTML = ''
  }
  if (name === 'gamelab') renderGameGrid()
  if (name === 'hardware') refreshPeripherals()
  if (name === 'upgrade') refreshUpgradeAdvisor()
  if (name === 'uninstall') {
    refreshInstalledApps()
    refreshTraceList()
    if (!systemCleanScanResult) {
      scanSystemClean()
    }
  }
  if (name === 'dev') {
    refreshDevTelemetry()
  }
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

  // Keep X locked to sample index; only Y is lerped, so Bezier stays stable.
  const pts = buildPoints(values, cssW, cssH, padL, padR, padT, padB)

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

const CHART_FPS = 24
const CHART_FRAME_MS = 1000 / CHART_FPS
let chartActive = true
let lastChartDraw = 0
let chartRaf = 0

function setChartActive(on) {
  chartActive = !!on
  if (chartActive && !chartRaf) {
    chartRaf = requestAnimationFrame(animateChart)
  }
}

function animateChart(now) {
  chartRaf = 0
  if (!chartActive) return
  // Fully pause when tab/window is hidden (tray minimize or inactive).
  if (typeof document !== 'undefined' && document.hidden) {
    chartRaf = requestAnimationFrame(animateChart)
    return
  }
  if (!lastChartDraw || (now - lastChartDraw) >= CHART_FRAME_MS) {
    lastChartDraw = now
    drainQueues(now)
    lerpSeries()
    drawTrendFrame()
    renderSparklines()
    sampleFrametime(now)
  }
  chartRaf = requestAnimationFrame(animateChart)
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

/** Reject null/NaN/0/-1 and absurd temps: always Н/Д or '-'. */
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

  fillSmooth(ctx, pts, cssH, top, bottom)

  ctx.save()
  ctx.lineWidth = 3
  ctx.lineJoin = 'round'
  ctx.lineCap = 'round'
  ctx.strokeStyle = glow
  ctx.shadowColor = glow
  ctx.shadowBlur = 12
  ctx.globalAlpha = 0.5
  strokeSmooth(ctx, pts)
  ctx.restore()

  ctx.lineWidth = 1.8
  ctx.lineJoin = 'round'
  ctx.lineCap = 'round'
  ctx.strokeStyle = stroke
  ctx.shadowColor = glow
  ctx.shadowBlur = 8
  strokeSmooth(ctx, pts)
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
  const vamps = filterNoiseVampires(m.vampires || [])
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

let insightsExpanded = false

function insightPriority(it) {
  if (it.id === 'thermal') return it.level === 'hot' ? 100 : 90
  if (it.id === 'ram' && it.level === 'hot') return 85
  if (it.id === 'disk') return 70
  if (it.id === 'lag' || it.id === 'stutter') return 60
  if (it.id === 'ram') return 55
  return 40
}

function collectInsights(m) {
  const items = []
  const dev = getDevState()
  const throttleC = Number(dev.throttleC) || 85
  const ramWarn = Number(dev.ramWarn) || 85
  const diskWarn = Number(dev.diskWarn) || 90
  const temp = validTemp(m.temperatureC, m.tempAvailable)
  const ramPct = safeNum(m.ramPercent, 0)
  const disks = m.disks || []
  const sysDisk = disks.find((d) => /^C/i.test(String(d.mount || ''))) || disks[0]
  const diskPct = safeNum(sysDisk?.percent, 0)
  const lags = filterNoiseLags(m.lagEvents || [])
  const stutters = filterNoiseLags((m.stutterEvents || []).filter((e) => safeNum(e.jitterMs, 0) >= 300))
  const vamps = filterNoiseVampires(m.vampires || []).filter((v) => !v.ecoMode && !v.frozen)

  if (Number.isFinite(temp) && temp >= throttleC) {
    items.push({
      id: 'thermal',
      level: temp >= throttleC + 5 ? 'hot' : 'warn',
      title: `${t('insightThermalShort')} • ${fmtTemp(temp, 0)}°C`,
      body: '',
      actions: [
        { id: 'power-silent', label: t('insightActSilent') },
        { id: 'eco-vampires', label: t('insightActEcoBg') },
      ].slice(0, 2),
    })
  }

  if (ramPct >= ramWarn) {
    items.push({
      id: 'ram',
      level: ramPct >= Math.min(99, ramWarn + 5) ? 'hot' : 'warn',
      title: `${t('insightRamShort')} • ${fmt(ramPct, 0)}%`,
      body: '',
      actions: [
        { id: 'tab-weekly', label: t('insightActTopProcs') },
        { id: 'eco-vampires', label: t('ecoAppetite') },
      ].slice(0, 2),
    })
  }

  if (sysDisk && diskPct >= diskWarn) {
    items.push({
      id: 'disk',
      level: 'warn',
      title: `${t('insightDiskShort')} • ${fmt(diskPct, 0)}%`,
      body: '',
      actions: [
        { id: 'disk-cleanup', label: t('insightActCleanup') },
        ...(dev.smartEnabled !== false ? [{ id: 'disk-smart', label: t('diskHealthBtn') }] : []),
      ].slice(0, 2),
    })
  }

  if (lags.length) {
    const e = lags[0]
    const name = e.culpritFriendly || e.culpritProcess || t('unknownProc')
    const canEco = e.culpritPID && e.culpritProcess && !isNoiseBackgroundProcess(e.culpritProcess)
    items.push({
      id: 'lag',
      level: 'warn',
      title: `${t('insightLagShort')} • ${name}`,
      body: '',
      actions: [
        ...(canEco ? [{ id: 'eco-pid', label: t('ecoLower'), pid: e.culpritPID, name: e.culpritProcess }] : []),
        { id: 'tab-lag', label: t('insightActOpenLags') },
      ].slice(0, 2),
    })
  } else if (stutters.length) {
    items.push({
      id: 'stutter',
      level: 'warn',
      title: t('insightStutterShort'),
      body: '',
      actions: [{ id: 'tab-lag', label: t('insightActOpenLags') }],
    })
  }

  if (vamps.length >= 3 && !items.some((i) => i.id === 'ram' || i.id === 'thermal')) {
    items.push({
      id: 'vampires',
      level: 'warn',
      title: `${t('insightVampShort')} • ${vamps.length}`,
      body: '',
      actions: [
        { id: 'eco-vampires', label: t('insightActEcoBg') },
        { id: 'tab-lag', label: t('insightActOpenLags') },
      ].slice(0, 2),
    })
  }

  return items.sort((a, b) => insightPriority(b) - insightPriority(a))
}

function insightCardHTML(it) {
  return `
    <div class="insight-card ${it.level || 'warn'}" data-insight="${it.id}">
      <div class="insight-text">
        <div class="insight-title">${it.title}</div>
      </div>
      <div class="insight-actions">
        ${(it.actions || []).slice(0, 2).map((a) => `
          <button type="button" class="btn compact insight-btn ${a.id.startsWith('eco') ? 'danger' : ''}"
            data-insight-act="${a.id}"
            ${a.pid != null ? `data-pid="${a.pid}"` : ''}
            ${a.name ? `data-eco-name="${a.name}"` : ''}>${a.label}</button>
        `).join('')}
      </div>
    </div>
  `
}

function renderInsightHost(hostId, items, { collapse = false } = {}) {
  const host = $(hostId)
  if (!host) return
  const moreBtn = hostId === 'insight-list' ? $('insight-more') : null
  if (!items.length) {
    host.innerHTML = ''
    host.classList.add('hidden')
    if (moreBtn) {
      moreBtn.classList.add('hidden')
      moreBtn.hidden = true
    }
    return
  }
  host.classList.remove('hidden')
  const visible = collapse && !insightsExpanded ? items.slice(0, 1) : items
  host.innerHTML = visible.map(insightCardHTML).join('')
  if (moreBtn) {
    const hiddenN = items.length - 1
    if (collapse && hiddenN > 0) {
      moreBtn.hidden = false
      moreBtn.classList.remove('hidden')
      moreBtn.textContent = insightsExpanded
        ? t('insightShowLess')
        : t('insightMore').replace('{n}', String(hiddenN))
    } else {
      moreBtn.classList.add('hidden')
      moreBtn.hidden = true
    }
  }
}

function renderInsights(m) {
  const items = collectInsights(m)
  renderInsightHost('insight-list', items, { collapse: true })
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

function formatCardTemp(raw, available) {
  if (available === false) return ''
  const temp = validTemp(raw, available)
  if (!Number.isFinite(temp)) return ''
  return `${fmtTemp(temp, 0)}°C`
}

function renderOverview(m) {
  $('ov-cpu').textContent = fmtPct(m.cpuPercent)
  $('ov-cpu-sub').textContent = m.cpuName || dash()
  $('ov-cpu-bar').style.width = `${Math.min(100, safeNum(m.cpuPercent, 0))}%`
  const cpuTempEl = $('ov-cpu-temp')
  if (cpuTempEl) {
    const cpuTemp = formatCardTemp(m.temperatureC, m.tempAvailable)
    cpuTempEl.textContent = cpuTemp
    cpuTempEl.classList.toggle('hidden', !cpuTemp)
  }

  $('ov-gpu').textContent = fmtPct(m.gpuPercent)
  $('ov-gpu-sub').textContent = m.gpuName || dash()
  $('ov-gpu-bar').style.width = `${Math.min(100, safeNum(m.gpuPercent, 0))}%`
  const gpuTempEl = $('ov-gpu-temp')
  if (gpuTempEl) {
    const gpuTemp = formatCardTemp(m.gpuTempC, m.gpuTempAvailable)
    gpuTempEl.textContent = gpuTemp
    gpuTempEl.classList.toggle('hidden', !gpuTemp)
  }

  $('ov-ram').textContent = fmtPct(m.ramPercent)
  const used = safeNum(m.ramUsedMB, NaN)
  const total = safeNum(m.ramTotalMB, NaN)
  $('ov-ram-sub').textContent = (Number.isFinite(used) && Number.isFinite(total) && total > 0)
    ? `${fmt(used, 0)} / ${fmt(total, 0)} MB`
    : na()
  $('ov-ram-bar').style.width = `${Math.min(100, safeNum(m.ramPercent, 0))}%`

  const bat = m.battery || {}
  const batCard = document.querySelector('.metric-card[data-metric="battery"]')
  if (batCard) {
    batCard.classList.toggle('hidden', !bat.available)
  }
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

  const temp = m.tempAvailable === false ? sensorUnavailableLabel() : fmtTemp(m.temperatureC)
  $('temp-chip').textContent = (temp === na() || temp === sensorUnavailableLabel())
    ? sensorUnavailableLabel()
    : `${temp} °C`
  $('cpu-name').textContent = m.cpuName || 'CPU'

  renderStatusStrip(m)
  renderInsights(m)

  latestDisks = m.disks || []
  renderDiskTabs(latestDisks)
  renderHwSSDShell(latestDisks)
  renderOverviewDiskDetails()
  prefetchDiskHealthBadges(latestDisks)
}

function diskHealthBadge(letter) {
  const r = cachedDiskReport[String(letter || '').toUpperCase()]
  if (!r) return { health: '…', temp: '' }
  if (r.ok === false) return { health: t('ssdUnavailable'), temp: '' }
  const health = r.verdictBadge || r.verdict || t('diskTileHealth')
  const tt = fmtTemp(r.temperatureC, 0)
  const temp = tt === na() ? '' : `${tt}°C`
  return { health, temp }
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
    const badge = diskHealthBadge(letter)
    return `
      <button type="button" class="disk-tab ${active ? 'active' : ''}" data-drive="${letter}" title="${t('ssdExpandHint')}">
        <div class="disk-tab-top"><span class="disk-tab-letter">${letter}:</span><span class="disk-tab-pct metric">${fmtPct(d.percent, 0)}</span></div>
        <div class="fill"><i style="width:${Math.min(100, d.percent || 0)}%"></i></div>
        <div class="disk-tab-meta">
          <span class="disk-tab-health">${escapeHtml(badge.health)}</span>
          <span class="disk-tab-temp">${escapeHtml(badge.temp)}</span>
        </div>
      </button>
    `
  }).join('')
}

function renderOverviewDiskDetails() {
  const box = $('ov-disks')
  if (!box) return
  const disks = latestDisks || []
  if (!disks.length) {
    box.innerHTML = `<div class="empty">${dash()}</div>`
    return
  }
  if (!selectedDrive && disks[0]) selectedDrive = driveLetter(disks[0].mount)
  const activeDisk = disks.find((d) => driveLetter(d.mount) === selectedDrive.toUpperCase()) || disks[0]
  const letter = driveLetter(activeDisk.mount)
  const r = cachedDiskReport[letter]
  let smart = `<div class="empty">${t('ssdLoading')}</div>`
  if (getDevState().smartEnabled === false) {
    smart = `<div class="empty">${t('devSmartOff')}</div>`
  } else if (r) {
    if (r.ok === false) {
      smart = `<div class="empty">${escapeHtml(r.errorMessage || t('ssdUnavailable'))}</div>`
    } else {
      const wear = ssdWearLabel(r)
      const temp = (() => {
        const tt = fmtTemp(r.temperatureC, 0)
        return tt === na() ? t('noControllerTemp') : `${tt} °C`
      })()
      const hours = r.powerOnHours > 0 ? `${fmt(r.powerOnHours, 0)} h` : t('noData')
      smart = `
        <div class="row"><span>${t('wearLeft')}</span><strong>${wear}</strong></div>
        <div class="row"><span>${t('temp')}</span><strong>${temp}</strong></div>
        <div class="row"><span>${t('powerOn')}</span><strong>${hours}</strong></div>
        <div class="row"><span>${t('lblStatus')}</span><strong>${escapeHtml(r.verdictBadge || r.verdict || na())}</strong></div>
      `
    }
  }
  box.innerHTML = `
    <div class="row disk-row active" data-drive="${letter}">
      <div>
        <div class="name">${letter}:</div>
        <div class="meta">${fmt(activeDisk.usedGB)} / ${fmt(activeDisk.totalGB)} GB · ${t('free')} ${fmt(activeDisk.freeGB)} GB</div>
        <div class="meta">${t('read')} ${fmt(activeDisk.readMBps)} MB/s · ${t('write')} ${fmt(activeDisk.writeMBps)} MB/s</div>
      </div>
      <div class="name metric">${fmtPct(activeDisk.percent, 0)}</div>
    </div>
    <div class="disk-smart-expand">${smart}</div>
  `
}

async function prefetchDiskHealthBadges(disks) {
  const list = Array.isArray(disks) ? disks : []
  for (const d of list.slice(0, 6)) {
    const letter = driveLetter(d.mount)
    if (cachedDiskReport[letter] || getDevState().smartEnabled === false) continue
    try {
      const r = await CheckDiskHealth(letter)
      if (r) cachedDiskReport[letter] = r
    } catch (_) {
      cachedDiskReport[letter] = { ok: false }
    }
  }
  renderDiskTabs(latestDisks)
  renderOverviewDiskDetails()
}

function lagExplanation(e) {
  const toCoreLabel = (raw) => {
    const n = Number(raw)
    return Number.isFinite(n) ? n + 1 : raw
  }
  const raw = String(e.explanation || '')
  if (raw.startsWith('lag_core_spike|')) {
    const parts = raw.split('|')
    const core = toCoreLabel(parts[1] ?? e.coreID)
    const load = parts[2] ?? fmt(e.coreLoad, 0)
    return t('lagExplain').replace('{core}', core).replace('{load}', load)
  }
  return t('lagExplain').replace('{core}', toCoreLabel(e.coreID)).replace('{load}', fmt(e.coreLoad, 0))
}

function updateSensorTip(show) {
  const tip = $('hw-temp-tip')
  if (!tip) return
  tip.classList.toggle('hidden', !show)
  const adminBtn = $('btn-restart-admin')
  if (adminBtn && show) {
    IsElevated().then((elevated) => {
      adminBtn.classList.toggle('hidden', !!elevated)
    }).catch(() => {
      adminBtn.classList.remove('hidden')
    })
  }
}

async function restartAsAdmin() {
  try {
    await RestartAsAdministrator()
  } catch (_) {
    showToast(t('sensorUnavailableTip'), false)
  }
}

function renderLag(m) {
  const stutters = filterNoiseLags((m.stutterEvents || []).filter((e) => safeNum(e.jitterMs, 0) >= 300))
  const stutterBox = $('stutter-log')
  if (stutterBox) {
    if (!stutters.length) {
      stutterBox.innerHTML = `<div class="status-banner ok">${t('noStutters')}</div>`
    } else {
      stutterBox.innerHTML = stutters.map((e) => {
        const canEco = e.culprit && !isNoiseBackgroundProcess(e.culprit)
        return `
        <div class="row">
          <div>
            <div class="name metric">${e.timestamp || dash()} · ${fmt(e.jitterMs, 0)} ms</div>
            <div class="meta stutter-cause">${e.cause || dash()}</div>
          </div>
          <div class="row-actions">
            ${canEco ? `<button type="button" class="btn compact danger" data-eco-name="${e.culprit}">${t('ecoCulprit')}</button>` : ''}
          </div>
        </div>`
      }).join('')
    }
  }

  const lags = filterNoiseLags(m.lagEvents || [])
  const lagBox = $('lag-log')
  if (!lags.length) {
    lagBox.innerHTML = `<div class="empty">${t('noLags')}</div>`
  } else {
    lagBox.innerHTML = lags.map((e) => {
      const canEco = e.culpritPID && e.culpritProcess && !isNoiseBackgroundProcess(e.culpritProcess)
      return `
      <div class="incident-card">
        <div>
          <div class="incident-title">${e.culpritFriendly || e.culpritProcess || t('unknownProc')}</div>
          <div class="incident-explain">${lagExplanation(e)}</div>
          <div class="meta">${e.timestamp || dash()} · ${e.culpritProcess || dash()} · PID ${e.culpritPID || dash()}</div>
        </div>
        <div class="row-actions">
          ${canEco ? `<button type="button" class="btn compact danger" data-act="freeze" data-pid="${e.culpritPID}">${t('ecoLower')}</button>` : ''}
        </div>
      </div>`
    }).join('')
  }

  const vamps = filterNoiseVampires(m.vampires || [])
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

/** Group logical per-thread loads into physical cores (SMT = 2 threads). */
function groupPhysicalCores(perCore, physicalCores, logicalCores) {
  const threads = Array.isArray(perCore) ? perCore : []
  const logic = Number(logicalCores) || threads.length || 0
  const phys = Number(physicalCores) || 0
  if (!logic && !threads.length) return []
  const coreCount = phys > 0 ? phys : (threads.length || logic)
  const tpc = Math.max(1, Math.round((logic || threads.length) / Math.max(1, coreCount)))
  const groups = []
  for (let c = 0; c < coreCount; c++) {
    const kids = []
    for (let tIdx = 0; tIdx < tpc; tIdx++) {
      const idx = c * tpc + tIdx
      if (idx >= Math.max(logic, threads.length) && kids.length) break
      const raw = idx < threads.length ? threads[idx] : null
      const num = Number(raw)
      kids.push({
        id: tIdx,
        load: Number.isFinite(num) ? num : null,
        logicalIndex: idx,
      })
    }
    if (!kids.length) continue
    const nums = kids.map((k) => k.load).filter((v) => Number.isFinite(v))
    const avg = nums.length ? nums.reduce((a, b) => a + b, 0) / nums.length : null
    groups.push({ id: c, load: avg, threads: kids })
  }
  return groups
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
  const baseMHz = cpu.baseFrequencyMHz || cpu.frequencyMHz
  const curMHz = cpu.currentFrequencyMHz || cpu.frequencyMHz
  if ($('hw-cpu-base')) {
    $('hw-cpu-base').textContent = baseMHz ? `${fmt(baseMHz / 1000, 2)} GHz` : na()
  }
  $('hw-cpu-freq').textContent = curMHz ? `${fmt(curMHz / 1000, 2)} GHz` : na()
  $('hw-cpu-load').textContent = fmtPct(cpu.percent ?? m.cpuPercent)
  if ($('hw-cpu-temp')) {
    const temp = m.tempAvailable === false ? sensorUnavailableLabel() : fmtTemp(m.temperatureC, 0)
    const unavailable = (temp === na() || temp === sensorUnavailableLabel())
    $('hw-cpu-temp').textContent = unavailable ? sensorUnavailableLabel() : `${temp} °C`
    updateSensorTip(unavailable || m.tempNeedsAdmin === true)
  }
  if ($('hw-cpu-cache')) {
    const kb = Number(cpu.cacheKB) || 0
    $('hw-cpu-cache').textContent = kb > 0
      ? (kb >= 1024 ? `${fmt(kb / 1024, 1)} MB` : `${kb} KB`)
      : na()
  }
  if ($('hw-cpu-arch')) {
    $('hw-cpu-arch').textContent = cpu.architecture || 'x64'
  }

  const threads = cpu.perCore || m.cpuPerCore || []
  const grid = $('core-grid')
  const wrap = $('core-grid-wrap')
  if (wrap) wrap.classList.remove('hidden')
  if (grid) {
    const physN = Number(phys) || 0
    const logicN = Number(logic) || threads.length || 0
    const groups = groupPhysicalCores(threads, physN, logicN)
    const n = groups.length
    const cols = n >= 12 ? 4 : n >= 6 ? 4 : n >= 4 ? 4 : Math.max(1, n)
    grid.style.gridTemplateColumns = n ? `repeat(${cols}, minmax(0, 1fr))` : ''
    grid.dataset.cores = String(n || 0)
    grid.dataset.smt = groups.some((g) => g.threads.length > 1) ? '1' : '0'
    const coreWord = t('lblCore')
    grid.innerHTML = groups.length
      ? groups.map((g) => {
        const load = Number.isFinite(g.load) ? Math.min(100, Math.max(0, g.load)) : 0
        const val = Number.isFinite(g.load) ? fmtPct(g.load, 0) : na()
        const coreLabel = g.id + 1 // physical cores are 1..N in the UI
        const threadRows = g.threads.map((th) => {
          const tLoad = Number.isFinite(Number(th.load)) ? Math.min(100, Math.max(0, Number(th.load))) : 0
          const tVal = Number.isFinite(Number(th.load)) ? fmtPct(th.load, 0) : na()
          return `
          <div class="core-thread" title="T${th.id + 1}">
            <span class="tid">${t('lblThread')}${th.id + 1}</span>
            <div class="mini-bar"><i style="width:${tLoad}%"></i></div>
            <span class="tval">${tVal}</span>
          </div>`
        }).join('')
        return `
      <div class="core-phys" data-core="${coreLabel}">
        <div class="core-phys-head">
          <span class="id">${coreWord} ${coreLabel}</span>
          <span class="val">${val}</span>
        </div>
        <div class="core-phys-bar"><i style="width:${load}%"></i></div>
        <div class="core-threads">${threadRows}</div>
      </div>`
      }).join('')
      : `<div class="empty">${na()}</div>`
  }

  let gpuName = gpu.name || m.gpuName || na()
  const forceAdapter = getDevState().forceAdapter
  if (forceAdapter && typeof gpuName === 'string' && gpuName !== na()) {
    // Display-only preference hint when force string matches.
    if (!gpuName.toLowerCase().includes(String(forceAdapter).toLowerCase())) {
      gpuName = `${gpuName} · force:${forceAdapter}`
    }
  }
  $('hw-gpu-name').textContent = gpuName
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
    if (ram.isSingleChannel) channel.textContent = t('fpsSingleChannel')
    else if ((ram.slotsUsed || 0) >= 2 || /dual/i.test(String(ram.channelStatus || ''))) {
      channel.textContent = t('fpsDualChannel')
    } else {
      channel.textContent = ram.channelStatus || na()
    }
    channel.classList.remove('warn', 'ok')
    if (ram.isSingleChannel) channel.classList.add('warn')
    else if ((ram.slotsUsed || 0) >= 2) channel.classList.add('ok')
  }

  if (pwr.hasBattery) {
    $('hw-pwr-name').textContent = pwr.deviceName || t('batteryPack')
    $('hw-pwr-status').textContent = pwr.statusText || na()
    $('hw-pwr-pct').textContent = fmtPct(pwr.percent, 0)
    // Field label is already "Износ" / Wear - show percent only (never "Износ 8%").
    const wearPct = Number(pwr.wearPercent)
    $('hw-pwr-wear').textContent = Number.isFinite(wearPct)
      ? fmtPct(wearPct, 0)
      : String(pwr.wearLabel || '').replace(/^(износ|wear|тозу|тузу|verschlei[sß]|劣化|磨损)\s*:?\s*/i, '') || na()
    $('hw-pwr-cap').textContent = pwr.capacitymWh
      ? `${fmt(pwr.capacitymWh, 0)} / ${fmt(pwr.designCapacitymWh, 0)} mWh`
      : na()
  } else {
    $('hw-pwr-name').textContent = t('desktopPC')
    $('hw-pwr-status').textContent = 'AC'
    $('hw-pwr-pct').textContent = 'AC'
    $('hw-pwr-wear').textContent = na()
    $('hw-pwr-cap').textContent = na()
  }
}

function ssdWearLabel(r) {
  if (!r || r.ok === false) return t('ssdUnavailable')
  if (Number(r.wearRemaining) >= 0) return `${fmt(r.wearRemaining, 0)}%`
  return t('ssdUnavailable')
}

let hwSSDSignature = ''

function renderHwSSDShell(disks) {
  const host = $('hw-ssd-list')
  if (!host) return
  const list = Array.isArray(disks) ? disks : []
  const sig = list.map((d) => driveLetter(d.mount)).join('|')
  if (!list.length) {
    hwSSDSignature = ''
    host.innerHTML = `<div class="empty">${t('ssdUnavailable')}</div>`
    return
  }
  // Avoid wiping open accordions on every 1s metrics tick.
  if (sig === hwSSDSignature && host.querySelector('.hw-ssd-acc')) {
    host.querySelectorAll('.hw-ssd-acc').forEach((acc) => {
      const letter = acc.dataset.drive
      const cached = cachedDiskReport[letter]
      const compact = acc.querySelector('[data-ssd-compact]')
      if (compact && cached) {
        compact.textContent = `${cached.model || t('diskPrefix')} · ${ssdWearLabel(cached)}`
      }
    })
    return
  }
  hwSSDSignature = sig
  const openMap = {}
  host.querySelectorAll('.hw-ssd-acc').forEach((acc) => {
    openMap[acc.dataset.drive] = !!acc.open
  })
  host.innerHTML = list.map((d, idx) => {
    const letter = driveLetter(d.mount)
    const cached = cachedDiskReport[letter]
    const model = cached?.model || `${t('diskPrefix')} ${letter}:`
    const wear = cached ? ssdWearLabel(cached) : '…'
    const isOpen = openMap[letter] != null ? openMap[letter] : idx === 0
    return `
      <details class="hw-ssd-acc" data-drive="${letter}" ${isOpen ? 'open' : ''}>
        <summary>
          <span class="ssd-sum-left">
            <span class="ssd-drive">${t('diskPrefix')} ${letter}:</span>
            <span class="ssd-compact" data-ssd-compact>${model} · ${wear}</span>
          </span>
          <span class="ssd-chevron" aria-hidden="true" title="${t('ssdExpandHint')}"></span>
        </summary>
        <div class="hw-ssd-details" data-ssd-body>
          <div class="empty">${t('ssdLoading')}</div>
        </div>
      </details>`
  }).join('')
  // Prefill open ones from cache or fetch once.
  host.querySelectorAll('.hw-ssd-acc[open]').forEach((acc) => {
    fillHwSSDDetails(acc, false)
  })
}

async function fillHwSSDDetails(acc, force = false) {
  if (!acc) return
  const letter = String(acc.dataset.drive || 'C').toUpperCase()
  const body = acc.querySelector('[data-ssd-body]')
  const compact = acc.querySelector('[data-ssd-compact]')
  if (!body) return
  if (getDevState().smartEnabled === false) {
    body.innerHTML = `<div class="empty">${t('devSmartOff')}</div>`
    if (compact) compact.textContent = t('ssdUnavailable')
    return
  }
  let r = !force ? cachedDiskReport[letter] : null
  if (!r) {
    body.innerHTML = `<div class="empty">${t('ssdLoading')}</div>`
    try {
      r = await CheckDiskHealth(letter)
      if (r && r.ok !== false) cachedDiskReport[letter] = r
    } catch (err) {
      body.innerHTML = `<div class="empty">${t('ssdUnavailable')}</div>`
      if (compact) compact.textContent = t('ssdUnavailable')
      return
    }
  }
  if (!r || r.ok === false) {
    body.innerHTML = `<div class="empty">${r?.errorMessage || t('ssdUnavailable')}</div>`
    if (compact) compact.textContent = t('ssdUnavailable')
    return
  }
  const wear = ssdWearLabel(r)
  const temp = (() => {
    const tt = fmtTemp(r.temperatureC, 0)
    return tt === na() ? t('noControllerTemp') : `${tt} °C`
  })()
  const tbw = r.tbwGB > 1024 ? `${fmt(r.tbwGB / 1024, 2)} TB` : (r.tbwGB > 0 ? `${fmt(r.tbwGB, 1)} GB` : t('noData'))
  const hours = r.powerOnHours > 0 ? `${fmt(r.powerOnHours, 0)} h` : t('noData')
  if (compact) compact.textContent = `${r.model || t('diskPrefix')} · ${wear}`
  body.innerHTML = `
    <div class="row"><span>${t('wearLeft')}</span><strong>${wear}</strong></div>
    <div class="row"><span>${t('temp')}</span><strong>${temp}</strong></div>
    <div class="row"><span>${t('critErr')}</span><strong>${r.hasCriticalErrors ? t('yes') : t('no')}</strong></div>
    <div class="row"><span>${t('tbw')}</span><strong>${tbw}</strong></div>
    <div class="row"><span>${t('powerOn')}</span><strong>${hours}</strong></div>
    <div class="row"><span>${t('lblStatus')}</span><strong>${r.verdictBadge || r.verdict || na()}</strong></div>
  `
}

function isDefenderName(name) {
  const n = String(name || '').toLowerCase()
  return /msmpeng|defender|защитник|антивирус|antivirus/.test(n)
}

/** AV / critical OS - never show in Lags vampires or suggest Eco Mode. */
function isNoiseBackgroundProcess(name) {
  const n = String(name || '').toLowerCase().replace(/^.*[\\/]/, '')
  if (!n) return true
  return /^(system|registry|idle|system idle process|secure system|memory compression)$/.test(n) ||
    /^(msmpeng|nissrv|nisrv|securityhealthservice|securityhealthsystray|securityhealthhost|mssense|mpcmdrun|sgrmbroker|smartscreen|wdavscannerservice)\.exe$/.test(n) ||
    /^(searchindexer|searchprotocolhost|searchhost|searchapp|svchost|csrss|lsass|smss|wininit|winlogon|services|dwm|explorer|fontdrvhost|sihost|taskhostw|conhost|ctfmon|runtimebroker|taskmgr)\.exe$/.test(n) ||
    isDefenderName(n)
}

function filterNoiseVampires(list) {
  return (list || []).filter((v) => !isNoiseBackgroundProcess(v?.name) && !isNoiseBackgroundProcess(v?.friendlyName))
}

function filterNoiseLags(list) {
  return (list || []).filter((e) => {
    const name = e?.culpritProcess || e?.culprit || ''
    return !name || !isNoiseBackgroundProcess(name)
  })
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
      const canEco = !isNoiseBackgroundProcess(a.name) && !isDefenderName(a.name) && !isDefenderName(a.friendlyName)
      return `
      <div class="row">
        <div>
          <div class="name">#${i + 1} ${loc.title}</div>
          <div class="meta">${loc.detail}</div>
        </div>
        <div class="row-actions">
          ${canEco ? `<button type="button" class="btn compact danger" data-weekly-eco="${a.name}">${t('ecoAppetite')}</button>` : ''}
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
    default: return { title: group || '-', ico: '🔌' }
  }
}

function renderPeripherals(items) {
  const box = $('periph-list')
  if (!box) return
  if (!items || !items.length) {
    box.innerHTML = `<div class="empty">-</div>`
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
    const rows = groups[g].map((p) => {
      const unavailable = String(p.status || '').toLowerCase() === 'unavailable' ||
        String(p.name || '').toLowerCase() === 'unavailable'
      const statusLabel = unavailable ? t('periphUnavailable') : t('periphActive')
      const name = unavailable ? t('sensorUnavailable') : (p.name || dash())
      return `
      <div class="periph-row ${unavailable ? 'unavailable' : ''}">
        <div class="periph-ico">${p.icon || meta.ico}</div>
        <div>
          <div class="periph-name">${name}</div>
          <div class="periph-meta">${unavailable ? '' : (p.kind || p.class || '')}</div>
        </div>
        <div class="periph-status">${statusLabel}</div>
      </div>`
    }).join('')
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

let externalIPHidden = true
let lastDiagnoseLag = null

async function runNetDiag() {
  const box = $('net-diag')
  box.innerHTML = `
    <div class="net-progress">
      <div class="label">${t('netProgress')}</div>
      <div class="track"><div class="fill"></div></div>
    </div>`
  try {
    const d = await DiagnoseLagOrigin()
    lastDiagnoseLag = d
    renderNetDiagUI(d)
  } catch (err) {
    box.innerHTML = `<div class="empty">${escapeHtml(String(err))}</div>`
  }
}

function renderNetDiagUI(d) {
  const box = $('net-diag')
  if (!box || !d) return
  const verdict = t(d.verdictKey || d.verdict || 'netVerdictOK')
  const localIP = d.localIP || '127.0.0.1'
  const realExtIP = d.externalIP || '127.0.0.1'
  const displayExtIP = externalIPHidden ? '••••••••••••' : realExtIP
  const toggleText = externalIPHidden ? t('netShowIP') : t('netHideIP')

  const gwRating = d.gatewayRating || (d.gatewayMs <= 30 ? 'Отлично для онлайн-игр' : d.gatewayMs <= 80 ? 'Норма' : 'Высокая задержка')
  const pubRating = d.publicRating || (d.publicMs <= 30 ? 'Отлично для онлайн-игр' : d.publicMs <= 80 ? 'Норма' : 'Высокая задержка')
  const stability = d.stabilityStatus || (d.localJitterMs < 2 ? 'Идеальная (скачки < 2 мс)' : d.localJitterMs < 8 ? 'Хорошая' : 'Нестабильно (скачки пакетов)')

  box.innerHTML = `
    <div class="net-ip-block">
      <div class="net-ip-card">
        <div>
          <div class="net-ip-label">${t('netLocalIP')}</div>
          <div class="net-ip-val">${escapeHtml(localIP)}</div>
        </div>
        <div class="net-ip-actions">
          <button type="button" class="btn compact ghost" id="btn-copy-local-ip">${t('netCopyIP')}</button>
        </div>
      </div>
      <div class="net-ip-card">
        <div>
          <div class="net-ip-label">${t('netExternalIP')}</div>
          <div class="net-ip-val" id="net-ext-val">${escapeHtml(displayExtIP)}</div>
        </div>
        <div class="net-ip-actions">
          <button type="button" class="btn compact ghost" id="btn-toggle-ext-ip">${toggleText}</button>
          <button type="button" class="btn compact ghost" id="btn-copy-ext-ip">${t('netCopyIP')}</button>
        </div>
      </div>
    </div>
    <div class="verdict-card ${d.level === 'ok' ? 'ok' : 'warn'}"><div class="v-title">${verdict}</div></div>
    <div class="net-card">
      <div>
        <div class="title">${t('netRouter')}</div>
        <div class="hint">${escapeHtml(gwRating)}</div>
        ${netBadge(!!d.gatewayBadgeOK, d.gatewayBadge || 'netBadgeIdeal')}
      </div>
      <div class="net-ms">${d.gatewayMs < 0 ? '-' : fmt(d.gatewayMs, 0) + ' ms'}</div>
    </div>
    <div class="net-card">
      <div>
        <div class="title">${t('netInternet')}</div>
        <div class="hint">${escapeHtml(pubRating)}</div>
        ${netBadge(!!d.publicBadgeOK, d.publicBadge || 'netBadgeFast')}
      </div>
      <div class="net-ms">${d.publicMs < 0 ? '-' : fmt(d.publicMs, 0) + ' ms'}</div>
    </div>
    <div class="net-card">
      <div>
        <div class="title">${t('netStability')}</div>
        <div class="hint">${escapeHtml(stability)}</div>
        ${netBadge(!!d.jitterBadgeOK, d.jitterBadge || 'netBadgeStable')}
      </div>
      <div class="net-ms">${fmt(d.localJitterMs, 0)} ms</div>
    </div>
  `

  $('btn-copy-local-ip')?.addEventListener('click', () => {
    navigator.clipboard.writeText(localIP)
    showToast(`${t('netLocalIP')}: ${t('netCopied')}`, true)
  })
  $('btn-toggle-ext-ip')?.addEventListener('click', () => {
    externalIPHidden = !externalIPHidden
    renderNetDiagUI(d)
  })
  $('btn-copy-ext-ip')?.addEventListener('click', () => {
    navigator.clipboard.writeText(realExtIP)
    showToast(`${t('netExternalIP')}: ${t('netCopied')}`, true)
  })
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
    const name = blockers[0] || '-'
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

function adjustWindowForMiniHUD() {
  const el = document.querySelector('.minihud-capsule')
  if (el) {
    const rect = el.getBoundingClientRect()
    const targetW = Math.max(340, Math.ceil(rect.width) + 36)
    const targetH = Math.max(54, Math.ceil(rect.height) + 28)
    try { WindowSetSize(targetW, targetH) } catch (_) {}
  }
}

function adjustWindowForGameOverlay() {
  const el = document.querySelector('.gameoverlay-capsule')
  if (el) {
    const rect = el.getBoundingClientRect()
    const targetW = Math.max(340, Math.ceil(rect.width) + 36)
    const targetH = Math.max(54, Math.ceil(rect.height) + 28)
    try { WindowSetSize(targetW, targetH) } catch (_) {}
  }
}

function renderMiniHUD(m) {
  $('mh-cpu').textContent = fmtPct(m.cpuPercent, 0)
  $('mh-ram').textContent = fmtPct(m.ramPercent, 0)
  $('mh-gpu').textContent = fmtPct(m.gpuPercent, 0)
  const tempEl = $('mh-temp')
  if (tempEl) {
    if (m.tempAvailable === false) {
      tempEl.textContent = sensorUnavailableLabel()
    } else {
      const temp = fmtTemp(m.temperatureC, 0)
      tempEl.textContent = temp === na() ? sensorUnavailableLabel() : `${temp}°`
    }
  }
  applyMiniHudPrefs()
  if (hudEnabled) {
    requestAnimationFrame(adjustWindowForMiniHUD)
  }
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
    if (enabled) {
      requestAnimationFrame(adjustWindowForMiniHUD)
    }
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
    if (enabled) {
      requestAnimationFrame(adjustWindowForGameOverlay)
    }
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

function expandSearchAliasesLocal(q) {
  const table = {
    'как достать соседа': ['neighbours from hell', 'neighbors from hell'],
    'соседей': ['neighbours from hell'],
    'герои 3': ['heroes of might and magic iii', 'homm3'],
    'герои iii': ['heroes of might and magic iii'],
    'герои 5': ['heroes of might and magic v'],
    'ведьмак 3': ['witcher 3'],
    'ведьмак': ['witcher'],
    'кдло': ['counter-strike', 'cs2'],
    'кс': ['counter-strike'],
    'кс го': ['counter-strike'],
    'дота': ['dota 2'],
    'танки': ['world of tanks'],
    'танки блиц': ['world of tanks blitz'],
    'майкрафт': ['minecraft'],
    'гта': ['grand theft auto'],
    'гта 5': ['gta v', 'grand theft auto v'],
    'гта сан': ['san andreas'],
    'халф лайф': ['half-life'],
    'халф-лайф': ['half-life'],
    'флиаут': ['flatout'],
    'флэтаут': ['flatout'],
    'нфс': ['need for speed'],
    'nfs underground': ['need for speed underground'],
  }
  const raw = String(q || '').trim().toLowerCase()
  if (!raw) return []
  if (table[raw]) return table[raw]
  for (const [k, v] of Object.entries(table)) {
    if (raw.includes(k)) return v
  }
  return []
}

function filterGamesInstant(query, catalog) {
  const list = Array.isArray(catalog) ? catalog : []
  const q = String(query || '').trim().toLowerCase()
  if (!q) return list.slice()
  const aliases = expandSearchAliasesLocal(q)
  return list.filter((g) => {
    const name = String(g?.name || '').toLowerCase()
    const id = String(g?.id || '').toLowerCase()
    const genre = String(g?.genre || '').toLowerCase()
    if (name.includes(q) || id.includes(q) || genre.includes(q)) return true
    return aliases.some((al) => name.includes(al) || id.includes(al))
  })
}

async function loadGames() {
  try {
    gameCatalog = await ListGames() || []
    fullGameCatalog = gameCatalog.slice()
  } catch (_) {
    gameCatalog = []
    fullGameCatalog = []
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
  const query = String(q || '').trim()
  // Instant client-side filter over the full catalog (RU aliases + EN names).
  if (!query) {
    gameCatalog = fullGameCatalog.length ? fullGameCatalog.slice() : (await ListGames().catch(() => []) || [])
    if (!fullGameCatalog.length) fullGameCatalog = gameCatalog.slice()
    renderGameGrid()
    return
  }
  const base = fullGameCatalog.length ? fullGameCatalog : gameCatalog
  gameCatalog = filterGamesInstant(query, base)
  renderGameGrid()
  try {
    const remote = await SearchGames(query) || []
    if (remote.length) {
      gameCatalog = mergeGameLists(filterGamesInstant(query, base), remote)
      renderGameGrid()
    }
  } catch (_) {
    // keep instant results
  }
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
    gameCatalog = fullGameCatalog.length ? fullGameCatalog.slice() : (await ListGames().catch(() => []) || [])
    if (!fullGameCatalog.length) fullGameCatalog = gameCatalog.slice()
    renderGameGrid()
    return
  }
  await searchGamesLocal(q)
  if (online || gameCatalog.length === 0) {
    await searchGamesOnlineMerge(q)
  }
}

function hashHue(name) {
  let h = 0
  const s = String(name || 'Game')
  for (let i = 0; i < s.length; i++) h = (h * 31 + s.charCodeAt(i)) >>> 0
  return h % 360
}

function posterFallbackHTML(name, cls) {
  const safe = String(name || 'Game').replace(/</g, '&lt;').replace(/>/g, '&gt;')
  const initials = String(name || 'G').trim().split(/\s+/).slice(0, 2).map((w) => w[0] || '').join('').toUpperCase() || 'PC'
  const hue = hashHue(name)
  return `<div class="${cls} fallback logo-card" style="--logo-hue:${hue}"><span class="logo-initials">${initials}</span><span class="logo-title">${safe}</span></div>`
}

function steamHeaderFromPoster(url) {
  if (!url) return ''
  return String(url)
    .replace('/library_600x900.jpg', '/header.jpg')
    .replace('/library_600x900_2x.jpg', '/header.jpg')
}

function steamCdnUrls(steamAppId) {
  const id = Number(steamAppId)
  if (!Number.isFinite(id) || id <= 0) return { poster: '', posterAlt: '', header: '' }
  return {
    poster: `https://shared.cloudflare.steamstatic.com/store_item_assets/steam/apps/${id}/library_600x900.jpg`,
    posterAlt: `https://cdn.cloudflare.steamstatic.com/steam/apps/${id}/library_600x900.jpg`,
    header: `https://cdn.cloudflare.steamstatic.com/steam/apps/${id}/header.jpg`,
  }
}

function resolveGameArt(g) {
  const cdn = steamCdnUrls(g?.steamAppId)
  // Prefer catalog URLs; empty / broken ones fall through to CDN then styled logo cards.
  const poster = g?.posterUrl || cdn.poster || ''
  const header = g?.headerUrl || cdn.header || steamHeaderFromPoster(poster) || ''
  const posterAlt = cdn.posterAlt && cdn.posterAlt !== poster ? cdn.posterAlt : ''
  return { poster, header, posterAlt }
}

window.__pcPosterFallback = function(el, name, cls) {
  const q = (el.dataset.fb || '').split('|').filter(Boolean)
  if (q.length) {
    el.dataset.fb = q.slice(1).join('|')
    el.src = q[0]
    return
  }
  const initials = String(name || 'G').trim().split(/\s+/).slice(0, 2).map((w) => w[0] || '').join('').toUpperCase() || 'PC'
  const d = document.createElement('div')
  d.className = `${cls} fallback logo-card`
  d.innerHTML = `<span class="logo-initials">${escapeHtml(initials)}</span><span class="logo-title">${escapeHtml(name)}</span>`
  el.replaceWith(d)
}

function imgWithFallbacks(cls, name, urls) {
  const safeName = String(name || 'Game').replace(/\\/g, '\\\\').replace(/'/g, "\\'").replace(/"/g, '&quot;')
  const chain = (urls || []).filter(Boolean)
  if (!chain.length) return posterFallbackHTML(name, cls)
  const src = chain[0]
  const rest = chain.slice(1)
  const fbAttr = rest.length ? rest.join('|') : ''
  const alt = String(name || 'Game').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;')
  return `<img class="${cls}" src="${src}" alt="${alt}" loading="lazy" decoding="async" data-fb="${fbAttr}" onerror="window.__pcPosterFallback(this, '${safeName}', '${cls}')" />`
}

function gamePosterHTML(g) {
  const { poster, header, posterAlt } = resolveGameArt(g)
  return imgWithFallbacks('game-poster', g.name, [poster, posterAlt, header])
}

function compactCoverHTML(p) {
  const { poster, header, posterAlt } = resolveGameArt(p)
  return imgWithFallbacks('game-cover', p.gameName, [poster, posterAlt, header])
}

function renderGameGrid() {
  const host = $('game-grid')
  if (!host) return
  const list = gameCatalog || []
  const q = String($('game-search')?.value || '').trim()
  if (!list.length) {
    host.innerHTML = q
      ? `<div class="game-empty-card">${t('gameNotFound')}</div>`
      : `<div class="empty">-</div>`
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
    const displayWarn = warnKeys.find((k) => k !== 'fpsWarnApprox') || ''
    const bottleneck = displayWarn ? t(displayWarn) : ''
    const verdictKey = p.verdict || ''
    const verdictText = verdictKey ? t(verdictKey) : ''
    const verdictCls = p.verdictLevel === 'bad' ? 'bad' : (p.verdictLevel === 'ok' ? 'ok' : 'warn')
    const show = (range, avg) => {
      if (range === '<5') return '<5'
      return (range && String(range).includes('-')) ? range : avg
    }
    const vramLabel = p.isIntegrated ? t('fpsSharedVRAM') : `${fmt(p.vramGB, 1)} GB`
    const ramLabel = p.dualChannel ? t('fpsDualChannel') : t('fpsSingleChannel')
    const scores = `${t('fpsScoreGPU')} <b>${fmt(p.gpuScore, 0)}</b> · ${t('fpsScoreCPU')} <b>${fmt(p.cpuScore, 0)}</b> · VRAM <b>${vramLabel}</b> · RAM <b>${ramLabel}</b>`
    const badge = p.badge || p.genre || p.category || ''
    const approx = !!(p.approximate || p.note === 'approximate' || warnKeys.includes('fpsWarnApprox'))
    const approxBadge = approx
      ? `<span class="game-badge approx" title="${t('approxFromSpecs')}">${t('approxFromSpecs')}</span>`
      : ''
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
            ${approxBadge}
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

function fillAboutSpecs(m = lastMetrics) {
  const host = $('about-specs')
  if (!host) return
  const hw = m?.hardware || {}
  const cpu = hw.cpu?.name || m?.cpuName || na()
  const gpu = hw.gpu?.name || m?.gpuName || na()
  const ramMB = hw.ram?.totalMB || m?.ramTotalMB
  const ram = ramMB ? `${Math.round(Number(ramMB) / 1024)} GB` : na()
  const os = m?.osName || m?.os || 'Windows'
  host.innerHTML = `
    <div class="about-line"><b>CPU</b>: ${cpu}</div>
    <div class="about-line"><b>GPU</b>: ${gpu}</div>
    <div class="about-line"><b>RAM</b>: ${ram}</div>
    <div class="about-line"><b>OS</b>: ${os}</div>
    <div class="about-line"><b>PulseCore</b>: ${APP_VERSION}</div>
  `
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
  fillAboutSpecs()
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
  return `CPU: ${cpu} | GPU: ${gpu} | RAM: ${ram} | OS: ${os} | PulseCore ${APP_VERSION}`
}

function flashCopyPCButton() {
  const btn = $('btn-copy-pc')
  if (!btn) return
  const prev = btn.dataset.label || btn.textContent
  btn.dataset.label = prev
  btn.textContent = t('aboutCopied')
  btn.classList.add('copied')
  clearTimeout(flashCopyPCButton._t)
  flashCopyPCButton._t = setTimeout(() => {
    btn.textContent = btn.dataset.label || t('aboutCopyPC')
    btn.classList.remove('copied')
  }, 2000)
}

async function copyPCConfig() {
  const line = buildPCConfigLine()
  try {
    await navigator.clipboard.writeText(line)
    flashCopyPCButton()
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
      flashCopyPCButton()
      showToast(t('aboutCopied'), true)
    } catch (err) {
      showToast(String(err), false)
    }
  }
}

let lastUpdateResult = null
let updateBannerDismissed = false

function openDownloadURL(url) {
  const target = (url && String(url).trim()) || DEFAULT_EXE_URL
  try {
    BrowserOpenURL(target)
  } catch (_) {
    window.open(target, '_blank', 'noopener,noreferrer')
  }
}

function setUpdateResultHost(host, html) {
  if (!host) return
  host.innerHTML = html
}

function renderUpdateStatus(host, res) {
  if (!host) return
  if (!res || res.status === 'offline') {
    setUpdateResultHost(host, t('updateOffline'))
    return
  }
  if (res.status === 'up_to_date') {
    const latest = res.latestVersion ? ` v${String(res.latestVersion).replace(/^v/i, '')}` : ''
    setUpdateResultHost(host, `${t('updateUpToDate')}${latest}`)
    return
  }
  if (res.status === 'update_available') {
    const ver = `v${String(res.latestVersion || '').replace(/^v/i, '')}`
    const label = t('updateAvailable').replace('{version}', ver)
    const notes = res.notes ? `<div class="muted" style="margin-top:4px">${escapeHtml(res.notes)}</div>` : ''
    setUpdateResultHost(
      host,
      `${escapeHtml(label)} <button type="button" class="btn compact update-dl-btn" data-update-dl="1">${escapeHtml(t('updateDownload'))}</button>${notes}`,
    )
    host.querySelector('[data-update-dl]')?.addEventListener('click', () => openDownloadURL(res.downloadURL))
    return
  }
  setUpdateResultHost(host, t('updateOffline'))
}

function showUpdateBanner(res) {
  const banner = $('update-banner')
  const text = $('update-banner-text')
  if (!banner || !text || !res || res.status !== 'update_available') return
  if (updateBannerDismissed) return
  const ver = `v${String(res.latestVersion || '').replace(/^v/i, '')}`
  text.textContent = t('updateBannerText').replace('{version}', ver)
  banner.dataset.downloadUrl = res.downloadURL || DEFAULT_EXE_URL
  banner.classList.remove('hidden')
}

function hideUpdateBanner() {
  updateBannerDismissed = true
  $('update-banner')?.classList.add('hidden')
}

async function runUpdateCheck({ showBanner = false, hosts = [] } = {}) {
  const targets = (hosts || []).filter(Boolean)
  for (const host of targets) {
    setUpdateResultHost(host, `<span class="update-spin" aria-hidden="true"></span>${escapeHtml(t('updateChecking'))}`)
  }
  let res
  try {
    res = await CheckForUpdates()
  } catch (_) {
    res = { status: 'offline', currentVersion: APP_VERSION.replace(/^v/i, ''), latestVersion: '', notes: '', downloadURL: DEFAULT_EXE_URL }
  }
  lastUpdateResult = res
  for (const host of targets) renderUpdateStatus(host, res)
  if (showBanner && res?.status === 'update_available') showUpdateBanner(res)
  return res
}

function syncSettingsVersionLabel() {
  const el = $('settings-app-ver')
  if (el) el.textContent = APP_VERSION
}

function diagStatusLabel(status) {
  switch (String(status || '')) {
    case 'ok': return t('diagStatusOk')
    case 'fallback': return t('diagStatusFallback')
    case 'admin': return t('diagStatusAdmin')
    default: return t('diagStatusUnavailable')
  }
}

function diagItemTitle(id, fallback) {
  const map = {
    cpu: 'diagItemCPU',
    gpu: 'diagItemGPU',
    ram: 'diagItemRAM',
    drives: 'diagItemDrives',
    registry: 'diagItemRegistry',
    runmode: 'diagItemRunMode',
  }
  const key = map[id]
  return key ? t(key) : (fallback || id)
}

function formatDiagTime(iso) {
  const raw = String(iso || '').trim()
  if (!raw) return ''
  const d = new Date(raw)
  if (Number.isNaN(d.getTime())) return raw
  const locMap = { ru: 'ru-RU', en: 'en-GB', kk: 'kk-KZ', tt: 'ru-RU', de: 'de-DE', ja: 'ja-JP', zh: 'zh-CN' }
  const loc = locMap[lang] || 'ru-RU'
  try {
    return d.toLocaleString(loc, {
      day: '2-digit',
      month: '2-digit',
      year: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
      hour12: false,
    })
  } catch (_) {
    const dd = String(d.getDate()).padStart(2, '0')
    const mm = String(d.getMonth() + 1).padStart(2, '0')
    const yyyy = d.getFullYear()
    const hh = String(d.getHours()).padStart(2, '0')
    const mi = String(d.getMinutes()).padStart(2, '0')
    return `${dd}.${mm}.${yyyy}, ${hh}:${mi}`
  }
}

function formatMemAmount(raw) {
  const s = String(raw || '').trim()
  const m = s.match(/^([\d.]+)\s*(MB|GB|mb|gb)?$/i)
  if (!m) return s
  let n = Number(m[1])
  if (!Number.isFinite(n)) return s
  const unit = (m[2] || 'MB').toUpperCase()
  if (unit === 'MB') {
    if (n >= 1024) {
      const gb = n / 1024
      return `${gb % 1 === 0 ? gb.toFixed(0) : gb.toFixed(1)} ${lang === 'ru' || lang === 'kk' || lang === 'tt' ? 'ГБ' : 'GB'}`
    }
    return `${Math.round(n)} ${lang === 'ru' || lang === 'kk' || lang === 'tt' ? 'МБ' : 'MB'}`
  }
  return `${n % 1 === 0 ? n.toFixed(0) : n.toFixed(1)} ${lang === 'ru' || lang === 'kk' || lang === 'tt' ? 'ГБ' : 'GB'}`
}

function translateDiagDetail(code) {
  const c = String(code || '').trim()
  const map = {
    ok: '',
    working: '',
    user: '',
    admin: '',
    needs_windows: 'diagDetailNeedsWindows',
    cpu_missing: 'diagDetailCpuMissing',
    fallback_acpi: 'diagDetailFallbackAcpi',
    admin_temp: 'diagDetailAdminTemp',
    gpu_missing: 'diagDetailGpuMissing',
    fallback_gpu: 'diagDetailFallbackGpu',
    ram_missing: 'diagDetailRamMissing',
    fallback_ram_slots: 'diagDetailRamSlots',
    drives_missing: 'diagDetailDrivesMissing',
    admin_smart: 'diagDetailAdminSmart',
    fallback_smart: 'diagDetailFallbackSmart',
    registry_missing: 'diagDetailRegistryMissing',
    fallback_paths: 'diagDetailFallbackPaths',
    // Legacy English strings from older backends / mock
    'Working': '',
    'Normal user': '',
    'Administrator': '',
    'Sensors require Windows': 'diagDetailNeedsWindows',
    'CPU sensors unavailable': 'diagDetailCpuMissing',
    'Fallback calculation (ACPI temp unavailable)': 'diagDetailFallbackAcpi',
    'Needs administrator rights for ACPI temperature': 'diagDetailAdminTemp',
    'GPU not detected': 'diagDetailGpuMissing',
    'Fallback calculation (vendor load counter missing)': 'diagDetailFallbackGpu',
    'RAM capacity unavailable': 'diagDetailRamMissing',
    'Fallback calculation (SMBIOS slots unavailable)': 'diagDetailRamSlots',
    'No fixed drives listed': 'diagDetailDrivesMissing',
    'Needs administrator rights for SMART': 'diagDetailAdminSmart',
    'Fallback calculation (SMART counters unavailable)': 'diagDetailFallbackSmart',
    'Registry and cleaner paths unreachable': 'diagDetailRegistryMissing',
    'Fallback calculation (some paths unreachable)': 'diagDetailFallbackPaths',
    'Windows paths only': 'diagDetailNeedsWindows',
    'Normal user (non-Windows)': '',
  }
  if (Object.prototype.hasOwnProperty.call(map, c)) {
    const key = map[c]
    return key ? t(key) : ''
  }
  if (/^Working$/i.test(c)) return ''
  return c
}

function formatDiagFact(raw) {
  const s = String(raw || '').trim()
  if (!s) return ''
  const eq = s.indexOf('=')
  if (eq > 0) {
    const key = s.slice(0, eq).trim()
    const val = s.slice(eq + 1).trim()
    switch (key) {
      case 'model':
        return `${t('diagFactModel')}: ${val}`
      case 'cores':
        return `${t('diagFactCores')}: ${val.replace('/', ' / ')}`
      case 'temp':
        if (val === 'unavailable' || val === 'null') return `${t('diagFactTemp')}: ${t('diagTempUnavailable')}`
        return `${t('diagFactTemp')}: ${String(val).replace(/C$/i, '')} °C`
      case 'load':
        if (val === 'fallback') return `${t('diagFactLoad')}: ${t('diagLoadFallback')}`
        return `${t('diagFactLoad')}: ${val.includes('%') ? val : `${val}%`}`
      case 'primary':
        return `${t('diagFactPrimary')}: ${val}`
      case 'integrated':
        return `${t('diagFactIntegrated')}: ${val}`
      case 'vram':
        return `${t('diagFactVram')}: ${formatMemAmount(val)}`
      case 'capacity':
        return `${t('diagFactCapacity')}: ${formatMemAmount(val)}`
      case 'slots':
        return `${t('diagFactSlots')}: ${val.replace('/', ' / ')}`
      case 'freq':
        return `${t('diagFactFreq')}: ${String(val).replace(/MHz$/i, '')} ${lang === 'ru' || lang === 'kk' || lang === 'tt' ? 'МГц' : 'MHz'}`
      case 'cleaner_roots':
        return `${t('diagFactCleanerRoots')}: ${val.replace('/', ' / ')}`
      case 'backups':
        if (val === 'missing') return `${t('diagFactBackups')}: ${t('diagBackupsMissing')}`
        return `${t('diagFactBackups')}: ${val}`
      case 'registry':
        if (val === 'ok') return `${t('diagFactRegistry')}: ${t('diagRegistryOk')}`
        if (val === 'limited') return `${t('diagFactRegistry')}: ${t('diagRegistryLimited')}`
        return `${t('diagFactRegistry')}: ${val}`
      case 'mode':
        return `${t('diagFactMode')}: ${val === 'admin' ? t('diagRunModeAdmin') : t('diagRunModeUser')}`
      case 'platform':
        return `${t('diagFactPlatform')}: ${val}`
      case 'smart':
        return `${t('diagFactSmart')}: ${val}`
      default:
        break
    }
  }
  // Drive line: "C:\ 200/512GB temp=35C smart=OK"
  const drive = s.match(/^(\S+)\s+([\d.]+)\/([\d.]+)GB(?:\s+temp=([\d.]+)C)?(?:\s+smart=(.+))?$/i)
  if (drive) {
    const parts = [`${t('diagFactDrive')}: ${drive[1]} ${drive[2]} / ${drive[3]} GB`]
    if (drive[4]) parts.push(`${t('diagFactTemp')}: ${drive[4]} °C`)
    if (drive[5]) parts.push(`${t('diagFactSmart')}: ${drive[5]}`)
    return parts.join(' · ')
  }
  return s
}

function buildPolishedDiagText(rep) {
  const lines = [
    `PulseCore ${t('diagTitle')}`,
    `${t('diagCheckedAt')}: ${formatDiagTime(rep?.generatedAt)}`,
    `${t('diagItemRunMode')}: ${rep?.runMode === 'admin' ? t('diagRunModeAdmin') : t('diagRunModeUser')}`,
  ]
  for (const it of (Array.isArray(rep?.items) ? rep.items : [])) {
    const title = diagItemTitle(it.id, it.title)
    const badge = diagStatusLabel(it.status)
    const detail = translateDiagDetail(it.detail)
    let head = `- ${title}: ${badge}`
    if (detail) head += ` (${detail})`
    lines.push(head)
    for (const f of (Array.isArray(it.facts) ? it.facts : [])) {
      const pretty = formatDiagFact(f)
      if (pretty) lines.push(`  ${pretty}`)
    }
  }
  return lines.join('\n')
}

function closeDiagModal() {
  $('diag-modal')?.classList.add('hidden')
}

function renderDiagReport(rep) {
  lastDiagReport = rep
  const list = $('diag-list')
  const line = $('diag-status-line')
  if (!list) return
  const modeKey = rep?.runMode === 'admin' ? 'diagRunModeAdmin' : 'diagRunModeUser'
  if (line) {
    const when = formatDiagTime(rep?.generatedAt)
    line.textContent = when
      ? `${t('diagItemRunMode')}: ${t(modeKey)} · ${t('diagCheckedAt')}: ${when}`
      : `${t('diagItemRunMode')}: ${t(modeKey)}`
  }
  const items = Array.isArray(rep?.items) ? rep.items : []
  list.innerHTML = items.map((it) => {
    const st = String(it.status || 'unavailable')
    const detail = translateDiagDetail(it.detail)
    const facts = (Array.isArray(it.facts) ? it.facts : [])
      .map(formatDiagFact)
      .filter(Boolean)
    const factsHtml = facts.length
      ? `<ul class="diag-item-facts">${facts.map((f) => `<li>${escapeHtml(f)}</li>`).join('')}</ul>`
      : ''
    const detailHtml = detail
      ? `<div class="diag-item-detail">${escapeHtml(detail)}</div>`
      : ''
    return `
      <div class="diag-item" data-diag-id="${escapeHtml(it.id || '')}">
        <div class="diag-item-head">
          <span class="diag-item-title">${escapeHtml(diagItemTitle(it.id, it.title))}</span>
          <span class="diag-badge ${escapeHtml(st)}">${escapeHtml(diagStatusLabel(st))}</span>
        </div>
        ${detailHtml}
        ${factsHtml}
      </div>`
  }).join('')
}

async function openSelfDiagnostics() {
  const modal = $('diag-modal')
  const list = $('diag-list')
  const line = $('diag-status-line')
  if (line) line.textContent = t('diagRunning')
  if (list) list.innerHTML = ''
  modal?.classList.remove('hidden')
  try {
    const rep = await RunSelfDiagnostics()
    renderDiagReport(rep || {})
  } catch (err) {
    if (line) line.textContent = String(err)
    showToast(String(err), false)
  }
}

async function copyDiagReport() {
  const text = lastDiagReport ? buildPolishedDiagText(lastDiagReport) : ''
  if (!text) {
    showToast(t('diagStatusUnavailable'), false)
    return
  }
  try {
    await navigator.clipboard.writeText(text)
    showToast(t('diagCopied'), true)
  } catch (_) {
    try {
      const ta = document.createElement('textarea')
      ta.value = text
      ta.style.position = 'fixed'
      ta.style.opacity = '0'
      document.body.appendChild(ta)
      ta.select()
      document.execCommand('copy')
      ta.remove()
      showToast(t('diagCopied'), true)
    } catch (err) {
      showToast(String(err), false)
    }
  }
}

async function applyMiniHUDHotkeySetting(chord, { toast = true } = {}) {
  const value = (chord || hudPrefs.hotkey || 'Ctrl+Shift+M').trim() || 'Ctrl+Shift+M'
  hudPrefs.hotkey = value
  saveHudPrefs()
  const input = $('mh-hotkey')
  if (input && input.value !== value) input.value = value
  try {
    const res = await SetMiniHUDHotkey(value)
    const saved = res?.chord || value
    hudPrefs.hotkey = saved
    saveHudPrefs()
    if (input) input.value = saved
    if (toast) {
      if (res?.ok || res?.active) showToast(t('mhHotkeySaved'), true)
      else showToast(t('mhHotkeyFailed'), false)
    }
  } catch (_) {
    if (toast) showToast(t('mhHotkeyFailed'), false)
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
  if (getDevState().smartEnabled === false) {
    showToast(t('devSmartOff'), false)
    return
  }
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

let upgradeGoal = localStorage.getItem('zp_upgrade_goal') || 'max_fps'
let installedAppsCache = []
let selectedUninstallApp = null
let residualItemsCache = []

function chassisLabel(pass) {
  if (pass?.isLaptop || pass?.chassis === 'laptop') return t('chassisLaptop')
  if (pass?.chassis === 'desktop') return t('chassisDesktop')
  return t('chassisUnknown')
}

function renderPassport(pass) {
  const host = $('upgrade-passport')
  if (!host || !pass) return
  const storage = (pass.storage || []).map((d) => {
    const size = d.sizeGB ? `${Math.round(d.sizeGB)} GB` : ''
    return `${d.model || '-'} (${d.interfaceType || '?'} ${d.mediaType || ''} ${size})`.trim()
  }).join('<br>') || '-'
  const notes = (pass.notes || []).map((n) => `<li>${escapeHtml(n)}</li>`).join('')
  const approxChip = pass.approximate
    ? `<div class="upgrade-chip approx">${escapeHtml(t('upgradeApproxBadge'))}</div>`
    : ''
  host.innerHTML = `
    <div class="upgrade-chip">${escapeHtml(chassisLabel(pass))}</div>
    ${approxChip}
    <div class="hw-grid upgrade-grid">
      <div><span class="muted" data-i18n="lblModel">CPU</span><div class="hw-val">${escapeHtml(pass.cpuName || '-')}</div></div>
      <div><span class="muted" data-i18n="lblCores">Cores</span><div class="hw-val metric">${pass.physicalCores || '-'} / ${pass.logicalCores || '-'}</div></div>
      <div><span class="muted" data-i18n="lblSocket">Socket</span><div class="hw-val">${escapeHtml(pass.socket || '-')}</div></div>
      <div><span class="muted" data-i18n="lblChipset">Chipset</span><div class="hw-val">${escapeHtml(pass.chipset || '-')}</div></div>
      <div><span class="muted" data-i18n="lblMobo">Board</span><div class="hw-val">${escapeHtml([pass.manufacturer, pass.motherboard].filter(Boolean).join(' ') || '-')}</div></div>
      <div><span class="muted">GPU</span><div class="hw-val">${escapeHtml(pass.gpuName || '-')} · ${pass.vramGB ? pass.vramGB.toFixed(1) + ' GB' : '-'}</div></div>
      <div><span class="muted">RAM</span><div class="hw-val metric">${pass.ramTotalGB ? pass.ramTotalGB.toFixed(0) + ' GB' : '-'} ${escapeHtml(pass.ramType || '')} · ${pass.ramSlotsUsed || 0}/${pass.ramSlotsTotal || 0}</div></div>
      <div><span class="muted" data-i18n="lblSlotsFree">Free slots</span><div class="hw-val metric">${pass.ramSlotsFree ?? '-'}</div></div>
      <div class="wide"><span class="muted" data-i18n="lblStorage">Storage</span><div class="hw-val">${storage}</div></div>
    </div>
    ${notes ? `<ul class="upgrade-notes">${notes}</ul>` : ''}
  `
  applyI18n()
}

function renderUpgradeRecs(advice) {
  const host = $('upgrade-recs')
  const summary = $('upgrade-summary')
  if (summary) summary.textContent = advice?.summary || ''
  if (!host) return
  const recs = advice?.recommendations || []
  if (!recs.length) {
    host.innerHTML = `<div class="empty">${t('upgradeEmpty')}</div>`
    return
  }
  host.innerHTML = recs.map((r) => {
    const pitfalls = (r.pitfalls || []).map((p) => `<li>${escapeHtml(p)}</li>`).join('')
    const model = r.model ? `<div class="upgrade-model">${escapeHtml(r.model)}</div>` : ''
    const gain = r.estimatedGain
      ? `<span class="upgrade-gain">${escapeHtml(r.estimatedGain)}</span>`
      : ''
    return `
      <article class="upgrade-rec">
        <div class="upgrade-rec-head">
          <h3>${escapeHtml(r.title || '')}</h3>
          <div class="upgrade-rec-meta">
            ${gain}
            <span class="upgrade-part">${escapeHtml(r.part || '')}</span>
          </div>
        </div>
        ${model}
        <p>${escapeHtml(r.body || '')}</p>
        <div class="pitfalls">
          <div class="pitfalls-title">${t('pitfallsTitle')}</div>
          <ul>${pitfalls}</ul>
        </div>
      </article>`
  }).join('')
}

async function refreshUpgradeAdvisor() {
  document.querySelectorAll('.btn.goal').forEach((b) => {
    b.classList.toggle('active', b.dataset.goal === upgradeGoal)
  })
  try {
    const advice = await GetUpgradeAdvice(upgradeGoal)
    renderPassport(advice?.passport)
    renderUpgradeRecs(advice)
  } catch (err) {
    const host = $('upgrade-passport')
    if (host) host.innerHTML = `<div class="empty">${escapeHtml(String(err))}</div>`
  }
}

function escapeHtml(s) {
  return String(s ?? '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
}

function renderInstalledApps(filter = '') {
  const host = $('uninstall-list')
  if (!host) return
  const q = String(filter || '').toLowerCase().trim()
  const list = installedAppsCache.filter((a) => {
    if (!q) return true
    return `${a.displayName || ''} ${a.publisher || ''}`.toLowerCase().includes(q)
  }).slice(0, 200)
  if (!list.length) {
    host.innerHTML = `<div class="empty">${t('weeklyEmpty')}</div>`
    return
  }
  host.innerHTML = list.map((a) => `
    <button type="button" class="row uninstall-row ${selectedUninstallApp?.id === a.id ? 'active' : ''}" data-app-id="${escapeHtml(a.id)}">
      <div class="row-main">
        <b>${escapeHtml(a.displayName || '-')}</b>
        <span class="muted">${escapeHtml(a.publisher || '')} · ${escapeHtml(a.displayVersion || '')}</span>
      </div>
      <span class="metric">${a.estimatedMB ? a.estimatedMB + ' MB' : ''}</span>
    </button>`).join('')
  host.querySelectorAll('[data-app-id]').forEach((btn) => {
    btn.addEventListener('click', () => selectUninstallApp(btn.dataset.appId))
  })
}

function formatCleanBytes(n) {
  const b = Number(n) || 0
  if (b >= 1024 * 1024 * 1024) return `${(b / (1024 * 1024 * 1024)).toFixed(1)} GB`
  if (b >= 1024 * 1024) return `${(b / (1024 * 1024)).toFixed(1)} MB`
  if (b >= 1024) return `${(b / 1024).toFixed(0)} KB`
  return `${b} B`
}

function renderCleanPreview(r) {
  const box = $('clean-preview')
  const runBtn = $('btn-clean-run')
  if (!box) return
  if (!r) {
    box.textContent = t('cleanPreviewEmpty')
    if (runBtn) runBtn.disabled = true
    return
  }
  const msg = t('cleanPreviewResult')
    .replace('{files}', String(r.deletedFiles ?? 0))
    .replace('{size}', formatCleanBytes(r.deletedBytes))
    .replace('{skipped}', String(r.skipped ?? 0))
  const targets = Array.isArray(r.targets) && r.targets.length ? ` · ${r.targets.join(', ')}` : ''
  box.textContent = `${msg}${targets}`
  if (runBtn) runBtn.disabled = !(Number(r.deletedFiles) > 0)
}

async function refreshCleanPreview() {
  const box = $('clean-preview')
  if (box) box.textContent = t('cleanChecking')
  try {
    const r = await PreviewSafeDiskCaches()
    renderCleanPreview(r)
  } catch (err) {
    if (box) box.textContent = String(err)
    const runBtn = $('btn-clean-run')
    if (runBtn) runBtn.disabled = true
  }
}

async function openSelectedAppFolder() {
  if (!selectedUninstallApp) return
  try {
    const r = await OpenAppInstallFolder(selectedUninstallApp.id)
    showToast(r?.message || t('uninstallOpenFolder'), !!r?.ok)
  } catch (err) {
    showToast(String(err), false)
  }
}

async function runSafeClean() {
  const runBtn = $('btn-clean-run')
  if (runBtn) runBtn.disabled = true
  try {
    const r = await CleanSafeDiskCaches()
    showToast(r?.message || t('cleanDone'), !!r?.ok)
    renderCleanPreview({ ...r, deletedFiles: 0, deletedBytes: 0 })
    if (runBtn) runBtn.disabled = true
  } catch (err) {
    showToast(String(err), false)
    if (runBtn) runBtn.disabled = false
  }
}

let systemCleanScanResult = null

async function scanSystemClean() {
  const scanBtn = $('btn-sysclean-scan')
  const runBtn = $('btn-sysclean-run')
  if (scanBtn) scanBtn.disabled = true
  if (runBtn) runBtn.disabled = true

  const tempSizeEl = $('clean-size-temp')
  const browserSizeEl = $('clean-size-browser')
  const dxSizeEl = $('clean-size-directx')
  if (tempSizeEl) tempSizeEl.textContent = '...'
  if (browserSizeEl) browserSizeEl.textContent = '...'
  if (dxSizeEl) dxSizeEl.textContent = '...'

  try {
    const res = await ScanSystemClean()
    systemCleanScanResult = res
    if (tempSizeEl) tempSizeEl.textContent = formatCleanBytes(res?.temp?.bytes || 0)
    if (browserSizeEl) browserSizeEl.textContent = formatCleanBytes(res?.browser?.bytes || 0)
    if (dxSizeEl) dxSizeEl.textContent = formatCleanBytes(res?.directx?.bytes || 0)

    updateSystemCleanRunButton()
  } catch (err) {
    showToast(String(err), false)
  } finally {
    if (scanBtn) scanBtn.disabled = false
  }
}

function updateSystemCleanRunButton() {
  const runBtn = $('btn-sysclean-run')
  if (!runBtn) return
  const tempChecked = $('clean-chk-temp')?.checked
  const browserChecked = $('clean-chk-browser')?.checked
  const dxChecked = $('clean-chk-directx')?.checked

  const hasAnyChecked = tempChecked || browserChecked || dxChecked
  runBtn.disabled = !hasAnyChecked || !systemCleanScanResult
}

async function runSystemCleanAction() {
  const runBtn = $('btn-sysclean-run')
  const cats = []
  if ($('clean-chk-temp')?.checked) cats.push('temp')
  if ($('clean-chk-browser')?.checked) cats.push('browser')
  if ($('clean-chk-directx')?.checked) cats.push('directx')

  if (!cats.length) {
    showToast(t('cleanNoSelection'), false)
    return
  }

  if (runBtn) runBtn.disabled = true
  try {
    const r = await CleanSystemCategories(cats)
    showToast(r?.message || t('cleanDone'), !!r?.ok)
    if (cats.includes('temp') && $('clean-size-temp')) $('clean-size-temp').textContent = '0 B'
    if (cats.includes('browser') && $('clean-size-browser')) $('clean-size-browser').textContent = '0 B'
    if (cats.includes('directx') && $('clean-size-directx')) $('clean-size-directx').textContent = '0 B'
  } catch (err) {
    showToast(String(err), false)
  } finally {
    updateSystemCleanRunButton()
  }
}

async function refreshDevTelemetry() {
  const logBox = $('dev-sensor-log')
  const tableBox = $('dev-telemetry-table')
  if (!logBox && !tableBox) return

  try {
    if (logBox) {
      const logs = await GetSensorLogs()
      if (Array.isArray(logs) && logs.length) {
        logBox.textContent = logs.join('\n')
        logBox.scrollTop = logBox.scrollHeight
      } else {
        logBox.textContent = t('noData') || 'Лог датчиков пуст'
      }
    }
  } catch (err) {
    if (logBox) logBox.textContent = String(err)
  }

  try {
    if (tableBox) {
      const data = await GetDevTelemetry()
      if (data && typeof data === 'object') {
        const rows = Object.entries(data).map(([k, v]) => {
          const valStr = typeof v === 'object' ? JSON.stringify(v) : String(v)
          return `<tr><td class="dev-telemetry-key">${escapeHtml(k)}</td><td class="dev-telemetry-val">${escapeHtml(valStr)}</td></tr>`
        }).join('')
        tableBox.innerHTML = `<table class="dev-telemetry-table"><tbody>${rows}</tbody></table>`
      }
    }
  } catch (err) {
    if (tableBox) tableBox.innerHTML = `<div class="empty">${escapeHtml(String(err))}</div>`
  }
}

async function refreshInstalledApps() {
  try {
    installedAppsCache = await ListInstalledApps() || []
    installedAppsCache.sort((a, b) => String(a.displayName || '').localeCompare(String(b.displayName || ''), undefined, { sensitivity: 'base' }))
    renderInstalledApps($('uninstall-search')?.value || '')
  } catch (err) {
    const host = $('uninstall-list')
    if (host) host.innerHTML = `<div class="empty">${escapeHtml(String(err))}</div>`
  }
}

function selectUninstallApp(id) {
  selectedUninstallApp = installedAppsCache.find((a) => a.id === id) || null
  const detail = $('uninstall-detail')
  if (!detail) return
  if (!selectedUninstallApp) {
    detail.hidden = true
    return
  }
  detail.hidden = false
  const title = $('uninstall-detail-title')
  if (title) title.textContent = selectedUninstallApp.displayName || '-'
  renderInstalledApps($('uninstall-search')?.value || '')
  const run = $('uninstall-running')
  if (run) run.innerHTML = `<div class="empty">${t('uninstallNoRunning')}</div>`
}

async function runUninstallPrecheck() {
  if (!selectedUninstallApp) return
  try {
    const r = await PrecheckUninstall(selectedUninstallApp.id)
    const host = $('uninstall-running')
    if (!host) return
    const running = r?.running || []
    if (!running.length) {
      host.innerHTML = `<div class="empty">${t('uninstallNoRunning')}</div>`
      if (r?.warning) showToast(r.warning, true)
      return
    }
    host.innerHTML = `<div class="panel-title">${t('uninstallRunning')}</div>` + running.map((p) => `
      <div class="row"><div class="row-main"><b>${escapeHtml(p.name)}</b><span class="muted">PID ${p.pid}</span></div></div>`).join('')
    if (r?.warning) showToast(r.warning, false)
  } catch (err) {
    showToast(String(err), false)
  }
}

async function runUninstall(killFirst) {
  if (!selectedUninstallApp) return
  try {
    const r = await UninstallApp(selectedUninstallApp.id, !!killFirst)
    showToast(r?.message || t('uninstallOnly'), !!r?.ok)
  } catch (err) {
    showToast(String(err), false)
  }
}

async function runResidualScan() {
  if (!selectedUninstallApp) return
  try {
    const r = await ScanResiduals(selectedUninstallApp.displayName || '', selectedUninstallApp.publisher || '')
    residualItemsCache = r?.items || []
    renderResiduals()
    showToast(r?.message || '', !!r?.ok)
  } catch (err) {
    showToast(String(err), false)
  }
}

function renderResiduals() {
  const host = $('residual-list')
  if (!host) return
  if (!residualItemsCache.length) {
    host.innerHTML = `<div class="empty">${t('residualEmpty')}</div>`
    return
  }
  host.innerHTML = residualItemsCache.map((it, idx) => `
    <label class="row residual-row ${it.safe ? '' : 'blocked'}">
      <input type="checkbox" data-res-idx="${idx}" ${it.selected && it.safe ? 'checked' : ''} ${it.safe ? '' : 'disabled'} />
      <div class="row-main">
        <b>${escapeHtml(it.label || it.path)}</b>
        <span class="muted">${escapeHtml(it.kind)} ${it.safe ? '' : '· ' + t('residualBlocked')}</span>
      </div>
    </label>`).join('')
  host.querySelectorAll('[data-res-idx]').forEach((cb) => {
    cb.addEventListener('change', () => {
      const i = Number(cb.dataset.resIdx)
      if (residualItemsCache[i]) residualItemsCache[i].selected = cb.checked
    })
  })
}

async function runResidualDelete() {
  if (!residualItemsCache.length) return
  try {
    const r = await DeleteResiduals(residualItemsCache)
    showToast(r?.message || '', !!r?.ok)
    await runResidualScan()
  } catch (err) {
    showToast(String(err), false)
  }
}

async function refreshTraceList() {
  const host = $('trace-list')
  if (!host) return
  try {
    const list = await ListInstallTraces() || []
    if (!list.length) {
      host.innerHTML = `<div class="empty">${t('traceEmpty')}</div>`
      return
    }
    host.innerHTML = list.map((tr) => `
      <div class="row">
        <div class="row-main">
          <b>${escapeHtml(tr.appName || tr.appId)}</b>
          <span class="muted">${escapeHtml(formatDiagTime(tr.finishedAt || tr.createdAt || ''))} · keys ${tr.newKeys || 0} · dirs ${tr.newDirs || 0}</span>
        </div>
        <button type="button" class="btn compact danger" data-trace-id="${escapeHtml(tr.appId)}">${t('traceCleanup')}</button>
      </div>`).join('')
    host.querySelectorAll('[data-trace-id]').forEach((btn) => {
      btn.addEventListener('click', async () => {
        try {
          const r = await CleanupFromTrace(btn.dataset.traceId)
          showToast(r?.message || '', !!r?.ok)
          refreshTraceList()
        } catch (err) {
          showToast(String(err), false)
        }
      })
    })
  } catch (err) {
    host.innerHTML = `<div class="empty">${escapeHtml(String(err))}</div>`
  }
}

function wireUI() {
  document.querySelectorAll('.nav-item').forEach((btn) => {
    btn.addEventListener('click', () => setActiveTab(btn.dataset.tab))
  })

  document.querySelectorAll('.btn.goal').forEach((btn) => {
    btn.addEventListener('click', () => {
      upgradeGoal = btn.dataset.goal || 'max_fps'
      localStorage.setItem('zp_upgrade_goal', upgradeGoal)
      refreshUpgradeAdvisor()
    })
  })
  $('btn-upgrade-refresh')?.addEventListener('click', () => refreshUpgradeAdvisor())

  $('btn-uninstall-refresh')?.addEventListener('click', () => refreshInstalledApps())
  $('uninstall-search')?.addEventListener('input', (ev) => renderInstalledApps(ev.target.value))
  $('btn-uninstall-precheck')?.addEventListener('click', () => runUninstallPrecheck())
  $('btn-uninstall-kill')?.addEventListener('click', () => runUninstall(true))
  $('btn-uninstall-only')?.addEventListener('click', () => runUninstall(false))
  $('btn-uninstall-open')?.addEventListener('click', () => openSelectedAppFolder())
  $('btn-residual-scan')?.addEventListener('click', () => runResidualScan())
  $('btn-residual-delete')?.addEventListener('click', () => runResidualDelete())
  $('btn-clean-preview')?.addEventListener('click', () => refreshCleanPreview())
  $('btn-clean-run')?.addEventListener('click', () => runSafeClean())

  $('btn-sysclean-scan')?.addEventListener('click', scanSystemClean)
  $('btn-sysclean-run')?.addEventListener('click', runSystemCleanAction)
  $('clean-chk-temp')?.addEventListener('change', updateSystemCleanRunButton)
  $('clean-chk-browser')?.addEventListener('change', updateSystemCleanRunButton)
  $('clean-chk-directx')?.addEventListener('change', updateSystemCleanRunButton)

  $('btn-dev-rescan')?.addEventListener('click', async () => {
    try {
      showToast(t('checking') || 'Сканирование...', true)
      await ForceRescanHardware()
      showToast(t('devRescanHw') || 'Железо пересканировано', true)
      await refreshDevTelemetry()
    } catch (err) {
      showToast(String(err), false)
    }
  })
  $('btn-dev-clearcache')?.addEventListener('click', async () => {
    try {
      await ResetAppCache()
      showToast(t('devClearCache') || 'Кэш сброшен', true)
    } catch (err) {
      showToast(String(err), false)
    }
  })
  $('btn-dev-checkadmin')?.addEventListener('click', async () => {
    try {
      const ok = await CheckAdminStatus()
      showToast(ok ? 'PulseCore: права Администратора активны' : 'PulseCore: обычные права (без Администратора)', ok)
    } catch (err) {
      showToast(String(err), false)
    }
  })
  $('btn-dev-copylog')?.addEventListener('click', () => {
    const text = $('dev-sensor-log')?.textContent || ''
    navigator.clipboard.writeText(text)
    showToast(t('devLogCopied') || 'Лог датчиков скопирован', true)
  })
  $('btn-dev-clearlog')?.addEventListener('click', async () => {
    try {
      await ClearSensorLogs()
      if ($('dev-sensor-log')) $('dev-sensor-log').textContent = ''
      showToast(t('devClearLog') || 'Лог очищен', true)
    } catch (err) {
      showToast(String(err), false)
    }
  })

  $('btn-trace-start')?.addEventListener('click', async () => {
    try {
      const r = await StartTracedInstall()
      showToast(r?.message || '', !!r?.ok)
    } catch (err) {
      showToast(String(err), false)
    }
  })
  $('btn-trace-finish')?.addEventListener('click', async () => {
    try {
      const r = await FinishTracedInstall()
      showToast(r?.message || '', !!r?.ok)
      refreshTraceList()
    } catch (err) {
      showToast(String(err), false)
    }
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

  const syncMaximiseButton = (isMax) => {
    const btn = $('btn-fs')
    if (!btn) return
    btn.textContent = isMax ? '❐' : '□'
    btn.title = isMax ? 'Restore' : 'Maximize'
    btn.setAttribute('aria-label', btn.title)
  }

  const toggleAppMaximise = async () => {
    try {
      const isMax = await TitlebarToggleMaximise()
      syncMaximiseButton(isMax)
    } catch (_) {
      try {
        WindowToggleMaximise()
        const isMax = await WindowIsMaximised()
        syncMaximiseButton(isMax)
      } catch (_) {}
    }
  }

  $('btn-min').addEventListener('click', () => WindowMinimise())
  $('btn-fs').addEventListener('click', toggleAppMaximise)
  $('btn-close').addEventListener('click', async () => {
    try { await HideToTray() } catch (_) {}
  })

  const titlebarEl = $('titlebar')
  if (titlebarEl) {
    titlebarEl.addEventListener('mousedown', async (ev) => {
      if (ev.button !== 0 || ev.target.closest('.titlebar-actions') || ev.target.closest('button')) return
      try {
        const isMax = await WindowMaximisedState()
        if (isMax) {
          await TitlebarDragBegin()
          syncMaximiseButton(false)
        }
      } catch (_) {}
    })
    titlebarEl.addEventListener('dblclick', async (ev) => {
      if (ev.target.closest('.titlebar-actions') || ev.target.closest('button')) return
      await toggleAppMaximise()
    })
  }

  window.addEventListener('resize', async () => {
    try {
      const isMax = await WindowMaximisedState()
      syncMaximiseButton(isMax)
    } catch (_) {}
  })

  $('ov-disks')?.addEventListener('click', async (ev) => {
    const row = ev.target.closest('[data-drive]') || ev.target.closest('.disk-row')
    const letter = row?.dataset?.drive || selectedDrive || 'C'
    await runDiskHealth(letter, false)
  })

  document.addEventListener('keydown', async (ev) => {
    if (ev.key === 'Escape') {
      const diag = $('diag-modal')
      const about = $('about-modal')
      const modal = $('modal')
      if (diag && !diag.classList.contains('hidden')) {
        ev.preventDefault()
        closeDiagModal()
        return
      }
      if (about && !about.classList.contains('hidden')) {
        ev.preventDefault()
        about.classList.add('hidden')
        return
      }
      if (modal && !modal.classList.contains('hidden')) {
        ev.preventDefault()
        modal.classList.add('hidden')
        return
      }
    }
    if (ev.key === 'F11') {
      ev.preventDefault()
      await toggleAppMaximise()
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
  $('btn-check-updates')?.addEventListener('click', () => {
    runUpdateCheck({ hosts: [$('about-update-result')] })
  })
  $('btn-check-updates-settings')?.addEventListener('click', () => {
    runUpdateCheck({ hosts: [$('settings-update-result')] })
  })
  $('update-banner-download')?.addEventListener('click', () => {
    openDownloadURL($('update-banner')?.dataset?.downloadUrl || lastUpdateResult?.downloadURL)
  })
  $('update-banner-close')?.addEventListener('click', hideUpdateBanner)
  $('update-banner-x')?.addEventListener('click', hideUpdateBanner)
  $('btn-check-sensors')?.addEventListener('click', openSelfDiagnostics)
  $('diag-close')?.addEventListener('click', closeDiagModal)
  $('diag-modal')?.addEventListener('click', (ev) => {
    if (ev.target.id === 'diag-modal') closeDiagModal()
  })
  $('btn-copy-diag')?.addEventListener('click', copyDiagReport)
  const hotkeyInput = $('mh-hotkey')
  if (hotkeyInput) {
    hotkeyInput.value = hudPrefs.hotkey || 'Ctrl+Shift+M'
    hotkeyInput.addEventListener('change', () => applyMiniHUDHotkeySetting(hotkeyInput.value))
    hotkeyInput.addEventListener('keydown', (ev) => {
      if (ev.key === 'Enter') {
        ev.preventDefault()
        applyMiniHUDHotkeySetting(hotkeyInput.value)
      }
    })
  }
  applyMiniHUDHotkeySetting(hudPrefs.hotkey, { toast: false }).catch(() => {})
  GetMiniHUDHotkey?.().then((res) => {
    if (res?.chord && hotkeyInput) {
      hotkeyInput.value = res.chord
      hudPrefs.hotkey = res.chord
      saveHudPrefs()
    }
  }).catch(() => {})
  $('about-github')?.addEventListener('click', (ev) => {
    ev.preventDefault()
    try {
      BrowserOpenURL(GITHUB_URL)
    } catch (_) {
      window.open(GITHUB_URL, '_blank', 'noopener,noreferrer')
    }
  })

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
        const vamps = filterNoiseVampires(lastMetrics?.vampires || []).filter((v) => !v.ecoMode && !v.frozen)
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
        const vamp = filterNoiseVampires(lastMetrics?.vampires || []).find((v) => Number(v.pid) === pid)
        const lag = filterNoiseLags(lastMetrics?.lagEvents || []).find((e) => Number(e.culpritPID) === pid)
        if (pid && (vamp || lag)) {
          await FreezeProcess(pid)
          frozen.add(pid)
          showToast(t('insightEcoDone'), true)
          refresh()
        }
      } else if (act === 'eco-name') {
        const name = btn.dataset.ecoName
        if (name && !isNoiseBackgroundProcess(name)) {
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
        setActiveTab('uninstall')
        $('clean-panel')?.scrollIntoView({ behavior: 'smooth', block: 'center' })
        try {
          await refreshCleanPreview()
        } catch (_) {}
      }
    } catch (err) {
      showToast(String(err), false)
    }
  }
  $('insight-list')?.addEventListener('click', onInsightAct)
  $('lag-insight-list')?.addEventListener('click', onInsightAct)
  $('insight-more')?.addEventListener('click', () => {
    insightsExpanded = !insightsExpanded
    if (lastMetrics) renderInsights(lastMetrics)
  })

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

  $('modal-smart-refresh')?.addEventListener('click', () => runDiskHealth(selectedDrive, true))
  $('hw-ssd-list')?.addEventListener('toggle', (ev) => {
    const acc = ev.target?.closest?.('.hw-ssd-acc')
    if (!acc || !acc.open) return
    fillHwSSDDetails(acc, false)
  }, true)
  // Prefetch SMART for the first visible SSD accordion when Hardware opens.
  document.querySelector('.nav-item[data-tab="hardware"]')?.addEventListener('click', () => {
    setTimeout(() => {
      const first = $('hw-ssd-list')?.querySelector('.hw-ssd-acc[open], .hw-ssd-acc')
      if (first) {
        first.open = true
        fillHwSSDDetails(first, false)
      }
    }, 80)
  })
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
    // Instant local filter of the full catalog; Steam online after a short pause.
    searchGamesLocal(q)
    searchTimer = setTimeout(() => searchGamesLive({ online: true }), 280)
  })
  searchInput?.addEventListener('keydown', async (ev) => {
    if (ev.key !== 'Enter') return
    clearTimeout(searchTimer)
    await searchGamesLive({ online: true })
  })

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
    renderOverviewDiskDetails()
    await runDiskHealth(selectedDrive, false)
  }
  $('disk-tabs')?.addEventListener('click', onDiskTabClick)
  $('modal-disk-tabs')?.addEventListener('click', onDiskTabClick)

  $('weekly-list')?.addEventListener('click', async (ev) => {
    const eco = ev.target.closest('[data-weekly-eco]')
    const hide = ev.target.closest('[data-weekly-hide]')
    if (eco) {
      const name = eco.dataset.weeklyEco
      if (isNoiseBackgroundProcess(name) || isDefenderName(name)) return
      try {
        await EcoModeByProcessName(name, true)
      } catch (_) {}
      eco.textContent = t('ecoLimited')
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
    if (isNoiseBackgroundProcess(name)) return
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
    if (name && isNoiseBackgroundProcess(name)) return
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
    // Near left edge / sidebar: nudge right.
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
  // Production wails build: Dev Mode nav/badge never appear (compile-time flag).
  if (DEV_MODE_ENABLED) {
    initDevMode({
      openAbout,
      showToast,
      setActiveTab,
      t,
      applyPoll: async (ms) => {
        await SetPollInterval(Number(ms) || 1000)
        document.querySelectorAll('.btn.poll').forEach((b) => {
          b.classList.toggle('active', Number(b.dataset.poll) === Number(ms))
        })
      },
      onChange: () => {
        if (lastMetrics) {
          renderHardware(lastMetrics)
          renderInsights(lastMetrics)
        }
      },
    })
  } else {
    $('dev-mode-badge')?.classList.add('hidden')
    document.querySelectorAll('.dev-mode-nav').forEach((n) => n.classList.add('hidden'))
    // Keep the panel out of the active stack; .tab CSS already hides inactive tabs.
    $('tab-dev')?.classList.remove('active')
  }
  $('btn-restart-admin')?.addEventListener('click', () => { restartAsAdmin() })
  $('hw-temp-tip-btn')?.addEventListener('click', () => {
    showToast(t('sensorUnavailableTip'), false)
  })
  document.addEventListener('visibilitychange', () => {
    setChartActive(!document.hidden)
  })
  SetLanguage(lang).catch(() => {})
  syncSettingsVersionLabel()
  refresh()
  refreshWeekly()
  refreshPeripherals()
  loadGames()
  setInterval(refresh, 1000)
  setChartActive(true)
  // Background update check shortly after launch
  setTimeout(() => {
    runUpdateCheck({ showBanner: true }).catch(() => {})
  }, 3000)

  try {
    EventsOn('metrics:update', (m) => {
      applyMetrics(m)
      if (m && typeof m.windowVisible === 'boolean') {
        setChartActive(!!m.windowVisible && !document.hidden)
      }
    })
    EventsOn('thermal:alarm', () => showToast(t('thermal')))
    EventsOn('ui:minihud', (enabled) => setHudMode(!!enabled))
    EventsOn('ui:gameoverlay', (enabled) => setGameOverlayMode(!!enabled, true))
    EventsOn('ui:visibility', (payload) => {
      const visible = payload && typeof payload === 'object' ? !!payload.visible : !!payload
      setChartActive(visible && !document.hidden)
    })
    EventsOn('peripherals:update', (items) => renderPeripherals(items || []))
  } catch (_) {}

  try {
    const params = new URLSearchParams(window.location.search)
    const tabParam = params.get('tab')
    if (tabParam) setActiveTab(tabParam)
  } catch (_) {}
}

boot()
