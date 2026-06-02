# 参考 Agent 主链路图 Markdown 转换

来源图片：[参考-agent主链路.png](../pics/参考-agent主链路.png)

本文档将参考图中的 “V2 AI 导购 Agent 流程总览” 转换为结构化 Markdown，方便后续对照、裁剪和二次设计。由于原图是截图，部分极小字号内容按可识别语义进行了整理。

## 1. 总体入口流程

```text
开始
  -> 请求入口：参数校验 + Sentinel 限流
  -> 判断用户个人缓存是否命中
     - 是：返回历史结果 offline，结束
     - 否：继续
  -> 创建会话记录：获取 metaId + recordId
  -> 判断全局热门缓存是否命中
     - 是：返回预计算结果 offline，结束
     - 否：继续
  -> 初始化请求缓存 + 加载多轮记忆
  -> Query 改写
  -> 商品相关性判断
  -> 意图分类
  -> 意图降级
  -> sceneType 分发
```

### 1.1 请求入口

入口节点负责基础网关能力：

- 参数校验。
- Sentinel 限流。
- 判断用户个人缓存是否命中。
- 判断全局热门缓存是否命中。
- 创建会话记录，获取 `metaId` 和 `recordId`。

缓存命中时会直接返回 offline 结果，不进入后续 Agent 链路。

### 1.2 多轮记忆与 Query 改写

初始化请求缓存并加载多轮记忆后，进入 Query 改写阶段。

图中包含两个相关 Agent：

#### 记忆检索 Agent

执行方式：异步。

输入：

- `query`
- `memory_context`

输出：

- `relevant_results`
- 与用户历史偏好或价格范围相关的记忆结果

#### Query 改写 Agent

执行方式：同步。

触发条件：

- 存在历史会话。
- 用户意图与历史上下文有关。

输入：

- `query`
- `memory_context`

输出：

- `rewriteQuery`

改写后的 Query 用于后续商品相关性判断、意图分类和检索。

### 1.3 商品相关性判断

节点名称：商品相关性判断。

作用：

- 判断单品场景下，商品标题和 Query 是否相关。
- 为后续精搜、通搜或降级处理提供依据。

### 1.4 意图分类

意图分类阶段包含并行分类能力：

#### 场景分类 Agent

输出：

- `sceneType`

用于判断进入购物场景、非购物场景或 GUI 场景。

#### 行业分类 Agent

输出：

- 行业类目，例如服饰、美妆、数码、酒旅等。

行业分类结果会影响 Prompt、商品筛选策略和推荐表达方式。

### 1.5 意图降级

节点名称：意图降级。

作用：

- 如果当前意图不在灰度范围内，则降级为 `chat_bot`。
- 避免未支持场景进入不可控链路。

## 2. sceneType 分发

图中主分支分为三类：

```text
sceneType 分发
  -> 购物场景：进入购物流程
  -> 非购物场景：进入非购物流程
  -> GUI 场景：进入 GUI 流程
```

## 3. 购物场景链路

购物场景是参考图中最核心、最复杂的分支。

```text
进入购物流程
  -> 并行预处理
  -> 用户意图分析 Agent
  -> 策略 Agent / Skill 配置
  -> 主 Agent 调用
  -> 推荐内容处理
  -> SSE 逐段推送
  -> 浏览更多商品卡片
  -> 追问建议
  -> 等待商品数据加载
  -> 推送结束事件
  -> 保存会话记录
  -> 会话后处理
  -> 清理缓存与释放限流
```

### 3.1 购物流程入口

购物场景共有 9 种。

进入购物流程后，先执行并行预处理。

### 3.2 并行预处理

图中标注并行预处理包含 4 个任务：

1. 用户画像
   - 来源：`iGraph -> base_portrait`

2. 属性改写
   - 改写促成价格、品牌等属性。

3. QP 查询
   - 结合用户行为。
   - 结合类目匹配。

4. 商品上下文
   - 读取 CPV 属性。
   - 读取 AI 评论。

这些预处理结果会输入后续用户意图分析 Agent。

### 3.3 用户意图分析 Agent

节点名称：用户意图分析 Agent。

执行方式：流式。

输入：

- `query`
- 商品信息
- 图片信息

输出：

- `user_display`
- `user_analysis`

其中：

- `user_display` 面向前端展示，可能包含推荐词或用户可见解释。
- `user_analysis` 面向后续主 Agent，作为深层意图分析结果。

### 3.4 策略 Agent 与 Skill 配置

用户意图分析后，图中进入一个策略选择分支。

判断条件：

```text
非默认场景 && 有 Brain 配置？
```

#### 策略 Agent

节点名称：策略 Agent，也标注为 Brain。

输出：

- `angle`
- `angleBuyer`
- `contentParadigm`

这些字段用于控制推荐角度、买手表达和内容范式。

#### 覆盖规则

图中注明 Skill 优先：

- `angleBuyer`
  - 大促值直接覆盖。

- `angle`
  - 仅 Skill 为空时用大脑补充。

- `contentParadigm`
  - 仅 Skill 为空时用大脑补充。

#### Skill 配置

当没有进入 Brain，或需要使用 Skill 时，会读取 Skill 场景配置。

图中标注为 Diamond 配置。

### 3.5 主 Agent 调用

节点名称：主 Agent 调用。

执行方式：

- ReAct。
- 流式。

Prompt：

- `prompt = promptExtMap 全量`

图中主 Agent 内部包含 ReAct 循环：

1. LLM 生成 `planning + answer`。
2. 如果存在 tool 调用：
   - `ItemSearchTool`：商品搜索。
   - `RagInfoTool`：知识库。
3. 如果没有 IO 结果：
   - 生成 summary。

这一段是参考图中最接近核心导购 Agent 的部分。

### 3.6 推荐内容判断

主 Agent 输出后，图中判断：

```text
有推荐？
```

如果有推荐，会进入推荐卡片处理分支。推荐处理分为：

- 精搜场景：`SpecificSearchListener`
- 通搜场景：`CommonSearchListener`

## 4. 精搜场景处理

精搜场景对应 `SpecificSearchListener`。

### 4.1 精搜标签

图中标注要保护的标签包括：

- `refine_title`
  - 标题。

- `item_card`
  - 组题。
  - 连续卡片必须作为完整组一起发送。

### 4.2 item_card 分组逻辑

规则：

- 多张连续卡片视为一组。
- 组未结束时缓存。
- 等组完整后一次性处理。

该逻辑是为了避免前端收到不完整商品卡片组。

### 4.3 精搜 Handler

节点名称：

```text
SpecialSearchItemCardFormatHandlerV2
```

职责：

- 匹配 `item_card` 标签。
- 提取商品字段：
  - `title`
  - `id`
  - `tags`
  - `reason`
- 替换为 `product` 占位符。
- 解析数据并写入 RDS。
- 图中标注 `asyncAllGlobalData`，表示异步写入全局数据。
- 客户端异步拉取商品卡片。

## 5. 通搜场景处理

通搜场景对应 `CommonSearchListener`。

### 5.1 通搜标签

图中标注通搜标签包括：

- `refine_title`
  - 标题。

- `item`
  - 商品。

- `buyer`
  - 买手。

- `inventory`
  - 商品列表。

- `further`
  - 追问。

- 低一级未闭合内容等待符号。

### 5.2 通搜商品 Handler

节点名称：

```text
AgentItemCardFormatHandlerV2
```

职责：

- 匹配 `item` 标签。
- 提取搜索词。
- 调用商品筛选 Agent。
- 异步方式：
  - `CompletableFuture`
- 将结果替换为 `product` 占位符。

### 5.3 买手 Handler

节点名称：

```text
BuyerFormatHandlerV2
```

职责：

- 匹配 `buyer` 标签。
- 调用买手 Agent。
- 将结果替换为 `buyer` 占位符。

## 6. SSE 推送与购物链路收尾

推荐内容经过 FormatHandler 处理后，进入前端推送和会话收尾阶段。

```text
逐段推送处理后内容给前端 SSE
  -> 主 Agent 完成
  -> CountDownLatch 释放
  -> 推送浏览更多商品卡片
  -> 判断是否默认场景或推荐
  -> 生成追问问题
  -> 等待导购商品数据加载完成
  -> 推送结束事件
  -> 保存内容到 RDS 会话记录
  -> 会话压缩 Agent 异步处理历史会话场景
  -> 清理缓存
  -> 释放限流
  -> 结束
```

### 6.1 浏览更多商品卡片

主 Agent 完成后，会推送“浏览更多商品卡片”。

随后判断：

```text
默认场景 / 推荐？
```

如果是默认场景或推荐场景：

- 通过追问 Agent 生成推荐问题。

如果是其他场景：

- 从主 Agent 输出的 `further` 标签中提取。

### 6.2 会话后处理

图中包含一个异步会话压缩 Agent。

节点名称：

```text
会话压缩 Agent
```

执行方式：异步。

作用：

- 压缩历史会话。
- 用于后续多轮记忆。

最后清理缓存并释放限流。

## 7. 非购物场景链路

图中标注：非购物场景 5 种。

```text
进入非购物流程
  -> sendToolNodeEnd 推送节点
  -> SkillHandlerFactory 根据 sceneType 选择 SkillHandler
  -> SkillExecutor.execute
  -> processSkillResponse
  -> 判断是否 hasAskMore
  -> 推送结果给前端
  -> 保存会话记录
  -> 结束
```

### 7.1 非购物场景类型

图中列出 5 种 `sceneType`：

- `pickup_code`
  - 取件码。

- `price_monitor`
  - 价格监控。

- `coupon_discover`
  - 优惠券。

- `review_summary`
  - 评价总结。

- `pdp_qa`
  - 商品问答。

### 7.2 SkillHandlerFactory

节点名称：

```text
SkillHandlerFactory
```

职责：

- 根据 `sceneType` 选择对应的 `SkillHandler`。

### 7.3 SkillExecutor

节点名称：

```text
SkillExecutor.execute()
```

职责：

- 调用对应 SkillStrategy。
- SkillStrategy 可能调用：
  - 外部 API。
  - LLM。

### 7.4 SkillResponse 处理

节点名称：

```text
processSkillResponse()
```

图中标注处理方式：

- `ViewModule -> NTOP` 协议。
- `TEXT`
  - 转纯文本内容。
- `CARD / RICH_TEXT`
  - 生成 TMC 标签。
  - 生成 `InterfaceData`。

### 7.5 AskMore 判断

处理完 SkillResponse 后，判断：

```text
有 askMore？
```

如果有：

- 设置返回问题推荐。

最后：

- 一次性推送结果给前端。
- 图中标注非流式。
- 保存会话记录。

## 8. GUI 场景链路

GUI 场景是第三条分支。

```text
进入 GUI 流程
  -> 读取 Diamond 配置
  -> 根据 sceneType / secondaryLevel 匹配
  -> 生成 GUI 操作指令
  -> 返回结构化 JSON 给前端
  -> 结束
```

### 8.1 GUI 配置

图中配置路径：

```text
mallx.copilot.aiguide.agent.gui.config
```

匹配条件：

- `sceneType`
- `secondaryLevel`

### 8.2 GUI 指令生成

节点名称：

```text
生成 GUI 操作指令
```

输出：

- 结构化 JSON。
- 用于前端执行或渲染 GUI 操作。

## 9. 图中关键组件索引

### 9.1 Agent

| Agent | 所属阶段 | 作用 |
| --- | --- | --- |
| 记忆检索 Agent | Query 改写前后 | 异步检索历史记忆 |
| Query 改写 Agent | Query 改写 | 基于历史会话改写 Query |
| 场景分类 Agent | 意图分类 | 输出 `sceneType` |
| 行业分类 Agent | 意图分类 | 输出行业类目 |
| 用户意图分析 Agent | 购物流程 | 输出 `user_display` 与 `user_analysis` |
| 策略 Agent / Brain | 购物流程 | 输出 `angle`、`angleBuyer`、`contentParadigm` |
| 主 Agent | 购物流程 | ReAct 流式生成导购回答 |
| 商品筛选 Agent | 通搜 Handler | 根据搜索词筛商品 |
| 买手 Agent | 通搜 Handler | 生成买手相关内容 |
| 追问 Agent | 购物收尾 | 生成推荐追问问题 |
| 会话压缩 Agent | 会话后处理 | 异步压缩历史会话 |

### 9.2 Tool

| Tool | 作用 |
| --- | --- |
| `ItemSearchTool` | 商品搜索 |
| `RagInfoTool` | 知识库检索 |

### 9.3 Listener

| Listener | 作用 |
| --- | --- |
| `SpecificSearchListener` | 精搜场景内容监听与处理 |
| `CommonSearchListener` | 通搜场景内容监听与处理 |

### 9.4 FormatHandler

| Handler | 作用 |
| --- | --- |
| `SpecialSearchItemCardFormatHandlerV2` | 精搜 `item_card` 标签处理 |
| `AgentItemCardFormatHandlerV2` | 通搜 `item` 标签处理 |
| `BuyerFormatHandlerV2` | 通搜 `buyer` 标签处理 |

## 10. 参考链路的主要特点

这张参考图体现出以下设计特点：

1. 缓存优先
   - 用户个人缓存和全局热门缓存命中时，直接返回 offline 结果。

2. 多 Agent 串并联
   - Query 改写、意图分类、用户意图分析、策略、主 Agent、追问、会话压缩分别由不同 Agent 承担。

3. 场景分流明显
   - 购物、非购物、GUI 三条路径分开处理。

4. 购物链路最重
   - 包含画像、属性改写、QP 查询、商品上下文、策略、ReAct、商品搜索、RAG、标签解析和 SSE 推送。

5. 前端协议复杂
   - 主 Agent 输出中存在 `item_card`、`item`、`buyer`、`further` 等标签，需要 Listener 和 FormatHandler 转换为前端可消费内容。

6. 异步处理较多
   - 记忆检索、商品筛选、全局数据写入、会话压缩等均存在异步逻辑。

7. 平台化程度高
   - Brain、Diamond、SkillHandlerFactory、SkillExecutor 等说明该链路服务于较成熟的平台系统，而不是单一 MVP 项目。

## 11. 对本项目的参考价值

对当前 “小猪小狗 AI 导购” 项目，最值得借鉴的是：

- 请求入口的限流、缓存和会话记录。
- 多轮记忆与 Query 改写。
- 场景分类和行业分类。
- 购物场景中的商品检索 + RAG + 主 Agent 生成。
- SSE 流式输出。
- 商品卡片标签解析和结构化渲染。
- 会话后处理与记忆压缩。

第一版可暂缓或简化的是：

- Brain / Diamond 的复杂策略配置。
- 多业务 Skill 平台。
- 非购物场景的多个垂直 Skill。
- GUI Agent 流程。
- 多层 Listener / FormatHandler。
- 复杂 ReAct 循环。

这些取舍已在 [Agent框架设计_v1.md](./Agent框架设计_v1.md) 中重新整理为更适合本项目 MVP 的简化 Agent 框架。
