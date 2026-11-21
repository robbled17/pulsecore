/**
 * Browser mock for Wails Go bindings + runtime.
 * Loaded only by demo.html / screenshot harness; production Wails path unchanged.
 * Supports hardware matrix profiles A/B/C for GamePulse + sensor fallback demos.
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

/** Demo-only hardware matrix (never used by live Wails). */
const PROFILES = {
  A: {
    id: 'A',
    label: 'Weak',
    cpuName: 'Intel Core i3-10100',
    gpuName: 'Intel UHD Graphics 630',
    physicalCores: 4,
    logicalCores: 8,
    frequencyMHz: 3600,
    ramTotalMB: 8192,
    dedicatedMB: 0,
    sharedMB: 2048,
    isIntegrated: true,
    isSingleChannel: true,
    slotsUsed: 1,
    slotsTotal: 2,
    ramType: 'DDR4',
    ramSpeed: 2666,
    channelStatus: 'Single Channel',
    diskPct: 95,
    diskUsed: 456,
    diskTotal: 480,
    cpuBase: 72,
    gpuBase: 55,
    ramBase: 91,
    tempMode: 'hot', // ~88-94°C
    cpuScore: 34,
    gpuScore: 11,
    vramGB: 0,
    ramGB: 8,
  },
  B: {
    id: 'B',
    label: 'Mid',
    cpuName: 'AMD Ryzen 7 5700X',
    gpuName: 'NVIDIA GeForce RTX 3060',
    physicalCores: 8,
    logicalCores: 16,
    frequencyMHz: 3900,
    ramTotalMB: 16384,
    dedicatedMB: 8192,
    sharedMB: 0,
    isIntegrated: false,
    isSingleChannel: false,
    slotsUsed: 2,
    slotsTotal: 4,
    ramType: 'DDR4',
    ramSpeed: 3200,
    channelStatus: 'Dual Channel',
    diskPct: 58,
    diskUsed: 552,
    diskTotal: 952,
    cpuBase: 42,
    gpuBase: 58,
    ramBase: 62,
    tempMode: 'normal',
    cpuScore: 68,
    gpuScore: 61,
    vramGB: 8,
    ramGB: 16,
  },
  C: {
    id: 'C',
    label: 'Top+',
    cpuName: 'AMD Ryzen 9 7950X',
    gpuName: 'NVIDIA GeForce RTX 4090',
    physicalCores: 16,
    logicalCores: 32,
    frequencyMHz: 4500,
    ramTotalMB: 32768,
    dedicatedMB: 24576,
    sharedMB: 0,
    isIntegrated: false,
    isSingleChannel: false,
    slotsUsed: 2,
    slotsTotal: 4,
    ramType: 'DDR5',
    ramSpeed: 6000,
    channelStatus: 'Dual Channel',
    diskPct: 34,
    diskUsed: 680,
    diskTotal: 2000,
    cpuBase: 28,
    gpuBase: 45,
    ramBase: 48,
    tempMode: 'broken', // null / 0 / NaN sensors
    cpuScore: 95,
    gpuScore: 100,
    vramGB: 24,
    ramGB: 32,
  },
}

let state = {
  language: 'ru',
  powerMode: 'balanced',
  pollIntervalMS: 1000,
  miniHUD: false,
  gameOverlay: false,
  fullscreen: false,
  tick: 0,
  profile: 'B',
}

function steamArt(id) {
  return {
    posterUrl: `https://cdn.cloudflare.steamstatic.com/steam/apps/${id}/library_600x900.jpg`,
    headerUrl: `https://cdn.cloudflare.steamstatic.com/steam/apps/${id}/header.jpg`,
    steamAppId: id,
  }
}

const GAMES = [
  { id: 'cs2', name: 'Counter-Strike 2', genre: 'FPS', category: 'modern', year: 2023, engine: 'Source 2', weight: 0.7, ...steamArt(730) },
  { id: 'gta5', name: 'Grand Theft Auto V', genre: 'Action', category: 'modern', year: 2015, engine: 'RAGE', weight: 0.85, ...steamArt(271590) },
  { id: 'witcher3', name: 'The Witcher 3', genre: 'RPG', category: 'classic', year: 2015, engine: 'REDengine', weight: 0.8, ...steamArt(292030) },
  { id: 'elden', name: 'ELDEN RING', genre: 'Action RPG', category: 'modern', year: 2022, engine: 'Proprietary', weight: 1.05, ...steamArt(1245620) },
  { id: 'dota2', name: 'Dota 2', genre: 'MOBA', category: 'classic', year: 2013, engine: 'Source 2', weight: 0.55, ...steamArt(570) },
  { id: 'cyberpunk', name: 'Cyberpunk 2077', genre: 'RPG', category: 'modern', year: 2020, engine: 'REDengine', weight: 1.25, ...steamArt(1091500) },
]

/** Simplified GamePulse req tables (mirrors gamelab.go spirit). */
const GAME_REQ = {
  cs2: { low: [90, 28, 30, 2], med: [85, 38, 38, 3], high: [80, 48, 45, 4], ultra: [75, 58, 52, 6], heavy: false, cap: 0 },
  gta5: { low: [70, 35, 32, 2], med: [65, 48, 42, 4], high: [60, 58, 50, 6], ultra: [55, 68, 58, 8], heavy: false, cap: 0 },
  witcher3: { low: [60, 32, 30, 2], med: [58, 45, 40, 4], high: [55, 55, 48, 6], ultra: [52, 65, 55, 8], heavy: false, cap: 0 },
  elden: { low: [55, 48, 40, 4], med: [52, 60, 50, 6], high: [50, 70, 58, 8], ultra: [48, 80, 68, 10], heavy: true, cap: 0 },
  dota2: { low: [100, 18, 22, 1], med: [95, 26, 28, 2], high: [90, 34, 34, 3], ultra: [85, 42, 40, 4], heavy: false, cap: 0 },
  cyberpunk: { low: [55, 55, 38, 3], med: [55, 70, 48, 6], high: [60, 68, 58, 8], ultra: [58, 78, 70, 10], heavy: true, cap: 0 },
}

function wave(base, amp, phase = 0) {
  return Math.max(0, Math.min(100, base + Math.sin(state.tick / 6 + phase) * amp))
}

function profile() {
  return PROFILES[state.profile] || PROFILES.B
}

function setMockProfile(id) {
  if (!PROFILES[id]) return state.profile
  state.profile = id
  emit('metrics:update', buildMetrics())
  emit('mock:profile', id)
  return id
}

function buildSystemPowerProfile() {
  const p = profile()
  return {
    gpuName: p.gpuName,
    isIntegrated: p.isIntegrated,
    vramGB: p.vramGB,
    gpuPowerScore: p.gpuScore,
    cpuName: p.cpuName,
    cpuCores: p.physicalCores,
    cpuThreads: p.logicalCores,
    cpuFreqMHz: p.frequencyMHz,
    cpuPowerScore: p.cpuScore,
    ramGB: p.ramGB,
    isDualChannel: !p.isSingleChannel,
  }
}

function clampFPS(v) {
  return Math.max(3, Math.min(400, Math.round(v)))
}

function estimatePreset(sys, base, reqGpu, reqCpu, reqVram, preset, cap) {
  let gpuRatio = sys.gpuPowerScore / Math.max(0.1, reqGpu)
  gpuRatio = Math.max(0.05, Math.min(2.5, gpuRatio))
  let cpuRatio = sys.cpuPowerScore / Math.max(0.1, reqCpu)
  cpuRatio = Math.max(0.15, Math.min(2.0, cpuRatio))
  const cpuBlend = 0.70 + 0.30 * Math.min(cpuRatio, 1.25)

  let ramMul = 1
  if (sys.isIntegrated && !sys.isDualChannel) ramMul = 0.68
  else if (!sys.isDualChannel) ramMul = 0.88
  else if (sys.isIntegrated) ramMul = 0.92

  let vramMul = 1
  let warn = ''
  if (sys.isIntegrated) {
    if ((preset === 'high' || preset === 'ultra') && sys.ramGB < reqVram * 2) {
      vramMul = 0.7
      warn = 'fpsWarnVRAM'
    }
  } else if (reqVram > 0 && sys.vramGB + 0.05 < reqVram) {
    vramMul = 0.7
    warn = 'fpsWarnVRAM'
  }

  let fps = base * Math.min(gpuRatio, 2.5) * cpuBlend * ramMul * vramMul
  if (sys.cpuCores > 0 && sys.cpuCores < 4 && (preset === 'high' || preset === 'ultra')) {
    fps *= 0.62
    if (!warn) warn = 'fpsWarnFewCores'
  }
  if (sys.ramGB > 0 && sys.ramGB < 8 && (preset === 'high' || preset === 'ultra')) fps *= 0.85
  if (cap > 0 && fps > cap) fps = cap

  const avg = clampFPS(fps)
  const spread = Math.max(2, avg * 0.12)
  const minV = clampFPS(avg - spread)
  const maxV = clampFPS(avg + spread)
  return { avg, range: maxV <= 5 ? '<5' : `${minV}-${maxV}`, warn }
}

function dynamicVerdict(sys, req, low, med, high) {
  if (!req.heavy && low >= 90) return ['fpsVerdictSmoothMax', 'ok']
  if (req.cap > 0 && low >= 55 && med >= 55) return ['fpsVerdictLocked60', 'ok']
  if (req.heavy && sys.gpuPowerScore <= 28 && high < 30) return ['fpsVerdictWeakHW', 'bad']
  if (req.heavy && (med < 20 || (low < 22 && sys.isIntegrated))) return ['fpsVerdictNeedGPU', 'bad']
  if (high >= 55 || med >= 45) return ['fpsVerdictPlayable', 'ok']
  if (low >= 40) return ['fpsVerdictOKLow', 'warn']
  if (low >= 25) return ['fpsVerdictBorderline', 'warn']
  if (sys.isIntegrated) return ['fpsVerdictHeavy', 'warn']
  return ['fpsVerdictBorderline', 'warn']
}

function predictFPS(gameID) {
  const g = GAMES.find((x) => x.id === gameID) || GAMES[0]
  const sys = buildSystemPowerProfile()
  const req = GAME_REQ[g.id] || GAME_REQ.gta5
  const presets = ['low', 'med', 'high', 'ultra']
  const results = {}
  const warnings = []
  for (const preset of presets) {
    const [base, rg, rc, rv] = req[preset]
    const r = estimatePreset(sys, base, rg, rc, rv, preset, req.cap)
    results[preset] = r
    if (r.warn && !warnings.includes(r.warn)) warnings.push(r.warn)
  }
  if (sys.isIntegrated && !sys.isDualChannel) warnings.push('fpsWarnIntegratedSingle')
  else if (!sys.isDualChannel) warnings.push('fpsWarnSingleChannel')
  if (sys.cpuCores > 0 && sys.cpuCores < 4) warnings.push('fpsWarnFewCores')

  const { low, med, high, ultra } = results
  let recommend = 'low'
  if (ultra.avg >= 55) recommend = 'ultra'
  else if (high.avg >= 55) recommend = 'high'
  else if (med.avg >= 55) recommend = 'med'

  const [verdict, verdictLevel] = dynamicVerdict(sys, req, low.avg, med.avg, high.avg)

  return {
    gameId: g.id,
    gameName: g.name,
    genre: g.genre,
    category: g.category,
    year: g.year,
    engine: g.engine,
    badge: g.genre,
    posterUrl: g.posterUrl || '',
    headerUrl: g.headerUrl || '',
    steamAppId: g.steamAppId || 0,
    recommend,
    verdict,
    verdictLevel,
    warnings: [...new Set(warnings)],
    isIntegrated: sys.isIntegrated,
    dualChannel: sys.isDualChannel,
    vramGB: sys.vramGB,
    ramGB: sys.ramGB,
    cpuCores: sys.cpuCores,
    gpuScore: sys.gpuPowerScore,
    cpuScore: sys.cpuPowerScore,
    lowAvg: low.avg,
    medAvg: med.avg,
    highAvg: high.avg,
    ultraAvg: ultra.avg,
    lowRange: low.range,
    medRange: med.range,
    highRange: high.range,
    ultraRange: ultra.range,
  }
}

function sensorTemp(p) {
  if (p.tempMode === 'broken') {
    // Cycle broken readings so UI must never show NaN/-1
    const cycle = state.tick % 3
    if (cycle === 0) return null
    if (cycle === 1) return 0
    return Number.NaN
  }
  if (p.tempMode === 'hot') return Number((88 + Math.sin(state.tick / 8) * 4).toFixed(1))
  return Number((58 + Math.sin(state.tick / 10) * 4).toFixed(1))
}

function buildMetrics() {
  const p = profile()
  const cpu = wave(p.cpuBase, 10, 0.2)
  const gpu = wave(p.gpuBase, 14, 1.1)
  const ramPct = wave(p.ramBase, 3, 2.0)
  const ramTotal = p.ramTotalMB
  const ramUsed = Math.round(ramTotal * (ramPct / 100))
  const temp = sensorTemp(p)
  const freeGB = Math.max(0, p.diskTotal - p.diskUsed)

  const brokenFan = p.tempMode === 'broken' ? null : 1200 + (state.tick % 40) * 5

  return {
    cpuPercent: Number(cpu.toFixed(1)),
    gpuPercent: Number(gpu.toFixed(1)),
    ramPercent: Number(ramPct.toFixed(1)),
    ramUsedMB: ramUsed,
    ramTotalMB: ramTotal,
    swapPercent: p.id === 'A' ? 28 : 2.1,
    swapUsedMB: p.id === 'A' ? 1800 : 256,
    swapTotalMB: p.id === 'A' ? 4096 : 8192,
    vramTotalMB: p.dedicatedMB,
    vramSharedMB: p.sharedMB,
    temperatureC: temp,
    tempAvailable: p.tempMode !== 'broken',
    fanRPM: brokenFan,
    powerMode: state.powerMode,
    pollIntervalMS: state.pollIntervalMS,
    miniHUD: state.miniHUD,
    language: state.language,
    osName: 'Windows 11',
    cpuName: p.cpuName,
    gpuName: p.gpuName,
    mockProfile: p.id,
    cpuPerCore: Array.from({ length: p.logicalCores }, (_, i) => {
      if (p.tempMode === 'broken' && i === 3) return Number.NaN
      return Number(wave(20 + (i % 5) * 8, 15, i * 0.4).toFixed(0))
    }),
    disks: [
      {
        mount: 'C:\\',
        usedGB: p.diskUsed,
        totalGB: p.diskTotal,
        percent: p.diskPct,
        readMBps: p.id === 'A' ? 42 : 18.2,
        writeMBps: p.id === 'A' ? 12 : 4.1,
        freeGB,
      },
    ],
    battery: {
      available: p.id !== 'C',
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
        name: p.cpuName,
        physicalCores: p.physicalCores,
        logicalCores: p.logicalCores,
        frequencyMHz: p.frequencyMHz,
        baseFrequencyMHz: p.frequencyMHz * 0.85,
        currentFrequencyMHz: p.frequencyMHz * (0.9 + Math.sin(state.tick / 20) * 0.08),
        cacheKB: p.logicalCores >= 16 ? 64 * 1024 : 32 * 1024,
        architecture: 'x64',
        percent: Number(cpu.toFixed(1)),
        perCore: Array.from({ length: p.logicalCores }, (_, i) => {
          if (p.tempMode === 'broken' && i === 3) return null
          return Number(wave(20 + (i % 5) * 8, 15, i * 0.4).toFixed(0))
        }),
      },
      gpu: {
        name: p.gpuName,
        driverVersion: p.isIntegrated ? '31.0.101.5186' : '566.36',
        dedicatedMB: p.dedicatedMB,
        sharedMB: p.sharedMB,
        isIntegrated: p.isIntegrated,
        vramGB: p.vramGB,
        fanRPM: brokenFan,
      },
      ram: {
        totalMB: ramTotal,
        usedMB: ramUsed,
        freeMB: ramTotal - ramUsed,
        type: p.ramType,
        speedMHz: p.ramSpeed,
        slotsUsed: p.slotsUsed,
        slotsTotal: p.slotsTotal,
        isSingleChannel: p.isSingleChannel,
        channelStatus: p.channelStatus,
      },
      power: {
        hasBattery: p.id !== 'C',
        deviceName: 'Primary',
        statusText: 'Зарядка',
        percent: 76,
        wearPercent: 8.2,
        wearLabel: 'Износ 8%',
        capacitymWh: 55200,
        designCapacitymWh: 60100,
      },
    },
    lagEvents: p.id === 'A' || p.id === 'B' ? [
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
    ] : [],
    stutterEvents: p.id === 'A' ? [
      { timestamp: '14:01:02', jitterMs: 340, cause: 'Disk IO spike · SearchIndexer.exe', culprit: 'SearchIndexer.exe' },
      { timestamp: '13:55:18', jitterMs: 480, cause: 'Low free RAM', culprit: 'chrome.exe' },
    ] : p.id === 'B' ? [
      { timestamp: '14:01:02', jitterMs: 340, cause: 'Disk IO spike · SearchIndexer.exe', culprit: 'SearchIndexer.exe' },
    ] : [],
    vampires: [
      { pid: 4120, pids: [4120], count: 1, name: 'MsMpEng.exe', friendlyName: 'Защитник Windows', cpuPercent: 6.4, diskMBps: 1.2, score: 10, ecoMode: false, frozen: false },
      { pid: 2204, pids: [2204, 2208, 2212], count: 3, name: 'chrome.exe', friendlyName: 'Chrome', cpuPercent: p.id === 'A' ? 18.4 : 9.1, diskMBps: 0.4, score: 14, ecoMode: false, frozen: false },
      { pid: 1180, pids: [1180], count: 1, name: 'OneDrive.exe', friendlyName: 'OneDrive', cpuPercent: 3.2, diskMBps: 2.8, score: 7, ecoMode: true, frozen: true },
      ...(p.id === 'A' ? [{ pid: 3300, pids: [3300], count: 1, name: 'SearchIndexer.exe', friendlyName: 'Индексатор Windows', cpuPercent: 11.2, diskMBps: 18.0, score: 22, ecoMode: false, frozen: false }] : []),
    ],
  }
}

const App = {
  GetMetrics: async () => buildMetrics(),
  GetSystemPowerProfile: async () => buildSystemPowerProfile(),
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
  ToggleEcoMode: async () => true,
  EcoModeByProcessName: async () => true,
  CheckDiskHealth: async (drive) => ({
    ok: true,
    model: 'Samsung SSD 980 PRO 1TB',
    serialNumber: 'S5GXNX0MOCK01',
    interfaceType: 'NVMe',
    mediaType: 'SSD',
    temperatureC: profile().tempMode === 'broken' ? null : 41,
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
  PredictGameFPS: async (id) => predictFPS(id),
  GetPCPassport: async () => {
    const p = profile()
    return {
      chassis: p.id === 'A' ? 'laptop' : 'desktop',
      chassisRaw: p.id === 'A' ? '9' : '3',
      isLaptop: p.id === 'A',
      motherboard: p.id === 'A' ? 'Laptop Board' : 'B550M Pro',
      manufacturer: p.id === 'A' ? 'OEM' : 'ASUS',
      chipset: p.id === 'A' ? 'Unknown' : 'B550',
      socket: p.id === 'A' ? 'BGA' : (p.cpuName.includes('7950') ? 'AM5' : 'AM4'),
      cpuName: p.cpuName,
      physicalCores: p.physicalCores,
      logicalCores: p.logicalCores,
      ramType: p.ramType,
      ramSpeedMHz: p.ramSpeed,
      ramTotalGB: p.ramGB,
      ramSlotsUsed: p.slotsUsed,
      ramSlotsTotal: p.slotsTotal,
      ramSlotsFree: Math.max(0, p.slotsTotal - p.slotsUsed),
      isSingleChannel: p.isSingleChannel,
      gpuName: p.gpuName,
      vramGB: p.vramGB,
      gpuIntegrated: p.isIntegrated,
      storage: [
        { model: 'Samsung 980 PRO 1TB', interfaceType: 'NVMe', mediaType: 'SSD', sizeGB: 1000 },
      ],
      hasFreeM2: true,
      notes: p.id === 'A'
        ? ['Ноутбук: процессор и видеокарта обычно впаяны. Менять их нельзя без замены всей платы.']
        : [],
    }
  },
  GetUpgradeAdvice: async (goal) => {
    const passport = await App.GetPCPassport()
    const g = String(goal || 'max_fps')
    const laptop = passport.isLaptop
    const socket = passport.socket || 'AM4'
    const ramType = passport.ramType || 'DDR4'
    const pitGPU = [
      'Перед покупкой GPU измерьте место в корпусе и проверьте мощность и разъёмы блока питания.',
      'Нужен запас по ваттам и родные разъёмы (8-pin / 12VHPWR).',
      'После установки поставьте свежий драйвер NVIDIA или AMD.',
    ]
    const pitCPU = [
      'Обновите BIOS перед установкой более нового процессора.',
      'Проверьте VRM на плате.',
      `Сокет сейчас: ${socket}. Новый процессор должен быть для этого сокета.`,
    ]
    const pitRAM = [
      `Тип памяти: ${ramType}. Другое поколение не подойдёт.`,
      'Совпадайте частоту и тайминги модулей.',
      'В BIOS включите XMP/EXPO.',
    ]
    const desktopByGoal = {
      max_fps: [
        {
          id: 'desktop-gpu', title: 'Видеокарта', part: 'gpu', priority: 1,
          model: 'RTX 4070 Super', estimatedGain: '+60% FPS',
          body: `Сейчас ${passport.gpuName}. Переход на RTX 4070 Super даёт примерно +60% FPS в типичных играх на 1080p/1440p.`,
          pitfalls: pitGPU,
        },
        {
          id: 'desktop-cpu', title: 'Процессор под сокет', part: 'cpu', priority: 2,
          model: 'Ryzen 7 5700X3D / 5800X3D', estimatedGain: '+10-20% 1% lows',
          body: `Сокет ${socket} ещё живой: 5700X3D встанет без смены платы. Сейчас у вас ${passport.cpuName}.`,
          pitfalls: pitCPU,
        },
        {
          id: 'desktop-ram', title: 'Оперативная память', part: 'ram', priority: 3,
          model: `2x16 GB ${ramType} dual-channel`, estimatedGain: 'меньше статтеров',
          body: `Свободно слотов: ${passport.ramSlotsFree}. Наберите 32 ГБ ${ramType} одной частоты и таймингов.`,
          pitfalls: pitRAM,
        },
      ],
      quiet_cool: [
        {
          id: 'desktop-cooling', title: 'Тихое охлаждение', part: 'cooling', priority: 1,
          model: 'Башенный кулер 120/140 мм', estimatedGain: 'тише на 5-15°C',
          body: 'Башенный кулер, нормальный поток воздуха и undervolt GPU/CPU часто тише смены железа.',
          pitfalls: ['Смените термопасту и проверьте посадку кулера.', 'Undervolt снижайте маленькими шагами.', 'Воздух должен выходить из корпуса.'],
        },
        {
          id: 'desktop-gpu', title: 'Тихая видеокарта', part: 'gpu', priority: 2,
          model: 'RTX 4070 Super (3-fan)', estimatedGain: 'тише под нагрузкой',
          body: 'RTX 4070 Super с трёхвентиляторным охлаждением и undervolt работает тише коротких дутышей.',
          pitfalls: pitGPU,
        },
        {
          id: 'desktop-cpu', title: 'CPU с умеренным TDP', part: 'cpu', priority: 3,
          model: `Keep ${passport.cpuName} + cooler`, estimatedGain: 'ниже температура',
          body: `На сокете ${socket} оставьте текущий чип и поставьте хороший башенный кулер.`,
          pitfalls: pitCPU,
        },
      ],
      work: [
        {
          id: 'desktop-ram', title: 'Оперативная память', part: 'ram', priority: 1,
          model: `2x16 GB ${ramType}`, estimatedGain: '32-64 GB',
          body: `Для работы целитесь в 32 или 64 ГБ ${ramType}. Все модули одной частоты и таймингов.`,
          pitfalls: pitRAM,
        },
        {
          id: 'desktop-cpu', title: 'Больше ядер на сокете', part: 'cpu', priority: 2,
          model: 'Ryzen 9 5950X / 5900X', estimatedGain: '+multi-core',
          body: `Для работы на сокете ${socket} логичен Ryzen 9: больше ядер для рендера и виртуалок.`,
          pitfalls: pitCPU,
        },
        {
          id: 'desktop-nvme', title: 'NVMe SSD', part: 'storage', priority: 3,
          model: 'NVMe 1-2 TB', estimatedGain: 'быстрее загрузка',
          body: 'Поставьте систему и проекты на NVMe. Свободный M.2: быстрый апгрейд без замены всего ПК.',
          pitfalls: ['Для NVMe нужен свободный M.2 и иногда радиатор.', 'Клонируйте систему или поставьте Windows заново.', 'SATA SSD медленнее NVMe.'],
        },
      ],
      budget: [
        {
          id: 'desktop-ram', title: 'Оперативная память', part: 'ram', priority: 1,
          model: `2x16 GB ${ramType}`, estimatedGain: 'лучший $/эффект',
          body: `Свободно слотов: ${passport.ramSlotsFree}. Dual-channel ${ramType} одной частоты и таймингов.`,
          pitfalls: pitRAM,
        },
        {
          id: 'desktop-gpu', title: 'Бюджетная видеокарта', part: 'gpu', priority: 2,
          model: 'RTX 4060 Ti 16GB (used OK)', estimatedGain: '+30-45% FPS',
          body: `Сейчас ${passport.gpuName}. RTX 4060 Ti (в том числе б/у) даёт заметный прирост без смены платформы.`,
          pitfalls: pitGPU,
        },
        {
          id: 'desktop-nvme', title: 'NVMe SSD', part: 'storage', priority: 3,
          model: 'NVMe 1 TB', estimatedGain: 'быстрее загрузка',
          body: 'Бюджетный NVMe ускоряет Windows и игры без дорогой смены GPU.',
          pitfalls: ['Проверьте свободный M.2.', 'Клонируйте систему при необходимости.', 'Берите диск с DRAM или хорошим SLC-кэшем.'],
        },
      ],
    }
    const laptopRecs = [
      {
        id: 'laptop-ram', title: 'RAM: dual-channel', part: 'ram', priority: 1,
        model: `1x 16 GB ${ramType} (match)`, estimatedGain: '+15-25% FPS (iGPU)',
        body: `Вторая планка ${ramType} той же частоты включает dual-channel. На ноутбуке CPU/GPU обычно впаяны.`,
        pitfalls: pitRAM,
      },
      {
        id: 'laptop-m2', title: 'Второй NVMe', part: 'storage', priority: 2,
        model: 'NVMe 1 TB (2nd M.2)', estimatedGain: 'быстрее загрузка',
        body: 'Второй M.2 или замена SSD ускоряет загрузку игр и проектов. Проверьте сервисное руководство.',
        pitfalls: ['Проверьте второй слот M.2 в руководстве.', 'Иногда нужен радиатор.', 'Клонируйте систему при замене загрузочного диска.'],
      },
      {
        id: 'laptop-cooling', title: 'Охлаждение и undervolt', part: 'cooling', priority: 3,
        model: 'Паста + чистка + undervolt', estimatedGain: 'стабильнее FPS',
        body: 'Чистка, новая паста и undervolt. Реальный апгрейд ноутбука без замены платы. Впаянный CPU/GPU менять нельзя.',
        pitfalls: ['Не закрывайте вентиляцию.', 'Undervolt маленькими шагами.', 'Не покупайте мобильный CPU/GPU отдельно.'],
      },
    ]
    const recommendations = laptop
      ? laptopRecs
      : (desktopByGoal[g] || desktopByGoal.max_fps)
    return {
      goal: g,
      chassis: passport.chassis,
      isLaptop: laptop,
      passport,
      recommendations,
      summary: laptop
        ? 'Ноутбук: только RAM, второй NVMe и охлаждение. CPU/GPU обычно впаяны.'
        : `ПК: конкретные модели под сокет ${socket} и текущую видеокарту.`,
    }
  },
  ListInstalledApps: async () => ([
    { id: 'mock1', displayName: 'Demo Editor', publisher: 'Acme Soft', installLocation: 'C:\\Program Files\\DemoEditor', uninstallString: 'uninstall.exe', displayVersion: '3.2.1', estimatedMB: 420, isMsi: false },
    { id: 'mock2', displayName: 'Chatty', publisher: 'Chatty Inc', installLocation: 'C:\\Users\\Demo\\AppData\\Local\\Chatty', uninstallString: 'msiexec /x {GUID}', displayVersion: '1.8.0', estimatedMB: 180, isMsi: true },
    { id: 'mock3', displayName: 'Old Launcher', publisher: 'Games Co', installLocation: 'C:\\Games\\OldLauncher', uninstallString: 'unins000.exe', displayVersion: '0.9.4', estimatedMB: 95, isMsi: false },
  ]),
  PrecheckUninstall: async (id) => {
    const apps = await App.ListInstalledApps()
    const app = apps.find((a) => a.id === id) || apps[0]
    const running = id === 'mock2' ? [{ pid: 4412, name: 'Chatty.exe', imagePath: app.installLocation + '\\Chatty.exe' }] : []
    return {
      ok: true,
      app,
      running,
      canKill: running.length > 0,
      warning: running.length ? `Найдено запущенных процессов: ${running.length}. Завершите их перед удалением или нажмите «Завершить и удалить».` : '',
    }
  },
  KillAppProcesses: async () => ({ ok: true, killed: 1, failed: 0, message: 'Завершено: 1, ошибок: 0' }),
  UninstallApp: async () => ({ ok: true, message: 'Запущен официальный деинсталлятор', appName: 'Demo' }),
  ScanResiduals: async (name) => ({
    ok: true,
    appName: name || 'Demo',
    message: 'Найдено остатков: 3',
    items: [
      { id: 'fs:1', kind: 'folder', path: '%LOCALAPPDATA%\\DemoEditor', label: 'Local · DemoEditor', safe: true, selected: false },
      { id: 'fs:2', kind: 'folder', path: '%APPDATA%\\DemoEditor', label: 'Roaming · DemoEditor', safe: true, selected: false },
      { id: 'reg:1', kind: 'registry', path: 'HKCU\\Software\\DemoEditor', label: 'HKCU\\Software\\DemoEditor', safe: true, selected: false },
      { id: 'reg:2', kind: 'registry', path: 'HKLM\\Software\\Microsoft', label: 'HKLM\\Software\\Microsoft', safe: false, selected: false },
    ],
  }),
  DeleteResiduals: async () => ({ ok: true, removed: 3, failed: 0, skipped: 1, message: 'Удалено: 3, ошибок: 0, пропущено: 1' }),
  StartTracedInstall: async () => ({ ok: true, appId: 'trace1', appName: 'setup.exe', message: 'Установщик запущен. После установки нажмите «Завершить трассировку».' }),
  FinishTracedInstall: async () => ({ ok: true, appId: 'trace1', path: 'traces/trace1.json', newKeys: 2, newDirs: 1, message: 'Трассировка сохранена: trace1.json' }),
  ListInstallTraces: async () => ([
    { appId: 'trace1', appName: 'setup.exe', createdAt: '2026-03-21T10:00:00Z', finishedAt: '2026-03-21T10:05:00Z', newKeys: 2, newDirs: 1, file: 'trace1.json' },
  ]),
  CleanupFromTrace: async () => ({ ok: true, removed: [], failed: [], message: 'Удалено: 1, пропущено/ошибок: 0 (реестр только вручную)' }),
  CleanSafeDiskCaches: async () => ({
    ok: true,
    deletedFiles: 12,
    deletedBytes: 48 * 1024 * 1024,
    skipped: 3,
    errors: 0,
    targets: ['%TEMP%', 'D3DSCache', 'Chrome/Default/Cache'],
    message: 'Удалено файлов: 12 (48.0 МБ), пропущено: 3, ошибок: 0',
  }),
  /** Demo-only helper: not present on live Wails App. */
  SetMockProfile: async (id) => setMockProfile(id),
  GetMockProfile: async () => state.profile,
}

window.go = { main: { App } }
window.__PULSECORE_MOCK__ = { setProfile: setMockProfile, getProfile: () => state.profile, profiles: PROFILES }

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

setInterval(() => {
  state.tick += 1
  emit('metrics:update', buildMetrics())
}, 1000)

console.info('[PulseCore mock] Wails bindings ready · profiles A/B/C')
