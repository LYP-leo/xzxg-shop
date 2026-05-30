import { mkdir, writeFile } from 'node:fs/promises';
import path from 'node:path';

const apiURL = 'https://dummyjson.com/products?limit=0';
const datasetRoot = process.env.ECOMMERCE_DATASET_ROOT ?? path.resolve('quality/data/ecommerce_agent_dataset');
const sourceName = 'DummyJSON public products API';

const categoryMap = {
  beauty: ['5_美妆个护', '美妆个护', '彩妆'],
  fragrances: ['5_美妆个护', '美妆个护', '香水'],
  'skin-care': ['5_美妆个护', '美妆个护', '身体护理'],
  furniture: ['8_家居家装', '家居家装', '家具'],
  'home-decoration': ['8_家居家装', '家居家装', '家居装饰'],
  'kitchen-accessories': ['9_厨房餐具', '厨房餐具', '厨房配件'],
  groceries: ['6_食品生鲜', '食品生鲜', '食品杂货'],
  'mens-shirts': ['12_服饰鞋包', '服饰鞋包', '男装'],
  'mens-shoes': ['12_服饰鞋包', '服饰鞋包', '男鞋'],
  'mens-watches': ['13_钟表配饰', '钟表配饰', '男士腕表'],
  'womens-bags': ['12_服饰鞋包', '服饰鞋包', '箱包'],
  'womens-dresses': ['12_服饰鞋包', '服饰鞋包', '女装'],
  'womens-jewellery': ['13_钟表配饰', '钟表配饰', '首饰'],
  'womens-shoes': ['12_服饰鞋包', '服饰鞋包', '女鞋'],
  'womens-watches': ['13_钟表配饰', '钟表配饰', '女士腕表'],
  sunglasses: ['13_钟表配饰', '钟表配饰', '眼镜'],
  tops: ['12_服饰鞋包', '服饰鞋包', '女装'],
  laptops: ['10_电脑办公', '电脑办公', '笔记本电脑'],
  tablets: ['10_电脑办公', '电脑办公', '平板电脑'],
  smartphones: ['11_手机数码', '手机数码', '智能手机'],
  'mobile-accessories': ['11_手机数码', '手机数码', '手机配件'],
  'sports-accessories': ['14_运动户外', '运动户外', '球类运动'],
  motorcycle: ['15_汽车摩托', '汽车摩托', '摩托车'],
  vehicle: ['15_汽车摩托', '汽车摩托', '汽车']
};

const response = await fetch(apiURL);
if (!response.ok) {
  throw new Error(`fetch products failed: ${response.status} ${await response.text()}`);
}
const payload = await response.json();
const products = payload.products ?? [];
let written = 0;

for (const product of products) {
  const productID = `p_dummyjson_${String(product.id).padStart(3, '0')}`;
  const mapped = taxonomyFor(productID) ?? categoryMap[product.category];
  if (!mapped) continue;
  const [folder, category, subCategory] = mapped;
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

function taxonomyFor(productID) {
  const id = Number(productID.replace('p_dummyjson_', ''));
  const inRange = (start, end) => id >= start && id <= end;
  if (inRange(1, 5)) return item('5_美妆个护', '美妆个护', '彩妆');
  if (inRange(6, 10)) return item('5_美妆个护', '美妆个护', '香水');
  if (inRange(118, 120)) return item('5_美妆个护', '美妆个护', '身体护理');
  if ([18, 22].includes(id)) return item('7_宠物生活', '宠物生活', '宠物食品');
  if ([16, 21, 25, 26, 30, 31, 33, 35, 37, 40].includes(id)) return item('6_食品生鲜', '食品生鲜', '水果蔬菜');
  if ([17, 19, 23, 24, 32].includes(id)) return item('6_食品生鲜', '食品生鲜', '肉禽蛋奶');
  if ([20, 27, 34, 36, 38].includes(id)) return item('6_食品生鲜', '食品生鲜', '粮油冲调');
  if ([28, 29, 39, 42].includes(id)) return item('6_食品生鲜', '食品生鲜', '饮品零食');
  if ([41].includes(id)) return item('8_家居家装', '家居家装', '纸品清洁');
  if (inRange(11, 15)) return item('8_家居家装', '家居家装', '家具');
  if (inRange(43, 47)) return item('8_家居家装', '家居家装', '家居装饰');
  if (inRange(48, 77)) return item('9_厨房餐具', '厨房餐具', '厨房配件');
  if (inRange(78, 82)) return item('10_电脑办公', '电脑办公', '笔记本电脑');
  if (inRange(159, 161)) return item('10_电脑办公', '电脑办公', '平板电脑');
  if (inRange(99, 101) || [103, 107].includes(id)) return item('11_手机数码', '手机数码', '耳机音箱');
  if ([102, 104, 105].includes(id)) return item('11_手机数码', '手机数码', '充电配件');
  if ([106].includes(id)) return item('11_手机数码', '手机数码', '智能穿戴');
  if (inRange(108, 112)) return item('11_手机数码', '手机数码', '拍摄配件');
  if (inRange(121, 136)) return item('11_手机数码', '手机数码', '智能手机');
  if (inRange(83, 87)) return item('12_服饰鞋包', '服饰鞋包', '男装');
  if (inRange(88, 92)) return item('12_服饰鞋包', '服饰鞋包', '男鞋');
  if (inRange(162, 166) || inRange(177, 181)) return item('12_服饰鞋包', '服饰鞋包', '女装');
  if (inRange(185, 189)) return item('12_服饰鞋包', '服饰鞋包', '女鞋');
  if (inRange(172, 176)) return item('12_服饰鞋包', '服饰鞋包', '箱包');
  if (inRange(93, 98)) return item('13_钟表配饰', '钟表配饰', '男士腕表');
  if (inRange(190, 194)) return item('13_钟表配饰', '钟表配饰', '女士腕表');
  if (inRange(182, 184)) return item('13_钟表配饰', '钟表配饰', '首饰');
  if (inRange(154, 158)) return item('13_钟表配饰', '钟表配饰', '眼镜');
  if (inRange(137, 153)) return item('14_运动户外', '运动户外', '球类运动');
  if (inRange(113, 117)) return item('15_汽车摩托', '汽车摩托', '摩托车');
  if (inRange(167, 171)) return item('15_汽车摩托', '汽车摩托', '汽车');
  return null;
}

function item(folder, category, subCategory) {
  return [folder, category, subCategory];
}
