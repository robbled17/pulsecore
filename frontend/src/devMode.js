import {
  IsDevMode,
  SetDevMode,
  GetDevModeState,
  GetDevHardwareCatalog,
  SetDryRun,
  IsDryRun,
  ApplyDevPreset,
  SetSimulatedSensors,
  SetSimulatedHardware,
  SetSimulatedDisks,
  SetSimulatedBattery,
  ResetDevMode,
  ResetSimulationsToReal,
  GetRawTelemetry,
  UnlockFileDryRun,
  ApplyDiskCleanup,
  ApplyEcoWithFeedback,
} from '../wailsjs/go/main/App.js'

import {
  t,
  setI18nDev,
  isI18nDev,
  catalogKeys,
  getLang,
  _catalogs,
} from './i18n.js'

let devState = null
let devCatalog = { cpus: [], gpus: [] }
let devDisks = []
let versionClickCount = 0
let versionClickTimer = null

let showToastFn = null
let setActiveTabFn = null
let refreshFn = null
let renderHomeFn = null
let renderHealthTabFn = null
let renderGamePulseFn = null
let escapeHtmlFn = (s) => String(s ?? '').replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;')

async function triggerLiveDevUpdates() {
  try {
    if (refreshFn) await refreshFn()
    if (renderHomeFn) await renderHomeFn()
    if (renderHealthTabFn) await renderHealthTabFn()
    if (renderGamePulseFn) await renderGamePulseFn()
  } catch (err) {
    console.error('triggerLiveDevUpdates error:', err)
  }
}

const $ = (id) => document.getElementById(id)

export function isDevModeActive() {
  return !!(devState && devState.devMode)
}

export async function initDevMode({ showToast, setActiveTab, refresh, escapeHtml, renderHome, renderHealthTab, renderGamePulse }) {
  if (showToast) showToastFn = showToast
  if (setActiveTab) setActiveTabFn = setActiveTab
  if (refresh) refreshFn = refresh
  if (escapeHtml) escapeHtmlFn = escapeHtml
  if (renderHome) renderHomeFn = renderHome
  if (renderHealthTab) renderHealthTabFn = renderHealthTab
  if (renderGamePulse) renderGamePulseFn = renderGamePulse

  wireVersionClickTrigger()
  wireHeaderAndNav()
  wirePresets()
  wireSensorsSimulator()
  wireHardwareConstructor()
  wireTelemetryInspector()
  wireI18nDebug()
  wireDryRunTests()

  try {
    const isDev = await IsDevMode()
    if (isDev) {
      await enableDevModeUI(false)
    } else {
      const badge = $('dev-mode-badge')
      if (badge) badge.classList.add('hidden')
      document.querySelectorAll('.dev-mode-nav').forEach((nav) => {
        nav.classList.add('hidden')
      })
    }
  } catch (err) {
    console.error('Failed to check Dev Mode state:', err)
  }
}

function wireVersionClickTrigger() {
  const trigger = $('about-app-name')
  if (!trigger) return

  trigger.addEventListener('click', async () => {
    clearTimeout(versionClickTimer)
    versionClickCount++

    if (versionClickCount >= 4 && versionClickCount < 7) {
      const remaining = 7 - versionClickCount
      if (showToastFn) {
        showToastFn(t('devModeClicksRemaining', { count: remaining }))
      }
    } else if (versionClickCount >= 7) {
      versionClickCount = 0
      await enableDevModeUI(true)
    }

    versionClickTimer = setTimeout(() => {
      versionClickCount = 0
    }, 2500)
  })
}

async function enableDevModeUI(isUserTrigger = false) {
  try {
    devState = await SetDevMode(true)
  } catch (err) {
    console.error('SetDevMode failed:', err)
  }

  const badge = $('dev-mode-badge')
  if (badge) badge.classList.remove('hidden')

  document.querySelectorAll('.dev-mode-nav').forEach((nav) => {
    nav.classList.remove('hidden')
  })

  await loadCatalogAndState()

  if (isUserTrigger && setActiveTabFn) {
    setActiveTabFn('dev')
  }

  if (isUserTrigger && showToastFn) {
    showToastFn(t('devModeActivatedToast'), true)
  }
}

export async function disableDevModeUI() {
  try {
    devState = await SetDevMode(false)
  } catch (err) {
    console.error('SetDevMode(false) failed:', err)
  }

  const badge = $('dev-mode-badge')
  if (badge) badge.classList.add('hidden')

  document.querySelectorAll('.dev-mode-nav').forEach((nav) => {
    nav.classList.add('hidden')
  })

  if (document.getElementById('tab-dev')?.classList.contains('active')) {
    if (setActiveTabFn) setActiveTabFn('home')
  }

  if (showToastFn) {
    showToastFn(t('devModeDisabledToast'), false)
  }

  await triggerLiveDevUpdates()
}

function wireHeaderAndNav() {
  const badge = $('dev-mode-badge')
  if (badge) {
    badge.addEventListener('click', () => {
      if (setActiveTabFn) setActiveTabFn('dev')
    })
  }

  const resetRealBtn = $('btn-dev-reset-real')
  if (resetRealBtn) {
    resetRealBtn.addEventListener('click', async () => {
      try {
        devState = await ResetSimulationsToReal()
        renderDevSensors()
        renderDevHardware()
        renderDevDisks()
        if (showToastFn) {
          showToastFn(t('devResetRealToast'), true)
        }
        await triggerLiveDevUpdates()
      } catch (err) {
        console.error('ResetSimulationsToReal failed:', err)
      }
    })
  }

  const disableBtn = $('btn-dev-disable')
  if (disableBtn) {
    disableBtn.addEventListener('click', () => disableDevModeUI())
  }

  const dryRunToggle = $('toggle-dev-dryrun')
  if (dryRunToggle) {
    dryRunToggle.addEventListener('change', async (e) => {
      const on = !!e.target.checked
      try {
        await SetDryRun(on)
        if (showToastFn) {
          showToastFn(on ? t('devDryRunToastOn') : t('devDryRunToastOff'), on)
        }
      } catch (err) {
        console.error('SetDryRun failed:', err)
      }
    })
  }
}

async function loadCatalogAndState() {
  try {
    devCatalog = await GetDevHardwareCatalog()
    populateHardwareSelectors()
  } catch (err) {
    console.error('Failed to get Dev Hardware Catalog:', err)
  }

  try {
    devState = await GetDevModeState()
    syncUIWithState(devState)
  } catch (err) {
    console.error('Failed to get Dev Mode State:', err)
  }
}

export async function renderDevTab() {
  await loadCatalogAndState()
  await refreshTelemetryTable()
}

function syncUIWithState(state) {
  if (!state) return

  const dryRunToggle = $('toggle-dev-dryrun')
  if (dryRunToggle) dryRunToggle.checked = !!state.dryRun

  // 1. Sensors
  if (state.sensors) {
    const s = state.sensors
    const cpuPct = Math.round(s.cpuPercent ?? s.cpuPct ?? 15)
    const cpuTemp = Math.round(s.cpuTempC ?? s.cpuTemp ?? 45)
    setRangeAndLabel('dev-range-cpu-load', 'dev-cpu-load-val', cpuPct, `${cpuPct}%`)
    setRangeAndLabel('dev-range-cpu-temp', 'dev-cpu-temp-val', cpuTemp, `${cpuTemp}°C`)

    const ramTotal = Math.round(s.ramTotalGB || 16)
    const ramSelect = $('dev-select-ram-total')
    if (ramSelect) ramSelect.value = String(ramTotal)
    const ramUsedPct = Math.round(s.ramUsedPct ?? 25)
    const ramUsedGB = (s.ramUsedGB != null ? s.ramUsedGB : (ramTotal * ramUsedPct) / 100).toFixed(1)
    setRangeAndLabel('dev-range-ram-used', 'dev-ram-used-val', ramUsedPct, `${ramUsedGB} GB (${ramUsedPct}%)`)

    const gpuPct = Math.round(s.gpuPercent ?? s.gpuPct ?? 10)
    setRangeAndLabel('dev-range-gpu-load', 'dev-gpu-load-val', gpuPct, `${gpuPct}%`)

    const vramUsedPct = Math.round(s.vramUsedPct ?? 20)
    const vramTotal = state.hardware?.gpuVRAMGB || 8
    const vramUsedGB = (s.vramUsedGB != null ? s.vramUsedGB : (vramTotal * vramUsedPct) / 100).toFixed(1)
    setRangeAndLabel('dev-range-vram-used', 'dev-vram-load-val', vramUsedPct, `${vramUsedGB} GB (${vramUsedPct}%)`)

    const gpuTemp = Math.round(s.gpuTempC ?? s.gpuTemp ?? 42)
    setRangeAndLabel('dev-range-gpu-temp', 'dev-gpu-temp-val', gpuTemp, `${gpuTemp}°C`)

    const hours = Math.round(s.uptimeHours ?? 5)
    const days = (hours / 24).toFixed(1)
    setRangeAndLabel('dev-range-uptime', 'dev-uptime-val', hours, `${hours} ${t('devHoursSuffix')} (${days} ${t('devDaysSuffix')})`)
  }

  // 2. Hardware
  if (state.hardware) {
    const hw = state.hardware
    const cpuNameInput = $('dev-hw-cpu-name')
    if (cpuNameInput) cpuNameInput.value = hw.cpuName || ''
    const cpuCoresInput = $('dev-hw-cpu-cores')
    if (cpuCoresInput) cpuCoresInput.value = hw.cpuCores || 6
    const cpuThreadsInput = $('dev-hw-cpu-threads')
    if (cpuThreadsInput) cpuThreadsInput.value = hw.cpuThreads || 12
    const cpuFreqInput = $('dev-hw-cpu-freq')
    if (cpuFreqInput) cpuFreqInput.value = Math.round(hw.cpuFreqMHz || 3600)

    const cpuSelect = $('dev-hw-cpu-select')
    if (cpuSelect) {
      const match = Array.from(cpuSelect.options).find((opt) => opt.value === hw.cpuName)
      cpuSelect.value = match ? match.value : 'custom'
    }

    const gpuNameInput = $('dev-hw-gpu-name')
    if (gpuNameInput) gpuNameInput.value = hw.gpuName || ''
    const gpuVramInput = $('dev-hw-gpu-vram')
    if (gpuVramInput) gpuVramInput.value = Math.round(hw.gpuVRAMGB || 8)
    const gpuIgpuCheck = $('dev-hw-gpu-igpu')
    if (gpuIgpuCheck) gpuIgpuCheck.checked = !!(hw.gpuIsIGPU ?? hw.isIGPU)

    const gpuSelect = $('dev-hw-gpu-select')
    if (gpuSelect) {
      const match = Array.from(gpuSelect.options).find((opt) => opt.value === hw.gpuName)
      gpuSelect.value = match ? match.value : 'custom'
    }

    // Battery inside hw
    const isLap = !!hw.isLaptop
    const desktopRadio = $('dev-device-desktop')
    const laptopRadio = $('dev-device-laptop')
    if (desktopRadio && laptopRadio) {
      desktopRadio.checked = !isLap
      laptopRadio.checked = isLap
    }
    const laptopControls = $('dev-laptop-controls')
    if (laptopControls) {
      laptopControls.classList.toggle('hidden', !isLap)
    }

    const batPct = Math.round(hw.batteryPercent ?? 80)
    const batWear = Math.round(hw.batteryWearPct ?? 10)
    setRangeAndLabel('dev-range-bat-pct', 'dev-bat-pct-val', batPct, `${batPct}%`)
    setRangeAndLabel('dev-range-bat-wear', 'dev-bat-wear-val', batWear, `${batWear}%`)
    const powerSelect = $('dev-select-bat-power')
    if (powerSelect) powerSelect.value = hw.batteryPlugged ? 'ac' : 'dc'
  }

  // 3. Disks
  if (state.disks && Array.isArray(state.disks)) {
    devDisks = state.disks.map((d) => ({
      letter: (d.letter || 'C').toUpperCase().replace(/[^A-Z]/g, ''),
      type: d.type || 'NVMe SSD',
      model: d.model || 'Solid State Drive',
      totalGB: Math.round(d.totalGB || 1000),
      usedPct: Math.round(d.usedPct || 45),
      healthPct: Math.round(d.healthPct || 100),
      tempC: Math.round(d.tempC || 35),
    }))
    renderDisksList()
  }
}

function setRangeAndLabel(rangeId, labelId, val, labelText) {
  const range = $(rangeId)
  if (range) range.value = val
  const label = $(labelId)
  if (label) label.textContent = labelText
}

function wirePresets() {
  document.querySelectorAll('.dev-preset-card[data-preset]').forEach((btn) => {
    btn.addEventListener('click', async () => {
      const preset = btn.dataset.preset
      try {
        devState = await ApplyDevPreset(preset)
        syncUIWithState(devState)
        if (showToastFn) {
          showToastFn(t('devPresetAppliedToast', { preset }), true)
        }
        await triggerLiveDevUpdates()
      } catch (err) {
        console.error('Failed to apply preset:', err)
      }
    })
  })
}

function wireSensorsSimulator() {
  const updateSensorLabels = () => {
    const cpuPct = Number($('dev-range-cpu-load')?.value || 0)
    const cpuTemp = Number($('dev-range-cpu-temp')?.value || 0)
    const ramTotal = Number($('dev-select-ram-total')?.value || 16)
    const ramUsedPct = Number($('dev-range-ram-used')?.value || 0)
    const gpuPct = Number($('dev-range-gpu-load')?.value || 0)
    const vramUsedPct = Number($('dev-range-vram-used')?.value || 0)
    const gpuTemp = Number($('dev-range-gpu-temp')?.value || 0)
    const uptime = Number($('dev-range-uptime')?.value || 1)

    const ramUsedGB = ((ramTotal * ramUsedPct) / 100).toFixed(1)
    const vramTotal = Number($('dev-hw-gpu-vram')?.value || 8)
    const vramUsedGB = ((vramTotal * vramUsedPct) / 100).toFixed(1)
    const days = (uptime / 24).toFixed(1)

    if ($('dev-cpu-load-val')) $('dev-cpu-load-val').textContent = `${cpuPct}%`
    if ($('dev-cpu-temp-val')) $('dev-cpu-temp-val').textContent = `${cpuTemp}°C`
    if ($('dev-ram-used-val')) $('dev-ram-used-val').textContent = `${ramUsedGB} GB (${ramUsedPct}%)`
    if ($('dev-gpu-load-val')) $('dev-gpu-load-val').textContent = `${gpuPct}%`
    if ($('dev-vram-load-val')) $('dev-vram-load-val').textContent = `${vramUsedGB} GB (${vramUsedPct}%)`
    if ($('dev-gpu-temp-val')) $('dev-gpu-temp-val').textContent = `${gpuTemp}°C`
    if ($('dev-uptime-val')) $('dev-uptime-val').textContent = `${uptime} ${t('devHoursSuffix')} (${days} ${t('devDaysSuffix')})`
  }

  const rangeIds = [
    'dev-range-cpu-load',
    'dev-range-cpu-temp',
    'dev-range-ram-used',
    'dev-range-gpu-load',
    'dev-range-vram-used',
    'dev-range-gpu-temp',
    'dev-range-uptime',
  ]

  let sensorDebounceTimer = null
  const applySensors = async (notify = true) => {
    const cpuPct = Number($('dev-range-cpu-load')?.value || 0)
    const cpuTemp = Number($('dev-range-cpu-temp')?.value || 0)
    const ramTotal = Number($('dev-select-ram-total')?.value || 16)
    const ramUsedPct = Number($('dev-range-ram-used')?.value || 0)
    const gpuPct = Number($('dev-range-gpu-load')?.value || 0)
    const vramUsedPct = Number($('dev-range-vram-used')?.value || 0)
    const gpuTemp = Number($('dev-range-gpu-temp')?.value || 0)
    const uptime = Number($('dev-range-uptime')?.value || 1)

    try {
      devState = await SetSimulatedSensors(cpuPct, cpuTemp, ramTotal, ramUsedPct, gpuPct, vramUsedPct, gpuTemp, uptime)
      if (notify && showToastFn) {
        showToastFn(t('devSensorsAppliedToast'), true)
      }
      await triggerLiveDevUpdates()
    } catch (err) {
      console.error('Failed to set sensors:', err)
    }
  }

  const triggerDebouncedSensors = () => {
    updateSensorLabels()
    clearTimeout(sensorDebounceTimer)
    sensorDebounceTimer = setTimeout(() => {
      applySensors(false)
    }, 120)
  }

  rangeIds.forEach((id) => {
    $(id)?.addEventListener('input', triggerDebouncedSensors)
    $(id)?.addEventListener('change', () => {
      clearTimeout(sensorDebounceTimer)
      applySensors(false)
    })
  })

  $('dev-select-ram-total')?.addEventListener('change', () => {
    updateSensorLabels()
    applySensors(false)
  })

  $('btn-dev-apply-sensors')?.addEventListener('click', () => applySensors(true))
}

function populateHardwareSelectors() {
  const cpuSelect = $('dev-hw-cpu-select')
  if (cpuSelect && devCatalog.cpus?.length) {
    const curVal = cpuSelect.value
    cpuSelect.innerHTML = `<option value="custom">${t('devCustomInput')}</option>` +
      devCatalog.cpus.map((c) => `<option value="${escapeHtmlFn(c.name)}">${escapeHtmlFn(c.name)}</option>`).join('')
    if (curVal) cpuSelect.value = curVal
  }

  const gpuSelect = $('dev-hw-gpu-select')
  if (gpuSelect && devCatalog.gpus?.length) {
    const curVal = gpuSelect.value
    gpuSelect.innerHTML = `<option value="custom">${t('devCustomInput')}</option>` +
      devCatalog.gpus.map((g) => `<option value="${escapeHtmlFn(g.name)}">${escapeHtmlFn(g.name)}</option>`).join('')
    if (curVal) gpuSelect.value = curVal
  }
}

function wireHardwareConstructor() {
  $('dev-hw-cpu-select')?.addEventListener('change', (e) => {
    const val = e.target.value
    if (val === 'custom') return
    const match = devCatalog.cpus?.find((c) => c.name === val)
    if (match) {
      if ($('dev-hw-cpu-name')) $('dev-hw-cpu-name').value = match.name
      if ($('dev-hw-cpu-cores')) $('dev-hw-cpu-cores').value = match.cores
      if ($('dev-hw-cpu-threads')) $('dev-hw-cpu-threads').value = match.threads
      if ($('dev-hw-cpu-freq')) $('dev-hw-cpu-freq').value = match.freqMHz
    }
  })

  $('dev-hw-gpu-select')?.addEventListener('change', (e) => {
    const val = e.target.value
    if (val === 'custom') return
    const match = devCatalog.gpus?.find((g) => g.name === val)
    if (match) {
      if ($('dev-hw-gpu-name')) $('dev-hw-gpu-name').value = match.name
      if ($('dev-hw-gpu-vram')) $('dev-hw-gpu-vram').value = match.vramGB
      if ($('dev-hw-gpu-igpu')) $('dev-hw-gpu-igpu').checked = !!(match.isIntegrated ?? match.isIGPU)
    }
  })

  const handleDeviceTypeChange = () => {
    const isLaptop = !!$('dev-device-laptop')?.checked
    $('dev-laptop-controls')?.classList.toggle('hidden', !isLaptop)
  }
  $('dev-device-desktop')?.addEventListener('change', handleDeviceTypeChange)
  $('dev-device-laptop')?.addEventListener('change', handleDeviceTypeChange)

  $('dev-range-bat-pct')?.addEventListener('input', (e) => {
    if ($('dev-bat-pct-val')) $('dev-bat-pct-val').textContent = `${e.target.value}%`
  })
  $('dev-range-bat-wear')?.addEventListener('input', (e) => {
    if ($('dev-bat-wear-val')) $('dev-bat-wear-val').textContent = `${e.target.value}%`
  })

  $('btn-dev-add-disk')?.addEventListener('click', () => {
    syncDisksFromDOM()
    const usedLetters = new Set(devDisks.map((d) => (d.letter || '').toUpperCase()))
    const candidateLetters = ['C', 'D', 'E', 'F', 'G', 'H', 'X', 'Z']
    const nextLetter = candidateLetters.find((l) => !usedLetters.has(l)) || 'D'

    const newDisk = {
      letter: nextLetter,
      type: 'SATA SSD',
      model: 'Kingston A400 480GB',
      totalGB: 480,
      usedPct: 35,
      healthPct: 99,
      tempC: 34,
    }
    devDisks.push(newDisk)
    renderDisksList()
    if (showToastFn) showToastFn(t('devDiskAddedToast'), true)
  })

  $('btn-dev-apply-hw')?.addEventListener('click', async () => {
    await applyHardwareConfiguration()
  })
}

async function applyHardwareConfiguration() {
  const cpuName = $('dev-hw-cpu-name')?.value || 'Intel Core i5-12400F'
  const cpuCores = Number($('dev-hw-cpu-cores')?.value || 6)
  const cpuThreads = Number($('dev-hw-cpu-threads')?.value || 12)
  const cpuFreq = Number($('dev-hw-cpu-freq')?.value || 4400)
  const cpuMatch = devCatalog.cpus?.find((c) => c.name === cpuName)
  const cpuScore = cpuMatch?.score || Math.round(cpuCores * (cpuFreq / 1000) * 16)

  const gpuName = $('dev-hw-gpu-name')?.value || 'NVIDIA RTX 4060 (8 GB)'
  const gpuVRAM = Number($('dev-hw-gpu-vram')?.value || 8)
  const gpuIgpu = !!$('dev-hw-gpu-igpu')?.checked
  const gpuMatch = devCatalog.gpus?.find((g) => g.name === gpuName)
  const gpuScore = gpuMatch?.score || (gpuIgpu ? 30 : Math.round(gpuVRAM * 15))

  const ramTotalGB = Number($('dev-select-ram-total')?.value || 16)

  const isLaptop = !!$('dev-device-laptop')?.checked
  const batPct = Number($('dev-range-bat-pct')?.value || 80)
  const batWear = Number($('dev-range-bat-wear')?.value || 10)
  const batPlugged = $('dev-select-bat-power')?.value === 'ac'

  syncDisksFromDOM()

  try {
    await SetSimulatedHardware(cpuName, cpuCores, cpuThreads, cpuFreq, cpuScore, gpuName, gpuVRAM, gpuIgpu, gpuScore, ramTotalGB)
    await SetSimulatedBattery(isLaptop, batPct, batPlugged, batWear)
    devState = await SetSimulatedDisks(devDisks)
    if (showToastFn) {
      showToastFn(t('devHardwareAppliedToast'), true)
    }
    await triggerLiveDevUpdates()
  } catch (err) {
    console.error('Failed to apply hardware:', err)
  }
}

function syncDisksFromDOM() {
  const container = $('dev-disks-container')
  if (!container) return

  const diskCards = container.querySelectorAll('.dev-disk-item')
  const nextDisks = []

  diskCards.forEach((card, idx) => {
    const rawLetter = card.querySelector('.dev-disk-letter')?.value || (idx === 0 ? 'C' : 'D')
    const letter = rawLetter.replace(/[^A-Z]/g, '')
    const type = card.querySelector('.dev-disk-type')?.value || 'NVMe SSD'
    const model = card.querySelector('.dev-disk-model')?.value || (type === 'HDD' ? 'WDC WD10EZEX' : 'Solid State Drive')
    const totalGB = Number(card.querySelector('.dev-disk-size')?.value || 1000)
    const usedPct = Number(card.querySelector('.dev-disk-used')?.value || 50)
    const health = Number(card.querySelector('.dev-disk-health')?.value || 100)
    const tempC = Number(card.querySelector('.dev-disk-temp')?.value || 35)

    nextDisks.push({
      letter: letter,
      type: type,
      model: model,
      totalGB: totalGB,
      usedPct: usedPct,
      healthPct: health,
      tempC: tempC,
    })
  })

  if (nextDisks.length > 0) {
    devDisks = nextDisks
  }
}

function renderDisksList() {
  const container = $('dev-disks-container')
  if (!container) return

  if (!devDisks || devDisks.length === 0) {
    devDisks = [{
      letter: 'C',
      type: 'NVMe SSD',
      model: 'Samsung SSD 980 1TB',
      totalGB: 1000,
      usedPct: 45,
      healthPct: 99,
      tempC: 38,
    }]
  }

  container.innerHTML = devDisks.map((disk, idx) => {
    const letter = (disk.letter || 'C').toUpperCase().replace(/[^A-Z]/g, '')
    const canDelete = devDisks.length > 1
    const sizeOpts = [120, 240, 480, 512, 1000, 2000, 4000]

    return `
      <div class="dev-disk-item" data-disk-idx="${idx}">
        <div class="dev-disk-item-header">
          <div class="dev-disk-item-title">
            <b>${letter}:</b> <span class="muted">(${escapeHtmlFn(disk.type || 'SSD')})</span>
          </div>
          ${canDelete ? `<button type="button" class="btn danger compact dev-disk-delete-btn" data-delete-idx="${idx}" title="Delete">✕</button>` : ''}
        </div>
        <div class="dev-disk-grid">
          <div class="dev-disk-col">
            <label class="dev-input-label">
              <span>Drive:</span>
              <select class="glass-control glass-select compact dev-disk-letter">
                ${['C', 'D', 'E', 'F', 'G', 'H', 'X'].map((l) => `<option value="${l}" ${l === letter ? 'selected' : ''}>${l}:</option>`).join('')}
              </select>
            </label>
            <label class="dev-input-label">
              <span>Type:</span>
              <select class="glass-control glass-select compact dev-disk-type">
                <option value="HDD" ${disk.type === 'HDD' ? 'selected' : ''}>${t('devDiskTypeHDD')}</option>
                <option value="SATA SSD" ${disk.type === 'SATA SSD' ? 'selected' : ''}>${t('devDiskTypeSATASSD')}</option>
                <option value="NVMe SSD" ${disk.type === 'NVMe SSD' ? 'selected' : ''}>${t('devDiskTypeNVMeSSD')}</option>
              </select>
            </label>
            <label class="dev-input-label">
              <span>Model:</span>
              <input type="text" class="glass-control compact dev-disk-model" value="${escapeHtmlFn(disk.model || '')}" placeholder="Model name" />
            </label>
          </div>
          <div class="dev-disk-col">
            <label class="dev-input-label">
              <span>Capacity:</span>
              <select class="glass-control glass-select compact dev-disk-size">
                ${sizeOpts.map((sz) => `<option value="${sz}" ${Math.abs(sz - disk.totalGB) < 20 ? 'selected' : ''}>${sz >= 1000 ? (sz / 1000) + ' TB' : sz + ' GB'}</option>`).join('')}
              </select>
            </label>
            <div class="dev-sensor-row">
              <div class="dev-sensor-label-col">
                <span>${t('devDiskUsedLabel')}:</span>
                <b class="dev-disk-used-val">${Math.round(disk.usedPct)}%</b>
              </div>
              <input type="range" class="dev-range dev-disk-used" min="0" max="99" value="${Math.round(disk.usedPct)}" />
            </div>
            <div class="dev-sensor-row">
              <div class="dev-sensor-label-col">
                <span>${t('devDiskHealthLabel')}:</span>
                <b class="dev-disk-health-val">${Math.round(disk.healthPct)}%</b>
              </div>
              <input type="range" class="dev-range dev-disk-health" min="1" max="100" value="${Math.round(disk.healthPct)}" />
            </div>
            <div class="dev-sensor-row">
              <div class="dev-sensor-label-col">
                <span>${t('devDiskTempLabel')}:</span>
                <b class="dev-disk-temp-val">${Math.round(disk.tempC)}°C</b>
              </div>
              <input type="range" class="dev-range dev-disk-temp" min="20" max="80" value="${Math.round(disk.tempC)}" />
            </div>
          </div>
        </div>
      </div>
    `
  }).join('')

  // Wire disk item events
  container.querySelectorAll('.dev-disk-delete-btn').forEach((btn) => {
    btn.addEventListener('click', () => {
      const idx = Number(btn.dataset.deleteIdx)
      syncDisksFromDOM()
      if (devDisks.length > 1) {
        devDisks.splice(idx, 1)
        renderDisksList()
        if (showToastFn) showToastFn(t('devDiskRemovedToast'), false)
      }
    })
  })

  container.querySelectorAll('.dev-disk-type').forEach((select) => {
    select.addEventListener('change', (e) => {
      const card = e.target.closest('.dev-disk-item')
      const modelInput = card?.querySelector('.dev-disk-model')
      if (modelInput) {
        if (e.target.value === 'HDD') {
          modelInput.value = 'WDC WD10EZEX'
        } else if (e.target.value === 'NVMe SSD') {
          modelInput.value = 'Samsung SSD 980 1TB'
        } else {
          modelInput.value = 'Kingston A400 480GB'
        }
      }
    })
  })

  container.querySelectorAll('.dev-disk-used').forEach((range) => {
    range.addEventListener('input', (e) => {
      const card = e.target.closest('.dev-disk-item')
      const valEl = card?.querySelector('.dev-disk-used-val')
      if (valEl) valEl.textContent = `${e.target.value}%`
    })
  })

  container.querySelectorAll('.dev-disk-health').forEach((range) => {
    range.addEventListener('input', (e) => {
      const card = e.target.closest('.dev-disk-item')
      const valEl = card?.querySelector('.dev-disk-health-val')
      if (valEl) valEl.textContent = `${e.target.value}%`
    })
  })

  container.querySelectorAll('.dev-disk-temp').forEach((range) => {
    range.addEventListener('input', (e) => {
      const card = e.target.closest('.dev-disk-item')
      const valEl = card?.querySelector('.dev-disk-temp-val')
      if (valEl) valEl.textContent = `${e.target.value}°C`
    })
  })
}

function wireTelemetryInspector() {
  $('btn-dev-refresh-telemetry')?.addEventListener('click', () => {
    refreshTelemetryTable()
  })
}

async function refreshTelemetryTable() {
  const tbody = $('dev-telemetry-tbody')
  if (!tbody) return

  try {
    const items = await GetRawTelemetry()
    if (!items || items.length === 0) {
      tbody.innerHTML = '<tr><td colspan="4" class="muted text-center">No telemetry items available</td></tr>'
      return
    }

    tbody.innerHTML = items.map((item) => {
      let sourceCls = 'badge-simulated'
      if (item.source === 'Real') sourceCls = 'badge-real'
      else if (item.source === 'NVML') sourceCls = 'badge-nvml'
      else if (item.source === 'WMI') sourceCls = 'badge-wmi'
      else if (item.source === 'SMART') sourceCls = 'badge-smart'

      return `
        <tr>
          <td><b>${escapeHtmlFn(item.metric || item.name || '')}</b></td>
          <td><code>${escapeHtmlFn(item.value || '')}</code></td>
          <td><span class="badge ${sourceCls}">${escapeHtmlFn(item.source || '')}</span></td>
          <td class="muted">${escapeHtmlFn(item.updated || item.updatedAt || '')}</td>
        </tr>
      `
    }).join('')
  } catch (err) {
    console.error('Failed to get raw telemetry:', err)
  }
}

function wireI18nDebug() {
  const toggle = $('toggle-i18n-debug')
  if (toggle) {
    toggle.checked = isI18nDev()
    toggle.addEventListener('change', (e) => {
      const on = !!e.target.checked
      setI18nDev(on)
      document.body.classList.toggle('i18n-debug-active', on)
      runI18nDebugScan()
    })
  }
}

function runI18nDebugScan() {
  const statusEl = $('dev-i18n-status')
  const listEl = $('dev-i18n-missing-list')
  if (!statusEl || !listEl) return

  const activeLang = getLang() || 'ru'
  const enKeys = catalogKeys('en') || []
  const currentCatalog = _catalogs[activeLang] || {}

  const missing = []
  enKeys.forEach((key) => {
    if (currentCatalog[key] == null || String(currentCatalog[key]).trim() === '') {
      missing.push(key)
    }
  })

  if (missing.length === 0) {
    statusEl.textContent = t('devI18nMissingZero')
    statusEl.classList.remove('bad')
    statusEl.classList.add('ok')
    listEl.classList.add('hidden')
    listEl.innerHTML = ''
  } else {
    statusEl.textContent = t('devI18nMissingCount', { count: missing.length })
    statusEl.classList.remove('ok')
    statusEl.classList.add('bad')
    listEl.classList.remove('hidden')
    listEl.innerHTML = missing.map((k) => `<div>${escapeHtmlFn(k)}</div>`).join('')
  }
}

function wireDryRunTests() {
  $('btn-test-dryrun-clean')?.addEventListener('click', async () => {
    try {
      const res = await ApplyDiskCleanup([])
      const msg = (getLang() === 'ru' ? res.dryRunMessageRu : res.dryRunMessageEn) ||
        res.dryRunMessageRu || res.dryRunMessageEn || '[DRY-RUN] Disk cleanup simulated'
      if (showToastFn) showToastFn(msg, true)
    } catch (err) {
      if (showToastFn) showToastFn(String(err), false)
    }
  })

  $('btn-test-dryrun-eco')?.addEventListener('click', async () => {
    try {
      const res = await ApplyEcoWithFeedback('dev_test_app.exe')
      const msg = (getLang() === 'ru' ? res.messageRu : res.messageEn) ||
        res.messageRu || res.messageEn || '[DRY-RUN] Process freeze intercepted'
      if (showToastFn) showToastFn(msg, true)
    } catch (err) {
      if (showToastFn) showToastFn(String(err), false)
    }
  })

  $('btn-test-dryrun-unlock')?.addEventListener('click', async () => {
    try {
      const res = await UnlockFileDryRun('C:\\Games\\Cyberpunk2077\\archive.dat')
      const msg = (getLang() === 'ru' ? res.messageRu : res.messageEn) ||
        res.messageRu || res.messageEn || '[DRY-RUN] File unlock intercepted'
      if (showToastFn) showToastFn(msg, true)
    } catch (err) {
      if (showToastFn) showToastFn(String(err), false)
    }
  })
}
