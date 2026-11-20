/**
 * Browser mock for Wails Go bindings + runtime.
 * Loaded only by demo.html / screenshot harness — production Wails path unchanged.
 */

const listeners = new Map()

function emit(event, ...args) {
  const set = listeners.get(event)
  if (!set) return
  for (const cb of [...set]) {
    try { cb(...args) } catch (err) { console.error(err) }
  }
}

function on(event, cb) {
  if (!listeners.has(event)) listeners.set(event, new Set())
  listeners.get(event).add(cb)
  return () => listeners.get(event)?.delete(cb)
}

let state = {
  language: 'ru',
  powerMode: 'balanced',
  pollIntervalMS: 1000,
  miniHUD: false,
  gameOverlay: false,
  fullscreen: false,
  tick: 0,
}

const GAMES = [
  { id: 'cs2', name: 'Counter-Strike 2', genre: 'FPS', category: 'modern', year: 2023, engine: 'Source 2', posterUrl: '', headerUrl: '', steamAppId: 730 },
  { id: 'gta5', name: 'Grand Theft Auto V', genre: 'Action', category: 'modern', year: 2015, engine: 'RAGE', posterUrl: '', headerUrl: '', steamAppId: 271590 },
  { id: 'witcher3', name: 'The Witcher 3', genre: 'RPG', category: 'classic', year: 2015, engine: 'REDengine', posterUrl: '', headerUrl: '', steamAppId: 292030 },
  { id: 'valorant', name: 'VALORANT', genre: 'FPS', category: 'modern', year: 2020, engine: 'Unreal', posterUrl: '', headerUrl: '', steamAppId: 0 },
  { id: 'dota2', name: 'Dota 2', genre: 'MOBA', category: 'classic', year: 2013, engine: 'Source 2', posterUrl: '', headerUrl: '', steamAppId: 570 },
]

function wave(base, amp, phase = 0) {
  return Math.max(0, Math.min(100, base + Math.sin(state.tick / 6 + phase) * amp))
}

function buildMetrics() {
  const cpu = wave(38, 12, 0.2)
  const gpu = wave(52, 18, 1.1)
  const ramPct = wave(61, 4, 2.0)
  const ramTotal = 32768
  const ramUsed = Math.round(ramTotal * (ramPct / 100))
  const temp = 58 + Math.sin(state.tick / 10) * 4

  return {
    cpuPercent: Number(cpu.toFixed(1)),
    gpuPercent: Number(gpu.toFixed(1)),
    ramPercent: Number(ramPct.toFixed(1)),
    ramUsedMB: ramUsed,
    ramTotalMB: ramTotal,
    swapPercent: 2.1,
    swapUsedMB: 256,
    swapTotalMB: 8192,
    vramTotalMB: 8192,
    vramSharedMB: 0,
    temperatureC: Number(temp.toFixed(1)),
    powerMode: state.powerMode,
    pollIntervalMS: state.pollIntervalMS,
    miniHUD: state.miniHUD,
    language: state.language,
    cpuName: 'AMD Ryzen 7 5800X',
    gpuName: 'NVIDIA GeForce RTX 3070',
    cpuPerCore: Array.from({ length: 16 }, (_, i) => Number(wave(20 + (i % 5) * 8, 15, i * 0.4).toFixed(0))),
    disks: [
      { mount: 'C:\\', usedGB: 412.4, totalGB: 952.0, percent: 43, readMBps: 18.2, writeMBps: 4.1, freeGB: 539.6 },
      { mount: 'D:\\', usedGB: 820.0, totalGB: 1863.0, percent: 44, readMBps: 2.0, writeMBps: 0.4, freeGB: 1043.0 },
    ],
    battery: {
      available: true,
      percent: 76,
      pluggedIn: true,
      status: 'charging',
      statusText: 'Зарядка',
      powerDrawWatts: 28.4,
      wearPercent: 8.2,
      wearLabel: 'Износ 8%',
      capacitymWh: 55200,
      designCapacitymWh: 60100,
      deviceName: 'Primary',
    },
    hardware: {
      cpu: {
        name: 'AMD Ryzen 7 5800X',
        physicalCores: 8,
        logicalCores: 16,
        frequencyMHz: 4200,
        percent: Number(cpu.toFixed(1)),
        perCore: Array.from({ length: 16 }, (_, i) => Number(wave(20 + (i % 5) * 8, 15, i * 0.4).toFixed(0))),
      },
      gpu: {
        name: 'NVIDIA GeForce RTX 3070',
        driverVersion: '566.36',
        dedicatedMB: 8192,
        sharedMB: 0,
      },
      ram: {
        totalMB: ramTotal,
        usedMB: ramUsed,
        freeMB: ramTotal - ramUsed,
        type: 'DDR4',
        speedMHz: 3200,
        slotsUsed: 2,
        slotsTotal: 4,
        isSingleChannel: false,
        channelStatus: 'Dual Channel',
      },
      power: {
        hasBattery: true,
        deviceName: 'Primary',
        statusText: 'Зарядка',
        percent: 76,
        wearPercent: 8.2,
        wearLabel: 'Износ 8%',
        capacitymWh: 55200,
        designCapacitymWh: 60100,
      },
    },
    lagEvents: [
      {
        timestamp: '14:02:11',
        coreID: 3,
        coreLoad: 99,
        overallLoad: 61,
        culpritProcess: 'MsMpEng.exe',
        culpritFriendly: 'Защитник Windows',
        culpritPID: 4120,
        explanation: 'lag_core_spike|3|99',
        icon: '',
      },
      {
        timestamp: '13:58:44',
        coreID: 7,
        coreLoad: 97,
        overallLoad: 48,
        culpritProcess: 'chrome.exe',
        culpritFriendly: 'Chrome',
        culpritPID: 8821,
        explanation: 'lag_core_spike|7|97',
        icon: '',
      },
    ],
    stutterEvents: [
      { timestamp: '14:01:02', jitterMs: 340, cause: 'Disk IO spike · SearchIndexer.exe', culprit: 'SearchIndexer.exe' },
      { timestamp: '13:55:18', jitterMs: 420, cause: 'GPU queue stall', culprit: '' },
    ],
    vampires: [
      { pid: 4120, pids: [4120], count: 1, name: 'MsMpEng.exe', friendlyName: 'Защитник Windows', cpuPercent: 6.4, diskMBps: 1.2, score: 10, ecoMode: false, frozen: false },
      { pid: 2204, pids: [2204, 2208, 2212], count: 3, name: 'chrome.exe', friendlyName: 'Chrome', cpuPercent: 9.1, diskMBps: 0.4, score: 14, ecoMode: false, frozen: false },
      { pid: 1180, pids: [1180], count: 1, name: 'OneDrive.exe', friendlyName: 'OneDrive', cpuPercent: 3.2, diskMBps: 2.8, score: 7, ecoMode: true, frozen: true },
    ],
  }
}

const App = {
  GetMetrics: async () => buildMetrics(),
  SetLanguage: async (lang) => { state.language = lang || 'ru'; return true },
  SetPowerProfile: async (mode) => { state.powerMode = mode || 'balanced'; return true },
  SetPollInterval: async (ms) => { state.pollIntervalMS = Number(ms) || 1000; return true },
  ToggleMiniHUD: async (enabled) => {
    state.miniHUD = !!enabled
    if (enabled) state.gameOverlay = false
    emit('ui:minihud', state.miniHUD)
    return state.miniHUD
  },
  ToggleGameOverlay: async (enabled) => {
    state.gameOverlay = !!enabled
    if (enabled) state.miniHUD = false
    emit('ui:gameoverlay', state.gameOverlay)
    return state.gameOverlay
  },
  ToggleFullscreen: async () => { state.fullscreen = !state.fullscreen; return state.fullscreen },
  HideToTray: async () => true,
  FreezeProcess: async () => true,
  ResumeProcess: async () => true,
  EcoModeByProcessName: async () => true,
  CheckDiskHealth: async (drive) => ({
    ok: true,
    model: 'Samsung SSD 980 PRO 1TB',
    serialNumber: 'S5GXNX0MOCK01',
    interfaceType: 'NVMe',
    mediaType: 'SSD',
    temperatureC: 41,
    tbwGB: 12_400,
    powerOnHours: 4820,
    wearRemaining: 94,
    hasCriticalErrors: false,
    verdict: 'Хорошее состояние',
    verdictBadge: 'OK',
    drive: drive || 'C',
  }),
  GetMonthlyAppReport: async () => ({
    ok: true,
    period: 'последние 7 дней',
    note: 'mock',
    topApps: [
      { name: 'MsMpEng.exe', friendlyName: 'procDefender', detail: 'procDefenderDetail', activeHours: 14.2, trafficMB: 12, energymWh: 4200 },
      { name: 'SearchIndexer.exe', friendlyName: 'procIndexer', detail: 'procIndexerDetail', activeHours: 6.1, trafficMB: 0, energymWh: 1800 },
      { name: 'chrome.exe', friendlyName: 'procBrowser', detail: 'procBrowserDetail', activeHours: 22.0, trafficMB: 8400, energymWh: 9100 },
    ],
  }),
  DiagnoseLagOrigin: async () => ({
    gatewayHost: '192.168.1.1',
    gatewayMs: 4,
    gatewayBadge: 'netBadgeIdeal',
    gatewayBadgeOK: true,
    publicHost: '8.8.8.8',
    publicMs: 18,
    publicBadge: 'netBadgeFast',
    publicBadgeOK: true,
    localJitterMs: 2,
    jitterBadge: 'netBadgeStable',
    jitterBadgeOK: true,
    verdict: 'netVerdictOK',
    verdictKey: 'netVerdictOK',
    level: 'ok',
    ok: true,
  }),
  CheckSleepBlockers: async () => ({
    ok: true,
    sleepOK: true,
    summary: 'sleep_ok',
    blockers: [],
    rawHint: '',
  }),
  GetConnectedPeripherals: async () => ([
    { group: 'input', name: 'Logitech G Pro X Superlight', kind: 'Mouse', class: 'HID', icon: '🖱' },
    { group: 'input', name: 'Keychron K2', kind: 'Keyboard', class: 'HID', icon: '⌨' },
    { group: 'audio', name: 'HyperX Cloud II', kind: 'Headset', class: 'Audio', icon: '🎧' },
    { group: 'display', name: 'LG UltraGear 27GL850', kind: 'Monitor', class: 'Display', icon: '🖥' },
  ]),
  ListGames: async () => GAMES,
  SearchGames: async (q) => {
    const s = String(q || '').toLowerCase()
    if (!s) return GAMES
    return GAMES.filter((g) => g.name.toLowerCase().includes(s))
  },
  SearchGamesOnline: async (q) => App.SearchGames(q),
  PredictGameFPS: async (id) => {
    const g = GAMES.find((x) => x.id === id) || GAMES[0]
    return {
      gameName: g.name,
      genre: g.genre,
      category: g.category,
      year: g.year,
      engine: g.engine,
      badge: g.genre,
      posterUrl: '',
      headerUrl: '',
      recommend: 'high',
      verdict: 'fpsVerdictPlayable',
      verdictLevel: 'ok',
      warnings: [],
      isIntegrated: false,
      dualChannel: true,
      vramGB: 8,
      gpuScore: 72,
      cpuScore: 78,
      lowAvg: 148,
      medAvg: 112,
      highAvg: 86,
      ultraAvg: 64,
      lowRange: '140-160',
      medRange: '100-120',
      highRange: '78-94',
      ultraRange: '58-70',
    }
  },
}

window.go = { main: { App } }

window.runtime = {
  EventsOnMultiple(eventName, callback) {
    return on(eventName, callback)
  },
  EventsOn(eventName, callback) {
    return on(eventName, callback)
  },
  EventsOff() {},
  EventsOffAll() { listeners.clear() },
  EventsEmit(eventName, ...args) { emit(eventName, ...args) },
  WindowMinimise() {},
  WindowMaximise() {},
  WindowUnmaximise() {},
  WindowIsMaximised() { return false },
  WindowFullscreen() { state.fullscreen = true },
  WindowUnfullscreen() { state.fullscreen = false },
  WindowIsFullscreen() { return state.fullscreen },
  WindowHide() {},
  WindowShow() {},
  Quit() {},
  LogPrint() {},
  LogTrace() {},
  LogDebug() {},
  LogInfo() {},
  LogWarning() {},
  LogError() {},
  LogFatal() {},
}

// Live-ish metric stream for sparklines / status strip
setInterval(() => {
  state.tick += 1
  if (!state.miniHUD && !state.gameOverlay) {
    emit('metrics:update', buildMetrics())
  } else {
    emit('metrics:update', buildMetrics())
  }
}, 1000)

console.info('[PulseCore mock] Wails bindings ready')
