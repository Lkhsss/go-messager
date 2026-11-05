import http, { type ApiResponse } from "./http";

// 与 Go 端 ReactionType 对齐
export type ReactionType =
  | "like"
  | "dislike"
  | "question"
  | "laugh"
  | "hooray"
  | "heart"
  | "rocket"
  | "eyes";

// 与 Go 端 ReactionSummary 对齐
export interface ReactionSummary {
  like: number;
  dislike: number;
  question: number;
  emoji?: Record<string, number>;
}

export interface ReactionResult {
  reactions: ReactionSummary;
  my_reactions: ReactionType[];
}

export interface ReactionOption {
  type: ReactionType;
  emoji: string;
  label: string;
}

// 三个主反应常驻在文章底部
export const PRIMARY_REACTIONS: ReactionOption[] = [
  { type: "like", emoji: "👍", label: "赞" },
  { type: "dislike", emoji: "👎", label: "踩" },
  { type: "question", emoji: "❓", label: "疑问" },
];

// GitHub 风格的额外表情（收在「更多」面板里）
export const EMOJI_REACTIONS: ReactionOption[] = [
  { type: "laugh", emoji: "😄", label: "开心" },
  { type: "hooray", emoji: "🎉", label: "庆祝" },
  { type: "heart", emoji: "❤️", label: "喜欢" },
  { type: "rocket", emoji: "🚀", label: "火箭" },
  { type: "eyes", emoji: "👀", label: "围观" },
];

// 从汇总里取某个反应的计数
export function reactionCount(summary: ReactionSummary | undefined, type: ReactionType): number {
  if (!summary) return 0;
  if (type === "like") return summary.like;
  if (type === "dislike") return summary.dislike;
  if (type === "question") return summary.question;
  return summary.emoji?.[type] ?? 0;
}

// 切换反应 POST /api/post/:id/reaction（再次点击取消）
export async function toggleReaction(postId: number, type: ReactionType): Promise<ReactionResult> {
  const { data } = await http.post<ApiResponse<ReactionResult>>(`/post/${postId}/reaction`, { type });
  if (!data.success || !data.data) {
    throw data.error ?? { code: "UNKNOWN_ERROR", message: "操作失败" };
  }
  return data.data;
}
