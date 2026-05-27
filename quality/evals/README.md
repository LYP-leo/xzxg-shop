# Quality Evals

本目录按 `.ai/prd/质量体系需求_v1.md` 落地三类测评。测评数据统一放在 `quality/data/eval`，脚本输出 JSON 到 `quality/reports`。

## 1. Agent 端到端测评

输入：用户 query。

输出：`query -> answer` 集合，暂时不自动打分，人工检查答案是否合理。

```bash
node quality/evals/run_agent_e2e.mjs quality/data/eval/agent_e2e_queries.jsonl
```

## 2. Agent 意图分类单项测评

输入：用户 query + 预期一级路由 + 预期意图。

后端接口：`POST /api/v1/eval/intent`，该接口调用线上同一套意图分类函数，避免测评和线上链路漂移。

输出：`query -> route -> expected_route -> intent -> expected_intent -> correct`。

意图体系使用参考 prompt 的两阶段分类：
- 一级路由：`guide`、`non_guide`、`fast_product`
- 导购细分：`product_deep`、`compare_decide`、`outfit_styling`、`category_shop_brand`、`category_shop_no_brand`、`category_shop_complex`、`scene_solution`、`open_explore`

```bash
node quality/evals/run_intent_eval.mjs quality/data/eval/intent_cases.jsonl
```

## 3. RAG 召回测评

输入：搜索关键词 + 预期可召回数据。

后端接口：`POST /api/v1/eval/rag`。

输出：`keyword -> recalled -> expected_recall -> hit`。

```bash
node quality/evals/run_rag_recall_eval.mjs quality/data/eval/rag_recall_cases.jsonl
```

## 图片搜索测评

后端接口：`POST /api/v1/eval/image-search`。当前先测链路降级正确性和处理耗时，报告会输出端到端、下载/MinIO 读取、embedding、向量检索、重排等耗时维度。

```bash
node quality/evals/run_image_search_eval.mjs quality/data/eval/image_search_cases.jsonl
```

## 环境变量

```bash
API_BASE_URL=http://127.0.0.1:8080/api/v1
EVAL_USERNAME=user
EVAL_PASSWORD=user123456
EVAL_ADMIN_USERNAME=admin
EVAL_ADMIN_PASSWORD=admin123456
EVAL_REPORT_DIR=quality/reports
```
