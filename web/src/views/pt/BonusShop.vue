<template>
  <div>
    <main-nav title="魔力商城" />

    <n-list class="main-content-wrap bonus-page" bordered>
      <!-- User Karma Overview Card -->
      <div class="bonus-header-card">
        <div class="balance-display">
          <div class="label">当前可用魔力值 (Karma)</div>
          <div class="balance">{{ userProfile ? userProfile.seed_bonus.toFixed(1) : '0.0' }}</div>
        </div>
        <div class="intro">
          <p>做种时间越长、做种体积越大，每小时结算获得的魔力值奖励越丰厚。</p>
          <p>您可以在下方使用魔力值兑换真实上传量或邀请名额！</p>
        </div>
      </div>

      <!-- Bonus Exchange Options Grid -->
      <h3 class="section-title">魔力兑换商城</h3>
      <n-spin :show="loading">
        <div class="options-grid">
          <div
            v-for="opt in options"
            :key="opt.id"
            class="option-card"
          >
            <div class="card-icon">
              <n-icon size="36" color="#8a2be2">
                <gift-outline />
              </n-icon>
            </div>
            <div class="card-title">{{ opt.name }}</div>
            <div class="card-desc">{{ opt.description }}</div>
            <div class="card-price">
              <span class="pts">{{ opt.points }}</span>
              <span class="unit">魔力值</span>
            </div>
            <n-button
              type="primary"
              secondary
              block
              :disabled="!userProfile || userProfile.seed_bonus < opt.points"
              @click="handleExchange(opt)"
            >
              {{ (!userProfile || userProfile.seed_bonus < opt.points) ? '魔力值不足' : '立即兑换' }}
            </n-button>
          </div>
        </div>
      </n-spin>
    </n-list>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { useRouter } from 'vue-router';
import {
  NButton,
  NIcon,
  NSpin,
  useMessage,
  useDialog,
} from 'naive-ui';
import {
  ArrowBackOutline,
  GiftOutline,
} from '@vicons/ionicons5';
import {
  getBonusOptions,
  exchangeBonus,
  getUserPTProfile,
  BonusOption,
  UserPTProfile,
} from '@/api/pt';

const router = useRouter();
const message = useMessage();
const dialog = useDialog();

const loading = ref(false);
const options = ref<BonusOption[]>([]);
const userProfile = ref<UserPTProfile | null>(null);

const loadData = async () => {
  loading.value = true;
  try {
    const [opts, prof] = await Promise.all([
      getBonusOptions(),
      getUserPTProfile().catch(() => null),
    ]);
    options.value = opts || [];
    userProfile.value = prof;
  } catch (e) {
    console.error(e);
  } finally {
    loading.value = false;
  }
};

const handleExchange = (opt: BonusOption) => {
  dialog.info({
    title: '确认兑换',
    content: `您确定花费 ${opt.points} 魔力值兑换「${opt.name}」吗？`,
    positiveText: '确认兑换',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        await exchangeBonus(opt.id);
        message.success(`成功兑换: ${opt.name}`);
        loadData();
      } catch (e: any) {
        message.error(e?.msg || '兑换失败');
      }
    },
  });
};

onMounted(() => {
  loadData();
});
</script>

<style lang="less" scoped>
.bonus-page {
  padding: 16px;
  max-width: 900px;
  margin: 0 auto;

  .top-nav {
    margin-bottom: 14px;
  }

  .bonus-header-card {
    display: flex;
    justify-content: space-between;
    align-items: center;
    background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
    color: #fff;
    border-radius: 8px;
    padding: 24px 30px;
    margin-bottom: 24px;
    box-shadow: 0 4px 12px rgba(118, 75, 162, 0.2);

    .balance-display {
      .label {
        font-size: 14px;
        opacity: 0.9;
      }
      .balance {
        font-size: 36px;
        font-weight: 700;
        margin-top: 4px;
      }
    }

    .intro {
      max-width: 380px;
      font-size: 13px;
      opacity: 0.85;
      line-height: 1.6;

      p {
        margin: 4px 0;
      }
    }
  }

  .section-title {
    font-size: 18px;
    font-weight: 600;
    margin-bottom: 16px;
  }

  .options-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
    gap: 16px;

    .option-card {
      background: var(--n-color);
      border-radius: 8px;
      padding: 20px;
      border: 1px solid rgba(0, 0, 0, 0.06);
      display: flex;
      flex-direction: column;
      align-items: center;
      text-align: center;
      transition: all 0.2s;

      &:hover {
        border-color: #8a2be2;
        transform: translateY(-2px);
        box-shadow: 0 4px 12px rgba(0, 0, 0, 0.06);
      }

      .card-icon {
        margin-bottom: 12px;
      }

      .card-title {
        font-size: 16px;
        font-weight: 600;
        margin-bottom: 6px;
      }

      .card-desc {
        font-size: 12px;
        color: #888;
        min-height: 36px;
        margin-bottom: 12px;
      }

      .card-price {
        margin-bottom: 16px;

        .pts {
          font-size: 22px;
          font-weight: 700;
          color: #8a2be2;
        }

        .unit {
          font-size: 12px;
          color: #888;
          margin-left: 4px;
        }
      }
    }
  }
}
</style>
