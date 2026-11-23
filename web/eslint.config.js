import reactHooks from 'eslint-plugin-react-hooks'
import { defineConfig } from 'eslint/config'
import tseslint from 'typescript-eslint'

// Texto de log e payload são entrada não confiável: nada de HTML cru (spec 0006, seção 4.7).
const rawHtmlMessage = 'Render log data as text; raw HTML is forbidden (spec 0006, section 4.7).'

export default defineConfig(
  { ignores: ['node_modules/'] },
  tseslint.configs.recommendedTypeChecked,
  reactHooks.configs.flat.recommended,
  {
    languageOptions: {
      parserOptions: {
        projectService: true,
        tsconfigRootDir: import.meta.dirname,
      },
    },
    rules: {
      'no-restricted-syntax': [
        'error',
        { selector: "JSXAttribute[name.name='dangerouslySetInnerHTML']", message: rawHtmlMessage },
        { selector: "Property[key.name='dangerouslySetInnerHTML']", message: rawHtmlMessage },
      ],
      'no-restricted-properties': [
        'error',
        { property: 'innerHTML', message: rawHtmlMessage },
        { property: 'outerHTML', message: rawHtmlMessage },
        { property: 'insertAdjacentHTML', message: rawHtmlMessage },
      ],
      'no-eval': 'error',
      'no-implied-eval': 'off',
      '@typescript-eslint/no-implied-eval': 'error',
      'no-new-func': 'error',
    },
  },
  // O próprio config é JS fora do tsconfig: sem regras que dependem de tipos.
  { files: ['eslint.config.js'], extends: [tseslint.configs.disableTypeChecked] },
)
