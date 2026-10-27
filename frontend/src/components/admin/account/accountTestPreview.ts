import DOMPurify from 'dompurify'

const MAX_PREVIEW_CHARS = 256 * 1024
const PREVIEW_CSP = "default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; img-src data: blob:; media-src data: blob:; font-src data:; connect-src 'none'; frame-src 'none'; object-src 'none'; base-uri 'none'; form-action 'none'; navigate-to 'none'"
const SVG_PREVIEW_CSP = "default-src 'none'; script-src 'none'; style-src 'unsafe-inline'; img-src data: blob:; font-src data:; connect-src 'none'; frame-src 'none'; object-src 'none'; base-uri 'none'; form-action 'none'; navigate-to 'none'"

export interface PelicanPreview {
  html: string
  svg: string
  format: 'html' | 'svg'
  animated: boolean
}

const wrapSvgPreview = (svg: string) => `<!doctype html><html><head><meta http-equiv="Content-Security-Policy" content="${SVG_PREVIEW_CSP}"><meta name="viewport" content="width=device-width,initial-scale=1"></head><body style="margin:0;min-height:100vh;display:grid;place-items:center;overflow:hidden"><div style="width:100%;height:100vh;display:grid;place-items:center;overflow:hidden">${svg}</div></body></html>`

const unwrapMarkdownFence = (value: string) => value
  .trim()
  .replace(/^```(?:html?|svg|xml)?\s*/i, '')
  .replace(/\s*```\s*$/, '')
  .trim()

const hasAnimation = (source: string) => /<(?:animate|animateTransform|animateMotion|set)\b|@keyframes|\banimation(?:-name)?\s*:|requestAnimationFrame\s*\(|\bset(?:Interval|Timeout)\s*\(/i.test(source)

export function buildPelicanPreview(output: string): PelicanPreview | null {
  if (output.length > MAX_PREVIEW_CHARS) return null
  const source = unwrapMarkdownFence(output)
  if (!source) return null

  const startsWithSvg = /^\s*(?:<\?xml[^>]*>\s*)?<svg\b/i.test(source)
  if (!startsWithSvg && /<(?:!doctype\s+html|html|head|body|style|script|main|div)\b/i.test(source)) {
    const document = new DOMParser().parseFromString(source, 'text/html')
    if (!document.querySelector('svg')) return null
    document.querySelectorAll('meta[http-equiv],base,iframe,frame,object,embed,form').forEach(node => node.remove())
    const policy = document.createElement('meta')
    policy.httpEquiv = 'Content-Security-Policy'
    policy.content = PREVIEW_CSP
    document.head.prepend(policy)
    const viewport = document.createElement('meta')
    viewport.name = 'viewport'
    viewport.content = 'width=device-width,initial-scale=1'
    document.head.append(viewport)
    const responsiveStyle = document.createElement('style')
    responsiveStyle.dataset.previewLayout = 'fit'
    responsiveStyle.textContent = `
      html,body{width:100%!important;height:100%!important;min-width:0!important;min-height:0!important;margin:0!important;overflow:hidden!important}
      body{display:grid!important;place-items:center!important}
      body>:not(script):not(style){max-width:100vw!important;max-height:100vh!important;box-sizing:border-box!important}
      svg,canvas,video,img{max-width:100%!important;max-height:100vh!important;object-fit:contain!important}
    `
    document.head.append(responsiveStyle)
    return {
      html: `<!doctype html>${document.documentElement.outerHTML}`,
      svg: '',
      format: 'html',
      animated: hasAnimation(source)
    }
  }

  const start = source.indexOf('<svg')
  const end = source.lastIndexOf('</svg>')
  if (start < 0 || end < start) return null
  const sanitized = DOMPurify.sanitize(source.slice(start, end + 6), {
    USE_PROFILES: { svg: true, svgFilters: true },
    ADD_TAGS: ['animate', 'animateTransform', 'animateMotion', 'set'],
    ADD_ATTR: ['attributeName', 'from', 'to', 'by', 'values', 'dur', 'begin', 'end', 'repeatCount', 'repeatDur', 'keyTimes', 'keySplines', 'calcMode', 'additive', 'accumulate'],
    FORBID_TAGS: ['script', 'foreignObject', 'iframe', 'image', 'object', 'embed']
  })
  if (!sanitized.startsWith('<svg')) return null

  const document = new DOMParser().parseFromString(sanitized, 'image/svg+xml')
  if (document.querySelector('parsererror') || document.documentElement.localName !== 'svg') return null
  const root = document.documentElement as unknown as SVGSVGElement
  if (!root.getAttribute('viewBox')?.trim()) {
    const width = root.getAttribute('width')?.match(/^(\d+(?:\.\d+)?)(?:px)?$/)?.[1]
    const height = root.getAttribute('height')?.match(/^(\d+(?:\.\d+)?)(?:px)?$/)?.[1]
    if (width && height && Number(width) > 0 && Number(height) > 0) {
      root.setAttribute('viewBox', `0 0 ${width} ${height}`)
    }
  }
  root.style.setProperty('display', 'block', 'important')
  root.style.setProperty('max-width', '100%', 'important')
  root.style.setProperty('max-height', '100%', 'important')
  root.style.setProperty('object-fit', 'contain', 'important')
  root.style.setProperty('width', root.getAttribute('viewBox') ? '100%' : 'auto', 'important')
  root.style.setProperty('height', root.getAttribute('viewBox') ? '100%' : 'auto', 'important')

  const svg = root.outerHTML
  return { html: wrapSvgPreview(svg), svg, format: 'svg', animated: hasAnimation(svg) }
}
