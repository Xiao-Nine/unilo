import { computed, ref } from 'vue'
import { acceptHMRUpdate, defineStore } from 'pinia'
import * as dropsApi from '../api/drops'
import type { Drop } from '../api/types'

const tagPattern = /(^|\s)#([\p{L}\p{N}_-]+)/gu

function extractTags(content: string) {
  const tags = new Set<string>()
  for (const match of content.matchAll(tagPattern)) {
    tags.add(match[2])
  }
  return [...tags]
}

export const useDropsStore = defineStore('drops', () => {
  const posts = ref<Drop[]>([])
  const activeTag = ref('')
  const loading = ref(false)
  const publishing = ref(false)
  const liking = ref<Record<string, boolean>>({})
  const commenting = ref<Record<string, boolean>>({})
  const deleting = ref<Record<string, boolean>>({})
  const error = ref('')
  const backendUnavailable = ref(false)

  const tags = computed(() => {
    const values = new Set<string>()
    for (const post of posts.value) {
      for (const tag of extractTags(post.content)) {
        values.add(tag)
      }
    }
    return [...values].sort((a, b) => a.localeCompare(b))
  })

  const filteredPosts = computed(() => {
    if (!activeTag.value) {
      return posts.value
    }
    return posts.value.filter((post) => extractTags(post.content).includes(activeTag.value))
  })

  async function loadDrops() {
    loading.value = true
    error.value = ''
    backendUnavailable.value = false

    try {
      const result = await dropsApi.listDrops()
      posts.value = result.items
    } catch (caught) {
      posts.value = []
      backendUnavailable.value = true
      error.value = caught instanceof Error ? caught.message : 'Drops 加载失败'
    } finally {
      loading.value = false
    }
  }

  function setActiveTag(tag: string) {
    activeTag.value = tag
  }

  async function publishDrop(content: string) {
    const nextContent = content.trim()
    if (!nextContent || publishing.value) {
      return null
    }

    publishing.value = true
    error.value = ''
    try {
      const created = await dropsApi.createDrop(nextContent)
      posts.value = [created, ...posts.value]
      backendUnavailable.value = false
      return created
    } catch (caught) {
      error.value = caught instanceof Error ? caught.message : '发布失败'
      return null
    } finally {
      publishing.value = false
    }
  }

  async function deleteDrop(dropId: string) {
    if (deleting.value[dropId]) {
      return false
    }

    deleting.value = { ...deleting.value, [dropId]: true }
    error.value = ''
    try {
      await dropsApi.deleteDrop(dropId)
      posts.value = posts.value.filter((post) => post.id !== dropId)
      return true
    } catch (caught) {
      error.value = caught instanceof Error ? caught.message : '删除失败'
      return false
    } finally {
      const next = { ...deleting.value }
      delete next[dropId]
      deleting.value = next
    }
  }

  async function createComment(dropId: string, content: string) {
    const nextContent = content.trim()
    if (!nextContent || commenting.value[dropId]) {
      return null
    }

    commenting.value = { ...commenting.value, [dropId]: true }
    error.value = ''
    try {
      const comment = await dropsApi.createDropComment(dropId, nextContent)
      posts.value = posts.value.map((post) =>
        post.id === dropId
          ? {
              ...post,
              comment_count: post.comment_count + 1,
              comments: [...(post.comments ?? []), comment],
            }
          : post,
      )
      return comment
    } catch (caught) {
      error.value = caught instanceof Error ? caught.message : '评论失败'
      return null
    } finally {
      const next = { ...commenting.value }
      delete next[dropId]
      commenting.value = next
    }
  }

  async function toggleLike(dropId: string) {
    if (liking.value[dropId]) {
      return
    }

    liking.value = { ...liking.value, [dropId]: true }
    error.value = ''
    try {
      const result = await dropsApi.toggleDropLike(dropId)
      posts.value = posts.value.map((post) =>
        post.id === dropId
          ? { ...post, like_count: result.current_like_count, is_liked_by_me: result.is_liked }
          : post,
      )
    } catch (caught) {
      error.value = caught instanceof Error ? caught.message : '点赞失败'
    } finally {
      const next = { ...liking.value }
      delete next[dropId]
      liking.value = next
    }
  }

  return {
    posts,
    activeTag,
    loading,
    publishing,
    liking,
    commenting,
    deleting,
    error,
    backendUnavailable,
    tags,
    filteredPosts,
    loadDrops,
    setActiveTag,
    publishDrop,
    deleteDrop,
    createComment,
    toggleLike,
  }
})

if (import.meta.hot) {
  import.meta.hot.accept(acceptHMRUpdate(useDropsStore, import.meta.hot))
}
