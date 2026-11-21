/** Frontend-local Dev Mode (About version 7-click). No Go simulator required. */

const STORAGE_KEY = 'zp_dev'

const DEFAULTS = {
  active: false,
  showPerCore: false,
  tempSource: 'auto',
  throttleC: 85,
  peakSens: 95,
  ecoAllow: '',
  ecoDeny: 'csrss.exe, lsass.exe, smss.exe, wininit.exe, services.exe, svchost.exe, dwm.exe, pulsecore.exe',
  forceAdapter: '',
  gpuTempWarn: 80,
  vramWarn: 90,
  ramWarn: 85,
  diskWarn: 90,
  smartEnabled: true,
  showRemovable: false,
  pingGw: '',
  pingPublic: '8.8.8.8',
  pollMs: 1000,
  mockProfile: '',
}

let state = { ...DEFAULTS }
let versionClickCount = 0
let versionClickTimer = null
let hooks = {
  showToast: () => {},
  setActiveTab: () => {},
  t: (k) => k,
  applyPoll: async () => {},
  onChange: () => {},
}

const $ = (id) => document.getElementById(id)

export function getDevState() {
  return { ...state }
}

export function isDevModeActive() {
  return !!state.active
}

export function loadDevState() {
  try {
    const raw = JSON.parse(localStorage.getItem(STORAGE_KEY) || '{}')
    state = { ...DEFAULTS, ...raw, active: !!raw.active }
  } catch (_) {
    state = { ...DEFAULTS }
  }
  return getDevState()
}

function persist() {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(state))
  hooks.onChange(getDevState())
}

function syncForm() {
  const map = {
    'dev-temp-source': 'tempSource',
    'dev-throttle-c': 'throttleC',
    'dev-peak-sens': 'peakSens',
    'dev-eco-allow': 'ecoAllow',
    'dev-eco-deny': 'ecoDeny',
    'dev-force-adapter': 'forceAdapter',
    'dev-gpu-temp': 'gpuTempWarn',
    'dev-vram-warn': 'vramWarn',
    'dev-ram-warn': 'ramWarn',
    'dev-disk-warn': 'diskWarn',
    'dev-ping-gw': 'pingGw',
    'dev-ping-public': 'pingPublic',
    'dev-poll-ms': 'pollMs',
  }
  Object.entries(map).forEach(([id, key]) => {
    const el = $(id)
    if (el) el.value = state[key]
  })
  const per = $('dev-show-percore')
  if (per) per.checked = !!state.showPerCore
  const smart = $('dev-smart')
  if (smart) smart.checked = !!state.smartEnabled
  const rem = $('dev-removable')
  if (rem) rem.checked = !!state.showRemovable
  document.querySelectorAll('#dev-mock-row .mock-profile-btn').forEach((b) => {
    b.classList.toggle('active', b.dataset.profile === state.mockProfile)
  })
}

function readForm() {
  state.tempSource = $('dev-temp-source')?.value || 'auto'
  state.throttleC = Number($('dev-throttle-c')?.value) || 85
  state.peakSens = Number($('dev-peak-sens')?.value) || 95
  state.ecoAllow = $('dev-eco-allow')?.value || ''
  state.ecoDeny = $('dev-eco-deny')?.value || DEFAULTS.ecoDeny
  state.forceAdapter = $('dev-force-adapter')?.value || ''
  state.gpuTempWarn = Number($('dev-gpu-temp')?.value) || 80
  state.vramWarn = Number($('dev-vram-warn')?.value) || 90
  state.ramWarn = Number($('dev-ram-warn')?.value) || 85
  state.diskWarn = Number($('dev-disk-warn')?.value) || 90
  state.pingGw = $('dev-ping-gw')?.value || ''
  state.pingPublic = $('dev-ping-public')?.value || '8.8.8.8'
  state.pollMs = Number($('dev-poll-ms')?.value) || 1000
  state.showPerCore = !!$('dev-show-percore')?.checked
  state.smartEnabled = !!$('dev-smart')?.checked
  state.showRemovable = !!$('dev-removable')?.checked
  persist()
}

export function enableDevModeUI(userTrigger = false) {
  state.active = true
  persist()
  $('dev-mode-badge')?.classList.remove('hidden')
  document.querySelectorAll('.dev-mode-nav').forEach((n) => n.classList.remove('hidden'))
  syncForm()
  if (userTrigger) {
    hooks.setActiveTab('dev')
    hooks.showToast(hooks.t('devModeActivatedToast'), true)
  }
}

export function disableDevModeUI() {
  state.active = false
  persist()
  $('dev-mode-badge')?.classList.add('hidden')
  document.querySelectorAll('.dev-mode-nav').forEach((n) => n.classList.add('hidden'))
  if ($('tab-dev')?.classList.contains('active')) hooks.setActiveTab('overview')
  hooks.showToast(hooks.t('devModeDisabledToast'), false)
}

function wireVersionClick() {
  const triggers = [$('about-app-ver'), $('about-app-name')].filter(Boolean)
  triggers.forEach((el) => {
    el.style.cursor = 'pointer'
    el.addEventListener('click', () => {
      if (state.active) return
      clearTimeout(versionClickTimer)
      versionClickCount += 1
      if (versionClickCount >= 4 && versionClickCount < 7) {
        hooks.showToast(hooks.t('devModeClicksRemaining').replace('{count}', String(7 - versionClickCount)), true)
      } else if (versionClickCount >= 7) {
        versionClickCount = 0
        enableDevModeUI(true)
      }
      versionClickTimer = setTimeout(() => { versionClickCount = 0 }, 2500)
    })
  })
}

export function initDevMode(opts = {}) {
  hooks = { ...hooks, ...opts }
  loadDevState()
  wireVersionClick()

  $('dev-mode-badge')?.addEventListener('click', () => hooks.setActiveTab('dev'))
  $('btn-dev-disable')?.addEventListener('click', () => disableDevModeUI())
  $('btn-dev-reset')?.addEventListener('click', () => {
    const was = state.active
    state = { ...DEFAULTS, active: was }
    syncForm()
    persist()
    hooks.showToast(hooks.t('devResetDone'), true)
  })

  const ids = [
    'dev-temp-source', 'dev-throttle-c', 'dev-peak-sens', 'dev-eco-allow', 'dev-eco-deny',
    'dev-force-adapter', 'dev-gpu-temp', 'dev-vram-warn', 'dev-ram-warn', 'dev-disk-warn',
    'dev-ping-gw', 'dev-ping-public', 'dev-poll-ms', 'dev-show-percore', 'dev-smart', 'dev-removable',
  ]
  ids.forEach((id) => {
    const el = $(id)
    if (!el) return
    el.addEventListener('change', async () => {
      readForm()
      if (id === 'dev-poll-ms') {
        try { await hooks.applyPoll(state.pollMs) } catch (_) {}
      }
    })
  })

  document.querySelectorAll('#dev-mock-row .mock-profile-btn').forEach((btn) => {
    btn.addEventListener('click', () => {
      if (!window.__PULSECORE_MOCK__) {
        hooks.showToast(hooks.t('devMockUnavailable'), false)
        return
      }
      state.mockProfile = btn.dataset.profile
      window.__PULSECORE_MOCK__.setProfile(btn.dataset.profile)
      syncForm()
      persist()
    })
  })

  if (state.active) {
    $('dev-mode-badge')?.classList.remove('hidden')
    document.querySelectorAll('.dev-mode-nav').forEach((n) => n.classList.remove('hidden'))
    syncForm()
  } else {
    $('dev-mode-badge')?.classList.add('hidden')
    document.querySelectorAll('.dev-mode-nav').forEach((n) => n.classList.add('hidden'))
  }
}
