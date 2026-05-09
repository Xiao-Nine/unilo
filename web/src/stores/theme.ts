import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

const STORAGE_KEY = 'unilo.theme'
type ThemeMode = 'dark' | 'light'

function readInitialTheme(): ThemeMode {
  const saved = localStorage.getItem(STORAGE_KEY)
  if (saved === 'dark' || saved === 'light') {
    return saved
  }
  return window.matchMedia('(prefers-color-scheme: light)').matches ? 'light' : 'dark'
}

export const useThemeStore = defineStore('theme', () => {
  const mode = ref<ThemeMode>(readInitialTheme())
  const label = computed(() => (mode.value === 'dark' ? '暗色' : '亮色'))

  function apply() {
    document.documentElement.dataset.theme = mode.value
    localStorage.setItem(STORAGE_KEY, mode.value)
  }

  function toggle() {
    mode.value = mode.value === 'dark' ? 'light' : 'dark'
    apply()
  }

  apply()

  return { mode, label, toggle, apply }
})
