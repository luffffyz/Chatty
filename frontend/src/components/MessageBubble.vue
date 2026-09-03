<script setup lang="ts">
import { computed } from 'vue'
import { splitBlocks } from '../lib/blocks'
import TypstBlock from './TypstBlock.vue'
import MermaidBlock from './MermaidBlock.vue'
import type { ChatRole } from '../types'

const props = defineProps<{
  role: ChatRole
  content: string
  streaming?: boolean
}>()

const isUser = computed(() => props.role === 'user')

// 流式进行中只显示原文；完成后按围栏分块渲染，保证块在稳定后只编译一次。
const blocks = computed(() =>
  props.streaming ? null : splitBlocks(props.content),
)

const renderedContent = computed(() => props.content)
</script>

<template>
  <div class="msg" :class="isUser ? 'msg--user' : 'msg--assistant'">
    <div class="msg__bubble">
      <!-- 流式中 / 纯文本 -->
      <template v-if="!blocks">
        <pre class="msg__stream"><span>{{ renderedContent }}</span><span
          v-if="streaming"
          class="msg__cursor"
        >▍</span></pre>
      </template>
      <template v-else>
        <template v-for="(b, i) in blocks" :key="i">
          <p v-if="b.kind === 'text'" class="msg__text">{{ b.content }}</p>
          <TypstBlock v-else-if="b.kind === 'typst'" :source="b.content" />
          <MermaidBlock v-else-if="b.kind === 'mermaid'" :source="b.content" />
          <pre v-else class="msg__code">{{ b.content }}</pre>
        </template>
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
  max-width: 86%;
  border-radius: 12px;
  padding: 10px 14px;
  background: #fff;
  border: 1px solid #e6e6e6;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.04);
}
.msg--user .msg__bubble {
  background: #eef4ff;
  border-color: #d6e4ff;
}
.msg__text {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-word;
  line-height: 1.6;
}
.msg__stream {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-word;
  line-height: 1.6;
  font-family: inherit;
  font-size: inherit;
}
.msg__cursor {
  animation: blink 1s step-end infinite;
  color: #4a7dff;
}
@keyframes blink {
  50% {
    opacity: 0;
  }
}
.msg__code {
  margin: 6px 0 0;
  background: #f6f8fa;
  border-radius: 6px;
  padding: 8px 10px;
  font-size: 13px;
  line-height: 1.5;
  overflow-x: auto;
  white-space: pre-wrap;
  word-break: break-word;
}
</style>
