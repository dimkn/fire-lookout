import { defineConfigWithVueTs, vueTsConfigs } from '@vue/eslint-config-typescript'
import skipFormatting from '@vue/eslint-config-prettier/skip-formatting'
import pluginVue from 'eslint-plugin-vue'

export default defineConfigWithVueTs(
  {
    name: 'app/files-to-lint',
    files: ['**/*.{ts,mts,vue}'],
  },
  {
    name: 'app/files-to-ignore',
    // src/api/schema.gen.ts is generated from the contract (AGENTS.md §10).
    ignores: ['dist/**', 'coverage/**', 'src/api/schema.gen.ts'],
  },
  pluginVue.configs['flat/essential'],
  vueTsConfigs.recommended,
  skipFormatting,
)
