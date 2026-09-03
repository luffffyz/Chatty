// 前端共享的消息类型。与 Go 侧 internal/chat.Message 对应。
export type ChatRole = 'user' | 'assistant'

export interface ChatMessage {
  id: string
  role: ChatRole
  content: string
  /** 正在流式生成中：内容未完成，不触发排版/图渲染 */
  streaming?: boolean
}
