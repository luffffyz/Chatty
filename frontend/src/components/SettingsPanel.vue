<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import type { Settings } from '../../bindings/chatty/internal/config/models'
import { useChat } from '../chat'

const { state, saveSettings } = useChat()
const saving = ref(false)
const errMsg = ref('')

onMounted(syncFromSettings)

// 简化版设置：MVP 只维护一个 active provider。
// 常用 OpenAI-compatible 端点预设。
const PRESETS = [
  { label: 'OpenRouter', baseURL: 'https://openrouter.ai/api/v1', example: 'openai/gpt-4o-mini' },
  { label: 'DeepSeek', baseURL: 'https://api.deepseek.com/v1', example: 'deepseek-chat' },
  { label: 'Ollama (本地)', baseURL: 'http://localhost:11434/v1', example: 'llama3.2' },
]

const form = reactive({
  label: 'OpenRouter',
  baseURL: PRESETS[0].baseURL,
  apiKey: '',
  model: PRESETS[0].example,
  systemPrompt: '',
})

function syncFromSettings() {
  const s = state.settings
  if (!s) return
  const p = s.providers && s.providers.length > 0 ? s.providers[0] : null
  form.label = p?.Label ?? 'OpenRouter'
  form.baseURL = p?.BaseURL ?? PRESETS[0].baseURL
  form.apiKey = p?.APIKey ?? ''
  form.model = s.activeModel || PRESETS[0].example
  form.systemPrompt = s.systemPrompt
}

function applyPreset(i: number) {
  const p = PRESETS[i]
  form.label = p.label
  form.baseURL = p.baseURL
  if (!form.model || form.baseURL !== p.baseURL) form.model = p.example
}

async function submit() {
  errMsg.value = ''
  if (!form.baseURL.trim()) {
    errMsg.value = 'Base URL 不能为空'
    return
  }
  const next: Settings = {
    activeProviderId: 'default',
    activeModel: form.model.trim(),
    systemPrompt: form.systemPrompt,
    providers: [
      {
        ID: 'default',
        Label: form.label.trim() || 'default',
        BaseURL: form.baseURL.trim(),
        APIKey: form.apiKey.trim(),
      },
    ],
  }
  saving.value = true
  try {
    await saveSettings(next)
  } catch (e) {
    errMsg.value = e instanceof Error ? e.message : String(e)
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="panel">
    <header class="panel__head">
      <h2>设置</h2>
      <button class="panel__close" @click="$emit('close')">✕</button>
    </header>

    <div class="panel__body">
      <p v-if="errMsg" class="panel__err">{{ errMsg }}</p>

      <label class="field">
        <span>服务</span>
        <div class="presets">
          <button
            v-for="(p, i) in PRESETS"
            :key="p.label"
            type="button"
            class="chip"
            :class="{ 'chip--on': form.baseURL === p.baseURL }"
            @click="applyPreset(i)"
          >
            {{ p.label }}
          </button>
        </div>
      </label>

      <label class="field">
        <span>Base URL</span>
        <input v-model="form.baseURL" type="text" placeholder="https://…/v1" />
      </label>

      <label class="field">
        <span>API Key</span>
        <input v-model="form.apiKey" type="password" placeholder="sk-…（本地 Ollama 可留空）" />
      </label>

      <label class="field">
        <span>模型</span>
        <input v-model="form.model" type="text" placeholder="openai/gpt-4o-mini" />
      </label>

      <label class="field">
        <span>系统提示（Typst 输出约定）</span>
        <textarea v-model="form.systemPrompt" rows="7"></textarea>
      </label>

      <p class="hint">API Key 明文保存在本地 settings.json，仅用于直连你配置的端点。</p>
    </div>

    <footer class="panel__foot">
      <button class="btn" :disabled="saving" @click="submit">保存</button>
      <button class="btn btn--ghost" @click="$emit('close')">取消</button>
    </footer>
  </div>
</template>

<script lang="ts">
export default { emits: ['close'] }
</script>

<style scoped>
.panel {
  position: fixed;
  inset: 0;
  margin: auto;
  width: min(560px, calc(100vw - 48px));
  height: min(680px, calc(100vh - 48px));
  background: #fff;
  border-radius: 14px;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.25);
  display: flex;
  flex-direction: column;
  z-index: 20;
}
.panel__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 18px;
  border-bottom: 1px solid #eee;
}
.panel__head h2 {
  margin: 0;
  font-size: 16px;
}
.panel__close {
  border: none;
  background: none;
  font-size: 16px;
  cursor: pointer;
  color: #666;
}
.panel__body {
  flex: 1;
  overflow-y: auto;
  padding: 16px 18px;
}
.panel__foot {
  display: flex;
  gap: 10px;
  padding: 12px 18px;
  border-top: 1px solid #eee;
  justify-content: flex-end;
}
.panel__err {
  color: #b42318;
  font-size: 13px;
}
.field {
  display: block;
  margin-bottom: 14px;
}
.field > span {
  display: block;
  font-size: 13px;
  color: #555;
  margin-bottom: 5px;
}
.field input,
.field textarea {
  width: 100%;
  box-sizing: border-box;
  padding: 8px 10px;
  border: 1px solid #ddd;
  border-radius: 8px;
  font-size: 13px;
  font-family: inherit;
  resize: vertical;
}
.presets {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}
.chip {
  border: 1px solid #ddd;
  background: #fff;
  border-radius: 999px;
  padding: 5px 12px;
  font-size: 12px;
  cursor: pointer;
}
.chip--on {
  border-color: #4a7dff;
  background: #eef4ff;
  color: #2b5cd9;
}
.hint {
  font-size: 12px;
  color: #999;
  margin-top: 4px;
}
.btn {
  padding: 8px 18px;
  border-radius: 8px;
  border: none;
  background: #4a7dff;
  color: #fff;
  cursor: pointer;
  font-size: 14px;
}
.btn:disabled {
  opacity: 0.6;
}
.btn--ghost {
  background: #f0f0f0;
  color: #333;
}
</style>
