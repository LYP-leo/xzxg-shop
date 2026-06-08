import { authHeaders, loadJSONL, login, requestJSON, writeJSONReport } from './lib.mjs';

const dataset = process.argv[2] ?? 'quality/data/eval/rag_recall_cases.jsonl';
const username = process.env.EVAL_ADMIN_USERNAME ?? 'admin';
const password = process.env.EVAL_ADMIN_PASSWORD ?? 'admin123456';
const topK = Number(process.env.PRODUCT_SEARCH_EVAL_TOP_K ?? 10);
const delayMS = Number(process.env.PRODUCT_SEARCH_EVAL_DELAY_MS ?? 150);
const variants = (process.env.PRODUCT_SEARCH_EVAL_VARIANTS ?? 'rule,model')
  .split(',')
  .map((item) => item.trim())
  .filter(Boolean);

const token = await login(username, password);
const cases = (await loadJSONL(dataset)).filter((item) => Array.isArray(item.expected_product_ids) && item.expected_product_ids.length);
const variantReports = [];

for (const variant of variants) {
  validateVariant(variant);
  const startedAt = Date.now();
    const results = [];
    for (const item of cases) {
      const query = item.query ?? item.keyword;
      const response = await requestJSONWithRetry('/eval/products', {
        method: 'POST',
        headers: { ...authHeaders(token), 'Content-Type': 'application/json' },
        body: JSON.stringify({
          query,
          limit: item.top_k ?? topK,
          constraints: constraintsFromCase(item),
          rerank_model_enabled: variant === 'model',
          llm_filter_enabled: variant === 'small_filter'
        })
      });
      const productIDs = response.product_ids ?? [];
      const rank = firstRank(productIDs, item.expected_product_ids);
      results.push({
        id: item.id,
        query,
        query_type: item.query_type ?? 'unknown',
        expected_product_ids: item.expected_product_ids,
        product_ids: productIDs,
        candidate_product_ids: response.candidate_product_ids ?? [],
        dropped_product_ids: response.dropped_product_ids ?? [],
        relevance_status: response.relevance_status,
        relevance_reason: response.relevance_reason,
        rerank: response.rerank ?? null,
        duration_ms: response.duration_ms ?? 0,
        rank,
        reciprocal_rank: rank > 0 ? 1 / rank : 0,
        hit: rank > 0
      });
      if (delayMS > 0) await sleep(delayMS);
    }
    variantReports.push({
      variant,
      config: await readConfig(token, [
        'retrieval.product.rerank.model_enabled',
        'retrieval.product.rerank.model',
        'retrieval.product.llm_filter.enabled'
      ]),
      duration_ms: Date.now() - startedAt,
      ...summary(results),
      rerank: rerankStats(results),
      by_query_type: groupStats(results),
      results
    });
  }

const report = {
  type: 'product_search_eval',
  dataset,
  generated_at: new Date().toISOString(),
  total_cases: cases.length,
  variants: variantReports,
  comparison: compareVariants(variantReports)
};

const file = await writeJSONReport('product_search_eval', report);
console.log(file);

function validateVariant(variant) {
  if (['rule', 'model', 'small_filter'].includes(variant)) return;
  throw new Error(`unknown product search eval variant: ${variant}`);
}

function constraintsFromCase(item) {
  const metadata = item.metadata ?? {};
  const constraints = { brands: [], terms: [], categories: [] };
  if (metadata.brand) constraints.brands.push(metadata.brand);
  if (metadata.sub_category) constraints.categories.push(metadata.sub_category);
  if (item.category) constraints.categories.push(item.category);
  return constraints;
}

function firstRank(actual, expected) {
  if (!Array.isArray(expected) || expected.length === 0) return 0;
  for (let index = 0; index < actual.length; index += 1) {
    if (expected.includes(actual[index])) return index + 1;
  }
  return 0;
}

function summary(results) {
  const hits = results.filter((item) => item.hit).length;
  const reciprocalRankSum = results.reduce((total, item) => total + item.reciprocal_rank, 0);
  const durations = results.map((item) => item.duration_ms).sort((a, b) => a - b);
  return {
    total: results.length,
    hits,
    hit_rate_at_k: results.length ? hits / results.length : null,
    mrr: results.length ? reciprocalRankSum / results.length : null,
    avg_duration_ms: results.length ? durations.reduce((total, item) => total + item, 0) / results.length : null,
    p95_duration_ms: percentile(durations, 0.95)
  };
}

function groupStats(items) {
  const groups = new Map();
  for (const item of items) {
    const key = item.query_type ?? 'unknown';
    const group = groups.get(key) ?? { items: [] };
    group.items.push(item);
    groups.set(key, group);
  }
  return Object.fromEntries(Array.from(groups.entries()).map(([key, group]) => [key, summary(group.items)]));
}

function rerankStats(results) {
  const stats = {};
  for (const item of results) {
    const rerank = item.rerank ?? {};
    const key = [
      rerank.provider || 'none',
      rerank.model || 'none',
      rerank.fallback ? 'fallback' : 'direct'
    ].join(':');
    stats[key] = (stats[key] ?? 0) + 1;
  }
  return stats;
}

function compareVariants(reports) {
  if (reports.length < 2) return {};
  const base = reports[0];
  return Object.fromEntries(
    reports.slice(1).map((item) => [
      `${base.variant}_vs_${item.variant}`,
      {
        hit_rate_delta: nullableDelta(item.hit_rate_at_k, base.hit_rate_at_k),
        mrr_delta: nullableDelta(item.mrr, base.mrr),
        avg_duration_ms_delta: nullableDelta(item.avg_duration_ms, base.avg_duration_ms),
        p95_duration_ms_delta: nullableDelta(item.p95_duration_ms, base.p95_duration_ms)
      }
    ])
  );
}

function percentile(values, p) {
  if (!values.length) return null;
  const index = Math.min(values.length - 1, Math.ceil(values.length * p) - 1);
  return values[index];
}

function nullableDelta(next, prev) {
  if (next === null || next === undefined || prev === null || prev === undefined) return null;
  return next - prev;
}

async function applyVariant(token, variant) {
  if (variant === 'rule') {
    await patchConfig(token, 'retrieval.product.rerank.model_enabled', 'false', 'bool');
    await patchConfig(token, 'retrieval.product.llm_filter.enabled', 'false', 'bool');
    return;
  }
  if (variant === 'model') {
    await patchConfig(token, 'retrieval.product.rerank.model_enabled', 'true', 'bool');
    await patchConfig(token, 'retrieval.product.rerank.model', 'qwen3-vl-rerank', 'string');
    await patchConfig(token, 'retrieval.product.llm_filter.enabled', 'false', 'bool');
    return;
  }
  throw new Error(`unknown product search eval variant: ${variant}`);
}

async function readConfig(token, keys) {
  const response = await requestJSONWithRetry('/admin/configs?page=1&page_size=500', {
    headers: authHeaders(token)
  });
  const byKey = new Map((response.items ?? []).map((item) => [item.config_key, item]));
  return Object.fromEntries(keys.map((key) => [key, byKey.get(key)?.config_value ?? '']));
}

async function restoreConfig(token, config) {
  for (const [key, value] of Object.entries(config)) {
    if (value === undefined) continue;
    await patchConfig(token, key, value, value === 'true' || value === 'false' ? 'bool' : 'string');
  }
}

async function patchConfig(token, key, value, valueType) {
  await requestJSONWithRetry(`/admin/configs/${encodeURIComponent(key)}`, {
    method: 'PATCH',
    headers: { ...authHeaders(token), 'Content-Type': 'application/json' },
    body: JSON.stringify({ value, value_type: valueType })
  });
}

async function requestJSONWithRetry(pathname, options = {}, attempts = 8) {
  let lastError;
  for (let attempt = 0; attempt < attempts; attempt += 1) {
    try {
      return await requestJSON(pathname, options);
    } catch (error) {
      lastError = error;
      const message = error instanceof Error ? error.message : String(error);
      if (!message.includes('429')) throw error;
      await sleep(Math.min(8000, 500 * (attempt + 1)));
    }
  }
  throw lastError;
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}
