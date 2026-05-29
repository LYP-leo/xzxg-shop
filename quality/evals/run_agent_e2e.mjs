import { authHeaders, loadJSONL, login, requestJSON, streamAgentAnswer, writeJSONReport } from './lib.mjs';

const dataset = process.argv[2] ?? 'quality/data/eval/agent_e2e_queries.jsonl';
const username = process.env.EVAL_USERNAME ?? 'user';
const password = process.env.EVAL_PASSWORD ?? 'user123456';
const reportType = dataset.includes('no_inventory') ? 'agent_no_inventory' : 'agent_e2e';

const token = await login(username, password);
const cases = await loadJSONL(dataset);
const results = [];
let evaluated = 0;
let hits = 0;

for (const item of cases) {
  const session = await requestJSON('/agent/sessions', {
    method: 'POST',
    headers: { ...authHeaders(token), 'Content-Type': 'application/json' },
    body: JSON.stringify({ title: `eval-${item.id}` })
  });
  const output = await streamAgentAnswer(token, session.session_id, item.query, item.attachments ?? []);
  const evaluation = evaluateCase(item, output);
  if (evaluation.evaluated) {
    evaluated += 1;
    if (evaluation.passed) hits += 1;
  }
  results.push({
    id: item.id,
    case_type: item.case_type,
    query: item.query,
    answer: output.answer,
    blocks: output.blocks,
    run_id: output.run_id,
    trace_id: output.trace_id,
    evaluation
  });
}

const report = {
  type: reportType,
  dataset,
  generated_at: new Date().toISOString(),
  total: cases.length,
  evaluated,
  hits,
  pass_rate: evaluated ? hits / evaluated : 0,
  results
};
const file = await writeJSONReport(reportType, report);
console.log(file);

function evaluateCase(item, output) {
  const checks = [];
  const answer = output.answer ?? '';
  const productIDs = productIDsFromBlocks(output.blocks ?? []);

  if (item.expected_no_product_refs) {
    const textProductIDs = Array.from(answer.matchAll(/\bp_[A-Za-z0-9_]+\b/g)).map((match) => match[0]);
    checks.push({
      name: 'no_product_refs',
      passed: productIDs.length === 0 && textProductIDs.length === 0,
      product_ids: productIDs,
      text_product_ids: Array.from(new Set(textProductIDs))
    });
  }
  if (Array.isArray(item.forbidden_terms) && item.forbidden_terms.length > 0) {
    const found = item.forbidden_terms.filter((term) => answer.includes(term));
    checks.push({
      name: 'forbidden_terms',
      passed: found.length === 0,
      found
    });
  }
  if (Array.isArray(item.expected_ack_terms) && item.expected_ack_terms.length > 0) {
    checks.push({
      name: 'expected_ack_terms',
      passed: item.expected_ack_terms.some((term) => answer.includes(term))
    });
  }

  return {
    evaluated: checks.length > 0,
    passed: checks.length > 0 ? checks.every((check) => check.passed) : false,
    checks
  };
}

function productIDsFromBlocks(blocks) {
  const ids = [];
  for (const block of blocks) {
    if (Array.isArray(block.product_ids)) ids.push(...block.product_ids);
    if (block.product?.productId) ids.push(block.product.productId);
  }
  return Array.from(new Set(ids));
}
