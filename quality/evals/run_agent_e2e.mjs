import { evaluateShoppingCase } from './shopping_assertions.mjs';
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
  const cartBefore = await requestJSON('/cart', { headers: authHeaders(token) });
  let output;
  try { output = await streamAgentAnswer(token, session.session_id, item.query, item.attachments ?? []); }
  catch (error) { output = { ...(error.output ?? {}), transport_error: error.message }; }
  output.cart_before = cartBefore;
  output.actual_cart = await requestJSON('/cart', { headers: authHeaders(token) });
  const evaluation = evaluateShoppingCase(item, output);
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
    evaluation,
    transport_error: output.transport_error,
    cart_before: output.cart_before,
    actual_cart: output.actual_cart
  });
}

const report = {
  type: reportType,
  dataset,
  generated_at: new Date().toISOString(),
  total: cases.length,
  evaluated,
  hits,
  coverage: cases.length ? evaluated / cases.length : 0,
  pass_rate: evaluated ? hits / evaluated : 0,
  results
};
const file = await writeJSONReport(reportType, report);
console.log(file);

const minimumPassRate = Number(process.env.EVAL_MIN_PASS_RATE ?? 1);
const minimumCoverage = Number(process.env.EVAL_MIN_COVERAGE ?? 1);
if (![minimumPassRate, minimumCoverage].every(n => Number.isFinite(n) && n >= 0 && n <= 1)) throw new Error('Invalid evaluation thresholds');
if (!cases.length || report.coverage < minimumCoverage || report.pass_rate < minimumPassRate) process.exitCode = 1;
