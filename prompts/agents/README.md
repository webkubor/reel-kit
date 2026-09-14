# 沸雪 Agent 创作中心 (Agent Center)

> **欢迎来到 Agent 创作中心**。这里是所有 AI Agent 进行剧本创作、分镜设计、视觉生成的统一调度站。

## 📖 SSOT 体系（唯一数据源）

| 数据类型 | 唯一口径文件 | 禁止重复维护 |
|----------|-------------|-------------|
| 世界观 / 天榜 / 势力 | `seasons/s1/bibles/WORLD.md` | ❌ 不另建排行文件 |
| 角色数据 | `seasons/s1/cast/*.json`（18 个 JSON） | ❌ 不另建百科全书 |
| 武器物理 | `seasons/s1/bibles/WEAPONS.md` | ❌ 不在其他文件重写武器属性 |
| 旁白文案标准 | `seasons/s1/bibles/VOICEOVER.md` | — |
| 学习素材 (15s 动作脚本) | `seasons/s1/scripts/storyboards/LEARNING_MATERIALS.md` | ❌ 不另建参考清单 |
| 创作红线 | `agents/CREATIVE_BIBLE.md` | — |

## 📁 导航

**SSOT 核心**（季特定，每季一份；默认 s1）
- [../seasons/s1/bibles/WORLD.md](../seasons/s1/bibles/WORLD.md) — 世界规则与排名
- [../seasons/s1/bibles/WEAPONS.md](../seasons/s1/bibles/WEAPONS.md) — 神兵谱（武器物理唯一口径）
- [../seasons/s1/bibles/VOICEOVER.md](../seasons/s1/bibles/VOICEOVER.md) — Solo 视频旁白文案标准
- [../seasons/s1/cast/](../seasons/s1/cast/) — 人物全量档案（JSON，唯一数据源）

**创作红线与学习素材**（跨季共享）
- [CREATIVE_BIBLE.md](CREATIVE_BIBLE.md) — 创作红线：视觉、动作、镜头、环境
- [../seasons/s1/scripts/storyboards/LEARNING_MATERIALS.md](../seasons/s1/scripts/storyboards/LEARNING_MATERIALS.md) — **学习素材总览**：8 个验证过的 15s 动作脚本（金标 / 天榜正式版 / 顾栖月系列 / 序章），每个标注「学习要点 / 适用场景 / 推荐程度」

**Skills（可复用生产模块，跨季通用）**
- [../skills/视频打斗戏/SKILL.md](../skills/视频打斗戏/SKILL.md) — 15秒打斗脚本生成规则（含大师思维框架 v2.0）
- [../skills/人设模板/SKILL.md](../skills/人设模板/SKILL.md) — 角色 JSON 结构规范
- [../skills/镜头美学/SKILL.md](../skills/镜头美学/SKILL.md) — 镜头偏好收集与 Camera Prompt 生成

## 🛠 创作流

1. **查红线** — 读 [CREATIVE_BIBLE.md](CREATIVE_BIBLE.md) 确认视觉与动作规则
2. **扫学习素材** — 来 [LEARNING_MATERIALS.md](../seasons/s1/scripts/storyboards/LEARNING_MATERIALS.md) 找跟你现在写的角色/动作/场景最像的那一条，先读完再去写 prompt
3. **读人设** — 从 [../seasons/s1/cast/](../seasons/s1/cast/) 取目标角色 JSON
4. **查世界** — 从 [../seasons/s1/bibles/WORLD.md](../seasons/s1/bibles/WORLD.md) 确认地理与势力逻辑
5. **查武器** — 从 [../seasons/s1/bibles/WEAPONS.md](../seasons/s1/bibles/WEAPONS.md) 确认武器物理特性
6. **写脚本** — 参考金标 `top10_rank09_顾栖月.md` 或最终交付格式样板 `雨夜古巷_顾栖月_15s.md` 的 prompt 结构（细节见 LEARNING_MATERIALS.md）
7. **出图出视频** — 进工作台 [../studio/README.md](../studio/README.md) → `/shots` 镜头台建条目 → museav 出图

---
*Created for Father (老爹) by Boiling Snow Team.*
