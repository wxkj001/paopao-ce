<template>
  <div class="torrents-page">
    <div class="header-section">
      <div class="title-row">
        <h2 class="title">PT 种子专区</h2>
        <div class="actions">
          <n-button type="primary" secondary @click="goTo('/pt/bonus')">
            <template #icon><n-icon><gift-outline /></n-icon></template>
            魔力商城
          </n-button>
          <n-button type="primary" @click="goTo('/pt/upload')">
            <template #icon><n-icon><cloud-upload-outline /></n-icon></template>
            发布种子
          </n-button>
        </div>
      </div>

      <!-- User PT Status Bar (if logged in) -->
      <div v-if="userProfile" class="user-pt-bar">
        <div class="stat-pill">
          <span class="label">分享率:</span>
          <span class="value ratio" :class="{ warning: userProfile.ratio < 1.0 }">
            {{ userProfile.ratio >= 0 ? userProfile.ratio.toFixed(2) : '∞' }}
          </span>
        </div>
        <div class="stat-pill">
          <span class="label">上传量:</span>
          <span class="value up">{{ formatSize(userProfile.uploaded) }}</span>
        </div>
        <div class="stat-pill">
          <span class="label">下载量:</span>
          <span class="value down">{{ formatSize(userProfile.downloaded) }}</span>
        </div>
        <div class="stat-pill">
          <span class="label">魔力值:</span>
          <span class="value bonus">{{ userProfile.seed_bonus.toFixed(1) }}</span>
        </div>
        <div class="stat-pill">
          <span class="label">当前做种:</span>
          <span class="value up">{{ userProfile.seeding_count }}</span>
        </div>
        <div class="stat-pill">
          <span class="label">当前下载:</span>
          <span class="value down">{{ userProfile.leech_count }}</span>
        </div>
        <div class="stat-pill passkey-box">
          <span class="label">Passkey:</span>
          <span class="value passkey" @click="copyPasskey">{{ maskedPasskey }}</span>
          <n-button text size="tiny" type="primary" @click="handleResetPasskey">重置</n-button>
        </div>
      </div>

      <!-- Filters Row -->
      <div class="filter-row">
        <div class="category-tabs">
          <n-button
            size="small"
            :type="filter.category_id === 0 ? 'primary' : 'default'"
            :quaternary="filter.category_id !== 0"
            @click="selectCategory(0)"
          >
            全部
          </n-button>
          <n-button
            v-for="cat in categories"
            :key="cat.id"
            size="small"
            :type="filter.category_id === cat.id ? 'primary' : 'default'"
            :quaternary="filter.category_id !== cat.id"
            @click="selectCategory(cat.id)"
          >
            {{ cat.name }}
          </n-button>
        </div>

        <div class="search-box">
          <n-input
            v-model:value="filter.keyword"
            placeholder="搜索种子标题或副标题..."
            clearable
            @keydown.enter="loadTorrents"
          >
            <template #prefix>
              <n-icon><search-outline /></n-icon>
            </template>
          </n-input>
          <n-select
            v-model:value="filter.sp_state"
            size="small"
            style="width: 130px; margin-left: 8px;"
            :options="spStateOptions"
            @update:value="loadTorrents"
          />
        </div>
      </div>
    </div>

    <!-- Torrents Table -->
    <div class="table-wrap">
      <n-spin :show="loading">
        <n-empty v-if="torrents.length === 0 && !loading" description="暂无符合条件的种子" style="margin: 40px 0" />

        <div v-else class="torrent-list">
          <div
            v-for="item in torrents"
            :key="item.id"
            class="torrent-card"
            @click="viewDetail(item.id)"
          >
            <div class="card-left">
              <div class="cat-badge">
                {{ item.category?.name || '综合' }}
              </div>
            </div>

            <div class="card-main">
              <div class="torrent-title">
                <span class="name">{{ item.name }}</span>
                <!-- Promotion Badge -->
                <span v-if="item.sp_state > 1" class="sp-badge" :class="'sp-' + item.sp_state">
                  {{ getSpStateLabel(item.sp_state) }}
                </span>
              </div>
              <div v-if="item.small_descr" class="small-descr">
                {{ item.small_descr }}
              </div>
              <div class="meta-row">
                <span class="uploader">
                  由 {{ item.is_anonymous ? '匿名用户' : (item.nickname || item.username || '用户') }} 发布
                </span>
                <span class="dot">·</span>
                <span class="time">{{ formatDate(item.created_at) }}</span>
              </div>
            </div>

            <div class="card-stats">
              <div class="stat-item size">
                <span class="val">{{ formatSize(item.size) }}</span>
                <span class="lbl">大小</span>
              </div>
              <div class="stat-item seeders">
                <span class="val">{{ item.seeders }}</span>
                <span class="lbl">做种</span>
              </div>
              <div class="stat-item leechers">
                <span class="val">{{ item.leechers }}</span>
                <span class="lbl">下载</span>
              </div>
              <div class="stat-item snatches">
                <span class="val">{{ item.times_completed }}</span>
                <span class="lbl">完成</span>
              </div>
            </div>

            <div class="card-actions" @click.stop>
              <n-button
                v-if="canDownloadTorrent(item)"
                type="primary"
                secondary
                size="small"
                @click="downloadTorrent(item.id)"
              >
                <template #icon><n-icon><arrow-down-circle-outline /></n-icon></template>
                下载
              </n-button>
            </div>
          </div>
        </div>

        <div v-if="total > filter.page_size" class="pagination-wrap">
          <n-pagination
            v-model:page="filter.page"
            :page-size="filter.page_size"
            :item-count="total"
            @update:page="loadTorrents"
          />
        </div>
      </n-spin>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, computed } from 'vue';
import { useRouter } from 'vue-router';
import {
  NButton,
  NIcon,
  NInput,
  NSelect,
  NSpin,
  NEmpty,
  NPagination,
  useMessage,
  useDialog,
} from 'naive-ui';
import {
  CloudUploadOutline,
  GiftOutline,
  SearchOutline,
  ArrowDownCircleOutline,
} from '@vicons/ionicons5';
import copy from 'copy-to-clipboard';
import moment from 'moment';
import {
  getPTCategories,
  getTorrents,
  getUserPTProfile,
  resetUserPasskey,
  getTorrentDownloadURL,
  PTCategory,
  TorrentItem,
  UserPTProfile,
} from '@/api/pt';
import { useStoreUser } from '@/store/user';

const router = useRouter();
const message = useMessage();
const dialog = useDialog();
const userStore = useStoreUser();

const loading = ref(false);
const categories = ref<PTCategory[]>([]);
const torrents = ref<TorrentItem[]>([]);
const total = ref(0);
const userProfile = ref<UserPTProfile | null>(null);

const filter = reactive({
  keyword: '',
  category_id: 0,
  sp_state: 0,
  sort_by: 'id',
  sort_order: 'desc',
  page: 1,
  page_size: 20,
});

const spStateOptions = [
  { label: '所有促销状态', value: 0 },
  { label: '普通 (1X)', value: 1 },
  { label: '免费 (Free)', value: 2 },
  { label: '双倍上传 (2X)', value: 3 },
  { label: '双倍免费 (2X Free)', value: 4 },
  { label: '半价 (50%)', value: 5 },
  { label: '双倍半价 (2X 50%)', value: 6 },
  { label: '30% 下载', value: 7 },
];

const maskedPasskey = computed(() => {
  if (!userProfile.value?.passkey) return '';
  const pk = userProfile.value.passkey;
  return pk.substring(0, 6) + '...' + pk.substring(pk.length - 6);
});

const goTo = (path: string) => {
  router.push(path);
};

const viewDetail = (id: number) => {
  router.push({ path: '/pt/detail', query: { id } });
};

const selectCategory = (catId: number) => {
  filter.category_id = catId;
  filter.page = 1;
  loadTorrents();
};

const getSpStateLabel = (sp: number) => {
  switch (sp) {
    case 2: return 'Free';
    case 3: return '2X';
    case 4: return '2X Free';
    case 5: return '50%';
    case 6: return '2X 50%';
    case 7: return '30%';
    default: return '';
  }
};

const formatSize = (bytes: number) => {
  if (!bytes || bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return (bytes / Math.pow(k, i)).toFixed(2) + ' ' + sizes[i];
};

const formatDate = (ts: number) => {
  if (!ts) return '';
  return moment(ts * 1000).format('YYYY-MM-DD HH:mm');
};

const copyPasskey = () => {
  if (userProfile.value?.passkey) {
    copy(userProfile.value.passkey);
    message.success('已复制 Passkey 到剪贴板');
  }
};

const handleResetPasskey = () => {
  dialog.warning({
    title: '重置 Passkey',
    content: '重置后，先前下载的所有种子客户端汇报都将失效，需要重新下载种子。确定重置吗？',
    positiveText: '确定重置',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        const res = await resetUserPasskey();
        if (userProfile.value) {
          userProfile.value.passkey = res.passkey;
        }
        message.success('Passkey 已成功重置');
      } catch (e: any) {
        message.error(e?.msg || '重置失败');
      }
    },
  });
};

const canDownloadTorrent = (item: TorrentItem) => {
  if (!userStore.userLogined) return false;
  if (userStore.userInfo?.is_admin) return true;
  const prof = userProfile.value || userStore.userPTProfile;
  if (!prof) return false;
  if (!prof.can_leech) return false;
  if (prof.can_download === false) return false;
  if (prof.class === 0) {
    return item.sp_state === 2 || item.sp_state === 4;
  }
  return true;
};

const downloadTorrent = (id: number) => {
  const url = getTorrentDownloadURL(id, userProfile.value?.passkey);
  window.open(url, '_blank');
};

const loadTorrents = async () => {
  loading.value = true;
  try {
    const res = await getTorrents(filter);
    torrents.value = res.list || [];
    total.value = res.total || 0;
  } catch (e) {
    console.error(e);
  } finally {
    loading.value = false;
  }
};

onMounted(async () => {
  try {
    categories.value = await getPTCategories();
  } catch (e) {}

  if (userStore.userInfo?.id > 0) {
    try {
      userProfile.value = await getUserPTProfile();
    } catch (e) {}
  }

  loadTorrents();
});
</script>

<style lang="less" scoped>
.torrents-page {
  padding: 16px;
  max-width: 1200px;
  margin: 0 auto;

  .header-section {
    background: var(--n-color);
    border-radius: 8px;
    padding: 16px;
    margin-bottom: 16px;
    box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);

    .title-row {
      display: flex;
      justify-content: space-between;
      align-items: center;
      margin-bottom: 12px;

      .title {
        margin: 0;
        font-size: 20px;
        font-weight: 600;
      }

      .actions {
        display: flex;
        gap: 8px;
      }
    }

    .user-pt-bar {
      display: flex;
      flex-wrap: wrap;
      gap: 12px;
      padding: 10px 14px;
      background: rgba(0, 0, 0, 0.02);
      border-radius: 6px;
      margin-bottom: 14px;
      font-size: 13px;

      .stat-pill {
        display: flex;
        align-items: center;
        gap: 6px;

        .label {
          color: #888;
        }

        .value {
          font-weight: 600;

          &.ratio {
            color: #18a058;
            &.warning {
              color: #f0a020;
            }
          }
          &.up {
            color: #18a058;
          }
          &.down {
            color: #d03050;
          }
          &.bonus {
            color: #8a2be2;
          }
          &.passkey {
            font-family: monospace;
            cursor: pointer;
            text-decoration: underline;
          }
        }
      }
    }

    .filter-row {
      display: flex;
      justify-content: space-between;
      align-items: center;
      flex-wrap: wrap;
      gap: 12px;

      .category-tabs {
        display: flex;
        gap: 6px;
        flex-wrap: wrap;
      }

      .search-box {
        display: flex;
        align-items: center;
        min-width: 320px;
      }
    }
  }

  .torrent-list {
    display: flex;
    flex-direction: column;
    gap: 8px;

    .torrent-card {
      display: flex;
      align-items: center;
      padding: 14px 16px;
      background: var(--n-color);
      border-radius: 8px;
      border: 1px solid rgba(0, 0, 0, 0.06);
      transition: all 0.2s;
      cursor: pointer;

      &:hover {
        border-color: #18a058;
        box-shadow: 0 2px 8px rgba(0, 0, 0, 0.06);
      }

      .card-left {
        margin-right: 14px;

        .cat-badge {
          padding: 4px 8px;
          border-radius: 4px;
          background: #eef5fe;
          color: #2080f0;
          font-size: 12px;
          font-weight: 500;
          white-space: nowrap;
        }
      }

      .card-main {
        flex: 1;
        min-width: 0;

        .torrent-title {
          display: flex;
          align-items: center;
          gap: 8px;

          .name {
            font-size: 15px;
            font-weight: 600;
            color: var(--n-text-color);
            overflow: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;
          }

          .sp-badge {
            font-size: 11px;
            padding: 1px 6px;
            border-radius: 4px;
            font-weight: 600;
            text-transform: uppercase;

            &.sp-2 {
              background: #e6f7ff;
              color: #1890ff;
              border: 1px solid #91d5ff;
            }
            &.sp-3 {
              background: #fff7e6;
              color: #fa8c16;
              border: 1px solid #ffd591;
            }
            &.sp-4 {
              background: #f6ffed;
              color: #52c41a;
              border: 1px solid #b7eb8f;
            }
            &.sp-5, &.sp-6, &.sp-7 {
              background: #fff0f6;
              color: #eb2f96;
              border: 1px solid #ffadd2;
            }
          }
        }

        .small-descr {
          font-size: 13px;
          color: #888;
          margin-top: 3px;
          overflow: hidden;
          text-overflow: ellipsis;
          white-space: nowrap;
        }

        .meta-row {
          display: flex;
          align-items: center;
          gap: 6px;
          font-size: 12px;
          color: #aaa;
          margin-top: 4px;

          .dot {
            font-weight: bold;
          }
        }
      }

      .card-stats {
        display: flex;
        gap: 16px;
        margin: 0 20px;
        text-align: center;

        .stat-item {
          display: flex;
          flex-direction: column;
          min-width: 48px;

          .val {
            font-size: 14px;
            font-weight: 600;
          }

          .lbl {
            font-size: 11px;
            color: #999;
          }

          &.size .val {
            color: #555;
          }
          &.seeders .val {
            color: #18a058;
          }
          &.leechers .val {
            color: #d03050;
          }
          &.snatches .val {
            color: #2080f0;
          }
        }
      }

      .card-actions {
        margin-left: 8px;
      }
    }
  }

  .pagination-wrap {
    display: flex;
    justify-content: center;
    margin-top: 20px;
  }
}
</style>
