<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useAuthStore } from "@/stores/auth";
import { isApiError } from "@/api/http";
import { fetchExperience, type DailyTaskState, type ExperienceStatus } from "@/api/experience";
import UserAvatar from "@/components/UserAvatar.vue";
import ProfileCard from "@/components/ProfileCard.vue";
import { EMOJI_REACTIONS, PRIMARY_REACTIONS, reactionCount } from "@/api/reaction";
import { fileToAvatarDataUrl } from "@/utils/avatar";

const auth = useAuthStore();

// 「我的」页展示「我获得的反应」用
const allReactions = [...PRIMARY_REACTIONS, ...EMOJI_REACTIONS];

// 本地预览（尚未保存的 data URL）
const pendingAvatar = ref("");
const fileInput = ref<HTMLInputElement | null>(null);
const saving = ref(false);
const errorMsg = ref("");
const successMsg = ref("");

const previewSrc = computed(() => pendingAvatar.value || undefined);

function pickFile() {
  fileInput.value?.click();
}

async function onFileChange(event: Event) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  input.value = ""; // 允许重复选择同一张图
  if (!file) return;

  errorMsg.value = "";
  successMsg.value = "";
  try {
    pendingAvatar.value = await fileToAvatarDataUrl(file);
  } catch (err) {
    errorMsg.value = err instanceof Error ? err.message : "图片处理失败";
  }
}

async function saveAvatar() {
  if (saving.value || !pendingAvatar.value) return;

  saving.value = true;
  errorMsg.value = "";
  successMsg.value = "";
  try {
    await auth.updateAvatar(pendingAvatar.value);
    pendingAvatar.value = "";
    successMsg.value = "头像已保存";
  } catch (err) {
    errorMsg.value = isApiError(err) ? err.message : "头像保存失败，请稍后再试";
  } finally {
    saving.value = false;
  }
}

async function removeAvatar() {
  if (saving.value) return;

  saving.value = true;
  errorMsg.value = "";
  successMsg.value = "";
  try {
    await auth.removeAvatar();
    pendingAvatar.value = "";
    successMsg.value = "头像已移除，将显示用户名首字母";
  } catch (err) {
    errorMsg.value = isApiError(err) ? err.message : "头像移除失败，请稍后再试";
  } finally {
    saving.value = false;
  }
}

// 经验条和每日任务
const experience = ref<ExperienceStatus | null>(null);
const experienceLoading = ref(false);
const experienceError = ref("");

const levelProgressPercent = computed(() => {
  const status = experience.value;
  if (!status || status.level_need_exp <= 0) return 0;
  return Math.min(100, Math.round((status.level_progress_exp / status.level_need_exp) * 100));
});

const completedTaskCount = computed(
  () => experience.value?.tasks.filter((task) => task.completed).length ?? 0,
);

async function loadExperience() {
  if (experienceLoading.value) return;

  experienceLoading.value = true;
  experienceError.value = "";
  try {
    const status = await fetchExperience();
    experience.value = status;
    auth.applyExperience(status.exp, status.level);
  } catch (err) {
    experienceError.value = isApiError(err) ? err.message : "获取经验信息失败，请稍后再试";
  } finally {
    experienceLoading.value = false;
  }
}

function taskStatusLabel(task: DailyTaskState): string {
  return task.completed ? "已完成" : `+${task.reward} 经验`;
}

onMounted(() => {
  void loadExperience();
});
</script>

<template>
  <main class="mx-auto w-full max-w-5xl p-4 sm:p-6 lg:p-8">
    <header class="flex flex-wrap items-end justify-between gap-4">
      <div>
        <p class="text-xs uppercase tracking-[0.3em] text-base-content/40">Profile</p>
        <h1 class="mt-1 text-2xl font-semibold lg:text-3xl">我的</h1>
        <div class="mt-1 flex flex-wrap items-center gap-2 text-sm text-base-content/60">
          <span>@{{ auth.user?.name ?? "?" }}</span>
          <span
            class="badge badge-sm rounded-full border-primary/30 bg-primary/10 font-mono text-primary"
          >
            Lv.{{ auth.user?.level ?? 1 }}
          </span>
          <span v-if="auth.user?.exp !== undefined" class="text-xs text-base-content/45">
            累计 {{ auth.user.exp }} 经验
          </span>
        </div>
      </div>
      <span v-if="auth.isAdmin" class="badge badge-primary badge-sm">管理员</span>
    </header>

    <!-- 上半区：个人名片 + 等级经验 -->
    <section class="mt-6 grid gap-6 lg:grid-cols-2 lg:items-start">
      <section aria-labelledby="profile-card-heading" class="flex flex-col items-center gap-3">
        <h2 id="profile-card-heading" class="sr-only">个人名片</h2>
        <ProfileCard
          :name="auth.user?.name ?? '?'"
          :role="auth.user?.role ?? 'user'"
          :level="auth.user?.level ?? 0"
          :avatar-version="auth.user?.avatar_version"
          :preview-src="previewSrc"
          :reaction-totals="auth.user?.reaction_totals"
        />
      </section>

      <section class="card bg-base-100 shadow-sm">
        <div class="card-body gap-5">
          <div class="flex flex-wrap items-center justify-between gap-2">
            <h2 class="card-title text-lg">等级与经验</h2>
            <button
              type="button"
              class="btn btn-ghost btn-xs"
              :disabled="experienceLoading"
              @click="loadExperience"
            >
              刷新
            </button>
          </div>

          <div
            v-if="experienceError"
            role="alert"
            class="alert alert-error alert-soft py-2 text-sm"
          >
            <span>{{ experienceError }}</span>
          </div>

          <div v-else-if="experience" class="space-y-4">
            <div class="flex items-end justify-between gap-3">
              <div>
                <p class="text-xs text-base-content/50">当前等级</p>
                <p class="font-mono text-4xl font-semibold leading-none">
                  Lv.{{ experience.level }}
                </p>
              </div>
              <div class="text-right">
                <p class="text-xs text-base-content/50">累计经验</p>
                <p class="font-mono text-2xl font-semibold">{{ experience.exp }}</p>
              </div>
            </div>

            <div class="space-y-1.5">
              <progress
                class="progress progress-primary w-full"
                :value="levelProgressPercent"
                max="100"
              ></progress>
              <div
                class="flex flex-wrap items-center justify-between gap-2 text-xs text-base-content/60"
              >
                <span
                  >本级 {{ experience.level_progress_exp }} /
                  {{ experience.level_need_exp }} 经验</span
                >
                <span
                  >还差
                  {{ Math.max(experience.level_need_exp - experience.level_progress_exp, 0) }}
                  经验升级</span
                >
              </div>
            </div>

            <div class="grid grid-cols-2 gap-2 text-xs">
              <div class="rounded-box bg-base-200/70 px-3 py-2">
                <p class="text-base-content/50">下一级门槛</p>
                <p class="mt-0.5 font-mono text-base font-semibold">
                  {{ experience.next_level_exp }}
                </p>
              </div>
              <div class="rounded-box bg-base-200/70 px-3 py-2">
                <p class="text-base-content/50">今日任务</p>
                <p class="mt-0.5 font-mono text-base font-semibold">
                  {{ completedTaskCount }}/{{ experience.tasks.length }}
                </p>
              </div>
            </div>
          </div>

          <div v-else class="flex justify-center py-6">
            <span class="loading loading-spinner loading-md"></span>
          </div>
        </div>
      </section>
    </section>

    <!-- 每日任务：整行 -->
    <section class="mt-6 card bg-base-100 shadow-sm">
      <div class="card-body gap-4">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <h2 class="card-title text-lg">每日任务</h2>
          <div class="flex items-center gap-2 text-xs text-base-content/45">
            <span v-if="experience">{{ experience.date }} 重置</span>
            <span v-if="experience"
              >已完成 {{ completedTaskCount }}/{{ experience.tasks.length }}</span
            >
            <button
              v-if="experienceError"
              type="button"
              class="btn btn-ghost btn-xs"
              :disabled="experienceLoading"
              @click="loadExperience"
            >
              重试
            </button>
          </div>
        </div>

        <div v-if="experienceError" role="alert" class="alert alert-error alert-soft py-2 text-sm">
          <span>{{ experienceError }}</span>
        </div>

        <div v-else-if="experience" class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
          <article
            v-for="task in experience.tasks"
            :key="task.key"
            class="flex flex-col gap-2 rounded-box border p-3"
            :class="task.completed ? 'border-success/30 bg-success/10' : 'border-base-200'"
          >
            <div class="flex items-start justify-between gap-2">
              <span class="font-medium">{{ task.label }}</span>
              <span
                class="badge badge-sm shrink-0 rounded-full"
                :class="task.completed ? 'badge-success' : 'badge-ghost'"
                >{{ taskStatusLabel(task) }}</span
              >
            </div>

            <p class="text-xs leading-5 text-base-content/55">{{ task.description }}</p>

            <progress
              class="progress w-full"
              :class="task.completed ? 'progress-success' : 'progress-primary'"
              :value="task.progress"
              :max="task.target"
            ></progress>

            <div
              class="mt-auto flex items-center justify-between gap-2 text-xs text-base-content/55"
            >
              <span class="font-mono">{{ task.progress }}/{{ task.target }}</span>
              <router-link
                v-if="task.key === 'article'"
                to="/post/new"
                class="btn btn-ghost btn-xs"
              >
                去写文章
              </router-link>
              <router-link
                v-else-if="task.key === 'comment' || task.key === 'interaction'"
                to="/"
                class="btn btn-ghost btn-xs"
              >
                去互动
              </router-link>
              <span v-else class="text-base-content/40">自动发放</span>
            </div>
          </article>
        </div>

        <div v-else class="flex justify-center py-6">
          <span class="loading loading-spinner loading-md"></span>
        </div>
      </div>
    </section>

    <!-- 下半区：头像 + 我获得的反应 -->
    <section class="mt-6 grid gap-6 lg:grid-cols-2 lg:items-start">
      <section class="card bg-base-100 shadow-sm">
        <div class="card-body gap-4">
          <h2 class="card-title text-lg">头像</h2>

          <div class="flex flex-wrap items-center gap-4">
            <UserAvatar
              :name="auth.user?.name ?? '?'"
              :version="auth.user?.avatar_version"
              :src="previewSrc"
              :size="72"
            />
            <div class="space-y-1 text-sm text-base-content/60">
              <p class="text-xs text-base-content/50">
                支持 JPEG/PNG，原图不超过 5MB；不设置则显示用户名首字母。
              </p>
            </div>
          </div>

          <input
            ref="fileInput"
            type="file"
            accept="image/jpeg,image/png"
            class="hidden"
            @change="onFileChange"
          />

          <div class="flex flex-wrap gap-2">
            <button
              type="button"
              class="btn btn-primary btn-sm"
              :disabled="saving"
              @click="pickFile"
            >
              选择图片
            </button>
            <button
              type="button"
              class="btn btn-sm"
              :disabled="saving || !pendingAvatar"
              @click="saveAvatar"
            >
              <span v-if="saving" class="loading loading-spinner loading-sm"></span>
              保存头像
            </button>
            <button
              type="button"
              class="btn btn-ghost btn-sm"
              :disabled="saving || (!auth.user?.avatar_version && !pendingAvatar)"
              @click="removeAvatar"
            >
              移除头像
            </button>
          </div>

          <div v-if="errorMsg" role="alert" class="alert alert-error alert-soft py-2 text-sm">
            <span>{{ errorMsg }}</span>
          </div>
          <div v-if="successMsg" role="status" class="alert alert-success alert-soft py-2 text-sm">
            <span>{{ successMsg }}</span>
          </div>
        </div>
      </section>

      <section class="card bg-base-100 shadow-sm">
        <div class="card-body gap-4">
          <h2 class="card-title text-lg">我获得的反应</h2>

          <div class="grid grid-cols-2 gap-2 sm:grid-cols-4">
            <div
              v-for="option in allReactions"
              :key="option.type"
              class="rounded-box bg-base-200/70 px-3 py-2"
              :title="option.label"
            >
              <div class="flex items-center justify-between gap-2">
                <span class="text-sm">{{ option.emoji }}</span>
                <span class="font-mono text-lg font-semibold">{{
                  reactionCount(auth.user?.reaction_totals, option.type)
                }}</span>
              </div>
              <p class="mt-0.5 text-[0.65rem] text-base-content/50">{{ option.label }}</p>
            </div>
          </div>
        </div>
      </section>
    </section>

    <p v-if="auth.isAdmin" class="mt-6 text-sm text-base-content/60">
      标签管理已移到侧边栏的「管理」页面。
    </p>
  </main>
</template>
