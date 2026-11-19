<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { fetchCaptcha, verifyCaptcha } from "@/api/captcha";
import { isApiError } from "@/api/http";
import { saveCaptchaTicket } from "@/utils/captchaTicket";
import { useSystemStore } from "@/stores/system";

const route = useRoute();
const router = useRouter();
const system = useSystemStore();

const challengeId = ref("");
const image = ref("");
const answer = ref("");
const loading = ref(false);
const refreshing = ref(false);
const errorMsg = ref("");
const remaining = ref(0);
const inputRef = ref<HTMLInputElement | null>(null);

let countdownTimer: number | undefined;

const expired = computed(() => image.value !== "" && remaining.value <= 0);
const canSubmit = computed(
  () =>
    !loading.value && challengeId.value !== "" && answer.value.trim() !== "" && remaining.value > 0,
);

function stopCountdown() {
  if (countdownTimer !== undefined) {
    window.clearInterval(countdownTimer);
    countdownTimer = undefined;
  }
}

function startCountdown() {
  stopCountdown();
  countdownTimer = window.setInterval(() => {
    if (remaining.value > 0) {
      remaining.value -= 1;
    }
    if (remaining.value <= 0) {
      stopCountdown();
      // 过期后自动换一张，避免用户对着旧题反复输入。
      void loadCaptcha();
    }
  }, 1000);
}

async function loadCaptcha(clearError = true) {
  if (refreshing.value) return;

  refreshing.value = true;
  if (clearError) errorMsg.value = "";
  answer.value = "";
  stopCountdown();

  try {
    const challenge = await fetchCaptcha();
    challengeId.value = challenge.id;
    image.value = challenge.image;
    remaining.value = challenge.expires_in;
    startCountdown();
    await nextTick();
    inputRef.value?.focus();
  } catch (err) {
    challengeId.value = "";
    image.value = "";
    remaining.value = 0;
    errorMsg.value = isApiError(err) ? err.message : "获取验证码失败，请稍后再试";
  } finally {
    refreshing.value = false;
  }
}

// 只接受站内路径，避免开放重定向。
function redirectTarget(): string {
  const redirect = route.query.redirect;
  if (typeof redirect === "string" && redirect.startsWith("/") && !redirect.startsWith("//")) {
    return redirect;
  }
  return "/login?mode=register";
}

async function onSubmit() {
  if (loading.value) return;
  errorMsg.value = "";

  const value = answer.value.trim();
  if (!challengeId.value || !value) {
    errorMsg.value = "请输入图片中的字符";
    return;
  }
  if (remaining.value <= 0) {
    errorMsg.value = "验证码已过期，请点击图片刷新";
    await loadCaptcha(false);
    return;
  }

  loading.value = true;
  try {
    const result = await verifyCaptcha(challengeId.value, value);
    saveCaptchaTicket(result.ticket, result.expires_in);
    await router.replace(redirectTarget());
  } catch (err) {
    errorMsg.value = isApiError(err) ? err.message : "验证失败，请稍后再试";
    // 验证码一次性：无论什么原因失败都换一张再试，同时保留错误提示。
    await loadCaptcha(false);
  } finally {
    loading.value = false;
  }
}

onMounted(() => {
  // 注册已关闭时不再走人机验证流程。
  if (!system.registerEnabled) {
    void router.replace("/login");
    return;
  }
  void loadCaptcha();
});

onUnmounted(stopCountdown);
</script>

<template>
  <div class="flex min-h-screen items-center justify-center bg-base-200 px-4">
    <div class="card w-full max-w-sm bg-base-100 shadow-xl">
      <div class="card-body gap-4">
        <div class="text-center">
          <h1 class="text-2xl font-semibold">人机验证</h1>
          <p class="mt-1 text-sm text-base-content/60">完成验证后继续注册</p>
        </div>

        <div class="flex flex-col items-center gap-2">
          <button
            type="button"
            class="w-full rounded-box border border-base-300 bg-base-200/60 p-2"
            :aria-label="image ? '点击换一张验证码' : '重新加载验证码'"
            :disabled="refreshing || loading"
            @click="loadCaptcha()"
          >
            <img
              v-if="image"
              :src="image"
              alt="验证码图片，点击换一张"
              draggable="false"
              class="mx-auto h-14 w-auto select-none"
            />
            <div v-else class="flex h-14 items-center justify-center text-sm text-base-content/50">
              <span v-if="refreshing" class="loading loading-spinner loading-sm"></span>
              <span v-else>验证码加载失败，点击重试</span>
            </div>
          </button>

          <div class="flex w-full items-center justify-between text-xs text-base-content/50">
            <span>{{
              expired
                ? "验证码已过期"
                : refreshing && !image
                  ? "验证码加载中…"
                  : `剩余 ${remaining} 秒`
            }}</span>
            <button
              type="button"
              class="link link-primary"
              :disabled="refreshing || loading"
              @click="loadCaptcha()"
            >
              换一张
            </button>
          </div>
        </div>

        <form class="flex flex-col gap-3" novalidate @submit.prevent="onSubmit">
          <label class="floating-label">
            <span>验证码</span>
            <input
              ref="inputRef"
              v-model="answer"
              type="text"
              name="captcha"
              placeholder="请输入图片中的 4 个字符"
              autocomplete="off"
              autocapitalize="characters"
              spellcheck="false"
              maxlength="4"
              class="input input-bordered w-full font-mono text-lg tracking-[0.3em] uppercase"
              :disabled="loading"
            />
          </label>

          <div v-if="errorMsg" role="alert" class="alert alert-error alert-soft py-2 text-sm">
            <span>{{ errorMsg }}</span>
          </div>

          <button type="submit" class="btn btn-primary w-full" :disabled="!canSubmit">
            <span v-if="loading" class="loading loading-spinner loading-sm"></span>
            验证并继续
          </button>
        </form>

        <router-link to="/login?mode=register" class="btn btn-ghost btn-sm w-full"
          >返回注册</router-link
        >
      </div>
    </div>
  </div>
</template>
