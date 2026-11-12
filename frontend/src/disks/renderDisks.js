/**
 * Pure disk tile HTML builders (no DOM). Used by main.js and fixture tests.
 */

export function driveLetter(mount) {
  return String(mount || 'C').replace(':\\', '').replace(':', '').toUpperCase()
}

/** Map statusLevel CODE → CSS class (never localized text). */
export function diskStatusClass(level) {
  switch (String(level || '')) {
    case 'good': return 'st-good'
    case 'caution': return 'st-caution'
    case 'bad': return 'st-bad'
    default: return 'st-unknown'
  }
}

/**
 * @param {object} ctx
 * @param {function} ctx.t
 * @param {function} ctx.msgText
 * @param {function} ctx.fmt
 * @param {function} ctx.escapeHtml
 * @param {function} ctx.diskStatusText
 * @param {function} ctx.diskHealthLabel
 * @param {function} ctx.formatCapacityGB
 * @param {function} ctx.mediaBadge
 * @param {function} ctx.chevronSVG
 * @param {Set<string>} ctx.expanded
 * @param {object} ctx.diskHistory
 */
export function buildDiskPillsHTML(disks, getReport, ctx) {
  const { t, msgText, fmt, escapeHtml, diskStatusText, diskHealthLabel, diskHistory } = ctx
  if (!disks || !disks.length) return ''
  return `<span class="disk-chips">${disks.map((d) => {
    const letter = driveLetter(d.mount)
    const r = getReport(letter)
    const health = diskStatusText(r) || diskHealthLabel(diskHistory?.[letter]?.lastStatus)
    const st = diskStatusClass(r?.statusLevel)
    let tip = ''
    if (r && (r.statusLevel === 'caution' || r.statusLevel === 'bad')) {
      tip = msgText(r.statusReason, r.statusReasonRu, r.statusReasonEn) || ''
      if (!tip && r.healthPercentOK) {
        tip = t('diskHealthPctFallback', { pct: fmt(r.healthPercent, 0) })
      }
    }
    const tipAttr = tip ? ` title="${escapeHtml(tip)}"` : ''
    // Short pill only: "C: Caution" — reason stays in tooltip + card.
    return `<span class="disk-chip ${st}"${tipAttr}>${letter}: ${escapeHtml(health)}</span>`
  }).join('')}</span>`
}

export function buildDiskCardsHTML(disks, getReport, ctx) {
  const {
    t, msgText, fmt, escapeHtml, diskStatusText,
    formatCapacityGB, mediaBadge, chevronSVG, expanded,
  } = ctx
  if (!disks || !disks.length) return { html: '', needsAdmin: false, adminRu: '', adminEn: '' }

  let needsAdmin = false
  let adminRu = ''
  let adminEn = ''
  const html = disks.map((d) => {
    const letter = driveLetter(d.mount)
    const r = getReport(letter)
    if (r?.needsAdmin) {
      needsAdmin = true
      adminRu = r.adminBannerRu || adminRu
      adminEn = r.adminBannerEn || adminEn
    }
    const model = r?.model || ''
    const cap = formatCapacityGB(r?.capacityGB || d.totalGB)
    const badge = mediaBadge(r, d)
    const status = diskStatusText(r)
    const stClass = diskStatusClass(r?.statusLevel)
    let reason = (r?.statusLevel === 'caution' || r?.statusLevel === 'bad')
      ? msgText(r.statusReason, r.statusReasonRu, r.statusReasonEn)
      : ''
    if (!reason && (r?.statusLevel === 'caution' || r?.statusLevel === 'bad') && r?.healthPercentOK) {
      reason = t('diskHealthPctFallback', { pct: fmt(r.healthPercent, 0) })
    }
    const temp = r?.tempAvailable && r.temperatureC > 0 ? `${fmt(r.temperatureC, 0)} °C` : ''
    const health = r?.healthPercentOK ? `${fmt(r.healthPercent, 0)}%` : ''
    const freeGB = d.freeGB != null ? fmt(d.freeGB, 0) : ''
    const usedPct = Math.min(100, Number(d.percent) || 0)
    const pohHuman = msgText(r?.powerOnHuman, r?.powerOnHumanRu, r?.powerOnHumanEn)
    const open = expanded?.has?.(letter)
    return `<div class="disk-card" data-drive="${letter}">
      <div class="disk-card-top">
        <div class="disk-card-left">
          <div class="disk-letter">${letter}:</div>
          <div class="disk-model">${escapeHtml(model || t('diskChecking'))}</div>
          <div class="disk-cap-row">
            ${cap ? `<span class="disk-cap">${escapeHtml(cap)}</span>` : ''}
            <span class="disk-media-badge">${escapeHtml(badge)}</span>
            ${pohHuman ? `<span class="disk-poh">${escapeHtml(pohHuman)}</span>` : ''}
          </div>
        </div>
        <div class="disk-card-right">
          <div class="disk-metric">
            <span class="disk-metric-lbl">${t('diskStatus')}</span>
            <span class="disk-status-pill ${stClass}">${escapeHtml(status)}</span>
            ${reason ? `<span class="disk-status-reason">${escapeHtml(reason)}</span>` : ''}
          </div>
          ${temp ? `<div class="disk-metric"><span class="disk-metric-lbl">${t('temp')}</span><span class="disk-metric-val">${temp}</span></div>` : ''}
          ${health ? `<div class="disk-metric"><span class="disk-metric-lbl">${t('diskHealthPct')}</span><span class="disk-metric-val">${health}</span></div>` : ''}
          <div class="disk-metric disk-used">
            <span class="disk-metric-lbl">${t('diskUsed')}</span>
            <div class="disk-used-bar"><i style="width:${usedPct}%"></i></div>
            <span class="disk-metric-val">${freeGB !== '' ? `${freeGB} ${t('diskFreeGB')}` : `${fmt(usedPct, 0)}%`}</span>
          </div>
        </div>
        <button type="button" class="btn soft compact disk-expand-btn" data-drive="${letter}" aria-expanded="${open ? 'true' : 'false'}">
          ${t('diskAllSpecs')} ${chevronSVG ? chevronSVG() : ''}
        </button>
      </div>
      <div class="disk-card-details ${open ? '' : 'hidden'}" ${open ? '' : 'hidden'} data-drive-details="${letter}"></div>
    </div>`
  }).join('')

  return { html, needsAdmin, adminRu, adminEn }
}
