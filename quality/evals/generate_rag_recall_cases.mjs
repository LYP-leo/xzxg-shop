import { readdir, readFile, writeFile } from 'node:fs/promises';
import path from 'node:path';

const datasetRoot = process.argv[2] ?? 'quality/data/ecommerce_agent_dataset';
const output = process.argv[3] ?? 'quality/data/eval/rag_recall_cases.jsonl';
const queryTypes = [
  'brand_category',
  'scenario_need',
  'pain_point',
  'attribute_constraint',
  'compare',
  'alias_typo',
  'negative_constraint',
  'brand_category'
];
const casesPerProduct = Number(process.env.RAG_CASES_PER_PRODUCT ?? 3);

const productFiles = [];
for (const dir of await readdir(datasetRoot, { withFileTypes: true })) {
  if (!dir.isDirectory()) continue;
  const dataDir = path.join(datasetRoot, dir.name, 'data');
  for (const file of await readdir(dataDir, { withFileTypes: true })) {
    if (file.isFile() && file.name.endsWith('.json')) {
      productFiles.push(path.join(dataDir, file.name));
    }
  }
}

productFiles.sort();
const products = [];
for (const file of productFiles.slice(0, 100)) {
  products.push(JSON.parse(await readFile(file, 'utf8')));
}
const cases = [];
for (const [productIndex, product] of products.entries()) {
  const id = product.product_id;
  const title = String(product.title ?? '').trim();
  const brand = String(product.brand ?? '').replace(/\s+/g, ' ').trim();
  const category = String(product.category ?? '').trim();
  const subCategory = String(product.sub_category ?? '').trim();
  const variants = queryVariantsForProduct(productIndex);
  for (const queryType of variants.slice(0, casesPerProduct)) {
    const query = buildQuery(product, queryType);
    const expectedProductIDs = expectedRelevantProducts(products, product, queryType).map((item) => item.product_id);
    cases.push({
      id: `rag_${String(cases.length + 1).padStart(3, '0')}`,
      query,
      query_type: queryType,
      top_k: 10,
      category,
      expected_product_ids: expectedProductIDs,
      expected_chunk_ids: expectedProductIDs.map((productID) => `${productID}_ck_marketing`),
      metadata: {
        brand,
        sub_category: subCategory,
        title,
        product_id: id
      }
    });
  }
}

await writeFile(output, cases.map((item) => JSON.stringify(item)).join('\n') + '\n');
console.log(`${output} ${cases.length}`);

function queryVariantsForProduct(index) {
  const secondary = ['scenario_need', 'pain_point', 'attribute_constraint', 'compare', 'alias_typo', 'negative_constraint'];
  return ['brand_category', secondary[index % secondary.length], secondary[(index + 3) % secondary.length]];
}

function buildQuery(product, queryType) {
  const brand = shortBrand(product.brand);
  const subCategory = String(product.sub_category ?? '').trim();
  const titleTerms = titleKeywords(product.title, brand, subCategory);
  const faqTerms = questionKeywords(product.rag_knowledge?.official_faq?.[0]?.question);
  const category = String(product.category ?? '').trim();
  const base = [brand, subCategory].filter(Boolean).join(' ');
  switch (queryType) {
    case 'brand_category':
      return compact([brand, subCategory].filter(Boolean).join(' '));
    case 'scenario_need':
      return compact([scenarioPhrase(category, subCategory), brand].filter(Boolean).join(' '));
    case 'pain_point':
      return compact([painPointPhrase(category, subCategory), titleTerms[0] ?? brand].filter(Boolean).join(' '));
    case 'attribute_constraint':
      return compact([brand, subCategory, titleTerms.slice(0, 2).join(' ')].filter(Boolean).join(' '));
    case 'compare':
      return compact([brand, titleTerms[0] ?? subCategory, '和同类怎么选'].filter(Boolean).join(' '));
    case 'alias_typo':
      return compact(aliasQuery(product, brand, subCategory, titleTerms));
    case 'negative_constraint':
      return compact([negativePhrase(category, subCategory), brand || titleTerms[0]].filter(Boolean).join(' '));
    default:
      return compact([base, faqTerms[0]].filter(Boolean).join(' '));
  }
}

function shortBrand(value) {
  const brand = String(value ?? '').trim();
  if (!brand) return '';
  const parts = brand.split(/\s+/).filter(Boolean);
  return parts.find((item) => /[\u4e00-\u9fa5]/.test(item)) ?? parts[0] ?? '';
}

function titleKeywords(title, brand, subCategory) {
  const stop = new Set([
    brand,
    subCategory,
    '男装',
    '女装',
    '旗舰',
    '商品',
    '全网通',
    '瓶装',
    '即饮',
    '基础',
    '新款'
  ]);
  const raw = String(title ?? '');
  const known = [
    '小棕瓶',
    '小黑瓶',
    '神仙水',
    '红腰子',
    '特护霜',
    '大红瓶',
    '双抗',
    '金灿',
    '特安',
    'iPhone 17 Pro Max',
    'iPhone 17 Pro',
    'iPhone 17',
    'MacBook Air',
    'MacBook Pro',
    'iPad Pro',
    'MateBook',
    'ThinkPad',
    'MIX Fold',
    'AIRism',
    'UltraBOOST',
    '速溶',
    '冷萃',
    '气泡水'
  ].filter((item) => raw.toLowerCase().includes(item.toLowerCase()));
  const splitTerms = raw
    .replace(/[×*]/g, ' ')
    .split(/[\s,，、()（）【】\-/]+/)
    .map((item) => item.trim())
    .filter((item) => item.length >= 2 && item.length <= 14 && !stop.has(item))
    .filter((item, index, arr) => arr.indexOf(item) === index)
    .slice(0, 3);
  return [...known, ...splitTerms].filter((item, index, arr) => arr.indexOf(item) === index).slice(0, 3);
}

function questionKeywords(question) {
  return String(question ?? '')
    .replace(/[？?，,。]/g, ' ')
    .split(/\s+/)
    .map((item) => item.trim())
    .filter((item) => item.length >= 2)
    .slice(0, 2);
}

function scenarioPhrase(category, subCategory) {
  if (category.includes('美妆')) return `${subCategory} 日常护肤推荐`;
  if (category.includes('数码')) return `${subCategory} 通勤办公推荐`;
  if (category.includes('服饰')) return `${subCategory} 日常穿搭推荐`;
  if (category.includes('食品')) return `${subCategory} 办公室囤货推荐`;
  return `${subCategory} 推荐`;
}

function painPointPhrase(category, subCategory) {
  if (category.includes('美妆')) return `敏感肌 ${subCategory} 修护`;
  if (category.includes('数码')) return `${subCategory} 续航 性能`;
  if (category.includes('服饰')) return `${subCategory} 舒适 好打理`;
  if (category.includes('食品')) return `${subCategory} 好喝 不腻`;
  return `${subCategory} 怎么选`;
}

function negativePhrase(category, subCategory) {
  if (category.includes('美妆')) return `不要刺激的 ${subCategory}`;
  if (category.includes('数码')) return `不要太重的 ${subCategory}`;
  if (category.includes('服饰')) return `不要闷热的 ${subCategory}`;
  if (category.includes('食品')) return `不要太甜的 ${subCategory}`;
  return `不要踩雷的 ${subCategory}`;
}

function aliasQuery(product, brand, subCategory, titleTerms) {
  const title = String(product.title ?? '').toLowerCase();
  if (title.includes('iphone')) return '苹果手机';
  if (title.includes('ipad')) return '苹果平板';
  if (title.includes('macbook') || title.includes('thinkpad') || title.includes('联想')) return `${brand || '电脑'} 笔记本`;
  if (subCategory.includes('咖啡')) return `${brand} 速溶咖啡`;
  if (subCategory.includes('面霜')) return `${brand} 修护霜`;
  return [brand, titleTerms[0] ?? subCategory].filter(Boolean).join(' ');
}

function compact(value) {
  return value.replace(/\s+/g, ' ').trim();
}

function expectedRelevantProducts(products, product, queryType) {
  if (['brand_category', 'scenario_need', 'negative_constraint'].includes(queryType)) {
    const brand = shortBrand(product.brand);
    const subCategory = String(product.sub_category ?? '').trim();
    const related = products.filter((item) => shortBrand(item.brand) === brand && String(item.sub_category ?? '').trim() === subCategory);
    return related.length > 0 ? related : [product];
  }
  return [product];
}
