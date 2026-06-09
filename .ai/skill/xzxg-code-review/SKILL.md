---
name: xzxg-code-review
description: Review code changes in the xzxg-shop repository. Use when checking Go backend, React admin frontend, Android native client, Agent/RAG/runtime, prompt/config, observability, risk control, evaluation, or API contract changes before merge.
---

# xzxg-shop Code Review

本 skill 用于本仓库代码评审。目标是发现会影响比赛演示、生产稳定性、真实商品召回、Agent 输出可信度、后台管理能力和客户端体验的问题。

## Review Stance

优先输出问题，不做泛泛总结。按严重程度排序，每个问题都必须包含：

- 代码位置：文件和行号。
- 影响：会导致什么错误、坏 case、数据风险或体验问题。
- 触发场景：什么请求、页面操作、并发条件或配置会触发。
- 建议修复：具体到函数、接口、配置项或测试用例。

如果没有发现阻塞问题，明确说明“未发现阻塞问题”，同时列出剩余风险和未覆盖测试。

## Repository Map

- `backend/`：Go 后端，HTTP API、鉴权、商品/订单/购物车、Agent runtime、RAG、Milvus、MinIO、Nacos、风控、测评、日志追踪。
- `frontend/`：React + Vite 管理后台。重点是管理员页面、商家页面、质量测评、链路追踪、Prompt 管理、风控管理。
- `android-native/`：原生 Android 客户端。重点是聊天、流式输出、商品卡片协议、语音/图片、评价、订单/购物车闭环。
- `.ai/prd/`：需求源。重要需求变更必须先对齐这里。
- `.ai/api/`：接口与 Agent 输出协议。接口或协议变更必须同步维护。
- `.ai/tech/`：技术设计、质量体系、RAG/Agent 方案。复杂改动应能在这里找到设计依据。

## First Checks

1. 看变更范围：
   ```bash
   git status --short
   git diff --stat
   git diff --name-only
   ```
2. 找相关需求和设计：
   ```bash
   rg -n "关键词|接口名|prompt|tool|trace|eval|rag|risk|milvus|nacos" .ai backend frontend android-native
   ```
3. 针对变更运行最小验证：
   ```bash
   cd backend && go test ./...
   cd frontend && npm run build
   cd android-native && gradle assembleDebug
   ```
   如果无法运行，说明原因和替代验证。

## Backend Review

重点检查这些生产级问题：

- **鉴权与租户/角色边界**：管理员、商家、用户接口是否互相越权；商家是否只能操作自己的商品/订单；调试接口是否被普通用户访问。
- **数据一致性**：加购、下单、库存、评价、风控状态变更是否使用事务；失败时是否会出现部分写入；重复请求是否幂等。
- **分页与查询保护**：列表接口是否分页；是否限制 `page_size`；是否存在全表扫、无限导出或未过滤查询。
- **上下文与超时**：外部模型、Milvus、MinIO、Nacos、数据库调用是否带 context timeout；失败是否可降级且有日志。
- **配置来源**：Prompt 应从数据库管理；模型、RAG 权重、风控、对象存储、语音等运行配置按当前设计从 Nacos/配置中心读取；不要硬编码 key、模型名、公网地址。
- **敏感信息**：日志、trace、接口返回不能泄露 API key、密码、token、图片私密路径、原始密钥。
- **并发安全**：全局 cache、prompt/config 热加载、embedding 初始化、trace 写入、SSE 推送是否有锁或明确生命周期。
- **错误语义**：对前端返回可处理错误；不要把内部工具状态、`relevance_status`、SQL/Milvus 错误直接暴露给用户回答。

## Agent / RAG Review

这是本项目的核心评审面，优先级高于普通 CRUD。

- **意图分类**：是否仍由模型判断主意图；不要用“看到加购就强制非导购”这类硬编码覆盖模型判断，除非有明确需求和测评。
- **工具调用**：`search_products`、图片检索、购物车、订单、评价、风控等 tool 的入参要表达清楚，尤其是 `negative`、品类、品牌、预算、图片 URL、召回类别。
- **真实商品保护**：
  - 工具内：RDS/Milvus 召回、风控过滤、状态过滤、库存/上下架过滤、候选归一、rerank。
  - 工具外：Prompt 约束、输出协议校验、`<item>` product_id 白名单校验、用户可见文本去内部状态。
  - 不允许推荐工具未返回的商品 ID。
- **RAG 召回**：不要为单个 bad case 写特殊逻辑；优先通过召回数量、query 改写、召回类别、rerank、Nacos 权重、测评集解决。
- **多模态**：图片 embedding 和文本 embedding 初始化不能每次启动重复消耗；失败要可观测；图片找同款不得被历史记忆错误干扰。
- **流式协议**：`<final>`、`<item>`、`<further>` 等协议不能破坏前端解析；不要输出 ```markdown 代码围栏；商品卡片应靠近对应说明。
- **Prompt 管理**：Prompt 不应分散在 Nacos 和数据库；当前要求统一走数据库并能在管理员页面编辑。

## Frontend Admin Review

管理员后台服务于调试、运营和质量治理，评审时看真实工作流是否可用：

- **页面职责拆分**：平台管理、链路追踪、质量测评、Prompt 管理、风控管理、RAG/向量库可视化不要堆在一个页面。
- **响应式**：表格、trace timeline、LLM prompt/raw output、测评详情必须适配屏幕宽度，不能强制横向拖动才能阅读核心内容。
- **可观测性**：请求标题应使用用户 query；每次 tool、LLM call、skill、rerank、RAG 召回都能展开看输入、输出、耗时、错误。
- **质量测评**：报告列表默认少量展示；详情支持分页、筛选、不同报告类型切换；指标不能因为字段名不一致显示 0%。
- **Prompt 管理**：乱码、旧 prompt、废弃 buyer/special_word 标签、Nacos 残留 prompt 都是高优问题。
- **删除废弃用户端**：React Web 用户端已废弃，新增工作不要再依赖用户首页、Web 聊天页、购物车页、订单页的旧组件。

## Android Client Review

客户端是主要用户体验面，评审时重点看协议兼容和弱网表现：

- SSE/流式输出能正确处理思考过程、`<final>`、`<item>`、`<further>`、非导购表单/操作标签。
- 商品卡片插入位置靠近对应文本，不在回答末尾堆叠。
- 图片、语音、TTS、浮窗向导失败时有清晰降级，不阻塞文字对话。
- 登录 token、后端地址、调试开关不能进入 release 暴露风险。
- 本地会话、图片 URI、评价、购物车和订单状态与后端接口一致。

## API / Docs Review

接口或协议变化必须同步检查：

- `.ai/api/API接口文档_v3.md`：新增、删除、字段变更、分页、鉴权、错误码。
- `.ai/api/Agent输出协议_v1.md`：Agent 标签、流式事件、商品卡片、非导购 block。
- `.ai/tech/`：复杂链路如 RAG、Prompt、质量测评、图片检索、TTS/ASR 应有设计或更新记录。

文档缺失本身是 review finding，尤其是前后端需要协作解析的协议。

## Test Expectations

按变更选择测试，不要求每次全量，但要覆盖风险面：

- 后端普通变更：`cd backend && go test ./...`
- 前端管理后台：`cd frontend && npm run build`
- Android 客户端：`cd android-native && gradle assembleDebug`
- Agent/RAG 改动：至少跑相关端到端测评、RAG 测评、意图测评或 bad case 回归。
- Prompt 改动：检查数据库中实际生效 prompt，不能只看本地文件。
- 配置改动：检查 Nacos/DB 当前值、脱敏逻辑和启动日志。

## Common Bad Cases To Regress

- “华为电脑和苹果电脑怎么选？主要写代码和演示”必须能召回两类真实电脑，不得只召回手机或单品牌。
- “推荐电脑，不要苹果”不得推荐苹果商品。
- 商品库没有“马桶”时，不得推荐无关数码商品，也不得把内部缺失原因暴露给用户。
- 图片找同款不能被上一轮文字记忆干扰。
- 加购/下单/查订单应走非导购工具闭环，不返回固定模板话术。
- 风险用户、风险商品、风险商家应在输入和召回阶段被拦截。

## Output Format

使用这个结构输出 review：

```markdown
**Findings**
- [P0] 标题
  文件: `path/to/file.go:123`
  影响: ...
  触发: ...
  建议: ...

**Open Questions**
- ...

**Verification**
- 已运行: ...
- 未运行: ...，原因: ...

**Summary**
简短说明整体风险和改动方向。
```

严重级别：

- `P0`：会导致核心链路不可用、数据泄露、错误下单/越权、演示阻塞。
- `P1`：会导致明显 bad case、测评下降、重要页面不可用、线上稳定性风险。
- `P2`：局部体验、可维护性、文档或测试缺口。

