import { cartLines, sameSet, evaluateShoppingCase } from './shopping_assertions.mjs';
import { authHeaders, baseURL, loadJSONL, login, requestJSON, streamAgentAnswer, writeJSONReport } from './lib.mjs';

const dataset = process.argv[2] ?? 'quality/data/eval/memory_retrieval_cases.jsonl';
const adminUsername = process.env.EVAL_ADMIN_USERNAME ?? 'admin';
const adminPassword = process.env.EVAL_ADMIN_PASSWORD ?? 'admin123456';
const userPassword = process.env.MEMORY_EVAL_PASSWORD ?? 'EvalUser123456';
const delayMS = Number(process.env.MEMORY_EVAL_DELAY_MS ?? 250);
const minPassRate = Number(process.env.MEMORY_EVAL_MIN_PASS_RATE ?? 0.8);

const adminToken = await login(adminUsername, adminPassword);
const cases = await loadJSONL(dataset);
const results = [];
let evaluated = 0;
let hits = 0;

for (const item of cases) {
  const username = `memory_eval_${Date.now()}_${Math.random().toString(16).slice(2, 8)}`;
  const token = await registerUser(username, userPassword);
  const session = await requestJSON('/agent/sessions', {
    method: 'POST',
    headers: { ...authHeaders(token), 'Content-Type': 'application/json' },
    body: JSON.stringify({ title: `memory-eval-${item.id}` })
  });

  const history = [];
  for (const query of item.history ?? []) {
    const output = await streamAgentAnswer(token, session.session_id, query);
    history.push({ query, answer: output.answer, blocks: output.blocks, run_id: output.run_id, trace_id: output.trace_id });
    await sleep(delayMS);
  }

  const cartBefore = await requestJSON('/cart', { headers: authHeaders(token) });
  let output;
  try { output = await streamAgentAnswer(token, session.session_id, item.query, item.attachments ?? []); }
  catch (error) { output = { ...(error.output ?? {}), transport_error: error.message }; }
  output.cart_before = cartBefore;
  output.actual_cart = await requestJSON('/cart', { headers: authHeaders(token) });
  const trace = output.run_id ? await requestTrace(adminToken, output.run_id) : [];
  const evaluation = evaluateCase(item, output, trace);
  if (evaluation.evaluated) {
    evaluated += 1;
    if (evaluation.passed) hits += 1;
  }
  results.push({
    id: item.id,
    case_type: item.case_type ?? 'memory_retrieval',
    history,
    query: item.query,
    answer: output.answer,
    blocks: output.blocks,
    run_id: output.run_id,
    trace_id: output.trace_id,
    memory_trace: memoryTraceSummary(trace),
    actual_cart: output.actual_cart,
    cart_before: output.cart_before,
    transport_error: output.transport_error,
    evaluation
  });
}

const passRate = evaluated ? hits / evaluated : 0;
const report = {
  type: 'memory_retrieval_eval',
  dataset,
  generated_at: new Date().toISOString(),
  total: cases.length,
  evaluated,
  hits,
  coverage: cases.length ? evaluated / cases.length : 0,
  pass_rate: passRate,
  min_pass_rate: minPassRate,
  results
};
const file = await writeJSONReport('memory_retrieval_eval', report);
console.log(file);
if (!Number.isFinite(minPassRate) || minPassRate < 0 || minPassRate > 1) throw new Error('Invalid memory evaluation threshold');
if (!cases.length || evaluated !== cases.length || passRate < minPassRate) process.exitCode = 1;

async function registerUser(username, password) {
  const response = await fetch(`${baseURL}/auth/register`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password, display_name: username })
  });
  if (!response.ok) {
    throw new Error(`register failed for ${username}: ${response.status} ${await response.text()}`);
  }
  return (await response.json()).token;
}

async function requestTrace(token, runID) {
  const payload = await requestJSON(`/admin/agent/runs/${runID}/trace`, {
    headers: authHeaders(token)
  });
  return payload.items ?? [];
}

function evaluateCase(item, output, trace) {
  const checks = [];
  const expectedMemoryProductIDs = item.expected_memory_product_ids ?? [];
  const forbiddenMemoryProductIDs = item.forbidden_memory_product_ids ?? [];
  const expectedCartProductIDs = item.expected_cart_product_ids ?? [];
  const expectedTerms = item.expected_answer_terms ?? [];
  const memoryProductIDs = productIDsFromMemoryTrace(trace);
  const memoryApplied = trace.some((event) => event.stage === 'memory' && event.event_type === 'applied');
  const cartProductIDs = cartLines(output.actual_cart).map(item => item.product_id);
  const productBlockIDs = productIDsFromProductBlocks(output.blocks ?? []);
  const answer = output.answer ?? '';

  if (typeof item.expected_memory_applied === 'boolean') {
    checks.push({
      name: 'memory_applied',
      passed: memoryApplied === item.expected_memory_applied,
      expected: item.expected_memory_applied,
      actual: memoryApplied
    });
  }
  if (Array.isArray(item.expected_memory_product_ids)) {
    checks.push({
      name: 'memory_referenced_products',
      passed: sameSet(memoryProductIDs, expectedMemoryProductIDs),
      expected: expectedMemoryProductIDs,
      actual: memoryProductIDs
    });
  }
  if (forbiddenMemoryProductIDs.length > 0) {
    checks.push({
      name: 'memory_forbidden_products_absent',
      passed: forbiddenMemoryProductIDs.every((id) => !memoryProductIDs.includes(id)),
      forbidden: forbiddenMemoryProductIDs,
      actual: memoryProductIDs
    });
  }
  if (Array.isArray(item.expected_cart_product_ids)) {
    checks.push({
      name: 'exact_actual_cart_products',
      passed: !!output.actual_cart && sameSet(cartProductIDs, expectedCartProductIDs),
      expected: expectedCartProductIDs,
      actual: cartProductIDs
    });
  }
  if (Array.isArray(item.expected_product_block_ids)) {
    checks.push({
      name: 'product_blocks_contain_products',
      passed: sameSet(productBlockIDs, item.expected_product_block_ids),
      expected: item.expected_product_block_ids,
      actual: productBlockIDs
    });
  }
  if (expectedTerms.length > 0) {
    checks.push({
      name: 'answer_terms',
      passed: expectedTerms.some((term) => answer.includes(term)),
      expected_any: expectedTerms
    });
  }

  const shopping = evaluateShoppingCase(item, output);
  const evaluated = checks.length > 0 || shopping.evaluated;
  checks.push(...shopping.checks);
  return { evaluated, passed: evaluated && checks.every(check => check.passed), checks };
}

function productIDsFromMemoryTrace(trace) {
  const ids = [];
  for (const event of trace) {
    if (event.stage !== 'memory' || event.event_type !== 'applied') continue;
    const metadata = parseMetadata(event.metadata_json);
    if (Array.isArray(metadata.referenced_product_ids)) ids.push(...metadata.referenced_product_ids);
  }
  return Array.from(new Set(ids));
}


function productIDsFromProductBlocks(blocks) {
  const ids = [];
  for (const block of blocks) {
    if (Array.isArray(block.product_ids)) ids.push(...block.product_ids);
    if (block.product?.productId) ids.push(block.product.productId);
  }
  return Array.from(new Set(ids));
}

function memoryTraceSummary(trace) {
  return trace
    .filter((event) => event.stage === 'memory' || event.event_type === 'memory.retrieval')
    .map((event) => ({
      stage: event.stage,
      event_type: event.event_type,
      status: event.status,
      metadata: parseMetadata(event.metadata_json)
    }));
}

function parseMetadata(raw) {
  if (!raw) return {};
  try {
    return JSON.parse(raw);
  } catch {
    return {};
  }
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}
