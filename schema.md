# Unilo 数据库设计

## 0. 设计约定

- 主数据库：PostgreSQL。
- 主键：业务实体默认使用 UUID；高频顺序消息可使用 `BIGSERIAL` 便于游标分页。
- 时间字段：统一使用 `TIMESTAMPTZ`。
- 软删除：频道、动态、文件等用户内容建议保留 `deleted_at`。
- JSON 扩展：消息、文件、AI 等扩展字段使用 `JSONB`。
- 命名：表名使用复数或业务名，字段使用 snake_case。

## 1. 枚举约定

### message_type

- `text`：普通文本。
- `code`：代码块。
- `image`：图片。
- `file`：文件。
- `agent`：AI 助手消息。

### storage_type

- `local`：本地文件系统。
- `s3`：S3 兼容对象存储，例如 MinIO。

### agent_message_role

- `user`
- `assistant`
- `system`
- `tool`

## 2. 用户与认证

### users

| 字段名 | 类型 | 约束 | 描述 |
| --- | --- | --- | --- |
| `id` | UUID | PK | 用户唯一标识 |
| `username` | VARCHAR(50) | UNIQUE, NOT NULL | 登录账号 |
| `password_hash` | VARCHAR(255) | NOT NULL | 密码哈希 |
| `nickname` | VARCHAR(50) | NOT NULL | 展示昵称 |
| `avatar_url` | VARCHAR(500) | NULL | 头像地址 |
| `created_at` | TIMESTAMPTZ | NOT NULL | 注册时间 |
| `updated_at` | TIMESTAMPTZ | NOT NULL | 更新时间 |
| `deleted_at` | TIMESTAMPTZ | NULL | 软删除时间 |

建议索引：

- `UNIQUE (username)`

### refresh_tokens

用于 Access Token + Refresh Token 策略。Access Token 可无状态校验，Refresh Token 需要落库以支持轮换和登出失效。

| 字段名 | 类型 | 约束 | 描述 |
| --- | --- | --- | --- |
| `id` | UUID | PK | Token 记录 ID |
| `user_id` | UUID | FK(users.id), NOT NULL | 所属用户 |
| `token_hash` | VARCHAR(255) | UNIQUE, NOT NULL | Refresh Token 哈希 |
| `expires_at` | TIMESTAMPTZ | NOT NULL | 过期时间 |
| `revoked_at` | TIMESTAMPTZ | NULL | 主动失效时间 |
| `created_at` | TIMESTAMPTZ | NOT NULL | 创建时间 |

建议索引：

- `INDEX (user_id, expires_at DESC)`
- `UNIQUE (token_hash)`

## 3. 频道聊天

### channels

| 字段名 | 类型 | 约束 | 描述 |
| --- | --- | --- | --- |
| `id` | UUID | PK | 频道唯一标识 |
| `name` | VARCHAR(100) | NOT NULL | 频道名称 |
| `created_by` | UUID | FK(users.id), NOT NULL | 创建者 ID |
| `created_at` | TIMESTAMPTZ | NOT NULL | 创建时间 |
| `updated_at` | TIMESTAMPTZ | NOT NULL | 更新时间 |
| `deleted_at` | TIMESTAMPTZ | NULL | 软删除时间 |

建议索引：

- `INDEX (deleted_at, created_at)`

### channel_messages

| 字段名 | 类型 | 约束 | 描述 |
| --- | --- | --- | --- |
| `id` | BIGSERIAL | PK | 递增消息 ID，用于游标分页 |
| `channel_id` | UUID | FK(channels.id), NOT NULL | 所属频道 ID |
| `sender_id` | UUID | FK(users.id), NOT NULL | 发送者 ID |
| `reply_to_id` | BIGINT | FK(channel_messages.id), NULL | 回复的目标消息 ID |
| `msg_type` | VARCHAR(20) | NOT NULL | `text`、`code`、`image`、`file`、`agent` |
| `content` | TEXT | NOT NULL | 消息正文或展示文本 |
| `metadata` | JSONB | NOT NULL DEFAULT '{}' | 特定消息类型的扩展信息 |
| `created_at` | TIMESTAMPTZ | NOT NULL | 发送时间 |
| `deleted_at` | TIMESTAMPTZ | NULL | 软删除时间 |

建议索引：

- `INDEX (channel_id, id DESC)`：频道历史消息分页。
- `INDEX (sender_id, created_at DESC)`：用户消息查询。
- `INDEX (reply_to_id)`：回复关系查询。

### channel_reads

记录每个用户在每个频道的已读位置，用于登录后恢复未读数和跨设备同步。

| 字段名 | 类型 | 约束 | 描述 |
| --- | --- | --- | --- |
| `user_id` | UUID | PK, FK(users.id) | 用户 ID |
| `channel_id` | UUID | PK, FK(channels.id) | 频道 ID |
| `last_read_message_id` | BIGINT | NOT NULL DEFAULT 0 | 用户已读到的最大消息 ID |
| `created_at` | TIMESTAMPTZ | NOT NULL | 创建时间 |
| `updated_at` | TIMESTAMPTZ | NOT NULL | 最近更新已读位置的时间 |

建议索引：

- `INDEX (channel_id, user_id)`：频道内按用户查询已读位置。

`metadata` 示例：

```json
{
  "language": "go",
  "file_id": "uuid",
  "width": 1280,
  "height": 720
}
```

### channel_members_presence

用于记录当前或近期在线状态。也可以只存在 Redis；如果需要跨重启恢复或展示最近在线时间，可落库。

| 字段名 | 类型 | 约束 | 描述 |
| --- | --- | --- | --- |
| `user_id` | UUID | PK, FK(users.id) | 用户 ID |
| `status` | VARCHAR(20) | NOT NULL | `online`、`offline`、`away` |
| `last_seen_at` | TIMESTAMPTZ | NOT NULL | 最近活跃时间 |

## 4. 异步动态 Drops

### drops

| 字段名 | 类型 | 约束 | 描述 |
| --- | --- | --- | --- |
| `id` | UUID | PK | 动态唯一标识 |
| `author_id` | UUID | FK(users.id), NOT NULL | 发布者 ID |
| `content` | TEXT | NOT NULL | Markdown 正文 |
| `like_count` | INT | NOT NULL DEFAULT 0 | 点赞冗余计数 |
| `comment_count` | INT | NOT NULL DEFAULT 0 | 评论冗余计数 |
| `created_at` | TIMESTAMPTZ | NOT NULL | 发布时间 |
| `updated_at` | TIMESTAMPTZ | NOT NULL | 更新时间 |
| `deleted_at` | TIMESTAMPTZ | NULL | 软删除时间 |

建议索引：

- `INDEX (created_at DESC)`
- `INDEX (author_id, created_at DESC)`

### drop_likes

| 字段名 | 类型 | 约束 | 描述 |
| --- | --- | --- | --- |
| `id` | BIGSERIAL | PK | 自增主键 |
| `drop_id` | UUID | FK(drops.id), NOT NULL | 被点赞的动态 ID |
| `user_id` | UUID | FK(users.id), NOT NULL | 点赞用户 ID |
| `created_at` | TIMESTAMPTZ | NOT NULL | 点赞时间 |

建议索引与约束：

- `UNIQUE (drop_id, user_id)`：同一用户对同一动态只能点赞一次。
- `INDEX (user_id, created_at DESC)`

### drop_comments

| 字段名 | 类型 | 约束 | 描述 |
| --- | --- | --- | --- |
| `id` | UUID | PK | 评论唯一标识 |
| `drop_id` | UUID | FK(drops.id), NOT NULL | 归属动态 ID |
| `user_id` | UUID | FK(users.id), NOT NULL | 评论发布者 ID |
| `parent_id` | UUID | FK(drop_comments.id), NULL | 回复的父评论；NULL 表示直接评论动态 |
| `reply_to_user_id` | UUID | FK(users.id), NULL | 被回复用户，便于展示“张三 回复 李四” |
| `content` | TEXT | NOT NULL | 评论正文 |
| `created_at` | TIMESTAMPTZ | NOT NULL | 评论时间 |
| `updated_at` | TIMESTAMPTZ | NOT NULL | 更新时间 |
| `deleted_at` | TIMESTAMPTZ | NULL | 软删除时间 |

建议索引：

- `INDEX (drop_id, created_at ASC)`
- `INDEX (parent_id, created_at ASC)`
- `INDEX (user_id, created_at DESC)`

## 5. 共享资源库 Workspace

### workspace_files

文件与文件夹使用同一张表建模。

| 字段名 | 类型 | 约束 | 描述 |
| --- | --- | --- | --- |
| `id` | UUID | PK | 文件或目录唯一标识 |
| `parent_id` | UUID | FK(workspace_files.id), NULL | 父目录 ID；NULL 表示根目录 |
| `is_folder` | BOOLEAN | NOT NULL | 是否为文件夹 |
| `name` | VARCHAR(255) | NOT NULL | 文件名或目录名 |
| `uploader_id` | UUID | FK(users.id), NOT NULL | 上传者或创建者 ID |
| `storage_type` | VARCHAR(20) | NULL | 文件存储驱动；目录为空 |
| `object_key` | VARCHAR(500) | NULL | 对象存储 key；目录为空 |
| `size_bytes` | BIGINT | NOT NULL DEFAULT 0 | 文件大小；目录可为 0 |
| `mime_type` | VARCHAR(100) | NULL | MIME 类型 |
| `file_hash` | VARCHAR(128) | NULL | SHA-256 等内容哈希；目录为空 |
| `metadata` | JSONB | NOT NULL DEFAULT '{}' | 预览、图片尺寸等扩展信息 |
| `created_at` | TIMESTAMPTZ | NOT NULL | 创建时间 |
| `updated_at` | TIMESTAMPTZ | NOT NULL | 更新时间 |
| `deleted_at` | TIMESTAMPTZ | NULL | 进入回收站时间 |
| `deleted_by` | UUID | FK(users.id), NULL | 删除操作者 |

建议索引与约束：

- `INDEX (parent_id, is_folder, name)`：目录列表。
- `INDEX (file_hash)`：秒传查询。
- `INDEX (uploader_id, created_at DESC)`：用户上传记录。
- 同一目录下未删除文件建议避免重名：可通过部分唯一索引实现 `UNIQUE (parent_id, name) WHERE deleted_at IS NULL`。

注意：

- 移动文件夹时需要防止把目录移动到自己的子目录中。
- 删除文件夹时一期采用递归软删除，并在回收站中展示。
- 恢复文件夹时需要同时恢复其子文件与子目录；如果原路径发生重名冲突，后端应要求用户指定新名称或新目录。
- 彻底删除时再删除对象存储中的实际文件内容。

### workspace_file_versions

如果一期支持 `.md` 在线编辑，建议记录简单版本历史；如果希望先简化，可暂缓实现。

| 字段名 | 类型 | 约束 | 描述 |
| --- | --- | --- | --- |
| `id` | UUID | PK | 版本 ID |
| `file_id` | UUID | FK(workspace_files.id), NOT NULL | 文件 ID |
| `editor_id` | UUID | FK(users.id), NOT NULL | 编辑者 ID |
| `object_key` | VARCHAR(500) | NOT NULL | 该版本对象 key |
| `file_hash` | VARCHAR(128) | NOT NULL | 该版本内容哈希 |
| `size_bytes` | BIGINT | NOT NULL | 文件大小 |
| `created_at` | TIMESTAMPTZ | NOT NULL | 版本创建时间 |

建议索引：

- `INDEX (file_id, created_at DESC)`

## 6. AI 助手与搜索

### agent_conversations

| 字段名 | 类型 | 约束 | 描述 |
| --- | --- | --- | --- |
| `id` | UUID | PK | AI 对话 ID |
| `user_id` | UUID | FK(users.id), NOT NULL | 发起用户 |
| `title` | VARCHAR(200) | NOT NULL | 对话标题 |
| `created_at` | TIMESTAMPTZ | NOT NULL | 创建时间 |
| `updated_at` | TIMESTAMPTZ | NOT NULL | 更新时间 |
| `deleted_at` | TIMESTAMPTZ | NULL | 软删除时间 |

建议索引：

- `INDEX (user_id, updated_at DESC)`

### agent_messages

| 字段名 | 类型 | 约束 | 描述 |
| --- | --- | --- | --- |
| `id` | UUID | PK | AI 消息 ID |
| `conversation_id` | UUID | FK(agent_conversations.id), NOT NULL | 所属对话 |
| `role` | VARCHAR(20) | NOT NULL | `user`、`assistant`、`system`、`tool` |
| `content` | TEXT | NOT NULL | 消息内容 |
| `metadata` | JSONB | NOT NULL DEFAULT '{}' | 模型、引用来源、工具调用等 |
| `created_at` | TIMESTAMPTZ | NOT NULL | 创建时间 |

建议索引：

- `INDEX (conversation_id, created_at ASC)`

### search_documents

用于统一索引聊天、动态、文件文本等可搜索内容。一期使用 PostgreSQL 全文搜索，后续再扩展向量检索。

| 字段名 | 类型 | 约束 | 描述 |
| --- | --- | --- | --- |
| `id` | UUID | PK | 索引文档 ID |
| `source_type` | VARCHAR(30) | NOT NULL | `message`、`drop`、`file` |
| `source_id` | VARCHAR(100) | NOT NULL | 源对象 ID；消息可存 BIGINT 字符串 |
| `title` | TEXT | NULL | 标题或展示名 |
| `content` | TEXT | NOT NULL | 可搜索文本 |
| `tsv` | TSVECTOR | NULL | PostgreSQL 全文搜索向量 |
| `metadata` | JSONB | NOT NULL DEFAULT '{}' | 来源上下文 |
| `created_at` | TIMESTAMPTZ | NOT NULL | 创建时间 |
| `updated_at` | TIMESTAMPTZ | NOT NULL | 更新时间 |

建议索引与约束：

- `UNIQUE (source_type, source_id)`
- `GIN (tsv)`
- `INDEX (source_type, created_at DESC)`

## 7. 操作日志（后续可选）

### audit_logs

如果后续加入管理员、权限或误删恢复，建议增加操作日志。

| 字段名 | 类型 | 约束 | 描述 |
| --- | --- | --- | --- |
| `id` | BIGSERIAL | PK | 日志 ID |
| `actor_id` | UUID | FK(users.id), NOT NULL | 操作用户 |
| `action` | VARCHAR(100) | NOT NULL | 操作类型 |
| `target_type` | VARCHAR(50) | NOT NULL | 目标类型 |
| `target_id` | VARCHAR(100) | NOT NULL | 目标 ID |
| `metadata` | JSONB | NOT NULL DEFAULT '{}' | 操作上下文 |
| `created_at` | TIMESTAMPTZ | NOT NULL | 操作时间 |

建议索引：

- `INDEX (actor_id, created_at DESC)`
- `INDEX (target_type, target_id)`

## 8. Redis 使用建议

Redis 不作为主数据源，主要用于实时与短期状态：

- WebSocket 在线连接映射。
- 用户在线状态与心跳过期。
- Redis Pub/Sub 频道广播 fanout。
- AI 任务队列或短期任务状态。
- 限流计数。

## 9. 对象存储结构建议

MinIO/S3 object key 建议使用稳定、不可猜测路径：

```text
workspace/{yyyy}/{mm}/{file_id}/{version_id_or_hash}/{filename}
```

数据库中的 `workspace_files.object_key` 指向当前版本；如果启用版本表，历史版本保存在 `workspace_file_versions.object_key`。
