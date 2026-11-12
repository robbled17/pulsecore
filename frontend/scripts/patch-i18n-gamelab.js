const fs = require('fs')
const path = require('path')
const p = path.join(__dirname, '..', 'src', 'i18n.js')
let s = fs.readFileSync(p, 'utf8')

const extras = {
  ru: {
    tabGameLab: '🎮 Game Lab',
    aboutBtn: 'О программе',
    aboutTitle: 'О программе',
    aboutAuthor: 'Автор',
    aboutStatus: 'Статус',
    gameLabTitle: 'Game Lab',
    gameLabSub: 'Предиктор FPS и живой оверлей кадровой плавности',
    overlayTitle: 'Live Frame Overlay',
    overlayToggle: 'Включить оверлей (F12)',
    overlayHint: 'FPS · 1% Low · Frametime поверх всех окон',
    predictorTitle: 'Game FPS Predictor',
    gameSearchPh: 'Поиск игр...',
    pickGame: 'Выберите игру, чтобы увидеть прогноз FPS',
    fpsLow: 'Низкие',
    fpsMed: 'Средние',
    fpsHigh: 'Высокие',
    fpsUltra: 'Ультра',
    fpsRec: 'Рекомендация для стабильных 60 FPS',
    presetLow: 'Низкие',
    presetMed: 'Средние',
    presetHigh: 'Высокие',
    presetUltra: 'Ультра',
    sleepOk: '✓ Ничего не мешает сну: все процессы спят, батарея ночью не разряжается.',
    sleepBlocked: '⚠️ Процесс [{name}] удерживает систему и не дает ноутбуку уснуть.',
    sleepFail: 'Не удалось опросить состояние сна Windows.',
    ecoLimited15: '✓ Ограничен до 15%',
    singleChannelNote: 'Одноканальная память снижает прогноз FPS.',
  },
  en: {
    tabGameLab: '🎮 Game Lab',
    aboutBtn: 'About',
    aboutTitle: 'About',
    aboutAuthor: 'Author',
    aboutStatus: 'Status',
    gameLabTitle: 'Game Lab',
    gameLabSub: 'FPS predictor and live frame overlay',
    overlayTitle: 'Live Frame Overlay',
    overlayToggle: 'Enable overlay (F12)',
    overlayHint: 'FPS · 1% Low · Frametime above all windows',
    predictorTitle: 'Game FPS Predictor',
    gameSearchPh: 'Search games...',
    pickGame: 'Pick a game to see FPS prediction',
    fpsLow: 'Low',
    fpsMed: 'Medium',
    fpsHigh: 'High',
    fpsUltra: 'Ultra',
    fpsRec: 'Recommended preset for stable 60 FPS',
    presetLow: 'Low',
    presetMed: 'Medium',
    presetHigh: 'High',
    presetUltra: 'Ultra',
    sleepOk: '✓ Nothing blocks sleep: processes are idle, battery should hold overnight.',
    sleepBlocked: '⚠️ Process [{name}] keeps the system awake and blocks sleep.',
    sleepFail: 'Could not query Windows sleep blockers.',
    ecoLimited15: '✓ Limited to 15%',
    singleChannelNote: 'Single-channel RAM reduces predicted FPS.',
  },
  kk: {
    tabGameLab: '🎮 Game Lab', aboutBtn: 'Бағдарлама туралы', aboutTitle: 'Бағдарлама туралы', aboutAuthor: 'Автор', aboutStatus: 'Күй',
    gameLabTitle: 'Game Lab', gameLabSub: 'FPS болжамы және кадр оверлейі',
    overlayTitle: 'Live Frame Overlay', overlayToggle: 'Оверлейді қосу (F12)', overlayHint: 'FPS · 1% Low · Frametime',
    predictorTitle: 'Game FPS Predictor', gameSearchPh: 'Ойын іздеу...', pickGame: 'FPS болжамын көру үшін ойынды таңдаңыз',
    fpsLow: 'Төмен', fpsMed: 'Орташа', fpsHigh: 'Жоғары', fpsUltra: 'Ультра',
    fpsRec: 'Тұрақты 60 FPS үшін ұсыныс', presetLow: 'Төмен', presetMed: 'Орташа', presetHigh: 'Жоғары', presetUltra: 'Ультра',
    sleepOk: '✓ Ұйқыға ештеңе кедергі емес.', sleepBlocked: '⚠️ [{name}] процесс жүйені ояу ұстайды.', sleepFail: 'Ұйқы күйін оқу мүмкін емес.',
    ecoLimited15: '✓ 15%-ке шектелді', singleChannelNote: 'Бір арналы RAM FPS болжамын төмендетеді.',
  },
  tt: {
    tabGameLab: '🎮 Game Lab', aboutBtn: 'Программа турында', aboutTitle: 'Программа турында', aboutAuthor: 'Автор', aboutStatus: 'Статус',
    gameLabTitle: 'Game Lab', gameLabSub: 'FPS фаразы һәм кадр оверлее',
    overlayTitle: 'Live Frame Overlay', overlayToggle: 'Оверлейны кабызу (F12)', overlayHint: 'FPS · 1% Low · Frametime',
    predictorTitle: 'Game FPS Predictor', gameSearchPh: 'Уен эзләү...', pickGame: 'FPS фаразын күрү өчен уен сайлагыз',
    fpsLow: 'Түбән', fpsMed: 'Урта', fpsHigh: 'Югары', fpsUltra: 'Ультра',
    fpsRec: 'Торыклы 60 FPS өчен тәкъдим', presetLow: 'Түбән', presetMed: 'Урта', presetHigh: 'Югары', presetUltra: 'Ультра',
    sleepOk: '✓ Йокыга бернәрсә дә комачауламый.', sleepBlocked: '⚠️ [{name}] процесс системаны уяу тота.', sleepFail: 'Йокы халәтен укып булмады.',
    ecoLimited15: '✓ 15%-ка чикләнде', singleChannelNote: 'Берканаллы RAM FPS фаразын киметә.',
  },
  de: {
    tabGameLab: '🎮 Game Lab', aboutBtn: 'Über', aboutTitle: 'Über die App', aboutAuthor: 'Autor', aboutStatus: 'Status',
    gameLabTitle: 'Game Lab', gameLabSub: 'FPS-Prognose und Live-Frame-Overlay',
    overlayTitle: 'Live Frame Overlay', overlayToggle: 'Overlay aktivieren (F12)', overlayHint: 'FPS · 1% Low · Frametime über allen Fenstern',
    predictorTitle: 'Game FPS Predictor', gameSearchPh: 'Spiele suchen...', pickGame: 'Spiel wählen für FPS-Prognose',
    fpsLow: 'Niedrig', fpsMed: 'Mittel', fpsHigh: 'Hoch', fpsUltra: 'Ultra',
    fpsRec: 'Empfehlung für stabile 60 FPS', presetLow: 'Niedrig', presetMed: 'Mittel', presetHigh: 'Hoch', presetUltra: 'Ultra',
    sleepOk: '✓ Nichts blockiert den Schlaf.', sleepBlocked: '⚠️ Prozess [{name}] hält das System wach.', sleepFail: 'Schlafblocker konnten nicht gelesen werden.',
    ecoLimited15: '✓ Auf 15% begrenzt', singleChannelNote: 'Einkanal-RAM senkt die FPS-Prognose.',
  },
  ja: {
    tabGameLab: '🎮 Game Lab', aboutBtn: 'このアプリについて', aboutTitle: 'このアプリについて', aboutAuthor: '作者', aboutStatus: 'ステータス',
    gameLabTitle: 'Game Lab', gameLabSub: 'FPS予測とライブフレームオーバーレイ',
    overlayTitle: 'Live Frame Overlay', overlayToggle: 'オーバーレイを有効化 (F12)', overlayHint: 'FPS · 1% Low · Frametime',
    predictorTitle: 'Game FPS Predictor', gameSearchPh: 'ゲーム検索...', pickGame: 'ゲームを選んでFPS予測を表示',
    fpsLow: '低', fpsMed: '中', fpsHigh: '高', fpsUltra: 'ウルトラ',
    fpsRec: '安定60FPS向けの推奨設定', presetLow: '低', presetMed: '中', presetHigh: '高', presetUltra: 'ウルトラ',
    sleepOk: '✓ スリープを妨げるものはありません。', sleepBlocked: '⚠️ プロセス [{name}] がスリープを妨げています。', sleepFail: 'スリープ状態を取得できません。',
    ecoLimited15: '✓ 15%に制限', singleChannelNote: 'シングルチャネルRAMは予測FPSを下げます。',
  },
  zh: {
    tabGameLab: '🎮 Game Lab', aboutBtn: '关于', aboutTitle: '关于程序', aboutAuthor: '作者', aboutStatus: '状态',
    gameLabTitle: 'Game Lab', gameLabSub: 'FPS 预测与实时帧率覆盖层',
    overlayTitle: 'Live Frame Overlay', overlayToggle: '启用覆盖层 (F12)', overlayHint: 'FPS · 1% Low · Frametime 置顶显示',
    predictorTitle: 'Game FPS Predictor', gameSearchPh: '搜索游戏...', pickGame: '选择游戏以查看 FPS 预测',
    fpsLow: '低', fpsMed: '中', fpsHigh: '高', fpsUltra: '超级',
    fpsRec: '稳定 60 FPS 的推荐画质', presetLow: '低', presetMed: '中', presetHigh: '高', presetUltra: '超级',
    sleepOk: '✓ 没有阻止睡眠的进程，夜间电池应保持稳定。', sleepBlocked: '⚠️ 进程 [{name}] 阻止系统进入睡眠。', sleepFail: '无法查询睡眠阻止项。',
    ecoLimited15: '✓ 已限制到 15%', singleChannelNote: '单通道内存会降低预测 FPS。',
  },
}

for (const [lang, obj] of Object.entries(extras)) {
  const marker = `${lang}: {`
  const idx = s.indexOf(marker)
  if (idx < 0) {
    console.error('miss', lang)
    continue
  }
  // Insert before closing of that language object: find next "\n  },\n  xx:" or end
  const start = idx
  let depth = 0
  let end = -1
  for (let i = start; i < s.length; i++) {
    if (s[i] === '{') depth++
    if (s[i] === '}') {
      depth--
      if (depth === 0) {
        end = i
        break
      }
    }
  }
  if (end < 0) continue
  let block = s.slice(start, end)
  for (const [k, v] of Object.entries(obj)) {
    const re = new RegExp(`\\b${k}\\s*:`)
    if (re.test(block)) {
      block = block.replace(new RegExp(`${k}\\s*:\\s*("[^"]*"|'[^']*')`), `${k}: ${JSON.stringify(v)}`)
    } else {
      block = block.replace(/\s*$/, `\n    ${k}: ${JSON.stringify(v)},\n  `)
    }
  }
  s = s.slice(0, start) + block + s.slice(end)
}

fs.writeFileSync(p, s)
console.log('patched gamelab i18n')
