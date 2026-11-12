const fs = require('fs')
const p = require('path').join(__dirname, '..', 'src', 'i18n.js')
let s = fs.readFileSync(p, 'utf8')

const extras = {
  ru: {
    ecoLimited15: '✓ Ограничено до 15%',
    procDefender: 'Защитник Windows',
    procDefenderDetail: 'Фоновый антивирусный скан',
    procIndexer: 'Индексатор Windows',
    procIndexerDetail: 'Индексация файлов',
    procBrowser: 'Браузер',
    procBrowserDetail: 'Фоновый расход батареи и трафика',
    groupInput: 'Устройства ввода',
    groupAudio: 'Звук и камера',
    groupStorage: 'Внешние накопители',
    groupDisplay: 'Дисплеи',
    smartRefresh: 'Обновить',
  },
  en: {
    ecoLimited15: '✓ Limited to 15%',
    procDefender: 'Windows Defender',
    procDefenderDetail: 'Background antivirus scan',
    procIndexer: 'Windows Search Indexer',
    procIndexerDetail: 'File indexing',
    procBrowser: 'Web Browser',
    procBrowserDetail: 'Frequent battery and traffic consumer',
    groupInput: 'Input devices',
    groupAudio: 'Audio & camera',
    groupStorage: 'External storage',
    groupDisplay: 'Displays',
    smartRefresh: 'Refresh',
    ecoAppetite: 'Throttle',
  },
  kk: {
    ecoLimited15: '✓ 15%-ке шектелді',
    procDefender: 'Windows Defender',
    procDefenderDetail: 'Фондық антивирус сканы',
    procIndexer: 'Windows Search Indexer',
    procIndexerDetail: 'Файл индекстеу',
    procBrowser: 'Браузер',
    procBrowserDetail: 'Батарея мен трафикті тұтыну',
    groupInput: 'Енгізу құрылғылары',
    groupAudio: 'Дыбыс және камера',
    groupStorage: 'Сыртқы дискілер',
    groupDisplay: 'Дисплейлер',
    smartRefresh: 'Жаңарту',
    ecoAppetite: 'Шектеу',
  },
  tt: {
    ecoLimited15: '✓ 15%-ка чикләнде',
    procDefender: 'Windows Defender',
    procDefenderDetail: 'Фон антивирус сканы',
    procIndexer: 'Windows Search Indexer',
    procIndexerDetail: 'Файл индексациясе',
    procBrowser: 'Браузер',
    procBrowserDetail: 'Батарея һәм трафик куллану',
    groupInput: 'Кертү җайланмалары',
    groupAudio: 'Тавыш һәм камера',
    groupStorage: 'Тышкы дисклар',
    groupDisplay: 'Дисплейлар',
    smartRefresh: 'Яңарту',
    ecoAppetite: 'Тизерү',
  },
  de: {
    ecoLimited15: '✓ Auf 15% begrenzt',
    procDefender: 'Windows Defender',
    procDefenderDetail: 'Hintergrund-Antivirenscan',
    procIndexer: 'Windows Search Indexer',
    procIndexerDetail: 'Dateindizierung',
    procBrowser: 'Webbrowser',
    procBrowserDetail: 'Häufiger Akku- und Traffic-Verbraucher',
    groupInput: 'Eingabegeräte',
    groupAudio: 'Audio & Kamera',
    groupStorage: 'Externe Laufwerke',
    groupDisplay: 'Displays',
    smartRefresh: 'Aktualisieren',
    ecoAppetite: 'Drosseln',
  },
  ja: {
    ecoLimited15: '✓ 15%に制限',
    procDefender: 'Windows Defender',
    procDefenderDetail: 'バックグラウンドウイルススキャン',
    procIndexer: 'Windows Search Indexer',
    procIndexerDetail: 'ファイルのインデックス',
    procBrowser: 'Webブラウザ',
    procBrowserDetail: 'バッテリーと通信量の消費',
    groupInput: '入力デバイス',
    groupAudio: '音声とカメラ',
    groupStorage: '外部ストレージ',
    groupDisplay: 'ディスプレイ',
    smartRefresh: '更新',
    ecoAppetite: '抑制',
  },
  zh: {
    ecoLimited15: '✓ 已限制到 15%',
    procDefender: 'Windows Defender',
    procDefenderDetail: '后台杀毒扫描',
    procIndexer: 'Windows Search Indexer',
    procIndexerDetail: '文件索引',
    procBrowser: '浏览器',
    procBrowserDetail: '常见耗电与流量占用',
    groupInput: '输入设备',
    groupAudio: '音频与摄像头',
    groupStorage: '外部存储',
    groupDisplay: '显示器',
    smartRefresh: '刷新',
    ecoAppetite: '节流',
  },
}

for (const [lang, obj] of Object.entries(extras)) {
  const re = new RegExp(`(${lang}:\\s*\\{[\\s\\S]*?)(weeklyEmpty:[^,\\n]+,)`)
  if (!re.test(s)) {
    console.error('miss', lang)
    continue
  }
  s = s.replace(re, (m, a, b) => {
    if (a.includes('procDefender:')) return m
    const lines = Object.entries(obj)
      .map(([k, v]) => `    ${k}: ${JSON.stringify(v)},`)
      .join('\n')
    return `${a}${b}\n${lines}`
  })
}

fs.writeFileSync(p, s)
console.log('ok', s.includes('procDefenderDetail'))
