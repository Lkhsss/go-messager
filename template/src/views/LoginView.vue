<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useAuthStore } from "@/stores/auth";
import { useSystemStore } from "@/stores/system";
import { useToastStore } from "@/stores/toast";
import { isApiError } from "@/api/http";
import { clearCaptchaTicket, readCaptchaTicket } from "@/utils/captchaTicket";

type Mode = "login" | "register";

// 这些错误码说明 ticket 缺失或已失效，需要重新走验证页。
const CAPTCHA_ERROR_CODES = [
  "CAPTCHA_REQUIRED",
  "CAPTCHA_INVALID",
  "CAPTCHA_EXPIRED",
  "CAPTCHA_TOO_MANY_ATTEMPTS",
  "CAPTCHA_RATE_LIMITED",
];

const route = useRoute();
const router = useRouter();
const auth = useAuthStore();
const system = useSystemStore();
const toast = useToastStore();

const mode = ref<Mode>("login");
const loading = ref(false);
const errorMsg = ref("");
// 是否已经拿到有效的人机验证 ticket。
const captchaReady = ref(false);

const form = reactive({
  name: "",
  password: "",
  confirm: "",
});

const isRegister = computed(() => mode.value === "register");
const title = computed(() => (isRegister.value ? "注册账号" : "登录"));
const submitText = computed(() => (isRegister.value ? "注册" : "登录"));

function syncCaptchaState() {
  captchaReady.value = readCaptchaTicket() !== null;
}

function switchMode(next: Mode) {
  if (mode.value === next) return;
  if (next === "register" && !system.registerEnabled) {
    errorMsg.value = "站点已关闭注册";
    return;
  }
  mode.value = next;
  form.password = "";
  form.confirm = "";
  errorMsg.value = "";
  syncCaptchaState();
}

// 验证完成后回到注册页，并保留原来的回跳地址。
function registerReturnPath(): string {
  const redirect = route.query.redirect;
  const safe =
    typeof redirect === "string" && redirect.startsWith("/") && !redirect.startsWith("//")
      ? `&redirect=${encodeURIComponent(redirect)}`
      : "";
  return `/login?mode=register${safe}`;
}

function goVerify() {
  void router.push({ name: "verify", query: { redirect: registerReturnPath() } });
}

function validate(): string {
  const name = form.name.trim();
  if (!name) return "请输入用户名";
  if (isRegister.value && [...name].length < 2) return "用户名至少 2 个字符";
  if (!form.password) return "请输入密码";
  if (isRegister.value) {
    if (new TextEncoder().encode(form.password).length < 8) return "密码至少 8 个字节";
    if (form.password !== form.confirm) return "两次输入的密码不一致";
  }
  return "";
}

onMounted(() => {
  if (route.query.mode === "register") {
    if (system.registerEnabled) {
      mode.value = "register";
    } else {
      errorMsg.value = "站点已关闭注册";
    }
  }
  syncCaptchaState();
});

async function onSubmit() {
  if (loading.value) return;
  if (isRegister.value && !system.registerEnabled) {
    errorMsg.value = "站点已关闭注册";
    return;
  }
  errorMsg.value = validate();
  if (errorMsg.value) return;

  // 注册前必须先通过独立验证页，服务端没有 ticket 也会拒绝。
  const captchaTicket = isRegister.value ? readCaptchaTicket() : null;
  if (isRegister.value && !captchaTicket) {
    errorMsg.value = "请先完成人机验证";
    goVerify();
    return;
  }

  loading.value = true;
  try {
    const credentials = { name: form.name.trim(), password: form.password };
    const result = isRegister.value
      ? await auth.signup({ ...credentials, captcha_ticket: captchaTicket ?? undefined })
      : await auth.login(credentials);

    // ticket 一次性，验证成功后立即丢弃本地缓存。
    clearCaptchaTicket();
    captchaReady.value = false;

    if (result.xp) {
      auth.applyExperience(result.xp.exp, result.xp.level);
      toast.notifyXp(result.xp);
    }

    // 只接受站内路径，避免开放重定向
    const redirect = route.query.redirect;
    const target =
      typeof redirect === "string" && redirect.startsWith("/") && !redirect.startsWith("//")
        ? redirect
        : "/";
    await router.replace(target);
  } catch (err) {
    if (isApiError(err) && CAPTCHA_ERROR_CODES.includes(err.code)) {
      clearCaptchaTicket();
      captchaReady.value = false;
      errorMsg.value = err.message;
      goVerify();
      return;
    }
    errorMsg.value = isApiError(err) ? err.message : "请求失败，请稍后再试";
  } finally {
    loading.value = false;
  }
}
</script>

<template>
  <div class="flex min-h-screen items-center justify-center bg-base-200 px-4">
    <div class="card w-full max-w-sm bg-base-100 shadow-xl">
      <div class="card-body gap-4">
        <div class="text-center">
          <h1 class="text-2xl font-semibold">Go Messager</h1>
          <p class="mt-1 text-sm text-base-content/60">{{ title }}</p>
        </div>

        <div
          v-if="system.isMaintenance"
          role="status"
          class="alert alert-warning alert-soft py-2 text-sm"
        >
          <span>站点维护中，仅管理员可登录</span>
        </div>

        <div role="tablist" class="tabs tabs-box w-full">
          <button
            type="button"
            role="tab"
            class="tab flex-1"
            :class="{ 'tab-active': !isRegister }"
            @click="switchMode('login')"
          >
            登录
          </button>
          <button
            v-if="system.registerEnabled"
            type="button"
            role="tab"
            class="tab flex-1"
            :class="{ 'tab-active': isRegister }"
            @click="switchMode('register')"
          >
            注册
          </button>
        </div>

        <form class="flex flex-col gap-3" novalidate @submit.prevent="onSubmit">
          <label class="floating-label">
            <span>用户名</span>
            <input
              v-model="form.name"
              type="text"
              name="name"
              placeholder="请输入用户名"
              autocomplete="username"
              class="input input-bordered w-full"
              :disabled="loading"
            />
          </label>

          <label class="floating-label">
            <span>密码</span>
            <input
              v-model="form.password"
              type="password"
              name="password"
              placeholder="请输入密码"
              :autocomplete="isRegister ? 'new-password' : 'current-password'"
              class="input input-bordered w-full"
              :disabled="loading"
            />
          </label>

          <label v-if="isRegister" class="floating-label">
            <span>确认密码</span>
            <input
              v-model="form.confirm"
              type="password"
              name="confirm"
              placeholder="请再次输入密码"
              autocomplete="new-password"
              class="input input-bordered w-full"
              :disabled="loading"
            />
          </label>

          <!-- 注册前的人机验证：在独立页面完成后带着 ticket 回到这里 -->
          <div
            v-if="isRegister"
            class="flex items-center justify-between gap-2 rounded-box border border-base-300 bg-base-200/50 px-3 py-2 text-xs"
          >
            <span v-if="captchaReady" class="flex items-center gap-1.5 font-medium text-success">
              <svg
                xmlns="http://www.w3.org/2000/svg"
                class="h-4 w-4"
                viewBox="0 0 20 20"
                fill="currentColor"
                aria-hidden="true"
              >
                <path
                  fill-rule="evenodd"
                  d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z"
                  clip-rule="evenodd"
                />
              </svg>
              人机验证已通过
            </span>
            <span v-else class="text-base-content/60">注册前需要完成人机验证</span>
            <button
              type="button"
              class="link link-primary shrink-0"
              :disabled="loading"
              @click="goVerify"
            >
              {{ captchaReady ? "重新验证" : "去验证" }}
            </button>
          </div>

          <div v-if="errorMsg" role="alert" class="alert alert-error alert-soft py-2 text-sm">
            <span>{{ errorMsg }}</span>
          </div>

          <button type="submit" class="btn btn-primary w-full" :disabled="loading">
            <span v-if="loading" class="loading loading-spinner loading-sm"></span>
            {{ submitText }}
          </button>
        </form>

        <p v-if="system.registerEnabled" class="text-center text-xs text-base-content/50">
          {{ isRegister ? "已有账号？" : "还没有账号？" }}
          <button
            type="button"
            class="link link-primary"
            @click="switchMode(isRegister ? 'login' : 'register')"
          >
            {{ isRegister ? "去登录" : "去注册" }}
          </button>
        </p>
        <p v-else class="text-center text-xs text-base-content/50">站点已关闭注册，请联系管理员</p>

        <router-link v-if="system.isPublic" to="/" class="btn btn-ghost btn-sm w-full"
          >先逛逛首页</router-link
        >
      </div>
    </div>
  </div>
</template>
