<template>
  <div class="admin-pt-users-page">
    <main-nav title="PT 用户与权限管理" />

    <n-card class="main-content-wrap user-table-card" size="small">
      <n-tabs v-model:value="activeTab" type="line" animated>
        <!-- Tab 1: 用户等级与状态列表 -->
        <n-tab-pane name="users" tab="👥 用户等级管理">
          <!-- 搜索与筛选工具栏 -->
          <div class="table-toolbar">
            <div class="toolbar-left">
              <n-input
                v-model:value="searchKeyword"
                placeholder="搜索用户名、昵称或 UID..."
                clearable
                style="width: 240px"
                @keyup.enter="handleSearch"
                @clear="handleSearch"
              >
                <template #prefix>
                  <n-icon><search-outline /></n-icon>
                </template>
              </n-input>

              <n-select
                v-model:value="selectedClassFilter"
                :options="filterClassOptions"
                placeholder="按等级筛选"
                clearable
                style="width: 160px"
                @update:value="handleSearch"
              />

              <n-button type="primary" secondary @click="handleSearch">
                <template #icon><n-icon><search-outline /></n-icon></template>
                查询
              </n-button>
            </div>

            <div class="toolbar-right">
              <n-button quaternary circle @click="loadUsers">
                <template #icon><n-icon><refresh-outline /></n-icon></template>
              </n-button>
            </div>
          </div>

          <!-- 用户数据表格 -->
          <n-data-table
            :columns="columns"
            :data="userList"
            :loading="loading"
            :pagination="pagination"
            :bordered="false"
            size="small"
            class="user-table"
            @update:page="handlePageChange"
          />
        </n-tab-pane>

        <!-- Tab 2: 晋升与考核阶梯指标自定义 -->
        <n-tab-pane name="promotion_rules" tab="📈 晋升与考核阶梯指标">
          <n-spin :show="rulesLoading">
            <div class="rules-container">
              <n-alert title="各阶段晋升与考核指标说明" type="info" style="margin-bottom: 20px">
                在此可自由配置普通会员由【进阶用户】逐级晋升至【Nexus 大师】的晋升考核门槛与降级触发线。每项指标（注册周数、最低下载量、晋升最低分享率、降级分享率、最低魔力值、首次晋升奖励邀请数）均可实时自定义调整。保存后，前台用户主页晋升目标进度与后台自动调度均即时生效！
              </n-alert>

              <div class="ladder-table-container">
                <table class="custom-ladder-table">
                  <thead>
                    <tr>
                      <th style="width: 140px">目标等级</th>
                      <th>注册时间要求</th>
                      <th>最低下载量</th>
                      <th>晋升最低分享率</th>
                      <th>降级触发分享率</th>
                      <th>最低做种魔力</th>
                      <th>赠送邀请数</th>
                      <th>推算最低上传</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="rule in rulesForm.promotion_rules" :key="rule.class">
                      <td>
                        <n-tag
                          size="small"
                          round
                          strong
                          :color="{ color: getClassColor(rule.class), textColor: '#fff' }"
                        >
                          {{ rule.class_name || getClassName(rule.class) }}
                        </n-tag>
                        <div class="class-num-hint">Class {{ rule.class }}</div>
                      </td>
                      <td>
                        <n-input-number
                          v-model:value="rule.min_weeks"
                          :min="0"
                          :step="1"
                          size="small"
                          style="width: 110px"
                        >
                          <template #suffix>周</template>
                        </n-input-number>
                      </td>
                      <td>
                        <n-input-number
                          v-model:value="rule.min_download_gb"
                          :min="0"
                          :step="10"
                          size="small"
                          style="width: 125px"
                        >
                          <template #suffix>GB</template>
                        </n-input-number>
                      </td>
                      <td>
                        <n-input-number
                          v-model:value="rule.min_ratio"
                          :min="0"
                          :step="0.05"
                          size="small"
                          style="width: 110px"
                        />
                      </td>
                      <td>
                        <n-input-number
                          v-model:value="rule.demote_ratio"
                          :min="0"
                          :step="0.05"
                          size="small"
                          style="width: 110px"
                        />
                      </td>
                      <td>
                        <n-input-number
                          v-model:value="rule.min_bonus"
                          :min="0"
                          :step="100"
                          size="small"
                          style="width: 125px"
                        >
                          <template #suffix>点</template>
                        </n-input-number>
                      </td>
                      <td>
                        <n-input-number
                          v-model:value="rule.award_invites"
                          :min="0"
                          :step="1"
                          size="small"
                          style="width: 100px"
                        >
                          <template #suffix>个</template>
                        </n-input-number>
                      </td>
                      <td>
                        <span class="est-upload-val">
                          {{ (rule.min_download_gb * rule.min_ratio).toFixed(1) }} GB
                        </span>
                      </td>
                    </tr>
                  </tbody>
                </table>

                <div class="ladder-save-row">
                  <n-button type="primary" size="large" :loading="rulesSaving" @click="handleSaveAuthorityRules">
                    💾 保存晋升考核阶梯设置
                  </n-button>
                  <span class="save-tip">保存后即时热更新生效，前台用户卡片与晋升后台计算均实时应用</span>
                </div>
              </div>
            </div>
          </n-spin>
        </n-tab-pane>

        <!-- Tab 3: 行为权限与特权全局设置 -->
        <n-tab-pane name="authority_rules" tab="⚙️ 行为门槛与特权设置">
          <n-spin :show="rulesLoading">
            <div class="rules-container">
              <n-alert title="权限体系说明" type="info" style="margin-bottom: 20px">
                对标 NexusPHP 权限架构体系。在此可自定义各项 PT 关键行为的最低等级门槛、养老退役与 VIP 专属特权、以及高阶会员做种魔力（Seed Bonus）阶梯加成。修改后全站毫秒级热更新生效，无需重启服务。
              </n-alert>

              <n-form label-placement="left" label-width="220px" :model="rulesForm" style="max-width: 780px">
                <div class="rules-section-title">🚦 行为准入门槛</div>

                <n-form-item label="最低下载与播放等级">
                  <n-select
                    v-model:value="rulesForm.min_class_download"
                    :options="classSelectOptions"
                    style="width: 280px"
                  />
                  <span class="field-hint">低于此等级的用户禁止下载种子与网页播放（默认普通会员 Class 1）</span>
                </n-form-item>

                <n-form-item label="最低发种等级">
                  <n-select
                    v-model:value="rulesForm.min_class_upload"
                    :options="classSelectOptions"
                    style="width: 280px"
                  />
                  <span class="field-hint">低于此等级的用户禁止发布种子（默认普通会员 Class 1）</span>
                </n-form-item>

                <n-form-item label="最低自定义促销等级">
                  <n-select
                    v-model:value="rulesForm.min_class_promotion"
                    :options="classSelectOptions"
                    style="width: 280px"
                  />
                  <span class="field-hint">允许自由为种子设定 Free/2X 等促销状态的最低等级（默认发布员 Class 12）</span>
                </n-form-item>

                <n-form-item label="最低匿名发种等级">
                  <n-select
                    v-model:value="rulesForm.min_class_anonymous"
                    :options="classSelectOptions"
                    style="width: 280px"
                  />
                  <span class="field-hint">允许发布种子时隐藏个人身份的最低等级（默认狂热用户 Class 4）</span>
                </n-form-item>

                <n-form-item label="最低查看 Peer 节点等级">
                  <n-select
                    v-model:value="rulesForm.min_class_view_peers"
                    :options="classSelectOptions"
                    style="width: 280px"
                  />
                  <span class="field-hint">允许在种子详情页查看 Swarm 详细 Peer 客户端列表的等级（默认进阶用户 Class 2）</span>
                </n-form-item>

                <n-divider style="margin: 20px 0" />

                <div class="rules-section-title">☕ 养老退役组 (Retiree, Class 11) 特权</div>

                <n-form-item label="养老免除上传贡献 (0x下载)">
                  <n-switch v-model:value="rulesForm.retiree_free_upload">
                    <template #checked>已开启 (0x 下载计费)</template>
                    <template #unchecked>已关闭 (正常计费)</template>
                  </n-switch>
                  <span class="field-hint">开启后，退役老管理下载任何种子均不计入下载量，享受永久免费下载特权</span>
                </n-form-item>

                <n-form-item label="养老终生免除考核降级">
                  <n-switch v-model:value="rulesForm.retiree_exempt_demote">
                    <template #checked>永久免受降级</template>
                    <template #unchecked>按常规指标降级</template>
                  </n-switch>
                  <span class="field-hint">无论退役老管理的分享率如何变化，系统均不会将其降级为贫民</span>
                </n-form-item>

                <n-divider style="margin: 20px 0" />

                <div class="rules-section-title">💎 做种魔力值 (Seed Bonus / Karma) 阶梯加成</div>

                <n-form-item label="官方发布员 (Uploader) 魔力倍率">
                  <n-input-number
                    v-model:value="rulesForm.uploader_karma_multi"
                    :min="1.0"
                    :max="10.0"
                    :step="0.1"
                    style="width: 160px"
                  />
                  <span class="field-hint">官方发布员做种时获得的魔力倍率（默认 1.5x）</span>
                </n-form-item>

                <n-form-item label="贵宾会员 (VIP) 魔力倍率">
                  <n-input-number
                    v-model:value="rulesForm.vip_karma_multi"
                    :min="1.0"
                    :max="10.0"
                    :step="0.1"
                    style="width: 160px"
                  />
                  <span class="field-hint">VIP 组会员做种获得的魔力倍率（默认 1.2x）</span>
                </n-form-item>

                <n-form-item label="Nexus 大师 (Master) 魔力倍率">
                  <n-input-number
                    v-model:value="rulesForm.master_karma_multi"
                    :min="1.0"
                    :max="10.0"
                    :step="0.1"
                    style="width: 160px"
                  />
                  <span class="field-hint">最高普通阶级 (Class 9) 做种魔力倍率（默认 1.3x）</span>
                </n-form-item>

                <n-form-item label="巅峰/极限会员 魔力倍率">
                  <n-input-number
                    v-model:value="rulesForm.ultimate_karma_multi"
                    :min="1.0"
                    :max="10.0"
                    :step="0.1"
                    style="width: 160px"
                  />
                  <span class="field-hint">Class 7 (Extreme) 及 Class 8 (Ultimate) 做种魔力倍率（默认 1.2x）</span>
                </n-form-item>

                <n-form-item label="资深/疯狂会员 魔力倍率">
                  <n-input-number
                    v-model:value="rulesForm.veteran_karma_multi"
                    :min="1.0"
                    :max="10.0"
                    :step="0.1"
                    style="width: 160px"
                  />
                  <span class="field-hint">Class 5 (Insane) 及 Class 6 (Veteran) 做种魔力倍率（默认 1.1x）</span>
                </n-form-item>

                <div style="margin-top: 24px; padding-left: 220px">
                  <n-button type="primary" size="large" :loading="rulesSaving" @click="handleSaveAuthorityRules">
                    💾 保存行为与特权设置
                  </n-button>
                </div>
              </n-form>
            </div>
          </n-spin>
        </n-tab-pane>
      </n-tabs>
    </n-card>

    <!-- 设置等级与权限弹窗 -->
    <n-modal
      v-model:show="showEditModal"
      preset="card"
      title="设置用户 PT 等级与权限"
      style="width: 520px; max-width: 92vw"
    >
      <div v-if="currentUser" class="edit-modal-content">
        <!-- 目标用户简介 -->
        <div class="target-user-summary">
          <n-avatar :size="48" :src="currentUser.avatar" round />
          <div class="user-meta">
            <div class="name-line">
              <strong>{{ currentUser.nickname }}</strong>
              <span class="user-handle">@{{ currentUser.username }}</span>
              <n-tag v-if="currentUser.is_admin" size="tiny" type="error" round>管理员</n-tag>
            </div>
            <div class="id-line">UID: {{ currentUser.user_id }} · 分享率: {{ currentUser.ratio >= 999 ? '∞' : currentUser.ratio.toFixed(2) }}</div>
          </div>
        </div>

        <n-divider style="margin: 14px 0" />

        <!-- 表单项 -->
        <n-form label-placement="left" label-width="100px">
          <n-form-item label="PT 等级">
            <n-select
              v-model:value="editForm.class"
              :options="classSelectOptions"
              style="width: 100%"
            />
          </n-form-item>

          <n-form-item label="自定义头衔">
            <n-input
              v-model:value="editForm.custom_title"
              placeholder="例如：开山鼻祖、筑基狂人（留空不设）"
              maxlength="20"
              show-count
            />
          </n-form-item>

          <n-form-item label="VIP 期限设置" v-if="editForm.class === 10 || currentUser.class === 10">
            <n-select
              v-model:value="editForm.vip_days"
              :options="vipDaysOptions"
              style="width: 100%"
            />
          </n-form-item>

          <n-form-item label="发种权限">
            <n-switch v-model:value="editForm.can_upload">
              <template #checked>允许发种</template>
              <template #unchecked>禁止发种 (受限)</template>
            </n-switch>
          </n-form-item>

          <n-form-item label="下载/做种权限">
            <n-switch v-model:value="editForm.can_leech">
              <template #checked>允许下载</template>
              <template #unchecked>禁止下载 (受限)</template>
            </n-switch>
          </n-form-item>
        </n-form>
      </div>

      <template #footer>
        <div style="display: flex; justify-content: flex-end; gap: 10px">
          <n-button @click="showEditModal = false">取消</n-button>
          <n-button type="primary" :loading="saving" @click="handleSaveUser">
            保存修改
          </n-button>
        </div>
      </template>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, h } from 'vue';
import {
  NCard,
  NTabs,
  NTabPane,
  NInput,
  NInputNumber,
  NSelect,
  NButton,
  NIcon,
  NDataTable,
  NModal,
  NForm,
  NFormItem,
  NSwitch,
  NAvatar,
  NTag,
  NDivider,
  NAlert,
  NSpin,
  useMessage,
  type DataTableColumns,
} from 'naive-ui';
import { SearchOutline, RefreshOutline, CreateOutline } from '@vicons/ionicons5';
import {
  adminGetUsers,
  adminGetClassOptions,
  adminUpdateUser,
  adminGetAuthorityRules,
  adminSaveAuthorityRules,
  type AdminPTUserItem,
  type AdminClassOption,
  type PTAuthorityRules,
} from '@/api/pt';

const message = useMessage();

const activeTab = ref('users');
const loading = ref(false);
const saving = ref(false);
const rulesLoading = ref(false);
const rulesSaving = ref(false);

const searchKeyword = ref('');
const selectedClassFilter = ref<number | null>(null);

const userList = ref<AdminPTUserItem[]>([]);
const classOptions = ref<AdminClassOption[]>([]);
const totalUsers = ref(0);
const currentPage = ref(1);
const pageSize = ref(15);

const showEditModal = ref(false);
const currentUser = ref<AdminPTUserItem | null>(null);

const editForm = reactive({
  user_id: 0,
  class: 1,
  custom_title: '',
  vip_days: 0,
  can_upload: true,
  can_leech: true,
});

const rulesForm = reactive<PTAuthorityRules>({
  min_class_download: 1,
  min_class_upload: 1,
  min_class_promotion: 12,
  min_class_anonymous: 4,
  min_class_view_peers: 2,
  retiree_free_upload: true,
  retiree_exempt_demote: true,
  uploader_karma_multi: 1.5,
  vip_karma_multi: 1.2,
  veteran_karma_multi: 1.1,
  ultimate_karma_multi: 1.2,
  master_karma_multi: 1.3,
  promotion_rules: [],
});

const vipDaysOptions = [
  { label: '保持当前不变', value: 0 },
  { label: '赋予 7 天 VIP', value: 7 },
  { label: '赋予 30 天 VIP (推荐)', value: 30 },
  { label: '赋予 90 天 VIP', value: 90 },
  { label: '赋予 365 天 VIP', value: 365 },
  { label: '清除 VIP 资格 (恢复原等级)', value: -1 },
];

const getClassColor = (c: number) => {
  const found = classOptions.value.find((item) => item.class === c);
  return found?.color || '#1890ff';
};

const getClassName = (c: number) => {
  const found = classOptions.value.find((item) => item.class === c);
  return found?.name || `等级 ${c}`;
};

const filterClassOptions = computed(() => {
  return classOptions.value.map((c) => ({
    label: `${c.name} (${c.class})`,
    value: c.class,
  }));
});

const classSelectOptions = computed(() => {
  return classOptions.value.map((c) => ({
    label: `${c.name} (Class ${c.class})`,
    value: c.class,
  }));
});

const pagination = computed(() => ({
  page: currentPage.value,
  pageSize: pageSize.value,
  itemCount: totalUsers.value,
  showSizePicker: false,
  prefix: () => `共 ${totalUsers.value} 位会员`,
}));

const formatBytes = (bytes: number) => {
  if (!bytes || bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
};

const columns: DataTableColumns<AdminPTUserItem> = [
  {
    title: '用户',
    key: 'user',
    width: 220,
    render(row) {
      return h('div', { class: 'table-user-cell' }, [
        h(NAvatar, { size: 36, src: row.avatar, round: true }),
        h('div', { class: 'meta' }, [
          h('div', { class: 'nickname-row' }, [
            h('span', { class: 'nickname' }, row.nickname),
            row.is_admin ? h(NTag, { size: 'tiny', type: 'error', round: true, style: 'margin-left: 4px' }, { default: () => '管理' }) : null,
          ]),
          h('div', { class: 'username-row' }, `@${row.username} · UID ${row.user_id}`),
        ]),
      ]);
    },
  },
  {
    title: 'PT 等级',
    key: 'class',
    width: 140,
    render(row) {
      return h(
        NTag,
        {
          size: 'small',
          round: true,
          strong: true,
          color: { color: row.class_color, textColor: '#fff' },
        },
        {
          default: () => (row.class === 10 ? `👑 ${row.class_name}` : row.class_name),
        }
      );
    },
  },
  {
    title: '自定义头衔',
    key: 'custom_title',
    width: 120,
    render(row) {
      if (row.custom_title) {
        return h(NTag, { size: 'small', type: 'info', round: true, secondary: true }, { default: () => row.custom_title });
      }
      return h('span', { style: 'color: #bbb' }, '-');
    },
  },
  {
    title: '分享率',
    key: 'ratio',
    width: 100,
    render(row) {
      const isWarn = row.downloaded > 0 && row.ratio < 1.0;
      const valStr = row.downloaded === 0 ? '∞' : row.ratio.toFixed(2);
      return h(
        'strong',
        {
          style: {
            color: isWarn ? '#d03050' : '#18a058',
          },
        },
        valStr
      );
    },
  },
  {
    title: '上传 / 下载',
    key: 'traffic',
    width: 160,
    render(row) {
      return h('div', { style: 'font-size: 12px; line-height: 1.4' }, [
        h('div', { style: 'color: #18a058' }, `↑ ${formatBytes(row.uploaded)}`),
        h('div', { style: 'color: #d03050' }, `↓ ${formatBytes(row.downloaded)}`),
      ]);
    },
  },
  {
    title: '魔力值',
    key: 'seed_bonus',
    width: 100,
    render(row) {
      return h('span', { style: 'color: #f0a020; font-weight: 600' }, row.seed_bonus.toFixed(1));
    },
  },
  {
    title: '状态',
    key: 'permissions',
    width: 120,
    render(row) {
      const tags = [];
      if (!row.can_upload) {
        tags.push(h(NTag, { size: 'tiny', type: 'error', round: true }, { default: () => '禁发种' }));
      }
      if (!row.can_leech) {
        tags.push(h(NTag, { size: 'tiny', type: 'error', round: true }, { default: () => '禁下载' }));
      }
      if (row.class === 10 && row.vip_until > 0) {
        const days = Math.max(0, Math.ceil((row.vip_until - Date.now() / 1000) / 86400));
        tags.push(h(NTag, { size: 'tiny', type: 'warning', round: true }, { default: () => `VIP ${days}天` }));
      }
      if (tags.length === 0) {
        return h(NTag, { size: 'tiny', type: 'success', round: true }, { default: () => '正常' });
      }
      return h('div', { style: 'display: flex; gap: 4px; flex-wrap: wrap' }, tags);
    },
  },
  {
    title: '操作',
    key: 'action',
    width: 110,
    render(row) {
      return h(
        NButton,
        {
          size: 'small',
          type: 'primary',
          secondary: true,
          onClick: () => openEditModal(row),
        },
        {
          icon: () => h(NIcon, null, { default: () => h(CreateOutline) }),
          default: () => '编辑',
        }
      );
    },
  },
];

const loadUsers = async () => {
  loading.value = true;
  try {
    const res = await adminGetUsers({
      keyword: searchKeyword.value || undefined,
      class: selectedClassFilter.value !== null ? selectedClassFilter.value : undefined,
      page: currentPage.value,
      page_size: pageSize.value,
    });
    userList.value = res.list || [];
    totalUsers.value = res.total || 0;
  } catch (e: any) {
    message.error(e.message || '加载用户列表失败');
  } finally {
    loading.value = false;
  }
};

const loadClassOptions = async () => {
  try {
    const res = await adminGetClassOptions();
    classOptions.value = res || [];
  } catch (e) {
    // ignore
  }
};

const loadAuthorityRules = async () => {
  rulesLoading.value = true;
  try {
    const res = await adminGetAuthorityRules();
    if (res) {
      Object.assign(rulesForm, res);
      rulesForm.promotion_rules = res.promotion_rules || [];
    }
  } catch (e: any) {
    message.error(e.message || '加载权限规则配置失败');
  } finally {
    rulesLoading.value = false;
  }
};

const handleSaveAuthorityRules = async () => {
  rulesSaving.value = true;
  try {
    await adminSaveAuthorityRules(rulesForm);
    message.success('配置保存成功，已即时全站生效！');
  } catch (e: any) {
    message.error(e.message || '保存配置失败');
  } finally {
    rulesSaving.value = false;
  }
};

const handleSearch = () => {
  currentPage.value = 1;
  loadUsers();
};

const handlePageChange = (page: number) => {
  currentPage.value = page;
  loadUsers();
};

const openEditModal = (row: AdminPTUserItem) => {
  currentUser.value = row;
  editForm.user_id = row.user_id;
  editForm.class = row.class;
  editForm.custom_title = row.custom_title || '';
  editForm.vip_days = 0;
  editForm.can_upload = row.can_upload !== false;
  editForm.can_leech = row.can_leech !== false;
  showEditModal.value = true;
};

const handleSaveUser = async () => {
  if (!editForm.user_id) return;
  saving.value = true;
  try {
    await adminUpdateUser({
      user_id: editForm.user_id,
      class: editForm.class,
      custom_title: editForm.custom_title,
      vip_days: editForm.vip_days,
      can_upload: editForm.can_upload,
      can_leech: editForm.can_leech,
    });
    message.success('用户 PT 等级与权限设置成功');
    showEditModal.value = false;
    loadUsers();
  } catch (e: any) {
    message.error(e.message || '保存失败');
  } finally {
    saving.value = false;
  }
};

onMounted(() => {
  loadClassOptions();
  loadUsers();
  loadAuthorityRules();
});
</script>

<style lang="less" scoped>
.admin-pt-users-page {
  .user-table-card {
    padding: 12px;
  }

  .table-toolbar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 14px;
    flex-wrap: wrap;
    gap: 10px;

    .toolbar-left {
      display: flex;
      align-items: center;
      gap: 10px;
      flex-wrap: wrap;
    }
  }

  .user-table {
    margin-top: 6px;

    :deep(.table-user-cell) {
      display: flex;
      align-items: center;
      gap: 10px;

      .meta {
        display: flex;
        flex-direction: column;
        line-height: 1.25;

        .nickname-row {
          display: flex;
          align-items: center;
          font-weight: 600;
          font-size: 13px;
        }

        .username-row {
          font-size: 11px;
          opacity: 0.65;
          margin-top: 2px;
        }
      }
    }
  }

  .rules-container {
    padding: 10px 4px 20px 4px;

    .rules-section-title {
      font-size: 15px;
      font-weight: 700;
      margin-bottom: 16px;
      color: var(--n-text-color-1, #262626);
    }

    .field-hint {
      margin-left: 12px;
      font-size: 12px;
      opacity: 0.65;
    }

    .ladder-table-container {
      overflow-x: auto;
      padding-bottom: 10px;
    }

    .custom-ladder-table {
      width: 100%;
      min-width: 860px;
      border-collapse: separate;
      border-spacing: 0;
      border: 1px solid var(--n-border-color, #efeff5);
      border-radius: 8px;
      overflow: hidden;

      th,
      td {
        padding: 10px 12px;
        text-align: left;
        border-bottom: 1px solid var(--n-border-color, #efeff5);
        vertical-align: middle;
      }

      th {
        background: var(--n-card-color, rgba(0, 0, 0, 0.02));
        font-weight: 600;
        font-size: 13px;
        color: var(--n-text-color-1, #333);
        white-space: nowrap;
      }

      tbody tr:last-child td {
        border-bottom: none;
      }

      tbody tr:hover {
        background: rgba(0, 0, 0, 0.015);
      }

      .class-num-hint {
        font-size: 11px;
        opacity: 0.6;
        margin-top: 3px;
      }

      .est-upload-val {
        font-size: 13px;
        font-weight: 600;
        color: #18a058;
        white-space: nowrap;
      }
    }

    .ladder-save-row {
      margin-top: 24px;
      display: flex;
      gap: 16px;
      align-items: center;

      .save-tip {
        font-size: 12px;
        opacity: 0.65;
      }
    }
  }

  .edit-modal-content {
    .target-user-summary {
      display: flex;
      align-items: center;
      gap: 14px;

      .user-meta {
        .name-line {
          display: flex;
          align-items: center;
          gap: 6px;
          font-size: 15px;

          .user-handle {
            font-size: 12px;
            opacity: 0.65;
          }
        }

        .id-line {
          font-size: 12px;
          opacity: 0.65;
          margin-top: 3px;
        }
      }
    }
  }
}
</style>
