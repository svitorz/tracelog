import type { Severity } from '../api/types'

// A severidade é sempre escrita; a cor só reforça (spec 0006, seção 4.8). Pares de cor
// escolhidos para contraste AA (conferido na fase 5, PROGRESS.md).
const styles: Record<Severity, string> = {
  debug: 'bg-slate-100 text-slate-800',
  info: 'bg-sky-100 text-sky-900',
  warn: 'bg-amber-100 text-amber-900',
  error: 'bg-red-100 text-red-900',
  fatal: 'bg-red-800 text-white',
}

export function SeverityBadge({ severity }: { severity: string }) {
  const style = styles[severity as Severity] ?? 'bg-slate-100 text-slate-800'
  return <span className={`inline-block rounded px-2 py-0.5 text-xs font-semibold uppercase ${style}`}>{severity}</span>
}
