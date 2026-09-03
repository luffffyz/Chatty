<script setup lang="ts">
import { nextTick, onMounted, ref, watch } from 'vue'
import MessageBubble from './components/MessageBubble.vue'
import SettingsPanel from './components/SettingsPanel.vue'
import { useChat } from './chat'
import { applyAppearance } from './theme'

const { state, init, send, selectSession, newSession, deleteSession } = useChat()

const input = ref('')
const showSettings = ref(false)
const bodyEl = ref<HTMLElement | null>(null)

async function scrollBottom() {
  await nextTick()
  if (bodyEl.value) bodyEl.value.scrollTop = bodyEl.value.scrollHeight
}

onMounted(async () => {
  await init()
  scrollBottom()
})

// 外观(主题/字号)应用：启动后 settings 载入即生效，保存后立即生效
watch(
  () => state.settings?.appearance,
  (a) => applyAppearance(a),
  { immediate: true },
)

watch(
  () => state.messages.map((m) => m.content + m.streaming).join('|'),
  () => scrollBottom(),
)

function submit() {
  const text = input.value
  input.value = ''
  send(text)
}

function onDelete(id: string) {
  if (confirm('删除这个会话？')) deleteSession(id)
}

function fmtTime(ms: number): string {
  const d = new Date(ms)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}
</script>

<template>
  <div class="app">
    <aside class="side">
      <div class="side__top">
        <span class="side__title">Chatty</span>
        <button class="side__new" title="新对话" @click="newSession">＋</button>
      </div>
      <div class="side__list">
        <button
          v-for="s in state.sessions"
          :key="s.id"
          class="item"
          :class="{ 'item--on': s.id === state.currentId }"
          @click="selectSession(s.id)"
        >
          <span class="item__title">{{ s.title }}</span>
          <span class="item__meta">{{ fmtTime(s.updatedAtMs) }}</span>
          <span class="item__del" title="删除" @click.stop="onDelete(s.id)">🗑</span>
        </button>
      </div>
      <div class="side__bottom">
        <button class="side__settings" @click="showSettings = true">⚙ 设置</button>
      </div>
    </aside>

    <main class="main">
      <div v-if="state.banner" class="banner">{{ state.banner }}</div>
      <div ref="bodyEl" class="msgs">
        <template v-if="state.messages.length">
          <MessageBubble
            v-for="m in state.messages"
            :key="m.key"
            :role="m.role"
            :content="m.content"
            :streaming="m.streaming"
            :failed="m.failed"
          />
        </template>
        <div v-else class="empty">
          <p class="empty__big">开始聊点什么</p>
          <p class="empty__small">公式/排版用 ```typst，图表用 ```mermaid 围栏输出即可</p>
        </div>
      </div>

      <footer class="composer">
        <textarea
          v-model="input"
          rows="1"
          class="composer__input"
          placeholder="输入消息，Enter 发送，Shift+Enter 换行"
          @keydown.enter.exact.prevent="submit"
        ></textarea>
        <button class="composer__send" :disabled="state.busy || !input.trim()" @click="submit">
          {{ state.busy ? '…' : '发送' }}
        </button>
      </footer>
    </main>

    <Teleport to="body">
      <div v-if="showSettings" class="mask" @click.self="showSettings = false">
        <SettingsPanel @close="showSettings = false" />
      </div>
    </Teleport>
  </div>
</template>

<style scoped>
.app {
  display: flex;
  height: 100vh;
}
.side {
  width: 240px;
  flex-shrink: 0;
  background: var(--bg-side);
  border-right: 1px solid var(--border);
  display: flex;
  flex-direction: column;
}
.side__top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px;
}
.side__title {
  font-weight: 700;
  font-size: var(--fs-lg);
  color: var(--text);
}
.side__new {
  width: 28px;
  height: 28px;
  border-radius: 8px;
  border: 1px solid var(--border-strong);
  background: var(--surface);
  color: var(--text);
  cursor: pointer;
  font-size: var(--fs-lg);
  line-height: 1;
}
.side__list {
  flex: 1;
  overflow-y: auto;
  padding: 0 8px;
}
.item {
  position: relative;
  width: 100%;
  text-align: left;
  border: none;
  background: transparent;
  border-radius: 10px;
  padding: 9px 10px;
  margin-bottom: 2px;
  cursor: pointer;
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.item:hover {
  background: var(--surface-hover);
}
.item--on {
  background: var(--accent-weak);
}
.item__title {
  font-size: var(--fs-sm);
  color: var(--text);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  padding-right: 20px;
}
.item__meta {
  font-size: var(--fs-2xs);
  color: var(--text-faint);
}
.item__del {
  position: absolute;
  right: 8px;
  top: 8px;
  font-size: var(--fs-xs);
  opacity: 0;
}
.item:hover .item__del {
  opacity: 0.7;
}
.side__bottom {
  padding: 10px;
}
.side__settings {
  width: 100%;
  padding: 8px;
  border: 1px solid var(--border);
  background: var(--surface);
  color: var(--text);
  border-radius: 8px;
  cursor: pointer;
  font-size: var(--fs-sm);
}
.main {
  flex: 1;
  display: flex;
  flex-direction: column;
  position: relative;
  background: var(--bg);
}
.banner {
  position: absolute;
  top: 12px;
  left: 50%;
  transform: translateX(-50%);
  background: var(--warn-bg);
  color: var(--warn-text);
  border: 1px solid var(--warn-border);
  border-radius: 8px;
  padding: 6px 14px;
  font-size: var(--fs-sm);
  z-index: 10;
  max-width: 70%;
}
.msgs {
  flex: 1;
  overflow-y: auto;
  padding: 20px 22px;
}
.empty {
  text-align: center;
  margin-top: 18vh;
  color: var(--text-faint);
}
.empty__big {
  font-size: var(--fs-xl);
  margin-bottom: 6px;
}
.empty__small {
  font-size: var(--fs-sm);
}
.composer {
  display: flex;
  gap: 10px;
  padding: 12px 18px;
  border-top: 1px solid var(--border);
  background: var(--bg-side);
  align-items: flex-end;
}
.composer__input {
  flex: 1;
  padding: 10px 12px;
  border-radius: 10px;
  border: 1px solid var(--border);
  color: var(--text);
  font-size: var(--fs-md);
  font-family: inherit;
  resize: none;
  max-height: 160px;
  min-height: 40px;
  box-sizing: border-box;
}
.composer__input::placeholder {
  color: var(--text-faint);
}
.composer__send {
  height: 40px;
  padding: 0 20px;
  border: none;
  border-radius: 10px;
  background: var(--accent);
  color: var(--on-accent);
  font-size: var(--fs-md);
  cursor: pointer;
}
.composer__send:disabled {
  opacity: 0.5;
  cursor: default;
}
.mask {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.35);
  z-index: 15;
  display: flex;
  align-items: center;
  justify-content: center;
}
</style>
