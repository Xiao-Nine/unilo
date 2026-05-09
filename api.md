# Unilo API 接口文档 v1.0

## 0. 全局规范

### 0.1 服务地址与基础路径

前端页面/APP 首次启动时，用户需要输入后端服务地址 `server_url` 和 secret key。验证通过后，后续 REST API 和 WebSocket 都基于该 `server_url` 访问。

- REST API Base URL：`${server_url}/api/v1`
- WebSocket URL：`${server_url}/api/v1/ws?token=<Access_Token>`
- 请求与响应默认使用 `application/json`。
- 文件上传使用 `multipart/form-data`。

### 0.2 鉴权

- 服务接入验证：前端首次接入时调用 `${server_url}/api/v1/auth/verify`，提交用户输入的 secret key。
- 公开认证接口：`POST /auth/verify`、`POST /auth/register`、`POST /auth/login`、`POST /auth/refresh`、`POST /auth/logout`。
- 受保护接口：除上述公开认证接口外，所有接口都需要 `Authorization: Bearer <Access_Token>`；其中 `GET /auth/me` 虽位于 `/auth` 前缀下，也需要 Bearer Access Token。
- Token 策略：Access Token + Refresh Token。
- WebSocket 连接通过 query 参数中的 access token 鉴权。

### 0.3 标准响应格式

所有 REST 接口统一返回：

```json
{
  "code": 200,
  "msg": "success",
  "data": {}
}
```

`code` 使用业务码：

| code | 含义 |
| --- | --- |
| 200 | 成功 |
| 400 | 参数错误 |
| 401 | 未登录或 token 无效 |
| 403 | 无权限或密钥错误 |
| 404 | 资源不存在 |
| 409 | 资源冲突，例如用户名重复 |
| 500 | 服务器内部错误 |

HTTP 状态码建议与业务码保持一致；如果网关或框架限制，也可以统一 HTTP 200 并通过 `code` 判断业务结果。

### 0.4 健康检查

后端提供公开健康检查接口，用于本地开发和部署探活。

- `GET /healthz`
- `GET /api/v1/healthz`

Response Data：

```json
{
  "status": "ok"
}
```

### 0.5 分页约定

两类分页：

1. 游标分页：用于消息流等实时插入场景。
2. 页码分页：用于动态流等普通列表。

游标分页响应：

```json
{
  "items": [],
  "next_cursor": "string_or_number",
  "has_more": true
}
```

页码分页响应：

```json
{
  "total": 100,
  "page": 1,
  "size": 20,
  "items": []
}
```

### 0.6 通用对象

#### User

```json
{
  "id": "uuid",
  "username": "feng",
  "nickname": "Feng",
  "avatar_url": "",
  "created_at": "2026-05-04T12:00:00Z"
}
```

#### Channel

```json
{
  "id": "uuid",
  "name": "日常闲聊",
  "created_by": "uuid",
  "created_at": "2026-05-04T12:00:00Z",
  "updated_at": "2026-05-04T12:00:00Z",
  "last_message_id": 1024,
  "last_read_message_id": 1001,
  "unread_count": 23
}
```

#### Message

```json
{
  "id": 1024,
  "channel_id": "uuid",
  "sender_id": "uuid",
  "sender": {
    "id": "uuid",
    "nickname": "Feng",
    "avatar_url": ""
  },
  "reply_to_id": null,
  "msg_type": "text",
  "content": "大家好",
  "metadata": {},
  "created_at": "2026-05-04T12:00:00Z"
}
```

## 1. 认证模块 Auth

### 1.1 服务接入校验

用于前端页面/APP 首次启动时验证用户输入的后端服务地址和 secret key 是否正确。前端应向用户输入的 `server_url` 发起请求：`${server_url}/api/v1/auth/verify`。验证通过后，前端进入登录/注册页面。

- Method：`POST`
- Path：`/auth/verify`

Request Body：

```json
{
  "secret_key": "string"
}
```

Response Data：

```json
{
  "is_valid": true,
  "server_name": "Unilo",
  "server_version": "0.1.0",
  "api_base_url": "https://unilo.example.com/api/v1"
}
```

前端处理规则：

- `is_valid = true`：保存 `server_url`、`server_name`、`api_base_url`，进入登录/注册页面。
- `is_valid = false` 或返回业务码 `403`：提示 secret key 错误。
- 请求超时、网络错误或响应格式不符合规范：提示后端服务地址不可用。

### 1.2 用户注册

- Method：`POST`
- Path：`/auth/register`

Request Body：

```json
{
  "username": "string",
  "password": "string",
  "nickname": "string"
}
```

Response Data：

```json
{
  "user": {
    "id": "uuid",
    "username": "feng",
    "nickname": "Feng",
    "avatar_url": "",
    "created_at": "2026-05-04T12:00:00Z"
  }
}
```

### 1.3 用户登录

- Method：`POST`
- Path：`/auth/login`

Request Body：

```json
{
  "username": "string",
  "password": "string"
}
```

Response Data：

```json
{
  "access_token": "eyJhbGciOi...",
  "refresh_token": "eyJhbGciOi...",
  "access_token_expires_at": "2026-05-04T14:00:00Z",
  "refresh_token_expires_at": "2026-05-11T12:00:00Z",
  "user": {
    "id": "uuid",
    "username": "feng",
    "nickname": "Feng",
    "avatar_url": ""
  }
}
```

### 1.4 刷新 Token

- Method：`POST`
- Path：`/auth/refresh`

Request Body：

```json
{
  "refresh_token": "eyJhbGciOi..."
}
```

Response Data：

```json
{
  "access_token": "eyJhbGciOi...",
  "refresh_token": "eyJhbGciOi...",
  "access_token_expires_at": "2026-05-04T14:00:00Z",
  "refresh_token_expires_at": "2026-05-11T12:00:00Z"
}
```

### 1.5 登出

- Method：`POST`
- Path：`/auth/logout`

Request Body：

```json
{
  "refresh_token": "eyJhbGciOi..."
}
```

Response Data：

```json
{
  "logged_out": true
}
```

### 1.6 获取当前用户

该接口需要 `Authorization: Bearer <Access_Token>`。

- Method：`GET`
- Path：`/auth/me`

Response Data：`User`

## 2. 频道聊天模块 Channels

### 2.1 获取频道列表

- Method：`GET`
- Path：`/channels`

Response Data：

```json
[
  {
    "id": "uuid",
    "name": "日常闲聊",
    "created_by": "uuid",
    "created_at": "2026-05-04T12:00:00Z",
    "updated_at": "2026-05-04T12:00:00Z",
    "last_message_id": 1024,
    "last_read_message_id": 1001,
    "unread_count": 23
  }
]
```

### 2.2 创建频道

- Method：`POST`
- Path：`/channels`

Request Body：

```json
{
  "name": "技术分享"
}
```

Response Data：`Channel`

创建成功后，服务端通过 WebSocket 广播 `channel_created`。

### 2.3 重命名频道

- Method：`PATCH`
- Path：`/channels/:channel_id`

Request Body：

```json
{
  "name": "新频道名"
}
```

Response Data：`Channel`

成功后广播 `channel_updated`。

### 2.4 删除频道

- Method：`DELETE`
- Path：`/channels/:channel_id`

Response Data：

```json
{
  "deleted": true
}
```

成功后广播 `channel_deleted`。一期建议后端软删除频道。

### 2.5 拉取历史消息

用于首次进入频道或向上滚动加载历史消息。

- Method：`GET`
- Path：`/channels/:channel_id/messages`

Query Params：

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| cursor | integer | 否 | 上一页最后一条消息 ID；不传或传 0 表示拉取最新消息 |
| limit | integer | 否 | 默认 50，最大 100 |

Response Data：

```json
{
  "messages": [
    {
      "id": 1024,
      "channel_id": "uuid",
      "sender_id": "uuid",
      "sender": {
        "id": "uuid",
        "nickname": "Feng",
        "avatar_url": ""
      },
      "reply_to_id": null,
      "msg_type": "text",
      "content": "大家好，这是第一条消息",
      "metadata": {},
      "created_at": "2026-05-04T12:00:00Z"
    }
  ],
  "next_cursor": 974,
  "has_more": true
}
```

### 2.6 发送消息

消息主要通过 WebSocket 发送；如需兼容弱网络或调试，也可以保留 REST 接口。

- Method：`POST`
- Path：`/channels/:channel_id/messages`

Request Body：

```json
{
  "reply_to_id": null,
  "msg_type": "text",
  "content": "Hello World",
  "metadata": {}
}
```

Response Data：`Message`

### 2.7 标记频道已读

客户端进入频道、滚动到底部或确认已看到新消息时调用。服务端只会向前推进已读位置，不会回退 `last_read_message_id`。

- Method：`POST`
- Path：`/channels/:channel_id/read`

Request Body：

```json
{
  "last_read_message_id": 1024
}
```

Response Data：

```json
{
  "channel_id": "uuid",
  "last_read_message_id": 1024,
  "unread_count": 0
}
```

## 3. 异步动态模块 Drops

本模块接口均需要 `Authorization: Bearer <Access_Token>`。

通用错误：

- `400`：请求体、分页参数或 UUID 参数无效。
- `401`：未登录或 Access Token 无效。
- `403`：尝试删除非本人发布的动态或评论。
- `404`：动态或评论不存在，或已被软删除。

### 3.1 获取动态列表

- Method：`GET`
- Path：`/drops`

Query Params：

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| page | integer | 否 | 默认 1 |
| size | integer | 否 | 默认 20，最大 100 |

Response Data：

```json
{
  "total": 105,
  "page": 1,
  "size": 20,
  "items": [
    {
      "id": "uuid",
      "author_id": "uuid",
      "author": {
        "id": "uuid",
        "nickname": "Feng",
        "avatar_url": ""
      },
      "content": "这是一条 Markdown 动态",
      "like_count": 5,
      "comment_count": 2,
      "is_liked_by_me": true,
      "created_at": "2026-05-04T12:00:00Z",
      "updated_at": "2026-05-04T12:00:00Z"
    }
  ]
}
```

### 3.2 发布动态

- Method：`POST`
- Path：`/drops`

Request Body：

```json
{
  "content": "支持 Markdown 的正文"
}
```

Response Data：Drop 对象。

### 3.3 获取动态详情

- Method：`GET`
- Path：`/drops/:drop_id`

Response Data：Drop 对象，包含按创建时间升序排列的未删除评论列表。

### 3.4 删除动态

- Method：`DELETE`
- Path：`/drops/:drop_id`

说明：仅动态作者可以删除；删除采用软删除，删除后列表和详情不再返回该动态。

Response Data：

```json
{
  "deleted": true
}
```

### 3.5 动态点赞 / 取消点赞

- Method：`POST`
- Path：`/drops/:drop_id/like`

说明：toggle 语义；如果已经点赞则取消，否则点赞。

Response Data：

```json
{
  "current_like_count": 6,
  "is_liked": true
}
```

### 3.6 动态发表评论

- Method：`POST`
- Path：`/drops/:drop_id/comments`

Request Body：

```json
{
  "content": "评论正文",
  "parent_id": "uuid_or_null",
  "reply_to_user_id": "uuid_or_null"
}
```

Response Data：Comment 对象。

### 3.7 删除评论

- Method：`DELETE`
- Path：`/drops/:drop_id/comments/:comment_id`

说明：仅评论作者可以删除；删除采用软删除，并同步减少动态的 `comment_count`。

Response Data：

```json
{
  "deleted": true
}
```

## 4. 共享资源库模块 Workspace

本模块接口均需要 `Authorization: Bearer <Access_Token>`。当前后端已实现目录列表、创建文件夹、文件 hash 检查、上传、下载、预览、重命名、移动、软删除、回收站列表、恢复、彻底删除和可编辑文本内容保存。

文件下载和预览接口返回文件流，不使用统一 JSON envelope；其他接口使用统一 JSON envelope。

### 4.1 获取目录下文件列表

- Method：`GET`
- Path：`/workspace/files`

Query Params：

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| parent_id | uuid | 否 | 不传表示根目录 |

Response Data：

```json
{
  "breadcrumbs": [
    { "id": "root", "name": "根目录" },
    { "id": "uuid", "name": "当前目录名" }
  ],
  "files": [
    {
      "id": "uuid",
      "parent_id": "uuid_or_null",
      "is_folder": false,
      "name": "架构图.png",
      "uploader_id": "uuid",
      "size_bytes": 204800,
      "mime_type": "image/png",
      "file_hash": "sha256...",
      "preview_url": "/api/v1/workspace/files/uuid/preview",
      "download_url": "/api/v1/workspace/files/uuid/download",
      "created_at": "2026-05-04T12:00:00Z",
      "updated_at": "2026-05-04T12:00:00Z"
    }
  ]
}
```

### 4.2 创建文件夹

- Method：`POST`
- Path：`/workspace/folders`

Request Body：

```json
{
  "name": "设计资料",
  "parent_id": "uuid_or_null"
}
```

Response Data：File 对象，`is_folder = true`。

### 4.3 文件哈希预检

上传大文件前，前端先计算 SHA-256，判断服务端是否已有相同内容。

- Method：`POST`
- Path：`/workspace/files/check`

Request Body：

```json
{
  "file_hash": "sha256...",
  "name": "example.pdf",
  "parent_id": "uuid_or_null"
}
```

Response Data：

```json
{
  "exists": true,
  "file_id": "uuid_or_null",
  "file": {}
}
```

### 4.4 上传文件流

- Method：`POST`
- Path：`/workspace/files/upload`
- Content-Type：`multipart/form-data`

Form Data：

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| file | binary | 是 | 文件内容 |
| parent_id | uuid | 否 | 目标目录 |
| file_hash | string | 是 | SHA-256 |

Response Data：File 对象。

### 4.5 下载文件

- Method：`GET`
- Path：`/workspace/files/:file_id/download`

Response：文件流，`Content-Disposition` 为 attachment。

### 4.6 预览文件

- Method：`GET`
- Path：`/workspace/files/:file_id/preview`

Response：适合预览的文件流，`Content-Disposition` 为 inline。当前支持文本、Markdown、图片和 PDF；不支持的类型返回 `400`。

### 4.7 重命名文件或文件夹

- Method：`PATCH`
- Path：`/workspace/files/:file_id`

Request Body：

```json
{
  "name": "新名称.md"
}
```

Response Data：File 对象。

### 4.8 移动文件或文件夹

- Method：`POST`
- Path：`/workspace/files/:file_id/move`

Request Body：

```json
{
  "target_parent_id": "uuid_or_null"
}
```

Response Data：File 对象。

说明：目标目录必须存在且是文件夹；移动文件夹时不能移动到自己或自己的子孙目录；同目录重名返回 `409`。

### 4.9 删除文件或文件夹

- Method：`DELETE`
- Path：`/workspace/files/:file_id`

Response Data：

```json
{
  "deleted": true,
  "trash_id": "uuid"
}
```

删除后文件进入回收站，当前实现采用软删除并从普通列表和搜索结果中隐藏。

### 4.10 获取回收站列表

- Method：`GET`
- Path：`/workspace/trash`

Response Data：

```json
{
  "files": []
}
```

### 4.11 从回收站恢复

- Method：`POST`
- Path：`/workspace/files/:file_id/restore`

Request Body：

```json
{
  "target_parent_id": "uuid_or_null"
}
```

Response Data：File 对象。

说明：可传 `target_parent_id` 指定恢复目录；不传时优先恢复到原父目录，原父目录不可用时恢复到根目录；同目录重名返回 `409`。

### 4.12 彻底删除

- Method：`DELETE`
- Path：`/workspace/files/:file_id/purge`

Response Data：

```json
{
  "purged": true
}
```

说明：仅能彻底删除已经在回收站中的条目；文件对象和版本对象会从对象存储删除，数据库元数据会永久删除。

### 4.13 保存可编辑文本文件

用于 `.md`、`.txt` 等一期允许在线编辑的文件。

- Method：`PUT`
- Path：`/workspace/files/:file_id/content`

Request Body：

```json
{
  "content": "# Markdown 内容",
  "file_hash": "new_sha256"
}
```

Response Data：File 对象。

说明：当前支持 Markdown、纯文本、JSON、XML、YAML、CSV 和其他 `text/*` 文件。服务端会计算 SHA-256；如果请求传入 `file_hash`，必须与内容一致。保存后会创建文件版本记录，并把文本内容写入搜索索引。

## 5. AI 助手模块 Agent

本模块接口均需要 `Authorization: Bearer <Access_Token>`。同步 REST 调用仍可用；异步调用会创建 Agent Run，返回 `queued` 状态，并可通过 REST 轮询或 WebSocket `agent_message` / `agent_delta` 接收状态与流式输出。传入频道上下文时，后端会按 `channel_id` 检索该频道消息作为模型上下文。

### 5.1 创建 AI 对话

- Method：`POST`
- Path：`/agent/conversations`

Request Body：

```json
{
  "title": "帮我整理项目讨论"
}
```

Response Data：

```json
{
  "id": "uuid",
  "title": "帮我整理项目讨论",
  "created_at": "2026-05-04T12:00:00Z",
  "updated_at": "2026-05-04T12:00:00Z"
}
```

### 5.2 获取 AI 对话列表

- Method：`GET`
- Path：`/agent/conversations`

Query Params：

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| page | integer | 否 | 默认 1 |
| size | integer | 否 | 默认 20，最大 100 |

Response Data：

```json
{
  "total": 12,
  "page": 1,
  "size": 20,
  "items": [
    {
      "id": "uuid",
      "title": "帮我整理项目讨论",
      "created_at": "2026-05-04T12:00:00Z",
      "updated_at": "2026-05-04T12:03:00Z"
    }
  ]
}
```

说明：仅返回当前登录用户的对话，按 `updated_at` 倒序排列。

### 5.3 发送 AI 消息

- Method：`POST`
- Path：`/agent/conversations/:conversation_id/messages`

Request Body：

```json
{
  "prompt": "总结一下今天频道里的讨论",
  "context": {
    "type": "channel",
    "channel_id": "uuid"
  }
}
```

Response Data：

```json
{
  "status": "completed",
  "conversation_id": "uuid",
  "user_message": {
    "id": "uuid",
    "role": "user",
    "content": "总结一下今天频道里的讨论",
    "metadata": {
      "context": {
        "type": "channel",
        "channel_id": "uuid"
      }
    },
    "created_at": "2026-05-04T12:00:00Z"
  },
  "assistant_message": {
    "id": "uuid",
    "role": "assistant",
    "content": "这里是总结内容...",
    "metadata": {
      "model": "gpt-4o-mini",
      "usage": {},
      "citations": []
    },
    "created_at": "2026-05-04T12:00:03Z"
  }
}
```

说明：同步调用成功时返回 `completed`。如果模型提供方调用失败，已保存的用户消息会保留，助手消息不会伪造。需要后台执行、轮询或 WebSocket 推送时，使用 Agent Run 接口。

### 5.4 异步提交 AI Run

- Method：`POST`
- Path：`/agent/conversations/:conversation_id/runs`

Request Body：同 `发送 AI 消息`。

Response Data：

```json
{
  "run_id": "uuid",
  "status": "queued",
  "conversation_id": "uuid",
  "user_message": {
    "id": "uuid",
    "role": "user",
    "content": "总结一下今天频道里的讨论",
    "metadata": {
      "context": {
        "type": "channel",
        "channel_id": "uuid"
      }
    },
    "created_at": "2026-05-04T12:00:00Z"
  }
}
```

说明：Run 会在后台执行，状态包括 `queued`、`running`、`streaming`、`completed`、`failed`。用户消息会立即持久化；模型失败时 Run 变为 `failed`，不会伪造助手消息。

### 5.5 获取 AI Run 状态

- Method：`GET`
- Path：`/agent/runs/:run_id`

Response Data：

```json
{
  "id": "uuid",
  "conversation_id": "uuid",
  "status": "completed",
  "user_message": {},
  "assistant_message": {},
  "metadata": {
    "context": {},
    "model": "gpt-4o-mini",
    "usage": {},
    "citations": []
  },
  "created_at": "2026-05-04T12:00:00Z",
  "started_at": "2026-05-04T12:00:01Z",
  "completed_at": "2026-05-04T12:00:03Z",
  "updated_at": "2026-05-04T12:00:03Z"
}
```

说明：仅允许查询当前登录用户自己的 Run。失败时 `status` 为 `failed`，并返回 `error`。

### 5.6 获取 AI 对话消息历史

- Method：`GET`
- Path：`/agent/conversations/:conversation_id/messages`

Query Params：

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| cursor | string | 否 | RFC3339Nano 时间戳，读取该时间之前的更早消息 |
| limit | integer | 否 | 默认 50，最大 100 |

Response Data：

```json
{
  "messages": [
    {
      "id": "uuid",
      "role": "user",
      "content": "总结一下今天频道里的讨论",
      "metadata": {
        "context": {
          "type": "channel",
          "channel_id": "uuid"
        }
      },
      "created_at": "2026-05-04T12:00:00Z"
    },
    {
      "id": "uuid",
      "role": "assistant",
      "content": "这里是总结内容...",
      "metadata": {
        "model": "gpt-4o-mini",
        "usage": {},
        "citations": []
      },
      "created_at": "2026-05-04T12:00:03Z"
    }
  ],
  "next_cursor": "2026-05-04T12:00:00Z",
  "has_more": true
}
```

说明：仅允许读取当前登录用户自己的对话消息。返回结果按 `created_at` 正序排列；继续翻页时将 `next_cursor` 作为下一次请求的 `cursor`。

### 5.7 搜索站内内容

该接口需要 `Authorization: Bearer <Access_Token>`。一期使用 PostgreSQL 全文搜索索引消息、动态、Workspace 文件名和可编辑文本文件内容；Agent MVP 会复用该搜索结果作为模型上下文。

- Method：`GET`
- Path：`/search`

Query Params：

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| q | string | 是 | 搜索关键词 |
| type | string | 否 | `all`、`messages`、`drops`、`files`；默认 `all` |
| limit | integer | 否 | 默认 20，最大 100 |
| channel_id | string | 否 | 仅用于消息搜索；按频道消息索引过滤 |

Response Data：

```json
{
  "items": [
    {
      "type": "message",
      "id": "1024",
      "title": "日常闲聊中的消息",
      "snippet": "...关键词...",
      "created_at": "2026-05-04T12:00:00Z"
    }
  ]
}
```

## 6. WebSocket 实时通信协议

### 6.1 通用格式

Client -> Server：

```json
{
  "action": "send_message",
  "request_id": "client-generated-id",
  "payload": {}
}
```

Server -> Client：

```json
{
  "event": "new_message",
  "request_id": "client-generated-id-or-null",
  "data": {}
}
```

错误事件：

```json
{
  "event": "error",
  "request_id": "client-generated-id-or-null",
  "data": {
    "code": 400,
    "msg": "invalid payload"
  }
}
```

### 6.2 Client -> Server Actions

#### 发送频道消息

```json
{
  "action": "send_message",
  "request_id": "uuid",
  "payload": {
    "channel_id": "uuid",
    "reply_to_id": null,
    "msg_type": "text",
    "content": "Hello World",
    "metadata": {}
  }
}
```

#### 输入状态

```json
{
  "action": "typing",
  "payload": {
    "channel_id": "uuid",
    "is_typing": true
  }
}
```

#### 触发 AI 助手

```json
{
  "action": "invoke_agent",
  "request_id": "uuid",
  "payload": {
    "conversation_id": "uuid_or_null",
    "channel_id": "uuid_or_null",
    "prompt": "帮我查一下昨天关于 Golang 架构的聊天记录",
    "context_type": "channel",
    "title": "可选的新对话标题"
  }
}
```

#### 心跳

```json
{
  "action": "ping",
  "request_id": "uuid",
  "payload": {}
}
```

服务端收到后返回 `pong` 事件：

```json
{
  "event": "pong",
  "request_id": "uuid",
  "data": {
    "ok": true
  }
}
```

### 6.3 Server -> Client Events

#### 新消息广播

```json
{
  "event": "new_message",
  "data": {
    "id": 1025,
    "channel_id": "uuid",
    "sender_id": "uuid",
    "sender": {
      "id": "uuid",
      "nickname": "Feng",
      "avatar_url": ""
    },
    "reply_to_id": null,
    "msg_type": "text",
    "content": "这是一条新消息",
    "metadata": {},
    "created_at": "2026-05-04T12:05:00Z"
  }
}
```

#### 频道变更

`channel_created` 与 `channel_updated` 的 `data` 为 `Channel` 对象：

```json
{
  "event": "channel_created",
  "data": {
    "id": "uuid",
    "name": "新频道",
    "created_by": "uuid",
    "created_at": "2026-05-04T12:00:00Z",
    "updated_at": "2026-05-04T12:00:00Z",
    "last_message_id": 0,
    "last_read_message_id": 0,
    "unread_count": 0
  }
}
```

`channel_deleted` 的 `data`：

```json
{
  "event": "channel_deleted",
  "data": {
    "id": "uuid"
  }
}
```

#### 已读位置同步

同一用户在任一客户端标记频道已读后，服务端会向该用户的在线连接推送同步事件。

```json
{
  "event": "channel_read_updated",
  "data": {
    "channel_id": "uuid",
    "last_read_message_id": 1024,
    "unread_count": 0
  }
}
```

#### 输入状态广播

```json
{
  "event": "typing_status",
  "data": {
    "channel_id": "uuid",
    "user_id": "uuid",
    "is_typing": true
  }
}
```

#### 在线用户变更

```json
{
  "event": "presence_updated",
  "data": {
    "user_id": "uuid",
    "status": "online"
  }
}
```

说明：在线状态按用户连接数统计；启用 Redis realtime 时，presence 会跨后端实例同步。

#### AI Run 状态消息

```json
{
  "event": "agent_message",
  "request_id": "uuid_or_null",
  "data": {
    "run_id": "uuid",
    "conversation_id": "uuid",
    "channel_id": "uuid_or_null",
    "status": "queued",
    "user_message": {},
    "assistant_message": null,
    "content": "",
    "error": "",
    "is_final": false
  }
}
```

说明：`agent_message` 会推送 `queued`、`running`、`streaming`、`completed`、`failed` 状态。Agent 事件为用户定向投递，不广播给其他用户。

#### AI 流式片段

```json
{
  "event": "agent_delta",
  "data": {
    "run_id": "uuid",
    "conversation_id": "uuid",
    "channel_id": "uuid_or_null",
    "status": "streaming",
    "delta": "局部输出",
    "is_final": false
  }
}
```
