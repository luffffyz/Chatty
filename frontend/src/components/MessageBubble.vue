<script setup lang="ts">
import { computed, ref } from 'vue'
import { Clipboard } from '@wailsio/runtime'
import { splitDocParts } from '../lib/blocks'
import type { DocPart } from '../lib/blocks'
import TypstDoc from './TypstDoc.vue'
import MermaidBlock from './MermaidBlock.vue'
import type { ChatRole } from '../types'

const props = defineProps<{
  role: ChatRole
  content: string
  streaming?: boolean
  failed?: boolean
  thinking?: string
  /** 有数据库 id（已持久化）时显示删除按钮 */
  deletable?: boolean
}>()

const emit = defineEmits<{ delete: [] }>()

const isUser = computed(() => props.role === 'user')
const hasText = computed(() => props.content.trim().length > 0)
/** 消息已结束（非流式）：显示思考折叠块、隐藏行尾光标 */
const whole = computed(() => props.role === 'assistant' && !props.streaming)
/** 分段：每段独立 Typst 排版 / mermaid；未闭合段(流式中)先显示原文 */
const parts = computed<DocPart[]>(() =>
  props.role === 'assistant' ? splitDocParts(props.content, !props.streaming) : [],
)

const copied = ref(false)
async function copyRaw() {
  try {
    await Clipboard.SetText(props.content)
    copied.value = true
    setTimeout(() => (copied.value = false), 1600)
  } catch {
    /* 忽略剪贴板失败 */
  }
}
</script>

<template>
  <div class="msg" :class="isUser ? 'msg--user' : 'msg--assistant'">
    <div class="msg__bubble" :class="{ 'msg__bubble--failed': failed }">
      <button
        v-if="deletable && !streaming"
        class="msg__del"
        title="删除这条消息"
        @click="emit('delete')"
      >
        ✕
      </button>
      <template v-if="isUser">
        <pre class="msg__plain">{{ content }}</pre>
      </template>

      <!-- assistant 尚无正文：思考中/等待/请求失败 -->
      <template v-else-if="!hasText">
        <!-- 有真实 thinking 文本才展示"思考中"；否则低调等待 -->
        <div v-if="streaming && !content" class="msg__wait">
          <template v-if="thinking && thinking.trim()">
            <div class="msg__think-head">思考中</div>
            <div class="msg__think-body">{{ thinking }}</div>
          </template>
          <span v-else class="msg__dots"><i></i><i></i><i></i></span>
        </div>
        <p v-if="failed && !content" class="msg__failed">请求失败，请检查设置或网络。</p>
      </template>

      <!-- assistant 正文：逐段独立排版（text/code → TypstDoc，mermaid → MermaidBlock） -->
      <template v-else>
        <!-- 兜底：分词器异常时原样显示，绝不空泡 -->
        <pre v-if="!parts.length" class="msg__plain"><span>{{ content }}</span><span v-if="streaming" class="msg__cursor">▍</span></pre>

        <template v-for="(p, pi) in parts" :key="p.kind === 'mermaid' ? `m${pi}` : `${pi}:${p.text.slice(0, 32)}`">
          <!-- mermaid 图 -->
          <div v-if="p.kind === 'mermaid'" class="msg__diagram">
            <MermaidBlock v-if="p.stable" :source="p.text" />
            <pre v-else class="msg__plain"><span>{{ p.text }}</span><span v-if="streaming" class="msg__cursor">▍</span></pre>
          </div>

          <!-- text / raw 代码段：闭合段独立排版；未闭合段(流式中)先显示原文 -->
          <div v-else class="msg__seg">
            <TypstDoc v-if="p.stable" :source="p.text" :label="String(pi + 1)" />
            <pre v-else class="msg__plain"><span>{{ p.text }}</span><span v-if="streaming" class="msg__cursor">▍</span></pre>
          </div>
        </template>

        <p v-if="failed" class="msg__failed">请求失败，请检查设置或网络。</p>

        <!-- 完整消息的思考原文折叠保留 -->
        <details v-if="whole && thinking && thinking.trim()" class="msg__think-fold">
          <summary class="msg__think-fold-sum">查看思考过程</summary>
          <div class="msg__think-fold-body">{{ thinking }}</div>
        </details>
      </template>

      <!-- 复制 raw 原文（排查排版用） -->
      <button v-if="!isUser && content && !streaming" class="msg__copy" @click="copyRaw">
        {{ copied ? '已复制' : '复制 raw' }}
      </button>
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
  position: relative;
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
.msg__wait {
  color: var(--text-faint);
  font-size: var(--fs-sm);
  padding: 2px 0;
}
.msg__think-head {
  font-size: var(--fs-xs);
  color: var(--text-faint);
  margin-bottom: 4px;
  letter-spacing: 0.05em;
}
.msg__think-body {
  font-size: var(--fs-xs);
  font-style: italic;
  line-height: 1.5;
  color: var(--text-dim);
  white-space: pre-wrap;
  word-break: break-word;
  max-height: 11em;
  overflow-y: auto;
  border-left: 2px solid var(--border);
  padding-left: 8px;
}
.msg__dots {
  display: inline-flex;
  gap: 4px;
  padding: 4px 0;
}
.msg__dots i {
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: var(--text-faint);
  animation: msg-blink 1.2s infinite ease-in-out;
}
.msg__dots i:nth-child(2) {
  animation-delay: 0.2s;
}
.msg__dots i:nth-child(3) {
  animation-delay: 0.4s;
}
@keyframes msg-blink {
  0%,
  60%,
  100% {
    opacity: 0.25;
  }
  30% {
    opacity: 1;
  }
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
.msg__seg {
  width: 100%;
}
.msg__seg + .msg__seg,
.msg__seg + .msg__diagram,
.msg__diagram + .msg__seg {
  margin-top: 8px;
}
.msg__think-fold {
  margin-top: 10px;
  border-top: 1px dashed var(--border);
  padding-top: 6px;
}
.msg__think-fold-sum {
  font-size: var(--fs-xs);
  color: var(--text-faint);
  cursor: pointer;
  user-select: none;
  letter-spacing: 0.03em;
}
.msg__think-fold-sum:hover {
  color: var(--accent);
}
.msg__think-fold-body {
  margin-top: 6px;
  font-size: var(--fs-xs);
  font-style: italic;
  line-height: 1.5;
  color: var(--text-dim);
  white-space: pre-wrap;
  word-break: break-word;
  max-height: 16em;
  overflow-y: auto;
  border-left: 2px solid var(--border);
  padding-left: 8px;
}
.msg__del {
  position: absolute;
  top: 6px;
  right: 8px;
  border: none;
  background: none;
  color: var(--text-faint);
  font-size: var(--fs-xs);
  line-height: 1;
  cursor: pointer;
  padding: 4px;
  border-radius: 6px;
  opacity: 0;
  transition: opacity 0.15s;
}
.msg:hover .msg__del {
  opacity: 1;
}
.msg__del:hover {
  color: var(--danger);
  background: var(--surface-hover);
}
.msg__copy {
  display: block;
  margin: 6px 0 0 auto;
  border: 1px solid var(--border);
  background: transparent;
  color: var(--text-faint);
  border-radius: 6px;
  padding: 2px 10px;
  font-size: var(--fs-xs);
  cursor: pointer;
  opacity: 0;
  transition: opacity 0.15s;
}
.msg:hover .msg__copy {
  opacity: 1;
}
.msg__copy:hover {
  color: var(--accent);
  border-color: var(--accent);
}
</style>
