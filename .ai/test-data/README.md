# Agent 测试数据

本目录用于沉淀 Agent 主链路评测输入。

- `agent_user_requests_v1.jsonl`：100 条用户请求，每行一个 JSON 对象。
- 字段说明：
  - `id`：样例编号。
  - `intent`：预期主意图。
  - `persona`：用户人群/场景。
  - `request`：用户原始请求。
  - `expected_focus`：评测时重点观察的能力。

这些数据只作为输入集，不包含标准答案。后续可以扩展 `expected_tools`、`must_include`、`must_not_include`、`golden_answer` 等字段做自动评测。
