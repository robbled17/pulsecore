/**
 * Contract: every loc.M("code") / loc.Msg code emitted by backend exists in ru+en,
 * and Msg JSON shape is {code, params?}.
 */
import assert from 'assert'
import fs from 'fs'
import path from 'path'
import { fileURLToPath } from 'url'
import { spawnSync } from 'child_process'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const root = path.resolve(__dirname, '..', '..')
const localesDir = path.join(__dirname, '..', 'src', 'i18n', 'locales')

const en = JSON.parse(fs.readFileSync(path.join(localesDir, 'en.json'), 'utf8'))
const ru = JSON.parse(fs.readFileSync(path.join(localesDir, 'ru.json'), 'utf8'))

function collectGoCodes() {
  const codes = new Set()
  const walk = (dir) => {
    for (const name of fs.readdirSync(dir)) {
      const p = path.join(dir, name)
      const st = fs.statSync(p)
      if (st.isDirectory()) {
        if (name === 'frontend' || name === 'build' || name === 'node_modules') continue
        walk(p)
        continue
      }
      if (!name.endsWith('.go') || name.endsWith('_test.go')) continue
      const text = fs.readFileSync(p, 'utf8')
      for (const m of text.matchAll(/loc\.M\(\s*"([^"]+)"/g)) codes.add(m[1])
      for (const m of text.matchAll(/Code:\s*"([a-zA-Z][a-zA-Z0-9_]*)"/g)) {
        if (m[1].startsWith('disk') || m[1].startsWith('home') || m[1].startsWith('adv') || m[1].startsWith('eco') || m[1].startsWith('stats') || m[1].startsWith('err')) {
          codes.add(m[1])
        }
      }
      // Status codes like diskStateGood
      for (const m of text.matchAll(/StatusCode\s*=\s*"([^"]+)"/g)) codes.add(m[1])
      for (const m of text.matchAll(/"diskState(Good|Warn|Bad|Unknown)"/g)) codes.add(m[0].slice(1, -1))
    }
  }
  walk(root)
  // Known status codes set in helpers
  ;['diskStateGood', 'diskStateWarn', 'diskStateBad', 'diskStateUnknown'].forEach((c) => codes.add(c))
  return [...codes].sort()
}

const codes = collectGoCodes()
let failed = 0
for (const code of codes) {
  // Dynamic advErr_* may use unknown suffixes — require base keys for concrete ones only
  if (code.startsWith('advErr_') && !(code in en)) {
    // Allowed if errGeneric exists as fallback path; still prefer concrete keys
    if (!('errGeneric' in en)) {
      console.error('missing errGeneric for advErr fallback')
      failed++
    }
    continue
  }
  if (!(code in en) || String(en[code]).trim() === '') {
    console.error(`[contract] en missing: ${code}`)
    failed++
  }
  if (!(code in ru) || String(ru[code]).trim() === '') {
    console.error(`[contract] ru missing: ${code}`)
    failed++
  }
}

// Shape check via go test dump (statusReason has code field)
const go = spawnSync('go', ['test', '-count=1', '-run', 'TestDiskJSONFixturesDump', '-v', '.'], {
  cwd: root,
  encoding: 'utf8',
  timeout: 60000,
})
if (go.status !== 0) {
  console.error(go.stdout || go.stderr)
  console.error('[contract] go dump failed')
  failed++
} else {
  assert.ok(go.stdout.includes('"statusReason"'), 'statusReason field')
  assert.ok(go.stdout.includes('"code"'), 'code field in Msg')
  // caution fixture uses params
  assert.ok(go.stdout.includes('diskReasonPending') || go.stdout.includes('"params"'), 'coded reason present')
}

console.log(`scanned ${codes.length} backend message codes`)
if (failed) {
  console.error(`msg contract FAILED (${failed})`)
  process.exit(1)
}
console.log('msg contract OK')
