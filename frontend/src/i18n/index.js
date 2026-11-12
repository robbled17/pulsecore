/**
 * PulseCore i18n — codes + params on the frontend.
 * Fallback: current lang → en → neutral / ⟦code⟧ (dev).
 */
import ru from './locales/ru.json'
import en from './locales/en.json'
import de from './locales/de.json'
import ja from './locales/ja.json'
import zh from './locales/zh.json'
import kk from './locales/kk.json'
import tt from './locales/tt.json'

/** Languages with 100% key coverage vs English catalog. */
export const COMPLETE_LANGS = ['ru', 'en', 'de', 'ja', 'zh', 'kk', 'tt']

export const LANG_META = {
  ru: { label: 'Русский' },
  en: { label: 'English' },
  de: { label: 'Deutsch' },
  ja: { label: '日本語' },
  zh: { label: '简体中文' },
  kk: { label: 'Қазақша' },
  tt: { label: 'Татарча' },
}

const LOCALE_TAG = {
  ru: 'ru-RU',
  en: 'en-US',
  de: 'de-DE',
  ja: 'ja-JP',
  zh: 'zh-CN',
  kk: 'kk-KZ',
  tt: 'tt-RU',
}

const catalogs = { ru, en, de, ja, zh, kk, tt }

let currentLang = 'ru'
let devHighlight = false
let pseudoLoc = false

export function setI18nDev(on) {
  devHighlight = !!on
  try { document.documentElement.classList.toggle('i18n-dev', devHighlight) } catch (_) {}
}

export function isI18nDev() {
  return devHighlight
}

/** Artificial string expansion for overflow QA (Settings / localStorage zp_i18n_pseudo=1). */
export function setPseudoLoc(on) {
  pseudoLoc = !!on
  try { document.documentElement.classList.toggle('i18n-pseudo', pseudoLoc) } catch (_) {}
}

export function isPseudoLoc() {
  return pseudoLoc
}

export function getLang() {
  return currentLang
}

export function setLang(lang) {
  const next = String(lang || 'en').toLowerCase()
  currentLang = COMPLETE_LANGS.includes(next) ? next : 'en'
  try {
    document.documentElement.lang = currentLang === 'zh' ? 'zh-CN' : currentLang
    document.documentElement.setAttribute('data-lang', currentLang)
  } catch (_) {}
  return currentLang
}

export function listSelectableLangs() {
  return COMPLETE_LANGS.map((id) => ({ id, label: (LANG_META[id] && LANG_META[id].label) || id }))
}

export function catalogKeys(lang = 'en') {
  return Object.keys(catalogs[lang] || catalogs.en || {})
}

function lookup(lang, key) {
  const cat = catalogs[lang]
  if (cat && Object.prototype.hasOwnProperty.call(cat, key) && cat[key] != null && cat[key] !== '') {
    return String(cat[key])
  }
  return null
}

export function formatTemplate(template, params = {}) {
  if (!params || typeof params !== 'object') return template
  return String(template).replace(/\{([a-zA-Z0-9_]+)\}/g, (_, k) => {
    if (params[k] == null) return `{${k}}`
    return String(params[k])
  })
}

function expandPseudo(text) {
  if (!pseudoLoc || !text) return text
  // Accents + length (~40%) without changing {placeholders}
  const parts = String(text).split(/(\{[a-zA-Z0-9_]+\})/)
  return parts.map((p) => {
    if (/^\{[a-zA-Z0-9_]+\}$/.test(p)) return p
    const accented = p
      .replace(/[aeiou]/gi, (c) => ({ a: 'á', e: 'é', i: 'í', o: 'ó', u: 'ú' }[c] || c))
      .replace(/[AEIOU]/g, (c) => ({ A: 'Á', E: 'É', I: 'Í', O: 'Ó', U: 'Ú' }[c] || c))
    const pad = Math.max(2, Math.ceil(p.length * 0.35))
    return `[!! ${accented}${'·'.repeat(pad)} !!]`
  }).join('')
}

/**
 * Plural forms:
 * - en/de: one / many
 * - ja/zh: always "many" key if present, else base (no grammatical plural)
 * - ru (+ kk/tt when added): one / few / many
 */
export function tPlural(baseKey, count, params = {}) {
  const n = Number(count) || 0
  const mod10 = n % 10
  const mod100 = n % 100
  let form = 'many'
  if (currentLang === 'ja' || currentLang === 'zh') {
    form = 'many'
  } else if (currentLang === 'en' || currentLang === 'de') {
    form = n === 1 ? 'one' : 'many'
  } else if (mod10 === 1 && mod100 !== 11) form = 'one'
  else if (mod10 >= 2 && mod10 <= 4 && !(mod100 >= 12 && mod100 <= 14)) form = 'few'
  const key = `${baseKey}_${form}`
  const hit = lookup(currentLang, key) || lookup('en', key) || lookup(currentLang, baseKey) || lookup('en', baseKey)
  return expandPseudo(formatTemplate(hit || baseKey, { ...params, count: n, n }))
}

const NEUTRAL_MISSING = '—'

export function t(key, params) {
  try {
    if (!key) return ''
    let raw = lookup(currentLang, key)
    let missing = false
    if (raw == null) {
      raw = lookup('en', key)
      missing = true
    }
    if (raw == null) {
      if (devHighlight) return `⟦${String(key)}⟧`
      return NEUTRAL_MISSING
    }
    let text = formatTemplate(raw, params)
    text = expandPseudo(text)
    if (devHighlight && missing) return `⟦${text}⟧`
    return text
  } catch (_) {
    return devHighlight ? `⟦${String(key || '')}⟧` : NEUTRAL_MISSING
  }
}

export function tMsg(msg, legacyRu, legacyEn) {
  try {
    const code = msg && (msg.code || msg.Code)
    if (code) return t(code, msg.params || msg.Params || {})
    if (currentLang === 'en') return expandPseudo(legacyEn || legacyRu || '')
    return expandPseudo(legacyRu || legacyEn || '')
  } catch (_) {
    return ''
  }
}

export function formatNumber(n, digits = 0) {
  const v = Number(n)
  if (!Number.isFinite(v)) return '-'
  try {
    return new Intl.NumberFormat(LOCALE_TAG[currentLang] || 'en-US', {
      maximumFractionDigits: digits,
      minimumFractionDigits: digits,
    }).format(v)
  } catch (_) {
    return v.toFixed(digits)
  }
}

export function formatDate(d, opts) {
  try {
    const date = d instanceof Date ? d : new Date(d)
    if (Number.isNaN(date.getTime())) return '-'
    return new Intl.DateTimeFormat(LOCALE_TAG[currentLang] || 'en-US', opts || {
      dateStyle: 'medium',
      timeStyle: 'short',
    }).format(date)
  } catch (_) {
    return String(d || '-')
  }
}

export { catalogs as _catalogs }
