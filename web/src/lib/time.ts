// A UI mostra todos os horários em UTC (spec 0006, seção 4.5).

/** Formats an RFC3339 timestamp as "YYYY-MM-DD HH:MM:SS UTC"; invalid input is returned as is. */
export function formatUTC(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return `${date.toISOString().slice(0, 19).replace('T', ' ')} UTC`
}

const dateOnly = /^\d{4}-\d{2}-\d{2}$/

export function isDateOnly(value: string | undefined): boolean {
  return value !== undefined && dateOnly.test(value)
}
