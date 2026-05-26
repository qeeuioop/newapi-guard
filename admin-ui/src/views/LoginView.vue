<template>
  <div class="login-shell">
    <div class="login-panel">
      <section class="frame login-card">
        <div>
          <div class="panel-label">登录</div>
          <h2 class="panel-title">进入 Guard 控制台</h2>
        </div>

        <form class="login-form" @submit.prevent="submitLogin">
          <label class="login-field" for="login-password">
            <span class="login-field-label">管理员密码</span>
            <input
              id="login-password"
              v-model="login.password"
              class="login-password-field"
              type="password"
              placeholder="请输入管理员密码"
              autocomplete="current-password"
            />
          </label>
        </form>

        <p v-if="login.notice" class="login-notice">{{ login.notice }}</p>

        <div class="auth-row">
          <n-button type="primary" size="large" :loading="authLoading" @click="submitLogin">进入 Guard 控制台</n-button>
        </div>
      </section>
    </div>
  </div>
</template>

<script setup>
import { inject } from 'vue'

const store = inject('guard')
const { auth, login, authLoading, submitLogin } = store
</script>
