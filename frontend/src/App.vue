<script setup lang="ts">
import MessageBubble from './components/MessageBubble.vue'
import type { ChatMessage } from './types'

// 演示消息：验证 文本 / Typst 排版 / Mermaid 图 / 流式 四类渲染。
// 后续接入 ChatService 后由真实会话数据替代。
const messages = reactiveDemo()

function reactiveDemo(): ChatMessage[] {
  return [
    {
      id: 'u1',
      role: 'user',
      content: '解释欧拉恒等式，公式用 Typst，关系画成 mermaid 图。',
    },
    {
      id: 'a1',
      role: 'assistant',
      content:
        '欧拉恒等式把五个最重要的数学常数联系起来：\n\n```typst\n#set page(width: auto, height: auto, margin: 12pt, fill: white)\n$ e^(i pi) + 1 = 0 $\n```\n\n```mermaid\nflowchart LR\n  A["复数单位圆"] -->|"e^(iθ) = cosθ + i·sinθ"| B["θ = π"]\n  B --> C["e^(iπ) = -1"]\n  C --> D["+1 移到右边"]\n  D --> E["e^(iπ) + 1 = 0"]\n```\n\n一句话：绕单位圆转半圈（π 弧度）恰好落在 `-1` 上。',
    },
  ]
}
</script>

<template>
  <div class="chat">
    <header class="chat__header">Chatty</header>
    <main class="chat__body">
      <MessageBubble
        v-for="m in messages"
        :key="m.id"
        :role="m.role"
        :content="m.content"
        :streaming="m.streaming"
      />
    </main>
    <footer class="chat__footer">
      <input
        class="chat__input"
        type="text"
        placeholder="输入消息…（对话后端接线中）"
        disabled
      />
    </footer>
  </div>
</template>

<style scoped>
.chat {
  display: flex;
  flex-direction: column;
  height: 100vh;
}
.chat__header {
  padding: 12px 18px;
  font-weight: 600;
  border-bottom: 1px solid #eee;
  background: #fafafa;
}
.chat__body {
  flex: 1;
  overflow-y: auto;
  padding: 16px 18px;
}
.chat__footer {
  padding: 10px 18px;
  border-top: 1px solid #eee;
  background: #fafafa;
}
.chat__input {
  width: 100%;
  padding: 10px 14px;
  border-radius: 10px;
  border: 1px solid #ddd;
  box-sizing: border-box;
  font-size: 14px;
}
</style>
