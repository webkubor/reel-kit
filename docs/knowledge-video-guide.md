# 知识短视频生产指南（吸收自 vibe-knowledge）

> 本规范提炼自开源项目 `vibe-knowledge`，无缝整合进 `@kubor/reel-kit` 工作流。
> 核心理念：**不依赖外部高成本视频大模型，纯靠结构化文案 + 权威出处 + 代码级精致排版与转场，零成本快速出片**。

---

## 一、6 种叙事母版

| 母版 | 核心心流 | 最佳适用题材 | 推荐风格 |
|---|---|---|---|
| **A · 猜谜概念片** | 抛出不可思议现象 → 否定直觉猜测 → 揭晓底层科学规律 → 应用启示 | 前沿科学、天文学、数学佯谬、计算机底层 | `darkmath`, `starfield`, `blueprint` |
| **B · 两难选择片** | 历史情境逼入绝境 → 选择 A 带来致命代价 → 选择 B 亦是艰难妥协 → 最终制度诞生 | 历史转折、古代制度、经济博弈、法律起源 | `gufeng`, `paper`, `dunhuang` |
| **C · 痛点原理片** | 日常痛点直击 → 传统解法为什么失效 → 核心原理解剖 → 优雅解决方案 | 工业工程、产品设计、生活冷知识、机械原理 | `blueprint`, `isometric`, `chalk` |
| **D · 一条主线片** | 抓牢单个微小线索（如一枚银币/一粒种子） → 串联起宏大的历史变迁 | 经济史、文明交流、生物演化、材料科技 | `paper`, `gufeng`, `collage` |
| **E · 闯关挑战片** | 层层递进的思维关卡（第1层你以为...第2层其实...终极关...） | 认知心理学、博弈论、算法挑战 | `pixel`, `rpg`, `keynote` |
| **F · 六面轮转片** | 针对一个核心对象，从历史/物理/经济/心理/文化等 6 个维度快速切片 | 人物传记、核心器物、现代城市、经典技术 | `editorial`, `swiss`, `investigation` |

---

## 二、文案句式与镜头节奏

1. **一句一镜（One Line, One Shot）**：
   - 每行字幕必须是一个完整的语义单元（约 12～20 字）。
   - 全片一般 6～10 句，时长控制在 25～45 秒（完播率最佳区间）。
2. **黄金前 3 秒（反常识开场）**：
   - 第一句严禁打招呼（不要“大家好我是...”），直接抛出颠覆认知的核心反差点。
   - 示例：“如果给你一万两黄金，但代价是必须在大秦服徭役，你接不接？”
3. **有据可查（权威出处）**：
   - 知识类视频最怕“地摊文学感”；使用 `--source` 显式标明出处（如《汉书·食货志》或 IEEE 2024）。

---

## 三、reel-kit 一键生成命令示例

```bash
# 1. 东方古风范式（自动匹配 #140d0b 赭黑底色 + 朱红印章 + 金色高亮）
reel make \
  --template knowledge-gufeng \
  --style gufeng \
  --title "大秦商鞅变法" \
  --subtitle "徙木立信的经济学账本" \
  --source "《史记·商君列传》" \
  --assets ./shots \
  --caps ./captions.txt \
  --transition fade \
  --voice demo_narrator \
  --out gufeng-demo.mp4

# 2. 现代科学与数学（暗场概念 + 3B1B 蓝黄高亮 + 蓝图网格）
reel make \
  --template knowledge-dark \
  --style darkmath \
  --title "傅里叶变换" \
  --subtitle "给所有波形做棱镜分解" \
  --source "MIT OpenCourseWare 18.06" \
  --assets ./shots \
  --caps ./captions.txt \
  --transition dissolve \
  --voice demo_narrator \
  --out fourier-demo.mp4
```
