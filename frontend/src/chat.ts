// 前端聊天状态：会话/消息/设置 + 流式事件接线。
// 单例模块：App 挂载时调用 init() 一次即可。
import { reactive } from 'vue'
import { Events } from '@wailsio/runtime'
import { ChatService } from '../bindings/chatty'
import type { SessionDTO } from '../bindings/chatty'
import type { Settings } from '../bindings/chatty/internal/config/models'
import type { ChatRole } from './types'

export interface ViewMessage {
  key: string
  role: ChatRole
  content: string
  streaming: boolean
  failed?: boolean
  /** 模型的推理/思考文本（reasoning_content），仅流式期间存在 */
  thinking?: string
}

interface ChatState {
  ready: boolean
  sessions: SessionDTO[]
  currentId: string
  messages: ViewMessage[]
  settings: Settings | null
  busy: boolean
  banner: string
}

const state = reactive<ChatState>({
  ready: false,
  sessions: [],
  currentId: '',
  messages: [],
  settings: null,
  busy: false,
  banner: '',
})

let seq = 0
function nextKey(role: ChatRole): string {
  return `${role === 'user' ? 'u' : 'a'}${++seq}`
}

function streamingMsg(): ViewMessage | undefined {
  for (let i = state.messages.length - 1; i >= 0; i--) {
    const m = state.messages[i]
    if (m.streaming) return m
  }
  return undefined
}

function setBanner(msg: string) {
  state.banner = msg
  if (msg) setTimeout(() => setBanner(''), 6000)
}

async function loadSessions() {
  state.sessions = (await ChatService.GetSessions()) ?? []
  if (state.sessions.length === 0) {
    const s = await ChatService.NewSession()
    if (s) state.sessions = [s]
  }
  if (!state.currentId && state.sessions.length > 0) {
    await selectSession(state.sessions[0].id)
  }
}

async function selectSession(id: string): Promise<void> {
  if (state.busy && id !== state.currentId) {
    setBanner('正在生成中，请稍候再切换会话')
    return
  }
  state.currentId = id
  const msgs = (await ChatService.GetMessages(id)) ?? []
  state.messages = msgs.map((m) => ({
    key: `m${m.id}`,
    role: m.role === 'user' ? 'user' : 'assistant',
    content: m.content,
    streaming: false,
  }))
}

async function newSession() {
  const s = await ChatService.NewSession()
  if (!s) return
  state.sessions.unshift(s)
  await selectSession(s.id)
}

async function deleteSession(id: string) {
  await ChatService.DeleteSession(id)
  state.sessions = state.sessions.filter((s) => s.id !== id)
  if (state.currentId === id) {
    state.messages = []
    state.currentId = ''
    await loadSessions()
  }
}

async function send(text: string): Promise<void> {
  const content = text.trim()
  if (!content || state.busy) return
  if (!state.currentId) {
    const s = await ChatService.NewSession()
    if (s) {
      state.sessions.unshift(s)
      state.currentId = s.id
    }
  }
  if (!state.currentId) return

  state.busy = true
  const um: ViewMessage = { key: nextKey('user'), role: 'user', content, streaming: false }
  state.messages.push(um)
  const placeholder: ViewMessage = {
    key: nextKey('assistant'),
    role: 'assistant',
    content: '',
    streaming: true,
    thinking: '',
  }
  state.messages.push(placeholder)

  try {
    await ChatService.SendMessage(state.currentId, content)
  } catch (e) {
    placeholder.streaming = false
    placeholder.failed = true
    placeholder.content = e instanceof Error ? e.message : String(e)
    state.busy = false
  }
}

function bindEvents() {
  Events.On('chat:delta', (ev) => {
    const d = ev.data
    if (d.sessionId !== state.currentId) return
    const m = streamingMsg()
    if (m) m.content += d.delta
  })

  Events.On('chat:done', (ev) => {
    const d = ev.data
    if (d.sessionId !== state.currentId) return
    const m = streamingMsg()
    if (m) {
      m.streaming = false
      m.content = d.content
      m.thinking = ''
    } else {
      state.messages.push({ key: nextKey('assistant'), role: 'assistant', content: d.content, streaming: false })
    }
    state.busy = false
    loadSessions() // 刷新排序与标题
  })

  Events.On('chat:thinking', (ev) => {
    const d = ev.data
    if (d.sessionId !== state.currentId) return
    const m = streamingMsg()
    if (m && !m.content) {
      m.thinking = (m.thinking ?? '') + d.text
    }
  })

  Events.On('chat:error', (ev) => {
    const d = ev.data
    if (d.sessionId !== state.currentId) return
    const m = streamingMsg()
    if (m) {
      m.streaming = false
      m.failed = true
      if (!m.content) m.content = ''
    }
    state.busy = false
    setBanner(d.error)
  })
}

async function loadSettings() {
  state.settings = await ChatService.GetSettings()
}

async function saveSettings(next: Settings): Promise<void> {
  await ChatService.SaveSettings(next)
  state.settings = next
  setBanner('设置已保存')
}

async function init(): Promise<void> {
  if (state.ready) return
  bindEvents()
  try {
    await loadSettings()
    await loadSessions()
  } finally {
    state.ready = true
  }
}

export function useChat() {
  return {
    state,
    init,
    send,
    selectSession,
    newSession,
    deleteSession,
    saveSettings,
  }
}
