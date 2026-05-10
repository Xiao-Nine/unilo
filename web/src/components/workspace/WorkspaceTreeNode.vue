<script setup lang="ts">
import { ref } from 'vue'
import type { WorkspaceFile } from '../../api/types'

const props = defineProps<{
  item: WorkspaceFile
  childrenByParentId: Record<string, WorkspaceFile[]>
  expandedFolderIds: Record<string, boolean>
  selectedItemId: string
  loadingFolders: Record<string, boolean>
  depth?: number
}>()

const emit = defineEmits<{
  select: [item: WorkspaceFile]
  toggle: [item: WorkspaceFile]
  menu: [item: WorkspaceFile, event: MouseEvent]
  dropFiles: [item: WorkspaceFile, files: File[]]
}>()

const isDragOver = ref(false)
let longPressTimer: number | undefined
let suppressNextClick = false

function clearLongPressTimer() {
  if (longPressTimer !== undefined) {
    window.clearTimeout(longPressTimer)
    longPressTimer = undefined
  }
}

function handlePointerDown(event: PointerEvent) {
  if (!props.item.is_folder || event.pointerType === 'mouse') {
    return
  }
  clearLongPressTimer()
  longPressTimer = window.setTimeout(() => {
    suppressNextClick = true
    emit('menu', props.item, event)
  }, 550)
}

function handleClick() {
  if (suppressNextClick) {
    suppressNextClick = false
    return
  }
  if (props.item.is_folder) {
    emit('toggle', props.item)
    return
  }
  emit('select', props.item)
}

function handleDragEnter(event: DragEvent) {
  if (!props.item.is_folder) {
    return
  }
  event.preventDefault()
  event.stopPropagation()
  isDragOver.value = true
}

function handleDragover(event: DragEvent) {
  if (!props.item.is_folder) {
    return
  }
  event.preventDefault()
  event.stopPropagation()
  isDragOver.value = true
}

function handleDragLeave() {
  isDragOver.value = false
}

function handleDrop(event: DragEvent) {
  if (!props.item.is_folder) {
    return
  }
  event.preventDefault()
  event.stopPropagation()
  isDragOver.value = false
  emit('dropFiles', props.item, [...(event.dataTransfer?.files ?? [])])
}
</script>

<template>
  <div class="workspace-tree-node">
    <button
      :class="['workspace-tree-row', { selected: selectedItemId === item.id, 'drop-target': item.is_folder && isDragOver }]"
      type="button"
      :style="{ paddingLeft: `${12 + (depth ?? 0) * 14}px` }"
      @click="handleClick"
      @contextmenu.prevent="emit('menu', item, $event)"
      @pointerdown="handlePointerDown"
      @pointerup="clearLongPressTimer"
      @pointercancel="clearLongPressTimer"
      @pointerleave="clearLongPressTimer"
      @dragenter="handleDragEnter"
      @dragover="handleDragover"
      @dragleave="handleDragLeave"
      @drop="handleDrop"
    >
      <span class="workspace-node-icon">
        <template v-if="item.is_folder">{{ expandedFolderIds[item.id] ? '▾' : '▸' }}</template>
        <template v-else>•</template>
      </span>
      <span class="workspace-node-name">{{ item.name }}</span>
    </button>
    <p v-if="item.is_folder && loadingFolders[item.id]" class="muted compact workspace-loading-row">加载中...</p>
    <div v-if="item.is_folder && expandedFolderIds[item.id]" class="workspace-tree-children">
      <WorkspaceTreeNode
        v-for="child in childrenByParentId[item.id] ?? []"
        :key="child.id"
        :item="child"
        :children-by-parent-id="childrenByParentId"
        :expanded-folder-ids="expandedFolderIds"
        :selected-item-id="selectedItemId"
        :loading-folders="loadingFolders"
        :depth="(depth ?? 0) + 1"
        @select="(child) => emit('select', child)"
        @toggle="(child) => emit('toggle', child)"
        @menu="(child, event) => emit('menu', child, event)"
        @drop-files="(child, files) => emit('dropFiles', child, files)"
      />
    </div>
  </div>
</template>
