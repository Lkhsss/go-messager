import { ref } from "vue";
import { defineStore } from "pinia";
import type { XPGain } from "@/api/experience";

// Toast 自动关闭时间。
const AUTO_HIDE_MS = 4000;

export interface ToastItem {
  id: number;
  title: string;
  detail: string;
  levelUp: boolean;
}

export const useToastStore = defineStore("toast", () => {
  const items = ref<ToastItem[]>([]);
  const timers = new Map<number, number>();
  let nextId = 1;

  function dismiss(id: number) {
    items.value = items.value.filter((item) => item.id !== id);
    const timer = timers.get(id);
    if (timer !== undefined) {
      window.clearTimeout(timer);
      timers.delete(id);
    }
  }

  // 最多同时显示 3 条，新的顶掉最旧的。
  function push(item: Omit<ToastItem, "id">) {
    const id = nextId++;
    items.value = [...items.value.slice(-2), { id, ...item }];
    timers.set(id, window.setTimeout(() => dismiss(id), AUTO_HIDE_MS));
  }

  // 经验增加提示，升级时额外高亮。
  function notifyXp(xp?: XPGain) {
    if (!xp || xp.gained <= 0) return;
    push({
      title: `${xp.task_label} +${xp.gained} 经验`,
      detail: xp.leveled_up
        ? `🎉 升级到 Lv.${xp.level}！`
        : `当前 Lv.${xp.level} · 共 ${xp.exp} 经验`,
      levelUp: xp.leveled_up,
    });
  }

  return { items, push, dismiss, notifyXp };
});
