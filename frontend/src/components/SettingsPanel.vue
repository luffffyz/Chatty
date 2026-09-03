<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import type { Settings } from '../../bindings/chatty/internal/config/models'
import { ChatService } from '../../bindings/chatty'
import { useChat } from '../chat'

const emit = defineEmits<{ close: [] }>()

const { state, saveSettings } = useChat()

const tab = ref<'provider' | 'appearance'>('provider')
const saving = ref(false)
const errMsg = ref('')

// ---------- 提供商表单 ----------
const PRESETS = [
  { label: 'OpenRouter', baseURL: 'https://openrouter.ai/api/v1', example: 'openai/gpt-4o-mini' },
  { label: 'DeepSeek', baseURL: 'https://api.deepseek.com/v1', example: 'deepseek-chat' },
  { label: 'Ollama (本地)', baseURL: 'http://localhost:11434/v1', example: 'llama3.2' },
]

const pform = reactive({
  label: 'OpenRouter',
  baseURL: PRESETS[0].baseURL,
  apiKey: '',
  model: PRESETS[0].example,
  systemPrompt: '',
  models: [] as string[],
})

// ---------- 提供商: 模型扫描 ----------
const scanning = ref(false)
const scanMsg = ref('')

async function scanModels() {
  scanMsg.value = ''
  if (!pform.baseURL.trim()) {
    scanMsg.value = '请先填写 Base URL'
    return
  }
  scanning.value = true
  try {
    const models = (await ChatService.ListModels(pform.baseURL.trim(), pform.apiKey.trim())) ?? []
    pform.models = models
    // 自动选中扫描到的第一个（或与当前输入匹配的）
    if (!models.includes(pform.model)) {
      pform.model = models[0] ?? ''
    }
    scanMsg.value = `扫描到 ${models.length} 个模型`
  } catch (e) {
    scanMsg.value = e instanceof Error ? e.message : String(e)
  } finally {
    scanning.value = false
  }
}

// ---------- 外观表单 ----------
const THEMES = [
  { id: 'light', label: '浅色' },
  { id: 'dark', label: '暗色' },
  { id: 'system', label: '跟随系统' },
]
const MIN_SIZE = 12
const MAX_SIZE = 22
const aform = reactive({ theme: 'system', fontSize: 14, chartBg: '' })

function cloneBase(): Settings {
  const cur = state.settings
  return {
    providers: cur?.providers ?? [],
    activeProviderId: cur?.activeProviderId ?? '',
    activeModel: cur?.activeModel ?? '',
    systemPrompt: cur?.systemPrompt ?? '',
    appearance: {
      theme: cur?.appearance?.theme ?? 'system',
      fontSize: cur?.appearance?.fontSize ?? 14,
      chartBg: cur?.appearance?.chartBg ?? '',
    },
  }
}

function syncFromSettings() {
  const cur = state.settings
  if (!cur) return

  const p = cur.providers && cur.providers.length > 0 ? cur.providers[0] : null
  if (p) {
    pform.label = p.Label ?? 'OpenRouter'
    pform.baseURL = p.BaseURL ?? PRESETS[0].baseURL
    pform.apiKey = p.APIKey ?? ''
    pform.model = cur.activeModel || PRESETS[0].example
    pform.systemPrompt = cur.systemPrompt
    pform.models = p.Models ?? []
  }
  aform.theme = cur.appearance?.theme ?? 'system'
  aform.fontSize = cur.appearance?.fontSize ?? 14
  aform.chartBg = cur.appearance?.chartBg ?? ''
}

function applyPreset(i: number) {
  const p = PRESETS[i]
  // 切换到不同提供商预设：清空 API Key / 模型 / 扫描结果，避免串配置
  if (pform.baseURL !== p.baseURL || pform.label !== p.label) {
    pform.label = p.label
    pform.baseURL = p.baseURL
    pform.apiKey = ''
    pform.model = ''
    pform.models = []
    scanMsg.value = ''
  }
}

async function saveProvider() {
  errMsg.value = ''
  if (!pform.baseURL.trim()) {
    errMsg.value = 'Base URL 不能为空'
    return
  }
  const next = cloneBase()
  next.activeProviderId = 'default'
  next.activeModel = pform.model.trim()
  next.systemPrompt = pform.systemPrompt
  next.providers = [
    {
      ID: 'default',
      Label: pform.label.trim() || 'default',
      BaseURL: pform.baseURL.trim(),
      APIKey: pform.apiKey.trim(),
      Models: pform.models,
    },
  ]
  await doSave(next)
}

async function saveAppearance() {
  errMsg.value = ''
  const next = cloneBase()
  next.appearance = { theme: aform.theme, fontSize: aform.fontSize, chartBg: aform.chartBg }
  await doSave(next)
}

function resetAppearance() {
  aform.theme = 'system'
  aform.fontSize = 14
  aform.chartBg = ''
  saveAppearance()
}

async function doSave(next: Settings) {
  saving.value = true
  try {
    await saveSettings(next)
    syncFromSettings() // 保存后回填（后端可能归一化）
  } catch (e) {
    errMsg.value = e instanceof Error ? e.message : String(e)
  } finally {
    saving.value = false
  }
}

onMounted(syncFromSettings)
</script>

<template>
  <div class="panel">
    <header class="panel__head">
      <h2>设置</h2>
      <nav class="tabs">
        <button
          type="button"
          class="tab"
          :class="{ 'tab--on': tab === 'provider' }"
          @click="tab = 'provider'"
        >
          提供商
        </button>
        <button
          type="button"
          class="tab"
          :class="{ 'tab--on': tab === 'appearance' }"
          @click="tab = 'appearance'"
        >
          外观
        </button>
      </nav>
      <button class="panel__close" @click="emit('close')">✕</button>
    </header>

    <div class="panel__body">
      <p v-if="errMsg" class="panel__err">{{ errMsg }}</p>

      <!-- ============ 提供商 ============ -->
      <template v-if="tab === 'provider'">
        <label class="field">
          <span>服务</span>
          <div class="presets">
            <button
              v-for="(p, i) in PRESETS"
              :key="p.label"
              type="button"
              class="chip"
              :class="{ 'chip--on': pform.baseURL === p.baseURL }"
              @click="applyPreset(i)"
            >
              {{ p.label }}
            </button>
          </div>
        </label>

        <label class="field">
          <span>Base URL</span>
          <input v-model="pform.baseURL" type="text" placeholder="https://…/v1" />
        </label>

        <label class="field">
          <span>API Key</span>
          <div class="key-row">
            <input v-model="pform.apiKey" type="password" placeholder="sk-…（本地 Ollama 可留空）" />
            <button
              type="button"
              class="btn btn--small"
              :disabled="scanning || !pform.baseURL.trim()"
              @click="scanModels"
            >
              {{ scanning ? '扫描中…' : '扫描模型' }}
            </button>
          </div>
        </label>

        <label class="field">
          <span>模型</span>
          <input v-model="pform.model" list="chatty-model-options" type="text" placeholder="openai/gpt-4o-mini" />
          <datalist id="chatty-model-options">
            <option v-for="m in pform.models" :key="m" :value="m"></option>
          </datalist>
          <span v-if="pform.models.length" class="hint">可点击输入框从下拉选择，也可手动输入。</span>
        </label>

        <p v-if="scanMsg" class="scan-msg">
          {{ scanMsg }}
        </p>

        <label class="field">
          <span>系统提示（Typst 输出约定）</span>
          <textarea v-model="pform.systemPrompt" rows="7"></textarea>
        </label>

        <p class="hint">API Key 明文保存在本地 settings.json，仅用于直连你配置的端点。</p>

        <div class="row">
          <button class="btn" :disabled="saving" @click="saveProvider">保存</button>
        </div>
      </template>

      <!-- ============ 外观 ============ -->
      <template v-else>
        <label class="field">
          <span>主题</span>
          <div class="themes">
            <button
              v-for="t in THEMES"
              :key="t.id"
              type="button"
              class="chip theme-chip"
              :class="{ 'chip--on': aform.theme === t.id }"
              @click="aform.theme = t.id"
            >
              {{ t.label }}
            </button>
          </div>
        </label>

        <label class="field">
          <span>字体大小</span>
          <div class="size-row">
            <button class="step" :disabled="aform.fontSize <= MIN_SIZE" @click="aform.fontSize--">－</button>
            <span class="size-val">{{ aform.fontSize }}px</span>
            <button class="step" :disabled="aform.fontSize >= MAX_SIZE" @click="aform.fontSize++">＋</button>
            <button type="button" class="chip" @click="resetAppearance">恢复默认</button>
          </div>
          <div class="size-bar">
            <input
              v-model.number="aform.fontSize"
              class="range"
              type="range"
              :min="MIN_SIZE"
              :max="MAX_SIZE"
              step="1"
            />
            <span class="size-tip">{{ MIN_SIZE }}px — {{ MAX_SIZE }}px</span>
          </div>
        </label>

        <div class="row">
          <button class="btn" :disabled="saving" @click="saveAppearance">保存</button>
        </div>
      </template>
    </div>
  </div>
</template>

<style scoped>
.panel {
  position: fixed;
  inset: 0;
  margin: auto;
  width: min(560px, calc(100vw - 48px));
  height: min(680px, calc(100vh - 48px));
  background: var(--surface);
  color: var(--text);
  border-radius: 14px;
  border: 1px solid var(--border);
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.25);
  display: flex;
  flex-direction: column;
  z-index: 20;
}
.panel__head {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 18px;
  border-bottom: 1px solid var(--border);
}
.panel__head h2 {
  margin: 0;
  font-size: var(--fs-lg);
}
.tabs {
  display: flex;
  gap: 4px;
  margin: 0 auto;
  background: var(--bg-side);
  border-radius: 9px;
  padding: 3px;
}
.tab {
  border: none;
  background: transparent;
  color: var(--text-dim);
  font-size: var(--fs-sm);
  padding: 5px 16px;
  border-radius: 7px;
  cursor: pointer;
}
.tab--on {
  background: var(--surface);
  color: var(--text);
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.12);
}
.panel__close {
  border: none;
  background: none;
  font-size: var(--fs-lg);
  cursor: pointer;
  color: var(--text-faint);
}
.panel__body {
  flex: 1;
  overflow-y: auto;
  padding: 16px 18px;
}
.panel__err {
  color: var(--danger);
  font-size: var(--fs-sm);
}
.field {
  display: block;
  margin-bottom: 14px;
}
.field > span {
  display: block;
  font-size: var(--fs-sm);
  color: var(--text-dim);
  margin-bottom: 5px;
}
.field input[type='text'],
.field input[type='password'],
.field textarea {
  width: 100%;
  box-sizing: border-box;
  padding: 8px 10px;
  border: 1px solid var(--border-strong);
  border-radius: 8px;
  background: var(--surface);
  font-size: var(--fs-sm);
  font-family: inherit;
  resize: vertical;
}
.field textarea {
  background: var(--bg);
}
.presets {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}
.key-row {
  display: flex;
  gap: 8px;
}
.key-row input {
  flex: 1;
}
.btn--small {
  padding: 6px 14px;
  font-size: var(--fs-sm);
  white-space: nowrap;
}
.scan-msg {
  margin: -6px 0 14px;
  font-size: var(--fs-xs);
  color: var(--text-dim);
}
.scan-msg--err {
  color: var(--danger);
}
.chip {
  border: 1px solid var(--border-strong);
  background: var(--surface);
  color: var(--text);
  border-radius: 999px;
  padding: 5px 12px;
  font-size: var(--fs-xs);
  cursor: pointer;
}
.chip--on {
  border-color: var(--accent);
  background: var(--accent-weak);
  color: var(--accent);
}
.themes {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}
.theme-chip {
  padding: 6px 16px;
  font-size: var(--fs-sm);
}
.size-row {
  display: flex;
  align-items: center;
  gap: 10px;
}
.step {
  width: 30px;
  height: 30px;
  border-radius: 8px;
  border: 1px solid var(--border-strong);
  background: var(--surface);
  color: var(--text);
  font-size: var(--fs-lg);
  line-height: 1;
  cursor: pointer;
}
.step:disabled {
  opacity: 0.4;
  cursor: default;
}
.size-val {
  min-width: 48px;
  text-align: center;
  font-size: var(--fs-md);
  font-variant-numeric: tabular-nums;
}
.size-bar {
  margin-top: 10px;
  display: flex;
  align-items: center;
  gap: 10px;
}
.range {
  flex: 1;
  accent-color: var(--accent);
}
.size-tip {
  font-size: var(--fs-xs);
  color: var(--text-faint);
  white-space: nowrap;
}
.row {
  margin-top: 18px;
  display: flex;
  justify-content: flex-end;
}
.charts {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}
.swatch {
  border: 1px solid var(--border-strong);
  background: var(--surface);
  border-radius: 8px;
  padding: 3px;
  cursor: pointer;
  line-height: 0;
}
.swatch--on {
  border-color: var(--accent);
  box-shadow: 0 0 0 1px var(--accent) inset;
}
.swatch__dot {
  display: inline-block;
  width: 22px;
  height: 22px;
  border-radius: 5px;
}
.swatch__dot--auto {
  background: conic-gradient(#6e2b2b, #75561c, #46692b, #254d59, #403159, #593148, #6e2b2b);
  color: #fff;
  font-size: 11px;
  line-height: 22px;
  text-align: center;
  font-weight: 700;
}
.btn {
  padding: 8px 22px;
  border-radius: 8px;
  border: none;
  background: var(--accent);
  color: var(--on-accent);
  cursor: pointer;
  font-size: var(--fs-md);
}
.btn:disabled {
  opacity: 0.6;
}
.hint {
  font-size: var(--fs-xs);
  color: var(--text-faint);
  margin-top: 4px;
}
</style>
