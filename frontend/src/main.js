import {
  GetMetrics,
  GetSystemMetrics,
  SetActiveTab,
  FreezeProcess,
  ResumeProcess,
  SetPowerProfile,
  SetPollInterval,
  SetLanguage,
  GetBehaviorSettings,
  SetBehaviorSettings,
  ToggleMiniHUD,
  ToggleGameOverlay,
  QuitApp,
  HideToTray,
  ToggleFullscreen,
  CheckDiskHealth,
  CheckDiskHealthForce,
  RefreshAllDiskHealth,
  GetMonthlyAppReport,
  DiagnoseLagOrigin,
  CheckSleepBlockers,
  EcoModeByProcessName,
  GetConnectedPeripherals,
  ListGames,
  SearchGames,
  SearchGamesOnline,
  PredictGameFPS,
  GetLagIncidents,
  GetIncidentTimeline,
  ApplyEcoWithFeedback,
  ApplyEcoPIDWithFeedback,
  RevertEcoSession,
  GetEcoLimitedGroups,
  RevertEcoGroup,
  RevertAllEco,
  ConfirmQuitAfterEco,
  GetWeeklySummary,
  ClearStats,
  GetActiveGameStatus,
  GetGameSessions,
  CompareGameSessions,
  GetProcessAnomalies,
  ClearProcessAnomalies,
  GetMemoryWatchCards,
  GetSystemHealth,
  GetStartupItems,
  GetStartupBrokenCount,
  RemoveBrokenStartupEntries,
  SetStartupItemEnabled,
  GetNetworkMonitor,
  PreviewDiagnosticReport,
  ExportDiagnosticReport,
  GetIntelligenceStatus,
  ResetIntelligenceProfile,
  GetHomeState,
  RunHomeAction,
  GetDiskWatchInfo,
  SetDiskWatching,
  OpenWindowsBackup,
  GetSmartFixPlan,
  ApplySmartFix,
  RevertSmartFix,
  ApplyDiskCleanup,
  PreviewDiskCleanup,
  ListInstalledApps,
  UninstallWithCleanup,
  RemoveAppLeftovers,
  StartUninstallWithCleanup,
  StartRemovePaths,
  StartRemoveOrphanLeftover,
  ListOrphanLeftovers,
  GetCleanStats,
  GetRecycleBinInfo,
  EmptyRecycleBin,
  CreateRestorePoint,
  OpenFolderInExplorer,
  PreviewDeepCleanup,
  DisableHibernation,
  EnableHibernation,
  HideUserGameMeta,
  CleanOldestShadow,
  CleanDeliveryOptimization,
  RefreshDiskMetricsEx,
  OpenPnPDeviceAction,
  IsElevated,
  RestartAsAdmin,
  GetAdvancedModeState,
  EnableAdvancedMode,
  DisableAdvancedMode,
  GetHelperDiagnostics,
  RestartHelper,
  TitlebarDragBegin,
  TitlebarToggleMaximise,
  WindowMaximisedState,
  ListDisplays,
  TestAndSetRefreshRate,
  ConfirmRefreshRate,
  RevertRefreshRate,
  GetLagDetectorStatus,
  GetUserHiddenGames,
  HideUserGame,
  RestoreHiddenGame,
  ResolveGameMeta,
  SetMiniHUDOpacity,
  SaveMiniHUDPosition,
  ResizeMiniHUD,
  CheckNetworkDetailed,
  CancelNetworkCheck,
  GetLocalDrives,
  IsConflictWatcherEnabled,
  SetConflictWatcherEnabled,
} from '../wailsjs/go/main/App.js'
import { EventsOn, WindowMinimise, BrowserOpenURL, WindowIsMaximised } from '../wailsjs/runtime/runtime.js'

window.__pcPosterError = function (img) {
  if (!img) return
  if (img.dataset.fb) {
    const u = img.dataset.fb
    img.dataset.fb = ''
    img.src = u
    return
  }
  let meta = { name: 'Game', year: 0, genre: '', cls: 'game-poster', hideTitle: false }
  try { meta = { ...meta, ...JSON.parse(img.getAttribute('data-fallback') || '{}') } } catch (_) {}
  try {
    const wrap = document.createElement('div')
    const hideTitle = !!meta.hideTitle
    const safe = String(meta.name || 'Game').replace(/</g, '&lt;').replace(/>/g, '&gt;')
    const y = !hideTitle && Number(meta.year) > 0 ? `<span class="poster-year">${Number(meta.year)}</span>` : ''
    const title = hideTitle ? '' : `<span class="poster-title">${safe}</span>`
    const g = String(meta.genre || '').toLowerCase()
    let bg = 'linear-gradient(145deg,#0b1220 0%,#1e3a8a 46%,#312e81 100%)'
    if (/(horror|survival)/.test(g)) bg = 'linear-gradient(145deg,#1a0b18 0%,#4a1740 45%,#0d2a2e 100%)'
    else if (/(racing|sports)/.test(g)) bg = 'linear-gradient(145deg,#07141f 0%,#0b3d5c 50%,#0f6b5c 100%)'
    else if (/(shooter|fps|esports|moba)/.test(g)) bg = 'linear-gradient(145deg,#0a1024 0%,#1a2a6c 48%,#0e4d7a 100%)'
    else if (/(rpg|adventure|aaa|action|open)/.test(g)) bg = 'linear-gradient(145deg,#12081f 0%,#3b1d6e 50%,#0f3d4a 100%)'
    else if (/(indie|classic|sim)/.test(g)) bg = 'linear-gradient(145deg,#071a14 0%,#145a45 50%,#1a3a5c 100%)'
    wrap.innerHTML = `<div class="${meta.cls || 'game-poster'} fallback glass-cover" style="background:${bg}" aria-hidden="true">${title}${y}</div>`
    const node = wrap.firstElementChild
    if (node && img.parentNode) img.replaceWith(node)
  } catch (_) {}
}

let confirmResolver = null
function glassConfirm({ title, body, confirmLabel, danger = true } = {}) {
  return new Promise((resolve) => {
    confirmResolver = resolve
    const modal = $('confirm-modal')
    const titleEl = $('confirm-title')
    const bodyEl = $('confirm-body')
    const ok = $('confirm-ok')
    const cancel = $('confirm-cancel')
    if (titleEl) titleEl.textContent = title || t('confirmTitle')
    if (bodyEl) bodyEl.textContent = body || ''
    if (ok) {
      ok.textContent = confirmLabel || t('confirmDeleteForever')
      ok.classList.toggle('danger', !!danger)
      ok.classList.toggle('primary', !danger)
    }
    if (cancel) cancel.textContent = t('commonCancel')
    modal?.classList.remove('hidden')
  })
}
function closeConfirmModal(result) {
  $('confirm-modal')?.classList.add('hidden')
  const r = confirmResolver
  confirmResolver = null
  if (r) r(!!result)
}

function openDeleteProgress(label) {
  const modal = $('delete-progress-modal')
  const text = $('delete-progress-text')
  const bar = $('delete-progress-bar')
  const done = $('delete-progress-done')
  if (text) text.textContent = t('deleteProgressWorking', { name: label || '' })
  if (bar) bar.style.width = '8%'
  done?.classList.add('hidden')
  modal?.classList.remove('hidden')
}
function updateDeleteProgress(ev) {
  const text = $('delete-progress-text')
  const bar = $('delete-progress-bar')
  const done = $('delete-progress-done')
  const pct = Math.max(0, Math.min(100, Number(ev?.percent) || 0))
  if (bar) bar.style.width = `${pct}%`
  if (ev?.phase === 'done') {
    const gb = formatBytesSmart(ev?.freedBytes || 0)
    if (text) text.textContent = t('deleteProgressDone', { gb })
    done?.classList.remove('hidden')
    setTimeout(() => {
      $('delete-progress-modal')?.classList.add('hidden')
    }, 1500)
    return
  }
  if (ev?.phase === 'error') {
    if (text) text.textContent = ev?.message || t('deleteProgressFail')
    return
  }
  if (text) {
    const name = ev?.label || ''
    if (ev?.phase === 'watching') text.textContent = t('diskCleanWatchingLauncher', { name })
    else if (ev?.phase === 'uninstall') text.textContent = t('deleteProgressUninstall', { name })
    else text.textContent = t('deleteProgressWorking', { name })
  }
}

const GITHUB_URL = 'https://github.com/robbled17/pulsecore'

function openGitHub() {
  try { BrowserOpenURL(GITHUB_URL) } catch (_) {
    try { window.open(GITHUB_URL, '_blank') } catch (__) {}
  }
}
import {
  t as tKey,
  tMsg,
  setLang as i18nSetLang,
  listSelectableLangs,
  setI18nDev,
  setPseudoLoc,
  COMPLETE_LANGS,
  i18n,
} from './i18n.js'
import {
  buildDiskCardsHTML,
  buildDiskPillsHTML,
  driveLetter as diskDriveLetter,
} from './disks/renderDisks.js'

const MIT_LICENSE = `PulseCore v1.1.0

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

let lang = (() => {
  const saved = localStorage.getItem('zp_lang') || 'ru'
  return COMPLETE_LANGS.includes(saved) ? saved : 'en'
})()
i18nSetLang(lang)
try {
  const dev = localStorage.getItem('zp_i18n_dev') === '1' || import.meta.env?.DEV
  setI18nDev(!!dev)
  setPseudoLoc(localStorage.getItem('zp_i18n_pseudo') === '1')
} catch (_) { setI18nDev(false) }
let frozen = new Set()
let activeMetric = 'cpu'
let isPolling = false
let currentInterval = 1000
let pollTimer = 0
let weeklyLoaded = false
let selectedDrive = 'C'
let hudEnabled = false
let gameOverlayOn = false
let latestDisks = []
let lastMetrics = null
let gameCatalog = []
let currentSearchQuery = ''
let gameSearchToken = 0
let gameCatalogFull = []
let selectedGameId = null
let gameSearchFocused = false
let gameGenre = 'all'
let gameSort = 'popular'
let gameCompat60 = false // legacy; replaced by gameReqFit
let gameReqFit = 'all'
const fpsCache = Object.create(null)
const periphCollapsed = new Set(JSON.parse(localStorage.getItem('zp_periph_collapsed') || JSON.stringify(['input', 'audio', 'storage', 'display'])))
const cachedDiskReport = Object.create(null)

/** Session-persistent expand state - survives poll redraws and tab switches. */
const uiExpand = {
  problems: new Set(), // problem id
  tiles: new Set(),    // tile-cpu | tile-gpu | …
  disks: new Set(),    // C | D | E
}

function chevronSVG() {
  return `<svg class="btn-chevron" viewBox="0 0 16 16" width="14" height="14" aria-hidden="true" focusable="false"><path d="M4 6l4 4 4-4" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>`
}
const diskHistory = JSON.parse(localStorage.getItem('pulsecore_disk_history') || '{}')
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

function t(key, params) {
  i18nSetLang(lang)
  return tKey(key, params)
}

/** Translate backend loc.Msg `{code,params}` with optional legacy Ru/En. */
function msgText(msg, legacyRu, legacyEn) {
  if (msg && (msg.code || msg.Code)) {
    return tMsg({ code: msg.code || msg.Code, params: msg.params || msg.Params || {} }, legacyRu, legacyEn)
  }
  return tMsg(null, legacyRu, legacyEn)
}

function silentModeKey() {
  return document.body.classList.contains('device-desktop') ? 'modeSilentDesktop' : 'modeSilent'
}

let powerSwitchPrev = 'balanced'
let powerSwitchBusy = false

function syncPowerRadiogroup(mode) {
  const m = mode || 'balanced'
  powerSwitchPrev = m
  document.querySelectorAll('.power-card').forEach((btn) => {
    const on = btn.dataset.mode === m
    btn.classList.toggle('selected', on)
    btn.setAttribute('aria-checked', on ? 'true' : 'false')
    btn.classList.remove('switching')
    btn.disabled = false
  })
  const st = $('power-switch-status')
  if (st) {
    st.textContent = ''
    st.classList.add('hidden')
  }
}

function setupPowerRadiogroup() {
  const group = $('power-radiogroup')
  if (!group || group.dataset.bound) return
  group.dataset.bound = '1'
  const cards = [...group.querySelectorAll('.power-card')]
  const apply = async (btn) => {
    if (powerSwitchBusy || !btn) return
    const mode = btn.dataset.mode
    if (!mode || mode === powerSwitchPrev) return
    const prev = powerSwitchPrev
    powerSwitchBusy = true
    cards.forEach((c) => { c.disabled = true })
    btn.classList.add('switching')
    const st = $('power-switch-status')
    if (st) {
      st.textContent = t('powerSwitching')
      st.classList.remove('hidden')
    }
    try {
      await SetPowerProfile(mode)
      syncPowerRadiogroup(mode)
      showToast(t('modeApplied'), true)
    } catch (err) {
      syncPowerRadiogroup(prev)
      if (st) {
        st.textContent = t('powerSwitchFail')
        st.classList.remove('hidden')
      }
      showToast(String(err || t('powerSwitchFail')), false)
    } finally {
      powerSwitchBusy = false
      cards.forEach((c) => { c.disabled = false })
    }
  }
  cards.forEach((btn) => {
    btn.addEventListener('click', () => { apply(btn).catch(() => {}) })
  })
  group.addEventListener('keydown', (ev) => {
    const idx = cards.indexOf(document.activeElement)
    if (idx < 0) return
    let next = -1
    if (ev.key === 'ArrowRight' || ev.key === 'ArrowDown') next = (idx + 1) % cards.length
    if (ev.key === 'ArrowLeft' || ev.key === 'ArrowUp') next = (idx - 1 + cards.length) % cards.length
    if (ev.key === ' ' || ev.key === 'Enter') {
      ev.preventDefault()
      apply(cards[idx]).catch(() => {})
      return
    }
    if (next >= 0) {
      ev.preventDefault()
      cards[next].focus()
    }
  })
}

function syncDeviceKind(hasBattery) {
  document.body.classList.toggle('device-desktop', !hasBattery)
  const silent = document.querySelector(
    '.power-card[data-mode="silent"] .power-card-title, .home-modes .btn.mode[data-mode="silent"] [data-i18n], .btn.mode[data-mode="silent"] [data-i18n]'
  )
  if (silent) {
    const key = silentModeKey()
    silent.setAttribute('data-i18n', key)
    silent.textContent = t(key)
  }
  const batCard = document.querySelector('.metric-card[data-metric="battery"]')
  if (batCard) batCard.classList.toggle('hidden', !hasBattery)
  const sleepPanel = $('sleep-panel')
  if (sleepPanel) sleepPanel.classList.toggle('hidden', !hasBattery)
  if (!hasBattery && activeMetric === 'battery') {
    activeMetric = 'cpu'
    document.querySelectorAll('.chart-tab').forEach((el) => {
      el.classList.toggle('active', el.dataset.metric === 'cpu')
    })
    updateChartTitle()
  }
}

function restoreTitlebarDrag() {
  const tb = $('titlebar')
  if (!tb) return
  tb.style.setProperty('--wails-draggable', 'drag')
  tb.style.setProperty('-webkit-app-region', 'drag')
  tb.classList.remove('hidden')
}

async function syncMaximisedChrome() {
  let max = false
  try {
    max = !!(await WindowMaximisedState())
  } catch (_) {
    try { max = !!(await WindowIsMaximised()) } catch (__) { max = false }
  }
  document.documentElement.classList.toggle('window-maximized', max)
}

function wireTitlebarNativeDrag() {
  const tb = $('titlebar')
  if (!tb || tb.dataset.nativeDragWired) return
  tb.dataset.nativeDragWired = '1'
  let lastDown = 0
  tb.addEventListener('mousedown', async (ev) => {
    if (ev.button !== 0) return
    if (ev.target.closest('.titlebar-actions, .win-btn, .theme-opt, .theme-switch, #titlebar-brand, button, a, input, select')) {
      return
    }
    const now = Date.now()
    if (now - lastDown < 350) {
      lastDown = 0
      try { await TitlebarToggleMaximise() } catch (_) {}
      syncMaximisedChrome().catch(() => {})
      return
    }
    lastDown = now
    try { await TitlebarDragBegin() } catch (_) {}
    setTimeout(() => { syncMaximisedChrome().catch(() => {}) }, 200)
  })
}

function applyI18n() {
  document.querySelectorAll('[data-i18n]').forEach((el) => {
    el.textContent = t(el.getAttribute('data-i18n'))
  })
  syncDeviceKind(!document.body.classList.contains('device-desktop'))
  document.querySelectorAll('[data-i18n-tip]').forEach((el) => {
    el.setAttribute('data-tip', t(el.getAttribute('data-i18n-tip')))
  })
  document.querySelectorAll('[data-i18n-placeholder]').forEach((el) => {
    el.setAttribute('placeholder', t(el.getAttribute('data-i18n-placeholder')))
  })
  document.querySelectorAll('[data-i18n-title]').forEach((el) => {
    el.setAttribute('title', t(el.getAttribute('data-i18n-title')))
  })
  syncCustomDropdownLabels()
  updateChartTitle()
  syncLangSelects()
  syncOverlayPill(gameOverlayOn)
  renderGameGrid()
  if (selectedGameId) predictSelectedGame()
}

function syncLangSelects() {
  const el = $('lang-select-settings')
  if (!el) return
  const opts = listSelectableLangs()
  const cur = el.value
  el.innerHTML = opts.map((o) => `<option value="${o.id}">${o.label}</option>`).join('')
  el.value = opts.some((o) => o.id === lang) ? lang : (opts[0]?.id || 'en')
  if (cur && cur !== el.value && !COMPLETE_LANGS.includes(cur)) {
    /* incomplete lang removed from list */
  }
}

/** Prefer coded message; fall back to legacy Ru/En fields. */
function resultMsg(res, fallbackKey) {
  if (!res) return fallbackKey ? t(fallbackKey) : ''
  if (res.message && (res.message.code || res.message.Code)) {
    return msgText(res.message)
  }
  const text = lang === 'en'
    ? (res.messageEn || res.messageRu || '')
    : (res.messageRu || res.messageEn || '')
  if (text) return text
  return fallbackKey ? t(fallbackKey) : ''
}

function isLightTheme() {
  return document.documentElement.getAttribute('data-theme') === 'light'
}

function themeToken(name, fallback = '') {
  const v = getComputedStyle(document.documentElement).getPropertyValue(name).trim()
  return v || fallback
}

function chartGridColor() {
  return themeToken('--chart-grid', isLightTheme() ? '#64748b' : '#334155')
}

function chartLabelColor() {
  return themeToken('--chart-label', isLightTheme() ? '#475569' : '#94a3b8')
}

function joinSummary(parts) {
  return parts.filter((p) => p != null && String(p).trim() !== '' && String(p) !== '-').join(' · ')
}

let lastAdvancedState = null
let advPollTimer = null
let advFastPollUntil = 0

function advancedPhase(st) {
  const p = String(st?.phase || '')
  if (p === 'off' || p === 'starting' || p === 'ok' || p === 'error') return p
  if (!st?.enabled) return 'off'
  if (st?.status === 'starting') return 'starting'
  if (st?.status === 'ok') return 'ok'
  if (st?.enabled) return 'error'
  return 'off'
}

function advancedStatusText(st) {
  const phase = advancedPhase(st)
  if (phase === 'ok') {
    if (st?.pawnIOPresent || st?.helperRunning) return t('advStatusOkDriver')
    return t('advStatusOk')
  }
  if (phase === 'starting') return t('advStatusStarting')
  if (phase === 'error') {
    const detail = lang === 'en' ? (st?.statusEn || '') : (st?.statusRu || '')
    return [t('advStatusError'), detail].filter(Boolean).join(' - ')
  }
  return t('advStatusOff')
}

function advancedActionText(st) {
  const phase = advancedPhase(st)
  if (phase === 'starting') return t('advStarting')
  if (phase === 'error') return lang === 'en' ? (st?.actionEn || '') : (st?.actionRu || '')
  return ''
}

function updateAdvancedDiagPanel(st) {
  const phase = advancedPhase(st)
  const set = (id, v) => { const el = $(id); if (el) el.textContent = v }
  const phaseKey = phase === 'ok' ? 'advPhaseOk' : phase === 'starting' ? 'advPhaseStarting' : phase === 'error' ? 'advPhaseError' : 'advPhaseOff'
  set('adv-diag-phase', t(phaseKey))
  set('adv-diag-task', st?.taskRunning ? t('advTaskOn') : t('advTaskOff'))
  set('adv-diag-proc', st?.processRunning
    ? `${t('commonYes')} / ${st.processPid || '-'}`
    : t('commonNo'))
  set('adv-diag-pipe', st?.pipePresent ? t('advPipeOn') : t('advPipeOff'))
  set('adv-diag-proto', st?.protocolVersion ? `v${st.protocolVersion}${st.helperVersion ? ` · ${st.helperVersion}` : ''}` : '-')
  set('adv-diag-last', st?.lastHelperReplyAt || '-')
  set('adv-diag-err', st?.lastError || '-')
  const restart = $('btn-adv-restart')
  if (restart) restart.classList.toggle('hidden', phase !== 'error')
}

function stopAdvancedFastPoll() {
  if (advPollTimer) {
    clearInterval(advPollTimer)
    advPollTimer = null
  }
  advFastPollUntil = 0
}

function startAdvancedFastPoll() {
  advFastPollUntil = Date.now() + 20000
  if (advPollTimer) return
  advPollTimer = setInterval(async () => {
    if (Date.now() > advFastPollUntil) {
      stopAdvancedFastPoll()
      return
    }
    const st = await refreshAdvancedModeUI()
    if (advancedPhase(st) === 'ok') {
      stopAdvancedFastPoll()
      try { await refreshAllDiskSmart(true) } catch (_) {}
    }
  }, 2000)
}

let appIsElevated = false

async function syncElevatedUI() {
  try {
    appIsElevated = !!(await IsElevated())
  } catch (_) {
    appIsElevated = false
  }
  // Hide / disable any “restart as admin” controls when already elevated (no UAC loop).
  document.querySelectorAll('[data-relaunch-admin], #btn-relaunch-admin').forEach((el) => {
    el.classList.toggle('hidden', appIsElevated)
    if ('disabled' in el) el.disabled = appIsElevated
  })
}

async function relaunchAsAdminSafe() {
  if (appIsElevated) return
  try {
    const elevated = !!(await IsElevated())
    appIsElevated = elevated
    if (elevated) {
      await syncElevatedUI()
      return
    }
    await RestartAsAdmin()
  } catch (err) {
    showToast(String(err), false, { holdMs: 5000 })
  }
}

async function refreshAdvancedModeUI() {
  let st = null
  try { st = await GetAdvancedModeState() } catch (_) { st = null }
  lastAdvancedState = st
  const toggle = $('opt-advanced-mode')
  if (toggle) toggle.checked = !!st?.enabled
  const status = $('adv-status')
  if (status) {
    status.textContent = advancedStatusText(st)
    const phase = advancedPhase(st)
    status.classList.toggle('is-ok', phase === 'ok')
    status.classList.toggle('is-warn', phase === 'starting')
    status.classList.toggle('is-bad', phase === 'error')
  }
  const action = $('adv-action')
  if (action) action.textContent = advancedActionText(st)
  updateAdvancedDiagPanel(st)
  updateAdvancedBanner(lastMetrics, st)
  await syncElevatedUI()
  return st
}

function updateAdvancedBanner(m, st) {
  const banner = $('advanced-mode-banner')
  if (!banner) return
  const phase = advancedPhase(st)
  const text = $('advanced-mode-banner-text')
  const btnEnable = $('btn-advanced-enable')
  const btnRestart = $('btn-advanced-restart')
  const btnDiag = $('btn-advanced-diag')
  // Banner «Включите» ONLY when off. Starting/error show their own copy - never ask to enable again.
  let show = false
  if (phase === 'off') {
    show = true
    if (text) text.textContent = t('advBannerBody')
    if (btnEnable) { btnEnable.classList.remove('hidden'); btnEnable.textContent = t('advEnable') }
    if (btnRestart) btnRestart.classList.add('hidden')
    if (btnDiag) btnDiag.classList.add('hidden')
  } else if (phase === 'starting') {
    show = true
    if (text) text.textContent = t('advStarting')
    if (btnEnable) btnEnable.classList.add('hidden')
    if (btnRestart) btnRestart.classList.add('hidden')
    if (btnDiag) btnDiag.classList.add('hidden')
  } else if (phase === 'error') {
    show = true
    const action = lang === 'en' ? (st?.actionEn || t('advStatusError')) : (st?.actionRu || t('advStatusError'))
    if (text) text.textContent = action
    if (btnEnable) btnEnable.classList.add('hidden')
    if (btnRestart) { btnRestart.classList.remove('hidden'); btnRestart.textContent = t('advRestart') }
    if (btnDiag) { btnDiag.classList.remove('hidden'); btnDiag.textContent = t('advCopyDiag') }
  } else {
    show = false
  }
  banner.classList.toggle('hidden', !show)
}

async function enableAdvancedModeFlow() {
  const status = $('adv-status')
  if (status) status.textContent = t('advStatusStarting')
  const action = $('adv-action')
  if (action) action.textContent = t('advStarting')
  updateAdvancedBanner(lastMetrics, { enabled: true, phase: 'starting', status: 'starting' })
  showToast(t('advStarting'), true, { holdMs: 2500 })
  startAdvancedFastPoll()
  let res
  try {
    res = await EnableAdvancedMode(false)
  } catch (err) {
    showToast(String(err), false, { holdMs: 7000 })
    await refreshAdvancedModeUI()
    return
  }
  if (res?.needConsentPawnIO) {
    const consent = await glassConfirm({
      title: t('advTitle'),
      body: t('advPawnIOConsent'),
      confirmLabel: t('commonYes'),
      danger: false,
    })
    if (!consent) {
      stopAdvancedFastPoll()
      await refreshAdvancedModeUI()
      return
    }
    if (status) status.textContent = t('advStatusStarting')
    try {
      res = await EnableAdvancedMode(true)
    } catch (err) {
      showToast(String(err), false, { holdMs: 7000 })
      await refreshAdvancedModeUI()
      return
    }
  }
  showToast(resultMsg(res), !!res?.ok, { holdMs: 7000 })
  await refreshAdvancedModeUI()
  startAdvancedFastPoll()
  if (res?.ok) {
    try { await refreshAllDiskSmart(true) } catch (_) {}
  }
}

async function restartHelperFlow() {
  showToast(t('advStarting'), true, { holdMs: 2000 })
  startAdvancedFastPoll()
  try {
    const res = await RestartHelper()
    showToast(resultMsg(res), !!res?.ok, { holdMs: 6000 })
  } catch (err) {
    showToast(String(err), false, { holdMs: 6000 })
  }
  await refreshAdvancedModeUI()
  try { await refreshAllDiskSmart(true) } catch (_) {}
}

async function copyAdvancedDiagnostics() {
  try {
    const text = await GetHelperDiagnostics()
    await navigator.clipboard.writeText(String(text || ''))
    showToast(t('advDiagCopied'), true, { holdMs: 3500 })
  } catch (err) {
    showToast(String(err), false, { holdMs: 5000 })
  }
}

async function setAdvancedMode(on) {
  if (on) {
    await enableAdvancedModeFlow()
    const st = await refreshAdvancedModeUI()
    const toggle = $('opt-advanced-mode')
    if (toggle) toggle.checked = !!st?.enabled
    return
  }
  stopAdvancedFastPoll()
  try {
    const res = await DisableAdvancedMode()
    showToast(resultMsg(res), !!res?.ok, { holdMs: 5000 })
  } catch (err) {
    showToast(String(err), false, { holdMs: 6000 })
  }
  await refreshAdvancedModeUI()
}

function diskHealthLabel(raw) {
  const s = String(raw || '').toLowerCase()
  if (!s) return t('diskStateUnknown')
  if (/неизвест|unknown/.test(s)) return t('diskStateUnknown')
  if (/отлич|хорош|good|healthy|ok/.test(s)) return t('diskStateGood')
  if (/вниман|warn|caution|degrad/.test(s)) return t('diskStateWarn')
  if (/плох|bad|fail|crit|error|ошиб/.test(s)) return t('diskStateBad')
  return t('diskStateUnknown')
}

function diskStatusClass(level) {
  switch (String(level || '')) {
    case 'good': return 'st-good'
    case 'caution': return 'st-caution'
    case 'bad': return 'st-bad'
    default: return 'st-unknown'
  }
}

function diskStatusText(r) {
  const phase = advancedPhase(lastAdvancedState)
  if (phase === 'starting') return t('advStarting')
  if (phase === 'error') return lang === 'en' ? (lastAdvancedState?.actionEn || t('advStatusError')) : (lastAdvancedState?.actionRu || t('advStatusError'))
  if (!r) return phase === 'ok' ? t('diskStateUnknown') : t('advDiskNeed')
  if (r.needsAdmin && phase === 'off') return t('advDiskNeed')
  if (r.statusCode && ['diskStateGood', 'diskStateWarn', 'diskStateBad', 'diskStateUnknown'].includes(r.statusCode)) {
    return t(r.statusCode)
  }
  if (r.needsAdmin && phase !== 'ok') {
    return lang === 'en' ? (lastAdvancedState?.actionEn || t('advStatusError')) : (lastAdvancedState?.actionRu || t('advStatusError'))
  }
  const word = lang === 'en' ? (r.statusWordEn || '') : (r.statusWordRu || '')
  if (/расширенн|advanced mode|включите/i.test(word) && phase !== 'off') {
    return phase === 'ok' ? t('diskStateUnknown') : (lang === 'en' ? (lastAdvancedState?.actionEn || word) : (lastAdvancedState?.actionRu || word))
  }
  if (!word || /неизвест|unknown/i.test(word)) {
    return phase === 'ok' ? t('diskStateUnknown') : t('advDiskNeed')
  }
  return word
}

function formatCapacityGB(gb) {
  if (!(gb > 0)) return ''
  if (gb >= 900) return `${fmt(gb / 1024, 1)} TB`
  return `${fmt(gb, 0)} GB`
}

const THEME_KEY = 'pulsecore_theme'
let themeMedia = null

function resolvedTheme(mode) {
  if (mode === 'system') {
    return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
  }
  return mode === 'light' ? 'light' : 'dark'
}

function applyTheme(mode) {
  const choice = mode || localStorage.getItem(THEME_KEY) || 'dark'
  localStorage.setItem(THEME_KEY, choice)
  document.documentElement.setAttribute('data-theme', resolvedTheme(choice))
  document.querySelectorAll('.theme-opt').forEach((btn) => {
    btn.classList.toggle('active', btn.dataset.theme === choice)
  })
  if (themeMedia) {
    themeMedia.onchange = null
    themeMedia = null
  }
  if (choice === 'system') {
    themeMedia = window.matchMedia('(prefers-color-scheme: dark)')
    themeMedia.onchange = () => {
      if ((localStorage.getItem(THEME_KEY) || 'dark') === 'system') {
        document.documentElement.setAttribute('data-theme', resolvedTheme('system'))
        drawTrendFrame()
      }
    }
  }
  drawTrendFrame()
}

function readBehavior() {
  return {
    autoStart: !!$('opt-autostart')?.checked,
    startMinimized: !!$('opt-start-min')?.checked,
    closeToTray: !!$('opt-close-tray')?.checked,
    showTempOnTray: !!$('opt-tray-temp')?.checked,
    tempAlerts: !!$('opt-temp-alert')?.checked,
    ecoExitPolicy: $('opt-eco-exit')?.value || 'ask',
    conflictWatcherEnabled: $('toggle-conflict-watcher') ? !!$('toggle-conflict-watcher').checked : ($('opt-conflict-watcher') ? !!$('opt-conflict-watcher').checked : true),
  }
}

async function saveBehavior() {
  try { await SetBehaviorSettings(readBehavior()) } catch (err) { console.error(err) }
}

async function loadBehavior() {
  try {
    const p = await GetBehaviorSettings()
    if (!p) return
    if ($('opt-autostart')) $('opt-autostart').checked = !!p.autoStart
    if ($('opt-start-min')) $('opt-start-min').checked = !!p.startMinimized
    if ($('opt-close-tray')) $('opt-close-tray').checked = p.closeToTray !== false
    if ($('opt-tray-temp')) $('opt-tray-temp').checked = p.showTempOnTray !== false
    if ($('opt-temp-alert')) $('opt-temp-alert').checked = p.tempAlerts !== false
    if ($('opt-eco-exit')) $('opt-eco-exit').value = p.ecoExitPolicy || 'ask'
    const watcherVal = p.conflictWatcherEnabled !== false
    if ($('toggle-conflict-watcher')) $('toggle-conflict-watcher').checked = watcherVal
    if ($('opt-conflict-watcher')) $('opt-conflict-watcher').checked = watcherVal
  } catch (_) {}
}

function procCountLabel(n) {
  const count = Math.max(0, Number(n) || 0)
  if (lang === 'en') {
    return `${count} ${count === 1 ? t('procCountOne') : t('procCountMany')}`
  }
  const mod10 = count % 10
  const mod100 = count % 100
  let key = 'procCountMany'
  if (mod10 === 1 && mod100 !== 11) key = 'procCountOne'
  else if (mod10 >= 2 && mod10 <= 4 && !(mod100 >= 12 && mod100 <= 14)) key = 'procCountFew'
  return `${count} ${t(key)}`
}

async function changeLanguage(next) {
  const want = String(next || 'en').toLowerCase()
  lang = COMPLETE_LANGS.includes(want) ? want : 'en'
  i18nSetLang(lang)
  localStorage.setItem('zp_lang', lang)
  try { await SetLanguage(lang) } catch (_) {}
  applyI18n()
  refresh()
  refreshWeekly()
  refreshPeripherals()
}

function setActiveTab(name) {
  let target = String(name || 'home')
  if (target === 'gamepulse') target = 'games'
  document.querySelectorAll('.nav-item').forEach((btn) => {
    if (btn.classList.contains('nav-hud')) return
    btn.classList.toggle('active', btn.dataset.tab === target)
  })
  document.querySelectorAll('.tab, .tab-pane').forEach((tab) => {
    const on = tab.id === `tab-${target}`
    tab.classList.toggle('active', on)
  })
  // Collectors: home needs overview+hardware metrics
  const backendTab = target === 'home' ? 'overview' : target
  if (target === 'home') {
    renderHome().catch(() => {})
    renderHardware(lastMetrics || {})
    if (!weeklyLoaded) {
      weeklyLoaded = true
      refreshWeekly()
    }
    renderSummary().catch(() => {})
  }
  if (target === 'games') {
    // Catalog + filters always visible; only sessions/compare hide when empty.
    renderGamesTab().catch(() => {})
    try { renderGameGrid() } catch (_) {}
  }
  if (target === 'startup') {
    renderStartupList().catch(() => {})
  }
  if (target === 'settings') {
    /* startup moved out */
  }
  SetActiveTab(backendTab).catch(() => {})
}

async function renderHome() {
  const scoreEl = $('home-score')
  const statusEl = $('home-status')
  const checkedEl = $('home-checked')
  const findings = $('home-findings')
  const ringFg = $('home-ring-fg')
  if (!scoreEl || !findings) return
  const scrollY = findings.scrollTop
  const focusId = document.activeElement?.id || document.activeElement?.getAttribute?.('data-home-action') || ''
  let st
  try { st = await GetHomeState() } catch (_) { st = null }
  if (!st) return
  const score = Number(st.score) || 0
  scoreEl.textContent = String(score)
  if (statusEl) statusEl.textContent = msgText(st.status, st.statusRu, st.statusEn)
  if (checkedEl) checkedEl.textContent = t('homeCheckedAgo', { n: st.checkedAgoSec || 1 })
  if (ringFg) {
    const circ = 2 * Math.PI * 52
    const offset = circ * (1 - Math.min(100, Math.max(0, score)) / 100)
    ringFg.style.strokeDasharray = String(circ)
    ringFg.style.strokeDashoffset = String(offset)
    ringFg.style.stroke = score >= 85
      ? themeToken('--ok', '#4ade80')
      : score >= 60
        ? themeToken('--warn', '#fbbf24')
        : themeToken('--danger', '#fb7185')
  }
  const probs = Array.isArray(st.problems) ? st.problems : []
  const showGains = score < 95
  const scoreIfAll = Number(st.scoreIfAllFixed) || score
  const smartBtn = $('btn-smart-fix')
  if (smartBtn) {
    const gain = Math.max(0, scoreIfAll - score)
    if (probs.length && showGains && gain > 0) {
      smartBtn.classList.remove('hidden')
      smartBtn.textContent = t('smartFixButton', { n: gain })
    } else {
      smartBtn.classList.add('hidden')
    }
  }
  if (!probs.length) {
    const ecoN = Number(st.ecoLimitedCount) || 0
    const power = String(st.powerMode || '').toLowerCase()
    const powerLabel = power === 'turbo' ? t('modeTurbo')
      : power === 'silent' ? t('modeSilent')
        : t('modeBalanced')
    const ecoLine = ecoN > 0
      ? t('homeEcoActive', { n: ecoN })
      : t('homeEcoIdle')
    const prepInFlight = !!findings.querySelector('[data-home-action="prepare_game"].is-loading, .home-prepare-progress')
    if (!prepInFlight) {
      let prep
      if (st.prepareActive) {
        prep = `
          <div class="home-prepare-result ok" role="status">
            <div class="home-prepare-result-title">${t('homePrepareReady')}</div>
            <div class="meta">${t('homePrepareFreed', { mb: fmt(st.prepareRAMMB, 0) })}</div>
            <div class="meta home-prepare-hint">${t('homeRevertPrepareHint')}</div>
            <button type="button" class="btn soft home-prepare-revert" data-home-action="revert_prepare">${t('homeRevertPrepare')}</button>
          </div>`
      } else {
        prep = `
          <button type="button" class="btn primary home-prepare-btn" data-home-action="prepare_game">
            <span class="home-prepare-bolt" aria-hidden="true">⚡</span>
            <span class="home-prepare-label">${t('homePrepare')}</span>
          </button>
          <div class="meta home-prepare-hint">${t('homePrepareHint')}</div>`
      }
      findings.innerHTML = `<div class="home-problem home-all-good st-good">
        <div class="home-all-good-head">
          <div class="problem-title">${t('homeAllGood')}</div>
          <div class="home-all-good-sub meta">${escapeHtml(msgText(st.status, st.statusRu, st.statusEn) || t('homeStatusExcellent'))}</div>
        </div>
        <div class="home-eco-status" data-eco-count="${ecoN}">
          <span class="home-eco-pill">${escapeHtml(ecoLine)}</span>
          <span class="home-eco-pill soft">${escapeHtml(powerLabel)}</span>
        </div>
        <div class="problem-actions home-prepare-actions">${prep}</div>
      </div>`
    } else {
      // Update Eco/power pills only — keep indeterminate prepare progress intact.
      const ecoHost = findings.querySelector('.home-eco-status')
      if (ecoHost) {
        ecoHost.dataset.ecoCount = String(ecoN)
        ecoHost.innerHTML = `<span class="home-eco-pill">${escapeHtml(ecoLine)}</span><span class="home-eco-pill soft">${escapeHtml(powerLabel)}</span>`
      }
    }
  } else {
    const head = showGains && scoreIfAll > score
      ? `<div class="home-score-if-all meta">${t('homeScoreIfAll', { n: scoreIfAll })}</div>`
      : ''
    findings.innerHTML = head + probs.map((p) => {
      const pid = String(p.id || '')
      const title = msgText(p.title, p.titleRu, p.titleEn)
      const why = msgText(p.why, p.whyRu, p.whyEn)
      const act = msgText(p.action, p.actionRu, p.actionEn)
      const detail = msgText(p.detail, p.detailRu, p.detailEn)
      const gain = Number(p.gainEstimate) || 0
      const open = uiExpand.problems.has(pid)
      const stClass = diskStatusClass(p.statusLevel)
      const ecoNote = p.actionId === 'eco_process' ? `<div class="meta">${t('ecoHint')}</div>` : ''
      const moreId = `home-more-${pid}`
      const moreBtn = detail
        ? `<button type="button" class="btn soft problem-more btn-details" data-problem-id="${escapeHtml(pid)}" aria-expanded="${open ? 'true' : 'false'}" aria-controls="${moreId}">
             <span>${t('homeMore')}</span>${chevronSVG()}
           </button>`
        : ''
      const moreBody = detail
        ? `<div class="problem-detail ${open ? 'open' : 'hidden'}" id="${moreId}" ${open ? '' : 'hidden'}>${escapeHtml(detail)}</div>`
        : ''
      let gainHtml = ''
      if (showGains && gain > 0) {
        const label = p.gainUpTo
          ? t('homeGainUpTo', { n: gain })
          : t('homeGain', { n: gain })
        gainHtml = `<span class="problem-gain">${label}</span>`
      }
      const dismissBtn = pid === 'disk_full'
        ? `<button type="button" class="btn soft compact" data-home-action="dismiss_problem" data-home-arg="${escapeHtml(pid)}">${t('homeDismissWarning')}</button>`
        : ''
      return `<div class="home-problem ${stClass}" data-problem-id="${escapeHtml(pid)}">
        <div class="problem-title">${escapeHtml(title)}</div>
        <div class="problem-why">${escapeHtml(why)}</div>
        ${ecoNote}
        <div class="problem-actions">
          <button type="button" class="btn primary" data-home-action="${escapeHtml(p.actionId || '')}" data-home-arg="${escapeHtml(p.actionArg || '')}">${escapeHtml(act)}</button>
          ${moreBtn}
          ${dismissBtn}
          ${gainHtml}
        </div>
        ${moreBody}
      </div>`
    }).join('')
  }
  findings.scrollTop = scrollY
  if (focusId) {
    const el = findings.querySelector(`[data-home-action="${CSS.escape(focusId)}"]`) || document.getElementById(focusId)
    try { el?.focus?.({ preventScroll: true }) } catch (_) {}
  }

  // Weekly summary only with 3+ days
  const panel = $('home-summary-panel')
  if (panel) {
    let s
    try { s = await GetWeeklySummary() } catch (_) { s = null }
    const days = Number(s?.daysObserved) || 0
    if (s && !s.insufficient && days >= 3) {
      panel.classList.remove('hidden')
    } else {
      panel.classList.add('hidden')
    }
  }
}

let reportPreviewOpen = false

async function showReportPreview() {
  const box = $('report-preview')
  if (!box) return
  let p
  try { p = await PreviewDiagnosticReport() } catch (_) { p = null }
  if (!p) {
    box.innerHTML = `<div class="empty">-</div>`
    return
  }
  const inc = (p.includes || []).map((x) => `<div class="row-main">✓ ${escapeHtml(x)}</div>`).join('')
  const exc = (p.excludes || []).map((x) => `<div class="row-main">✗ ${escapeHtml(x)}</div>`).join('')
  box.innerHTML = `
    <div class="panel-title sub">${t('reportIncludes')}</div>${inc}
    <div class="panel-title sub" style="margin-top:8px">${t('reportExcludes')}</div>${exc}`
}

async function toggleReportPreview() {
  const box = $('report-preview')
  const btn = $('btn-about-report-toggle')
  if (!box) return
  reportPreviewOpen = !reportPreviewOpen
  box.classList.toggle('hidden', !reportPreviewOpen)
  box.hidden = !reportPreviewOpen
  btn?.setAttribute('aria-expanded', reportPreviewOpen ? 'true' : 'false')
  btn?.classList.toggle('open', reportPreviewOpen)
  if (reportPreviewOpen) await showReportPreview()
}

async function copySupportReport() {
  const format = $('report-format')?.value || 'txt'
  let res
  try { res = await ExportDiagnosticReport(format) } catch (err) {
    showToast(String(err), false, { holdMs: 5000 })
    return
  }
  if (!res?.ok || !res.content) {
    showToast(resultMsg(res, 'reportFail'), false, { holdMs: 5000 })
    return
  }
  try {
    await navigator.clipboard.writeText(String(res.content))
    showToast(t('reportCopied'), true, { holdMs: 4000 })
  } catch (_) {
    showToast(t('reportFail'), false, { holdMs: 5000 })
  }
}

async function exportReport(format) {
  let res
  try { res = await ExportDiagnosticReport(format) } catch (err) {
    showToast(String(err), false, { holdMs: 5000 })
    return
  }
  if (!res?.ok || !res.content) {
    showToast(resultMsg(res, 'reportFail'), false, { holdMs: 5000 })
    return
  }
  try {
    const blob = new Blob([res.content], { type: format === 'json' ? 'application/json' : 'text/plain' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = res.filename || `pulsecore-report.${format}`
    document.body.appendChild(a)
    a.click()
    a.remove()
    URL.revokeObjectURL(url)
    showToast(t('reportSaved'), true, { holdMs: 4000 })
  } catch (_) {
    showToast(t('reportFail'), false, { holdMs: 5000 })
  }
}

async function renderStartupList() {
  const box = $('startup-list')
  if (!box) return
  let items = []
  let brokenN = 0
  try { items = await GetStartupItems() } catch (_) { items = [] }
  try {
    if (typeof GetStartupBrokenCount === 'function') brokenN = await GetStartupBrokenCount()
  } catch (_) { brokenN = 0 }
  const brokenRow = brokenN > 0
    ? `<div class="incident-card startup-broken-row">
        <div class="incident-body">
          <div class="incident-title">${t('startupBroken', { n: brokenN })}</div>
        </div>
        <div class="row-actions">
          <button type="button" class="btn compact danger" id="btn-startup-broken-remove">${t('startupBrokenRemove')}</button>
        </div>
      </div>`
    : ''
  if ((!items || !items.length) && !brokenN) {
    box.innerHTML = ''
    return
  }
  const rows = (items || []).map((it) => {
    const loc = lang === 'en' ? (it.locationEn || it.location) : (it.locationRu || it.location)
    const cat = it.categoryKey ? t(it.categoryKey) : ''
    const live = it.running
      ? ` · CPU ${fmt(it.cpuPercent, 0)}% · RAM ${fmt(it.ramMB, 0)} MB`
      : ''
    const impactBits = []
    if (cat) impactBits.push(cat)
    else if (it.impactRu || it.impactEn) {
      impactBits.push(lang === 'en' ? (it.impactEn || it.impact) : (it.impactRu || it.impact))
    }
    const detail = lang === 'en' ? (it.detailEn || it.detailRu) : (it.detailRu || it.detailEn)
    const pub = it.publisher ? `${t('startupPub')}: ${escapeHtml(it.publisher)} · ` : ''
    const adviseKey = it.advise || (it.impact === 'high' || it.impact === 'medium' ? 'startupAdviseDisable' : 'startupAdviseKeep')
    const badges = [
      !it.enabled ? `<span class="badge-count">${t('startupDisabled')}</span>` : '',
      `<span class="badge-count">${escapeHtml(t(adviseKey))}</span>`,
    ].filter(Boolean).join(' ')
    let btn = ''
    if (it.canDisable) {
      const label = it.enabled ? t('startupDisable') : t('startupEnable')
      btn = `<button type="button" class="btn compact ${it.enabled ? 'danger' : ''}" data-startup-id="${escapeHtml(it.id)}" data-startup-on="${it.enabled ? '0' : '1'}" data-startup-name="${escapeHtml(it.name || '')}">${label}</button>`
    }
    const metaImpact = impactBits.length
      ? ` · ${t('startupImpact')}: ${escapeHtml(impactBits.join(' · '))}${escapeHtml(live)}`
      : (live ? ` · ${escapeHtml(live.trim().replace(/^·\s*/, ''))}` : '')
    return `
      <div class="incident-card">
        <div class="incident-body">
          <div class="incident-title">${escapeHtml(it.name || '-')} ${badges}</div>
          ${detail ? `<div class="incident-explain">${escapeHtml(detail)}</div>` : ''}
          <div class="meta">${t('startupLoc')}: ${escapeHtml(loc)}${metaImpact} · ${pub}${escapeHtml(it.path || it.command || '')}</div>
        </div>
        <div class="row-actions">${btn}</div>
      </div>`
  }).join('')
  box.innerHTML = brokenRow + rows
  $('btn-startup-broken-remove')?.addEventListener('click', async () => {
    try {
      if (typeof RemoveBrokenStartupEntries === 'function') await RemoveBrokenStartupEntries()
    } catch (_) { /* ignore */ }
    renderStartupList().catch(() => {})
  })
}

async function renderHealthTab() {
  const scoreEl = $('health-score')
  const sumEl = $('health-summary')
  const missEl = $('health-missing')
  const factorsEl = $('health-factors')
  const histEl = $('health-history')
  if (!scoreEl || !factorsEl) return
  let rep
  try { rep = await GetSystemHealth() } catch (_) { rep = null }
  if (!rep) {
    scoreEl.textContent = '-'
    if (sumEl) sumEl.textContent = t('healthEmpty')
    return
  }
  const score = Number(rep.score) || 0
  scoreEl.textContent = String(score)
  scoreEl.className = `health-score ${score >= 85 ? 'ok' : score >= 70 ? 'fair' : score >= 50 ? 'watch' : 'bad'}`
  if (sumEl) sumEl.textContent = lang === 'en' ? (rep.summaryEn || rep.summaryRu) : (rep.summaryRu || rep.summaryEn)
  if (missEl) missEl.textContent = rep.incomplete ? (rep.missingNote || t('healthExcluded')) : ''

  const fillList = (id, items) => {
    const el = $(id)
    if (!el) return
    if (!items || !items.length) {
      el.innerHTML = `<div class="empty">-</div>`
      return
    }
    el.innerHTML = items.map((x) => `<div class="row-main">${escapeHtml(x)}</div>`).join('')
  }
  fillList('health-ok', rep.ok)
  fillList('health-watch', rep.watch)
  fillList('health-action', rep.action)

  const factors = Array.isArray(rep.factors) ? rep.factors : []
  factorsEl.innerHTML = factors.map((f) => {
    const label = lang === 'en' ? (f.labelEn || f.labelRu) : (f.labelRu || f.labelEn)
    const detail = lang === 'en' ? (f.detailEn || f.detailRu) : (f.detailRu || f.detailEn)
    const avail = f.available
    const sc = avail ? Math.round(Number(f.score) || 0) : '-'
    const st = f.status || 'unknown'
    return `
      <div class="incident-card">
        <div class="incident-body">
          <div class="incident-title">${escapeHtml(label)} · ${sc}${avail ? '/100' : ''} · ${escapeHtml(st)}</div>
          <div class="incident-explain">${escapeHtml(detail || '')}</div>
          <div class="meta">${t('healthWeight')}: ${fmt(f.weight, 1)}${avail ? '' : ` · ${t('healthExcluded')}`}</div>
        </div>
      </div>`
  }).join('') || `<div class="empty">${t('healthEmpty')}</div>`

  if (histEl) {
    const hist = Array.isArray(rep.history) ? rep.history : []
    if (!hist.length) {
      histEl.innerHTML = `<div class="empty">${t('healthEmpty')}</div>`
    } else {
      histEl.innerHTML = hist.slice(-24).map((p) => `
        <div class="health-hist-item"><span>${escapeHtml(p.at || '')}</span><b>${Number(p.score) || 0}</b></div>
      `).join('')
    }
  }
  let dedBox = $('health-deductions')
  if (!dedBox && factorsEl?.parentElement) {
    dedBox = document.createElement('div')
    dedBox.id = 'health-deductions'
    dedBox.className = 'list'
    const title = document.createElement('div')
    title.className = 'panel-title'
    title.setAttribute('data-i18n', 'healthDeductions')
    title.textContent = t('healthDeductions')
    factorsEl.parentElement.appendChild(title)
    factorsEl.parentElement.appendChild(dedBox)
  }
  if (dedBox) {
    const deds = Array.isArray(rep.deductions) ? rep.deductions : []
    if (!deds.length) {
      dedBox.innerHTML = `<div class="empty">${t('healthEmpty')}</div>`
    } else {
      dedBox.innerHTML = deds.map((d) => {
        const label = lang === 'en' ? (d.labelEn || d.labelRu) : (d.labelRu || d.labelEn)
        const detail = lang === 'en' ? (d.detailEn || d.detailRu) : (d.detailRu || d.detailEn)
        return `<div class="incident-card"><div class="incident-body">
          <div class="incident-title">−${Number(d.points) || 0}: ${escapeHtml(label)}</div>
          <div class="meta">${escapeHtml(detail || '')}</div>
        </div></div>`
      }).join('')
    }
  }
  renderIntelligence().catch(() => {})
}

function intelScenarioLabel(sc) {
  switch (sc) {
    case 'idle': return t('intelScenarioIdle')
    case 'work': return t('intelScenarioWork')
    case 'game': return t('intelScenarioGame')
    case 'high': return t('intelScenarioHigh')
    default: return sc || '-'
  }
}

async function renderIntelligence() {
  const note = $('intel-note')
  const bases = $('intel-baselines')
  const list = $('intel-insights')
  if (!bases || !list) return
  let st
  try { st = await GetIntelligenceStatus() } catch (_) { st = null }
  if (!st) return
  if (note) note.textContent = lang === 'en' ? (st.noteEn || '') : (st.noteRu || '')
  const bl = Array.isArray(st.baselines) ? st.baselines : []
  bases.innerHTML = bl.map((b) => `
    <div class="summary-card">
      <div class="label">${intelScenarioLabel(b.scenario)}</div>
      <div class="value">${b.ready ? t('intelReady') : t('intelLearning')}</div>
      <div class="delta">${b.samples || 0}/${st.minSamples || 30} ${t('intelSamples')} · CPU ${fmt(b.cpu, 0)}% · RAM ${fmt(b.ram, 0)}%</div>
    </div>`).join('')
  const insights = Array.isArray(st.insights) ? st.insights : []
  if (!insights.length) {
    list.innerHTML = `<div class="empty">${t('intelEmpty')}</div>`
    return
  }
  list.innerHTML = insights.map((ins) => {
    const text = lang === 'en' ? (ins.textEn || ins.textRu) : (ins.textRu || ins.textEn)
    const conf = Math.round((Number(ins.confidence) || 0) * 100)
    return `
      <div class="incident-card">
        <div class="incident-body">
          <div class="incident-title">${intelScenarioLabel(ins.scenario)} · ${escapeHtml(ins.metric || '')} · ${escapeHtml(ins.detectedAt || '')}</div>
          <div class="incident-explain">${escapeHtml(text || '')}</div>
          <div class="meta">${t('confidence')}: ${conf}%</div>
        </div>
      </div>`
  }).join('')
}

async function renderGamesTab() {
  const live = $('game-live')
  const list = $('game-sessions')
  const sessionsPanel = $('game-sessions-panel')
  const comparePanel = $('game-compare-panel')
  const selA = $('game-cmp-a')
  const selB = $('game-cmp-b')
  if (!live || !list) return
  let status
  try { status = await GetActiveGameStatus() } catch (_) { status = { active: false } }
  const livePanel = $('game-live-panel')
  if (status && status.active) {
    livePanel?.classList.remove('hidden')
    live.className = 'status-banner warn'
    live.innerHTML = `<b>${escapeHtml(status.name || status.exe)}</b> · ${t('unitsMin', { n: fmt(status.durationMin, 1) })}<br>
      CPU ${fmt(status.cpuPercent, 0)}% · GPU ${fmt(status.gpuPercent, 0)}% · RAM ${fmt(status.ramPercent, 0)}% ·
      CPU °C ${status.tempCPU ? fmt(status.tempCPU, 0) : '-'} · ${t('gameFreezes')}: ${status.freezes || 0}<br>
      <span class="meta">${escapeHtml(status.fpsNote || '')}</span>`
  } else {
    live.innerHTML = ''
    livePanel?.classList.add('hidden')
  }
  let sessions = []
  try { sessions = await GetGameSessions() } catch (_) { sessions = [] }
  const hasSessions = sessions.length > 0
  sessionsPanel?.classList.toggle('hidden', !hasSessions)
  if (hasSessions) {
    list.innerHTML = sessions.map((s) => `
      <div class="row">
        <div class="row-main">
          <div class="name">${escapeHtml(s.name || s.exe)} ${s.active ? '<span class="eco-badge">LIVE</span>' : ''}</div>
          <div class="meta">${escapeHtml(s.exe)} · ${t('unitsMin', { n: fmt(s.durationMin, 1) })} · CPU ${fmt(s.cpuMax, 0)}% / GPU ${fmt(s.gpuMax, 0)}% · ${t('gameFreezes')} ${s.freezes || 0}</div>
          <div class="meta">${s.start || ''} → ${s.end || '…'} · ${s.verdict || ''}</div>
        </div>
      </div>
    `).join('')
  } else {
    list.innerHTML = ''
  }
  const finished = sessions.filter((s) => !s.active)
  const canCompare = finished.length >= 2
  comparePanel?.classList.toggle('hidden', !canCompare)
  const opts = finished.map((s) =>
    `<option value="${s.id}">${escapeHtml(s.name || s.exe)} (${t('unitsMinShort', { n: fmt(s.durationMin, 0) })})</option>`
  ).join('')
  if (selA) selA.innerHTML = opts
  if (selB) selB.innerHTML = opts
}

async function runGameCompare() {
  const a = $('game-cmp-a')?.value
  const b = $('game-cmp-b')?.value
  const out = $('game-compare-out')
  if (!out || !a || !b || a === b) {
    if (out) out.textContent = t('gameComparePick')
    return
  }
  try {
    const r = await CompareGameSessions(a, b)
    if (!r.ok) {
      out.textContent = t('noData')
      return
    }
    out.innerHTML = (r.notes || []).map((n) => `<div>• ${escapeHtml(n)}</div>`).join('')
  } catch (err) {
    out.textContent = String(err)
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
  const grid = chartGridColor()
  const label = chartLabelColor()
  ctx.setLineDash([3, 4])
  ctx.lineWidth = 1
  ctx.font = '10px ui-monospace, Consolas, monospace'
  marks.forEach((m) => {
    const y = padT + plotH - (m.pct / 100) * plotH
    ctx.strokeStyle = grid
    ctx.globalAlpha = 0.35
    ctx.beginPath()
    ctx.moveTo(padL, y)
    ctx.lineTo(cssW - padR, y)
    ctx.stroke()
    ctx.globalAlpha = 1
    ctx.fillStyle = label
    ctx.fillText(m.label, 4, y + 3)
  })
  ctx.setLineDash([])

  const timeLabels = [
    { t: 0, label: '-30s' },
    { t: 1 / 3, label: '-20s' },
    { t: 2 / 3, label: '-10s' },
    { t: 1, label: t('nowLabel') },
  ]
  ctx.fillStyle = label
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
  const overview = document.getElementById('tab-overview')?.classList.contains('active')
  if (overview && !document.hidden) drawTrendFrame()
  if (gameOverlayOn) sampleFrametime(now)
  if (gameOverlayOn) requestAnimationFrame(animateChart)
  else setTimeout(() => requestAnimationFrame(animateChart), 500)
}

function formatVRAM(mb) {
  const n = Number(mb)
  if (!Number.isFinite(n) || n <= 0) return '-'
  if (n >= 1024) {
    const gb = n / 1024
    const rounded = Math.round(gb)
    if (Math.abs(gb - rounded) <= 0.2) return `${rounded} GB`
    return `${gb.toFixed(1)} GB`
  }
  return `${fmt(n, 0)} MB`
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
  return diskDriveLetter(mount)
}

/** Per-tile error shield: never leave Home empty on a render exception.
 *  Does NOT wipe tile-body structure (IDs must survive for the next poll). */
function showTileError(tileId, sectionKey, err) {
  const tile = $(tileId)
  if (!tile) return
  const body = tile.querySelector('.tile-body')
  if (!body) return
  const detail = String(err && (err.stack || err.message || err) || 'unknown')
  console.error('[PulseCore] tile render failed:', sectionKey, err)
  let banner = body.querySelector(':scope > [data-tile-error]')
  if (!banner) {
    banner = document.createElement('div')
    banner.className = 'tile-error'
    banner.setAttribute('role', 'alert')
    body.prepend(banner)
  }
  banner.setAttribute('data-tile-error', sectionKey)
  banner.innerHTML = '<div class="tile-error-msg">' + escapeHtml(t('tileRenderFail', { section: t(sectionKey) })) + '</div>'
    + '<button type="button" class="btn soft compact tile-error-copy" data-tile-diag="' + escapeHtml(sectionKey) + '">' + escapeHtml(t('tileCopyDiag')) + '</button>'
    + '<pre class="tile-error-log hidden">' + escapeHtml(detail) + '</pre>'
}

function clearTileError(tileId) {
  $(tileId)?.querySelector('.tile-body > [data-tile-error]')?.remove()
}

function guardTile(tileId, sectionKey, fn) {
  try {
    fn()
    clearTileError(tileId)
  } catch (err) {
    showTileError(tileId, sectionKey, err)
  }
}

function renderOverview(m) {
  const setTxt = (id, v) => { const el = $(id); if (el) el.textContent = v }
  const setW = (id, pct) => { const el = $(id); if (el) el.style.width = `${Math.min(100, pct || 0)}%` }
  const hw = m.hardware || {}
  const cpu = hw.cpu || {}
  const ram = hw.ram || {}

  setTxt('ov-cpu', `${fmt(m.cpuPercent)}%`)
  const cpuTemp = (m.temperatureAvailable && Number(m.temperatureC) > 0) ? `${fmt(m.temperatureC, 0)}°C` : ''
  const cpuFreq = cpu.frequencyMHz ? `${fmt(cpu.frequencyMHz / 1000, 2)} GHz` : ''
  setTxt('ov-cpu-sum', joinSummary([`${fmt(m.cpuPercent)}%`, cpuTemp, cpuFreq]))
  setTxt('ov-cpu-sub', m.cpuName || '')
  setW('ov-cpu-bar', m.cpuPercent)
  updateAdvancedBanner(m, lastAdvancedState)

  setTxt('ov-gpu', `${fmt(m.gpuPercent)}%`)
  const vramUsed = Number(m.vramUsedMB) || 0
  const vramTotal = Number(m.vramTotalMB) || Number(hw.gpu?.dedicatedMB) || 0
  const vramLine = vramTotal > 0
    ? `${formatVRAM(vramUsed)} / ${formatVRAM(vramTotal)}`
    : ''
  const gpuTemp = (m.gpuTemperatureAvailable && Number(m.gpuTemperatureC) > 0)
    ? `${fmt(m.gpuTemperatureC, 0)}°C`
    : ''
  setTxt('ov-gpu-sum', joinSummary([`${fmt(m.gpuPercent)}%`, vramLine, gpuTemp]))
  setTxt('ov-gpu-sub', m.gpuName || '')
  setW('ov-gpu-bar', m.gpuPercent)

  setTxt('ov-ram', `${fmt(m.ramPercent)}%`)
  const ramUsed = Number(m.ramUsedMB) || Number(ram.usedMB) || 0
  const ramTotal = Number(m.ramTotalMB) || Number(ram.totalMB) || 0
  const ramSpeed = ram.speedMHz ? `${ram.speedMHz} MHz` : ''
  const ramOcc = ramTotal > 0 ? `${fmt(ramUsed / 1024, 1)} / ${fmt(ramTotal / 1024, 1)} GB` : ''
  setTxt('ov-ram-sum', joinSummary([ramOcc || `${fmt(m.ramPercent)}%`, ramSpeed]))
  setTxt('ov-ram-sub', ramOcc)
  setW('ov-ram-bar', m.ramPercent)

  const modeKey = { silent: silentModeKey(), balanced: 'modeBalanced', turbo: 'modeTurbo' }
  const modeLabel = t(modeKey[m.powerMode] || 'modeBalanced')
  const bat = m.battery || {}
  setTxt('ov-bat-sum', modeLabel)
  if (bat.available) {
    setTxt('ov-bat', `${fmt(bat.percent, 0)}%`)
    setTxt('ov-bat-sub', `${modeLabel} · ${bat.statusText || ''}`)
    setW('ov-bat-bar', bat.percent)
  } else {
    setTxt('ov-bat', modeLabel)
    setTxt('ov-bat-sub', bat.wearLabel || t('desktopPC'))
    setW('ov-bat-bar', 100)
  }

  const chip = $('temp-chip')
  if (chip) {
    if (m.temperatureAvailable && Number(m.temperatureC) > 0) {
      chip.textContent = `${fmt(m.temperatureC)} °C`
    } else {
      chip.textContent = ''
    }
  }
  const cpuName = $('cpu-name')
  if (cpuName) cpuName.textContent = m.cpuName || 'CPU'

  latestDisks = m.disks || []
  try { renderDiskTabs(latestDisks) } catch (err) { console.error('[PulseCore] disk tabs', err) }
  const disks = latestDisks
  if (disks.length && !selectedDrive) selectedDrive = driveLetter(disks[0].mount)
  const box = $('ov-disks')
  // Don't rebuild cards every poll if already present (preserves expanded SMART panels)
  guardTile('tile-disks', 'tileSectionDisks', () => {
    if (box && (!box.querySelector('.disk-card') || box.querySelectorAll('.disk-card').length !== disks.length)) {
      renderDiskCards(disks)
    }
  })
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

function lagBackgroundWarn(m) {
  const vamps = m.vampires || []
  let sum = 0
  let top = null
  vamps.forEach((v) => {
    const cpu = Number(v.cpuPercent) || 0
    sum += cpu
    if (!top || cpu > (Number(top.cpuPercent) || 0)) top = v
  })
  const cores = m.cpuPerCore || m.hardware?.cpu?.perCore || []
  const maxCore = cores.reduce((a, b) => Math.max(a, Number(b) || 0), 0)
  if (top && (sum >= 50 || maxCore >= 99.5) && (Number(top.cpuPercent) || 0) >= 4) {
    return t('vampBackground')
      .replace('{name}', top.friendlyName || top.name || 'process')
      .replace('{cpu}', fmt(top.cpuPercent, 0))
  }
  return ''
}

async function openIncidentTimeline(id) {
  const panel = $('incident-timeline')
  const meta = $('timeline-meta')
  const canvas = $('timeline-chart')
  const legend = $('timeline-legend')
  if (!panel || !canvas) return
  let data
  try {
    data = await GetIncidentTimeline(id)
  } catch (_) {
    return
  }
  if (!data || data.error) return
  panel.classList.remove('hidden')
  panel.setAttribute('aria-hidden', 'false')
  const v = data.verdict || {}
  if (meta) {
    meta.textContent = `${data.start || ''} → ${data.end || ''} · ${v.type || ''} · ${v.culprit || t('unknownProc')}`
  }
  const series = [...(data.before || []), ...(data.during || []), ...(data.after || [])]
  drawTimelineChart(canvas, series, data.start, data.end)
  if (legend) {
    legend.textContent = `${t('timelineLegend')}: CPU / RAM / GPU / ${t('disk')} / ${t('temp')}`
  }
}

function drawTimelineChart(canvas, snaps, startISO, endISO) {
  const ctx = canvas.getContext('2d')
  if (!ctx) return
  const w = canvas.width
  const h = canvas.height
  ctx.clearRect(0, 0, w, h)
  ctx.fillStyle = 'rgba(20,24,32,0.9)'
  ctx.fillRect(0, 0, w, h)
  if (!snaps.length) {
    ctx.fillStyle = '#94a3b8'
    ctx.fillText(t('noData'), 12, h / 2)
    return
  }
  const pad = 12
  const plotW = w - pad * 2
  const plotH = h - pad * 2
  const seriesDefs = [
    { key: 'cpuPercent', color: '#38bdf8' },
    { key: 'ramPercent', color: '#a78bfa' },
    { key: 'gpuPercent', color: '#34d399' },
    { key: 'diskBusyPct', color: '#fbbf24' },
    { key: 'cpuTempC', color: '#f87171', scale: 100 },
  ]
  const n = snaps.length
  seriesDefs.forEach((s) => {
    ctx.beginPath()
    ctx.strokeStyle = s.color
    ctx.lineWidth = 1.5
    snaps.forEach((snap, i) => {
      let val = Number(snap[s.key]) || 0
      if (s.scale) val = Math.min(100, (val / s.scale) * 100)
      const x = pad + (i / Math.max(1, n - 1)) * plotW
      const y = pad + plotH - (Math.min(100, Math.max(0, val)) / 100) * plotH
      if (i === 0) ctx.moveTo(x, y)
      else ctx.lineTo(x, y)
    })
    ctx.stroke()
  })
  // Mark incident window
  if (startISO && n > 1) {
    const t0 = Date.parse(startISO)
    const t1 = Date.parse(endISO || startISO)
    const times = snaps.map((s) => Date.parse(s.timestamp) || 0)
    const minT = Math.min(...times.filter(Boolean))
    const maxT = Math.max(...times.filter(Boolean))
    if (maxT > minT && t0) {
      const x0 = pad + ((t0 - minT) / (maxT - minT)) * plotW
      const x1 = pad + (((t1 || t0) - minT) / (maxT - minT)) * plotW
      ctx.fillStyle = 'rgba(248,113,113,0.15)'
      ctx.fillRect(x0, pad, Math.max(2, x1 - x0), plotH)
      ctx.strokeStyle = 'rgba(248,113,113,0.7)'
      ctx.beginPath()
      ctx.moveTo(x0, pad)
      ctx.lineTo(x0, pad + plotH)
      ctx.stroke()
    }
  }
}

function ecoMsg(res) {
  if (!res) return ''
  return lang === 'en' ? (res.messageEn || res.messageRu || '') : (res.messageRu || res.messageEn || '')
}

function renderEcoResultBox(cardEl, res) {
  if (!cardEl || !res) return
  let box = cardEl.querySelector('.eco-result')
  if (!box) {
    box = document.createElement('div')
    box.className = 'eco-result'
    cardEl.appendChild(box)
  }
  const level = !res.ok ? 'bad' : (res.softEffect ? 'warn' : 'ok')
  box.className = `eco-result ${level}`
  const revert = res.canRevert && res.sessionId
    ? `<div style="margin-top:6px"><button type="button" class="btn compact" data-eco-revert="${res.sessionId}">${t('ecoRevert')}</button></div>`
    : ''
  box.innerHTML = `<div>${ecoMsg(res)}</div>${revert}`
}

async function applyEcoFeedback({ name, pid, cardEl, btn }) {
  if (btn) {
    btn.disabled = true
    btn.textContent = t('ecoApplying')
  }
  let res
  try {
    if (name) res = await ApplyEcoWithFeedback(name)
    else res = await ApplyEcoPIDWithFeedback(Number(pid) || 0)
  } catch (err) {
    res = { ok: false, messageRu: String(err), messageEn: String(err) }
  }
  showToast(ecoMsg(res), !!res.ok, { sessionId: res?.canRevert ? res.sessionId : '', holdMs: res?.canRevert ? 15000 : 5000 })
  if (cardEl) renderEcoResultBox(cardEl, res)
  renderEcoLimitedPanel().catch(() => {})
  if (btn) {
    btn.disabled = false
    if (res && res.ok) {
      btn.textContent = t('ecoLimited')
      btn.classList.remove('danger')
      btn.classList.add('eco-on')
      if (pid) frozen.add(Number(pid))
    } else {
      btn.textContent = t('ecoLower')
    }
  }
  return res
}

function setLagSystemVerdict(hasIncidents) {
  const el = $('lag-system-verdict')
  if (!el) return
  if (hasIncidents) {
    el.className = 'status-banner warn'
    el.textContent = t('lagSystemWarn')
  } else {
    el.className = 'status-banner ok'
    el.textContent = t('lagSystemOk')
  }
}

async function renderLagIncidents() {
  const stutterBox = $('stutter-log')
  const panel = $('lag-incidents-panel')
  const liveCard = $('lag-live-card')
  if (!stutterBox) return

  let status = null
  try { status = await GetLagDetectorStatus() } catch (_) { status = null }

  let cards = []
  try { cards = await GetLagIncidents() } catch (_) { cards = [] }
  const hourCards = Array.isArray(status?.recent) ? status.recent : []
  const real = (cards || []).filter((c) => !c.synthetic)
  const hasHour = hourCards.length > 0 || real.some((c) => !c.synthetic)
  setLagSystemVerdict(hasHour)

  if (liveCard) {
    if (!hasHour) {
      liveCard.className = 'lag-live-card ok'
      liveCard.innerHTML = `
        <div class="lag-live-title">${t('lagBufferActive')}</div>
        <div class="lag-live-meta">${t('lagBufferWindow', { n: status?.windowSec || 60 })}</div>
        <div class="lag-live-meta">${t('lagGameDrops', { n: Number(status?.gameDropsBelow60) || 0 })}</div>`
    } else {
      const top = hourCards[0] || real[0]
      const culprit = top?.culprit === 'many_processes' ? t('manyProcesses') : (top?.culprit || t('unknownProc'))
      liveCard.className = 'lag-live-card warn'
      liveCard.innerHTML = `
        <div class="lag-live-title">${escapeHtml(top?.time || '')} — ${escapeHtml(top?.verdictText || t('lagMicrofreeze'))}</div>
        <div class="lag-live-meta">${t('culprit')}: ${escapeHtml(culprit)}</div>`
    }
  }

  const listSource = real.length ? real : hourCards
  if (!listSource.length) {
    stutterBox.innerHTML = ''
    panel?.classList.add('hidden')
    return
  }
  panel?.classList.remove('hidden')
  stutterBox.innerHTML = listSource.map((c) => {
    const demo = c.synthetic ? `<span class="badge-count">DEMO</span>` : ''
    const targets = Array.isArray(c.ecoTargets) ? c.ecoTargets : []
    let ecoBtns = ''
    if (targets.length) {
      ecoBtns = targets.map((tg) => `
        <button type="button" class="btn compact danger" data-eco-name="${tg.name || ''}" data-pid="${tg.pid || 0}">
          ${t('ecoLower')}${tg.name ? `: ${tg.name}` : ''}
        </button>
      `).join('')
    } else if (c.canEco) {
      ecoBtns = `<button type="button" class="btn compact danger" data-eco-name="${c.culprit || ''}" data-pid="${c.culpritPID || 0}">${t('ecoLower')}</button>`
    }
    const human = (c.evidence || []).filter(Boolean)
    const humanBlock = human.length
      ? `<div class="incident-evidence-human">${human.map((e) => escapeHtml(e)).join(' · ')}</div>`
      : ''
    const culpritLabel = c.culprit === 'many_processes' ? t('manyProcesses') : (c.culprit || t('unknownProc'))
    const timelineBtn = c.id
      ? `<button type="button" class="btn compact" data-timeline="${c.id}">${t('openTimeline')}</button>`
      : ''
    return `
      <div class="incident-card" data-incident-id="${c.id || ''}">
        <div class="incident-body">
          <div class="incident-title">${escapeHtml(c.time || '')} ${demo}</div>
          <div class="incident-explain">${escapeHtml(c.verdictText || t('verdictUnknown'))}</div>
          <div class="meta">${t('culprit')}: ${escapeHtml(culpritLabel)}</div>
          <div class="incident-evidence">${humanBlock}</div>
        </div>
        <div class="row-actions">
          ${timelineBtn}
          ${ecoBtns}
        </div>
      </div>
    `
  }).join('')
}

let hzTrialTimer = null
let hzTrialState = null

async function openMonitorsModal(preselectDevice = '', preferHz = 0) {
  const modal = $('monitors-modal')
  const list = $('monitors-list')
  if (!modal || !list) return
  modal.classList.remove('hidden')
  list.innerHTML = `<div class="sf-loading">${t('monitorsLoading')}</div>`
  let displays = []
  try { displays = await ListDisplays() } catch (err) {
    list.innerHTML = `<div class="sf-loading">${escapeHtml(String(err))}</div>`
    return
  }
  const items = Array.isArray(displays) ? displays : []
  if (!items.length) {
    list.innerHTML = `<div class="empty">${t('monitorsEmpty')}</div>`
    return
  }
  list.innerHTML = items.map((d, idx) => {
    const name = d.monitorName || d.deviceName || `Monitor ${idx + 1}`
    const role = d.isPrimary ? t('monitorsPrimary') : t('monitorsSecondary', { n: idx + 1 })
    const hzList = Array.isArray(d.availableHz) ? d.availableHz : []
    const selectedHz = preferHz && d.deviceName === preselectDevice
      ? preferHz
      : (d.needsAttention ? (d.maxHz || d.maxSupportedHz) : d.currentHz)
    const chips = hzList.map((hz) => {
      const on = Number(hz) === Number(selectedHz)
      const cur = Number(hz) === Number(d.currentHz)
      return `<button type="button" class="hz-chip ${on ? 'active' : ''} ${cur ? 'current' : ''}" data-hz="${hz}">${hz} ${t('unitsHz')}</button>`
    }).join('') || `<span class="meta">${d.currentHz || '—'} ${t('unitsHz')}</span>`
    const warn = d.needsAttention
      ? `<div class="mon-warn">${t('monitorsUnderclocked', { cur: d.currentHz, max: d.maxHz || d.maxSupportedHz })}</div>`
      : ''
    return `
      <div class="mon-card ${d.needsAttention ? 'needs' : ''}" data-device="${escapeHtml(d.deviceName || '')}" data-selected-hz="${selectedHz || d.currentHz || 0}">
        <div class="mon-card-head">
          <div class="mon-name">${escapeHtml(name)}</div>
          <div class="mon-role">${escapeHtml(role)}</div>
        </div>
        <div class="mon-meta">${d.currentWidth || d.width || '?'}×${d.currentHeight || d.height || '?'} · ${d.currentHz || '—'} ${t('unitsHz')}</div>
        ${warn}
        <div class="hz-chip-row">${chips}</div>
        <button type="button" class="btn primary compact mon-test-btn">${t('monitorsTestHz')}</button>
      </div>`
  }).join('')

  list.querySelectorAll('.mon-card').forEach((card) => {
    card.querySelectorAll('.hz-chip').forEach((chip) => {
      chip.addEventListener('click', () => {
        card.querySelectorAll('.hz-chip').forEach((c) => c.classList.remove('active'))
        chip.classList.add('active')
        card.dataset.selectedHz = chip.getAttribute('data-hz') || ''
      })
    })
    card.querySelector('.mon-test-btn')?.addEventListener('click', async () => {
      const device = card.dataset.device || ''
      const hz = Number(card.dataset.selectedHz) || 0
      if (!device || !hz) return
      await startHzTrial(device, hz)
    })
  })
}

function closeMonitorsModal() {
  $('monitors-modal')?.classList.add('hidden')
}

async function startHzTrial(deviceName, hz) {
  let res
  try { res = await TestAndSetRefreshRate(deviceName, hz) } catch (err) {
    showToast(String(err), false, { holdMs: 5000 })
    return
  }
  if (!res?.ok) {
    showToast(res?.message || t('hzTrialUnsupported'), false, { holdMs: 6000 })
    return
  }
  hzTrialState = {
    deviceName: res.deviceName || deviceName,
    hz: Number(res.hz) || hz,
    previousHz: Number(res.previousHz) || 0,
    left: Number(res.timeoutSec) || 15,
  }
  openHzTrialModal()
}

function openHzTrialModal() {
  const modal = $('hz-trial-modal')
  if (!modal || !hzTrialState) return
  modal.classList.remove('hidden')
  $('hz-trial-title').textContent = t('hzTrialTitle', { hz: hzTrialState.hz })
  $('hz-trial-hint').textContent = t('hzTrialHint')
  $('hz-trial-sub').textContent = t('hzTrialSub')
  updateHzCountdown(hzTrialState.left)
  clearInterval(hzTrialTimer)
  hzTrialTimer = setInterval(() => {
    if (!hzTrialState) return
    hzTrialState.left -= 1
    updateHzCountdown(Math.max(0, hzTrialState.left))
    if (hzTrialState.left <= 0) {
      clearInterval(hzTrialTimer)
      hzTrialTimer = null
      closeHzTrialModal()
      showToast(t('hzTrialAutoReverted'), true, { holdMs: 4000 })
      openMonitorsModal().catch(() => {})
    }
  }, 1000)
}

function updateHzCountdown(n) {
  const el = $('hz-trial-countdown')
  if (el) el.textContent = String(n)
  $('hz-trial-sub').textContent = t('hzTrialCountdown', { n })
}

function closeHzTrialModal() {
  clearInterval(hzTrialTimer)
  hzTrialTimer = null
  $('hz-trial-modal')?.classList.add('hidden')
}

async function confirmHzTrial() {
  if (!hzTrialState) return
  const { deviceName, hz } = hzTrialState
  clearInterval(hzTrialTimer)
  hzTrialTimer = null
  let res
  try { res = await ConfirmRefreshRate(deviceName, hz) } catch (err) {
    showToast(String(err), false)
    return
  }
  closeHzTrialModal()
  hzTrialState = null
  showToast(res?.ok ? t('hzTrialSaved', { hz }) : (res?.message || t('hzTrialFail')), !!res?.ok)
  openMonitorsModal().catch(() => {})
  renderHome().catch(() => {})
}

async function revertHzTrial() {
  const device = hzTrialState?.deviceName || ''
  clearInterval(hzTrialTimer)
  hzTrialTimer = null
  let res
  try { res = await RevertRefreshRate(device) } catch (err) {
    showToast(String(err), false)
    return
  }
  closeHzTrialModal()
  hzTrialState = null
  showToast(res?.ok ? t('hzTrialReverted') : (res?.message || t('hzTrialFail')), !!res?.ok)
  openMonitorsModal().catch(() => {})
}

function escapeHtml(s) {
  return String(s ?? '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
}

function radarTypeLabel(type) {
  switch (type) {
    case 'cpu_spike': return t('radarTypeCpu')
    case 'ram_spike': return t('radarTypeRam')
    case 'disk_spike': return t('radarTypeDisk')
    case 'orphan_load': return t('radarTypeOrphan')
    case 'ram_creep': return t('radarTypeCreep')
    default: return type || '-'
  }
}

function memVerdictLabel(v) {
  switch (v) {
    case 'sustained_growth': return t('memVerdictGrowth')
    case 'possible_leak': return t('memVerdictPossible')
    case 'normal': return t('memVerdictNormal')
    case 'insufficient_data': return t('memVerdictNeedData')
    default: return v || '-'
  }
}

async function renderRadarPanel() {
  const panel = $('radar-panel')
  const box = $('radar-list')
  const memBox = $('memwatch-list')
  if (!box || !memBox) return
  let anomalies = []
  let memCards = []
  try {
    anomalies = await GetProcessAnomalies()
  } catch (_) { anomalies = [] }
  try {
    memCards = await GetMemoryWatchCards()
  } catch (_) { memCards = [] }

  // Only real loaders - no Δ%/weight noise; skip low-confidence noise.
  const loadTypes = new Set(['cpu_spike', 'disk_spike', 'ram_spike', 'orphan_load', 'ram_creep'])
  const meaningful = (anomalies || []).filter((a) => {
    if (!loadTypes.has(a.type)) return false
    const conf = Number(a.confidence) || 0
    const delta = Number(a.deltaPct)
    if (conf < 0.45) return false
    if (Number.isFinite(delta) && delta < 40 && a.type !== 'ram_creep') return false
    return true
  })

  const memReal = (memCards || []).filter((c) =>
    c.verdict === 'sustained_growth' || c.verdict === 'possible_leak'
  )

  if (!meaningful.length && !memReal.length) {
    panel?.classList.add('hidden')
    box.innerHTML = ''
    memBox.innerHTML = ''
    return
  }
  panel?.classList.remove('hidden')

  box.innerHTML = meaningful.length
    ? meaningful.map((a) => {
      const text = lang === 'en' ? (a.textEn || a.textRu) : (a.textRu || a.textEn)
      const when = a.detectedAt ? `<div class="meta">${escapeHtml(a.detectedAt)}</div>` : ''
      return `
        <div class="incident-card">
          <div class="incident-body">
            <div class="incident-title">${escapeHtml(a.friendly || a.process || '-')}</div>
            <div class="incident-explain">${escapeHtml(text || '')}</div>
            ${when}
          </div>
        </div>`
    }).join('')
    : ''

  memBox.innerHTML = memReal.length
    ? memReal.map((c) => {
      const text = lang === 'en' ? (c.textEn || c.textRu) : (c.textRu || c.textEn)
      return `
        <div class="incident-card">
          <div class="incident-body">
            <div class="incident-title">${escapeHtml(c.friendly || c.process || '-')} · ${memVerdictLabel(c.verdict)}</div>
            <div class="incident-explain">${escapeHtml(text || '')}</div>
          </div>
        </div>`
    }).join('')
    : ''
}

async function renderNetworkMonitor() {
  const sys = $('net-mon-sys')
  const note = $('net-mon-note')
  const list = $('net-mon-list')
  if (!sys || !list) return
  let snap
  try { snap = await GetNetworkMonitor() } catch (_) { snap = null }
  if (!snap || !snap.ok) {
    sys.textContent = '-'
    if (note) note.textContent = ''
    list.innerHTML = `<div class="empty">${t('netMonEmpty')}</div>`
    return
  }
  sys.textContent = t('netMonSys')
    .replace('{send}', fmt(snap.systemSendKBps, 1))
    .replace('{recv}', fmt(snap.systemRecvKBps, 1))
    .replace('{sendMB}', fmt(snap.sessionSendMB, 1))
    .replace('{recvMB}', fmt(snap.sessionRecvMB, 1))
  if (note) note.textContent = lang === 'en' ? (snap.perProcessNoteEn || '') : (snap.perProcessNoteRu || '')
  const procs = Array.isArray(snap.processes) ? snap.processes : []
  if (!procs.length) {
    list.innerHTML = `<div class="empty">${t('netMonEmpty')}</div>`
    return
  }
  list.innerHTML = procs.map((p) => {
    const detail = lang === 'en' ? (p.detailEn || p.detailRu) : (p.detailRu || p.detailEn)
    return `
      <div class="incident-card">
        <div class="incident-body">
          <div class="incident-title">${escapeHtml(p.friendly || p.name || '-')} · PID ${p.pid || 0}</div>
          <div class="meta">${p.connCount || 0} ${t('netMonConns')} · ${escapeHtml(detail || '')}</div>
        </div>
      </div>`
  }).join('')
}

function renderLag(m) {
  renderLagIncidents().catch(() => {})
  renderEcoLimitedPanel().catch(() => {})
  renderRadarPanel().catch(() => {})

  const lags = m.lagEvents || []
  const lagBox = $('lag-log')
  if (lagBox) lagBox.innerHTML = ''
  if (lags.length) setLagSystemVerdict(true)

  const vamps = m.vampires || []
  const vampBox = $('vampire-list')
  const vampPanel = $('vampires-panel')
  const wl = loadVampWhitelist()
  const visible = vamps.filter((v) => !isVampWhitelisted(v, wl))
  renderVampExclusions(wl)
  if (!visible.length) {
    if (vampBox) {
      vampBox.innerHTML = (vamps.length || wl.length)
        ? `<div class="empty">${t('vampAllHidden')}</div>`
        : ''
    }
    if (!vamps.length && !wl.length) vampPanel?.classList.add('hidden')
    else vampPanel?.classList.remove('hidden')
  } else {
    vampPanel?.classList.remove('hidden')
    vampBox.innerHTML = visible.map((v) => {
      const isEco = frozen.has(v.pid) || v.ecoMode || v.frozen
      const count = v.count || (v.pids ? v.pids.length : 1) || 1
      const badge = count > 1 ? `<span class="badge-count">${procCountLabel(count)}</span>` : ''
      const ecoBadge = isEco ? `<span class="eco-badge">Eco</span>` : ''
      const reason = lang === 'en' ? (v.reasonEn || v.reasonRu) : (v.reasonRu || v.reasonEn)
      return `
        <div class="row">
          <div class="row-main">
            <div class="name"><span>${escapeHtml(v.friendlyName || v.name)}</span>${badge}${ecoBadge}</div>
            <div class="meta">${escapeHtml(v.name)} · CPU ${fmt(v.cpuPercent)}% · disk ~${fmt(v.diskMBps)} MB/s</div>
            ${reason ? `<div class="vamp-reason">${escapeHtml(reason)}</div>` : ''}
          </div>
          <div class="row-actions">
            <button type="button" class="btn compact ghost vamp-ignore" data-vamp-ignore="${escapeHtml(v.name)}" title="${escapeHtml(t('vampIgnore'))}" aria-label="${escapeHtml(t('vampIgnore'))}">👁</button>
            <button type="button" class="btn compact ${isEco ? 'eco-on' : 'danger'}" data-vamp-eco="${escapeHtml(v.name)}" data-act="${isEco ? 'resume' : 'freeze'}" data-pid="${v.pid}">
              ${isEco ? t('ecoLimited') : (count > 1 ? t('ecoAll') : t('freeze'))}
            </button>
          </div>
        </div>
      `
    }).join('')
  }
}

const VAMP_WL_KEY = 'vampires_whitelist'

function loadVampWhitelist() {
  try {
    const raw = localStorage.getItem(VAMP_WL_KEY)
    const arr = raw ? JSON.parse(raw) : []
    return Array.isArray(arr) ? arr.map((x) => String(x || '').toLowerCase()).filter(Boolean) : []
  } catch (_) {
    return []
  }
}

function saveVampWhitelist(list) {
  try {
    localStorage.setItem(VAMP_WL_KEY, JSON.stringify([...new Set(list)]))
  } catch (_) {}
}

function isVampWhitelisted(v, wl = loadVampWhitelist()) {
  const name = String(v?.name || '').toLowerCase()
  const friend = String(v?.friendlyName || '').toLowerCase()
  return wl.includes(name) || (friend && wl.includes(friend))
}

function addVampWhitelist(name) {
  const key = String(name || '').toLowerCase()
  if (!key) return
  const wl = loadVampWhitelist()
  if (!wl.includes(key)) {
    wl.push(key)
    saveVampWhitelist(wl)
  }
}

function removeVampWhitelist(name) {
  const key = String(name || '').toLowerCase()
  saveVampWhitelist(loadVampWhitelist().filter((x) => x !== key))
}

function renderVampExclusions(wl = loadVampWhitelist()) {
  const box = $('vamp-exclusions')
  const list = $('vamp-exclusions-list')
  const summary = $('vamp-exclusions-summary')
  if (!box || !list) return
  if (!wl.length) {
    box.classList.add('hidden')
    list.innerHTML = ''
    return
  }
  box.classList.remove('hidden')
  if (summary) summary.textContent = t('vampExclusions', { n: wl.length })
  list.innerHTML = wl.map((name) => `
    <div class="row vamp-excl-row">
      <div class="row-main"><div class="name">${escapeHtml(name)}</div></div>
      <button type="button" class="btn compact soft" data-vamp-restore="${escapeHtml(name)}">${t('vampRestore')}</button>
    </div>
  `).join('')
}

function renderHardware(m) {
  const hw = m.hardware || {}
  const cpu = hw.cpu || {}
  const gpu = hw.gpu || {}
  const ram = hw.ram || {}
  const pwr = hw.power || {}

  const setHw = (id, v) => { const el = $(id); if (el) el.textContent = v }
  guardTile('tile-cpu', 'tileSectionCPU', () => {
  setHw('hw-cpu-name', cpu.name || m.cpuName || '-')
  setHw('hw-cpu-cores', `${cpu.physicalCores || '-'} / ${cpu.logicalCores || '-'}`)
  setHw('hw-cpu-freq', cpu.frequencyMHz ? `${fmt(cpu.frequencyMHz / 1000, 2)} GHz` : '-')
  setHw('hw-cpu-load', `${fmt(cpu.percent ?? m.cpuPercent)}%`)
  const cpuTempRow = $('hw-cpu-temp-row')
  const cpuTempSrc = $('hw-cpu-temp-src')
  if (m.temperatureAvailable && Number(m.temperatureC) > 0) {
    setHw('hw-cpu-temp', `${fmt(m.temperatureC, 0)} °C`)
    if (cpuTempRow) cpuTempRow.classList.remove('hidden')
    if (cpuTempSrc) {
      const srcLabel = lang === 'en'
        ? (m.temperatureSourceLabelEn || '')
        : (m.temperatureSourceLabelRu || m.temperatureSourceLabelEn || '')
      cpuTempSrc.textContent = srcLabel
      cpuTempSrc.classList.toggle('hidden', !srcLabel)
    }
  } else {
    if (cpuTempRow) cpuTempRow.classList.add('hidden')
    if (cpuTempSrc) {
      cpuTempSrc.textContent = ''
      cpuTempSrc.classList.add('hidden')
    }
  }

  const cores = cpu.perCore || m.cpuPerCore || []
  const phys = Math.max(1, Number(cpu.physicalCores) || Math.ceil(cores.length / 2) || 1)
  const threadsPer = Math.max(1, Math.round((cores.length || phys) / phys))
  const coreBlocks = []
  for (let p = 0; p < phys; p++) {
    const threads = []
    for (let t = 0; t < threadsPer; t++) {
      const idx = p * threadsPer + t
      if (idx >= cores.length) break
      const load = cores[idx] || 0
      threads.push(`
        <div class="thread-cell">
          <div class="id">T${t}</div>
          <div class="val">${fmt(load, 0)}%</div>
          <div class="mini-bar"><i style="width:${Math.min(100, load)}%"></i></div>
        </div>`)
    }
    if (!threads.length) continue
    coreBlocks.push(`
      <div class="phys-core">
        <div class="phys-core-title">${t('hwCoreTitle', { n: p })}</div>
        <div class="phys-core-threads">${threads.join('')}</div>
      </div>`)
  }
  const coreGrid = $('core-grid')
  if (coreGrid) coreGrid.innerHTML = coreBlocks.join('') || ''
  })

  guardTile('tile-gpu', 'tileSectionGPU', () => {
  setHw('hw-gpu-name', gpu.name || m.gpuName || '-')
  setHw('hw-gpu-driver', gpu.driverVersion || '-')
  setHw('hw-gpu-ded', formatVRAM(gpu.dedicatedMB ?? m.vramTotalMB))
  {
    const sharedMB = Number(gpu.sharedMB ?? m.vramSharedMB) || 0
    const sharedRow = $('hw-gpu-shared-row')
    if (sharedMB > 0) {
      setHw('hw-gpu-shared', `${fmt(sharedMB, 0)} MB`)
      if (sharedRow) sharedRow.classList.remove('hidden')
    } else {
      setHw('hw-gpu-shared', '')
      if (sharedRow) sharedRow.classList.add('hidden')
    }
  }
  const gpuTempOK = !!(m.gpuTemperatureAvailable || gpu.tempAvailable) && Number(m.gpuTemperatureC ?? gpu.temperatureC) > 0
  const gpuTempRow = $('hw-gpu-temp-row')
  if (gpuTempOK) {
    setHw('hw-gpu-temp', `${fmt(m.gpuTemperatureC ?? gpu.temperatureC, 0)} °C`)
    if (gpuTempRow) gpuTempRow.classList.remove('hidden')
  } else if (gpuTempRow) {
    gpuTempRow.classList.add('hidden')
  }
  const gpuPowerOK = !!(m.gpuPowerAvailable || gpu.powerAvailable) && Number(m.gpuPowerW ?? gpu.powerW) > 0
  const gpuPowerRow = $('hw-gpu-power-row')
  if (gpuPowerOK) {
    setHw('hw-gpu-power', `${fmt(m.gpuPowerW ?? gpu.powerW, 0)} W`)
    if (gpuPowerRow) gpuPowerRow.classList.remove('hidden')
  } else if (gpuPowerRow) {
    gpuPowerRow.classList.add('hidden')
  }
  const coreMHz = Number(m.gpuClockCoreMHz ?? gpu.clockCoreMHz) || 0
  const memMHz = Number(m.gpuClockMemMHz ?? gpu.clockMemMHz) || 0
  const gpuClockRow = $('hw-gpu-clock-row')
  if (coreMHz > 0 || memMHz > 0) {
    const parts = []
    if (coreMHz > 0) parts.push(`${fmt(coreMHz, 0)} MHz`)
    if (memMHz > 0) parts.push(`mem ${fmt(memMHz, 0)} MHz`)
    setHw('hw-gpu-clocks', parts.join(' · '))
    if (gpuClockRow) gpuClockRow.classList.remove('hidden')
  } else if (gpuClockRow) {
    gpuClockRow.classList.add('hidden')
  }
  })

  guardTile('tile-ram', 'tileSectionRAM', () => {
  const ramModel = $('hw-ram-model')
  if (ramModel) {
    ramModel.textContent = ram.moduleModel
      || (ram.partNumber ? t('hwRamModelPart', { mfr: ram.manufacturer || 'RAM', part: ram.partNumber }) : '-')
  }
  setHw('hw-ram-total', `${ram.totalMB || m.ramTotalMB || 0} MB`)
  setHw('hw-ram-type', `${ram.type || '-'}${ram.speedMHz ? ` · ${ram.speedMHz} MHz` : ''}`)
  setHw('hw-ram-slots', `${ram.slotsUsed || 0}/${ram.slotsTotal || 0}`)
  setHw('hw-ram-use', `${ram.usedMB || m.ramUsedMB || 0} / ${ram.freeMB || 0} MB`)
  const channel = $('hw-ram-channel')
  const channelHint = $('hw-ram-channel-hint')
  if (channel) {
    channel.classList.remove('status-ok', 'status-warn')
    if (ram.isSingleChannel) {
      channel.textContent = t('memSingle')
      channel.classList.add('status-warn')
      if (channelHint) {
        channelHint.textContent = t('memSingleHint')
        channelHint.classList.remove('hidden')
      }
    } else if ((ram.slotsUsed || 0) >= 2) {
      channel.textContent = t('memDual')
      channel.classList.add('status-ok')
      if (channelHint) channelHint.classList.add('hidden')
    } else {
      channel.textContent = ram.channelStatus || '-'
      if (channelHint) channelHint.classList.add('hidden')
    }
  }
  })

  guardTile('tile-power', 'tileSectionPower', () => {
  if (pwr.hasBattery) {
    const ln = $('hw-pwr-lbl-name'); if (ln) ln.textContent = t('lblModel')
    const ls = $('hw-pwr-lbl-status'); if (ls) ls.textContent = t('lblStatus')
    const lp = $('hw-pwr-lbl-pct'); if (lp) lp.textContent = t('lblCharge')
    const lw = $('hw-pwr-lbl-wear'); if (lw) lw.textContent = t('lblWear')
    const lc = $('hw-pwr-lbl-cap'); if (lc) lc.textContent = t('lblCapacityWh')
    setHw('hw-pwr-name', pwr.deviceName || t('batteryPack'))
    setHw('hw-pwr-status', pwr.statusText || '-')
    setHw('hw-pwr-pct', `${fmt(pwr.percent, 0)}%`)
    setHw('hw-pwr-wear', pwr.wearLabel || `${fmt(pwr.wearPercent, 0)}%`)
    setHw('hw-pwr-cap', pwr.capacitymWh
      ? `${fmt(pwr.capacitymWh, 0)} / ${fmt(pwr.designCapacitymWh, 0)} mWh`
      : '-')
  } else {
    const ln = $('hw-pwr-lbl-name'); if (ln) ln.textContent = t('lblModel')
    const ls = $('hw-pwr-lbl-status'); if (ls) ls.textContent = t('pwrDesktopStatus')
    const lp = $('hw-pwr-lbl-pct'); if (lp) lp.textContent = t('pwrDesktopPower')
    const lw = $('hw-pwr-lbl-wear'); if (lw) lw.textContent = t('pwrDesktopDraw')
    const lc = $('hw-pwr-lbl-cap'); if (lc) lc.textContent = t('pwrDesktopPlan')
    setHw('hw-pwr-name', pwr.deviceName || t('pwrDesktopName'))
    setHw('hw-pwr-status', pwr.powerScheme || pwr.statusText || t('desktopMode'))
    setHw('hw-pwr-pct', t('pwrDesktopAC'))
    setHw('hw-pwr-wear', pwr.wearLabel ||
      (pwr.totalWatts
        ? `${t('pwrEstimate')}: CPU ${fmt(pwr.cpuWatts, 0)}W + GPU ${fmt(pwr.gpuWatts, 0)}W ≈ ${fmt(pwr.totalWatts, 0)}W`
        : t('desktopPC')))
    setHw('hw-pwr-cap', pwr.powerScheme || '-')
  }
  })
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
    if (r.warmingUp || r.note === 'session_warming') {
      box.innerHTML = `<div class="empty">${t('weeklyWarming')}</div>`
      return
    }
    const apps = (r.topApps || []).filter((a) => !whitelist.has(String(a.name || '').toLowerCase()))
    if (!apps.length) {
      box.innerHTML = `<div class="empty">${t('weeklySessionEmpty')}</div>`
      return
    }
    box.innerHTML = apps.map((a, i) => {
      const title = a.friendlyName || a.name
      const detail = a.detail || `CPU ${fmt(a.cpuSec, 0)}s · disk ${fmt(a.diskMB, 1)} MB`
      return `
      <div class="row">
        <div>
          <div class="name">#${i + 1} ${title}</div>
          <div class="meta">${detail}</div>
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

let netLastDetail = null

function syncNetAccordionChrome() {
  const body = $('net-acc-body')
  const btn = $('btn-net-toggle')
  const chev = $('net-acc-chevron')
  const open = uiExpand.tiles.has('net-lags')
  body?.classList.toggle('collapsed', !open)
  if (body) body.hidden = !open
  btn?.setAttribute('aria-expanded', open ? 'true' : 'false')
  if (chev) chev.innerHTML = chevronSVG()
  btn?.classList.toggle('open', open)
  updateNetCollapsedSummary()
}

function updateNetCollapsedSummary() {
  const el = $('net-acc-summary')
  if (!el) return
  if (!netLastDetail || !netLastDetail.checked) {
    el.textContent = t('netCheckNetwork')
    return
  }
  const d = netLastDetail
  if (d.level === 'ok' && d.summaryPingMs >= 0) {
    const msLabel = d.summaryPingMs < 1 ? '<1' : fmt(d.summaryPingMs, 0)
    el.textContent = t('netStableSummary', { ms: msLabel })
    return
  }
  el.textContent = t(d.verdictKey || 'netVerdictOffline')
}

function formatPingMs(ms) {
  const v = Number(ms)
  if (!Number.isFinite(v) || v < 0) return ''
  if (v < 1) return '<1 ms'
  return `${fmt(v, 0)} ms`
}

function formatLinkSpeedClient(mbps) {
  const v = Number(mbps) || 0
  if (v <= 0 || v >= 200000) return ''
  if (v >= 1000) {
    const gb = v / 1000
    return Number.isInteger(gb) ? `${gb} Gbps` : `${gb.toFixed(1)} Gbps`
  }
  return `${Math.round(v)} Mbps`
}

function netDetailRows(d) {
  const rows = []
  const add = (label, value) => {
    if (value == null || value === '' || value === false) return
    rows.push(`<div class="row"><div class="name">${escapeHtml(label)}</div><div class="meta">${escapeHtml(String(value))}</div></div>`)
  }
  add(t('netConnType'), d.connType)
  add(t('netLocalIP'), d.localIP)
  if (d.gateway) {
    const ms = formatPingMs(d.gatewayMs)
    add(t('netGateway'), ms ? `${d.gateway} · ${ms}` : d.gateway)
  }
  if (Array.isArray(d.dnsServers) && d.dnsServers.length) {
    const ms = formatPingMs(d.dnsMs)
    add(t('netDNS'), ms ? `${d.dnsServers.join(', ')} · ${ms}` : d.dnsServers.join(', '))
  }
  if (d.internetHost) {
    const ms = formatPingMs(d.internetMs)
    add(t('netInternet'), ms ? `${d.internetHost} · ${ms}` : d.internetHost)
  }
  if (d.adapterName) {
    const speed = d.linkSpeedLabel || (d.linkMbps > 0 && d.linkMbps < 200000 ? formatLinkSpeedClient(d.linkMbps) : '')
    const bits = [d.adapterName, d.adapterStatus, speed].filter(Boolean)
    add(t('netAdapter'), bits.join(' · '))
  }
  if (d.wifiSignal > 0) add(t('netWifiSignal'), `${d.wifiSignal}%`)
  if (d.wifiBand) add(t('netWifiBand'), d.wifiBand)
  if (d.wifiSSID) add(t('netWifiSSID'), d.wifiSSID)
  return rows.join('')
}

async function runNetDiag() {
  const box = $('net-diag')
  if (!box) return
  box.innerHTML = `
    <div class="net-progress">
      <div class="label">${t('netProgress')}</div>
      <div class="track"><div class="fill"></div></div>
    </div>`
  try {
    let d
    if (typeof CheckNetworkDetailed === 'function') {
      d = await CheckNetworkDetailed(true)
    } else {
      const legacy = await DiagnoseLagOrigin()
      d = {
        checked: true,
        level: legacy.level,
        verdictKey: legacy.verdictKey || legacy.verdict,
        summaryPingMs: legacy.publicMs >= 0 ? legacy.publicMs : legacy.gatewayMs,
        gateway: legacy.gatewayHost,
        gatewayMs: legacy.gatewayMs,
        internetHost: legacy.publicHost,
        internetMs: legacy.publicMs,
        ok: legacy.ok,
      }
    }
    netLastDetail = d
    updateNetCollapsedSummary()
    const offline = d.level === 'offline'
    let text = t(d.verdictKey || 'netVerdictOK')
    let verdictCls = offline ? 'bad' : (d.level === 'ok' ? 'ok' : 'warn')
    if (d.level === 'ok' && d.summaryPingMs >= 0) {
      const msLabel = d.summaryPingMs < 1 ? '<1' : fmt(d.summaryPingMs, 0)
      text = t('netStableSummary', { ms: msLabel })
    }
    const details = netDetailRows(d)
    box.innerHTML = `
      <div class="verdict-card ${verdictCls}"><div class="v-title">${escapeHtml(text)}</div></div>
      ${details || ''}`
    if (!uiExpand.tiles.has('net-lags')) {
      uiExpand.tiles.add('net-lags')
      syncNetAccordionChrome()
    }
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

function hudTone(pct) {
  const v = Number(pct) || 0
  if (v >= 90) return 'hot'
  if (v >= 80) return 'warn'
  return 'ok'
}

function renderMiniHUD(m) {
  if (!m) return
  const set = (id, pct) => {
    const el = $(id)
    if (!el) return
    el.textContent = `${fmt(pct, 0)}%`
    el.classList.remove('ok', 'warn', 'hot')
    el.classList.add(hudTone(pct))
  }
  set('mh-cpu', m.cpuPercent)
  set('mh-ram', m.ramPercent)
  set('mh-gpu', m.gpuPercent)
  const cpuT = Number(m.temperatureC || m.cpuTempC || m.tempCPU || m.hardware?.cpu?.temperatureC || 0)
  const gpuT = Number(m.gpuTemperatureC || m.gpuTempC || m.tempGPU || m.hardware?.gpu?.temperatureC || 0)
  const cpuKnown = !!(m.temperatureAvailable || (Number.isFinite(cpuT) && cpuT > 0))
  const gpuKnown = !!(m.gpuTemperatureAvailable || m.hardware?.gpu?.tempAvailable || (Number.isFinite(gpuT) && gpuT > 0))
  const paintTemp = (id, show, value) => {
    const el = $(id)
    if (!el) return
    if (!show || !(value > 0)) {
      el.textContent = ''
      el.classList.add('hidden')
      return
    }
    el.classList.remove('hidden')
    el.textContent = ` · ${fmt(value, 0)}°C`
    el.classList.remove('ok', 'warn', 'hot')
    el.classList.add(value >= 90 ? 'hot' : value >= 80 ? 'warn' : 'ok')
  }
  paintTemp('mh-cpu-temp', cpuKnown, cpuT)
  paintTemp('mh-gpu-temp', gpuKnown, gpuT)
  if (hudEnabled) {
    const box = $('minihud')
    if (box && !box.classList.contains('hidden')) {
      const w = Math.ceil(box.scrollWidth + 24)
      const h = Math.ceil(box.scrollHeight + 8)
      if (w > 40 && h > 20) {
        const prevW = renderMiniHUD._w || 0
        const prevH = renderMiniHUD._h || 0
        if (Math.abs(w - prevW) > 2 || Math.abs(h - prevH) > 2) {
          renderMiniHUD._w = w
          renderMiniHUD._h = h
          try { ResizeMiniHUD(w, h) } catch (_) { /* ignore */ }
        }
      }
    }
  }
}

function syncSettings(m) {
  document.querySelectorAll('.btn.mode').forEach((btn) => {
    btn.classList.toggle('active', btn.dataset.mode === (m.powerMode || 'balanced'))
  })
  syncPowerRadiogroup(m.powerMode || 'balanced')
  document.querySelectorAll('.btn.poll').forEach((btn) => {
    btn.classList.toggle('active', Number(btn.dataset.poll) === Number(m.pollIntervalMS || 1000))
  })
}

function deltaText(cur, prev, suffix = '') {
  if (prev == null || Number.isNaN(Number(prev))) return ''
  const d = Number(cur) - Number(prev)
  const sign = d > 0 ? '+' : ''
  return `${sign}${fmt(d, 1)}${suffix}`
}

async function renderSummary() {
  const status = $('summary-status')
  const cards = $('summary-cards')
  const canvas = $('summary-chart')
  const legend = $('summary-legend')
  if (!status || !cards) return
  let s
  try {
    s = await GetWeeklySummary()
  } catch (_) {
    status.textContent = t('noData')
    return
  }
  if (s.insufficient) {
    status.className = 'status-banner ok'
    status.textContent = resultMsg(s, 'noData')
  } else {
    status.className = 'status-banner ok'
    status.textContent = resultMsg(s) || t('statsObserving', { days: s.daysObserved || 0 })
  }
  const items = [
    { label: t('summaryGameHours'), value: fmt(s.gameHours, 1), delta: deltaText(s.gameHours, s.prevGameHours, ' h') },
    { label: t('summaryFreezes'), value: String(s.freezes || 0), delta: deltaText(s.freezes, s.prevFreezes) },
    { label: t('summaryCPUPeak'), value: s.cpuTempPeak ? fmt(s.cpuTempPeak, 0) : '-', delta: deltaText(s.cpuTempPeak, s.prevCPUTempPeak, '°') },
    { label: t('summaryGPUPeak'), value: s.gpuTempPeak ? fmt(s.gpuTempPeak, 0) : '-', delta: deltaText(s.gpuTempPeak, s.prevGPUTempPeak, '°') },
    { label: t('summaryStable'), value: s.stableDay || '-' },
    { label: t('summaryCulprit'), value: s.topCulprit || '-' },
    { label: t('summaryEcoRAM'), value: `${fmt(s.ecoRAMFreedMB, 0)} MB`, delta: deltaText(s.ecoRAMFreedMB, s.prevEcoRAMMB, ' MB') },
    { label: t('summaryEcoCount'), value: String(s.ecoCount || 0), delta: deltaText(s.ecoCount, s.prevEcoCount) },
    { label: t('summaryHighLoad'), value: fmt(s.highLoadMin, 0), delta: deltaText(s.highLoadMin, s.prevHighLoadMin) },
  ]
  cards.innerHTML = items.map((it) => `
    <div class="summary-card">
      <div class="label">${it.label}</div>
      <div class="value">${escapeHtml(it.value)}</div>
      ${it.delta ? `<div class="delta">${escapeHtml(it.delta)}</div>` : ''}
    </div>
  `).join('')
  drawSummaryChart(canvas, s.series || [])
  if (legend) legend.textContent = `${t('summaryCPUPeak')} / ${t('summaryFreezes')} / ${t('summaryGameHours')}`
}

function drawSummaryChart(canvas, series) {
  if (!canvas) return
  const ctx = canvas.getContext('2d')
  if (!ctx) return
  const w = canvas.width
  const h = canvas.height
  ctx.clearRect(0, 0, w, h)
  ctx.fillStyle = 'rgba(20,24,32,0.9)'
  ctx.fillRect(0, 0, w, h)
  if (!series.length) {
    ctx.fillStyle = '#94a3b8'
    ctx.fillText(t('noData'), 12, h / 2)
    return
  }
  const pad = 14
  const plotW = w - pad * 2
  const plotH = h - pad * 2
  const n = series.length
  const maxTemp = Math.max(60, ...series.map((d) => Number(d.cpuTempMax) || 0), ...series.map((d) => Number(d.gpuTempMax) || 0))
  const maxFreeze = Math.max(1, ...series.map((d) => Number(d.incidents) || 0))
  const drawLine = (vals, color, maxV) => {
    ctx.beginPath()
    ctx.strokeStyle = color
    ctx.lineWidth = 1.6
    vals.forEach((v, i) => {
      const x = pad + (i / Math.max(1, n - 1)) * plotW
      const y = pad + plotH - (Math.min(maxV, Math.max(0, v)) / maxV) * plotH
      if (i === 0) ctx.moveTo(x, y)
      else ctx.lineTo(x, y)
    })
    ctx.stroke()
  }
  drawLine(series.map((d) => Number(d.cpuTempMax) || 0), '#f87171', maxTemp)
  drawLine(series.map((d) => Number(d.incidents) || 0), '#fbbf24', maxFreeze)
  // game minutes as small bars
  const maxGame = Math.max(1, ...series.map((d) => Object.values(d.gameMinutes || {}).reduce((a, b) => a + Number(b || 0), 0)))
  series.forEach((d, i) => {
    const mins = Object.values(d.gameMinutes || {}).reduce((a, b) => a + Number(b || 0), 0)
    const x = pad + (i / Math.max(1, n - 1)) * plotW
    const bh = (mins / maxGame) * (plotH * 0.35)
    ctx.fillStyle = 'rgba(56,189,248,0.45)'
    ctx.fillRect(x - 4, pad + plotH - bh, 8, bh)
  })
}

async function copySummaryPng() {
  const canvas = $('summary-chart')
  if (!canvas) return
  try {
    const blob = await new Promise((resolve) => canvas.toBlob(resolve, 'image/png'))
    if (!blob) throw new Error('blob')
    if (navigator.clipboard && window.ClipboardItem) {
      await navigator.clipboard.write([new ClipboardItem({ 'image/png': blob })])
      showToast(t('summaryCopied'), true, { holdMs: 4000 })
      return
    }
    throw new Error('no clipboard')
  } catch (_) {
    try {
      const a = document.createElement('a')
      a.href = canvas.toDataURL('image/png')
      a.download = `pulsecore-summary-${Date.now()}.png`
      a.click()
      showToast(t('summaryCopyFail'), false, { holdMs: 5000 })
    } catch (err) {
      showToast(t('summaryCopyFail'), false, { holdMs: 5000 })
    }
  }
}

function applyMetrics(m) {
  if (!m) return
  lastMetrics = m
  if (m.language && m.language !== lang && !localStorage.getItem('zp_lang')) {
    lang = m.language
    applyI18n()
  }
  pushSample('cpu', m.cpuPercent || 0)
  pushSample('gpu', m.gpuPercent || 0)
  pushSample('ram', m.ramPercent || 0)
  pushSample('battery', batterySeriesValue(m))
  try { renderOverview(m) } catch (err) { console.error('[PulseCore] renderOverview', err) }
  try { renderLag(m) } catch (err) { console.error('[PulseCore] renderLag', err) }
  const activeTab = document.querySelector('.nav-item.active')?.dataset?.tab
  if (activeTab === 'home') {
    renderHome().catch(() => {})
  }
  if (activeTab === 'games' && !gameSearchFocused) {
    renderGamesTab().catch(() => {})
  }
  try { renderHardware(m) } catch (err) { console.error('[PulseCore] renderHardware', err) }
  updateHomeDiskSummary(m)
  try { renderMiniHUD(m) } catch (err) { console.error('[PulseCore] renderMiniHUD', err) }
  syncSettings(m)
  setHudMode(!!m.miniHUD)
  const hasBattery = !!(m.battery && m.battery.available)
  syncDeviceKind(hasBattery)
}

function updateHomeDiskSummary(m) {
  try {
    const sum = $('home-disk-sum')
    if (!sum) return
    const disks = m.disks || []
    if (!disks.length) {
      sum.innerHTML = ''
      return
    }
    sum.innerHTML = buildDiskPillsHTML(disks, (letter) => cachedDiskReport[letter], {
      t, msgText, fmt, escapeHtml, diskStatusText, diskHealthLabel, diskHistory,
    })
  } catch (err) {
    console.error('[PulseCore] disk pills', err)
    const sum = $('home-disk-sum')
    if (sum) sum.textContent = t('tileRenderFail', { section: t('tileSectionDisks') })
  }
}

function diskField(label, value, tip = '') {
  if (value == null || value === '') return ''
  const tipHtml = tip
    ? ` <span class="tip" title="${escapeHtml(tip)}" tabindex="0" aria-label="${escapeHtml(tip)}">?</span>`
    : ''
  return `<div class="modal-row"><span class="k">${label}${tipHtml}</span><span class="v">${escapeHtml(String(value))}</span></div>`
}

function formatHostTB(gb) {
  if (!(gb > 0)) return null
  const tb = gb / 1024
  if (tb >= 0.01) return `${fmt(tb, 2)} TB`
  return `${fmt(gb, 1)} GB`
}

function renderSmartTable(attrs, highlightId = 0) {
  if (!attrs || !attrs.length) return ''
  const rows = attrs.map((a) => {
    const name = lang === 'en' ? (a.name || '') : (a.nameRu || a.name || '')
    const th = a.threshold > 0 ? String(a.threshold) : ''
    const hl = highlightId && a.id === highlightId ? ' class="smart-row-hl"' : ''
    return `<tr${hl} data-smart-id="${a.id}">
      <td>${a.id}</td><td>${escapeHtml(name)}</td><td>${a.current}</td><td>${a.worst}</td>
      <td>${th}</td><td>${escapeHtml(a.raw || '')}</td>
    </tr>`
  }).join('')
  return `<div class="panel-title" style="margin-top:12px">${t('smartTable')}</div>
    <div class="smart-table-wrap"><table class="smart-table">
      <thead><tr><th>ID</th><th>${t('smartName')}</th><th>${t('smartCurrent')}</th><th>${t('smartWorst')}</th><th>${t('smartThresh')}</th><th>Raw</th></tr></thead>
      <tbody>${rows}</tbody>
    </table></div>`
}

function mediaBadge(r) {
  const media = String(r?.mediaType || '').trim()
  const rpm = Number(r?.rotationRPM) || 0
  if (media === 'HDD' || r?.mediaKind === 'hdd') {
    if (rpm > 1) return `HDD · ${t('unitsRpm', { n: fmt(rpm, 0) })}`
    return 'HDD'
  }
  if (media === 'SSD' || r?.mediaKind === 'nvme' || r?.mediaKind === 'sata_ssd') return 'SSD'
  if (media && media !== 'NVMe SSD' && !/не определён|unknown/i.test(media)) {
    if (/nvme/i.test(media)) return 'SSD'
    return media
  }
  return t('mediaUnknown')
}

function renderDiskCards(disks) {
  const box = $('ov-disks')
  if (!box) return
  if (!disks || !disks.length) {
    box.innerHTML = ''
    return
  }
  const expanded = new Set(uiExpand.disks)
  const built = buildDiskCardsHTML(disks, (letter) => cachedDiskReport[letter], {
    t, msgText, fmt, escapeHtml, diskStatusText,
    formatCapacityGB, mediaBadge, chevronSVG, expanded,
  })
  box.innerHTML = built.html

  const banner = $('disk-admin-banner')
  const bannerText = $('disk-admin-text')
  if (banner && bannerText) {
    // Disk admin banner only when advanced mode is OFF - never nag when enabled/starting/error.
    const phase = advancedPhase(lastAdvancedState)
    const needAdv = built.needsAdmin && phase === 'off'
    if (needAdv) {
      banner.classList.remove('hidden')
      bannerText.textContent = lang === 'en'
        ? (built.adminEn || t('diskAdminBanner'))
        : (built.adminRu || t('diskAdminBanner'))
      const btn = $('btn-disk-admin')
      if (btn) btn.textContent = t('advEnable')
    } else {
      banner.classList.add('hidden')
    }
  }
  expanded.forEach((letter) => {
    try { fillDiskDetails(letter) } catch (err) { console.error('[PulseCore] fillDiskDetails', letter, err) }
  })
}

function fillDiskDetails(letter) {
  const host = document.querySelector(`[data-drive-details="${letter}"]`)
  const r = cachedDiskReport[letter]
  if (!host) return
  if (!r) {
    host.innerHTML = `<div class="disk-loading"><span class="disk-spinner" aria-hidden="true"></span><span>${t('diskChecking')}</span></div>`
    return
  }
  if (r._loading) {
    host.innerHTML = `<div class="disk-loading"><span class="disk-spinner" aria-hidden="true"></span><span>${t('diskChecking')}</span></div>`
    return
  }
  const src = msgText(r.healthSourceMsg, r.healthSourceRu, r.healthSourceEn)
  const reason = msgText(r.statusReason, r.statusReasonRu, r.statusReasonEn)
  const written = formatHostTB(r.hostWriteGB || r.tbwGB)
  const read = formatHostTB(r.hostReadGB)
  const cap = r.capacityGB > 0 ? formatCapacityGB(r.capacityGB) : null
  const temp = r.tempAvailable && r.temperatureC > 0 ? `${fmt(r.temperatureC, 0)} °C` : null
  const checked = r.checkedAt ? formatDiskCheckedAt(new Date(r.checkedAt)) : (diskHistory[letter]?.lastChecked || null)
  const rpm = r.rotationRPM > 1 ? t('unitsRpm', { n: fmt(r.rotationRPM, 0) }) : null
  const iface = r.busInterface || r.interfaceType || null
  const media = mediaBadge(r)
  let health = null
  let healthTip = ''
  if (r.healthPercentOK) {
    health = `${fmt(r.healthPercent, 0)}%`
    if (r.mediaKind === 'hdd' || r.mediaType === 'HDD') healthTip = t('diskHealthTipHDD')
  } else if (r.mediaKind === 'hdd' || r.mediaType === 'HDD') {
    health = src || t('diskHealthUnavailable')
    healthTip = t('diskHealthTipHDD')
  }
  const missSum = lang === 'en' ? r.missingSummaryEn : r.missingSummaryRu
  const statusLine = reason
    ? `${diskStatusText(r)} - ${reason}`
    : diskStatusText(r)
  const infoNote = (r.infoNote && (r.infoNote.code || r.infoNote.Code))
    ? msgText(r.infoNote, '', '')
    : ''
  const hlId = Number(r.triggerAttrId) || 0
  host.innerHTML = `
    ${diskField(t('lblStatus'), statusLine)}
    ${diskField(t('lblModel'), r.model)}
    ${diskField(t('serial'), r.serialNumber)}
    ${diskField(t('firmware'), r.firmware)}
    ${diskField(t('ifaceLabel') || t('iface'), iface)}
    ${diskField(t('mediaLabel') || t('mediaUnknown'), media)}
    ${rpm ? diskField(t('rpmLabel'), rpm) : ''}
    ${diskField(t('capacity'), cap)}
    ${diskField(t('temp'), temp)}
    ${diskField(t('diskHealthPct'), health, healthTip)}
    ${diskField(t('healthSource'), src)}
    ${diskField(t('diskWritten'), written)}
    ${diskField(t('diskRead'), read)}
    ${diskField(t('powerOn'), r.powerOnHours > 0 ? `${fmt(r.powerOnHours, 0)} ${t('hoursShort')}` : null)}
    ${diskField(t('powerCycles'), r.powerCycles > 0 ? fmt(r.powerCycles, 0) : null)}
    ${diskField(t('realloc'), (r.attributes && r.attributes.length) || r.reallocatedSectors != null ? String(r.reallocatedSectors || 0) : null)}
    ${diskField(t('pendingSec'), (r.attributes && r.attributes.length) || r.pendingSectors != null ? String(r.pendingSectors || 0) : null)}
    ${diskField(t('uncorrSec'), (r.attributes && r.attributes.length) ? String(r.uncorrectableSectors || 0) : null)}
    ${diskField(t('lastCheck'), checked)}
    ${infoNote ? `<div class="disk-info-note">${escapeHtml(infoNote)}</div>` : ''}
    ${renderSmartTable(r.attributes, hlId)}
    ${missSum ? `<div class="disk-missing-summary">${escapeHtml(missSum)}</div>` : ''}
  `
}

let diskSmartLoading = false
async function refreshAllDiskSmart(force = false) {
  if (diskSmartLoading) return
  const disks = latestDisks || []
  if (!disks.length) return
  diskSmartLoading = true
  try {
    const letters = disks.map((d) => driveLetter(d.mount))
    // Show skeleton cards immediately
    letters.forEach((key) => {
      if (!cachedDiskReport[key]) cachedDiskReport[key] = { _loading: true, statusLevel: 'unknown', statusWordRu: t('diskStateUnknown'), statusWordEn: 'Unknown' }
    })
    renderDiskCards(disks)
    updateHomeDiskSummary({ disks })

    let results = []
    try {
      if (force && typeof RefreshAllDiskHealth === 'function') {
        results = await RefreshAllDiskHealth(letters)
      } else {
        results = await Promise.all(letters.map(async (key) => {
          if (!force && cachedDiskReport[key] && !cachedDiskReport[key]._loading && cachedDiskReport[key].ok) {
            return cachedDiskReport[key]
          }
          try {
            return force ? await CheckDiskHealthForce(key) : await CheckDiskHealth(key)
          } catch (_) {
            return { driveLetter: key + ':', ok: false, statusLevel: 'unknown' }
          }
        }))
      }
    } catch (_) {
      results = []
    }
    results.forEach((r, i) => {
      const key = letters[i] || driveLetter(r?.driveLetter)
      if (!r) return
      cachedDiskReport[key] = r
      if (r.ok) saveDiskHistory(key, r)
    })
    renderDiskCards(disks)
    updateHomeDiskSummary({ disks })
  } finally {
    diskSmartLoading = false
  }
}

function setHudMode(enabled) {
  hudEnabled = !!enabled
  if (enabled) setGameOverlayMode(false, true)
  else {
    // Allow a fresh measure next time HUD opens (still grow-only while open).
    renderMiniHUD._w = 0
    renderMiniHUD._h = 0
  }
  document.body.classList.toggle('mini-hud-active', enabled)
  document.documentElement.classList.toggle('mini-hud-active', enabled)
  const hud = $('minihud')
  hud?.classList.toggle('hidden', !enabled)
  // HUD stays visually dark; do not flip capsule to light theme tokens.
  if (hud) {
    hud.removeAttribute('title')
    const op = hud.dataset.opacity || '80'
    hud.dataset.opacity = op
  }
  if (enabled || gameOverlayOn) {
    $('shell').classList.add('hidden')
    $('titlebar').classList.add('hidden')
  } else {
    $('shell').classList.remove('hidden')
    restoreTitlebarDrag()
    $('mh-menu')?.classList.add('hidden')
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
    restoreTitlebarDrag()
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

let hiddenGames = new Set()
let hiddenGamesMeta = []

/** Shared human title path: metadata → catalog → careful slug → id. Never invent covers. */
function humanizeGameSlug(raw) {
  const s = String(raw || '').trim()
  if (!s) return ''
  // Prefer already-human titles.
  if (/[ A-ZА-ЯЁ]/.test(s) && !/^[a-z0-9]+(?:[_-][a-z0-9]+)+$/i.test(s)) return s
  return s
    .replace(/^app[:_-]?/i, '')
    .replace(/[_]+/g, ' ')
    .replace(/[-]+/g, ' ')
    .replace(/\s+/g, ' ')
    .trim()
    .replace(/\b([a-zа-яё])/gi, (ch) => ch.toUpperCase())
}

function foldGameName(s) {
  return String(s || '').toLowerCase().replace(/[^a-z0-9а-яё]+/gi, '')
}

function strictTitleMatch(a, b) {
  const x = foldGameName(a)
  const y = foldGameName(b)
  return !!x && x === y
}

function looksLikeSlug(name, id) {
  const n = String(name || '').trim()
  const i = String(id || '').trim()
  if (!n) return true
  if (i && n.toLowerCase() === i.toLowerCase()) return true
  if (/^[a-z0-9]+(?:[_-][a-z0-9]+)+$/i.test(n) && n === n.toLowerCase()) return true
  return false
}

function gameDisplayName(g) {
  if (!g) return ''
  const id = String(g.id || g.ID || '').trim()
  const meta = String(g.name || g.Name || g.title || g.Title || '').trim()
  if (meta && !looksLikeSlug(meta, id)) return meta
  const fromCatalog = lookupCatalogGame(id)
  if (fromCatalog?.name && !looksLikeSlug(fromCatalog.name, id)) return fromCatalog.name
  if (meta) return humanizeGameSlug(meta)
  return humanizeGameSlug(id) || id
}

async function enrichHiddenGameTitles(list) {
  const out = []
  for (const h of list || []) {
    const id = String(h?.id || '').trim()
    if (!id) continue
    let name = String(h?.name || '').trim()
    let poster = String(h?.posterUrl || h?.PosterURL || '').trim()
    let year = Number(h?.year || h?.Year) || 0
    const catalog = lookupCatalogGame(id)
    if (catalog) {
      if (!name || looksLikeSlug(name, id)) name = catalog.name || name
      if (!poster) poster = catalog.posterUrl || catalog.headerUrl || ''
      if (!year) year = Number(catalog.year) || 0
    }
    if ((!name || looksLikeSlug(name, id) || !poster) && typeof ResolveGameMeta === 'function') {
      try {
        const meta = await ResolveGameMeta(name && !looksLikeSlug(name, id) ? name : id)
        if (meta?.name && (!name || looksLikeSlug(name, id))) name = meta.name
        if (!poster && meta?.name && strictTitleMatch(meta.name, name) && (meta.posterUrl || meta.headerUrl)) {
          poster = meta.posterUrl || meta.headerUrl || ''
        }
        if (!year && meta?.year) year = Number(meta.year) || 0
      } catch (_) { /* offline / fail closed to placeholder */ }
    }
    out.push({
      id,
      name: gameDisplayName({ id, name }),
      posterUrl: poster,
      headerUrl: '',
      year,
    })
  }
  return out
}

async function syncHiddenGamesFromBackend() {
  try {
    const list = await GetUserHiddenGames() || []
    hiddenGamesMeta = await enrichHiddenGameTitles(Array.isArray(list) ? list : [])
    hiddenGames = new Set(hiddenGamesMeta.map((h) => String(h?.id || '').toLowerCase()).filter(Boolean))
    // One-shot migrate legacy localStorage list into prefs.
    try {
      const legacy = JSON.parse(localStorage.getItem('zp_hidden_games') || '[]')
      if (Array.isArray(legacy) && legacy.length) {
        for (const id of legacy) {
          const sid = String(id || '').trim()
          if (!sid || hiddenGames.has(sid.toLowerCase())) continue
          const g = lookupCatalogGame(sid)
          const title = gameDisplayName(g || { id: sid, name: sid })
          const poster = g?.posterUrl || g?.headerUrl || ''
          const year = Number(g?.year) || 0
          if (typeof HideUserGameMeta === 'function') await HideUserGameMeta(sid, title, poster, year)
          else await HideUserGame(sid, title)
        }
        localStorage.removeItem('zp_hidden_games')
        const refreshed = await GetUserHiddenGames() || []
        hiddenGamesMeta = await enrichHiddenGameTitles(Array.isArray(refreshed) ? refreshed : [])
        hiddenGames = new Set(hiddenGamesMeta.map((h) => String(h?.id || '').toLowerCase()).filter(Boolean))
      }
    } catch (_) { /* ignore */ }
  } catch (_) {
    hiddenGamesMeta = []
    hiddenGames = new Set()
  }
  updateHiddenGamesButton()
}

function updateHiddenGamesButton() {
  const btn = $('btn-hidden-games')
  if (!btn) return
  const n = hiddenGamesMeta.length
  btn.classList.toggle('hidden', n <= 0)
  btn.textContent = t('gameHiddenBtn', { n })
}

/** Normalize game objects so missing covers/fields never crash render/search. */
function normalizeGameObj(g) {
  if (!g || typeof g !== 'object') return null
  const id = String(g.id ?? g.ID ?? '').trim()
  const name = String(g.name ?? g.Name ?? g.title ?? g.Title ?? '').trim()
  if (!id && !name) return null
  return {
    id: id || `name:${name.toLowerCase()}`,
    name: name || id,
    title: name || id,
    genre: String(g.genre ?? g.Genre ?? ''),
    weight: Number(g.weight ?? g.Weight) || 0,
    steamAppId: Number(g.steamAppId ?? g.SteamAppID) || 0,
    posterUrl: String(g.posterUrl ?? g.PosterURL ?? ''),
    headerUrl: String(g.headerUrl ?? g.HeaderURL ?? ''),
    category: String(g.category ?? g.Category ?? ''),
    year: Number(g.year ?? g.Year) || 0,
    badge: String(g.badge ?? g.Badge ?? ''),
    engine: String(g.engine ?? g.Engine ?? ''),
    installPath: String(g.installPath ?? g.InstallPath ?? ''),
    fromSteam: !!(g.fromSteam ?? g.FromSteam),
  }
}

function normalizeGameList(list) {
  const out = []
  const seen = new Set()
  for (const raw of list || []) {
    const g = normalizeGameObj(raw)
    if (!g || !g.name) continue
    const title = String(g.name || g.title || '').trim()
    if (!title) continue
    const hid = g.id.toLowerCase()
    if (hiddenGames.has(hid)) continue
    if (seen.has(hid)) continue
    seen.add(hid)
    out.push(g)
  }
  return out
}

function gameMatchesQuery(g, qRaw) {
  const q = String(qRaw || '').trim().toLocaleLowerCase()
  if (!q || !g) return true
  const blob = [g.name, g.id, g.genre, g.badge, g.category, g.engine]
    .map((x) => String(x || '').toLocaleLowerCase())
    .join(' ')
  return blob.includes(q)
}

function applyGameFilters() {
  const q = String($('game-search')?.value ?? currentSearchQuery ?? '').trim()
  currentSearchQuery = q
  const base = normalizeGameList(gameCatalogFull || [])
  if (!q) {
    gameCatalog = base
    return
  }
  const narrowed = base.filter((g) => gameMatchesQuery(g, q))
  const extra = (gameCatalog || []).filter((g) => gameMatchesQuery(g, q) && !narrowed.some((x) => x.id === g.id))
  gameCatalog = narrowed.concat(extra)
}

async function loadGames() {
  await syncHiddenGamesFromBackend()
  const token = gameSearchToken
  try {
    gameCatalogFull = normalizeGameList(await ListGames() || [])
  } catch (_) {
    gameCatalogFull = gameCatalogFull || []
  }
  if (token !== gameSearchToken && currentSearchQuery) {
    applyGameFilters()
    renderGameGrid()
    return
  }
  applyGameFilters()
  renderGameGrid()
}

function setSearchSpinner(on) {
  $('game-search-spinner')?.classList.toggle('hidden', !on)
}

function mergeGameLists(localList, onlineList) {
  const out = []
  const seen = new Set()
  for (const raw of [...(localList || []), ...(onlineList || [])]) {
    const g = normalizeGameObj(raw)
    if (!g || !g.title) continue
    if (hiddenGames.has(g.id.toLowerCase())) continue
    const key = String(g.steamAppId > 0 ? `app:${g.steamAppId}` : g.id).toLowerCase()
    if (seen.has(key)) continue
    seen.add(key)
    out.push(g)
  }
  return out
}

async function searchGamesLocal(q, token) {
  try {
    if (!q) {
      gameCatalogFull = normalizeGameList(await ListGames() || [])
      if (token != null && token !== gameSearchToken) return
      gameCatalog = gameCatalogFull
    } else {
      const remote = normalizeGameList(await SearchGames(q) || [])
      if (token != null && token !== gameSearchToken) return
      gameCatalog = remote.filter((g) => gameMatchesQuery(g, q))
    }
  } catch (_) {
    if (token != null && token !== gameSearchToken) return
    applyGameFilters()
  }
  renderGameGrid()
  if (gameReqFit && gameReqFit !== 'all') warmCompatScores(gameCatalog)
}

async function searchGamesOnlineMerge(q, token) {
  if (!q) return
  setSearchSpinner(true)
  try {
    const online = await SearchGamesOnline(q) || []
    const local = await SearchGames(q) || []
    if (token != null && token !== gameSearchToken) return
    gameCatalog = mergeGameLists(local, online).filter((g) => gameMatchesQuery(g, q))
  } catch (_) {
    // keep current local results
  } finally {
    if (token == null || token === gameSearchToken) setSearchSpinner(false)
  }
  if (token != null && token !== gameSearchToken) return
  renderGameGrid()
  if (gameCatalog.length === 1) {
    const onlyId = gameCatalog[0]?.id
    if (!onlyId) return
    if (selectedGameId === onlyId) {
      showFpsDetail()
      await predictSelectedGame()
    } else {
      await selectGame(onlyId)
    }
  }
}

async function searchGamesLive(opts = {}) {
  const online = !!opts.online
  const token = ++gameSearchToken
  const q = String($('game-search')?.value || '').trim()
  currentSearchQuery = q
  if (!q) {
    setSearchSpinner(false)
    try {
      gameCatalogFull = normalizeGameList(await ListGames() || [])
      if (token !== gameSearchToken) return
      gameCatalog = gameCatalogFull
    } catch (_) {
      if (token !== gameSearchToken) return
      gameCatalog = []
    }
    renderGameGrid()
    return
  }
  applyGameFilters()
  renderGameGrid()
  await searchGamesLocal(q, token)
  if (token !== gameSearchToken) return
  if (online || gameCatalog.length === 0) {
    await searchGamesOnlineMerge(q, token)
  }
}

function genreGradient(genreOrCategory) {
  const g = String(genreOrCategory || '').toLowerCase()
  if (/(horror|survival)/.test(g)) return 'linear-gradient(145deg,#1a0b18 0%,#4a1740 45%,#0d2a2e 100%)'
  if (/(racing|sports)/.test(g)) return 'linear-gradient(145deg,#07141f 0%,#0b3d5c 50%,#0f6b5c 100%)'
  if (/(shooter|fps|esports|moba)/.test(g)) return 'linear-gradient(145deg,#0a1024 0%,#1a2a6c 48%,#0e4d7a 100%)'
  if (/(rpg|adventure|aaa|action|open)/.test(g)) return 'linear-gradient(145deg,#12081f 0%,#3b1d6e 50%,#0f3d4a 100%)'
  if (/(indie|classic|sim)/.test(g)) return 'linear-gradient(145deg,#071a14 0%,#145a45 50%,#1a3a5c 100%)'
  return 'linear-gradient(145deg,#0b1220 0%,#1e3a8a 46%,#312e81 100%)'
}

function posterFallbackHTML(name, cls, year = 0, genre = '', opts = {}) {
  const hideTitle = !!opts.hideTitle
  const safe = String(name || 'Game').replace(/</g, '&lt;').replace(/>/g, '&gt;')
  const y = !hideTitle && Number(year) > 0 ? `<span class="poster-year">${Number(year)}</span>` : ''
  const title = hideTitle ? '' : `<span class="poster-title">${safe}</span>`
  const bg = genreGradient(genre)
  return `<div class="${cls} fallback glass-cover" style="background:${bg}" aria-hidden="true">${title}${y}</div>`
}

function steamHeaderFromPoster(url) {
  if (!url) return ''
  return String(url)
    .replace('/library_600x900.jpg', '/header.jpg')
    .replace('/library_600x900_2x.jpg', '/header.jpg')
}

function replaceBrokenPoster(img, name, year, genre, cls) {
  try {
    const wrap = document.createElement('div')
    wrap.innerHTML = posterFallbackHTML(name, cls, year, genre)
    const node = wrap.firstElementChild
    if (node && img?.parentNode) img.replaceWith(node)
  } catch (_) {
    if (img) img.classList.add('hidden')
  }
}

function gamePosterHTML(g) {
  const game = normalizeGameObj(g) || { name: 'Game', posterUrl: '', headerUrl: '', category: '', genre: '', badge: '', year: 0 }
  const name = String(game.name || 'Game').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;')
  const genre = `${game.category || ''} ${game.genre || ''} ${game.badge || ''}`
  const year = Number(game.year) || 0
  const primary = game.posterUrl || ''
  const secondary = game.headerUrl || steamHeaderFromPoster(primary)
  if (!primary && !secondary) return posterFallbackHTML(game.name, 'game-poster', year, genre)
  const src = primary || secondary
  const fb = secondary && secondary !== src ? secondary : ''
  const payload = JSON.stringify({ name: game.name || 'Game', year, genre, cls: 'game-poster' })
  return `<img class="game-poster" src="${src}" alt="${name}" loading="lazy" data-fb="${fb}" data-fallback='${payload.replace(/'/g, '&#39;')}' onerror="window.__pcPosterError && window.__pcPosterError(this)" />`
}

function compactCoverHTML(p, opts = {}) {
  const hideTitle = !!opts.hideTitle
  const name = String(p?.gameName || p?.name || 'Game')
  const genre = String(p?.category || p?.genre || '')
  const year = Number(p?.year) || 0
  const primary = p?.posterUrl || ''
  const secondary = p?.headerUrl || steamHeaderFromPoster(primary)
  if (!primary && !secondary) return posterFallbackHTML(name, 'game-cover', year, genre, { hideTitle })
  const src = primary || secondary
  const fb = secondary && secondary !== src ? secondary : ''
  const payload = JSON.stringify({ name, year, genre, cls: 'game-cover', hideTitle })
  return `<img class="game-cover" src="${src}" alt="" data-fb="${fb}" data-fallback='${payload.replace(/'/g, '&#39;')}' onerror="window.__pcPosterError && window.__pcPosterError(this)" />`
}

function genreBucket(g) {
  const cat = String(g.category || '').toLowerCase()
  const genre = String(g.genre || '').toLowerCase()
  const badge = String(g.badge || '').toLowerCase()
  const blob = `${cat} ${genre} ${badge} ${String(g.name || '').toLowerCase()}`
  if (/(shooter|fps|esports|counter|valorant|cs2)/.test(blob) && /esports|moba|cs2|valorant|dota/.test(blob)) return 'esports'
  if (/(shooter|fps|call of duty|battlefield)/.test(blob)) return 'shooter'
  if (/(racing|forza|need for speed|assetto)/.test(blob)) return 'racing'
  if (/(horror|survival|resident|outlast|dead space)/.test(blob)) return 'horror'
  if (/(open world|action|aaa|adventure|rpg|gta|cyberpunk|rdr)/.test(blob)) return 'action'
  if (cat === 'shooter') return 'shooter'
  if (cat === 'moba' || badge.includes('esports')) return 'esports'
  return cat || 'action'
}

function sortGames(list) {
  const out = [...list]
  if (gameSort === 'name') {
    out.sort((a, b) => String(a.name || '').localeCompare(String(b.name || ''), lang, { sensitivity: 'base' }))
  } else if (gameSort === 'year') {
    out.sort((a, b) => (Number(b.year) || 0) - (Number(a.year) || 0))
  } else {
    out.sort((a, b) => (Number(b.weight) || 0) - (Number(a.weight) || 0))
  }
  return out
}

function filteredGameList() {
  let list = normalizeGameList(gameCatalog || [])
  if (gameGenre !== 'all') {
    list = list.filter((g) => genreBucket(g) === gameGenre)
  }
  if (gameReqFit && gameReqFit !== 'all') {
    list = list.filter((g) => {
      const hit = fpsCache[g.id]
      if (!hit || !hit.verdictLevel) return true // not scored yet - keep until warm
      return hit.verdictLevel === gameReqFit
    })
  }
  return sortGames(list)
}

function syncGamePanelPadding(open) {
  const wrap = $('games-carousel-wrapper')
  const grid = $('game-grid')
  if (!wrap) return
  wrap.classList.toggle('has-selection', !!open)
  grid?.classList.toggle('has-selection', !!open)
  if (open) {
    wrap.style.setProperty('--game-panel-h', '165px')
    wrap.style.paddingBottom = '165px'
  } else {
    wrap.style.removeProperty('--game-panel-h')
    wrap.style.removeProperty('padding-bottom')
  }
}

function renderGameGrid() {
  const host = $('game-grid')
  if (!host) return
  const typed = String($('game-search')?.value ?? currentSearchQuery ?? '').trim()
  currentSearchQuery = typed
  let list = filteredGameList()
  if (typed) list = list.filter((g) => gameMatchesQuery(g, typed))
  const q = typed
  if (!list.length) {
    host.innerHTML = q || gameGenre !== 'all' || (gameReqFit && gameReqFit !== 'all')
      ? `<div class="game-empty-card" role="status">${t('gameSearchEmpty')}</div>`
      : `<div class="empty">${t('intelEmpty')}</div>`
    host.dataset.gridSig = ''
    return
  }
  const ids = list.map((g) => g?.id).filter(Boolean)
  const sig = `${ids.join('|')}::${gameSort}|${gameGenre}|${gameReqFit}|${q}`
  // Same catalog membership: update selection classes only — never rebuild posters mid-scroll.
  if (host.dataset.gridSig === sig && host.querySelector('.game-card')) {
    const want = String(selectedGameId || '')
    host.querySelectorAll('.game-card').forEach((card) => {
      const on = card.dataset.game === want
      card.classList.toggle('selected', on)
      card.classList.toggle('active', on)
    })
    return
  }
  host.dataset.gridSig = sig
  const frag = document.createDocumentFragment()
  const tmp = document.createElement('div')
  tmp.innerHTML = list.filter((g) => g && (g.name || g.title)).map((g) => {
    const safeName = String(g.name || g.title || '').replace(/</g, '&lt;').replace(/>/g, '&gt;')
    const titleAttr = String(g.name || g.title || '').replace(/"/g, '&quot;')
    const idAttr = String(g.id || '').replace(/"/g, '&quot;')
    if (!idAttr || !safeName) return ''
    return `
    <button type="button" class="game-card ${g.id === selectedGameId ? 'active selected' : ''}" data-game="${idAttr}" title="${titleAttr}">
      ${gamePosterHTML(g)}
      <div class="game-card-body">
        <div class="g-name title">${safeName}</div>
      </div>
    </button>`
  }).join('')
  while (tmp.firstChild) frag.appendChild(tmp.firstChild)
  host.replaceChildren(frag)
}

let gamePanelLastFocus = null

function hideFpsDetail() {
  const box = $('fps-result')
  if (!box) return
  box.classList.remove('open')
  box.classList.add('hidden')
  box.classList.add('empty')
  box.setAttribute('aria-hidden', 'true')
  box.removeAttribute('tabindex')
  box.innerHTML = ''
  syncGamePanelPadding(false)
  if (gamePanelLastFocus && typeof gamePanelLastFocus.focus === 'function') {
    try { gamePanelLastFocus.focus() } catch (_) { /* ignore */ }
  }
  gamePanelLastFocus = null
}

function showFpsDetail() {
  const box = $('fps-result')
  if (!box) return
  box.classList.remove('hidden')
  box.classList.remove('empty')
  box.classList.add('open')
  box.setAttribute('aria-hidden', 'false')
  syncGamePanelPadding(true)
}

async function hideSelectedGame() {
  if (!selectedGameId) return
  const id = String(selectedGameId)
  const g = lookupCatalogGame(id)
  const name = gameDisplayName(g || { id, name: id })
  const poster = g?.posterUrl || g?.headerUrl || ''
  const year = Number(g?.year) || 0
  try {
    if (typeof HideUserGameMeta === 'function') await HideUserGameMeta(id, name, poster, year)
    else await HideUserGame(id, name)
  } catch (_) { /* ignore */ }
  hiddenGames.add(id.toLowerCase())
  if (!hiddenGamesMeta.some((h) => String(h.id).toLowerCase() === id.toLowerCase())) {
    hiddenGamesMeta.push({ id, name, posterUrl: poster, year })
  }
  updateHiddenGamesButton()
  selectedGameId = null
  predictToken += 1
  document.querySelectorAll('.game-card.selected, .game-card.active').forEach((card) => {
    card.classList.remove('selected', 'active')
  })
  hideFpsDetail()
  gameCatalog = normalizeGameList(gameCatalog)
  gameCatalogFull = normalizeGameList(gameCatalogFull)
  const host = $('game-grid')
  if (host) host.dataset.gridSig = ''
  renderGameGrid()
}

function getGameInitials(rawTitle) {
  const clean = String(rawTitle || '')
    .replace(/\(.*?\)/g, '')
    .replace(/\[.*?\]/g, '')
    .replace(/[:\-–—_]/g, ' ')
    .trim()
  const stopWords = new Set(['the', 'a', 'an', 'of', 'and', 'in', 'for', 'to', 'on', 'at'])
  const words = clean.split(/\s+/).filter((w) => w && !stopWords.has(w.toLowerCase()))
  if (!words.length) return 'PC'
  if (words.length === 1) {
    const w = words[0]
    if (w.length <= 4) return w.toUpperCase()
    return w.slice(0, 3).toUpperCase()
  }
  if (words.length === 2) {
    return (words[0][0] + words[1][0]).toUpperCase()
  }
  return (words[0][0] + words[1][0] + words[2][0]).toUpperCase()
}

function openHiddenGamesModal() {
  const modal = $('hidden-games-modal')
  const list = $('hidden-games-list')
  if (!modal || !list) return
  if (!hiddenGamesMeta.length) {
    modal.classList.add('hidden')
    return
  }
  list.innerHTML = hiddenGamesMeta.map((h) => {
    const title = gameDisplayName(h)
    const year = Number(h.year) > 0 ? String(h.year) : ''
    const poster = h.posterUrl || h.headerUrl || ''
    const initials = getGameInitials(title)
    const thumbHTML = poster
      ? `<div class="hidden-game-thumb"><img src="${escapeHtml(poster)}" alt="${escapeHtml(title)}" loading="lazy" onerror="this.parentElement.innerHTML='<span class=\\'hidden-game-initials\\'>${escapeHtml(initials)}</span>'" /></div>`
      : `<div class="hidden-game-thumb"><span class="hidden-game-initials">${escapeHtml(initials)}</span></div>`
    return `
    <div class="hidden-game-row" data-hidden-id="${escapeHtml(h.id)}">
      ${thumbHTML}
      <div class="hidden-game-info">
        <h4>${escapeHtml(title)}</h4>
        ${year ? `<span>${escapeHtml(year)}</span>` : ''}
      </div>
      <button type="button" class="btn compact" data-restore-game="${escapeHtml(h.id)}">${t('gameRestore')}</button>
    </div>`
  }).join('')
  modal.classList.remove('hidden')
}

function closeHiddenGamesModal() {
  $('hidden-games-modal')?.classList.add('hidden')
}

async function restoreHiddenGame(id) {
  const sid = String(id || '').trim()
  if (!sid) return
  try { await RestoreHiddenGame(sid) } catch (_) { /* ignore */ }
  hiddenGames.delete(sid.toLowerCase())
  hiddenGamesMeta = hiddenGamesMeta.filter((h) => String(h.id).toLowerCase() !== sid.toLowerCase())
  updateHiddenGamesButton()
  if (!hiddenGamesMeta.length) closeHiddenGamesModal()
  else openHiddenGamesModal()
  await loadGames()
}

async function selectGame(gameId) {
  const id = String(gameId || '')
  if (!id) return
  if (selectedGameId === id) {
    selectedGameId = null
    predictToken += 1
    document.querySelectorAll('.game-card.selected, .game-card.active').forEach((card) => {
      card.classList.remove('selected', 'active')
    })
    hideFpsDetail()
    return
  }
  gamePanelLastFocus = document.activeElement
  selectedGameId = id
  // Selection is class-only — never rebuild the catalog grid.
  document.querySelectorAll('.game-card').forEach((card) => {
    const on = card.dataset.game === id
    card.classList.toggle('selected', on)
    card.classList.toggle('active', on)
  })
  showFpsDetail()
  const box = $('fps-result')
  if (box) {
    const g = lookupCatalogGame(id)
    const title = gameDisplayName(g || { id, name: id })
    box.className = 'fps-result game-selected-panel open'
    box.innerHTML = `
      <div class="game-selected-panel-bar">
        <div class="selected-game-banner game-compact">
          ${g ? gamePosterHTML(g).replace('class="game-poster"', 'class="game-cover selected-game-poster-mini"') : posterFallbackHTML(title, 'game-cover selected-game-poster-mini', 0, '', { hideTitle: true })}
          <div class="selected-game-info game-compact-body">
            <div class="game-compact-title">${escapeHtml(title)}</div>
            <div class="meta">${t('checking')}</div>
          </div>
        </div>
        <div class="game-selected-actions">
          <button type="button" class="btn compact btn-game-panel-close" id="btn-game-panel-close" aria-label="${t('gameClosePanel')}">✕</button>
        </div>
      </div>`
    $('btn-game-panel-close')?.addEventListener('click', () => {
      selectedGameId = null
      predictToken += 1
      document.querySelectorAll('.game-card.selected, .game-card.active').forEach((card) => {
        card.classList.remove('selected', 'active')
      })
      hideFpsDetail()
    })
    syncGamePanelPadding(true)
  }
  await predictSelectedGame()
  requestAnimationFrame(() => {
    $('btn-game-panel-close')?.focus()
    syncGamePanelPadding(true)
  })
}

function closeAllDropdowns() {
  document.querySelectorAll('.custom-dropdown.open').forEach((dd) => {
    dd.classList.remove('open')
    dd.querySelector('.dropdown-trigger')?.setAttribute('aria-expanded', 'false')
  })
}

function dropdownCurrentValue(kind) {
  if (kind === 'sort') return gameSort
  if (kind === 'game-req-fit') return gameReqFit
  return gameGenre
}

function syncCustomDropdownLabels() {
  document.querySelectorAll('.custom-dropdown').forEach((dd) => {
    const kind = dd.dataset.dropdown
    const value = dropdownCurrentValue(kind)
    const items = dd.querySelectorAll('.dropdown-item')
    let label = ''
    items.forEach((item) => {
      const on = item.dataset.value === value
      item.classList.toggle('active', on)
      const key = item.getAttribute('data-i18n')
      if (key) item.textContent = t(key)
      if (on) label = item.textContent
    })
    const selected = dd.querySelector('.dropdown-selected')
    if (selected && label) {
      selected.textContent = label
      const activeItem = dd.querySelector(`.dropdown-item[data-value="${value}"]`)
      const key = activeItem?.getAttribute('data-i18n')
      if (key) selected.setAttribute('data-i18n', key)
    }
  })
}

function setDropdownValue(dd, value) {
  if (!dd) return
  const kind = dd.dataset.dropdown
  if (kind === 'sort') gameSort = value || 'popular'
  else if (kind === 'game-req-fit') {
    gameReqFit = value || 'all'
    if (gameReqFit !== 'all') warmCompatScores(gameCatalog)
  } else gameGenre = value || 'all'
  syncCustomDropdownLabels()
  closeAllDropdowns()
  renderGameGrid()
}

function wireCustomDropdowns() {
  document.querySelectorAll('.custom-dropdown').forEach((dd) => {
    const trigger = dd.querySelector('.dropdown-trigger')
    trigger?.addEventListener('click', (ev) => {
      ev.stopPropagation()
      const willOpen = !dd.classList.contains('open')
      closeAllDropdowns()
      if (willOpen) {
        dd.classList.add('open')
        trigger.setAttribute('aria-expanded', 'true')
      }
    })
    dd.querySelectorAll('.dropdown-item').forEach((item) => {
      item.addEventListener('click', (ev) => {
        ev.stopPropagation()
        setDropdownValue(dd, item.dataset.value)
      })
    })
  })
  document.addEventListener('click', (ev) => {
    if (ev.target.closest('.custom-dropdown')) return
    closeAllDropdowns()
  })
  syncCustomDropdownLabels()
}

let warmCompatToken = 0
async function warmCompatScores(list) {
  if (!gameReqFit || gameReqFit === 'all') return
  const token = ++warmCompatToken
  const sample = (list || []).slice(0, 24)
  let changed = false
  // Yield between predictions so carousel scroll stays responsive.
  for (const g of sample) {
    if (token !== warmCompatToken) return
    if (!g?.id || fpsCache[g.id]?.verdictLevel) continue
    try {
      const p = await PredictGameFPS(g.id)
      if (token !== warmCompatToken) return
      fpsCache[g.id] = {
        medAvg: p.medAvg,
        highAvg: p.highAvg,
        verdictLevel: p.verdictLevel || 'warn',
      }
      changed = true
    } catch (_) {}
    await new Promise((r) => setTimeout(r, 0))
  }
  if (changed && token === warmCompatToken && gameReqFit && gameReqFit !== 'all') {
    renderGameGrid()
  }
}

function formatCompactFitDetails(p, fit) {
  const need = Number(p?.reqVRAMGB) || 0
  const have = Number(p?.vramGB) || 0
  const size = Number(fit?.storageNeedGB) || 0
  const disks = Array.isArray(fit?.disks) ? fit.disks : []
  const best = disks.reduce((acc, d) => ((Number(d?.freeGB) || 0) > (Number(acc?.freeGB) || 0) ? d : acc), null)
  const formatVram = (gb) => {
    if (gb <= 0) return '?'
    if (gb < 1) return t('unitsMB', { n: fmt(gb * 1024, 0) })
    return t('unitsGB', { n: String(Math.round(gb)) })
  }
  const vram = t('fitVramYou', {
    need: formatVram(need),
    have: formatVram(have),
  })
  let sizeBit = ''
  if (size > 0) {
    sizeBit = size < 1 ? t('fitSizeMB', { n: fmt(size * 1024, 0) }) : t('fitSizeApprox', { n: String(Math.round(size)) })
  }
  let diskBit = ''
  if (best) {
    const letter = String(best.mount || '').replace(/:\\?$/, '').replace(':', '')
    diskBit = t('fitDiskFree', { disk: letter, free: String(Math.round(Number(best.freeGB) || 0)) })
  }
  return [vram, sizeBit, diskBit].filter(Boolean).join(' · ')
}

function formatCompactFitLine(label, p, fit) {
  const details = formatCompactFitDetails(p, fit)
  return [label, details].filter(Boolean).join(' · ')
}

function formatFitDownload(fit) {
  const need = Number(fit.storageNeedGB) || 0
  const disks = Array.isArray(fit.disks) ? fit.disks : []
  const size = t('fitSize', { n: need > 0 ? Math.round(need) : '?' })
  const diskParts = disks.length
    ? disks.map((d) => {
      const letter = String(d.mount || '').replace(/:\\?$/, '').replace(':', '')
      return `${letter}: [${t('unitsGB', { n: Math.round(Number(d.freeGB) || 0) })}]`
    }).join(', ')
    : t('noData')
  const free = t('fitFree', { disks: diskParts })
  const mount = String(fit.recommendMount || '').replace(/:\\?$/, '').replace(':', '')
  let rec = ''
  if (mount) {
    const best = disks.find((d) => String(d.mount || '').startsWith(mount))
    const freeGB = Number(best?.freeGB) || 0
    rec = (need > 0 && freeGB >= need)
      ? t('fitInstallRec').replace('{disk}', mount)
      : t('fitInstallTight').replace('{disk}', mount)
  }
  const title = t('fitDownloadTitle')
  return [title, [size, free, rec].filter(Boolean).join(' · ')].filter(Boolean).join('\n')
}

function lookupCatalogGame(id) {
  const want = String(id || '')
  const pools = [gameCatalog, gameCatalogFull]
  for (const list of pools) {
    for (const g of list || []) {
      if (g && String(g.id) === want) return g
    }
  }
  return null
}

let predictToken = 0
async function predictSelectedGame() {
  const box = $('fps-result')
  if (!box || !selectedGameId) return
  const reqId = ++predictToken
  const wantId = String(selectedGameId)
  showFpsDetail()
  try {
    const p = await PredictGameFPS(wantId)
    if (reqId !== predictToken || selectedGameId !== wantId) return // stale race
    const catalogGame = lookupCatalogGame(wantId)
    // Prefer live catalog cover for THIS id only; never borrow another title's art.
    const coverSrc = catalogGame?.posterUrl || catalogGame?.headerUrl
      || (String(p.id || '') === wantId || String(p.gameId || '') === wantId
        ? (p.posterUrl || p.headerUrl || p.cover || p.imageURL || '')
        : '')
      || ''
    const headerSrc = catalogGame?.headerUrl
      || (coverSrc && p.headerUrl && p.headerUrl !== coverSrc ? p.headerUrl : '')
      || ''
    const fit = p.fit || {}
    const level = fit.level || 'unknown'
    fpsCache[wantId] = { medAvg: p.medAvg, highAvg: p.highAvg, verdictLevel: p.verdictLevel || fitLevelToFilterUI(level) }
    const useEn = lang === 'en'
    const label = (useEn ? fit.labelEn : fit.labelRu) || t(p.verdict || 'fitVerdictUnknown')
    const verdictCls = level === 'excellent' || level === 'ok' ? 'ok'
      : (level === 'tight' ? 'warn' : (level === 'wont' ? 'bad' : 'warn'))
    const badge = p.badge || p.genre || p.category || ''
    const displayTitle = gameDisplayName({
      id: wantId,
      name: p.gameName || catalogGame?.name || '',
      title: catalogGame?.title || '',
    })
    const coverPayload = {
      gameName: displayTitle,
      posterUrl: coverSrc,
      headerUrl: headerSrc,
      category: p.category || catalogGame?.category || '',
      genre: p.genre || catalogGame?.genre || '',
      year: p.year || catalogGame?.year || 0,
    }
    const cover = compactCoverHTML(coverPayload, { hideTitle: true })
      .replace('class="game-cover"', 'class="game-cover selected-game-poster-mini"')
    if (reqId !== predictToken || selectedGameId !== wantId) return
    box.className = 'fps-result game-selected-panel open'
    box.setAttribute('role', 'dialog')
    box.setAttribute('aria-modal', 'false')
    box.setAttribute('aria-label', t('gameSelectedPanel'))
    box.setAttribute('aria-hidden', 'false')
    const genreBadge = p.genre || catalogGame?.genre || badge
    const line = formatCompactFitLine(label, p, fit)
    box.innerHTML = `
      <div class="game-selected-panel-bar">
        <div class="selected-game-banner game-compact">
          ${cover}
          <div class="selected-game-info game-compact-body">
            <div class="game-compact-title" id="game-selected-title">${escapeHtml(displayTitle)}</div>
            <div class="game-compact-meta">
              ${genreBadge ? `<span class="game-badge era">${escapeHtml(genreBadge)}</span>` : ''}
              ${p.engine ? `<span class="game-badge">${escapeHtml(p.engine)}</span>` : ''}
              ${p.year ? `<span class="game-badge">${p.year}</span>` : ''}
            </div>
            <div class="game-compact-line ${verdictCls}">
              <span class="game-verdict-badge ${verdictCls}">${escapeHtml(label)}</span>
              <span class="game-verdict-details">${escapeHtml(formatCompactFitDetails(p, fit))}</span>
            </div>
          </div>
        </div>
        <div class="game-selected-actions">
          <button type="button" class="btn compact" id="btn-game-hide" data-i18n-skip="1">${t('gameHide')}</button>
          <button type="button" class="btn compact btn-game-panel-close" id="btn-game-panel-close" aria-label="${t('gameClosePanel')}">✕</button>
        </div>
      </div>
    `
    $('btn-game-panel-close')?.addEventListener('click', () => {
      selectedGameId = null
      predictToken += 1
      document.querySelectorAll('.game-card.selected, .game-card.active').forEach((card) => {
        card.classList.remove('selected', 'active')
      })
      hideFpsDetail()
    })
    $('btn-game-hide')?.addEventListener('click', () => hideSelectedGame())
    requestAnimationFrame(() => syncGamePanelPadding(true))
  } catch (err) {
    if (reqId !== predictToken || selectedGameId !== wantId) return
    box.className = 'fps-result game-selected-panel open empty'
    box.innerHTML = `<div class="game-selected-error" role="alert">${escapeHtml(String(err))}</div>
      <button type="button" class="btn compact btn-game-panel-close" id="btn-game-panel-close" aria-label="${t('gameClosePanel')}">✕</button>`
    $('btn-game-panel-close')?.addEventListener('click', () => {
      selectedGameId = null
      predictToken += 1
      hideFpsDetail()
    })
    syncGamePanelPadding(true)
  }
}

function fitLevelToFilterUI(level) {
  if (level === 'excellent' || level === 'ok') return 'ok'
  if (level === 'tight') return 'warn'
  if (level === 'wont') return 'bad'
  return 'warn'
}

function openAbout() {
  const box = $('license-box')
  if (box) box.textContent = MIT_LICENSE
  $('about-modal')?.classList.remove('hidden')
}

function closeAbout() {
  $('about-modal')?.classList.add('hidden')
}

function showToast(text, ok = false, opts = {}) {
  // Toasts belong to the main shell only — never over Mini-HUD / game overlay.
  if (hudEnabled || gameOverlayOn || document.body.classList.contains('mini-hud-active') || document.body.classList.contains('game-overlay-active')) {
    return
  }
  const el = $('toast')
  if (!el) return
  const sessionId = opts.sessionId || ''
  const holdMs = opts.holdMs != null ? opts.holdMs : (sessionId ? 15000 : 4200)
  el.classList.toggle('ok', !!ok)
  el.classList.remove('hidden')
  const revertBtn = sessionId
    ? `<div class="toast-actions"><button type="button" class="btn compact" data-toast-revert="${sessionId}">${t('ecoRevert')}</button></div>`
    : ''
  el.innerHTML = `<div class="toast-text">${escapeHtml(text)}</div>${revertBtn}`
  clearTimeout(showToast._t)
  const started = Date.now()
  const hide = () => {
    const left = holdMs - (Date.now() - started)
    if (left > 50) {
      showToast._t = setTimeout(hide, left)
      return
    }
    el.classList.add('hidden')
    el.innerHTML = ''
  }
  showToast._t = setTimeout(hide, holdMs)
  el.onclick = async (ev) => {
    const btn = ev.target.closest('[data-toast-revert]')
    if (!btn) return
    ev.stopPropagation()
    btn.disabled = true
    let res
    try {
      res = await RevertEcoSession(btn.dataset.toastRevert)
    } catch (err) {
      res = { ok: false, messageRu: String(err), messageEn: String(err) }
    }
    clearTimeout(showToast._t)
    showToast(ecoMsg(res), !!res.ok, { holdMs: 5000 })
    renderEcoLimitedPanel().catch(() => {})
    refresh()
  }
}

async function renderEcoLimitedPanel() {
  const panel = $('eco-limited-panel')
  const list = $('eco-limited-list')
  if (!panel || !list) return
  let groups = []
  try {
    groups = await GetEcoLimitedGroups()
  } catch (_) {
    groups = []
  }
  if (!groups || !groups.length) {
    panel.classList.add('hidden')
    list.innerHTML = ''
    return
  }
  panel.classList.remove('hidden')
  list.innerHTML = groups.map((g) => `
    <div class="row">
      <div class="row-main">
        <div class="name">${escapeHtml(g.friendlyName || g.name)} <span class="badge-count">×${g.count}</span></div>
        <div class="meta">${escapeHtml(g.name)} · ${t('ecoAppliedAt')} ${g.appliedAt || '-'}</div>
      </div>
      <div class="row-actions">
        <button type="button" class="btn compact" data-eco-group-revert="${escapeHtml(g.name)}">${t('ecoRevert')}</button>
      </div>
    </div>
  `).join('')
}

function openModal(html) {
  $('modal-body').innerHTML = html
  $('modal').classList.remove('hidden')
}

let diskCleanTab = 'apps'
let diskCleanAppsCache = []
let diskCleanLastLeftovers = []
let scanRevision = 0

function openDiskCleanModal() {
  $('diskclean-modal')?.classList.remove('hidden')
}
function closeDiskCleanModal() {
  $('diskclean-modal')?.classList.add('hidden')
  renderHome().catch(() => {})
}

let selectedCleanerDrive = 'C'
let selectedCleanDrive = 'C'
let diskCleanFilterCandidates = false
let localDrivesCache = []

async function renderDiskCleanDrives() {
  const container = $('cleaner-drives-bar') || $('diskclean-drives')
  if (!container) return
  try {
    localDrivesCache = await GetLocalDrives()
  } catch (err) {
    console.warn('Failed to get local drives:', err)
    localDrivesCache = []
  }
  if (!Array.isArray(localDrivesCache) || localDrivesCache.length === 0) {
    container.innerHTML = ''
    return
  }

  const driveLetters = localDrivesCache.map((d) => (d.letter || '').replace(/[:\\\/]/g, '').toUpperCase())
  if (!driveLetters.includes(selectedCleanerDrive.toUpperCase()) && driveLetters.length > 0) {
    selectedCleanerDrive = driveLetters[0]
  }
  selectedCleanDrive = selectedCleanerDrive

  updateDiskCleanTitle()

  container.innerHTML = localDrivesCache.map((d) => {
    const letter = (d.letter || '').replace(/[:\\\/]/g, '').toUpperCase()
    const isActive = letter === selectedCleanerDrive.toUpperCase()
    const freeGb = Math.round(d.freeGB || ((d.freeBytes || 0) / (1024 * 1024 * 1024)))
    const cardText = t('diskCleanDriveCard', {
      letter,
      free: `${freeGb} ${t('unitsGB', { n: '' }).trim()}`,
    })
    return `
      <button type="button" class="cleaner-drive-pill dc-drive-card ${isActive ? 'active' : ''}" data-drive="${letter}" role="tab" aria-selected="${isActive}">
        <span>${escapeHtml(cardText)}</span>
      </button>
    `
  }).join('')

  container.querySelectorAll('.cleaner-drive-pill').forEach((card) => {
    card.addEventListener('click', async (ev) => {
      ev.preventDefault()
      ev.stopPropagation()
      const drv = card.getAttribute('data-drive')
      if (drv && drv !== selectedCleanerDrive) {
        selectedCleanerDrive = drv
        selectedCleanDrive = drv
        await renderDiskCleanDrives()
        await renderDiskCleanBody()
      }
    })
  })
}

function updateDiskCleanTitle() {
  const title = $('diskclean-title')
  if (title) {
    title.textContent = t('diskCleanDriveTitle', { letter: selectedCleanerDrive || 'C' })
  }
}

async function openDiskCleanupWizard(tab = 'apps') {
  diskCleanTab = tab || 'apps'
  diskCleanFilterCandidates = false
  openDiskCleanModal()
  await renderDiskCleanDrives()
  renderDiskCleanTabs()
  await renderDiskCleanBody()
}

function formatBytesSmart(bytes, withSign = false) {
  const n = Math.max(0, Number(bytes) || 0)
  const sign = withSign ? '+' : ''
  const isEn = lang === 'en'
  // units handled via t()
  // units handled via t()
  // units handled via t()

  if (n === 0) {
    return `${sign}0 ` + t('unitsMB', { n: '' }).trim()
  }
  if (n < 1024 * 1024) {
    const kb = Math.round(n / 1024)
    return `${sign}` + t('unitsKB', { n: String(kb) })
  }
  if (n < 1024 * 1024 * 1024) {
    const mb = n / (1024 * 1024)
    const val = mb >= 10 ? fmt(mb, 0) : fmt(mb, 1)
    return `${sign}` + t('unitsMB', { n: String(val) })
  }
  const gb = n / (1024 * 1024 * 1024)
  const val = fmt(gb, 1)
    return `${sign}` + t('unitsGB', { n: String(val) })
}

function bytesToGB(n) {
  return fmt((Number(n) || 0) / (1024 * 1024 * 1024), 1)
}

function cleanStatBytes(stats, name) {
  if (!stats) return 0
  const snake = String(name).replace(/[A-Z]/g, (m) => `_${m.toLowerCase()}`)
  return Number(stats[name] ?? stats[snake] ?? 0) || 0
}

function formatCleanStat(bytes) {
  return formatBytesSmart(bytes)
}

function cleanCompareCardHTML(usedBefore, usedAfter, freed) {
  const ub = bytesToGB(usedBefore)
  const ua = bytesToGB(usedAfter)
  const fr = formatBytesSmart(freed, true)
  if (!(Number(usedBefore) > 0 || Number(freed) > 0)) return ''
  return `<div class="clean-compare-card">
    <div class="clean-compare-row"><span>${t('cleanWasUsed')}</span><b>${escapeHtml(t('unitsGB', { n: ub }))}</b></div>
    <div class="clean-compare-row"><span>${t('cleanNowUsed')}</span><b>${escapeHtml(t('unitsGB', { n: ua }))}</b></div>
    <div class="clean-compare-freed">${t('cleanFreedNow', { val: fr, gb: fr, size: fr })}</div>
  </div>`
}

async function renderCleanStatsBanner(host) {
  if (!host) return
  let stats
  try { stats = await GetCleanStats() } catch (_) { stats = null }
  const today = formatBytesSmart(cleanStatBytes(stats, 'todayFreed'))
  const month = formatBytesSmart(cleanStatBytes(stats, 'monthFreed'))
  const all = formatBytesSmart(cleanStatBytes(stats, 'allTimeFreed'))
  host.innerHTML = `<div class="clean-stats-banner" id="clean-stats-banner">
    <div class="clean-stats-title">${t('cleanStatsTitle')}</div>
    <div class="clean-stats-grid">
      <div class="clean-stat-pill"><span>${t('cleanStatsToday')}</span><b data-anim="today">${escapeHtml(today)}</b></div>
      <div class="clean-stat-pill"><span>${t('cleanStatsMonth')}</span><b data-anim="month">${escapeHtml(month)}</b></div>
      <div class="clean-stat-pill"><span>${t('cleanStatsAll')}</span><b data-anim="all">${escapeHtml(all)}</b></div>
    </div>
  </div>`
  host.querySelectorAll('.clean-stat-pill b').forEach((el) => {
    el.classList.remove('bump')
    void el.offsetWidth
    el.classList.add('bump')
  })
}

function renderDiskCleanTabs() {
  const el = $('diskclean-tabs')
  if (!el) return
  const tabs = [
    { id: 'apps', label: t('diskCleanTabApps') },
    { id: 'leftovers', label: t('diskCleanTabLeftovers') },
    { id: 'cache', label: t('diskCleanTabCache') },
    { id: 'system', label: t('diskCleanTabSystem') },
  ]
  el.innerHTML = tabs.map((tab) => (
    `<button type="button" class="diskclean-tab ${diskCleanTab === tab.id ? 'active' : ''}" data-dc-tab="${tab.id}" role="tab" aria-selected="${diskCleanTab === tab.id ? 'true' : 'false'}">${escapeHtml(tab.label)}</button>`
  )).join('')
  el.querySelectorAll('[data-dc-tab]').forEach((btn) => {
    btn.addEventListener('click', (ev) => {
      ev.preventDefault()
      ev.stopPropagation()
      diskCleanTab = btn.getAttribute('data-dc-tab') || 'apps'
      renderDiskCleanTabs()
      renderDiskCleanBody().catch(() => {})
    })
  })
}

async function renderDiskCleanBody() {
  const body = $('diskclean-body')
  if (!body) return
  body.innerHTML = `<div class="sf-loading">${t('diskCleanLoading')}</div>`
  if (diskCleanTab === 'apps') {
    await renderDiskCleanApps(body)
  } else if (diskCleanTab === 'leftovers') {
    await renderDiskCleanLeftovers(body)
  } else if (diskCleanTab === 'cache') {
    await renderDiskCleanCache(body)
  } else {
    await renderDiskCleanSystem(body)
  }
}

function formatAppSize(bytes, label) {
  if (label && label !== '—') return label
  return formatCleanupSize(bytes)
}

async function renderDiskCleanApps(body) {
  const currentRev = ++scanRevision
  let apps = []
  try { apps = await ListInstalledApps() } catch (err) {
    if (currentRev !== scanRevision) return
    body.innerHTML = `<div class="sf-loading">${escapeHtml(String(err))}</div>`
    return
  }
  if (currentRev !== scanRevision) return
  diskCleanAppsCache = Array.isArray(apps) ? apps : []

  const targetDrive = (selectedCleanerDrive || selectedCleanDrive || 'C').toUpperCase()
  const driveFiltered = diskCleanAppsCache.filter((a) => {
    const appDrive = (a.driveLetter || '').toUpperCase()
    if (!appDrive) {
      return targetDrive === 'C'
    }
    return appDrive === targetDrive
  })

  const candidatesCount = driveFiltered.filter((a) => a.candidateCleanup).length

  const candidateFiltered = diskCleanFilterCandidates
    ? driveFiltered.filter((a) => a.candidateCleanup)
    : driveFiltered

  const q = (body.querySelector('#dc-app-search')?.value || '').trim().toLowerCase()
  const finalFiltered = candidateFiltered.filter((a) => {
    if (!q) return true
    const hay = `${a.displayName || ''} ${a.publisher || ''}`.toLowerCase()
    return hay.includes(q)
  })

  const rows = finalFiltered.slice(0, 100).map((a) => {
    const size = formatAppSize(a.sizeBytes, a.sizeLabel)
    const disabled = a.canUninstall ? '' : 'disabled'
    const loc = String(a.installLocation || '').trim()
    const openBtn = loc
      ? `<button type="button" class="btn soft compact dc-open-folder" data-path="${escapeHtml(loc)}" title="${escapeHtml(t('diskCleanOpenFolder'))}">📁</button>`
      : ''
    const candidateBadge = (a.candidateCleanup || a.candidateBadgeRu || a.candidateBadgeEn)
      ? `<div class="dc-candidate-badge">${escapeHtml(lang === 'en' ? (a.candidateBadgeEn || a.candidateBadgeRu) : (a.candidateBadgeRu || a.candidateBadgeEn))}</div>`
      : ''
    return `<div class="dc-app-row" data-app-id="${escapeHtml(a.id)}">
      <div class="dc-app-main">
        <div class="dc-app-name">${escapeHtml(a.displayName || '')}</div>
        <div class="dc-app-meta">${escapeHtml(a.publisher || '')}${a.displayVersion ? ' · ' + escapeHtml(a.displayVersion) : ''}</div>
        ${candidateBadge}
      </div>
      <div class="dc-app-size">${escapeHtml(size)}</div>
      <div class="dc-app-actions">
        ${openBtn}
        <button type="button" class="btn soft compact dc-uninstall" data-app-id="${escapeHtml(a.id)}" ${disabled}>${t('diskCleanUninstall')}</button>
      </div>
      <div class="dc-row-loader hidden" aria-hidden="true"></div>
    </div>`
  }).join('')

  if (!body.querySelector('#dc-app-search')) {
    body.innerHTML = `
    <div class="dc-pane">
      <div id="dc-stats-slot"></div>
      <div class="dc-filter-toolbar">
        <div class="dc-filter-pills">
          <button type="button" class="btn compact dc-filter-btn ${!diskCleanFilterCandidates ? 'active' : ''}" id="dc-filter-all">${t('diskCleanAllApps')}</button>
          <button type="button" class="btn compact dc-filter-btn ${diskCleanFilterCandidates ? 'active' : ''}" id="dc-filter-candidates">${t('diskCleanCandidates')} (${candidatesCount})</button>
        </div>
        <input type="search" id="dc-app-search" class="dc-search" placeholder="${escapeHtml(t('diskCleanSearch'))}" autocomplete="off" />
      </div>
      <div id="cleaner-list-body" class="dc-app-list"></div>
      <div id="dc-app-status" class="dc-status hidden"></div>
    </div>`

    const search = body.querySelector('#dc-app-search')
    let searchTimer = null
    search?.addEventListener('input', () => {
      clearTimeout(searchTimer)
      searchTimer = setTimeout(() => renderDiskCleanApps(body).catch(() => {}), 160)
    })

    body.querySelector('#dc-filter-all')?.addEventListener('click', () => {
      if (diskCleanFilterCandidates) {
        diskCleanFilterCandidates = false
        renderDiskCleanApps(body).catch(() => {})
      }
    })

    body.querySelector('#dc-filter-candidates')?.addEventListener('click', () => {
      if (!diskCleanFilterCandidates) {
        diskCleanFilterCandidates = true
        renderDiskCleanApps(body).catch(() => {})
      }
    })
  } else {
    const btnAll = body.querySelector('#dc-filter-all')
    const btnCand = body.querySelector('#dc-filter-candidates')
    if (btnAll) btnAll.classList.toggle('active', !diskCleanFilterCandidates)
    if (btnCand) {
      btnCand.classList.toggle('active', diskCleanFilterCandidates)
      btnCand.textContent = `${t('diskCleanCandidates')} (${candidatesCount})`
    }
  }

  const listHost = body.querySelector('#cleaner-list-body')
  if (listHost) listHost.innerHTML = rows || `<div class="sf-empty">${t('diskCleanAppsEmpty')}</div>`
  await renderCleanStatsBanner(body.querySelector('#dc-stats-slot'))
  body.querySelectorAll('.dc-open-folder').forEach((btn) => {
    btn.addEventListener('click', async (ev) => {
      ev.preventDefault()
      ev.stopPropagation()
      const path = btn.getAttribute('data-path') || ''
      if (!path) return
      try { await OpenFolderInExplorer(path) } catch (err) {
        showToast(String(err), false)
      }
    })
  })
  body.querySelectorAll('.dc-uninstall').forEach((btn) => {
    btn.addEventListener('click', async (ev) => {
      ev.preventDefault()
      ev.stopPropagation()
      const id = btn.getAttribute('data-app-id') || ''
      if (!id) return
      const app = diskCleanAppsCache.find((x) => x.id === id)
      const name = app?.displayName || ''
      const sizeLabel = formatAppSize(app?.sizeBytes, app?.sizeLabel)
      const pathHint = app?.installLocation || ''
      const ok = await glassConfirm({
        title: t('diskCleanDeleteTitle', { name }),
        body: t('diskCleanDeleteBody', { size: sizeLabel, path: pathHint || '—' }),
        confirmLabel: t('confirmDeleteForever'),
        danger: true,
      })
      if (!ok) return
      const row = btn.closest('.dc-app-row')
      btn.disabled = true
      btn.textContent = t('diskCleanDeleting', { size: sizeLabel })
      row?.classList.add('deleting')
      row?.querySelector('.dc-row-loader')?.classList.remove('hidden')
      openDeleteProgress(name)
      window.__dcPendingDeleteId = id
      window.__dcRefreshAfterDelete = async () => {
        const pending = body.querySelector(`.dc-app-row[data-app-id="${CSS.escape(id)}"]`)
        if (pending) {
          pending.classList.add('fade-out')
          setTimeout(() => pending.remove(), 250)
        }
        renderDiskCleanDrives().catch(() => {})
        await renderDiskCleanApps(body)
      }
      try {
        await StartUninstallWithCleanup(id)
      } catch (err) {
        updateDeleteProgress({ phase: 'error', message: String(err) })
        btn.disabled = false
        btn.textContent = t('diskCleanUninstall')
        row?.classList.remove('deleting')
        row?.querySelector('.dc-row-loader')?.classList.add('hidden')
      }
    })
  })
}

let lastTrashedLeftover = null

async function renderDiskCleanLeftovers(body) {
  const currentRev = ++scanRevision
  let items = []
  try { items = await ListOrphanLeftovers() } catch (err) {
    if (currentRev !== scanRevision) return
    body.innerHTML = `<div class="sf-loading">${escapeHtml(String(err))}</div>`
    return
  }
  if (currentRev !== scanRevision) return
  let binBytes = 0
  try {
    const bin = await GetRecycleBinInfo()
    binBytes = Number(bin?.bytes) || 0
  } catch (_) { binBytes = 0 }
  const q = (body.querySelector('#dc-left-search')?.value || '').trim().toLowerCase()
  const filtered = (Array.isArray(items) ? items : []).filter((it) => {
    if (!q) return true
    return `${it.name || ''} ${it.path || ''}`.toLowerCase().includes(q)
  })
  const rows = filtered.map((it) => {
    const ageBadge = (it.ageBadgeRu || it.ageBadgeEn)
      ? `<div class="dc-age-badge ${it.ageStatus || 'neutral'}">${escapeHtml(lang === 'en' ? (it.ageBadgeEn || it.ageBadgeRu) : (it.ageBadgeRu || it.ageBadgeEn))}</div>`
      : ''
    return `
    <div class="dc-app-row" data-orphan-path="${escapeHtml(it.path)}">
      <div class="dc-app-main">
        <div class="dc-app-name">${escapeHtml(it.name || '')}</div>
        <div class="dc-app-meta">${escapeHtml(it.path || '')}</div>
        ${ageBadge}
      </div>
      <div class="dc-app-size">${escapeHtml(formatBytesSmart(it.sizeBytes))}</div>
      <button type="button" class="btn soft compact dc-orphan-del" data-path="${escapeHtml(it.path)}" data-name="${escapeHtml(it.name || '')}">${t('diskCleanDeleteLeftover')}</button>
    </div>`
  }).join('')

  if (!body.querySelector('#dc-left-search')) {
    body.innerHTML = `
    <div class="dc-pane">
      <div id="dc-stats-slot"></div>
      <div class="dc-app-row" id="dc-recycle-row">
        <div class="dc-app-main">
          <div class="dc-app-name">${t('diskCleanRecycleTitle')}</div>
        </div>
        <div class="dc-app-size" id="dc-recycle-size">${escapeHtml(binBytes > 0 ? formatBytesSmart(binBytes) : t('recycleBinEmptyStatus'))}</div>
        <button type="button" class="btn soft compact" id="dc-empty-recycle" ${binBytes <= 0 ? 'disabled' : ''}>${t('diskCleanRecycleAction')}</button>
      </div>

      <div id="dc-leftover-undo-banner" class="dc-undo-banner ${lastTrashedLeftover ? '' : 'hidden'}">
        <div class="dc-undo-header">
          ${t('dcUndoHeader', { name: escapeHtml(lastTrashedLeftover?.name || '') })}
        </div>
        <div class="dc-undo-actions">
          <button type="button" class="btn primary compact" id="dc-btn-restore-last">${t('dcUndoRestoreBtn')}</button>
          <button type="button" class="btn soft compact" id="dc-btn-open-recycle">${t('dcUndoOpenRecycle')}</button>
        </div>
      </div>

      <input type="search" id="dc-left-search" class="dc-search" placeholder="${escapeHtml(t('diskCleanSearchLeftovers'))}" autocomplete="off" />
      <div class="sf-glass-block">
        <div class="sf-block-title">${t('diskCleanLeftoversTitle')}</div>
        <div class="dc-app-meta" style="margin-bottom:10px">${t('diskCleanLeftoversHint')}</div>
        <div id="cleaner-list-body" class="dc-app-list"></div>
      </div>
    </div>`

    const search = body.querySelector('#dc-left-search')
    let searchTimer = null
    search?.addEventListener('input', () => {
      clearTimeout(searchTimer)
      searchTimer = setTimeout(() => renderDiskCleanLeftovers(body).catch(() => {}), 160)
    })
    body.querySelector('#dc-empty-recycle')?.addEventListener('click', async () => {
      const btn = body.querySelector('#dc-empty-recycle')
      if (btn) btn.disabled = true
      try {
        await EmptyRecycleBin()
      } catch (err) {
        showToast(String(err), false)
      }
      renderDiskCleanDrives().catch(() => {})
      await renderCleanStatsBanner(body.querySelector('#dc-stats-slot'))
      await renderDiskCleanLeftovers(body)
    })
  }

  // Bind Undo & Open Recycle Bin actions
  const undoBanner = body.querySelector('#dc-leftover-undo-banner')
  if (undoBanner) {
    if (lastTrashedLeftover) {
      undoBanner.querySelector('.dc-undo-header').textContent = t('dcUndoHeader', { name: lastTrashedLeftover.name || '' })
      undoBanner.classList.remove('hidden')
    } else {
      undoBanner.classList.add('hidden')
    }

    body.querySelector('#dc-btn-restore-last')?.addEventListener('click', async () => {
      const btn = body.querySelector('#dc-btn-restore-last')
      if (btn) {
        btn.disabled = true
        btn.textContent = t('dcUndoRestoring')
      }
      try {
        const res = await window.go.main.App.RestoreLastTrashedItem()
        if (res?.success) {
          showToast(t('dcUndoSuccess', { name: res.name }), true)
          lastTrashedLeftover = null
          await renderDiskCleanLeftovers(body)
        } else {
          showToast(res?.message || t('dcUndoFailed'), false)
          if (btn) {
            btn.disabled = false
          btn.textContent = t('dcUndoRestoreBtn')
          }
        }
      } catch (err) {
        showToast(String(err), false)
        if (btn) {
          btn.disabled = false
          btn.textContent = t('dcUndoRestoreBtn')
        }
      }
    })

    body.querySelector('#dc-btn-open-recycle')?.addEventListener('click', async () => {
      try {
        if (window.go?.main?.App?.OpenRecycleBin) {
          await window.go.main.App.OpenRecycleBin()
        }
      } catch (_) {}
    })
  }

  const sizeEl = body.querySelector('#dc-recycle-size')
  if (sizeEl) sizeEl.textContent = binBytes > 0 ? formatBytesSmart(binBytes) : t('recycleBinEmptyStatus')
  const emptyBtn = body.querySelector('#dc-empty-recycle')
  if (emptyBtn) emptyBtn.disabled = binBytes <= 0
  const listHost = body.querySelector('#cleaner-list-body')
  if (listHost) listHost.innerHTML = rows || `<div class="sf-empty">${t('diskCleanLeftoversEmpty')}</div>`
  await renderCleanStatsBanner(body.querySelector('#dc-stats-slot'))

  body.querySelectorAll('.dc-orphan-del').forEach((btn) => {
    btn.addEventListener('click', async (ev) => {
      ev.preventDefault()
      ev.stopPropagation()
      const path = btn.getAttribute('data-path') || ''
      const name = btn.getAttribute('data-name') || path
      if (!path) return
      const row = btn.closest('.dc-app-row')
      btn.disabled = true
      openDeleteProgress(name)
      window.__dcRefreshAfterDelete = async () => {
        if (row) {
          row.classList.add('fade-out')
          setTimeout(() => row.remove(), 250)
        }
        lastTrashedLeftover = { name, path }
        renderDiskCleanDrives().catch(() => {})
        await renderDiskCleanLeftovers(body)
      }
      try {
        await StartRemoveOrphanLeftover(path)
      } catch (err) {
        updateDeleteProgress({ phase: 'error', message: String(err) })
        btn.disabled = false
      }
    })
  })
}

async function renderDiskCleanCache(body) {
  const currentRev = ++scanRevision
  let cats = []
  try { cats = await PreviewDiskCleanup() } catch (err) {
    if (currentRev !== scanRevision) return
    body.innerHTML = `<div class="sf-loading">${escapeHtml(String(err))}</div>`
    return
  }
  if (currentRev !== scanRevision) return
  const list = (Array.isArray(cats) ? cats : []).filter((c) => Number(c.sizeBytes) > 0 || c.id === 'recycle')
  body.innerHTML = `
    <div class="dc-pane">
      <div id="dc-stats-slot"></div>
      ${(selectedCleanerDrive || 'C').toUpperCase() !== 'C' ? `<div class="dc-cache-notice">ℹ️ ${escapeHtml(t('diskCleanCacheNotice'))}</div>` : ''}
      <div class="sf-glass-block">
        <div class="sf-block-title">${t('diskCleanTabCache')}</div>
        <div class="sf-cleanup-list">${list.map((c) => {
          const isRecycleEmpty = c.id === 'recycle' && (!c.sizeBytes || Number(c.sizeBytes) <= 0)
          const checked = c.selected !== false && !c.needsAdmin && !isRecycleEmpty ? 'checked' : ''
          const disabled = c.needsAdmin ? 'disabled' : ''
          const badges = c.irreversible ? `<span class="sf-badge irr">${t('smartFixIrreversible')}</span>` : ''
          const sizeDisplay = isRecycleEmpty ? t('recycleBinEmptyStatus') : formatCleanupSize(c.sizeBytes)
          return `<label class="sf-cleanup-row">
            <input type="checkbox" data-dc-cat="${escapeHtml(c.id)}" ${checked} ${disabled} />
            <span class="sf-cat-name">${escapeHtml(cleanupCatLabel(c.id))}</span>
            <span class="sf-cat-size">${escapeHtml(sizeDisplay)}</span>
            ${badges}
          </label>`
        }).join('') || `<div class="sf-empty">${t('diskCleanCacheEmpty')}</div>`}</div>
      </div>
      <div class="btn-row sf-actions">
        <button type="button" class="btn primary" id="dc-clean-selected">${t('diskCleanApplyCache')}</button>
      </div>
      <div id="dc-cache-status" class="dc-status hidden"></div>
    </div>`
  await renderCleanStatsBanner(body.querySelector('#dc-stats-slot'))
  body.querySelector('#dc-clean-selected')?.addEventListener('click', async () => {
    const selected = [...body.querySelectorAll('input[data-dc-cat]:checked')].map((el) => el.getAttribute('data-dc-cat')).filter(Boolean)
    const status = body.querySelector('#dc-cache-status')
    if (status) {
      status.classList.remove('hidden')
      status.textContent = t('diskCleanWorking')
    }
    let res
    try { res = await ApplyDiskCleanup(selected) } catch (err) {
      if (status) status.textContent = String(err)
      return
    }
    renderDiskCleanDrives().catch(() => {})
    try { await RefreshDiskMetricsEx() } catch (_) {}
    window.__lastCleanCompare = {
      usedBefore: res?.usedBefore || 0,
      usedAfter: res?.usedAfter || 0,
      freed: res?.actualCleanedBytes || res?.freedBytes || 0,
    }
    if (status) {
      status.innerHTML = cleanCompareCardHTML(
        window.__lastCleanCompare.usedBefore,
        window.__lastCleanCompare.usedAfter,
        window.__lastCleanCompare.freed,
      ) || `<div class="dc-status-ok">${t('smartFixCleanupDone', { gb: formatBytesSmart(window.__lastCleanCompare.freed, true) })}</div>`
    }
    renderHome().catch(() => {})
    await renderCleanStatsBanner(body.querySelector('#dc-stats-slot'))
  })
}

async function renderDiskCleanSystem(body) {
  const currentRev = ++scanRevision
  let deep
  try { deep = await PreviewDeepCleanup() } catch (err) {
    if (currentRev !== scanRevision) return
    body.innerHTML = `<div class="sf-loading">${escapeHtml(String(err))}</div>`
    return
  }
  if (currentRev !== scanRevision) return
  const hibBytes = Number(deep?.hibernationBytes) || 0
  const hibOn = !!deep?.hibernationOn
  const shadow = Number(deep?.shadowBytes) || 0
  const delivery = Number(deep?.deliveryOptBytes) || 0
  body.innerHTML = `
    <div class="dc-pane">
      <div id="dc-stats-slot"></div>
      <div class="sf-glass-block dc-sys-row">
        <div>
          <div class="sf-cat-name">${t('diskCleanHibernation')}</div>
          <div class="dc-app-meta">${hibOn ? t('diskCleanHibernationOn', { size: formatCleanupSize(hibBytes) }) : t('diskCleanHibernationOff')}</div>
        </div>
        <button type="button" class="btn soft compact" id="dc-hibernation">${hibOn ? t('diskCleanDisableHibernation') : t('diskCleanEnableHibernation')}</button>
      </div>
      <div class="sf-glass-block dc-sys-row">
        <div>
          <div class="sf-cat-name">${t('diskCleanVSS')}</div>
          <div class="dc-app-meta">${t('diskCleanVSSHint', { size: formatCleanupSize(shadow) })}</div>
        </div>
        <div class="dc-sys-actions">
          <button type="button" class="btn soft compact" id="dc-vss" ${shadow > 0 ? '' : 'disabled'} title="${shadow > 0 ? '' : escapeHtml(t('diskCleanVSSNoPoints'))}">${t('diskCleanVSSAction')}</button>
          <button type="button" class="btn soft compact" id="dc-create-rp">${t('diskCleanCreateRP')}</button>
        </div>
      </div>
      <div class="sf-glass-block dc-sys-row">
        <div>
          <div class="sf-cat-name">${t('diskCleanDelivery')}</div>
          <div class="dc-app-meta">${formatCleanupSize(delivery)}</div>
        </div>
        <button type="button" class="btn soft compact" id="dc-delivery" ${delivery > 0 ? '' : 'disabled'}>${t('diskCleanDeliveryAction')}</button>
      </div>
      <div id="dc-sys-status" class="dc-status hidden"></div>
    </div>`
  await renderCleanStatsBanner(body.querySelector('#dc-stats-slot'))
  const status = body.querySelector('#dc-sys-status')
  const setStatus = (html) => {
    if (!status) return
    status.classList.remove('hidden')
    status.innerHTML = html
  }
  body.querySelector('#dc-hibernation')?.addEventListener('click', async () => {
    setStatus(t('diskCleanWorking'))
    let res
    try {
      res = hibOn
        ? await DisableHibernation()
        : await EnableHibernation()
    } catch (err) {
      setStatus(escapeHtml(String(err)))
      return
    }
    try {
      const st = await GetCleanStats()
      setStatus(cleanCompareCardHTML(cleanStatBytes(st, 'lastUsedBefore'), cleanStatBytes(st, 'lastUsedAfter'), cleanStatBytes(st, 'lastCleanFreed') || res?.freedBytes || 0)
        || `<div class="dc-status-ok">${t('smartFixCleanupDone', { gb: formatBytesGB(res?.actualCleanedBytes || res?.freedBytes || 0) })}</div>`)
    } catch (_) {
      setStatus(`<div class="dc-status-ok">${t('smartFixCleanupDone', { gb: formatBytesGB(res?.actualCleanedBytes || res?.freedBytes || 0) })}</div>`)
    }
    renderHome().catch(() => {})
    await renderDiskCleanSystem(body)
  })
  body.querySelector('#dc-vss')?.addEventListener('click', async () => {
    const ok = await glassConfirm({
      title: t('diskCleanVSS'),
      body: t('diskCleanVSSConfirm'),
      confirmLabel: t('diskCleanVSSAction'),
      danger: true,
    })
    if (!ok) return
    setStatus(t('diskCleanWorking'))
    let res
    try { res = await CleanOldestShadow() } catch (err) { setStatus(escapeHtml(String(err))); return }
    setStatus(`<div class="dc-status-ok">${t('smartFixCleanupDone', { gb: formatBytesGB(res?.actualCleanedBytes || res?.freedBytes || 0) })}</div>`)
    renderHome().catch(() => {})
    await renderDiskCleanSystem(body)
  })
  body.querySelector('#dc-delivery')?.addEventListener('click', async () => {
    setStatus(t('diskCleanWorking'))
    let res
    try { res = await CleanDeliveryOptimization() } catch (err) { setStatus(escapeHtml(String(err))); return }
    setStatus(`<div class="dc-status-ok">${t('smartFixCleanupDone', { gb: formatBytesGB(res?.actualCleanedBytes || res?.freedBytes || 0) })}</div>`)
    renderHome().catch(() => {})
    await renderCleanStatsBanner(body.querySelector('#dc-stats-slot'))
    await renderDiskCleanSystem(body)
  })
  body.querySelector('#dc-create-rp')?.addEventListener('click', async () => {
    const btn = body.querySelector('#dc-create-rp')
    if (btn) {
      btn.disabled = true
      btn.textContent = t('diskCleanCreateRPWorking')
    }
    setStatus(t('diskCleanCreateRPWorking'))
    let res
    try { res = await CreateRestorePoint() } catch (err) {
      setStatus(escapeHtml(String(err)))
      if (btn) {
        btn.disabled = false
        btn.textContent = t('diskCleanCreateRP')
      }
      return
    }
    if (btn) {
      btn.disabled = false
      btn.textContent = t('diskCleanCreateRP')
    }
    const code = String(res?.message || '')
    if (res?.ok) setStatus(`<div class="dc-status-ok">${t('diskCleanCreateRPOK')}</div>`)
    else if (code === 'restore_point_24h') setStatus(`<div class="dc-status-warn">${t('diskCleanCreateRP24h')}</div>`)
    else if (code === 'restore_point_admin') setStatus(`<div class="dc-status-warn">${t('diskCleanCreateRPAdmin')}</div>`)
    else setStatus(`<div class="dc-status-warn">${escapeHtml(code || t('diskCleanCreateRPAdmin'))}</div>`)
  })
}

function openSmartFixModal() {
  $('smartfix-modal')?.classList.remove('hidden')
}

function closeSmartFixModal() {
  $('smartfix-modal')?.classList.add('hidden')
}

function formatBytesGB(bytes) {
  const n = Number(bytes) || 0
  if (n <= 0) return '0'
  return fmt(n / (1024 * 1024 * 1024), 1)
}

function cleanupCatLabel(id) {
  const key = {
    shader_cache: 'cleanupCatShader',
    recycle: 'cleanupCatRecycle',
    app_cache: 'cleanupCatApps',
    user_temp: 'cleanupCatUserTemp',
    browser_cache: 'cleanupCatBrowser',
    thumbnails: 'cleanupCatThumbs',
    wer_reports: 'cleanupCatWER',
    package_cache: 'cleanupCatPackages',
    wu_cache: 'cleanupCatWU',
  }[id]
  return key ? t(key) : id
}

function formatCleanupSize(bytes) {
  const n = Number(bytes) || 0
  if (n <= 0) return t('unitsMB', { n: 0 })
  const gb = n / (1024 * 1024 * 1024)
  if (gb >= 0.1) return t('unitsGB', { n: fmt(gb, 1) })
  return t('unitsMB', { n: fmt(n / (1024 * 1024), 0) })
}

let smartFixPlanCache = null
let smartFixLastSession = ''
let smartFixCleanupCats = []

function impactSecondsLabel(impact) {
  const key = String(impact || '').toLowerCase()
  if (key === 'high') return t('sfStartupImpactHigh')
  if (key === 'medium') return t('sfStartupImpactMed')
  if (key === 'low') return t('sfStartupImpactLow')
  return t('sfStartupImpactUnk')
}

function appInitial(name) {
  const s = String(name || '?').trim()
  return (s[0] || '?').toUpperCase()
}

function bindSfAccordions(root) {
  root.querySelectorAll('[data-sf-acc]').forEach((btn) => {
    if (btn.dataset.boundAcc === '1') return
    btn.dataset.boundAcc = '1'
    btn.addEventListener('click', (ev) => {
      ev.preventDefault()
      ev.stopPropagation()
      const id = btn.getAttribute('data-sf-acc')
      const panel = id ? root.querySelector(`[data-sf-panel="${id}"]`) : null
      const open = btn.getAttribute('aria-expanded') !== 'true'
      btn.setAttribute('aria-expanded', open ? 'true' : 'false')
      btn.classList.toggle('open', open)
      if (panel) panel.classList.toggle('open', open)
      const chev = btn.querySelector('.sf-acc-chevron')
      if (chev) chev.textContent = open ? '▴' : '▾'
    })
  })
}

async function openSmartFixPlan() {
  const body = $('smartfix-body')
  const title = $('smartfix-title')
  if (!body) return
  if (title) title.textContent = t('smartFixTitle')
  body.innerHTML = `<div class="sf-loading">${t('smartFixLoading')}</div>`
  openSmartFixModal()
  let plan
  try { plan = await GetSmartFixPlan() } catch (err) {
    body.innerHTML = `<div class="sf-loading">${escapeHtml(String(err))}</div>`
    return
  }
  smartFixPlanCache = plan
  const steps = Array.isArray(plan?.steps) ? plan.steps : []
  if (!steps.length) {
    body.innerHTML = `<div class="sf-empty">${t('smartFixEmpty')}</div>`
    return
  }
  const cleanup = (Array.isArray(plan.cleanupPreview) ? plan.cleanupPreview : []).filter((c) => Number(c.sizeBytes) > 0 || c.id === 'recycle')
  smartFixCleanupCats = cleanup
  const cleanupStep = steps.find((s) => s.actionId === 'cleanup_safe')
  const hasCleanupStep = !!cleanupStep
  const cleanupStepId = cleanupStep ? escapeHtml(cleanupStep.id) : ''

  const cleanupHtml = plan.hasCleanup
    ? `<div class="sf-glass-block">
        <div class="sf-block-title">${t('cleanupPreview')}</div>
        <div class="sf-cleanup-list">${cleanup.map((c) => {
          const badges = [
            c.irreversible ? `<span class="sf-badge irr">${t('smartFixIrreversible')}</span>` : '',
            c.needsAdmin ? `<span class="sf-badge admin">${t('smartFixNeedsAdmin')}</span>` : '',
          ].filter(Boolean).join('')
          const checked = c.selected !== false && !c.needsAdmin ? 'checked' : ''
          const disabled = c.needsAdmin ? 'disabled' : ''
          return `<label class="sf-cleanup-row">
            <input type="checkbox" data-sf-cat="${escapeHtml(c.id)}" ${checked} ${disabled} />
            <span class="sf-cat-name">${escapeHtml(cleanupCatLabel(c.id))}</span>
            <span class="sf-cat-size">${escapeHtml(formatCleanupSize(c.sizeBytes))}</span>
            ${badges}
          </label>`
        }).join('')}</div>
        <button type="button" class="btn soft compact" id="sf-scan-folders">${t('largeFoldersTitle')}</button>
      </div>`
    : ''

  const startupSteps = steps.filter((s) => s.scenarioId === 'startup_item')
  const deviceStep = steps.find((s) => s.scenarioId === 'device_errors')
  const refreshStep = steps.find((s) => s.scenarioId === 'refresh_rate')
  const otherSteps = steps.filter((s) =>
    s.actionId !== 'cleanup_safe' &&
    s.scenarioId !== 'startup_item' &&
    s.scenarioId !== 'device_errors' &&
    s.scenarioId !== 'refresh_rate'
  )

  const startupItems = Array.isArray(plan.startupCandidates) && plan.startupCandidates.length
    ? plan.startupCandidates
    : startupSteps.map((s) => ({ id: s.actionArg, name: msgText(s.title), impact: 'high', advise: 'startupAdviseDisable' }))

  let startupHtml = ''
  if (startupSteps.length || startupItems.length) {
    const gain = startupSteps.reduce((n, s) => n + (Number(s.gainEstimate) || 0), 0)
    const rows = startupItems.map((it) => {
      const sid = startupSteps.find((s) => s.actionArg === it.id)?.id || `sf_startup_${it.id}`
      const name = it.name || it.id
      const checked = 'checked'
      return `<label class="sf-acc-item">
        <span class="sf-app-ico">${escapeHtml(appInitial(name))}</span>
        <span class="sf-acc-item-body">
          <span class="sf-acc-item-title">${escapeHtml(name)}</span>
          <span class="sf-acc-item-meta">${escapeHtml(impactSecondsLabel(it.impact))}</span>
          <span class="sf-acc-item-hint">${t('sfStartupSafeHint')}</span>
        </span>
        <input type="checkbox" data-sf-id="${escapeHtml(sid)}" ${checked} />
      </label>`
    }).join('')
    startupHtml = `<div class="sf-acc">
      <button type="button" class="sf-acc-head" data-sf-acc="startup" aria-expanded="false">
        <span class="sf-acc-title">${t('sfStartupAccTitle', { n: startupItems.length })} ${gain ? `<span class="problem-gain">+${gain}</span>` : ''}</span>
        <span class="sf-acc-chevron">▾</span>
      </button>
      <div class="sf-acc-panel" data-sf-panel="startup">${rows}</div>
    </div>`
  }

  const devices = Array.isArray(plan.devices) ? plan.devices : []
  let devicesHtml = ''
  if (deviceStep || devices.length) {
    const n = devices.length || Number(deviceStep?.title?.params?.n) || 0
    const rows = devices.map((d) => {
      const explain = msgText(d.explain) || t('pnpExplainGeneric', { code: d.errorCode })
      const act = msgText(d.action) || t('pnpActionProps')
      return `<div class="sf-acc-item sf-device-item">
        <span class="sf-app-ico">!</span>
        <span class="sf-acc-item-body">
          <span class="sf-acc-item-title">${escapeHtml(d.name || '')}</span>
          <span class="sf-acc-item-meta">${t('pnpErrorCode', { code: d.errorCode })}</span>
          <span class="sf-acc-item-hint">${escapeHtml(explain)}</span>
        </span>
        <button type="button" class="btn soft compact sf-device-act" data-pnp-action="${escapeHtml(d.actionId || 'device_props')}" data-pnp-id="${escapeHtml(d.deviceId || '')}">${escapeHtml(act)}</button>
      </div>`
    }).join('') || `<div class="sf-acc-item-hint">${escapeHtml(msgText(deviceStep?.why) || '')}</div>`
    devicesHtml = `<div class="sf-acc">
      <button type="button" class="sf-acc-head" data-sf-acc="devices" aria-expanded="false">
        <span class="sf-acc-title">${t('sfDeviceAccTitle', { n })} ${deviceStep?.gainEstimate ? `<span class="problem-gain">+${deviceStep.gainEstimate}</span>` : ''}</span>
        <span class="sf-acc-chevron">▾</span>
      </button>
      <div class="sf-acc-panel" data-sf-panel="devices">
        ${rows}
        ${deviceStep ? `<label class="sf-acc-item sf-acc-include"><input type="checkbox" data-sf-id="${escapeHtml(deviceStep.id)}" checked /> ${t('sfDeviceInclude')}</label>` : ''}
      </div>
    </div>`
  }

  let refreshHtml = ''
  if (refreshStep) {
    const max = refreshStep?.title?.params?.max || refreshStep?.actionArg?.split('|').pop() || ''
    const arg = refreshStep.actionArg || ''
    refreshHtml = `<div class="sf-glass-block sf-refresh-block">
      <div class="sf-step-body">
        <div class="sf-step-title">${escapeHtml(msgText(refreshStep.title))} ${refreshStep.gainEstimate ? `<span class="problem-gain">+${refreshStep.gainEstimate}</span>` : ''}</div>
        <div class="sf-step-why">${escapeHtml(msgText(refreshStep.why))}</div>
      </div>
      <button type="button" class="btn soft compact" id="sf-open-monitors" data-refresh-arg="${escapeHtml(arg)}">${t('homeActionSetRefresh', { max })}</button>
    </div>`
  }

  const otherHtml = otherSteps.map((s) => {
    const titleText = msgText(s.title)
    const whyText = msgText(s.why)
    const badges = [
      s.reversible ? `<span class="sf-badge ok">${t('smartFixReversible')}</span>` : '',
      s.irreversible ? `<span class="sf-badge irr">${t('smartFixIrreversible')}</span>` : '',
      s.needsAdmin ? `<span class="sf-badge admin">${t('smartFixNeedsAdmin')}</span>` : '',
      s.gainEstimate > 0 ? `<span class="problem-gain">+${s.gainEstimate}</span>` : '',
    ].filter(Boolean).join('')
    return `<label class="sf-step">
      <input type="checkbox" data-sf-id="${escapeHtml(s.id)}" ${s.selected ? 'checked' : ''} />
      <span class="sf-step-body">
        <span class="sf-step-title">${escapeHtml(titleText)} ${badges}</span>
        <span class="sf-step-why">${escapeHtml(whyText)}</span>
      </span>
    </label>`
  }).join('')

  body.innerHTML = `
    <div class="sf-plan">
      <div class="sf-score-hint">${t('smartFixScoreHint', { now: plan.scoreNow, next: plan.scoreIfApplied })}</div>
      ${cleanupHtml}
      ${startupHtml}
      ${devicesHtml}
      ${refreshHtml}
      ${otherHtml ? `<div class="sf-steps">${otherHtml}</div>` : ''}
      <div class="btn-row sf-actions">
        <button type="button" class="btn primary" id="sf-apply" data-cleanup-step="${cleanupStepId}" data-has-cleanup="${hasCleanupStep ? '1' : '0'}">${t('smartFixApply')}</button>
        <button type="button" class="btn soft" id="sf-cancel">${t('smartFixCancel')}</button>
      </div>
    </div>`
  bindSfAccordions(body)
  body.querySelector('#sf-cancel')?.addEventListener('click', closeSmartFixModal)
  body.querySelector('#sf-apply')?.addEventListener('click', () => applySmartFixFromUI().catch(() => {}))
  body.querySelector('#sf-scan-folders')?.addEventListener('click', async (ev) => {
    ev.preventDefault()
    ev.stopPropagation()
    closeSmartFixModal()
    openDiskCleanupWizard('apps').catch(() => {})
  })
  body.querySelector('#sf-open-monitors')?.addEventListener('click', (ev) => {
    ev.preventDefault()
    ev.stopPropagation()
    const arg = ev.currentTarget.getAttribute('data-refresh-arg') || ''
    const [dev, hzStr] = String(arg).split('|')
    closeSmartFixModal()
    openMonitorsModal(dev || '', Number(hzStr) || 0).catch(() => {})
  })
  body.querySelectorAll('.sf-device-act').forEach((btn) => {
    btn.addEventListener('click', async (ev) => {
      ev.preventDefault()
      ev.stopPropagation()
      const action = btn.getAttribute('data-pnp-action') || 'device_props'
      const id = btn.getAttribute('data-pnp-id') || ''
      btn.disabled = true
      try { await OpenPnPDeviceAction(action, id) } catch (_) {}
      btn.disabled = false
      showToast(t('pnpActionDone'), true)
    })
  })
}

async function applySmartFixFromUI() {
  const body = $('smartfix-body')
  if (!body) return
  const ids = [...body.querySelectorAll('input[data-sf-id]:checked')].map((el) => el.getAttribute('data-sf-id')).filter(Boolean)
  const applyBtn = body.querySelector('#sf-apply')
  const cleanupStepId = applyBtn?.getAttribute('data-cleanup-step') || ''
  const selectedCats = [...body.querySelectorAll('input[data-sf-cat]:checked')].map((el) => el.getAttribute('data-sf-cat')).filter(Boolean)
  if (cleanupStepId && selectedCats.length) {
    ids.push(cleanupStepId)
    // Pass selected categories via rewriting step arg on backend: store on window for Apply override
    window.__sfCleanupCats = selectedCats
  } else {
    window.__sfCleanupCats = null
  }
  body.innerHTML = `<div class="sf-loading">${t('smartFixApplying')}</div>`
  let res
  try {
    // If cleanup cats chosen, call ApplyDiskCleanup path through ApplySmartFix with step id;
    // backend uses scenario ActionArg — override by calling ApplyDiskCleanup first when only cats matter.
    if (window.__sfCleanupCats?.length && cleanupStepId) {
      // Patch: apply cleanup with selected cats, then other steps without cleanup_safe
      const otherIds = ids.filter((id) => id !== cleanupStepId)
      const cleanRes = await ApplyDiskCleanup(window.__sfCleanupCats)
      window.__lastCleanCompare = {
        usedBefore: cleanRes?.usedBefore || 0,
        usedAfter: cleanRes?.usedAfter || 0,
        freed: cleanRes?.actualCleanedBytes || cleanRes?.freedBytes || 0,
      }
      res = otherIds.length ? await ApplySmartFix(otherIds) : {
        ok: !!(cleanRes?.ok || cleanRes?.actualCleanedBytes > 0 || cleanRes?.freedBytes > 0),
        sessionId: '',
        scoreBefore: smartFixPlanCache?.scoreNow || 0,
        scoreAfter: smartFixPlanCache?.scoreNow || 0,
        freedBytes: cleanRes?.actualCleanedBytes || cleanRes?.freedBytes || 0,
        freedRAMMB: 0,
        canRevert: false,
        applied: [{
          ok: true, scenarioId: 'disk_full',
          title: { code: 'smartFixCleanupVerdict' },
          detail: { code: 'smartFixCleanupDone', params: { gb: formatBytesSmart(cleanRes?.actualCleanedBytes || cleanRes?.freedBytes || 0, true) } },
        }],
        message: { code: 'smartFixCleanupVerdict' },
      }
      if (otherIds.length && cleanRes) {
        res.freedBytes = (Number(res.freedBytes) || 0) + (Number(cleanRes.actualCleanedBytes || cleanRes.freedBytes) || 0)
        res.applied = [
          {
            ok: true, scenarioId: 'disk_full',
            title: { code: 'smartFixCleanupVerdict' },
            detail: { code: 'smartFixCleanupDone', params: { gb: formatBytesSmart(cleanRes.actualCleanedBytes || cleanRes.freedBytes || 0, true) } },
          },
          ...(Array.isArray(res.applied) ? res.applied.filter((a) => a.scenarioId !== 'disk_full') : []),
        ]
      }
      // Refresh score after cleanup
      try {
        const st = await GetHomeState()
        if (st) {
          res.scoreAfter = Number(st.score) || res.scoreAfter
          if (!res.scoreBefore) res.scoreBefore = smartFixPlanCache?.scoreNow || res.scoreAfter
        }
      } catch (_) {}
    } else {
      res = await ApplySmartFix(ids)
    }
  } catch (err) {
    body.innerHTML = `<div class="sf-loading">${escapeHtml(String(err))}</div>`
    return
  }
  if (res?.openTab === 'monitors') {
    closeSmartFixModal()
    openMonitorsModal().catch(() => {})
    return
  }
  renderSmartFixResult(res)
}

function summarizeSmartFixApplied(applied, freedBytes) {
  const lines = []
  const ok = (applied || []).filter((a) => a.ok)
  const startupN = ok.filter((a) => String(a.scenarioId || '').startsWith('startup') || a.scenarioId === 'startup_item').length
  const cleaned = ok.find((a) => a.scenarioId === 'disk_full')
  const refresh = ok.find((a) => a.scenarioId === 'refresh_rate')
  if (refresh) {
    const hz = refresh.detail?.params?.hz || ''
    lines.push({ kind: 'refresh', text: t('smartFixResultRefreshLine', { hz: hz || msgText(refresh.detail) }) })
  }
  if (cleaned || Number(freedBytes) > 0) {
    lines.push({ kind: 'cleanup', text: t('smartFixResultFreedLine', { gb: formatBytesSmart(freedBytes, true) }) })
  }
  if (startupN > 0) {
    lines.push({ kind: 'startup', text: t('smartFixResultStartupLine', { n: startupN }) })
  }
  for (const a of ok) {
    if (a.scenarioId === 'disk_full' || a.scenarioId === 'startup_item' || String(a.scenarioId || '').startsWith('startup')) continue
    if (a.scenarioId === 'refresh_rate') continue
    if (a.scenarioId === 'device_errors') {
      lines.push({ kind: 'device', text: msgText(a.title) || t('sfDeviceErrTitle', { n: 1 }) })
      continue
    }
    const detail = msgText(a.detail)
    const title = msgText(a.title)
    lines.push({ kind: 'other', text: detail || title })
  }
  return lines
}

function renderSmartFixResult(res) {
  const body = $('smartfix-body')
  if (!body) return
  smartFixLastSession = res?.sessionId || ''
  const title = $('smartfix-title')
  if (title) title.textContent = t('smartFixResultTitle')
  const before = Number(res?.scoreBefore) || 0
  const after = Number(res?.scoreAfter) || before
  const delta = after - before
  const freedGB = formatBytesSmart(res?.freedBytes, true)
  const lines = summarizeSmartFixApplied(res?.applied, res?.freedBytes)
  const listHtml = lines.map((l) => {
    if (l.kind === 'device') {
      return `<div class="sf-result-row ok">
        <span>✓ ${escapeHtml(l.text)}</span>
        <button type="button" class="btn soft compact" id="sf-open-devmgr">${t('smartFixOpenDeviceMgr')}</button>
      </div>`
    }
    return `<div class="sf-result-row ok">✓ ${escapeHtml(l.text)}</div>`
  }).join('')
  const revertBtn = res?.canRevert
    ? `<button type="button" class="btn soft" id="sf-revert">${t('smartFixRevert')}</button>`
    : ''
  const cmp = window.__lastCleanCompare || {}
  const compareHtml = cleanCompareCardHTML(cmp.usedBefore, cmp.usedAfter, cmp.freed || res?.freedBytes || 0)
  body.innerHTML = `
    <div class="sf-result">
      <div class="sf-verdict-card">
        <div class="sf-verdict-title">${t('smartFixDone')}</div>
        ${compareHtml || ''}
        <div class="sf-verdict-stats">
          ${!compareHtml && Number(res?.freedBytes) > 0 ? `<div class="sf-stat"><span class="sf-stat-lbl">${t('smartFixFreedLabel')}</span><b>${escapeHtml(t('unitsGB', { n: freedGB }))}</b></div>` : ''}
          <div class="sf-stat"><span class="sf-stat-lbl">${t('smartFixScoreLabel')}</span><b>${t('smartFixScoreGrew', { before, after })}</b></div>
        </div>
      </div>
      <div class="sf-result-list">${listHtml || `<div class="sf-result-row">${escapeHtml(msgText(res?.message) || '')}</div>`}</div>
      <div class="btn-row sf-actions">
        ${revertBtn}
        <button type="button" class="btn primary" id="sf-done">${t('smartFixClose')}</button>
      </div>
    </div>`
  body.querySelector('#sf-done')?.addEventListener('click', () => {
    closeSmartFixModal()
    renderHome().catch(() => {})
  })
  body.querySelector('#sf-revert')?.addEventListener('click', async () => {
    if (!smartFixLastSession) return
    let r
    try { r = await RevertSmartFix(smartFixLastSession) } catch (err) {
      showToast(String(err), false)
      return
    }
    showToast(msgText(r?.message) || t('smartFixRevertOK', { before: r?.scoreBefore, after: r?.scoreAfter }), !!r?.ok)
    closeSmartFixModal()
    renderHome().catch(() => {})
  })
  body.querySelector('#sf-open-devmgr')?.addEventListener('click', async () => {
    try { await RunHomeAction('open_settings', 'devmgmt.msc') } catch (_) {
      try { BrowserOpenURL('ms-settings:devices') } catch (__) {}
    }
  })
}

function revealDiskSpecs(letter, highlightAttrId = 0) {
  const L = String(letter || 'C').replace(':', '').toUpperCase()
  const tile = $('tile-disks')
  const toggle = tile?.querySelector('.home-tile-toggle')
  if (tile && !tile.classList.contains('open')) toggle?.click()
  setTimeout(() => {
    const exp = document.querySelector(`.disk-expand-btn[data-drive="${L}"]`)
    if (exp && exp.getAttribute('aria-expanded') !== 'true') exp.click()
    else fillDiskDetails(L)
    const card = document.querySelector(`.disk-card[data-drive="${L}"]`)
    card?.scrollIntoView({ behavior: 'smooth', block: 'nearest' })
    if (highlightAttrId) {
      setTimeout(() => {
        const row = document.querySelector(`[data-drive-details="${L}"] tr[data-smart-id="${highlightAttrId}"]`)
        row?.scrollIntoView({ behavior: 'smooth', block: 'nearest' })
        row?.classList.add('smart-row-hl')
      }, 250)
    }
  }, 200)
}

async function openDiskAdvicePanel(letter) {
  const L = String(letter || 'C').replace(':', '').toUpperCase()
  let r = cachedDiskReport[L]
  if (!r || r._loading) {
    try { r = await CheckDiskHealth(L) } catch (_) { r = null }
    if (r) cachedDiskReport[L] = r
  }
  let watch = { watching: false, trendCode: '', from: 0, to: 0, days: 0 }
  try { watch = await GetDiskWatchInfo(L) || watch } catch (_) {}
  const n = Math.max(Number(r?.pendingSectors) || 0, Number(r?.reallocatedSectors) || 0)
  const reason = msgText(r?.statusReason, r?.statusReasonRu, r?.statusReasonEn)
    || t('diskReasonPendingHuman', { n: n || 2 })
  const explain = t('diskAdviceExplain', { n: n || 2 })
  let trend = ''
  if (watch.trendCode === 'diskTrendPendingGrew' || watch.trendCode === 'diskTrendReallocatedGrew') {
    trend = t(watch.trendCode, { from: watch.from, to: watch.to })
  } else if (watch.trendCode === 'diskTrendStableDays' && watch.days > 0) {
    trend = t('diskTrendStableDays', { n: watch.days })
  } else {
    trend = t('diskTrendNoHistory')
  }
  const watching = !!watch.watching
  openModal(`
    <div class="disk-advice-panel">
      <div class="panel-title">${t('diskAdviceTitle', { letter: L })}</div>
      <div class="advice-explain">${escapeHtml(explain)}</div>
      <div class="advice-meta">${escapeHtml(t('diskAdviceCurrent', { reason }))}</div>
      <div class="advice-meta">${escapeHtml(trend)}</div>
      <div class="advice-steps">
        <div class="advice-step">
          <span class="advice-step-text">${escapeHtml(t('diskAdviceStepBackup'))}</span>
          <button type="button" class="btn soft compact" data-advice="backup">${escapeHtml(t('diskAdviceOpenBackup'))}</button>
        </div>
        <div class="advice-step">
          <span class="advice-step-text">${escapeHtml(t('diskAdviceStepWatch'))}</span>
          <button type="button" class="btn soft compact" data-advice="watch" ${watching ? 'disabled' : ''}>${escapeHtml(watching ? t('diskAdviceWatchOn') : t('diskAdviceEnableWatch'))}</button>
        </div>
        <div class="advice-step">
          <span class="advice-step-text">${escapeHtml(t('diskAdviceStepSpecs'))}</span>
          <button type="button" class="btn soft compact" data-advice="specs">${escapeHtml(t('diskAdviceShowSpecs'))}</button>
        </div>
      </div>
    </div>
  `)
  const body = $('modal-body')
  body?.querySelectorAll('[data-advice]').forEach((b) => {
    b.addEventListener('click', async () => {
      const kind = b.getAttribute('data-advice')
      if (kind === 'backup') {
        try { await OpenWindowsBackup() } catch (_) {
          try { BrowserOpenURL('ms-settings:backup') } catch (__) {}
        }
        showToast(t('diskAdviceBackupOpened'), true, { holdMs: 4000 })
      } else if (kind === 'watch') {
        try { await SetDiskWatching(L, true) } catch (_) {}
        b.disabled = true
        b.textContent = t('diskAdviceWatchOn')
        showToast(t('diskWatchEnabled', { letter: L }), true, { holdMs: 4000 })
      } else if (kind === 'specs') {
        closeModal()
        revealDiskSpecs(L, Number(r?.triggerAttrId) || 197)
      }
    })
  })
}

function closeModal() {
  $('modal').classList.add('hidden')
}

function formatDiskCheckedAt(d = new Date()) {
  try {
    return new Intl.DateTimeFormat(lang === 'en' ? 'en-GB' : 'ru-RU', {
      day: '2-digit', month: '2-digit', year: 'numeric',
      hour: '2-digit', minute: '2-digit',
    }).format(d)
  } catch (_) {
    return d.toLocaleString()
  }
}

function saveDiskHistory(key, r) {
  const status = r.statusWordRu || r.verdictBadge || r.verdict || r.errorMessage || t('noData')
  diskHistory[key] = {
    lastChecked: formatDiskCheckedAt(),
    lastStatus: status,
    healthPercent: r.healthPercent || 0,
  }
  localStorage.setItem('pulsecore_disk_history', JSON.stringify(diskHistory))
}

function renderDiskHealthBody(r) {
  const letter = driveLetter(r.driveLetter || selectedDrive)
  fillDiskDetails(letter)
  const host = document.querySelector(`[data-drive-details="${letter}"]`)
  return host ? host.innerHTML : `<div class="meta">${t('diskChecking')}</div>`
}

async function runDiskHealth(drive, force = false) {
  if (drive) selectedDrive = driveLetter(drive)
  await refreshAllDiskSmart(force)
  const key = (selectedDrive || 'C').toUpperCase()
  const card = document.querySelector(`.disk-card[data-drive="${key}"]`)
  const btn = card?.querySelector('.disk-expand-btn')
  const details = document.querySelector(`[data-drive-details="${key}"]`)
  if (btn && details) {
    details.classList.remove('hidden')
    details.hidden = false
    btn.setAttribute('aria-expanded', 'true')
    fillDiskDetails(key)
    card?.scrollIntoView({ behavior: 'smooth', block: 'nearest' })
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
  document.addEventListener('click', async (ev) => {
    const btn = ev.target.closest?.('.tile-error-copy')
    if (!btn) return
    const pre = btn.parentElement?.querySelector?.('.tile-error-log')
    const text = pre?.textContent || btn.getAttribute('data-tile-diag') || ''
    try {
      await navigator.clipboard.writeText(String(text))
      showToast(t('advDiagCopied'), true, { holdMs: 2500 })
    } catch (err) {
      showToast(String(err), false, { holdMs: 4000 })
    }
  })

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
    try {
      if (typeof TitlebarToggleMaximise === 'function') await TitlebarToggleMaximise()
      else await ToggleFullscreen()
    } catch (_) {
      try { await ToggleFullscreen() } catch (__) {}
    }
    syncMaximisedChrome().catch(() => {})
  })
  $('btn-close').addEventListener('click', async () => {
    try {
      const p = await GetBehaviorSettings()
      if (p && p.closeToTray !== false) await HideToTray()
      else await QuitApp()
    } catch (_) {
      try { await HideToTray() } catch (__) {}
    }
  })
  $('brand-link')?.addEventListener('click', openGitHub)
  $('titlebar-brand')?.addEventListener('click', openGitHub)
  wireTitlebarNativeDrag()
  syncMaximisedChrome().catch(() => {})
  setInterval(() => { syncMaximisedChrome().catch(() => {}) }, 800)

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
  document.querySelectorAll('.theme-opt').forEach((btn) => {
    btn.addEventListener('click', () => applyTheme(btn.dataset.theme))
  })
  wireCustomDropdowns()
  ;['opt-autostart', 'opt-start-min', 'opt-close-tray', 'opt-tray-temp', 'opt-conflict-watcher', 'toggle-conflict-watcher', 'opt-temp-alert', 'opt-eco-exit'].forEach((id) => {
    $(id)?.addEventListener('change', saveBehavior)
  })
  $('toggle-conflict-watcher')?.addEventListener('change', async (ev) => {
    try { await SetConflictWatcherEnabled(!!ev.target.checked) } catch (err) { console.error(err) }
  })
  $('btn-adv-diag-toggle')?.addEventListener('click', () => {
    const panel = $('adv-diag-panel')
    const btn = $('btn-adv-diag-toggle')
    if (!panel || !btn) return
    const open = panel.classList.contains('hidden') || panel.hidden
    panel.classList.toggle('hidden', !open)
    panel.hidden = !open
    btn.setAttribute('aria-expanded', open ? 'true' : 'false')
    btn.classList.toggle('open', open)
  })
  $('btn-about-report-toggle')?.addEventListener('click', () => { toggleReportPreview().catch(() => {}) })
  $('btn-report-copy')?.addEventListener('click', () => { copySupportReport().catch(() => {}) })

  try {
    EventsOn('ui:tray-tip', () => {
      showToast(t('trayTip'), true, { holdMs: 8000 })
    })
  } catch (_) {}

  $('btn-summary-png')?.addEventListener('click', () => { copySummaryPng().catch(() => {}) })
  $('btn-game-refresh')?.addEventListener('click', () => { renderGamesTab().catch(() => {}) })
  $('btn-game-compare')?.addEventListener('click', () => { runGameCompare().catch(() => {}) })
  $('btn-clear-stats')?.addEventListener('click', async () => {
    const ok = await glassConfirm({
      title: t('clearStats'),
      body: t('clearStatsConfirm'),
      confirmLabel: t('commonYes'),
      danger: true,
    })
    if (!ok) return
    try {
      await ClearStats()
      showToast(t('clearStats'), true, { holdMs: 4000 })
      renderSummary().catch(() => {})
    } catch (err) {
      showToast(String(err), false)
    }
  })

  $('btn-radar-clear')?.addEventListener('click', async () => {
    try { await ClearProcessAnomalies() } catch (_) {}
    renderRadarPanel().catch(() => {})
  })
  $('btn-startup-refresh')?.addEventListener('click', () => { renderStartupList().catch(() => {}) })
  // Report UI lives in About / Help (toggle + copy for support).
  $('btn-intel-reset')?.addEventListener('click', async () => {
    const ok = await glassConfirm({
      title: t('intelReset'),
      body: t('intelResetConfirm'),
      confirmLabel: t('commonYes'),
      danger: true,
    })
    if (!ok) return
    try { await ResetIntelligenceProfile() } catch (_) {}
    renderIntelligence().catch(() => {})
  })
  $('startup-list')?.addEventListener('click', async (ev) => {
    const btn = ev.target.closest('[data-startup-id]')
    if (!btn) return
    const id = btn.getAttribute('data-startup-id')
    const name = btn.getAttribute('data-startup-name') || id
    const enable = btn.getAttribute('data-startup-on') === '1'
    const msg = (enable ? t('startupConfirmOn') : t('startupConfirmOff')).replace('{name}', name)
    const ok = await glassConfirm({
      title: name,
      body: msg,
      confirmLabel: enable ? t('startupEnable') : t('startupDisable'),
      danger: !enable,
    })
    if (!ok) return
    btn.disabled = true
    try {
      const res = await SetStartupItemEnabled(id, enable)
      showToast(resultMsg(res), !!res?.ok, { holdMs: 5000 })
    } catch (err) {
      showToast(String(err), false, { holdMs: 5000 })
    }
    renderStartupList().catch(() => {})
  })
  $('btn-eco-revert-all')?.addEventListener('click', async () => {
    let res
    try { res = await RevertAllEco() } catch (err) { res = { ok: false, messageRu: String(err), messageEn: String(err) } }
    showToast(ecoMsg(res), !!res.ok, { holdMs: 6000 })
    renderEcoLimitedPanel().catch(() => {})
    refresh()
  })

  $('eco-limited-list')?.addEventListener('click', async (ev) => {
    const btn = ev.target.closest('[data-eco-group-revert]')
    if (!btn) return
    btn.disabled = true
    let res
    try { res = await RevertEcoGroup(btn.dataset.ecoGroupRevert) } catch (err) {
      res = { ok: false, messageRu: String(err), messageEn: String(err) }
    }
    showToast(ecoMsg(res), !!res.ok, { holdMs: 6000 })
    renderEcoLimitedPanel().catch(() => {})
    refresh()
  })

  const hideEcoExit = () => $('eco-exit-modal')?.classList.add('hidden')
  $('eco-exit-cancel')?.addEventListener('click', hideEcoExit)
  $('eco-exit-stay')?.addEventListener('click', hideEcoExit)
  $('eco-exit-clear')?.addEventListener('click', async () => {
    hideEcoExit()
    try { await ConfirmQuitAfterEco(true) } catch (_) {}
  })
  $('eco-exit-keep')?.addEventListener('click', async () => {
    hideEcoExit()
    try { await ConfirmQuitAfterEco(false) } catch (_) {}
  })
  try {
    EventsOn('eco:exit-prompt', () => {
      $('eco-exit-modal')?.classList.remove('hidden')
    })
  } catch (_) {}

  setupPowerRadiogroup()
  document.querySelectorAll('.btn.mode').forEach((btn) => {
    // Legacy mode buttons outside home power tile (if any)
    btn.addEventListener('click', async (ev) => {
      if (ev.target.closest('.tip')) return
      if (btn.classList.contains('power-card')) return
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

  $('btn-hud-side')?.addEventListener('click', enableHud)
  $('btn-home-hud')?.addEventListener('click', enableHud)
  $('btn-home-cleanup')?.addEventListener('click', () => {
    openDiskCleanupWizard('cache').catch(() => {})
  })
  $('btn-hidden-games')?.addEventListener('click', () => openHiddenGamesModal())
  $('hidden-games-close')?.addEventListener('click', closeHiddenGamesModal)
  $('hidden-games-modal')?.addEventListener('click', (ev) => {
    if (ev.target?.id === 'hidden-games-modal') closeHiddenGamesModal()
    const btn = ev.target?.closest?.('[data-restore-game]')
    if (btn) restoreHiddenGame(btn.getAttribute('data-restore-game'))
  })
  const mh = $('minihud')
  mh?.addEventListener('contextmenu', (ev) => {
    if (!hudEnabled) return
    ev.preventDefault()
    $('mh-menu')?.classList.toggle('hidden')
  })
  $('mh-menu')?.addEventListener('click', async (ev) => {
    const item = ev.target?.closest?.('[data-mh-opacity],[data-mh-close]')
    if (!item) return
    if (item.hasAttribute('data-mh-close')) {
      $('mh-menu')?.classList.add('hidden')
      try { await ToggleMiniHUD(false) } catch (_) { setHudMode(false) }
      return
    }
    const pct = Number(item.getAttribute('data-mh-opacity')) || 80
    mh.dataset.opacity = String(pct)
    try { await SetMiniHUDOpacity(pct) } catch (_) { /* ignore */ }
    $('mh-menu')?.classList.add('hidden')
  })
  document.querySelectorAll('.home-tile').forEach((tile) => {
    if (uiExpand.tiles.has(tile.id)) {
      tile.classList.add('open')
      tile.querySelector('.home-tile-toggle')?.setAttribute('aria-expanded', 'true')
    }
  })
  document.querySelectorAll('.home-tile-toggle').forEach((btn) => {
    if (btn.dataset.boundToggle === '1') return
    btn.dataset.boundToggle = '1'
    btn.addEventListener('click', (ev) => {
      ev.preventDefault()
      ev.stopPropagation()
      const tile = btn.closest('.home-tile')
      if (!tile) return
      const open = tile.classList.toggle('open')
      btn.setAttribute('aria-expanded', open ? 'true' : 'false')
      if (tile.id) {
        if (open) uiExpand.tiles.add(tile.id)
        else uiExpand.tiles.delete(tile.id)
      }
    })
  })
  $('home-findings')?.addEventListener('click', async (ev) => {
    const more = ev.target.closest('.problem-more, .btn-details')
    if (more) {
      ev.preventDefault()
      ev.stopPropagation()
      const pid = more.getAttribute('data-problem-id') || ''
      const id = more.getAttribute('aria-controls')
      const body = id ? document.getElementById(id) : null
      const open = more.getAttribute('aria-expanded') !== 'true'
      more.setAttribute('aria-expanded', open ? 'true' : 'false')
      if (body) {
        body.classList.toggle('open', open)
        body.classList.toggle('hidden', !open)
        body.hidden = !open
      }
      if (pid) {
        if (open) uiExpand.problems.add(pid)
        else uiExpand.problems.delete(pid)
      }
      return
    }
    const btn = ev.target.closest('[data-home-action]')
    if (!btn || btn.disabled) return
    const action = btn.getAttribute('data-home-action') || ''
    const arg = btn.getAttribute('data-home-arg') || ''
    const actionsHost = btn.closest('.home-prepare-actions') || btn.parentElement
    btn.disabled = true
    const labelEl = btn.querySelector('.home-prepare-label')
    if (action === 'prepare_game' || action === 'revert_prepare') {
      // Immediate honest UI: indeterminate progress, no fake %.
      if (actionsHost) {
        const label = action === 'prepare_game' ? t('homePrepareLoading') : t('homeRevertLoading')
        actionsHost.innerHTML = `
          <div class="home-prepare-progress" role="status" aria-live="polite" aria-busy="true">
            <div class="home-prepare-progress-label">${escapeHtml(label)}</div>
            <div class="home-prepare-track" aria-hidden="true"><div class="home-prepare-fill"></div></div>
          </div>`
      } else {
        btn.classList.add('is-loading')
        if (labelEl) labelEl.textContent = t('homePrepareLoading')
      }
    }
    let res
    try { res = await RunHomeAction(action, arg) } catch (err) {
      res = { ok: false, message: { code: 'errGeneric', params: { err: String(err) } } }
    }
    if (action === 'disk_advice') {
      openDiskAdvicePanel(arg || 'C').catch(() => {})
      btn.disabled = false
      return
    }
    if (action === 'free_disk' || res?.openTab === 'disk_cleanup') {
      openDiskCleanupWizard().catch(() => {})
      btn.disabled = false
      return
    }
    if (action === 'set_refresh_rate' || res?.openTab === 'monitors') {
      const a = btn.getAttribute('data-home-arg') || ''
      const [dev, hzStr] = String(a).split('|')
      openMonitorsModal(dev || '', Number(hzStr) || 0).catch(() => {})
      btn.disabled = false
      return
    }
    if ((action === 'prepare_game' || action === 'revert_prepare') && res && res.ok === false && actionsHost) {
      const errText = resultMsg(res) || t('homePrepareFail')
      actionsHost.innerHTML = `
        <div class="home-prepare-result err" role="alert">
          <div class="home-prepare-result-title">${escapeHtml(errText)}</div>
          <button type="button" class="btn primary home-prepare-btn" data-home-action="prepare_game">
            <span class="home-prepare-bolt" aria-hidden="true">⚡</span>
            <span class="home-prepare-label">${t('homePrepare')}</span>
          </button>
          <div class="meta home-prepare-hint">${t('homePrepareHint')}</div>
        </div>`
      showToast(errText, false, { holdMs: 6000 })
      return
    }
    const msg = resultMsg(res)
    if (msg) showToast(msg, !!res?.ok, { holdMs: res?.canRevert ? 12000 : 5000 })
    if (res?.openTab) setActiveTab(res.openTab)
    if (action === 'open_disk') {
      revealDiskSpecs(arg || 'C')
    }
    // Clear in-flight marker so renderHome can rebuild result/restore UI.
    actionsHost?.querySelector('.home-prepare-progress')?.remove()
    btn.classList.remove('is-loading')
    btn.disabled = false
    renderHome().catch(() => {})
  })
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

  $('btn-smart-fix')?.addEventListener('click', () => { openSmartFixPlan().catch(() => {}) })
  $('smartfix-close')?.addEventListener('click', closeSmartFixModal)
  $('diskclean-close')?.addEventListener('click', closeDiskCleanModal)
  $('diskclean-modal')?.addEventListener('click', (ev) => {
    if (ev.target?.id === 'diskclean-modal') closeDiskCleanModal()
  })
  $('smartfix-modal')?.addEventListener('click', (ev) => {
    if (ev.target.id === 'smartfix-modal') closeSmartFixModal()
  })
  $('monitors-close')?.addEventListener('click', closeMonitorsModal)
  $('monitors-modal')?.addEventListener('click', (ev) => {
    if (ev.target?.id === 'monitors-modal') closeMonitorsModal()
  })
  $('hz-trial-confirm')?.addEventListener('click', () => { confirmHzTrial().catch(() => {}) })
  $('hz-trial-revert')?.addEventListener('click', () => { revertHzTrial().catch(() => {}) })
  try {
    EventsOn('display-refresh-reverted', () => {
      closeHzTrialModal()
      hzTrialState = null
      showToast(t('hzTrialAutoReverted'), true, { holdMs: 4000 })
      if (!$('monitors-modal')?.classList.contains('hidden')) {
        openMonitorsModal().catch(() => {})
      }
      renderHome().catch(() => {})
    })
  } catch (_) {}
  $('confirm-close')?.addEventListener('click', () => closeConfirmModal(false))
  $('confirm-cancel')?.addEventListener('click', () => closeConfirmModal(false))
  $('confirm-ok')?.addEventListener('click', () => closeConfirmModal(true))
  $('confirm-modal')?.addEventListener('click', (ev) => {
    if (ev.target?.id === 'confirm-modal') closeConfirmModal(false)
  })
  try {
    EventsOn('delete-progress', (ev) => {
      updateDeleteProgress(ev || {})
      if (ev?.phase === 'cleanup' && ev?.label) {
        const text = $('delete-progress-text')
        if (text) text.textContent = t('deleteProgressWorking', { name: ev.label })
      }
      if (ev?.phase === 'watching' && ev?.label) {
        const text = $('delete-progress-text')
        if (text) text.textContent = t('diskCleanWatchingLauncher', { name: ev.label })
        showToast(t('diskCleanWatchingLauncher', { name: ev.label }), true, { holdMs: 6000 })
      }
      if (ev?.phase === 'done') {
        window.__lastCleanCompare = {
          usedBefore: ev?.usedBefore || 0,
          usedAfter: ev?.usedAfter || 0,
          freed: ev?.freedBytes || 0,
        }
        try { RefreshDiskMetricsEx() } catch (_) {}
        renderHome().catch(() => {})
        if (typeof window.__dcRefreshAfterDelete === 'function') {
          window.__dcRefreshAfterDelete().catch?.(() => {})
          window.__dcRefreshAfterDelete = null
        }
      }
    })
    EventsOn('app-deleted-success', (ev) => {
      const id = ev?.appId || window.__dcPendingDeleteId || ''
      const row = id ? document.querySelector(`.dc-app-row[data-app-id="${CSS.escape(id)}"]`) : null
      if (row) {
        row.classList.add('fade-out')
        setTimeout(() => row.remove(), 320)
      }
      window.__dcPendingDeleteId = null
      try { RefreshDiskMetricsEx() } catch (_) {}
      renderHome().catch(() => {})
    })
    EventsOn('external-app-deleted', (ev) => {
      const name = ev?.name || ''
      const freed = Number(ev?.freed_bytes || ev?.freedBytes || 0)
      const id = ev?.appId || window.__dcPendingDeleteId || ''
      showToast(t('diskCleanExternalDeleted', {
        name,
        size: formatBytesSmart(freed, true),
      }), true, { holdMs: 7000 })
      window.__lastCleanCompare = {
        usedBefore: 0,
        usedAfter: 0,
        freed,
      }
      const row = id
        ? document.querySelector(`.dc-app-row[data-app-id="${CSS.escape(id)}"]`)
        : [...document.querySelectorAll('.dc-app-row')].find((el) =>
          (el.querySelector('.dc-app-name')?.textContent || '') === name)
      if (row) {
        row.classList.add('fade-out')
        setTimeout(() => row.remove(), 320)
      }
      window.__dcPendingDeleteId = null
      try { RefreshDiskMetricsEx() } catch (_) {}
      renderHome().catch(() => {})
      const body = $('diskclean-body')
      if (body && typeof renderDiskCleanApps === 'function') {
        renderCleanStatsBanner(body.querySelector('#dc-stats-slot')).catch(() => {})
      }
      if (typeof window.__dcRefreshAfterDelete === 'function') {
        window.__dcRefreshAfterDelete().catch?.(() => {})
        window.__dcRefreshAfterDelete = null
      }
    })
    EventsOn('recycle-bin-updated', (ev) => {
      const b = Number(ev?.bytes) || 0
      const sizeEl = document.querySelector('#dc-recycle-size')
      if (sizeEl) sizeEl.textContent = b > 0 ? formatBytesSmart(b) : t('recycleBinEmptyStatus')
      const emptyBtn = document.querySelector('#dc-empty-recycle')
      if (emptyBtn) emptyBtn.disabled = b <= 0
      if (ev?.lastTrashed) {
        lastTrashedLeftover = { name: ev.lastTrashed, path: ev.path }
        const banner = document.querySelector('#dc-leftover-undo-banner')
        if (banner) {
          const header = banner.querySelector('.dc-undo-header')
          if (header) header.textContent = t('dcUndoHeader', { name: ev.lastTrashed })
          banner.classList.remove('hidden')
        }
      }
    })
  } catch (_) {}
  window.addEventListener('focus', () => {
    const modal = $('diskclean-modal')
    if (modal && !modal.classList.contains('hidden')) {
      renderDiskCleanDrives().catch(() => {})
      const body = $('diskclean-body')
      if (body) {
        if (diskCleanTab === 'apps') {
          renderDiskCleanApps(body).catch(() => {})
        } else if (diskCleanTab === 'leftovers') {
          renderDiskCleanLeftovers(body).catch(() => {})
        } else if (diskCleanTab === 'cache') {
          renderDiskCleanCache(body).catch(() => {})
        } else if (diskCleanTab === 'system') {
          renderDiskCleanSystem(body).catch(() => {})
        }
      }
    }
  })
  $('btn-disk-health')?.addEventListener('click', () => runDiskHealth(selectedDrive, false))
  $('modal-smart-refresh')?.addEventListener('click', () => refreshAllDiskSmart(true))
  $('btn-disk-admin')?.addEventListener('click', () => { enableAdvancedModeFlow().catch(() => {}) })
  $('btn-advanced-enable')?.addEventListener('click', () => { enableAdvancedModeFlow().catch(() => {}) })
  $('btn-advanced-restart')?.addEventListener('click', () => { restartHelperFlow().catch(() => {}) })
  $('btn-advanced-diag')?.addEventListener('click', () => { copyAdvancedDiagnostics().catch(() => {}) })
  $('btn-adv-restart')?.addEventListener('click', () => { restartHelperFlow().catch(() => {}) })
  $('btn-adv-diag')?.addEventListener('click', () => { copyAdvancedDiagnostics().catch(() => {}) })
  $('opt-advanced-mode')?.addEventListener('change', (ev) => {
    setAdvancedMode(!!ev.target.checked).catch(() => {})
  })
  refreshAdvancedModeUI().then((st) => {
    if (advancedPhase(st) === 'starting' || (st?.enabled && advancedPhase(st) !== 'ok')) {
      startAdvancedFastPoll()
    }
  }).catch(() => {})
  $('ov-disks')?.addEventListener('click', async (ev) => {
    const btn = ev.target.closest('.disk-expand-btn')
    if (!btn || btn.disabled) return
    const letter = btn.getAttribute('data-drive') || ''
    const details = document.querySelector(`[data-drive-details="${letter}"]`)
    if (!details) return
    const open = btn.getAttribute('aria-expanded') !== 'true'
    btn.setAttribute('aria-expanded', open ? 'true' : 'false')
    details.classList.toggle('hidden', !open)
    details.hidden = !open
    if (open) uiExpand.disks.add(letter)
    else uiExpand.disks.delete(letter)
    if (!open) return
    btn.disabled = true
    fillDiskDetails(letter)
    const had = cachedDiskReport[letter] && !cachedDiskReport[letter]._loading && cachedDiskReport[letter].ok
    if (!had) {
      cachedDiskReport[letter] = { ...(cachedDiskReport[letter] || {}), _loading: true }
      fillDiskDetails(letter)
      try {
        const r = await CheckDiskHealth(letter)
        if (r) {
          cachedDiskReport[letter] = r
          if (r.ok) saveDiskHistory(letter, r)
        }
      } catch (_) {}
      renderDiskCards(latestDisks)
      const btn2 = document.querySelector(`.disk-expand-btn[data-drive="${letter}"]`)
      const det2 = document.querySelector(`[data-drive-details="${letter}"]`)
      if (btn2 && det2) {
        btn2.setAttribute('aria-expanded', 'true')
        det2.classList.remove('hidden')
        det2.hidden = false
        fillDiskDetails(letter)
      }
    } else {
      // Refresh in background without blocking
      CheckDiskHealth(letter).then((r) => {
        if (r && r.ok) {
          cachedDiskReport[letter] = r
          saveDiskHistory(letter, r)
          if (document.querySelector(`.disk-expand-btn[data-drive="${letter}"][aria-expanded="true"]`)) {
            fillDiskDetails(letter)
            updateHomeDiskSummary({ disks: latestDisks })
          }
        }
      }).catch(() => {})
    }
    btn.disabled = false
  })
  // Load SMART when disks tile first opens
  $('tile-disks')?.querySelector('.home-tile-toggle')?.addEventListener('click', () => {
    setTimeout(() => {
      if ($('tile-disks')?.classList.contains('open')) {
        refreshAllDiskSmart(false).catch(() => {})
      }
    }, 0)
  })
  $('btn-weekly-refresh')?.addEventListener('click', refreshWeekly)
  $('btn-periph-refresh')?.addEventListener('click', refreshPeripherals)
  $('btn-net-diag')?.addEventListener('click', runNetDiag)
  $('btn-net-toggle')?.addEventListener('click', () => {
    if (uiExpand.tiles.has('net-lags')) uiExpand.tiles.delete('net-lags')
    else uiExpand.tiles.add('net-lags')
    try { localStorage.setItem('zp_expand_tiles', JSON.stringify([...uiExpand.tiles])) } catch (_) {}
    syncNetAccordionChrome()
  })
  try {
    const savedTiles = JSON.parse(localStorage.getItem('zp_expand_tiles') || '[]')
    if (Array.isArray(savedTiles)) savedTiles.forEach((k) => uiExpand.tiles.add(k))
  } catch (_) {}
  syncNetAccordionChrome()
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
  searchInput?.addEventListener('focus', () => { gameSearchFocused = true })
  searchInput?.addEventListener('blur', () => { gameSearchFocused = false })
  searchInput?.addEventListener('input', () => {
    clearTimeout(searchTimer)
    gameSearchFocused = true
    const q = String(searchInput.value || '').trim()
    if (!q) {
      setSearchSpinner(false)
      searchGamesLive({ online: false })
      return
    }
    // Debounce ~150ms: local + online cascade (null-safe, case/Cyrillic-insensitive).
    searchTimer = setTimeout(() => searchGamesLive({ online: true }), 150)
  })
  searchInput?.addEventListener('keydown', async (ev) => {
    if (ev.key !== 'Enter') return
    clearTimeout(searchTimer)
    await searchGamesLive({ online: true })
  })
  document.addEventListener('keydown', (ev) => {
    if (ev.key !== 'Escape') return
    if (!selectedGameId) return
    const box = $('fps-result')
    if (!box || box.classList.contains('hidden') || !box.classList.contains('open')) return
    ev.preventDefault()
    selectedGameId = null
    document.querySelectorAll('.game-card.selected, .game-card.active').forEach((card) => {
      card.classList.remove('selected', 'active')
    })
    hideFpsDetail()
  })

  const carousel = $('games-carousel-wrapper') || document.querySelector('.games-carousel-wrapper')
  if (carousel) {
    let wheelLock = false
    carousel.addEventListener('wheel', (e) => {
      e.preventDefault()
      if (wheelLock) return
      wheelLock = true
      const step = 206
      // Instant scroll avoids smooth-scroll + paint jank stacking during rapid wheel.
      carousel.scrollBy({
        top: e.deltaY > 0 ? step : -step,
        behavior: 'auto',
      })
      requestAnimationFrame(() => { wheelLock = false })
    }, { passive: false })
  }

  $('game-grid')?.addEventListener('click', (ev) => {
    const card = ev.target.closest('[data-game]')
    if (!card) return
    // Fire-and-forget: selection UI is sync; prediction is raced via predictToken.
    selectGame(card.dataset.game).catch(() => {})
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
        // No kernel/PPL error toasts - button still reflects intended outcome for Defender.
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
    const tl = ev.target.closest('[data-timeline]')
    if (tl) {
      openIncidentTimeline(tl.dataset.timeline)
      return
    }
    const revertBtn = ev.target.closest('[data-eco-revert]')
    if (revertBtn) {
      const sid = revertBtn.dataset.ecoRevert
      revertBtn.disabled = true
      let res
      try {
        res = await RevertEcoSession(sid)
      } catch (err) {
        res = { ok: false, messageRu: String(err), messageEn: String(err) }
      }
      showToast(ecoMsg(res), !!res.ok)
      const card = revertBtn.closest('.incident-card')
      if (card) renderEcoResultBox(card, res)
      return
    }
    const btn = ev.target.closest('[data-eco-name]')
    if (!btn) return
    const name = btn.dataset.ecoName
    const pid = btn.dataset.pid
    const card = btn.closest('.incident-card')
    await applyEcoFeedback({ name, pid, cardEl: card, btn })
  })

  $('btn-timeline-close')?.addEventListener('click', () => {
    const panel = $('incident-timeline')
    if (!panel) return
    panel.classList.add('hidden')
    panel.setAttribute('aria-hidden', 'true')
  })

  $('vamp-exclusions')?.addEventListener('click', (ev) => {
    const btn = ev.target.closest('[data-vamp-restore]')
    if (!btn) return
    ev.preventDefault()
    removeVampWhitelist(btn.getAttribute('data-vamp-restore') || '')
    renderLag(lastMetrics || { vampires: [] })
    showToast(t('vampRestored'), true, { holdMs: 2500 })
  })

  $('vampire-list')?.addEventListener('click', async (ev) => {
    const ignoreBtn = ev.target.closest('[data-vamp-ignore]')
    if (ignoreBtn) {
      ev.preventDefault()
      addVampWhitelist(ignoreBtn.getAttribute('data-vamp-ignore') || '')
      renderLag(lastMetrics || { vampires: [] })
      showToast(t('vampIgnored'), true, { holdMs: 2500 })
      return
    }
    const btn = ev.target.closest('button[data-vamp-eco], button[data-pid]')
    if (!btn) return
    const name = btn.dataset.vampEco
    const pid = Number(btn.dataset.pid)
    const row = btn.closest('.row')
    try {
      if (btn.dataset.act === 'freeze') {
        btn.disabled = true
        btn.textContent = t('ecoApplying')
        const res = name
          ? await ApplyEcoWithFeedback(name)
          : await ApplyEcoPIDWithFeedback(pid)
        btn.disabled = false
        showToast(ecoMsg(res), !!res.ok, { sessionId: res?.canRevert ? res.sessionId : '', holdMs: res?.canRevert ? 15000 : 5000 })
        if (res && res.ok) {
          if (pid) frozen.add(pid)
          btn.textContent = t('ecoLimited')
          btn.classList.remove('danger')
          btn.classList.add('eco-on')
          btn.dataset.act = 'resume'
          if (res.sessionId) btn.dataset.ecoSession = res.sessionId
        } else {
          btn.textContent = name ? t('ecoAll') : t('freeze')
        }
        if (row) renderEcoResultBox(row, res)
        renderEcoLimitedPanel().catch(() => {})
      } else {
        const sid = btn.dataset.ecoSession
        if (sid) {
          const res = await RevertEcoSession(sid)
          showToast(ecoMsg(res), !!res.ok)
          if (row) renderEcoResultBox(row, res)
        } else if (name) {
          await EcoModeByProcessName(name, false)
        } else {
          await ResumeProcess(pid)
        }
        if (pid) frozen.delete(pid)
        btn.textContent = name ? t('ecoAll') : t('freeze')
        btn.classList.add('danger')
        btn.classList.remove('eco-on')
        btn.dataset.act = 'freeze'
      }
      refresh()
    } catch (err) {
      btn.disabled = false
      showToast(String(err), false)
    }
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
    // Near left edge / sidebar - nudge right.
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

async function pollMetrics() {
  if (isPolling) return
  isPolling = true
  try {
    const data = await GetSystemMetrics()
    if (data && data.pollIntervalMS) currentInterval = Math.max(500, Number(data.pollIntervalMS) || 1000)
    applyMetrics(data)
  } catch (e) {
    console.error(e)
  } finally {
    isPolling = false
    clearTimeout(pollTimer)
    pollTimer = setTimeout(pollMetrics, currentInterval || 1000)
  }
}

function boot() {
  applyTheme(localStorage.getItem(THEME_KEY) || 'dark')
  applyI18n()
  wireUI()
  loadBehavior()
  SetLanguage(lang).catch(() => {})
  syncElevatedUI().catch(() => {})
  setActiveTab('home')
  loadGames()
  pollMetrics()
  setTimeout(() => { refreshAllDiskSmart(false).catch(() => {}) }, 1500)
  requestAnimationFrame(animateChart)

  try {
    EventsOn('thermal:alarm', () => showToast(t('thermal')))
    EventsOn('ui:minihud', (enabled) => setHudMode(!!enabled))
    EventsOn('ui:gameoverlay', (enabled) => setGameOverlayMode(!!enabled, true))
    EventsOn('peripherals:update', (items) => renderPeripherals(items || []))
  } catch (_) {}
}

boot()
