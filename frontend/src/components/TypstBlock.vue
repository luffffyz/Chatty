<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { renderTypst } from '../lib/typst'

const props = defineProps<{ source: string }>()

const svg = ref('')
const error = ref('')
const pending = ref(false)

async function compile() {
  if (svg.value || pending.value) return
  pending.value = true
  error.value = ''
  try {
    svg.value = await renderTypst(props.source)
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    pending.value = false
  }
}

onMounted(compile)
watch(() => props.source, compile)
</script>

<template>
  <div class="typst-block">
    <div v-if="pending" class="typst-block__status">排版中…</div>
    <div v-else-if="error" class="typst-block__error">
      <div class="typst-block__error-msg">Typst 编译失败：{{ error }}</div>
      <pre class="typst-block__raw">{{ source }}</pre>
    </div>
    <div v-else v-html="svg" class="typst-block__svg"></div>
  </div>
</template>

<style scoped>
.typst-block {
  overflow-x: auto;
}
.typst-block__svg {
  display: inline-block;
  background: var(--paper);
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 6px 10px;
  line-height: 0;
}
.typst-block__svg :deep(svg) {
  max-width: 100%;
  height: auto;
}
.typst-block__status {
  color: var(--text-faint);
  font-size: var(--fs-xs);
  padding: 4px 0;
}
.typst-block__error {
  color: var(--danger);
}
.typst-block__error-msg {
  font-size: var(--fs-xs);
  margin-bottom: 4px;
}
.typst-block__raw {
  background: var(--danger-bg);
  border-radius: 6px;
  padding: 8px;
  font-size: var(--fs-xs);
  white-space: pre-wrap;
  margin: 0;
}
</style>
