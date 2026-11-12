/**
 * Compatibility shim — prefer importing from './i18n/index.js'.
 * Keeps legacy `i18n[lang][key]` lookups working during migration.
 */
export {
  t,
  tMsg,
  tPlural,
  setLang,
  getLang,
  listSelectableLangs,
  formatNumber,
  formatDate,
  formatTemplate,
  setI18nDev,
  isI18nDev,
  setPseudoLoc,
  isPseudoLoc,
  COMPLETE_LANGS,
  LANG_META,
  catalogKeys,
  _catalogs,
} from './i18n/index.js'

import { _catalogs as catalogs } from './i18n/index.js'

/** @deprecated use t(key, params) */
export const i18n = catalogs
