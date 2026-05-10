<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useSessionStore } from '../stores/session'

const router = useRouter()
const session = useSessionStore()
const mode = ref<'login' | 'register'>('login')
const username = ref('')
const password = ref('')
const nickname = ref('')
const message = ref('')

async function returnToSetup() {
  session.resetServer()
  await router.push('/setup')
}

async function submit() {
  message.value = ''

  try {
    if (mode.value === 'login') {
      await session.login({ username: username.value, password: password.value })
    } else {
      await session.register({
        username: username.value,
        password: password.value,
        nickname: nickname.value || username.value,
      })
    }
    await router.push('/app')
  } catch (error) {
    message.value = error instanceof Error ? error.message : '认证失败'
  }
}
</script>

<template>
  <main class="auth-page">
    <section class="auth-card">
      <button class="auth-close-button" type="button" aria-label="返回服务设置" @click="returnToSetup">×</button>
      <div class="eyebrow">{{ session.serverName || 'Unilo' }}</div>
      <h1>进入协作空间</h1>
      <p class="muted">当前服务：{{ session.serverUrl }}</p>

      <div class="segmented">
        <button :class="{ active: mode === 'login' }" type="button" @click="mode = 'login'">登录</button>
        <button :class="{ active: mode === 'register' }" type="button" @click="mode = 'register'">注册</button>
      </div>

      <form class="form-stack" @submit.prevent="submit">
        <label>
          <span>用户名</span>
          <input v-model="username" required autocomplete="username" placeholder="username" />
        </label>
        <label v-if="mode === 'register'">
          <span>昵称</span>
          <input v-model="nickname" autocomplete="nickname" placeholder="display name" />
        </label>
        <label>
          <span>密码</span>
          <input v-model="password" type="password" required autocomplete="current-password" placeholder="password" />
        </label>
        <button class="primary-button" type="submit" :disabled="session.loading">
          {{ session.loading ? '处理中...' : mode === 'login' ? '登录' : '创建账号' }}
        </button>
      </form>

      <p v-if="message || session.error" class="error-text">{{ message || session.error }}</p>
    </section>
  </main>
</template>
