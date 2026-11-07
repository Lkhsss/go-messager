import http, { type ApiResponse, type PaginationMeta } from "./http";
import type { TagItem } from "./tag";
import type { ReactionSummary, ReactionType } from "./reaction";

// 与 Go 端 model.go 的 Post 对齐：
// gorm.Model 没有 json tag，ID / 时间字段是首字母大写，业务字段来自 Post 的 tag（小写）。
export interface PostItem {
  ID: number;
  CreatedAt: string;
  UpdatedAt: string;
  DeletedAt: string | null;
  user: string;
  title: string;
  subtitle: string;
  body?: string;
  // 首页摘要（纯文本），列表接口不返回正文
  excerpt?: string;
  // 管理员隐藏后为 true；仅作者与管理员可见
  private: boolean;
  // 评论字段：原文为空；评论的 root_id 指向原文，parent_id 指向父评论
  parent_id?: number | null;
  root_id?: number | null;
  // 作者头像版本（头像图片走 /api/avatar 接口）
  user_avatar_version?: string;
  tags?: TagItem[];
  // 反应汇总 + 当前用户已做出的反应
  reactions?: ReactionSummary;
  my_reactions?: ReactionType[];
}

export interface CommentItem extends PostItem {
  children: CommentItem[];
}

export interface PostQuery {
  page?: number;
  perPage?: number;
  // 必须同时包含的标签（AND）
  includeTagIds?: number[];
  // 必须一个都不包含的标签
  excludeTagIds?: number[];
}

export interface PostListResult {
  posts: PostItem[];
  meta: PaginationMeta;
}

// 帖子列表 GET /api/post?page=1&per_page=20&include_tag_ids=1,2&exclude_tag_ids=3
// page 最小为 1；per_page 范围 1~100（后端会截断），默认 20。
// 标签用逗号串传递：axios 默认会把数组序列化成 tag[]=1，gin 那边解析不到。
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
    // 后端空页也会返回 meta；缺失时按当前返回结果兜底
    meta: data.meta ?? {
      page,
      per_page: perPage,
      total: posts.length,
      total_pages: posts.length > 0 ? 1 : 0,
    },
  };
}

// 近 24 小时新发文章数 GET /api/recent-post-count
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
  // 最多 5 个，后端会再校验一次
  tag_ids?: number[];
}

// 创建帖子 POST /api/post/new（需登录，作者由 token 决定）
export async function createPost(input: CreatePostInput): Promise<PostItem> {
  const { data } = await http.post<ApiResponse<PostItem>>("/post/new", input);
  if (!data.success || !data.data) {
    throw data.error ?? { code: "UNKNOWN_ERROR", message: "发布失败" };
  }
  return data.data;
}


// 我的文章 GET /api/me/posts?page=1&per_page=10
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

// 文章详情 GET /api/post/:id
export async function fetchPost(id: number): Promise<PostItem> {
  const { data } = await http.get<ApiResponse<PostItem>>(`/post/${id}`);
  if (!data.success || !data.data) {
    throw data.error ?? { code: "UNKNOWN_ERROR", message: "获取文章失败" };
  }
  return data.data;
}

// 编辑自己的文章 PUT /api/post/:id
export async function updatePost(id: number, input: CreatePostInput): Promise<PostItem> {
  const { data } = await http.put<ApiResponse<PostItem>>(`/post/${id}`, input);
  if (!data.success || !data.data) {
    throw data.error ?? { code: "UNKNOWN_ERROR", message: "保存失败" };
  }
  return data.data;
}

// 删除自己的文章 DELETE /api/post/:id
export async function deletePost(id: number): Promise<void> {
  const { data } = await http.delete<ApiResponse<{ id: number }>>(`/post/${id}`);
  if (!data.success) {
    throw data.error ?? { code: "UNKNOWN_ERROR", message: "删除失败" };
  }
}

// 管理员隐藏/取消隐藏文章 PATCH /api/post/:id/private
export async function setPostPrivate(id: number, isPrivate: boolean): Promise<PostItem> {
  const { data } = await http.patch<ApiResponse<PostItem>>(`/post/${id}/private`, { private: isPrivate });
  if (!data.success || !data.data) {
    throw data.error ?? { code: "UNKNOWN_ERROR", message: "更新文章可见性失败" };
  }
  return data.data;
}


// 获取某篇文章的评论（扁平列表，前端按 parent_id 组树）
export async function fetchComments(postId: number): Promise<PostItem[]> {
  const { data } = await http.get<ApiResponse<PostItem[]>>(`/post/${postId}/comments`);
  if (!data.success || !data.data) {
    throw data.error ?? { code: "UNKNOWN_ERROR", message: "获取评论失败" };
  }
  return data.data;
}

// 发表评论或回复 POST /api/post/:id/comments
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


