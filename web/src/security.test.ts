// @vitest-environment node
import { ESLint } from 'eslint'
import tseslint from 'typescript-eslint'
import { describe, expect, it } from 'vitest'

// Prova que o lint do projeto rejeita HTML cru (spec 0006, seção 4.7, critério 5). Usa o
// eslint.config.js real; só as regras com tipos são desligadas, porque o arquivo é virtual.
const eslint = new ESLint({
  cwd: new URL('..', import.meta.url).pathname,
  overrideConfig: [{ files: ['**/*.{ts,tsx}'], ...tseslint.configs.disableTypeChecked }],
})

async function messages(code: string, file = 'src/probe.tsx') {
  const [result] = await eslint.lintText(code, { filePath: file })
  return (result?.messages ?? []).map((m) => m.ruleId)
}

describe('lint forbids raw HTML', () => {
  it('rejects dangerouslySetInnerHTML in JSX and in props objects', async () => {
    expect(await messages('export const A = (p: { h: string }) => <div dangerouslySetInnerHTML={{ __html: p.h }} />\n')).toContain(
      'no-restricted-syntax',
    )
    expect(await messages("import { createElement } from 'react'\nexport const B = (h: string) => createElement('div', { dangerouslySetInnerHTML: { __html: h } })\n")).toContain(
      'no-restricted-syntax',
    )
  })

  it('rejects innerHTML, outerHTML, insertAdjacentHTML and eval', async () => {
    expect(await messages("export function f(el: HTMLElement, h: string) { el.innerHTML = h }\n", 'src/probe.ts')).toContain('no-restricted-properties')
    expect(await messages("export function f(el: HTMLElement, h: string) { el.insertAdjacentHTML('beforeend', h) }\n", 'src/probe.ts')).toContain('no-restricted-properties')
    expect(await messages('export const g = (s: string): unknown => eval(s)\n', 'src/probe.ts')).toContain('no-eval')
  })

  it('accepts text rendering', async () => {
    expect(await messages('export const C = (p: { m: string }) => <pre>{p.m}</pre>\n')).toEqual([])
  })
})
