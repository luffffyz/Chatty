<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { renderMermaid } from '../lib/mermaid'

const props = defineProps<{ source: string }>()

const svg = ref('')
const error = ref('')
const pending = ref(false)

async function render() {
  if (svg.value || pending.value) return
  pending.value = true
  error.value = ''
  try {
    svg.value = await renderMermaid(props.source)
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    pending.value = false
  }
}

onMounted(render)
watch(() => props.source, render)
</script>

<template>
  <div class="mermaid-block">
    <div v-if="pending" class="mermaid-block__status">渲染图中…</div>
    <div v-else-if="error" class="mermaid-block__error">
      <div class="mermaid-block__error-msg">Mermaid 渲染失败：{{ error }}</div>
      <pre class="mermaid-block__raw">{{ source }}</pre>
    </div>
    <div v-else v-html="svg" class="mermaid-block__svg"></div>
  </div>
</template>

<style scoped>
.mermaid-block {
  overflow-x: auto;
}
.mermaid-block__svg {
  display: flex;
  justify-content: center;
  background: white;
  border-radius: 6px;
  padding: 8px;
}
.mermaid-block__svg :deep(svg) {
  max-width: 100%;
  height: auto;
}
.mermaid-block__status {
  color: #888;
  font-size: 12px;
  padding: 4px 0;
}
.mermaid-block__error {
  color: #b42318;
}
.mermaid-block__error-msg {
  font-size: 12px;
  margin-bottom: 4px;
}
.mermaid-block__raw {
  background: #fdf2f2;
  border-radius: 6px;
  padding: 8px;
  font-size: 12px;
  white-space: pre-wrap;
  margin: 0;
}
</style>
