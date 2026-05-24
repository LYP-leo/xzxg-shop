# 动态配置拆分与 Prompt 管理设计 v1

更新时间：2026-05-24

实现状态：已落地第一版。

- 后端 `configcenter.NacosCenter` 已支持聚合读取旧 `xzxg-shop-app-config.json` 以及新的 `xzxg-shop-app-config.json`、`xzxg-shop-rag-config.json`、`xzxg-shop-infra-config.json`、`xzxg-shop-agent-prompts.json`。
- 普通动态配置保存时会按 key 自动写入对应 dataId；Prompt 会写入 `xzxg-shop-agent-prompts.json` 的 `prompts` 数组。
- 新增 MySQL 表 `agent_prompts`、`agent_prompt_publish_records`，启动时会从当前 Nacos/default prompt 初始化 active 版本。
- 新增管理员 Prompt 管理页，支持保存草稿、发布到 Nacos、查看最近发布记录。
- 普通动态配置页已过滤 `agent.prompt.*`，Prompt 不再和 RAG/Infra/App 配置混在一个表里。

## 背景

当前 Nacos 里主要使用一个 `xzxg-shop-app-config.json`，里面混放了：

- 模型配置
- Milvus / Embedding 配置
- RAG 召回与重排配置
- Agent Prompt
- Agent 行为开关

这种方式短期方便，但后续会有几个问题：

1. 配置职责混乱，管理员页面难以按功能维护。
2. Prompt 体积较大，和普通 key-value 配置混在一起不利于版本管理。
3. RAG 参数调优、模型切换、Prompt 编辑是不同权限和发布节奏。
4. 后续如果要做 Prompt 审核、版本回滚、灰度发布，单文件配置会很难扩展。

## 目标

1. Nacos 配置按功能域拆分。
2. Agent Prompt 支持管理员页面编辑。
3. Prompt 可以落库，支持版本、启停、回滚。
4. 保留当前配置读取兼容能力，避免一次性迁移导致服务不可用。

## 推荐配置拆分

### 1. 应用基础配置

Data ID：

```text
xzxg-shop-app-config.json
```

用途：

- 服务级开关
- 模型基础配置
- Agent 行为开关

建议内容：

```json
{
  "configs": [
    {"config_key": "ai.base_url", "config_value": "https://dashscope.aliyuncs.com/compatible-mode/v1", "value_type": "string"},
    {"config_key": "ai.small_model", "config_value": "qwen3.5-flash", "value_type": "string"},
    {"config_key": "ai.large_model", "config_value": "qwen3.6-plus", "value_type": "string"},
    {"config_key": "ai.api_key", "config_value": "...", "value_type": "string", "is_secret": true},
    {"config_key": "ai.enabled", "config_value": "true", "value_type": "bool"},
    {"config_key": "ai.enable_thinking", "config_value": "false", "value_type": "bool"},
    {"config_key": "agent.followups_enabled", "config_value": "true", "value_type": "bool"},
    {"config_key": "agent.config_refresh_seconds", "config_value": "15", "value_type": "int"}
  ]
}
```

### 2. RAG 召回配置

Data ID：

```text
xzxg-shop-rag-config.json
```

用途：

- RAG 召回候选池
- RAG 重排权重
- 泛词降权
- 召回链路开关

建议内容：

```json
{
  "configs": [
    {"config_key": "retrieval.keyword.top_n", "config_value": "200", "value_type": "int"},
    {"config_key": "retrieval.vector.top_n", "config_value": "80", "value_type": "int"},
    {"config_key": "retrieval.rerank.weight.title_match", "config_value": "0.30", "value_type": "float"},
    {"config_key": "retrieval.rerank.weight.term_match", "config_value": "0.35", "value_type": "float"},
    {"config_key": "retrieval.rerank.weight.required_match", "config_value": "0.15", "value_type": "float"},
    {"config_key": "retrieval.rerank.weight.brand_boost", "config_value": "0.45", "value_type": "float"},
    {"config_key": "retrieval.rerank.weight.model_boost", "config_value": "0.35", "value_type": "float"},
    {"config_key": "retrieval.rerank.weight.category_boost", "config_value": "0.20", "value_type": "float"},
    {"config_key": "retrieval.rerank.weight.generic_penalty", "config_value": "0.50", "value_type": "float"},
    {"config_key": "retrieval.rerank.generic_terms", "config_value": "推荐,怎么选,同类,区别,性价比,通勤,办公,好喝,不腻,续航,性能,修护", "value_type": "string"}
  ]
}
```

### 3. 向量与中间件配置

Data ID：

```text
xzxg-shop-infra-config.json
```

用途：

- Milvus
- Embedding
- 其他中间件客户端配置

建议内容：

```json
{
  "configs": [
    {"config_key": "vector.enabled", "config_value": "true", "value_type": "bool"},
    {"config_key": "milvus.address", "config_value": "http://127.0.0.1:19530", "value_type": "string"},
    {"config_key": "milvus.token", "config_value": "root:Milvus", "value_type": "string", "is_secret": true},
    {"config_key": "milvus.database", "config_value": "", "value_type": "string"},
    {"config_key": "milvus.collection.products", "config_value": "product_text_vectors", "value_type": "string"},
    {"config_key": "milvus.collection.knowledge", "config_value": "knowledge_text_chunks", "value_type": "string"},
    {"config_key": "embedding.base_url", "config_value": "https://dashscope.aliyuncs.com/compatible-mode/v1", "value_type": "string"},
    {"config_key": "embedding.model", "config_value": "text-embedding-v4", "value_type": "string"},
    {"config_key": "embedding.batch_size", "config_value": "10", "value_type": "int"}
  ]
}
```

### 4. Agent Prompt 配置

Data ID：

```text
xzxg-shop-agent-prompts.json
```

用途：

- 路由 Prompt
- guide 意图 Prompt
- 主 Agent Prompt
- tool protocol Prompt
- followups Prompt
- intent-specific Prompt

建议内容：

```json
{
  "prompts": [
    {"prompt_key": "agent.prompt.route", "title": "一级路由 Prompt", "content": "...", "status": "active", "version": 1},
    {"prompt_key": "agent.prompt.guide_intent", "title": "导购细分 Prompt", "content": "...", "status": "active", "version": 1},
    {"prompt_key": "agent.prompt.answer_base", "title": "主 Agent 基础 Prompt", "content": "...", "status": "active", "version": 1},
    {"prompt_key": "agent.prompt.tool_protocol", "title": "工具协议 Prompt", "content": "...", "status": "active", "version": 1},
    {"prompt_key": "agent.prompt.followups", "title": "追问生成 Prompt", "content": "...", "status": "active", "version": 1}
  ]
}
```

## Prompt 是否落库

建议：Prompt 应该落 MySQL，同时同步发布到 Nacos。

原因：

1. Nacos 适合运行时配置分发，不适合做复杂版本管理。
2. 管理员页面需要查看历史版本、回滚、草稿、发布记录，这些更适合关系型表。
3. MySQL 可以记录谁修改、什么时候修改、发布说明、状态。
4. 服务运行时仍从 Nacos 读取 active prompt，保证低耦合和动态刷新。

## Prompt 表设计

### agent_prompts

```sql
CREATE TABLE agent_prompts (
  prompt_id VARCHAR(64) PRIMARY KEY,
  prompt_key VARCHAR(128) NOT NULL,
  title VARCHAR(128) NOT NULL,
  content MEDIUMTEXT NOT NULL,
  status VARCHAR(32) NOT NULL,
  version INT NOT NULL,
  description TEXT,
  created_by VARCHAR(64) NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_prompt_key_version (prompt_key, version),
  KEY idx_prompt_key_status (prompt_key, status)
);
```

### agent_prompt_publish_records

```sql
CREATE TABLE agent_prompt_publish_records (
  record_id VARCHAR(64) PRIMARY KEY,
  prompt_key VARCHAR(128) NOT NULL,
  version INT NOT NULL,
  operator_id VARCHAR(64) NOT NULL DEFAULT '',
  publish_target VARCHAR(128) NOT NULL,
  publish_status VARCHAR(32) NOT NULL,
  error TEXT,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY idx_prompt_key_created_at (prompt_key, created_at)
);
```

## 后端读取策略

短期：

1. `configcenter.Center` 支持多个 Nacos dataId 聚合读取。
2. `GetMap` 返回所有 config + prompt 的扁平 map，保持现有调用不变。
3. Prompt 从 `xzxg-shop-agent-prompts.json` 读取后转换为：

```text
prompt_key -> content
```

中期：

1. 管理员页面修改 Prompt 时先写 MySQL。
2. 点击发布后同步到 Nacos `xzxg-shop-agent-prompts.json`。
3. Runtime 仍只读 Nacos，不直接依赖 MySQL Prompt 表。

长期：

1. 支持 Prompt 草稿、审核、灰度、回滚。
2. 支持按环境隔离：dev/test/prod。
3. 支持 prompt eval 绑定版本，知道某次测评用的是哪个 prompt version。

## 管理员页面设计

### 配置中心页面

按域展示：

- 应用配置
- RAG 检索配置
- 基础设施配置

每个域单独保存，对应不同 Nacos dataId。

### Prompt 管理页面

功能：

- Prompt 列表
- 查看 active 版本
- 编辑草稿
- 发布到 Nacos
- 回滚到历史版本
- 查看发布记录
- 绑定测评报告

字段：

- prompt_key
- title
- version
- status
- updated_at
- updated_by
- 操作：编辑、发布、回滚、复制为新版本

## 迁移步骤

### 第一步：拆 Nacos 文件

1. 从当前 `xzxg-shop-app-config.json` 拆出：
   - `xzxg-shop-app-config.json`
   - `xzxg-shop-rag-config.json`
   - `xzxg-shop-infra-config.json`
   - `xzxg-shop-agent-prompts.json`
2. 后端先支持多 dataId 聚合读取。
3. 保留旧 dataId 兼容一段时间。

### 第二步：Prompt 落库

1. 新增 `agent_prompts` 和 `agent_prompt_publish_records`。
2. 启动迁移：从 Nacos active prompt 初始化 MySQL active version。
3. 管理员页面新增 Prompt 管理。
4. 发布时写 Nacos。

### 第三步：页面分域

1. 配置页面按 dataId/domain 展示。
2. Prompt 从配置页面移出，进入独立 Prompt 管理页面。
3. 测评报告展示 prompt version。

## 风险与注意事项

1. 不要让运行时直接读未发布草稿。
2. Prompt 发布要校验非空，避免把空 prompt 发布到 Nacos。
3. Secret 配置不能进入前端明文。
4. 多 dataId 聚合时，重复 key 要有优先级，建议：

```text
agent-prompts > rag-config > infra-config > app-config
```

5. 迁移初期保留旧 `xzxg-shop-app-config.json` 作为 fallback，避免 Nacos 拆分不完整导致服务启动失败。

## 已实现 API

### 配置

- `GET /api/v1/admin/configs?page=1&page_size=10`
- `PATCH /api/v1/admin/configs/{config_key}`

说明：该接口只展示非 Prompt 配置。后端会按 `config_key` 自动归属：

- `retrieval.*` -> `xzxg-shop-rag-config.json`
- `vector.*`、`milvus.*`、`embedding.*` -> `xzxg-shop-infra-config.json`
- `agent.prompt.*` -> `xzxg-shop-agent-prompts.json`
- 其他 -> `xzxg-shop-app-config.json`

### Prompt

- `GET /api/v1/admin/prompts?page=1&page_size=10`
- `PATCH /api/v1/admin/prompts/{prompt_key}`
- `POST /api/v1/admin/prompts/{prompt_key}/publish`

保存草稿请求：

```json
{
  "title": "主 Agent 基础 Prompt",
  "description": "主导购 Agent 系统提示词",
  "content": "..."
}
```

发布语义：

1. 取该 `prompt_key` 的最新版本。
2. 将旧 active 版本归档。
3. 将最新版本置为 active。
4. 写入发布记录。
5. 同步到 Nacos `xzxg-shop-agent-prompts.json`。
