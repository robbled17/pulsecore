#!/usr/bin/env node
/**
 * Fails if any present locale JSON is not 100% vs en (missing/extra/empty),
 * or placeholder sets diverge, or UI still has hardcoded Cyrillic outside locales.
 * COMPLETE set = every *.json in locales/ (all must be 100%).
 */
import fs from 'fs'
import path from 'path'
import { fileURLToPath } from 'url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const root = path.resolve(__dirname, '..')
const localesDir = path.join(root, 'src', 'i18n', 'locales')

function loadJSON(id) {
  const p = path.join(localesDir, `${id}.json`)
  if (!fs.existsSync(p)) return null
  return JSON.parse(fs.readFileSync(p, 'utf8'))
}

function placeholders(str) {
  return [...String(str).matchAll(/\{([a-zA-Z0-9_]+)\}/g)].map((m) => m[1]).sort()
}

const en = loadJSON('en')
if (!en) {
  console.error('missing en.json')
  process.exit(1)
}
const enKeys = Object.keys(en).sort()
const enKeySet = new Set(enKeys)

const langIds = fs.readdirSync(localesDir)
  .filter((f) => f.endsWith('.json'))
  .map((f) => f.replace(/\.json$/, ''))
  .sort()

if (!langIds.includes('en') || !langIds.includes('ru')) {
  console.error('[fail] en.json and ru.json are required')
  process.exit(1)
}

let failed = false
for (const id of langIds) {
  const cat = loadJSON(id)
  if (!cat) {
    console.error(`[fail] cannot read ${id}.json`)
    failed = true
    continue
  }
  const keys = Object.keys(cat)
  const keySet = new Set(keys)
  for (const k of enKeys) {
    if (!keySet.has(k) || cat[k] == null || String(cat[k]).trim() === '') {
      console.error(`[fail] ${id} missing/empty: ${k}`)
      failed = true
    } else {
      const a = placeholders(en[k]).join(',')
      const b = placeholders(cat[k]).join(',')
      if (a !== b) {
        console.error(`[fail] ${id} placeholder mismatch on ${k}: en=[${a}] ${id}=[${b}]`)
        failed = true
      }
    }
  }
  for (const k of keys) {
    if (!enKeySet.has(k)) {
      console.error(`[fail] ${id} extra key: ${k}`)
      failed = true
    }
  }
}

const CYR = /[\u0400-\u04FF]/
const scanFiles = [
  path.join(root, 'src', 'main.js'),
  path.join(root, 'index.html'),
]
const allowRe = [
  /PulseCore/, /GitHub/, /PawnIO/, /SMART/, /NVMe/, /Eco Mode/, /Mini-HUD/, /Windows/, /CrystalDisk/,
]

for (const file of scanFiles) {
  const text = fs.readFileSync(file, 'utf8')
  const lines = text.split(/\r?\n/)
  lines.forEach((line, i) => {
    if (line.includes('data-i18n')) return
    if (line.trim().startsWith('//') || line.trim().startsWith('*') || line.trim().startsWith('<!--')) return
    if (file.endsWith('main.js') && CYR.test(line) && !allowRe.some((r) => r.test(line))) {
      if (/\/[^\n]*[\u0400-\u04FF]/.test(line) && line.includes('/')) return
      console.error(`[fail] hardcoded Cyrillic ${path.relative(root, file)}:${i + 1}: ${line.trim().slice(0, 120)}`)
      failed = true
    }
  })
}

const html = fs.readFileSync(path.join(root, 'index.html'), 'utf8')
for (const m of html.matchAll(/data-i18n(?:-tip|-placeholder|-title)?="([^"]+)"/g)) {
  if (!enKeySet.has(m[1])) {
    console.error(`[fail] data-i18n key missing in en.json: ${m[1]}`)
    failed = true
  }
}

// Selector contract: COMPLETE_LANGS in index.js must match existing 100% locale files
const indexSrc = fs.readFileSync(path.join(root, 'src', 'i18n', 'index.js'), 'utf8')
const m = indexSrc.match(/export const COMPLETE_LANGS = \[([^\]]+)\]/)
if (!m) {
  console.error('[fail] COMPLETE_LANGS not found in index.js')
  failed = true
} else {
  const listed = m[1].split(',').map((s) => s.replace(/['"\s]/g, '')).filter(Boolean)
  for (const id of listed) {
    if (!langIds.includes(id)) {
      console.error(`[fail] COMPLETE_LANGS includes ${id} but locales/${id}.json missing`)
      failed = true
    }
  }
  // Required wired set for this release
  for (const id of ['ru', 'en', 'de', 'ja', 'zh', 'kk', 'tt']) {
    if (!listed.includes(id)) {
      console.error(`[fail] COMPLETE_LANGS must include ${id}`)
      failed = true
    }
  }
}

if (failed) {
  console.error('i18n-check FAILED')
  process.exit(1)
}
console.log(`i18n-check OK (${enKeys.length} keys, langs: ${langIds.join(',')})`)
