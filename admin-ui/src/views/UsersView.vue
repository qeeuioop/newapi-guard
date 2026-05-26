<template>
  <section class="section-stack">
    <div class="filters">
      <input
        v-model="userSearchDraft"
        class="app-input filter-input"
        type="text"
        placeholder="搜索 newapi id、discord id、discord 名称或用户名"
        @keyup.enter="applyUserSearch"
      />
      <n-button type="primary" @click="applyUserSearch">搜索</n-button>
      <n-button tertiary @click="resetUserSearch">清空</n-button>
      <n-button secondary @click="openCreateUserModal">创建用户</n-button>
    </div>

    <div class="toolbar-split soft-card">
      <div class="table-meta">
        {{ userQuery.search ? "关键字：" + userQuery.search + " · " : "" }}共 {{ formatNumber(userQuery.total || users.length) }} 条 · 第
        {{ userQuery.page }} / {{ userPageCount }} 页
      </div>
      <div class="table-actions">
        <select class="app-select page-size-select" :value="userQuery.size" @change="handleUserPageSizeChange(Number($event.target.value))">
          <option v-for="option in userPageSizeOptions" :key="'user-size-' + option.value" :value="option.value">{{ option.label }}</option>
        </select>
        <n-pagination :page="userQuery.page" :page-count="userPageCount" simple @update:page="handleUserPageChange" />
      </div>
    </div>

    <article class="overview-block">
      <div class="panel-heading">
        <div>
          <h3 class="block-title">用户列表</h3>
        </div>
        <n-tag size="large" :bordered="false">共 {{ formatNumber(userQuery.total || users.length) }} 条 · 本页 {{ users.length }} 条</n-tag>
      </div>

      <div class="table-shell" v-if="users.length">
        <table>
          <thead>
            <tr>
              <th>用户</th>
              <th>配额 / 状态</th>
              <th>身份映射</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="item in users" :key="'user-' + item.newapi_user_id">
              <td>
                <div class="cell-stack">
                  <div class="cell-title">{{ item.display_name || item.username || "用户 " + item.newapi_user_id }}</div>
                  <div class="cell-sub mono">newapi {{ item.newapi_user_id }}</div>
                  <div class="cell-sub">{{ item.email || "未绑定邮箱" }}</div>
                </div>
              </td>
              <td>
                <div class="cell-stack">
                  <div class="cell-title">{{ formatQuotaUSD(item.quota) }}</div>
                  <div class="cell-sub">状态：{{ item.status === 2 ? "已封禁" : "正常" }}</div>
                  <div class="cell-sub">创建：{{ parseDate(item.created_at || item.created_at_unix) }}</div>
                </div>
              </td>
              <td>
                <div class="cell-stack">
                  <div class="cell-sub">Discord：{{ item.discord_name || item.discord_id || "未绑定" }}</div>
                  <div class="cell-sub">白名单：{{ item.is_whitelist ? "是" : "否" }}</div>
                  <div class="cell-sub">分组：{{ item.group || "default" }}</div>
                </div>
              </td>
              <td>
                <div class="action-row">
                  <n-button size="small" secondary @click="toggleWhitelist(item)">{{ item.is_whitelist ? "移出白名单" : "加入白名单" }}</n-button>
                  <n-button size="small" type="warning" ghost @click="openBanModal(item)">{{ item.status === 2 ? "再次记录封禁" : "封禁" }}</n-button>
                  <n-button v-if="item.status === 2" size="small" type="success" ghost @click="quickUnban(item)">解除封禁</n-button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <div v-else class="empty-shell">
        <n-empty description="暂时没有匹配的用户">
          <template #extra>
            <div class="action-row">
              <n-button tertiary @click="resetUserSearch">清空筛选</n-button>
              <n-button secondary @click="openCreateUserModal">创建用户</n-button>
            </div>
          </template>
        </n-empty>
      </div>
    </article>
  </section>
</template>

<script setup>
import { inject } from 'vue'

const store = inject('guard')
const {
  users,
  userQuery,
  userSearchDraft,
  userPageCount,
  userPageSizeOptions,
  formatNumber,
  formatQuotaUSD,
  parseDate,
  applyUserSearch,
  resetUserSearch,
  handleUserPageChange,
  handleUserPageSizeChange,
  openCreateUserModal,
  toggleWhitelist,
  openBanModal,
  quickUnban
} = store
</script>
