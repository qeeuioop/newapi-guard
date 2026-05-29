import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from "vue";
import { createDiscreteApi, darkTheme } from "naive-ui";

export function useGuardStore() {
  // ─── Theme ───────────────────────────────────────────────
  const themeOptions = [
    { value: "system", label: "自动" },
    { value: "light", label: "日间" },
    { value: "dark", label: "夜间" }
  ];

  const storedThemeMode = localStorage.getItem("guard_theme_mode");
  const themeMode = ref(themeOptions.some((option) => option.value === storedThemeMode) ? storedThemeMode : "system");
  const systemTheme = ref("dark");
  const resolvedTheme = computed(() => (themeMode.value === "system" ? systemTheme.value : themeMode.value));
  const currentThemeLabel = computed(() => {
    const currentThemeOption = themeOptions.find((option) => option.value === themeMode.value);
    return currentThemeOption ? currentThemeOption.label : themeOptions[0].label;
  });
  const naiveTheme = computed(() => (resolvedTheme.value === "dark" ? darkTheme : null));

  const themeOverrides = computed(() => {
    const isDark = resolvedTheme.value === "dark";
    return {
      common: {
        primaryColor: "oklch(0.78 0.124 174)",
        primaryColorHover: "oklch(0.83 0.11 174)",
        primaryColorPressed: "oklch(0.72 0.11 174)",
        infoColor: "oklch(0.78 0.124 174)",
        successColor: "oklch(0.76 0.12 160)",
        warningColor: "oklch(0.8 0.12 78)",
        errorColor: "oklch(0.7 0.16 27)",
        bodyColor: isDark ? "oklch(0.19 0.02 220)" : "oklch(0.985 0.008 210)",
        cardColor: isDark ? "oklch(0.235 0.022 220)" : "oklch(0.998 0.006 210)",
        modalColor: isDark ? "oklch(0.235 0.022 220)" : "oklch(0.998 0.006 210)",
        popoverColor: isDark ? "oklch(0.235 0.022 220)" : "oklch(0.998 0.006 210)",
        borderColor: isDark ? "rgba(255,255,255,0.08)" : "rgba(27,52,67,0.12)",
        textColorBase: isDark ? "oklch(0.92 0.01 220)" : "oklch(0.25 0.018 220)",
        textColor2: isDark ? "oklch(0.75 0.014 220)" : "oklch(0.48 0.016 220)",
        fontFamily: "\"Noto Sans SC\", \"Segoe UI\", sans-serif",
        fontFamilyMono: "\"Chakra Petch\", sans-serif"
      },
      Button: {
        borderRadiusMedium: "14px",
        borderRadiusSmall: "12px"
      },
      Input: {
        color: isDark ? "rgba(255,255,255,0.02)" : "rgba(255,255,255,0.78)",
        borderRadius: "14px"
      }
    };
  });

  const discreteProviderProps = computed(() => ({
    theme: naiveTheme.value,
    themeOverrides: themeOverrides.value
  }));
  const { message, dialog } = createDiscreteApi(["message", "dialog"], {
    configProviderProps: discreteProviderProps
  });

  // ─── Navigation ──────────────────────────────────────────
  const navItems = [
    { key: "dashboard", label: "总览", hint: "站点态势与风险信号" },
    { key: "users", label: "用户", hint: "检索、创建、配额与白名单" },
    { key: "whitelist", label: "白名单", hint: "豁免用户清单" },
    { key: "bans", label: "封禁", hint: "当前封禁与历史记录" },
    { key: "settings", label: "设置", hint: "代理、签到、OAuth 与准入" },
    { key: "logs", label: "日志", hint: "封禁、签到与统计趋势" }
  ];

  const sectionMap = {
    dashboard: {
      kicker: "系统总览",
      title: "总览与风险信号",
      copy: "先判断站点是否稳定，再进入具体用户、封禁或日志视图。这个页只回答一个问题：现在值不值得立刻干预。"
    },
    users: {
      kicker: "用户台",
      title: "用户检索与快速操作",
      copy: "优先用列表搜索锁定用户，再直接处理白名单、封禁和额度入口。这里既看 New API 实时状态，也看 Guard 补充的 Discord 映射。"
    },
    whitelist: {
      kicker: "信任通道",
      title: "白名单清单",
      copy: "查看当前被豁免 UA 与 RPM 控制的用户，适合维护长期可信客户端与特殊账号。"
    },
    bans: {
      kicker: "风控台",
      title: "封禁处置台",
      copy: "当前封禁视图直接对齐 New API 的 status=2，历史视图则保留 Guard 的审计上下文。"
    },
    settings: {
      kicker: "控制面",
      title: "系统设置与准入参数",
      copy: "这里集中维护代理目标、签到策略、管理员凭据、OAuth Provider 配置以及 Discord 准入规则。"
    },
    logs: {
      kicker: "时间线",
      title: "日志与趋势",
      copy: "把封禁、签到和每日统计放在同一观察面板内，方便回看行为与追踪异常变化。"
    }
  };

  // ─── Auth ────────────────────────────────────────────────
  const auth = reactive({
    token: localStorage.getItem("guard_token") || ""
  });

  const login = reactive({
    password: "",
    notice: ""
  });

  const authLoading = ref(false);

  // ─── Core UI state ───────────────────────────────────────
  const pageLoadingCount = ref(0);
  const pageLoading = computed(() => pageLoadingCount.value > 0);
  const activeView = ref("dashboard");
  const sidebarCollapsed = ref(false);
  const logView = ref("bans");
  const banMode = ref("active");
  const banFilter = ref("");
  const checkinFilter = ref("");
  const userSearchDraft = ref("");

  const viewLoaded = reactive({
    dashboard: false,
    users: false,
    whitelist: false,
    bans: false,
    settings: false,
    logs: false
  });

  const viewState = reactive({
    dashboard: { error: "", syncedAt: 0 },
    users: { error: "", syncedAt: 0 },
    whitelist: { error: "", syncedAt: 0 },
    bans: { error: "", syncedAt: 0 },
    settings: { error: "", syncedAt: 0 },
    logs: { error: "", syncedAt: 0 }
  });

  // ─── Data ────────────────────────────────────────────────
  const dashboard = reactive({
    today: {
      total_requests: 0,
      blocked_ua: 0,
      blocked_rpm: 0,
      checkins: 0,
      new_users: 0,
      new_bans: 0
    },
    total_users: 0,
    active_bans: 0,
    whitelist_count: 0
  });

  const userQuery = reactive({
    search: "",
    page: 1,
    size: 20,
    total: 0
  });

  const users = ref([]);
  const whitelist = ref([]);
  const activeBans = ref([]);
  const banHistory = ref([]);
  const banLogs = ref([]);
  const checkinLogs = ref([]);
  const statsLogs = ref([]);

  const settingsModel = reactive({});
  const settingsText = reactive({
    discord_oauth_scopes: "",
    allowed_origins: "",
    oauth_allowed_redirect_uris: ""
  });
  const allowedUADraft = reactive({
    items: [],
    pending: "",
    editIndex: -1,
    editValue: ""
  });
  const discordAccessPolicyDraft = ref(createPolicyGroup());

  // ─── Help topics ─────────────────────────────────────────
  const helpTopics = {
    allowed_ua: {
      title: "允许的 UA 前缀",
      explain:
        "后端会把这里当成字符串数组处理，逐条用前缀匹配请求头里的 User-Agent。这里留空时，后端会直接放行全部 UA，不启用 UA 限制。",
      howto:
        "在界面里逐条新增、编辑或删除前缀即可，不需要自己写数组格式。保存时前端会自动整理成后端需要的字符串数组。",
      example: `条目示例
FLClash/
Shadowrocket/
ClashMeta/
OpenClash/`
    },
    discord_access_policy: {
      title: "Discord 准入规则",
      explain:
        '后端会把这里按 JSON 对象解析，并递归执行 logic / conditions / groups。当前代码只支持两种条件：field=”guild_id” 配 op=”eq”，以及 field=”roles” 配 op=”contains”。其中 guild_id 对应当前 Discord 服务器 ID，roles 对应 Discord 返回的成员角色 ID 列表，不认角色名称。',
      howto:
        '直接在界面里选择 AND / OR、增加条件或嵌套规则组即可。想要求“必须在某个服务器，并且拥有 A 或 B 角色之一”，就把根规则设成 AND，再新增一个 OR 子规则组来放多个角色条件。空规则会被后端视为全部放行。',
      example: `示例结构
根规则：AND
条件 1：服务器 ID = 123456789012345678
子规则组：OR
- 角色 ID = 987654321000000001
- 角色 ID = 987654321000000002`
    }
  };

  const helpModal = reactive({
    show: false,
    key: "allowed_ua"
  });

  // ─── Modals ──────────────────────────────────────────────
  const createUserModal = reactive({
    show: false,
    loading: false,
    form: {
      mode: "password",
      username: "",
      password: "",
      discord_id: "",
      discord_name: "",
      initial_quota: 0,
      is_whitelist: false
    }
  });

  const banModal = reactive({
    show: false,
    loading: false,
    form: {
      user_ref: "",
      newapi_user_id: null,
      discord_id: "",
      reason: "",
      duration: "permanent"
    }
  });

  const banDurationOptions = [
    { label: "永久封禁", value: "permanent" },
    { label: "7 天", value: "7d" },
    { label: "30 天", value: "30d" }
  ];

  const userPageSizeOptions = [
    { label: "20 / 页", value: 20 },
    { label: "50 / 页", value: 50 },
    { label: "100 / 页", value: 100 }
  ];

  // ─── Computed ────────────────────────────────────────────
  const sectionMeta = computed(() => sectionMap[activeView.value]);
  const displayedBans = computed(() => (banMode.value === "active" ? activeBans.value : banHistory.value));
  const userPageCount = computed(() => Math.max(1, Math.ceil((Number(userQuery.total) || 0) / userQuery.size)));
  const activeBansWithContext = computed(() => activeBans.value.filter((item) => !item.context_missing).length);
  const helpContent = computed(() => helpTopics[helpModal.key] || helpTopics.allowed_ua);
  const currentViewError = computed(() => viewState[activeView.value].error);
  const currentViewSyncedAt = computed(() => {
    const syncedAt = viewState[activeView.value].syncedAt;
    return syncedAt ? new Date(syncedAt).toLocaleString("zh-CN") : "尚未同步";
  });
  const currentViewHealth = computed(() => {
    if (viewState[activeView.value].error) {
      return {
        label: "需要复查",
        note: "当前视图最近一次同步失败，建议先刷新当前视图。"
      };
    }
    if (viewState[activeView.value].syncedAt) {
      return {
        label: "同步正常",
        note: "当前视图已接通后端接口，可以继续执行管理操作。"
      };
    }
    return {
      label: "等待加载",
      note: "当前视图还没有完成首次同步。"
    };
  });

  const filteredBans = computed(() => {
    const keyword = banFilter.value.trim().toLowerCase();
    if (!keyword) {
      return displayedBans.value;
    }
    return displayedBans.value.filter((item) =>
      [
        item.display_name,
        item.username,
        item.discord_id,
        item.discord_name,
        item.reason,
        item.violation_ua,
        item.client_ip,
        item.newapi_user_id
      ].some((value) => String(value || "").toLowerCase().includes(keyword))
    );
  });

  // ─── Helpers ─────────────────────────────────────────────
  function clone(value, fallback) {
    return JSON.parse(JSON.stringify(value ?? fallback));
  }

  function applyResolvedTheme(theme) {
    if (typeof document === "undefined") return;
    document.documentElement.dataset.theme = theme;
    document.documentElement.style.colorScheme = theme;
  }

  function openHelp(key) {
    helpModal.key = key;
    helpModal.show = true;
  }

  function parseDate(value) {
    if (!value) return "—";
    if (typeof value === "number") {
      return new Date(value * 1000).toLocaleString("zh-CN");
    }
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return value;
    return date.toLocaleString("zh-CN");
  }

  function formatNumber(value) {
    return new Intl.NumberFormat("zh-CN").format(Number(value || 0));
  }

  function formatQuotaUSD(value) {
    return new Intl.NumberFormat("en-US", {
      style: "currency",
      currency: "USD",
      minimumFractionDigits: 0,
      maximumFractionDigits: 2
    }).format(Number(value || 0) / 500000);
  }

  function formatMaybeArray(value) {
    return Array.isArray(value) ? value.join("\n") : "";
  }

  function createLocalId() {
    if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
      return crypto.randomUUID();
    }
    return `guard-${Math.random().toString(16).slice(2)}-${Date.now()}`;
  }

  function createPolicyCondition(field = "guild_id", value = "") {
    const normalizedField = field === "roles" ? "roles" : "guild_id";
    return {
      id: createLocalId(),
      field: normalizedField,
      op: normalizedField === "roles" ? "contains" : "eq",
      value: String(value ?? "")
    };
  }

  function normalizePolicyCondition(condition) {
    if (!condition || typeof condition !== "object") {
      return null;
    }
    return createPolicyCondition(condition.field, condition.value);
  }

  function createPolicyGroup(group = {}) {
    return {
      id: createLocalId(),
      logic: String(group.logic || "").toLowerCase() === "or" ? "or" : "and",
      conditions: Array.isArray(group.conditions) ? group.conditions.map(normalizePolicyCondition).filter(Boolean) : [],
      groups: Array.isArray(group.groups) ? group.groups.map((item) => createPolicyGroup(item)) : []
    };
  }

  function parsePolicyValue(value) {
    if (!value) {
      return createPolicyGroup();
    }
    if (typeof value === "string") {
      try {
        return createPolicyGroup(JSON.parse(value));
      } catch {
        return createPolicyGroup();
      }
    }
    if (typeof value === "object") {
      return createPolicyGroup(value);
    }
    return createPolicyGroup();
  }

  function serializePolicyGroup(group) {
    const conditions = (group.conditions || [])
      .map((condition) => ({
        field: condition.field === "roles" ? "roles" : "guild_id",
        op: condition.field === "roles" ? "contains" : "eq",
        value: String(condition.value || "").trim()
      }))
      .filter((condition) => condition.value);

    const groups = (group.groups || [])
      .map((item) => serializePolicyGroup(item))
      .filter((item) => item.conditions.length || item.groups.length);

    return {
      logic: group.logic === "or" ? "or" : "and",
      conditions,
      groups
    };
  }

  function validatePolicyGroup(group, path = "根规则") {
    for (const [index, condition] of (group.conditions || []).entries()) {
      if (!String(condition.value || "").trim()) {
        throw new Error(`${path} 的第 ${index + 1} 个条件还没有填写值`);
      }
    }
    for (const [index, child] of (group.groups || []).entries()) {
      validatePolicyGroup(child, `${path} > 子组 ${index + 1}`);
    }
  }

  function normalizeStringList(value) {
    if (Array.isArray(value)) {
      return value.map((item) => String(item || "").trim()).filter(Boolean);
    }
    if (typeof value === "string") {
      try {
        const parsed = JSON.parse(value);
        if (Array.isArray(parsed)) {
          return parsed.map((item) => String(item || "").trim()).filter(Boolean);
        }
      } catch {
        return value
          .split(/\n+/)
          .map((item) => item.trim())
          .filter(Boolean);
      }
    }
    return [];
  }

  function syncAllowedUADraft(value) {
    const items = normalizeStringList(value);
    allowedUADraft.items.splice(0, allowedUADraft.items.length, ...items);
    allowedUADraft.pending = "";
    allowedUADraft.editIndex = -1;
    allowedUADraft.editValue = "";
  }

  function syncDiscordAccessPolicyDraft(value) {
    discordAccessPolicyDraft.value = parsePolicyValue(value);
  }

  function addAllowedUAItem() {
    const nextValue = allowedUADraft.pending.trim();
    if (!nextValue) {
      message.warning("请先输入一个 UA 前缀");
      return;
    }
    if (allowedUADraft.items.includes(nextValue)) {
      message.info("这个 UA 前缀已经存在");
      return;
    }
    allowedUADraft.items.push(nextValue);
    allowedUADraft.pending = "";
  }

  function startAllowedUAEdit(index) {
    allowedUADraft.editIndex = index;
    allowedUADraft.editValue = allowedUADraft.items[index] || "";
  }

  function cancelAllowedUAEdit() {
    allowedUADraft.editIndex = -1;
    allowedUADraft.editValue = "";
  }

  function saveAllowedUAEdit() {
    const nextValue = allowedUADraft.editValue.trim();
    if (allowedUADraft.editIndex < 0) return;
    if (!nextValue) {
      message.warning("UA 前缀不能为空");
      return;
    }
    if (allowedUADraft.items.some((item, index) => index !== allowedUADraft.editIndex && item === nextValue)) {
      message.info("这个 UA 前缀已经存在");
      return;
    }
    allowedUADraft.items.splice(allowedUADraft.editIndex, 1, nextValue);
    cancelAllowedUAEdit();
  }

  function removeAllowedUAItem(index) {
    allowedUADraft.items.splice(index, 1);
    if (allowedUADraft.editIndex === index) {
      cancelAllowedUAEdit();
    } else if (allowedUADraft.editIndex > index) {
      allowedUADraft.editIndex -= 1;
    }
  }

  function navCount(key) {
    switch (key) {
      case "dashboard":
        return dashboard.active_bans || 0;
      case "users":
        return users.value.length;
      case "whitelist":
        return whitelist.value.length || dashboard.whitelist_count || 0;
      case "bans":
        return displayedBans.value.length;
      case "settings":
        return "Cfg";
      case "logs":
        return statsLogs.value.length || "Log";
      default:
        return 0;
    }
  }

  // ─── API ─────────────────────────────────────────────────
  async function request(path, options = {}) {
    const headers = { ...(options.headers || {}) };
    if (auth.token) {
      headers.Authorization = `Bearer ${auth.token}`;
    }
    if (options.body !== undefined && !headers["Content-Type"]) {
      headers["Content-Type"] = "application/json";
    }

    const response = await fetch(path, {
      method: options.method || "GET",
      headers,
      body: options.body !== undefined ? JSON.stringify(options.body) : undefined
    });
    const data = await response.json().catch(() => ({}));

    if (response.status === 401 || data.message === "未授权") {
      await logout(true);
      throw new Error("登录状态已失效，请重新登录");
    }
    if (!response.ok) {
      throw new Error(data.message || `请求失败（${response.status}）`);
    }
    if (Object.prototype.hasOwnProperty.call(data, "success") && data.success === false) {
      throw new Error(data.message || "请求失败");
    }
    return data;
  }

  async function performPageTask(task, successText, errorText) {
    pageLoadingCount.value++;
    try {
      const result = await task();
      if (successText) {
        message.success(successText);
      }
      return result;
    } catch (error) {
      message.error(error.message || errorText || "请求失败");
      throw error;
    } finally {
      pageLoadingCount.value--;
    }
  }

  function markViewSuccess(view) {
    viewLoaded[view] = true;
    viewState[view].error = "";
    viewState[view].syncedAt = Date.now();
  }

  function markViewError(view, error) {
    viewState[view].error = error.message || "同步失败";
  }

  async function runViewLoad(view) {
    try {
      if (view === "dashboard") await fetchDashboard();
      if (view === "users") await fetchUsers();
      if (view === "whitelist") await fetchWhitelist();
      if (view === "bans") await fetchBans();
      if (view === "settings") await fetchSettings();
      if (view === "logs") await fetchLogs();
      markViewSuccess(view);
    } catch (error) {
      markViewError(view, error);
      throw error;
    }
  }

  async function fetchDashboard() {
    const data = await request("/guard/api/dashboard");
    Object.assign(dashboard, clone(data, dashboard));
  }

  async function fetchUsers() {
    const query = new URLSearchParams({
      page: String(userQuery.page),
      size: String(userQuery.size),
      search: userQuery.search || ""
    });
    const data = await request(`/guard/api/users?${query.toString()}`);
    users.value = data.items || [];
    userQuery.total = data.total || users.value.length;
  }

  async function fetchWhitelist() {
    const data = await request("/guard/api/whitelist");
    whitelist.value = data.items || [];
  }

  async function fetchBans() {
    const [active, history] = await Promise.all([request("/guard/api/bans?status=active"), request("/guard/api/bans?status=all")]);
    activeBans.value = active.items || [];
    banHistory.value = history.items || [];
  }

  async function fetchSettings() {
    const data = await request("/guard/api/settings");
    const payload = data.data || {};
    Object.assign(settingsModel, clone(payload, {}));
    syncAllowedUADraft(payload.allowed_ua);
    settingsText.allowed_origins = formatMaybeArray(payload.allowed_origins);
    settingsText.oauth_allowed_redirect_uris = formatMaybeArray(payload.oauth_allowed_redirect_uris);
    settingsText.discord_oauth_scopes = formatMaybeArray(payload.discord_oauth_scopes);
    syncDiscordAccessPolicyDraft(payload.discord_access_policy);
  }

  async function fetchLogs() {
    await Promise.all([fetchBanLogs(), fetchCheckinLogs(), fetchStatsLogs()]);
  }

  async function fetchBanLogs() {
    const data = await request("/guard/api/logs/bans?page=1&size=50");
    banLogs.value = data.items || [];
  }

  async function fetchCheckinLogs() {
    const params = new URLSearchParams({ page: "1", size: "50" });
    if (checkinFilter.value) {
      params.set("user_id", checkinFilter.value);
    }
    const data = await request(`/guard/api/logs/checkins?${params.toString()}`);
    checkinLogs.value = data.items || [];
  }

  async function fetchStatsLogs() {
    const data = await request("/guard/api/logs/stats?days=30");
    statsLogs.value = data.items || [];
  }

  // ─── Actions ─────────────────────────────────────────────
  async function submitLogin() {
    if (!login.password) {
      login.notice = "请输入管理密码。";
      return;
    }
    authLoading.value = true;
    try {
      const data = await request("/guard/api/auth/login", {
        method: "POST",
        body: { password: login.password }
      });
      auth.token = data.token;
      localStorage.setItem("guard_token", auth.token);
      login.notice = "登录成功，正在同步控制台数据。";
      await refreshAll();
    } catch (error) {
      login.notice = error.message || "登录失败";
      message.error(login.notice);
    } finally {
      authLoading.value = false;
    }
  }

  async function logout(silent = false) {
    if (auth.token) {
      try {
        await fetch("/guard/api/auth/logout", {
          method: "POST",
          headers: { Authorization: `Bearer ${auth.token}` }
        });
      } catch {
        // 忽略退出时的网络波动
      }
    }
    auth.token = "";
    login.password = "";
    localStorage.removeItem("guard_token");
    Object.keys(viewLoaded).forEach((key) => {
      viewLoaded[key] = false;
      viewState[key].error = "";
      viewState[key].syncedAt = 0;
    });
    users.value = [];
    whitelist.value = [];
    activeBans.value = [];
    banHistory.value = [];
    banLogs.value = [];
    checkinLogs.value = [];
    statsLogs.value = [];
    if (!silent) {
      message.success("已退出控制台");
    }
  }

  function openView(view) {
    activeView.value = view;
  }

  async function refreshAll() {
    await performPageTask(async () => {
      const views = ["dashboard", "users", "whitelist", "bans", "settings", "logs"];
      const results = await Promise.allSettled(views.map((view) => runViewLoad(view)));
      const failed = results.filter((item) => item.status === "rejected");
      if (failed.length) {
        throw new Error(`有 ${failed.length} 个视图同步失败，请查看对应错误提示`);
      }
    }, "已完成全量同步");
  }

  async function refreshCurrentView() {
    await performPageTask(async () => {
      await runViewLoad(activeView.value);
    }, "当前视图已刷新");
  }

  async function ensureView(view) {
    if (!auth.token || viewLoaded[view]) return;
    await runViewLoad(view);
  }

  async function applyUserSearch() {
    userQuery.page = 1;
    userQuery.search = userSearchDraft.value.trim();
    await performPageTask(async () => {
      await runViewLoad("users");
    }, "用户列表已更新");
  }

  async function handleUserPageChange(page) {
    userQuery.page = page;
    await performPageTask(async () => {
      await runViewLoad("users");
    });
  }

  async function handleUserPageSizeChange(size) {
    userQuery.size = size;
    userQuery.page = 1;
    await performPageTask(async () => {
      await runViewLoad("users");
    });
  }

  async function resetUserSearch() {
    userSearchDraft.value = "";
    userQuery.search = "";
    userQuery.page = 1;
    await performPageTask(async () => {
      await runViewLoad("users");
    }, "已清空搜索条件");
  }

  async function refreshLogsOnly() {
    await performPageTask(async () => {
      await runViewLoad("logs");
    }, "日志已刷新");
  }

  async function loadCheckinLogsAction() {
    await performPageTask(async () => {
      await fetchCheckinLogs();
      markViewSuccess("logs");
    }, "签到日志已刷新");
  }

  async function toggleWhitelist(item) {
    const method = item.is_whitelist ? "DELETE" : "POST";
    await performPageTask(async () => {
      await request(`/guard/api/whitelist/${item.newapi_user_id}`, { method });
      await Promise.all([runViewLoad("users"), runViewLoad("whitelist"), runViewLoad("dashboard")]);
    }, item.is_whitelist ? "已移出白名单" : "已加入白名单");
  }

  function resetCreateUserModal() {
    createUserModal.form.mode = "password";
    createUserModal.form.username = "";
    createUserModal.form.password = "";
    createUserModal.form.discord_id = "";
    createUserModal.form.discord_name = "";
    createUserModal.form.initial_quota = 0;
    createUserModal.form.is_whitelist = false;
  }

  function openCreateUserModal() {
    resetCreateUserModal();
    createUserModal.show = true;
  }

  async function submitCreateUser() {
    createUserModal.loading = true;
    try {
      await request("/guard/api/users", {
        method: "POST",
        body: clone(createUserModal.form, {})
      });
      createUserModal.show = false;
      message.success("用户已创建");
      await Promise.all([runViewLoad("users"), runViewLoad("whitelist"), runViewLoad("dashboard")]);
    } catch (error) {
      message.error(error.message || "创建失败");
    } finally {
      createUserModal.loading = false;
    }
  }

  function resetBanModal() {
    banModal.form.user_ref = "";
    banModal.form.newapi_user_id = null;
    banModal.form.discord_id = "";
    banModal.form.reason = "";
    banModal.form.duration = "permanent";
  }

  function openBanModal(item = null) {
    resetBanModal();
    if (item) {
      banModal.form.newapi_user_id = item.newapi_user_id || null;
      banModal.form.user_ref = item.discord_id || String(item.newapi_user_id || "");
      banModal.form.discord_id = item.discord_id || "";
      banModal.form.reason = item.reason && item.reason !== "无上下文（可能直接在 New API 后台封禁）" ? item.reason : "";
    }
    banModal.show = true;
  }

  async function submitBan() {
    if (!banModal.form.reason) {
      message.warning("请先填写封禁原因");
      return;
    }
    banModal.loading = true;
    try {
      await request("/guard/api/bans", {
        method: "POST",
        body: {
          user_ref: banModal.form.user_ref,
          newapi_user_id: banModal.form.newapi_user_id,
          discord_id: banModal.form.discord_id,
          reason: banModal.form.reason,
          duration: banModal.form.duration
        }
      });
      banModal.show = false;
      message.success("封禁操作已提交");
      await Promise.all([runViewLoad("bans"), runViewLoad("dashboard"), runViewLoad("users"), fetchBanLogs().then(() => markViewSuccess("logs"))]);
    } catch (error) {
      message.error(error.message || "封禁失败");
    } finally {
      banModal.loading = false;
    }
  }

  async function quickUnban(item) {
    const payload = {
      user_ref: item.discord_id || String(item.newapi_user_id || ""),
      newapi_user_id: item.newapi_user_id,
      discord_id: item.discord_id || ""
    };

    dialog.warning({
      title: "确认解除封禁",
      content: `将对 ${item.display_name || item.username || item.newapi_user_id} 执行解除封禁操作。`,
      positiveText: "确认解除",
      negativeText: "取消",
      onPositiveClick: async () => {
        try {
          await request("/guard/api/bans/unban", {
            method: "POST",
            body: payload
          });
          message.success("已解除封禁");
          await Promise.all([runViewLoad("bans"), runViewLoad("dashboard"), runViewLoad("users"), fetchBanLogs().then(() => markViewSuccess("logs"))]);
        } catch (error) {
          message.error(error.message || "解除封禁失败");
        }
      }
    });
  }

  async function saveSettings() {
    try {
      const payload = clone(settingsModel, {});
      validatePolicyGroup(discordAccessPolicyDraft.value);
      payload.allowed_ua = [...allowedUADraft.items];
      payload.allowed_origins = settingsText.allowed_origins
        .split(/\n+/)
        .map((item) => item.trim())
        .filter(Boolean);
      payload.oauth_allowed_redirect_uris = settingsText.oauth_allowed_redirect_uris
        .split(/\n+/)
        .map((item) => item.trim())
        .filter(Boolean);
      payload.discord_oauth_scopes = settingsText.discord_oauth_scopes
        .split(/\n+/)
        .map((item) => item.trim())
        .filter(Boolean);
      payload.discord_access_policy = serializePolicyGroup(discordAccessPolicyDraft.value);

      await performPageTask(async () => {
        await request("/guard/api/settings", { method: "PUT", body: payload });
        await Promise.all([runViewLoad("settings"), runViewLoad("dashboard")]);
      }, "设置已保存");
    } catch (error) {
      message.error(error.message || "设置保存失败");
    }
  }

  function resetSettingsDraft() {
    syncAllowedUADraft(settingsModel.allowed_ua);
    settingsText.allowed_origins = formatMaybeArray(settingsModel.allowed_origins);
    settingsText.oauth_allowed_redirect_uris = formatMaybeArray(settingsModel.oauth_allowed_redirect_uris);
    settingsText.discord_oauth_scopes = formatMaybeArray(settingsModel.discord_oauth_scopes);
    syncDiscordAccessPolicyDraft(settingsModel.discord_access_policy);
    message.info("已恢复为当前已加载配置");
  }

  function switchBanMode(mode) {
    banMode.value = mode;
  }

  function trendWidth(value) {
    const max = Math.max(...statsLogs.value.map((item) => Number(item.total_requests || 0)), 1);
    return `${Math.max(8, Math.round((Number(value || 0) / max) * 100))}%`;
  }

  // ─── Watchers & lifecycle ────────────────────────────────
  let colorSchemeMediaQuery;
  let removeColorSchemeListener;

  watch(themeMode, (mode) => {
    localStorage.setItem("guard_theme_mode", mode);
  });

  watch(
    resolvedTheme,
    (theme) => {
      applyResolvedTheme(theme);
    },
    { immediate: true }
  );

  watch(activeView, (view) => {
    ensureView(view).catch((error) => {
      message.error(error.message || "视图加载失败");
    });
  });

  onMounted(async () => {
    if (typeof window !== "undefined" && typeof window.matchMedia === "function") {
      colorSchemeMediaQuery = window.matchMedia("(prefers-color-scheme: dark)");
      const syncSystemTheme = () => {
        systemTheme.value = colorSchemeMediaQuery.matches ? "dark" : "light";
      };
      syncSystemTheme();
      if (typeof colorSchemeMediaQuery.addEventListener === "function") {
        colorSchemeMediaQuery.addEventListener("change", syncSystemTheme);
        removeColorSchemeListener = () => colorSchemeMediaQuery.removeEventListener("change", syncSystemTheme);
      } else if (typeof colorSchemeMediaQuery.addListener === "function") {
        colorSchemeMediaQuery.addListener(syncSystemTheme);
        removeColorSchemeListener = () => colorSchemeMediaQuery.removeListener(syncSystemTheme);
      }
    }

    userSearchDraft.value = userQuery.search;
    if (auth.token) {
      try {
        await refreshAll();
      } catch (err) {
        if (!auth.token) return;
        message.warning(err.message || "加载数据失败，请刷新页面重试");
      }
    }
  });

  onBeforeUnmount(() => {
    removeColorSchemeListener?.();
  });

  // ─── Return ──────────────────────────────────────────────
  return {
    // Theme
    themeOptions,
    themeMode,
    systemTheme,
    resolvedTheme,
    currentThemeLabel,
    naiveTheme,
    themeOverrides,

    // Navigation
    navItems,
    sectionMap,
    sectionMeta,
    activeView,
    sidebarCollapsed,
    openView,
    navCount,

    // Auth
    auth,
    login,
    authLoading,
    submitLogin,
    logout,

    // UI state
    pageLoading,
    logView,
    banMode,
    banFilter,
    checkinFilter,
    userSearchDraft,
    viewLoaded,
    viewState,

    // Data
    dashboard,
    userQuery,
    users,
    whitelist,
    activeBans,
    banHistory,
    banLogs,
    checkinLogs,
    statsLogs,

    // Settings
    settingsModel,
    settingsText,
    allowedUADraft,
    discordAccessPolicyDraft,

    // Modals
    helpModal,
    helpContent,
    createUserModal,
    banModal,
    banDurationOptions,
    userPageSizeOptions,

    // Computed
    displayedBans,
    userPageCount,
    activeBansWithContext,
    currentViewError,
    currentViewSyncedAt,
    currentViewHealth,
    filteredBans,

    // Helpers
    parseDate,
    formatNumber,
    formatQuotaUSD,
    trendWidth,
    openHelp,

    // Actions
    refreshAll,
    refreshCurrentView,
    applyUserSearch,
    handleUserPageChange,
    handleUserPageSizeChange,
    resetUserSearch,
    refreshLogsOnly,
    loadCheckinLogsAction,
    toggleWhitelist,
    openCreateUserModal,
    submitCreateUser,
    openBanModal,
    submitBan,
    quickUnban,
    saveSettings,
    resetSettingsDraft,
    switchBanMode,

    // Allowed UA draft actions
    addAllowedUAItem,
    startAllowedUAEdit,
    cancelAllowedUAEdit,
    saveAllowedUAEdit,
    removeAllowedUAItem
  };
}
