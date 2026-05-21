# Quality Evals

质量测评数据统一放在 `quality/data`，端到端 Agent 与 RAG 回归 case 放在当前目录。

## Agent 端到端测评

启动后端后运行：

```bash
API_BASE_URL=http://127.0.0.1:8080/api/v1 EVAL_USERNAME=user EVAL_PASSWORD=user123456 node quality/evals/run_agent_eval.mjs
```

当前覆盖：
- 问候耗时回归。
- 手机推荐不编造预算、不暴露无关商品。
- 自然语言加购、改数量、删除购物车、确认下单。
- 多商品对比富文本 block。
- 多模态能力未完成时的降级边界。

后续接入 RAG 评测时，优先把真实商品数据入库，再为 `rag_*` case 补 `expected_product_ids` 和引用命中要求。
