import http, { type ApiResponse, type PaginationMeta } from "./http";
import type { TagItem } from "./tag";
import type { ReactionSummary, ReactionType } from "./reaction";

// 对应 Go 端 Post，gorm 字段首字母大写。
export interface PostItem {
  ID: number;
  CreatedAt: string;
  UpdatedAt: string;
  DeletedAt: string | null;
  user: string;
  title: string;
  subtitle: string;
  body?: string;
  // 列表接口只返回摘要。
  excerpt?: string;
  // 仅作者与管理员可见。
  private: boolean;
  // 评论：root_id 指向原文，parent_id 指向父评论。
  parent_id?: number | null;
  root_id?: number | null;
  // 作者头像版本。
  user_avatar_version?: string;
  tags?: TagItem[];
  // 反应汇总和我的反应。
  reactions?: ReactionSummary;
  my_reactions?: ReactionType[];
}

export interface CommentItem extends PostItem {
  children: CommentItem[];
}

export interface PostQuery {
  page?: number;
  perPage?: number;
  // 必须同时包含。
  includeTagIds?: number[];
  // 必须都不包含。
  excludeTagIds?: number[];
}

export interface PostListResult {
  posts: PostItem[];
  meta: PaginationMeta;
}

// 标签用逗号串传递，避免 axios 序列化成数组。
export async function fetchPosts({
  page = 1,
  perPage = 20,
  includeTagIds,
  excludeTagIds,
}: PostQuery = {}): Promise<PostListResult> {
  const params: Record<string, string | number> = { page, per_page: perPage };
  if (includeTagIds && includeTagIds.length > 0) {
    params.include_tag_ids = includeTagIds.join(",");
  }
  if (excludeTagIds && excludeTagIds.length > 0) {
    params.exclude_tag_ids = excludeTagIds.join(",");
  }

  const { data } = await http.get<ApiResponse<PostItem[]>>("/post", { params });

  if (!data.success || !data.data) {
    throw data.error ?? { code: "UNKNOWN_ERROR", message: "获取帖子失败" };
  }

  const posts = data.data;
  return {
    posts,
    meta: data.meta ?? {
      page,
      per_page: perPage,
      total: posts.length,
      total_pages: posts.length > 0 ? 1 : 0,
    },
  };
}

// 近 24 小时文章数。
export async function fetchRecentPostCount(): Promise<number> {
  const { data } = await http.get<ApiResponse<{ count: number }>>("/recent-post-count");
  if (!data.success || !data.data) {
    throw data.error ?? { code: "UNKNOWN_ERROR", message: "获取 24 小时文章数失败" };
  }
  return data.data.count;
}

export interface CreatePostInput {
  title: string;
  subtitle?: string;
  body: string;
  // 最多 5 个。
  tag_ids?: number[];
}

// 创建帖子。
export async function createPost(input: CreatePostInput): Promise<PostItem> {
  const { data } = await http.post<ApiResponse<PostItem>>("/post/new", input);
  if (!data.success || !data.data) {
    throw data.error ?? { code: "UNKNOWN_ERROR", message: "发布失败" };
  }
  return data.data;
}


// 我的文章。
export async function fetchMyPosts({ page = 1, perPage = 10 }: Pick<PostQuery, "page" | "perPage"> = {}): Promise<PostListResult> {
  const { data } = await http.get<ApiResponse<PostItem[]>>("/me/posts", {
    params: { page, per_page: perPage },
  });

  if (!data.success || !data.data) {
    throw data.error ?? { code: "UNKNOWN_ERROR", message: "获取我的文章失败" };
  }

  const posts = data.data;
  return {
    posts,
    meta: data.meta ?? {
      page,
      per_page: perPage,
      total: posts.length,
      total_pages: posts.length > 0 ? 1 : 0,
    },
  };
}

// 文章详情。
export async function fetchPost(id: number): Promise<PostItem> {
  const { data } = await http.get<ApiResponse<PostItem>>(`/post/${id}`);
  if (!data.success || !data.data) {
    throw data.error ?? { code: "UNKNOWN_ERROR", message: "获取文章失败" };
  }
  return data.data;
}

// 编辑文章。
export async function updatePost(id: number, input: CreatePostInput): Promise<PostItem> {
  const { data } = await http.put<ApiResponse<PostItem>>(`/post/${id}`, input);
  if (!data.success || !data.data) {
    throw data.error ?? { code: "UNKNOWN_ERROR", message: "保存失败" };
  }
  return data.data;
}

// 删除文章。
export async function deletePost(id: number): Promise<void> {
  const { data } = await http.delete<ApiResponse<{ id: number }>>(`/post/${id}`);
  if (!data.success) {
    throw data.error ?? { code: "UNKNOWN_ERROR", message: "删除失败" };
  }
}

// 隐藏/取消隐藏文章。
export async function setPostPrivate(id: number, isPrivate: boolean): Promise<PostItem> {
  const { data } = await http.patch<ApiResponse<PostItem>>(`/post/${id}/private`, { private: isPrivate });
  if (!data.success || !data.data) {
    throw data.error ?? { code: "UNKNOWN_ERROR", message: "更新文章可见性失败" };
  }
  return data.data;
}


// 评论列表，前端按 parent_id 组树。
export async function fetchComments(postId: number): Promise<PostItem[]> {
  const { data } = await http.get<ApiResponse<PostItem[]>>(`/post/${postId}/comments`);
  if (!data.success || !data.data) {
    throw data.error ?? { code: "UNKNOWN_ERROR", message: "获取评论失败" };
  }
  return data.data;
}

// 发表评论或回复。
export async function createComment(postId: number, body: string, parentId?: number): Promise<PostItem> {
  const payload: { body: string; parent_id?: number } = { body };
  if (parentId) {
    payload.parent_id = parentId;
  }

  const { data } = await http.post<ApiResponse<PostItem>>(`/post/${postId}/comments`, payload);
  if (!data.success || !data.data) {
    throw data.error ?? { code: "UNKNOWN_ERROR", message: "发表评论失败" };
  }
  return data.data;
}


