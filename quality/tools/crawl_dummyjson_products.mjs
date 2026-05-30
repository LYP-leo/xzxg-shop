import { mkdir, writeFile } from 'node:fs/promises';
import path from 'node:path';

const apiURL = 'https://dummyjson.com/products?limit=0';
const datasetRoot = process.env.ECOMMERCE_DATASET_ROOT ?? path.resolve('quality/data/ecommerce_agent_dataset');
const sourceName = 'DummyJSON public products API';

const categoryMap = {
  beauty: ['5_香水美妆', '香水美妆', '彩妆'],
  fragrances: ['5_香水美妆', '香水美妆', '香水'],
  'skin-care': ['5_香水美妆', '香水美妆', '身体护理'],
  furniture: ['6_家居家装', '家居家装', '家具'],
  'home-decoration': ['6_家居家装', '家居家装', '家居装饰'],
  'kitchen-accessories': ['7_厨房用品', '厨房用品', '厨房配件'],
  groceries: ['8_海外食品', '海外食品', '食品杂货'],
  'mens-shirts': ['9_服装鞋包', '服装鞋包', '男士衬衫'],
  'mens-shoes': ['9_服装鞋包', '服装鞋包', '男鞋'],
  'mens-watches': ['10_腕表配饰', '腕表配饰', '男士腕表'],
  'womens-bags': ['10_腕表配饰', '腕表配饰', '女包'],
  'womens-dresses': ['9_服装鞋包', '服装鞋包', '女装连衣裙'],
  'womens-jewellery': ['10_腕表配饰', '腕表配饰', '女士首饰'],
  'womens-shoes': ['9_服装鞋包', '服装鞋包', '女鞋'],
  'womens-watches': ['10_腕表配饰', '腕表配饰', '女士腕表'],
  sunglasses: ['10_腕表配饰', '腕表配饰', '太阳镜'],
  tops: ['9_服装鞋包', '服装鞋包', '上装'],
  laptops: ['11_电脑办公', '电脑办公', '笔记本电脑'],
  tablets: ['11_电脑办公', '电脑办公', '平板电脑'],
  smartphones: ['12_手机通讯', '手机通讯', '智能手机'],
  'mobile-accessories': ['12_手机通讯', '手机通讯', '手机配件'],
  'sports-accessories': ['13_运动户外', '运动户外', '运动配件'],
  motorcycle: ['14_汽车摩托', '汽车摩托', '摩托车'],
  vehicle: ['14_汽车摩托', '汽车摩托', '汽车用品']
};

const response = await fetch(apiURL);
if (!response.ok) {
  throw new Error(`fetch products failed: ${response.status} ${await response.text()}`);
}
const payload = await response.json();
const products = payload.products ?? [];
let written = 0;

for (const product of products) {
  const mapped = categoryMap[product.category];
  if (!mapped) continue;
  const [folder, category, subCategory] = mapped;
  const productID = `p_dummyjson_${String(product.id).padStart(3, '0')}`;
  const imageFile = `${productID}_live.jpg`;
  const imagePath = `${folder}/images/${imageFile}`;
  const dataDir = path.join(datasetRoot, folder, 'data');
  const imageDir = path.join(datasetRoot, folder, 'images');
  await mkdir(dataDir, { recursive: true });
  await mkdir(imageDir, { recursive: true });

  const imageURL = product.thumbnail || product.images?.[0];
  if (imageURL) {
    await downloadImage(imageURL, path.join(imageDir, imageFile));
  }

  const item = {
    product_id: productID,
    title: product.title,
    brand: product.brand || brandFromTitle(product.title),
    category,
    sub_category: subCategory,
    base_price: toCNY(product.price),
    image_path: imagePath,
    skus: buildSKUs(productID, product),
    rag_knowledge: buildKnowledge(product, category, subCategory)
  };
  await writeFile(path.join(dataDir, `${productID}.json`), `${JSON.stringify(item, null, 2)}\n`);
  written += 1;
}

console.log(JSON.stringify({ source: sourceName, products: written, datasetRoot }, null, 2));

async function downloadImage(url, file) {
  const response = await fetch(url);
  if (!response.ok) {
    throw new Error(`download image failed ${url}: ${response.status}`);
  }
  const buffer = Buffer.from(await response.arrayBuffer());
  await writeFile(file, buffer);
}

function toCNY(usd) {
  return Number((Number(usd || 0) * 7.2).toFixed(2));
}

function buildSKUs(productID, product) {
  const basePrice = toCNY(product.price);
  const stock = Number(product.stock ?? 0);
  const rating = Number(product.rating ?? 0);
  const first = {
    sku_id: `s_${productID}_1`,
    properties: { 规格: '标准款', 库存状态: stock > 20 ? '现货' : '少量现货' },
    price: basePrice
  };
  const second = {
    sku_id: `s_${productID}_2`,
    properties: { 规格: rating >= 4.5 ? '高评分优选款' : '组合优惠款' },
    price: Number((basePrice * 1.08).toFixed(2))
  };
  return [first, second];
}

function buildKnowledge(product, category, subCategory) {
  const rating = Number(product.rating ?? 0).toFixed(1);
  const discount = Number(product.discountPercentage ?? 0).toFixed(1);
  const stock = Number(product.stock ?? 0);
  const brand = product.brand || brandFromTitle(product.title);
  const description = String(product.description || '').trim();
  return {
    marketing_description: `${product.title} 是来自 ${sourceName} 的公开商品数据，归入${category}/${subCategory}。商品品牌为 ${brand}，参考售价约 ${toCNY(product.price)} 元，当前公开数据里的评分为 ${rating} 分，库存约 ${stock} 件，折扣信息约 ${discount}%。${description} 适合用于导购 Agent 的跨品类推荐、价格带比较、规格咨询和挂品测试。由于该数据来自公开测试商品 API，实际售卖状态、保修和配送以真实平台页面为准。`,
    official_faq: [
      {
        question: `${product.title} 适合什么购物场景？`,
        answer: `它属于${category}/${subCategory}，适合用户提出跨品类推荐、预算比较、同类商品筛选或希望快速了解品牌、价格和基础卖点时使用。`
      },
      {
        question: `这条商品数据的价格和库存是否可以直接作为下单依据？`,
        answer: `价格、库存和评分来自 ${sourceName} 的公开接口快照，只适合演示和测评。正式交易前应以真实商城的商品详情、库存、售后政策和结算页为准。`
      }
    ],
    user_reviews: [
      {
        nickname: '公开数据用户A',
        rating: Math.max(3, Math.round(Number(product.rating ?? 4))),
        content: `看中 ${product.title} 的品牌和价格，适合先放进候选清单里和同类商品对比。`
      },
      {
        nickname: '公开数据用户B',
        rating: 4,
        content: `商品信息比较完整，但购买前还需要确认规格、售后和真实库存。`
      }
    ]
  };
}

function brandFromTitle(title) {
  return String(title || 'Imported').split(/\s+/)[0] || 'Imported';
}
