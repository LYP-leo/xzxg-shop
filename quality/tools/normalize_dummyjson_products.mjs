import { mkdir, readdir, readFile, rename, rm, writeFile } from 'node:fs/promises';
import path from 'node:path';

const datasetRoot = process.env.ECOMMERCE_DATASET_ROOT ?? path.resolve('quality/data/ecommerce_agent_dataset');

const titleTranslations = {
  p_dummyjson_001: 'Essence 卷翘浓密睫毛膏 Lash Princess',
  p_dummyjson_002: 'Glamour Beauty 带镜眼影盘',
  p_dummyjson_003: 'Velvet Touch 细腻定妆散粉',
  p_dummyjson_004: 'Chic Cosmetics 经典正红口红',
  p_dummyjson_005: 'Nail Couture 正红指甲油',
  p_dummyjson_006: 'Calvin Klein CK One 中性淡香水',
  p_dummyjson_007: 'Chanel Coco Noir 黑色可可女士香水',
  p_dummyjson_008: "Dior J'adore 真我女士香水",
  p_dummyjson_009: 'Dolce & Gabbana Dolce Shine 花果香调香水',
  p_dummyjson_010: 'Gucci Bloom 花悦女士香水',
  p_dummyjson_011: 'Annibale Colombo 双人床',
  p_dummyjson_012: 'Annibale Colombo 沙发',
  p_dummyjson_013: '非洲樱桃木床头柜',
  p_dummyjson_014: 'Knoll Saarinen 行政会议椅',
  p_dummyjson_015: '木质浴室洗手台带镜柜',
  p_dummyjson_016: '进口苹果',
  p_dummyjson_017: '牛排',
  p_dummyjson_018: '宠物猫粮',
  p_dummyjson_019: '鸡肉',
  p_dummyjson_020: '食用油',
  p_dummyjson_021: '黄瓜',
  p_dummyjson_022: '宠物狗粮',
  p_dummyjson_023: '鸡蛋',
  p_dummyjson_024: '鱼排',
  p_dummyjson_025: '青甜椒',
  p_dummyjson_026: '青辣椒',
  p_dummyjson_027: '蜂蜜罐装',
  p_dummyjson_028: '冰淇淋',
  p_dummyjson_029: '果汁饮料',
  p_dummyjson_030: '猕猴桃',
  p_dummyjson_031: '柠檬',
  p_dummyjson_032: '牛奶',
  p_dummyjson_033: '桑葚',
  p_dummyjson_034: '雀巢咖啡',
  p_dummyjson_035: '土豆',
  p_dummyjson_036: '蛋白粉',
  p_dummyjson_037: '红洋葱',
  p_dummyjson_038: '大米',
  p_dummyjson_039: '软饮料',
  p_dummyjson_040: '草莓',
  p_dummyjson_041: '盒装纸巾',
  p_dummyjson_042: '饮用水',
  p_dummyjson_043: '装饰秋千',
  p_dummyjson_044: '家庭树相框',
  p_dummyjson_045: '家居摆件绿植',
  p_dummyjson_046: '绿植花盆',
  p_dummyjson_047: '台灯',
  p_dummyjson_048: '竹制锅铲',
  p_dummyjson_049: '黑色铝杯',
  p_dummyjson_050: '黑色打蛋器',
  p_dummyjson_051: '盒装料理机',
  p_dummyjson_052: '碳钢炒锅',
  p_dummyjson_053: '厨房砧板',
  p_dummyjson_054: '黄色柑橘榨汁器',
  p_dummyjson_055: '鸡蛋切片器',
  p_dummyjson_056: '电陶炉',
  p_dummyjson_057: '细网滤筛',
  p_dummyjson_058: '餐叉',
  p_dummyjson_059: '玻璃杯',
  p_dummyjson_060: '黑色擦丝器',
  p_dummyjson_061: '手持料理棒',
  p_dummyjson_062: '冰格模具',
  p_dummyjson_063: '厨房筛网',
  p_dummyjson_064: '厨房刀具',
  p_dummyjson_065: '便当盒',
  p_dummyjson_066: '微波炉',
  p_dummyjson_067: '马克杯杯架',
  p_dummyjson_068: '平底锅',
  p_dummyjson_069: '餐盘',
  p_dummyjson_070: '红色食品夹',
  p_dummyjson_071: '玻璃盖银色汤锅',
  p_dummyjson_072: '漏孔锅铲',
  p_dummyjson_073: '调味料收纳架',
  p_dummyjson_074: '餐勺',
  p_dummyjson_075: '托盘',
  p_dummyjson_076: '木质擀面杖',
  p_dummyjson_077: '黄色削皮器',
  p_dummyjson_078: 'Apple MacBook Pro 14 英寸深空灰笔记本',
  p_dummyjson_079: 'Asus Zenbook Pro 双屏笔记本电脑',
  p_dummyjson_080: 'Huawei MateBook X Pro 笔记本电脑',
  p_dummyjson_081: 'Lenovo Yoga 920 翻转笔记本',
  p_dummyjson_082: 'Dell XPS 13 9300 笔记本电脑',
  p_dummyjson_083: '蓝黑格纹男士衬衫',
  p_dummyjson_084: 'Gigabyte Aorus 男士 T 恤',
  p_dummyjson_085: '男士格纹衬衫',
  p_dummyjson_086: '男士短袖衬衫',
  p_dummyjson_087: '男士格子衬衫',
  p_dummyjson_088: 'Nike Air Jordan 1 红黑配色男鞋',
  p_dummyjson_089: 'Nike 棒球钉鞋',
  p_dummyjson_090: 'Puma Future Rider 运动休闲鞋',
  p_dummyjson_091: 'Off White 红白运动鞋',
  p_dummyjson_092: 'Off White 红白运动休闲鞋',
  p_dummyjson_093: '棕色皮带男士腕表',
  p_dummyjson_094: 'Longines 名匠系列男士腕表',
  p_dummyjson_095: 'Rolex Cellini Date 黑盘腕表',
  p_dummyjson_096: 'Rolex Cellini 月相腕表',
  p_dummyjson_097: 'Rolex Datejust 日志型腕表',
  p_dummyjson_098: 'Rolex Submariner 潜航者腕表',
  p_dummyjson_099: 'Amazon Echo Plus 智能音箱',
  p_dummyjson_100: 'Apple AirPods 真无线耳机',
  p_dummyjson_101: 'Apple AirPods Max 银色头戴耳机',
  p_dummyjson_102: 'Apple AirPower 无线充电板',
  p_dummyjson_103: 'Apple HomePod mini 深空灰智能音箱',
  p_dummyjson_104: 'Apple iPhone 充电器',
  p_dummyjson_105: 'Apple MagSafe 外接电池',
  p_dummyjson_106: 'Apple Watch Series 4 金色智能手表',
  p_dummyjson_107: 'Beats Flex 无线耳机',
  p_dummyjson_108: 'iPhone 12 MagSafe 梅子色硅胶保护壳',
  p_dummyjson_109: '手机拍摄独脚架',
  p_dummyjson_110: 'iPhone 自拍补光灯',
  p_dummyjson_111: '自拍杆独脚架',
  p_dummyjson_112: '电视演播室摄像机脚架',
  p_dummyjson_113: '通用款摩托车',
  p_dummyjson_114: 'Kawasaki Z800 摩托车',
  p_dummyjson_115: 'MotoGP CI.H1 摩托车',
  p_dummyjson_116: '踏板摩托车',
  p_dummyjson_117: '运动型摩托车',
  p_dummyjson_118: 'Attitude Super Leaves 洗手液',
  p_dummyjson_119: 'Olay 乳木果滋润沐浴露',
  p_dummyjson_120: 'Vaseline 男士身体和面部乳液',
  p_dummyjson_121: 'Apple iPhone 5s 智能手机',
  p_dummyjson_122: 'Apple iPhone 6 智能手机',
  p_dummyjson_123: 'Apple iPhone 13 Pro 智能手机',
  p_dummyjson_124: 'Apple iPhone X 智能手机',
  p_dummyjson_125: 'Oppo A57 智能手机',
  p_dummyjson_126: 'Oppo F19 Pro Plus 智能手机',
  p_dummyjson_127: 'Oppo K1 智能手机',
  p_dummyjson_128: 'Realme C35 智能手机',
  p_dummyjson_129: 'Realme X 智能手机',
  p_dummyjson_130: 'Realme XT 智能手机',
  p_dummyjson_131: 'Samsung Galaxy S7 智能手机',
  p_dummyjson_132: 'Samsung Galaxy S8 智能手机',
  p_dummyjson_133: 'Samsung Galaxy S10 智能手机',
  p_dummyjson_134: 'Vivo S1 智能手机',
  p_dummyjson_135: 'Vivo V9 智能手机',
  p_dummyjson_136: 'Vivo X21 智能手机',
  p_dummyjson_137: '美式橄榄球',
  p_dummyjson_138: '棒球',
  p_dummyjson_139: '棒球手套',
  p_dummyjson_140: '篮球',
  p_dummyjson_141: '篮球框',
  p_dummyjson_142: '板球',
  p_dummyjson_143: '板球球棒',
  p_dummyjson_144: '板球头盔',
  p_dummyjson_145: '板球三柱门',
  p_dummyjson_146: '羽毛球',
  p_dummyjson_147: '足球',
  p_dummyjson_148: '高尔夫球',
  p_dummyjson_149: '高尔夫铁杆',
  p_dummyjson_150: '金属棒球棒',
  p_dummyjson_151: '网球',
  p_dummyjson_152: '网球拍',
  p_dummyjson_153: '排球',
  p_dummyjson_154: '黑色太阳镜',
  p_dummyjson_155: '经典太阳镜',
  p_dummyjson_156: '绿黑配色眼镜',
  p_dummyjson_157: '派对眼镜',
  p_dummyjson_158: '时尚太阳镜',
  p_dummyjson_159: 'iPad mini 2021 星光色平板电脑',
  p_dummyjson_160: 'Samsung Galaxy Tab S8 Plus 灰色平板',
  p_dummyjson_161: 'Samsung Galaxy Tab 白色平板',
  p_dummyjson_162: '蓝色连衣裙',
  p_dummyjson_163: '夏季女童连衣裙',
  p_dummyjson_164: '灰色连衣裙',
  p_dummyjson_165: '短款连衣裙',
  p_dummyjson_166: '格纹连衣裙',
  p_dummyjson_167: 'Chrysler 300 Touring 汽车',
  p_dummyjson_168: 'Dodge Charger SXT 后驱汽车',
  p_dummyjson_169: 'Dodge Hornet GT Plus 汽车',
  p_dummyjson_170: 'Dodge Durango SXT 后驱汽车',
  p_dummyjson_171: 'Chrysler Pacifica Touring 汽车',
  p_dummyjson_172: '蓝色女士手提包',
  p_dummyjson_173: 'Heshe 女士皮革包',
  p_dummyjson_174: 'Prada 女士手袋',
  p_dummyjson_175: '白色仿皮双肩包',
  p_dummyjson_176: '黑色女士手提包',
  p_dummyjson_177: '黑色女士礼服裙',
  p_dummyjson_178: '皮革束身上衣半裙套装',
  p_dummyjson_179: '黑色半裙束身套装',
  p_dummyjson_180: '豌豆绿连衣裙',
  p_dummyjson_181: 'Marni 红黑套装',
  p_dummyjson_182: '绿色水晶耳环',
  p_dummyjson_183: '绿色椭圆耳环',
  p_dummyjson_184: '热带风耳环',
  p_dummyjson_185: '黑棕拼色女士拖鞋',
  p_dummyjson_186: 'Calvin Klein 女士高跟鞋',
  p_dummyjson_187: '金色女士鞋',
  p_dummyjson_188: 'Pampi 女鞋',
  p_dummyjson_189: '红色女鞋',
  p_dummyjson_190: 'IWC Ingenieur 精钢自动腕表',
  p_dummyjson_191: 'Rolex Cellini 月相女士腕表',
  p_dummyjson_192: 'Rolex Datejust 女士日志型腕表',
  p_dummyjson_193: '金色女士腕表',
  p_dummyjson_194: '女士手表'
};

const brandTranslations = {
  Fashion: '时尚品牌',
  Green: '绿色系列',
  Black: '黑色系列',
  Red: '红色系列',
  Blue: '蓝色系列',
  Gray: '灰色系列',
  Short: '短款系列',
  Tartan: '格纹系列',
  Dress: '连衣裙品牌',
  Girl: '夏季女装',
  Tropical: '热带风格',
  Classic: '经典服饰',
  Casual: '休闲服饰',
  Urban: '都市风格',
  Comfort: '舒适系列',
  Elegance: '优雅系列',
  Imported: '公开商品'
};

const files = await findDummyJSONFiles(datasetRoot);

let changed = 0;
const oldTopDirs = new Set();
const targetTopDirs = new Set();
for (const file of files) {
  const raw = await readFile(file, 'utf8');
  const product = JSON.parse(raw);
  const taxonomy = taxonomyFor(product.product_id);
  targetTopDirs.add(taxonomy.folder);
  const originalCategory = product.category;
  const originalSubCategory = product.sub_category;
  const translatedTitle = titleTranslations[product.product_id] ?? product.title;
  const translatedBrand = brandTranslations[product.brand] ?? product.brand;
  const imageFile = path.basename(product.image_path);
  const oldImage = path.join(datasetRoot, product.image_path);
  const newImagePath = path.join(taxonomy.folder, 'images', imageFile);
  const newImage = path.join(datasetRoot, newImagePath);
  const targetDataDir = path.join(datasetRoot, taxonomy.folder, 'data');
  const targetImageDir = path.join(datasetRoot, taxonomy.folder, 'images');
  await mkdir(targetDataDir, { recursive: true });
  await mkdir(targetImageDir, { recursive: true });

  product.title = translatedTitle;
  product.brand = translatedBrand;
  product.category = taxonomy.category;
  product.sub_category = taxonomy.subCategory;
  product.image_path = newImagePath;
  product.rag_knowledge = localizeKnowledge(product, originalCategory, originalSubCategory, taxonomy);

  const newFile = path.join(targetDataDir, `${product.product_id}.json`);
  await writeFile(newFile, `${JSON.stringify(product, null, 2)}\n`);
  if (oldImage !== newImage) {
    await rename(oldImage, newImage).catch(async (error) => {
      if (error.code !== 'ENOENT') throw error;
    });
  }
  if (file !== newFile) {
    await rm(file, { force: true });
  }
  const relative = path.relative(datasetRoot, file);
  oldTopDirs.add(relative.split(path.sep)[0]);
  changed += 1;
}

for (const dir of oldTopDirs) {
  if (!targetTopDirs.has(dir) && (dir.startsWith('5_公开商品扩展') || /^[0-9]+_/.test(dir))) {
    await rm(path.join(datasetRoot, dir), { recursive: true, force: true });
  }
}

console.log(JSON.stringify({ normalized: changed, categories: categorySummary(files.length) }, null, 2));

async function findDummyJSONFiles(root) {
  const out = [];
  async function walk(dir) {
    for (const entry of await readdir(dir, { withFileTypes: true })) {
      const full = path.join(dir, entry.name);
      if (entry.isDirectory()) {
        await walk(full);
      } else if (entry.name.startsWith('p_dummyjson_') && entry.name.endsWith('.json')) {
        out.push(full);
      }
    }
  }
  await walk(root);
  return out.sort();
}

function localizeKnowledge(product, originalCategory, originalSubCategory, taxonomy) {
  const price = product.base_price;
  const title = product.title;
  const brand = product.brand;
  const sourceNote = `该商品来自 DummyJSON 公开商品数据，已按标准类目体系归入“${taxonomy.category}/${taxonomy.subCategory}”。`;
  return {
    marketing_description: `${title} 是一款公开真实商品数据，品牌为 ${brand}，参考售价约 ${price} 元。${sourceNote} 适合用于跨品类导购演示、商品卡片展示、价格带比较和工具链测评。购买前仍需以真实商城的商品详情、库存、售后和结算页为准。`,
    official_faq: [
      {
        question: `${title} 属于什么数据来源？`,
        answer: `${sourceNote} 商品名称、价格和图片已整理为中文展示文本，便于中文导购场景使用。`
      },
      {
        question: `${title} 的价格和库存能直接下单吗？`,
        answer: `这里的价格和库存来自公开测试数据，只用于演示和测评。真实购买前应以正式商城页面为准。`
      }
    ],
    user_reviews: [
      {
        nickname: '公开数据用户A',
        rating: 4,
        content: `${title} 信息已做中文整理，适合作为导购演示里的真实商品候选。`
      },
      {
        nickname: '公开数据用户B',
        rating: 4,
        content: `这类公开商品适合补充品类覆盖，实际购买前还需要确认真实规格、库存和售后。`
      }
    ]
  };
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
  if (id === 41) return item('8_家居家装', '家居家装', '纸品清洁');
  if (inRange(11, 15)) return item('8_家居家装', '家居家装', '家具');
  if (inRange(43, 47)) return item('8_家居家装', '家居家装', '家居装饰');
  if (inRange(48, 77)) return item('9_厨房餐具', '厨房餐具', '厨房配件');
  if (inRange(78, 82)) return item('10_电脑办公', '电脑办公', '笔记本电脑');
  if (inRange(159, 161)) return item('10_电脑办公', '电脑办公', '平板电脑');
  if (inRange(121, 136)) return item('11_手机数码', '手机数码', '智能手机');
  if ([100, 101, 103, 107].includes(id)) return item('11_手机数码', '手机数码', '耳机音箱');
  if ([102, 104, 105].includes(id)) return item('11_手机数码', '手机数码', '充电配件');
  if (id === 106) return item('11_手机数码', '手机数码', '智能穿戴');
  if ([99, 108, 109, 110, 111, 112].includes(id)) return item('11_手机数码', '手机数码', '拍摄配件');
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
  return item('99_其他商品', '其他商品', '其他');
}

function item(folder, category, subCategory) {
  return { folder, category, subCategory };
}

function categorySummary(total) {
  return { total, taxonomy: 'standard' };
}
