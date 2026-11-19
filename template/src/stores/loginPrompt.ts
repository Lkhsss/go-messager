import { ref } from "vue";
import { defineStore } from "pinia";

// Toast 自动关闭时间。
const AUTO_HIDE_MS = 5000;

export const useLoginPromptStore = defineStore("loginPrompt", () => {
  const visible = ref(false);
  // 登录成功后要回到的位置。
  const redirect = ref("/");
  let timer: number | undefined;

  // 显示登录提示浮层。
  function show(target: string) {
    redirect.value = target || "/";
    visible.value = true;
    window.clearTimeout(timer);
    timer = window.setTimeout(hide, AUTO_HIDE_MS);
  }

  function hide() {
    visible.value = false;
    window.clearTimeout(timer);
    timer = undefined;
  }

  return { visible, redirect, show, hide };
});
