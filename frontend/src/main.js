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
  GetWeeklySummary,
  DiagnoseLagOrigin,
  CheckSleepBlockers,
  EcoModeByProcessName,
  GetConnectedPeripherals,
  ListGames,
  SearchGames,
  SearchGamesOnline,
  PredictGameFPS,
  GetGameModeStatus,
  SetGameModeEnabled,
  GetProcessRadar,
  GetMemoryLeakWatch,
  GetSystemHealth,
  GetStartupEntries,
  AnalyzeStartup,
  SetAutostartEnabled,
  IsAutostartEnabled,
  GetNetworkMonitor,
  BuildDiagnosticReport,
  GetIntelligenceInsights,
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
  cpu: { stroke: '#94a3b8', top: 'rgba(148, 163, 184, 0.22)', bottom: 'rgba(148, 163, 184, 0.02)' },
  gpu: { stroke: '#64748b', top: 'rgba(100, 116, 139, 0.22)', bottom: 'rgba(100, 116, 139, 0.02)' },
  ram: { stroke: '#cbd5e1', top: 'rgba(203, 213, 225, 0.18)', bottom: 'rgba(203, 213, 225, 0.02)' },
  battery: { stroke: '#fbbf24', top: 'rgba(251, 191, 36, 0.18)', bottom: 'rgba(251, 191, 36, 0.02)' },
}

let lang = localStorage.getItem('zp_lang') || 'ru'
let frozen = new Set()
let activeMetric = 'cpu'
let selectedDrive = 'C'
let hudEnabled = false
let gameOverlayOn = false
let latestDisks = []
let gameCatalog = []
let selectedGameId = ''
const periphCollapsed = new Set(JSON.parse(localStorage.getItem('zp_periph_collapsed') || '[]'))
const cachedDiskReport = Object.create(null)
const whitelist = new Set(JSON.parse(localStorage.getItem('zp_whitelist') || '[]'))
const chartScroll = { pos: 0, target: 0 }
const frameSamples = []

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
  ;['lang-select', 'lang-select-settings'].forEach((id) => {
    const el = $(id)
    if (el) el.value = lang
  })
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
    if (btn.classList.contains('nav-hud')) return
    btn.classList.toggle('active', btn.dataset.tab === name)
  })
  document.querySelectorAll('.tab').forEach((tab) => {
    tab.classList.toggle('active', tab.id === `tab-${name}`)
  })
  if (name === 'hardware') refreshPeripherals()
  if (name === 'overview') { refreshWeekly(); refreshHealth(); refreshIntel() }
  if (name === 'lag') { refreshRadar(); refreshLeaks() }
  if (name === 'settings') { refreshStartup(); refreshGameMode(); refreshAutostart() }
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
  if (pct < 20) return { stroke: '#d35a66', top: 'rgba(211, 90, 102, 0.28)', bottom: 'rgba(211, 90, 102, 0.02)' }
  if (pct < 40) return { stroke: '#d4a017', top: 'rgba(212, 160, 23, 0.26)', bottom: 'rgba(212, 160, 23, 0.02)' }
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
  const cssH = 160
  if (canvas.width !== Math.floor(cssW * dpr) || canvas.height !== Math.floor(cssH * dpr)) {
    canvas.width = Math.floor(cssW * dpr)
    canvas.height = Math.floor(cssH * dpr)
  }
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
  ctx.clearRect(0, 0, cssW, cssH)

  const padL = 36
  const padR = 36
  const padT = 14
  const padB = 28
  const plotW = cssW - padL - padR
  const plotH = cssH - padT - padB
  const marks = [
    { pct: 100, label: '100%' },
    { pct: 50, label: '50%' },
    { pct: 0, label: '0%' },
  ]
  ctx.setLineDash([3, 4])
  ctx.lineWidth = 1
  ctx.font = '10px ui-monospace, Consolas, monospace'
  marks.forEach((m) => {
    const y = padT + plotH - (m.pct / 100) * plotH
    ctx.strokeStyle = 'rgba(255, 255, 255, 0.08)'
    ctx.beginPath()
    ctx.moveTo(padL, y)
    ctx.lineTo(cssW - padR, y)
    ctx.stroke()
    ctx.fillStyle = 'rgba(255, 255, 255, 0.45)'
    ctx.fillText(m.label, 4, y + 3)
  })
  ctx.setLineDash([])

  const timeLabels = [
    { t: 0, label: '-30s' },
    { t: 1 / 3, label: '-20s' },
    { t: 2 / 3, label: '-10s' },
    { t: 1, label: t('nowLabel') },
  ]
  ctx.fillStyle = 'rgba(255, 255, 255, 0.45)'
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
  ctx.lineWidth = 2.2
  ctx.lineJoin = 'round'
  ctx.lineCap = 'round'
  ctx.strokeStyle = theme.stroke
  strokeSmooth(ctx, pts)

  const cur = $('chart-current')
  if (cur) cur.textContent = `${t('currentPct')}: ${fmt(last, 1)}%`
}

function animateChart(now) {
  drainQueues(now)
  lerpSeries()
  drawTrendFrame()
  sampleFrametime(now)
  requestAnimationFrame(animateChart)
}

function fmt(n, digits = 1) {
  const v = Number(n)
  if (!Number.isFinite(v)) return '--'
  return v.toFixed(digits)
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
  $('ov-cpu').textContent = `${fmt(m.cpuPercent)}%`
  $('ov-cpu-sub').textContent = m.cpuName || '-'
  $('ov-cpu-bar').style.width = `${Math.min(100, m.cpuPercent || 0)}%`

  $('ov-gpu').textContent = `${fmt(m.gpuPercent)}%`
  $('ov-gpu-sub').textContent = m.gpuName || '-'
  $('ov-gpu-bar').style.width = `${Math.min(100, m.gpuPercent || 0)}%`

  $('ov-ram').textContent = `${fmt(m.ramPercent)}%`
  $('ov-ram-sub').textContent = `${m.ramUsedMB || 0} / ${m.ramTotalMB || 0} MB`
  $('ov-ram-bar').style.width = `${Math.min(100, m.ramPercent || 0)}%`

  const bat = m.battery || {}
  if (bat.available) {
    $('ov-bat').textContent = `${fmt(bat.percent, 0)}%`
    const wear = bat.wearLabel ? ` · ${bat.wearLabel}` : ''
    $('ov-bat-sub').textContent = `${bat.statusText || '-'} · ${bat.deviceName || t('batteryPack')}${wear}`
    $('ov-bat-bar').style.width = `${Math.min(100, bat.percent || 0)}%`
  } else {
    $('ov-bat').textContent = 'AC'
    $('ov-bat-sub').textContent = bat.wearLabel || bat.statusText || t('desktopPC')
    $('ov-bat-bar').style.width = '100%'
  }

  $('temp-chip').textContent = `${fmt(m.temperatureC)} °C`
  $('cpu-name').textContent = m.cpuName || 'CPU'

  latestDisks = m.disks || []
  renderDiskTabs(latestDisks)
  const box = $('ov-disks')
  const disks = latestDisks
  if (!disks.length) {
    box.innerHTML = `<div class="empty">-</div>`
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
        <div class="name">${fmt(d.percent, 0)}%</div>
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

function verdictText(e) {
  const key = e.verdictKey || e.cause
  if (key && typeof key === 'string' && !key.includes(' ') && t(key) !== key) return t(key)
  if (key && typeof key === 'string' && key.startsWith('verdict') && t(key) !== key) return t(key)
  // Legacy RU free-text causes still render as-is.
  return key || t('verdictUnknown')
}

function renderTimeline(e) {
  const box = $('lag-timeline')
  if (!box) return
  const pre = e?.preWindow || []
  const post = e?.postWindow || []
  const pts = [...pre, ...post]
  if (!pts.length) {
    box.classList.add('hidden')
    box.innerHTML = ''
    return
  }
  box.classList.remove('hidden')
  const maxCPU = Math.max(1, ...pts.map((p) => p.cpuPercent || 0))
  box.innerHTML = `
    <div class="panel-title">${t('timelineTitle')}</div>
    <div class="timeline-bars">
      ${pts.map((p) => {
        const h = Math.round(((p.cpuPercent || 0) / maxCPU) * 100)
        const disk = (p.diskReadMBps || 0) + (p.diskWriteMBps || 0)
        return `<div class="tl-col" title="t=${fmt(p.offsetSec, 1)}s CPU ${fmt(p.cpuPercent, 0)}% disk ${fmt(disk, 1)} MB/s">
          <i style="height:${Math.max(4, h)}%"></i>
          <span>${fmt(p.offsetSec, 0)}s</span>
        </div>`
      }).join('')}
    </div>
  `
}

function renderLag(m) {
  const stutters = (m.stutterEvents || []).filter((e) => (e.jitterMs || 0) >= 300)
  const banner = $('smoothness-banner')
  if (banner) {
    if (!stutters.length) {
      banner.className = 'status-banner ok'
      banner.textContent = t('verdictWatching')
    } else {
      const top = stutters[0]
      const level = top.severity === 'critical' ? 'warn' : (top.severity === 'unknown' ? 'unknown' : 'warn')
      banner.className = `status-banner ${level === 'unknown' ? 'unknown' : 'warn'}`
      const conf = top.confidence != null ? ` · ${t('confidenceLabel')} ${Math.round((top.confidence || 0) * 100)}%` : ''
      banner.textContent = `${verdictText(top)} (+${fmt(top.jitterMs, 0)} ms)${conf}`
    }
  }
  const ring = $('blackbox-ring')
  if (ring && window.go?.main?.App?.GetSnapshotRingLen) {
    window.go.main.App.GetSnapshotRingLen().then((n) => {
      ring.textContent = t('blackboxRing').replace('{n}', String(n ?? 0))
    }).catch(() => {})
  }

  const stutterBox = $('stutter-log')
  if (stutterBox) {
    if (!stutters.length) {
      stutterBox.innerHTML = `<div class="empty">${t('verdictWatching')}</div>`
      renderTimeline(null)
    } else {
      stutterBox.innerHTML = stutters.map((e, idx) => `
        <div class="row stutter-row ${idx === 0 ? 'active' : ''}" data-stutter-idx="${idx}">
          <div>
            <div class="name">${e.timestamp} · ${fmt(e.jitterMs, 0)} ms · ${t(e.causeKey || 'verdictCauseUnknown')}</div>
            <div class="meta stutter-cause">${verdictText(e)}${e.detailKey && t(e.detailKey) !== e.detailKey ? ' — ' + t(e.detailKey) : ''}</div>
            ${e.culprit ? `<div class="meta">${e.culprit}</div>` : ''}
          </div>
          <div class="row-actions">
            ${e.culprit ? `<button type="button" class="btn compact danger" data-eco-name="${e.culprit}">${t('ecoCulprit')}</button>` : ''}
          </div>
        </div>
      `).join('')
      renderTimeline(stutters[0])
      stutterBox.querySelectorAll('.stutter-row').forEach((row) => {
        row.addEventListener('click', () => {
          const i = Number(row.getAttribute('data-stutter-idx') || 0)
          stutterBox.querySelectorAll('.stutter-row').forEach((r) => r.classList.remove('active'))
          row.classList.add('active')
          renderTimeline(stutters[i])
        })
      })
    }
  }

  const lags = m.lagEvents || []
  const lagBox = $('lag-log')
  if (!lags.length) {
    lagBox.innerHTML = `<div class="empty">${t('noLags')}</div>`
  } else {
    lagBox.innerHTML = lags.map((e) => `
      <div class="incident-card">
        <div class="incident-ico">${e.icon || '⚙️'}</div>
        <div>
          <div class="incident-title">${e.culpritFriendly || e.culpritProcess || t('unknownProc')}</div>
          <div class="incident-explain">${lagExplanation(e)}</div>
          <div class="meta">${e.timestamp} · ${e.culpritProcess || ''} · PID ${e.culpritPID || '-'}</div>
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
      return `
        <div class="row">
          <div>
            <div class="name">${v.friendlyName || v.name}${badge}</div>
            <div class="meta">${v.name}</div>
            <div class="meta">CPU ${fmt(v.cpuPercent)}% · disk ~${fmt(v.diskMBps)} MB/s</div>
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

  $('hw-cpu-name').textContent = cpu.name || m.cpuName || '-'
  $('hw-cpu-cores').textContent = `${cpu.physicalCores || '-'} / ${cpu.logicalCores || '-'}`
  $('hw-cpu-freq').textContent = cpu.frequencyMHz ? `${fmt(cpu.frequencyMHz / 1000, 2)} GHz` : '-'
  $('hw-cpu-load').textContent = `${fmt(cpu.percent ?? m.cpuPercent)}%`

  const cores = cpu.perCore || m.cpuPerCore || []
  $('core-grid').innerHTML = cores.map((c, i) => `
    <div class="core-cell">
      <div class="id">C${i}</div>
      <div class="val">${fmt(c, 0)}%</div>
      <div class="mini-bar"><i style="width:${Math.min(100, c)}%"></i></div>
    </div>
  `).join('')

  $('hw-gpu-name').textContent = gpu.name || m.gpuName || '-'
  $('hw-gpu-driver').textContent = gpu.driverVersion || '-'
  $('hw-gpu-ded').textContent = `${fmt(gpu.dedicatedMB ?? m.vramTotalMB, 0)} MB`
  $('hw-gpu-shared').textContent = `${fmt(gpu.sharedMB ?? m.vramSharedMB, 0)} MB`

  $('hw-ram-total').textContent = `${ram.totalMB || m.ramTotalMB || 0} MB`
  $('hw-ram-type').textContent = `${ram.type || 'Unknown'}${ram.speedMHz ? ` · ${ram.speedMHz} MHz` : ''}`
  $('hw-ram-slots').textContent = `${ram.slotsUsed || 0}/${ram.slotsTotal || 0}`
  $('hw-ram-use').textContent = `${ram.usedMB || m.ramUsedMB || 0} / ${ram.freeMB || 0} MB`
  const channel = $('hw-ram-channel')
  if (channel) {
    channel.textContent = ram.channelStatus || '-'
    channel.classList.remove('warn', 'ok')
    if (ram.isSingleChannel) channel.classList.add('warn')
    else if ((ram.slotsUsed || 0) >= 2) channel.classList.add('ok')
  }

  if (pwr.hasBattery) {
    $('hw-pwr-name').textContent = pwr.deviceName || t('batteryPack')
    $('hw-pwr-status').textContent = pwr.statusText || '-'
    $('hw-pwr-pct').textContent = `${fmt(pwr.percent, 0)}%`
    $('hw-pwr-wear').textContent = pwr.wearLabel || `${fmt(pwr.wearPercent, 0)}%`
    $('hw-pwr-cap').textContent = pwr.capacitymWh
      ? `${fmt(pwr.capacitymWh, 0)} / ${fmt(pwr.designCapacitymWh, 0)} mWh`
      : '-'
  } else {
    $('hw-pwr-name').textContent = t('desktopPC')
    $('hw-pwr-status').textContent = 'AC'
    $('hw-pwr-pct').textContent = 'AC'
    $('hw-pwr-wear').textContent = pwr.wearLabel || t('desktopPC')
    $('hw-pwr-cap').textContent = '-'
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

async function refreshHealth() {
  const box = $('health-box')
  if (!box) return
  try {
    const h = await GetSystemHealth()
    const tips = (h.tips || []).map((k) => t(k)).filter(Boolean).join(' · ')
    const factors = (h.factors || []).map((f) => `${f.key} −${f.impact}`).join(', ')
    box.innerHTML = `
      <div class="row">
        <div>
          <div class="name">${t(h.summaryKey || 'healthGood')} · ${h.score}/100</div>
          <div class="meta">${h.level || ''}${factors ? ' · ' + factors : ''}</div>
          ${tips ? `<div class="meta">${tips}</div>` : ''}
        </div>
      </div>`
  } catch (err) {
    box.innerHTML = `<div class="empty">${String(err)}</div>`
  }
}

async function refreshRadar() {
  const box = $('radar-list')
  if (!box) return
  try {
    const rows = await GetProcessRadar()
    if (!rows?.length) {
      box.innerHTML = `<div class="empty">—</div>`
      return
    }
    box.innerHTML = rows.slice(0, 8).map((r) => `
      <div class="row">
        <div>
          <div class="name">${r.name} · ${r.trend || ''}</div>
          <div class="meta">CPU ${fmt(r.cpuPercent, 1)}% · RSS ${fmt(r.rssMB, 0)} MB · score ${fmt(r.score, 0)}</div>
        </div>
      </div>`).join('')
  } catch (err) {
    box.innerHTML = `<div class="empty">${String(err)}</div>`
  }
}

async function refreshLeaks() {
  const box = $('leak-list')
  if (!box) return
  try {
    const rows = await GetMemoryLeakWatch()
    if (!rows?.length) {
      box.innerHTML = `<div class="empty">${t('leakEmpty')}</div>`
      return
    }
    box.innerHTML = rows.map((r) => `
      <div class="row">
        <div>
          <div class="name">${r.name}</div>
          <div class="meta">${t(r.summaryKey || 'leakSuspect')} · +${fmt(r.deltaMB, 0)} MB · RSS ${fmt(r.rssMB, 0)} MB</div>
        </div>
      </div>`).join('')
  } catch (err) {
    box.innerHTML = `<div class="empty">${String(err)}</div>`
  }
}

async function refreshNetMon() {
  const box = $('net-mon')
  if (!box) return
  try {
    const n = await GetNetworkMonitor()
    box.innerHTML = `
      <div class="row">
        <div>
          <div class="name">${t(n.noteKey || 'netMonOK')} · ${n.level || ''}</div>
          <div class="meta">${n.primaryIface || '-'} · ↓ ${fmt(n.recvMBps, 2)} ↑ ${fmt(n.sendMBps, 2)} MB/s · gw ${fmt(n.gatewayProbeMs, 0)} ms</div>
        </div>
      </div>`
  } catch (err) {
    box.innerHTML = `<div class="empty">${String(err)}</div>`
  }
}

async function refreshIntel() {
  const box = $('intel-list')
  if (!box) return
  try {
    const rows = await GetIntelligenceInsights()
    box.innerHTML = (rows || []).map((r) => `
      <div class="row">
        <div>
          <div class="name">${t(r.titleKey)} · ${r.severity || ''}</div>
          <div class="meta">${t(r.detailKey)}</div>
          <div class="meta">${t(r.actionKey)}</div>
        </div>
      </div>`).join('')
  } catch (err) {
    box.innerHTML = `<div class="empty">${String(err)}</div>`
  }
}

async function refreshStartup() {
  const box = $('startup-list')
  if (!box) return
  try {
    const a = await AnalyzeStartup()
    const head = `<div class="row"><div><div class="name">${t(a.summaryKey || 'startupOK')}</div><div class="meta">${t(a.noteKey || 'startupNote')} · ${a.total || 0} / heavy ${a.heavyCount || 0}</div></div></div>`
    const rows = (a.entries || []).slice(0, 12).map((e) => `
      <div class="row">
        <div>
          <div class="name">${e.name}${e.heavy ? ' · heavy' : ''}</div>
          <div class="meta">${e.location} · ${e.impact} · ${e.command || ''}</div>
        </div>
      </div>`).join('')
    box.innerHTML = head + rows
  } catch (err) {
    box.innerHTML = `<div class="empty">${String(err)}</div>`
  }
}

async function refreshGameMode() {
  const st = $('gamemode-status')
  const btn = $('btn-gamemode-toggle')
  try {
    const g = await GetGameModeStatus()
    if (btn) btn.textContent = g.enabled ? t('gameModeDisable') : t('gameModeEnable')
    if (st) {
      st.textContent = g.active
        ? `${t('gameModeActive')}: ${g.gameName || ''} (${g.sessionSec || 0}s)`
        : t(g.noteKey || 'gameModeIdle')
    }
  } catch (_) {}
}

async function refreshAutostart() {
  const btn = $('btn-autostart')
  if (!btn) return
  try {
    const on = await IsAutostartEnabled()
    btn.textContent = on ? t('autostartOn') : t('autostartOff')
    btn.classList.toggle('active', !!on)
  } catch (_) {}
}

async function refreshWeekly() {
  const box = $('weekly-list')
  if (!box) return
  box.innerHTML = `<div class="empty">${t('weeklyLoading')}</div>`
  try {
    let localHead = ''
    try {
      const sum = await GetWeeklySummary()
      if (sum && sum.ok) {
        localHead = `
          <div class="row weekly-local">
            <div>
              <div class="name">${t('weeklyLocalTitle')}</div>
              <div class="meta">CPU ${fmt(sum.cpuAvg, 0)}% · RAM ${fmt(sum.ramAvg, 0)}% · GPU ${fmt(sum.gpuAvg, 0)}%</div>
              <div class="meta">${t('weeklyLags')}: ${sum.lagCount || 0} · Eco: ${sum.ecoToggles || 0}${sum.topProcess ? ' · ' + sum.topProcess : ''}</div>
              <div class="meta">${t(sum.noteKey || 'weeklyLocalNote')}</div>
            </div>
          </div>`
      }
    } catch (_) {}
    const r = await GetMonthlyAppReport()
    const apps = (r.topApps || []).filter((a) => !whitelist.has(String(a.name || '').toLowerCase()))
    const appsHTML = !apps.length
      ? `<div class="empty">${t('weeklyEmpty')}</div>`
      : apps.map((a, i) => {
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
    box.innerHTML = localHead + appsHTML
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

function renderMiniHUD(m) {
  $('mh-cpu').textContent = `${fmt(m.cpuPercent, 0)}%`
  $('mh-ram').textContent = `${fmt(m.ramPercent, 0)}%`
  $('mh-gpu').textContent = `${fmt(m.gpuPercent, 0)}%`
}

function syncSettings(m) {
  document.querySelectorAll('.btn.mode').forEach((btn) => {
    btn.classList.toggle('active', btn.dataset.mode === (m.powerMode || 'balanced'))
  })
  document.querySelectorAll('.btn.poll').forEach((btn) => {
    btn.classList.toggle('active', Number(btn.dataset.poll) === Number(m.pollIntervalMS || 1000))
  })
}

function applyMetrics(m) {
  if (!m) return
  if (m.language && m.language !== lang && !localStorage.getItem('zp_lang')) {
    lang = m.language
    applyI18n()
  }
  pushSample('cpu', m.cpuPercent || 0)
  pushSample('gpu', m.gpuPercent || 0)
  pushSample('ram', m.ramPercent || 0)
  pushSample('battery', batterySeriesValue(m))
  renderOverview(m)
  renderLag(m)
  renderHardware(m)
  renderMiniHUD(m)
  syncSettings(m)
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

function gamePosterHTML(g) {
  const name = String(g.name || 'Game').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;')
  const safeName = String(g.name || 'Game').replace(/\\/g, '\\\\').replace(/'/g, "\\'")
  const primary = g.posterUrl || ''
  const secondary = g.headerUrl || steamHeaderFromPoster(primary)
  if (!primary && !secondary) return posterFallbackHTML(g.name, 'game-poster')
  const src = primary || secondary
  const fb = secondary && secondary !== src ? secondary : ''
  return `<img class="game-poster" src="${src}" alt="${name}" loading="lazy" data-fb="${fb}" onerror="if(this.dataset.fb){const u=this.dataset.fb;this.dataset.fb='';this.src=u;}else{this.replaceWith(Object.assign(document.createElement('div'),{className:'game-poster fallback',textContent:'${safeName}'}))}" />`
}

function compactCoverHTML(p) {
  const name = String(p.gameName || 'Game')
  const safeName = name.replace(/\\/g, '\\\\').replace(/'/g, "\\'")
  const primary = p.posterUrl || ''
  const secondary = p.headerUrl || steamHeaderFromPoster(primary)
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
  const box = $('license-box')
  if (box) box.textContent = MIT_LICENSE
  $('about-modal')?.classList.remove('hidden')
}

function closeAbout() {
  $('about-modal')?.classList.add('hidden')
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
    <div class="modal-row"><span class="k">${t('temp')}</span><span class="v">${r.temperatureC ? fmt(r.temperatureC, 0) + ' °C' : t('noControllerTemp')}</span></div>
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
    if (btn.classList.contains('nav-hud')) return
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

  ;['lang-select', 'lang-select-settings'].forEach((id) => {
    $(id)?.addEventListener('change', (ev) => changeLanguage(ev.target.value))
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

  document.querySelectorAll('.btn.poll').forEach((btn) => {
    btn.addEventListener('click', async () => {
      await SetPollInterval(Number(btn.dataset.poll))
      document.querySelectorAll('.btn.poll').forEach((b) => b.classList.toggle('active', b === btn))
    })
  })

  const enableHud = async () => {
    await ToggleMiniHUD(true)
    setHudMode(true)
  }
  const disableHud = async () => {
    await ToggleMiniHUD(false)
    setHudMode(false)
  }

  $('btn-hud-side').addEventListener('click', enableHud)
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

  $('btn-disk-health').addEventListener('click', () => runDiskHealth(selectedDrive, false))
  $('modal-smart-refresh')?.addEventListener('click', () => runDiskHealth(selectedDrive, true))
  $('btn-weekly-refresh')?.addEventListener('click', refreshWeekly)
  $('btn-health-refresh')?.addEventListener('click', refreshHealth)
  $('btn-intel-refresh')?.addEventListener('click', refreshIntel)
  $('btn-radar-refresh')?.addEventListener('click', refreshRadar)
  $('btn-leak-refresh')?.addEventListener('click', refreshLeaks)
  $('btn-net-mon')?.addEventListener('click', refreshNetMon)
  $('btn-startup-refresh')?.addEventListener('click', refreshStartup)
  $('btn-gamemode-toggle')?.addEventListener('click', async () => {
    try {
      const g = await GetGameModeStatus()
      await SetGameModeEnabled(!g.enabled)
      refreshGameMode()
    } catch (_) {}
  })
  $('btn-autostart')?.addEventListener('click', async () => {
    try {
      const on = await IsAutostartEnabled()
      await SetAutostartEnabled(!on)
      refreshAutostart()
    } catch (_) {}
  })
  $('btn-diag-report')?.addEventListener('click', async () => {
    const st = $('diag-status')
    try {
      const r = await BuildDiagnosticReport()
      if (st) st.textContent = `${t(r.summaryKey || 'diagReportReady')}${r.path ? ': ' + r.path : ''}`
      showToast(t(r.summaryKey || 'diagReportReady'))
    } catch (err) {
      if (st) st.textContent = String(err)
    }
  })
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

function boot() {
  applyI18n()
  wireUI()
  SetLanguage(lang).catch(() => {})
  refresh()
  refreshWeekly()
  refreshHealth()
  refreshIntel()
  refreshRadar()
  refreshLeaks()
  refreshStartup()
  refreshGameMode()
  refreshAutostart()
  refreshPeripherals()
  loadGames()
  setInterval(refresh, 1000)
  setInterval(() => { refreshRadar(); refreshLeaks(); refreshGameMode() }, 5000)
  requestAnimationFrame(animateChart)

  try {
    EventsOn('metrics:update', (m) => applyMetrics(m))
    EventsOn('thermal:alarm', () => showToast(t('thermal')))
    EventsOn('ui:minihud', (enabled) => setHudMode(!!enabled))
    EventsOn('ui:gameoverlay', (enabled) => setGameOverlayMode(!!enabled, true))
    EventsOn('peripherals:update', (items) => renderPeripherals(items || []))
    EventsOn('gamemode:update', () => refreshGameMode())
    EventsOn('eco:delta', (d) => {
      if (!d) return
      const key = d.summaryKey || 'ecoDeltaPending'
      const name = d.name || ''
      const before = d.before?.processCPU
      const after = d.after?.processCPU
      let extra = ''
      if (before != null && after != null && d.after?.atUnixMs) {
        extra = ` · ${t('ecoBeforeAfter')} ${fmt(before, 1)}% → ${fmt(after, 1)}%`
      }
      showToast(`${t(key)}${name ? ' · ' + name : ''}${extra}`)
    })
  } catch (_) {}
}

boot()
