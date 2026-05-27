<template>
  <section class="section-stack">
    <div class="settings-stack-page">
      <article class="overview-block settings-stack">
        <div>
          <h3 class="block-title">代理与运维设置</h3>
        </div>
        <div class="compact-grid">
          <label class="field">
            <span class="field-label">New API 内网地址</span>
            <input v-model="settingsModel.newapi_base_url" class="app-input" type="text" placeholder="http://new-api:3000" />
          </label>
          <label class="field">
            <span class="field-label">New API 管理员令牌</span>
            <input v-model="settingsModel.newapi_admin_token" class="app-input" type="password" placeholder="用于用户与封禁操作" autocomplete="off" />
          </label>
          <label class="field">
            <span class="field-label">控制台公开地址</span>
            <input v-model="settingsModel.public_base_url" class="app-input" type="text" placeholder="可留空自动推断" />
          </label>
          <label class="field">
            <span class="field-label">管理员密码</span>
            <input v-model="settingsModel.admin_password" class="app-input" type="password" placeholder="控制台登录密码" autocomplete="off" />
          </label>
          <label class="field">
            <span class="field-label">用户级 RPM 限制</span>
            <input v-model.number="settingsModel.rpm_limit" class="app-input" type="number" min="1" />
          </label>
          <label class="field">
            <span class="field-label">UA 违规封禁阈值</span>
            <input v-model.number="settingsModel.ua_ban_strikes" class="app-input" type="number" min="1" />
          </label>
          <label class="field">
            <span class="field-label">签到限额
              <span class="field-hint" v-if="settingsModel.checkin_threshold">余额 ≥ ${{ (settingsModel.checkin_threshold / 500000).toFixed(2) }} 时禁止签到</span>
            </span>
            <input v-model.number="settingsModel.checkin_threshold" class="app-input" type="number" min="0" placeholder="0 表示不限制" />
          </label>
        </div>
        <div class="field field-full">
          <span class="field-label field-label-with-help">
            <span>允许的 UA 前缀</span>
            <button class="help-icon" type="button" aria-label="查看允许的 UA 前缀说明" @click="openHelp('allowed_ua')">?</button>
          </span>
          <div class="settings-visual-stack">
            <p class="field-caption">通过条目维护允许前缀，保存时仍会写成字符串数组。</p>

            <div v-if="allowedUADraft.items.length" class="list-editor-stack">
              <div v-for="(item, index) in allowedUADraft.items" :key="'allowed-ua-' + index" class="list-editor-row soft-card">
                <template v-if="allowedUADraft.editIndex === index">
                  <input
                    v-model="allowedUADraft.editValue"
                    class="app-input list-editor-input mono"
                    type="text"
                    placeholder="例如 FLClash/"
                    @keyup.enter="saveAllowedUAEdit"
                  />
                  <div class="action-row">
                    <n-button size="small" type="primary" @click="saveAllowedUAEdit">保存</n-button>
                    <n-button size="small" tertiary @click="cancelAllowedUAEdit">取消</n-button>
                  </div>
                </template>
                <template v-else>
                  <div class="list-editor-content">
                    <div class="cell-title mono">{{ item }}</div>
                    <div class="field-caption">前缀 {{ index + 1 }}</div>
                  </div>
                  <div class="action-row">
                    <n-button size="small" secondary @click="startAllowedUAEdit(index)">编辑</n-button>
                    <n-button size="small" type="warning" ghost @click="removeAllowedUAItem(index)">删除</n-button>
                  </div>
                </template>
              </div>
            </div>
            <div v-else class="section-note">当前没有配置允许的 UA 前缀。留空时表示不启用 UA 限制。</div>

            <div class="list-editor-create">
              <input
                v-model="allowedUADraft.pending"
                class="app-input list-editor-input"
                type="text"
                placeholder="新增一个允许前缀，例如 FLClash/"
                @keyup.enter="addAllowedUAItem"
              />
              <n-button secondary @click="addAllowedUAItem">新增前缀</n-button>
            </div>
          </div>
        </div>
      </article>

      <article class="overview-block settings-stack">
        <div>
          <h3 class="block-title">OAuth 与 Discord 准入</h3>
        </div>
        <div class="compact-grid">
          <label class="field">
            <span class="field-label">OAuth Client ID</span>
            <input v-model="settingsModel.oauth_client_id" class="app-input" type="text" />
          </label>
          <label class="field">
            <span class="field-label">OAuth Client Secret</span>
            <input v-model="settingsModel.oauth_client_secret" class="app-input" type="password" autocomplete="off" />
          </label>
          <label class="field">
            <span class="field-label">Provider Slug</span>
            <input v-model="settingsModel.oauth_provider_slug" class="app-input" type="text" />
          </label>
          <label class="field">
            <span class="field-label">Discord Client ID</span>
            <input v-model="settingsModel.discord_client_id" class="app-input" type="text" />
          </label>
          <label class="field">
            <span class="field-label">Discord Client Secret</span>
            <input v-model="settingsModel.discord_client_secret" class="app-input" type="password" autocomplete="off" />
          </label>
          <label class="field">
            <span class="field-label">Discord Guild ID</span>
            <input v-model="settingsModel.discord_guild_id" class="app-input" type="text" />
          </label>
          <label class="field">
            <span class="field-label">State TTL（秒）</span>
            <input v-model.number="settingsModel.oauth_state_ttl_seconds" class="app-input" type="number" min="60" />
          </label>
          <label class="field">
            <span class="field-label">Code TTL（秒）</span>
            <input v-model.number="settingsModel.oauth_code_ttl_seconds" class="app-input" type="number" min="60" />
          </label>
          <label class="field">
            <span class="field-label">Token TTL（秒）</span>
            <input v-model.number="settingsModel.oauth_token_ttl_seconds" class="app-input" type="number" min="60" />
          </label>
        </div>
        <div class="field field-full">
          <span class="field-label">Discord OAuth Scopes</span>
          <textarea v-model="settingsText.discord_oauth_scopes" class="app-textarea" rows="4" placeholder="每行一个 scope，例如 identify"></textarea>
        </div>
        <div class="field field-full">
          <span class="field-label field-label-with-help">
            <span>Discord 准入规则</span>
            <button class="help-icon" type="button" aria-label="查看 Discord 准入规则说明" @click="openHelp('discord_access_policy')">?</button>
          </span>
          <div class="settings-visual-stack">
            <p class="field-caption">在界面里维护条件和规则组，保存时仍会写回后端当前使用的 JSON 结构。</p>
            <PolicyGroupEditor :group="discordAccessPolicyDraft" root />
          </div>
        </div>
      </article>
    </div>

    <article class="overview-block settings-actions">
      <div class="action-row">
        <n-button secondary @click="resetSettingsDraft">重置草稿</n-button>
        <n-button type="primary" @click="saveSettings">保存设置</n-button>
      </div>
    </article>
  </section>
</template>

<script setup>
import { inject } from 'vue'
import PolicyGroupEditor from '../components/PolicyGroupEditor.vue'

const store = inject('guard')
const {
  settingsModel,
  settingsText,
  allowedUADraft,
  discordAccessPolicyDraft,
  openHelp,
  addAllowedUAItem,
  startAllowedUAEdit,
  cancelAllowedUAEdit,
  saveAllowedUAEdit,
  removeAllowedUAItem,
  resetSettingsDraft,
  saveSettings
} = store
</script>
