<script setup lang="ts">
import DOMPurify from 'dompurify'
import { marked } from 'marked'
import { nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { fetchAuthorizedBlob } from '../api/client'

const props = defineProps<{
  content: string
}>()

const emit = defineEmits<{
  previewImage: [src: string, alt: string]
}>()

const markdownRoot = ref<HTMLElement | null>(null)
const html = ref('')
let objectUrls: string[] = []
let renderVersion = 0

function revokeObjectUrls() {
  for (const url of objectUrls) {
    URL.revokeObjectURL(url)
  }
  objectUrls = []
}

function isWorkspacePreviewUrl(value: string) {
  try {
    const url = new URL(value, window.location.origin)
    return (
      (url.pathname.startsWith('/api/v1/workspace/files/') || url.pathname.startsWith('/workspace/files/')) &&
      url.pathname.endsWith('/preview')
    )
  } catch {
    return false
  }
}

function isSafeDirectImageUrl(value: string) {
  try {
    const url = new URL(value, window.location.origin)
    return url.protocol === 'http:' || url.protocol === 'https:' || url.protocol === 'blob:'
  } catch {
    return false
  }
}

function hardenHtml(markup: string) {
  const doc = new DOMParser().parseFromString(`<div>${markup}</div>`, 'text/html')
  const container = doc.body.firstElementChild
  if (!container) {
    return ''
  }

  for (const link of container.querySelectorAll('a')) {
    link.setAttribute('target', '_blank')
    link.setAttribute('rel', 'noopener noreferrer')
  }

  for (const image of container.querySelectorAll('img')) {
    const source = image.getAttribute('src') ?? ''
    if (isWorkspacePreviewUrl(source)) {
      image.setAttribute('data-auth-src', source)
      image.removeAttribute('src')
    } else if (!isSafeDirectImageUrl(source)) {
      image.remove()
    }
  }

  for (const pre of container.querySelectorAll('pre')) {
    const button = doc.createElement('button')
    button.type = 'button'
    button.className = 'copy-code-button'
    button.type = 'button'
    button.setAttribute('aria-label', '复制代码')
    button.innerHTML = '<span class="copy-code-icon" aria-hidden="true"></span>'
    pre.prepend(button)
  }

  return container.innerHTML
}

async function loadAuthorizedImages(version: number) {
  await nextTick()
  if (version !== renderVersion || !markdownRoot.value) {
    return
  }

  const images = [...markdownRoot.value.querySelectorAll<HTMLImageElement>('img[data-auth-src]')]
  for (const image of images) {
    const source = image.dataset.authSrc
    if (!source) {
      continue
    }

    try {
      const blob = await fetchAuthorizedBlob(source)
      if (version !== renderVersion) {
        return
      }
      const objectUrl = URL.createObjectURL(blob)
      objectUrls.push(objectUrl)
      image.src = objectUrl
      image.dataset.previewSrc = objectUrl
    } catch {
      image.classList.add('image-load-failed')
      image.alt = image.alt || '图片加载失败'
    }
  }
}

async function handleClick(event: MouseEvent) {
  const target = event.target
  if (!(target instanceof Element)) {
    return
  }

  const copyButton = target.closest<HTMLButtonElement>('.copy-code-button')
  if (copyButton) {
    const code = copyButton.closest('pre')?.querySelector('code')?.textContent ?? ''
    if (code) {
      await navigator.clipboard.writeText(code)
      copyButton.classList.add('copied')
      copyButton.setAttribute('aria-label', '已复制')
      window.setTimeout(() => {
        copyButton.classList.remove('copied')
        copyButton.setAttribute('aria-label', '复制代码')
      }, 1200)
    }
    return
  }

  if (!(target instanceof HTMLImageElement)) {
    return
  }

  const source = target.dataset.previewSrc || target.src
  if (source) {
    emit('previewImage', source, target.alt)
  }
}

function renderMarkdown() {
  renderVersion += 1
  const version = renderVersion
  revokeObjectUrls()

  const parsed = marked.parse(props.content, {
    async: false,
    breaks: true,
    gfm: true,
  }) as string
  const sanitized = DOMPurify.sanitize(parsed)
  html.value = hardenHtml(sanitized)
  void loadAuthorizedImages(version)
}

watch(() => props.content, renderMarkdown, { immediate: true })

onBeforeUnmount(() => {
  renderVersion += 1
  revokeObjectUrls()
})
</script>

<template>
  <div ref="markdownRoot" class="markdown-message" @click="handleClick" v-html="html"></div>
</template>
