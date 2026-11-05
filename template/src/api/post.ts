import http, { type ApiResponse, type PaginationMeta } from "./http";
import type { TagItem } from "./tag";

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
  body: string;
  // 作者头像哈希（md5(email)），由后端按作者邮箱计算
  user_gravatar?: string;
  tags?: TagItem[];
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

