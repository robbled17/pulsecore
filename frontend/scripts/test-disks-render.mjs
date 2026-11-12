/**
 * Fixture tests for Home disk pills + cards HTML.
 */
import assert from 'assert'
import fs from 'fs'
import path from 'path'
import { fileURLToPath } from 'url'
import { buildDiskCardsHTML, buildDiskPillsHTML, diskStatusClass } from '../src/disks/renderDisks.js'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const localesDir = path.join(__dirname, '..', 'src', 'i18n', 'locales')
const catalogs = {
  en: JSON.parse(fs.readFileSync(path.join(localesDir, 'en.json'), 'utf8')),
  ru: JSON.parse(fs.readFileSync(path.join(localesDir, 'ru.json'), 'utf8')),
}
let lang = 'ru'
function setLang(l) { lang = l }
function t(key, params = {}) {
  let raw = catalogs[lang]?.[key]
  if (raw == null) raw = catalogs.en?.[key]
  if (raw == null) return '—'
  return String(raw).replace(/\{([a-zA-Z0-9_]+)\}/g, (_, k) => (params[k] == null ? `{${k}}` : String(params[k])))
}
function tMsg(msg, legacyRu, legacyEn) {
  const code = msg && (msg.code || msg.Code)
  if (code) return t(code, msg.params || msg.Params || {})
  return lang === 'en' ? (legacyEn || legacyRu || '') : (legacyRu || legacyEn || '')
}

setLang('ru')

function escapeHtml(s) {
  return String(s ?? '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
}
const fmt = (n, d = 0) => Number(n).toFixed(d)
const msgText = (msg, ru, en) => tMsg(msg, ru, en)
const diskStatusText = (r) => {
  if (!r) return t('diskStateUnknown')
  if (r.statusCode) return t(r.statusCode)
  return r.statusWordRu || t('diskStateUnknown')
}
const diskHealthLabel = () => t('diskStateUnknown')
const formatCapacityGB = (gb) => (gb > 0 ? `${fmt(gb, 0)} GB` : '')
const mediaBadge = (r) => {
  if (r?.mediaKind === 'hdd') return 'HDD'
  if (r?.mediaKind === 'nvme' || r?.mediaKind === 'sata_ssd') return 'SSD'
  return t('mediaUnknown')
}
const chevronSVG = () => ''

const disks = [
  { mount: 'C:\\', percent: 40, freeGB: 300, totalGB: 500 },
  { mount: 'D:\\', percent: 10, freeGB: 900, totalGB: 1000 },
  { mount: 'E:\\', percent: 55, freeGB: 45, totalGB: 100 },
]

const fixtures = {
  ssd_ok: {
    C: {
      model: 'Samsung SSD 980', mediaKind: 'nvme', mediaType: 'NVMe SSD',
      statusLevel: 'good', statusCode: 'diskStateGood',
      healthPercent: 98, healthPercentOK: true, tempAvailable: true, temperatureC: 42,
      capacityGB: 500, statusReason: { code: '' },
    },
  },
  hdd: {
    D: {
      model: 'WD Blue', mediaKind: 'hdd', mediaType: 'HDD', rotationRPM: 7200,
      statusLevel: 'good', statusCode: 'diskStateGood',
      healthPercent: 91, healthPercentOK: true, tempAvailable: true, temperatureC: 35,
      capacityGB: 1000, statusReason: { code: '' },
    },
  },
  smart_unavailable: {
    E: {
      model: 'USB Disk', mediaKind: 'unknown',
      statusLevel: 'unknown', statusCode: 'diskStateUnknown',
      statusReason: { code: 'diskReasonSmartUnread' },
      statusReasonRu: 'SMART не прочитан', statusReasonEn: 'SMART was not read',
      healthPercentOK: false,
    },
  },
  helper_off: {
    C: {
      needsAdmin: true, statusLevel: 'unknown', statusCode: 'diskStateUnknown',
      statusWordRu: 'Включите расширенный режим', statusWordEn: 'Enable advanced mode',
      adminBannerRu: 'banner', adminBannerEn: 'banner',
      statusReason: { code: '' },
    },
  },
  caution: {
    D: {
      model: 'WD HDD', mediaKind: 'hdd', mediaType: 'HDD',
      statusLevel: 'caution', statusCode: 'diskStateWarn',
      statusReason: { code: 'diskReasonPending', params: { n: 3 } },
      healthPercent: 72, healthPercentOK: true, tempAvailable: true, temperatureC: 38,
      capacityGB: 1000,
    },
  },
}

function ctx(expanded = new Set()) {
  return {
    t, msgText, fmt, escapeHtml, diskStatusText, diskHealthLabel,
    formatCapacityGB, mediaBadge, chevronSVG, expanded, diskHistory: {},
  }
}

function getReportFactory(map) {
  return (letter) => map[letter]
}

let failed = 0
function check(name, fn) {
  try {
    fn()
    console.log('OK', name)
  } catch (e) {
    failed++
    console.error('FAIL', name, e.message)
  }
}

check('pills+cards ssd_ok', () => {
  const map = { ...fixtures.ssd_ok }
  const pills = buildDiskPillsHTML(disks, getReportFactory(map), ctx())
  assert.ok(pills.includes('disk-chip'), 'pills')
  assert.ok(pills.includes('C:'), 'C pill')
  const { html, needsAdmin } = buildDiskCardsHTML(disks, getReportFactory(map), ctx())
  assert.ok(html.includes('disk-card'), 'cards')
  assert.ok(html.includes('disk-expand-btn'), 'all specs btn')
  assert.ok(html.includes('Samsung SSD 980'), 'model')
  assert.ok(html.includes('disk-status-pill'), 'status pill')
  assert.equal(needsAdmin, false)
})

check('hdd card', () => {
  const map = { ...fixtures.hdd }
  const { html } = buildDiskCardsHTML(disks, getReportFactory(map), ctx())
  assert.ok(html.includes('data-drive="D"'))
  assert.ok(html.includes('disk-card'))
})

check('smart unavailable', () => {
  const map = { ...fixtures.smart_unavailable }
  const pills = buildDiskPillsHTML(disks, getReportFactory(map), ctx())
  assert.ok(pills.includes('E:'))
  const { html } = buildDiskCardsHTML(disks, getReportFactory(map), ctx())
  assert.ok(html.includes('data-drive="E"'))
})

check('helper off needsAdmin', () => {
  const map = { ...fixtures.helper_off }
  const { html, needsAdmin } = buildDiskCardsHTML(disks, getReportFactory(map), ctx())
  assert.equal(needsAdmin, true)
  assert.ok(html.includes('disk-card'))
})

check('caution reason via code', () => {
  setLang('en')
  const map = { ...fixtures.caution }
  const pills = buildDiskPillsHTML(disks, getReportFactory(map), ctx())
  assert.ok(pills.includes('st-caution'), 'pill uses status code class')
  assert.ok(!pills.includes('disk-chip-why'), 'pill has no inline reason')
  assert.ok(pills.includes('title='), 'reason in tooltip')
  const { html } = buildDiskCardsHTML(disks, getReportFactory(map), ctx())
  assert.ok(html.includes('disk-status-reason'))
  assert.ok(html.includes('st-caution'))
  setLang('ru')
})

check('status class by code', () => {
  assert.equal(diskStatusClass('good'), 'st-good')
  assert.equal(diskStatusClass('caution'), 'st-caution')
  assert.equal(diskStatusClass('bad'), 'st-bad')
  assert.equal(diskStatusClass('unknown'), 'st-unknown')
  assert.equal(diskStatusClass('Хорошо'), 'st-unknown')
})

check('unknown msg code does not throw', () => {
  const text = tMsg({ code: 'definitelyMissingCodeXYZ', params: { a: 1 } })
  assert.equal(text, '—')
})

check('empty reports still render 3 cards+pills', () => {
  const pills = buildDiskPillsHTML(disks, () => null, ctx())
  assert.ok(pills.includes('C:') && pills.includes('D:') && pills.includes('E:'))
  const { html } = buildDiskCardsHTML(disks, () => null, ctx())
  assert.equal((html.match(/class="disk-card"/g) || []).length, 3)
  assert.ok(html.includes('disk-expand-btn'))
})

if (failed) {
  console.error(`disk render tests FAILED (${failed})`)
  process.exit(1)
}
console.log('disk render tests OK')
