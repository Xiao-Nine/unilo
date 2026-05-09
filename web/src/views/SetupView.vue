<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { DEFAULT_DEV_SERVER_URL, useSessionStore } from '../stores/session'

const router = useRouter()
const session = useSessionStore()
const serverUrl = ref(session.serverUrl || DEFAULT_DEV_SERVER_URL)
const secretKey = ref('')
const message = ref('')

async function submit() {
  message.value = ''

  try {
    await session.verify(serverUrl.value, secretKey.value)
    await router.push('/login')
  } catch (error) {
    message.value = error instanceof Error ? error.message : '服务验证失败'
  }
}
</script>

<template>
  <main class="auth-page">
    <section class="auth-card setup-card">
      <div class="eyebrow">Unilo setup</div>
      <h1>连接你的协作后端</h1>
      <p class="muted">输入 ali6 后端地址和服务密钥，通过验证后即可登录或注册。</p>

      <form class="form-stack" @submit.prevent="submit">
        <label>
          <span>服务地址</span>
          <input v-model="serverUrl" type="url" required placeholder="http://47.98.210.172:8000" />
        </label>
        <label>
          <span>服务密钥</span>
          <input v-model="secretKey" type="password" required placeholder="Service secret key" />
        </label>
        <button class="primary-button" type="submit" :disabled="session.loading">
          {{ session.loading ? '验证中...' : '验证并继续' }}
        </button>
      </form>

      <p v-if="message || session.error" class="error-text">{{ message || session.error }}</p>
    </section>
  </main>
</template>
