const fs = require('fs')
const path = require('path')
const p = path.join(__dirname, '..', 'src', 'i18n.js')
let s = fs.readFileSync(p, 'utf8')

const extras = {
  ru: {
    tabGameLab: '🎮 GamePulse',
    gameLabTitle: 'GamePulse',
    gameLabSub: 'Предиктор FPS и живой оверлей кадровой плавности',
    lagExplain: 'Процесс резко нагрузил ядро #{core} на {load}%, что могло вызвать микрофриз',
    ecoLower: 'Снизить приоритет (Eco)',
    ecoAll: 'Ограничить все (Eco Mode)',
    procCount: 'процессов',
    unknownProc: 'Неизвестный процесс',
    netProgress: 'Диагностика сети: проверяем роутер и интернет...',
    netRouter: 'Домашний Wi-Fi / Роутер',
    netRouterHint: 'Задержка до шлюза локальной сети',
    netInternet: 'Интернет / Провайдер',
    netInternetHint: 'Отклик внешнего DNS (8.8.8.8)',
    netJitter: 'Сетевая стабильность (Джиттер)',
    netJitterHintOk: 'Скачков пинга нет, в играх задержек не будет',
    netJitterHintBad: 'Есть скачки задержки — возможны микролаги в онлайне',
    netBadgeIdeal: 'Идеальный сигнал',
    netBadgeWeak: 'Слабый Wi-Fi',
    netBadgeFail: 'Нет ответа',
    netBadgeFast: 'Быстрый отклик',
    netBadgeSlow: 'Медленный отклик',
    netBadgeStable: 'Стабильно',
    netBadgeUnstable: 'Нестабильно',
    netVerdictWifi: 'Проблема в Wi-Fi (слабый сигнал или перегрузка роутера).',
    netVerdictISP: 'Проблема у интернет-провайдера.',
    netVerdictPC: 'Сеть в порядке. Причина лагов внутри ПК.',
    netVerdictOK: 'Сеть и ПК в норме.',
    sleepOkDesktop: '✓ Все процессы спят, ночного разряда не обнаружено',
    sleepBlockedMedia: '⚠️ Процесс [{name}] не дает ноутбуку уснуть (активен аудио/видео поток)',
  },
  en: {
    tabGameLab: '🎮 GamePulse', gameLabTitle: 'GamePulse', gameLabSub: 'FPS predictor and live frame overlay',
    lagExplain: 'The process spiked core #{core} to {load}%, which may have caused a micro-stutter',
    ecoLower: 'Lower priority (Eco)', ecoAll: 'Throttle all (Eco Mode)', procCount: 'processes', unknownProc: 'Unknown process',
    netProgress: 'Network diagnostics: checking router and internet...',
    netRouter: 'Home Wi-Fi / Router', netRouterHint: 'Latency to your local gateway',
    netInternet: 'Internet / ISP', netInternetHint: 'External DNS response (8.8.8.8)',
    netJitter: 'Network stability (Jitter)', netJitterHintOk: 'No ping spikes — online games should feel smooth',
    netJitterHintBad: 'Latency spikes detected — online micro-stutters possible',
    netBadgeIdeal: 'Ideal signal', netBadgeWeak: 'Weak Wi-Fi', netBadgeFail: 'No reply',
    netBadgeFast: 'Fast response', netBadgeSlow: 'Slow response', netBadgeStable: 'Stable', netBadgeUnstable: 'Unstable',
    netVerdictWifi: 'Wi-Fi issue (weak signal or router congestion).',
    netVerdictISP: 'ISP issue detected.', netVerdictPC: 'Network is fine. Lag is inside the PC.', netVerdictOK: 'Network and PC look healthy.',
    sleepOkDesktop: '✓ All processes are sleeping — no overnight drain detected',
    sleepBlockedMedia: '⚠️ Process [{name}] keeps the laptop awake (active audio/video stream)',
  },
  kk: {
    tabGameLab: '🎮 GamePulse', gameLabTitle: 'GamePulse', gameLabSub: 'FPS болжамы және кадр оверлейі',
    lagExplain: 'Процесс #{core} ядросын {load}%-ға жүктеді — микрофриз болуы мүмкін',
    ecoLower: 'Приоритетті төмендету (Eco)', ecoAll: 'Барлығын шектеу (Eco)', procCount: 'процесс', unknownProc: 'Белгісіз процесс',
    netProgress: 'Желі диагностикасы...', netRouter: 'Үй Wi-Fi / Роутер', netRouterHint: 'Шлюзке дейінгі кідіріс',
    netInternet: 'Интернет / Провайдер', netInternetHint: 'Сыртқы DNS', netJitter: 'Желі тұрақтылығы',
    netJitterHintOk: 'Пинг тұрақты', netJitterHintBad: 'Пинг секірістері бар',
    netBadgeIdeal: 'Керемет сигнал', netBadgeWeak: 'Әлсіз Wi-Fi', netBadgeFail: 'Жауап жоқ',
    netBadgeFast: 'Жылдам', netBadgeSlow: 'Баяу', netBadgeStable: 'Тұрақты', netBadgeUnstable: 'Тұрақсыз',
    netVerdictWifi: 'Wi-Fi мәселесі.', netVerdictISP: 'Провайдер мәселесі.', netVerdictPC: 'Желі жақсы, лаг ПК ішінде.', netVerdictOK: 'Желі мен ПК қалыпты.',
    sleepOkDesktop: '✓ Барлық процесс ұйықтап жатыр', sleepBlockedMedia: '⚠️ [{name}] ноутбукті ұйықтатпайды',
  },
  tt: {
    tabGameLab: '🎮 GamePulse', gameLabTitle: 'GamePulse', gameLabSub: 'FPS фаразы һәм кадр оверлее',
    lagExplain: 'Процесс #{core} төшәгенне {load}%-ка йөкләде — микрофриз булырга мөмкин',
    ecoLower: 'Приоритетны төшерү (Eco)', ecoAll: 'Барысын чикләү (Eco)', procCount: 'процесс', unknownProc: 'Билгесез процесс',
    netProgress: 'Челтәр диагностикасы...', netRouter: 'Өй Wi-Fi / Роутер', netRouterHint: 'Шлюзга кадәр тоткарлык',
    netInternet: 'Интернет / Провайдер', netInternetHint: 'Тышкы DNS', netJitter: 'Челтәр тотрыклыгы',
    netJitterHintOk: 'Пинг тотрыклы', netJitterHintBad: 'Пинг сикерешләре бар',
    netBadgeIdeal: 'Идеаль сигнал', netBadgeWeak: 'Зәгыйфь Wi-Fi', netBadgeFail: 'Җавап юк',
    netBadgeFast: 'Тиз', netBadgeSlow: 'Әкрен', netBadgeStable: 'Тотрыклы', netBadgeUnstable: 'Тотрыксыз',
    netVerdictWifi: 'Wi-Fi проблемасы.', netVerdictISP: 'Провайдер проблемасы.', netVerdictPC: 'Челтәр яхшы, лаг ПК эчендә.', netVerdictOK: 'Челтәр һәм ПК нормада.',
    sleepOkDesktop: '✓ Барлык процесслар йоклый', sleepBlockedMedia: '⚠️ [{name}] ноутбукта йокларга бирми',
  },
  de: {
    tabGameLab: '🎮 GamePulse', gameLabTitle: 'GamePulse', gameLabSub: 'FPS-Prognose und Live-Frame-Overlay',
    lagExplain: 'Prozess hat Kern #{core} auf {load}% belastet — möglicher Micro-Stutter',
    ecoLower: 'Priorität senken (Eco)', ecoAll: 'Alle drosseln (Eco Mode)', procCount: 'Prozesse', unknownProc: 'Unbekannter Prozess',
    netProgress: 'Netzwerkdiagnose läuft...', netRouter: 'Heim-Wi-Fi / Router', netRouterHint: 'Latenz zum Gateway',
    netInternet: 'Internet / ISP', netInternetHint: 'Externes DNS', netJitter: 'Netzstabilität (Jitter)',
    netJitterHintOk: 'Keine Ping-Spitzen', netJitterHintBad: 'Latenzspitzen erkannt',
    netBadgeIdeal: 'Ideales Signal', netBadgeWeak: 'Schwaches Wi-Fi', netBadgeFail: 'Keine Antwort',
    netBadgeFast: 'Schnelle Antwort', netBadgeSlow: 'Langsam', netBadgeStable: 'Stabil', netBadgeUnstable: 'Instabil',
    netVerdictWifi: 'Wi-Fi-Problem.', netVerdictISP: 'ISP-Problem.', netVerdictPC: 'Netz OK, Lag im PC.', netVerdictOK: 'Netz und PC in Ordnung.',
    sleepOkDesktop: '✓ Alle Prozesse schlafen', sleepBlockedMedia: '⚠️ Prozess [{name}] verhindert den Schlaf (Audio/Video)',
  },
  ja: {
    tabGameLab: '🎮 GamePulse', gameLabTitle: 'GamePulse', gameLabSub: 'FPS予測とライブフレームオーバーレイ',
    lagExplain: 'プロセスがコア#{core}を{load}%に急上昇させ、マイクロスタッターの原因になった可能性があります',
    ecoLower: '優先度を下げる (Eco)', ecoAll: 'すべて制限 (Eco Mode)', procCount: 'プロセス', unknownProc: '不明なプロセス',
    netProgress: 'ネットワーク診断中...', netRouter: '自宅Wi-Fi / ルーター', netRouterHint: 'ゲートウェイ遅延',
    netInternet: 'インターネット / ISP', netInternetHint: '外部DNS', netJitter: 'ネットワーク安定性',
    netJitterHintOk: 'ピングの急変なし', netJitterHintBad: '遅延スパイクあり',
    netBadgeIdeal: '理想的な信号', netBadgeWeak: '弱いWi-Fi', netBadgeFail: '応答なし',
    netBadgeFast: '高速応答', netBadgeSlow: '遅い応答', netBadgeStable: '安定', netBadgeUnstable: '不安定',
    netVerdictWifi: 'Wi-Fiの問題。', netVerdictISP: 'ISPの問題。', netVerdictPC: 'ネットワークは正常。ラグはPC側。', netVerdictOK: 'ネットワークとPCは正常。',
    sleepOkDesktop: '✓ すべてのプロセスがスリープ中', sleepBlockedMedia: '⚠️ プロセス [{name}] がスリープを妨げています',
  },
  zh: {
    tabGameLab: '🎮 GamePulse', gameLabTitle: 'GamePulse', gameLabSub: 'FPS 预测与实时帧率覆盖层',
    lagExplain: '进程将核心 #{core} 瞬间拉到 {load}%，可能导致微卡顿',
    ecoLower: '降低优先级 (Eco)', ecoAll: '全部限制 (Eco Mode)', procCount: '个进程', unknownProc: '未知进程',
    netProgress: '正在诊断网络：检查路由器与互联网...',
    netRouter: '家庭 Wi-Fi / 路由器', netRouterHint: '到网关的延迟',
    netInternet: '互联网 / 运营商', netInternetHint: '外部 DNS 响应',
    netJitter: '网络稳定性（抖动）', netJitterHintOk: '无明显 Ping 波动，游戏延迟更稳', netJitterHintBad: '存在延迟尖峰',
    netBadgeIdeal: '理想信号', netBadgeWeak: 'Wi-Fi 较弱', netBadgeFail: '无响应',
    netBadgeFast: '响应迅速', netBadgeSlow: '响应偏慢', netBadgeStable: '稳定', netBadgeUnstable: '不稳定',
    netVerdictWifi: 'Wi-Fi 问题。', netVerdictISP: '运营商问题。', netVerdictPC: '网络正常，卡顿在电脑端。', netVerdictOK: '网络与电脑状态正常。',
    sleepOkDesktop: '✓ 所有进程都在休眠，未发现夜间掉电',
    sleepBlockedMedia: '⚠️ 进程 [{name}] 阻止笔记本睡眠（音视频流活跃）',
  },
}

for (const [lang, obj] of Object.entries(extras)) {
  const marker = `${lang}: {`
  const idx = s.indexOf(marker)
  if (idx < 0) continue
  let depth = 0, end = -1
  for (let i = idx; i < s.length; i++) {
    if (s[i] === '{') depth++
    if (s[i] === '}') {
      depth--
      if (depth === 0) { end = i; break }
    }
  }
  if (end < 0) continue
  let block = s.slice(idx, end)
  for (const [k, v] of Object.entries(obj)) {
    const re = new RegExp(`${k}\\s*:\\s*("[^"]*"|'[^']*')`)
    if (re.test(block)) block = block.replace(re, `${k}: ${JSON.stringify(v)}`)
    else block = block.replace(/\s*$/, `\n    ${k}: ${JSON.stringify(v)},\n  `)
  }
  s = s.slice(0, idx) + block + s.slice(end)
}
fs.writeFileSync(p, s)
console.log('ux i18n patched')
