<script setup lang="ts">
import { computed } from 'vue'
import { splitTypstMessage } from '../lib/blocks'
import TypstDoc from './TypstDoc.vue'
import MermaidBlock from './MermaidBlock.vue'
import type { ChatRole } from '../types'

const props = defineProps<{
  role: ChatRole
  content: string
  streaming?: boolean
  failed?: boolean
}>()

const isUser = computed(() => props.role === 'user')

// assistant 消息 = 一段 Typst 排版 + 若干 mermaid 图（流式完成后再编译）
const seg = computed(() =>
  props.role === 'assistant' && !props.streaming ? splitTypstMessage(props.content) : null,
)
</script>

<template>
  <div class="msg" :class="isUser ? 'msg--user' : 'msg--assistant'">
    <div class="msg__bubble" :class="{ 'msg__bubble--failed': failed }">
      <template v-if="isUser">
        <pre class="msg__plain">{{ content }}</pre>
      </template>

      <!-- 流式中/出错：显示原文 -->
      <template v-else-if="!seg">
        <pre class="msg__plain"><span>{{ content }}</span><span v-if="streaming" class="msg__cursor">▍</span></pre>
        <p v-if="failed && !content" class="msg__failed">请求失败，请检查设置或网络。</p>
      </template>

      <!-- 完整消息：Typst 排版主体 + mermaid 图 -->
      <template v-else>
        <TypstDoc v-if="seg.typst.trim()" :source="seg.typst" />
        <div v-for="(d, i) in seg.diagrams" :key="`m${i}`" class="msg__diagram">
          <MermaidBlock :source="d" />
        </div>
      </template>
    </div>
  </div>
</template>

<style scoped>
.msg {
  display: flex;
  margin-bottom: 12px;
}
.msg--user {
  justify-content: flex-end;
}
.msg--assistant {
  justify-content: flex-start;
}
.msg__bubble {
  max-width: 92%;
  min-width: 0;
  border-radius: 12px;
  padding: 10px 14px;
  background: var(--surface);
  border: 1px solid var(--border);
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.04);
}
.msg--user .msg__bubble {
  background: var(--accent-weak);
  border-color: var(--accent);
}
.msg__bubble--failed {
  border-color: var(--danger);
}
.msg__plain {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-word;
  line-height: 1.6;
  font-family: inherit;
  font-size: inherit;
  color: var(--text);
}
.msg__cursor {
  animation: blink 1s step-end infinite;
  color: var(--accent);
}
@keyframes blink {
  50% {
    opacity: 0;
  }
}
.msg__failed {
  color: var(--danger);
  font-size: var(--fs-sm);
  margin: 6px 0 0;
}
.msg__diagram {
  margin-top: 10px;
}
</style>
