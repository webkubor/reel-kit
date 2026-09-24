# AGENTS.md — reel-kit 仓库级行为准则

## 这是什么

**短视频合成引擎 · AI 无人值守的批量装配器**(用户 2026-08-24 拍板定位)。

素材 + 逐句文案 + 配音/BGM → mp4。CLI 是主形态,Studio 是辅,
同仓同一条流水线(共享 `src/` 核心模块)。

定位边界(明确不做什么):
- **不生成视频**(AI 出视频是上游 Hailuo 这类工具的事,reel-kit 不做)
- **不剪辑已有视频**(没 timeline、没 trim、没调色)
- **不是端到端生产线**(用户得自己给文案 + 素材 + 审片)

## 改代码前必须知道的三条

### 1. concat 列表**不要**重复末帧

`src/compose.mjs` 的 `buildConcatList()` 末尾**不要**再重复最后一行。
基于 ffmpeg 7.x 实测:重复末行反而让 ffmpeg 多输出一整段时长
(5 镜 2.5s 期望 12.5s,重复末行实测 15.0s,差 1 镜)。
不重复末行实测 12.4s,差 < 0.1s(末帧舍入)。

这条跟早期文档描述的"必须重复"相反,改自实测。详见
`docs/video-compositing-notes.md` 第 3 条。

### 2. 音画对齐靠 apad，不要改成算 offset

每句念白后补「镜头时长 − 念白时长」的静音（`apad=whole_dur=<镜头时长>`），
再首尾相接。这样**不需要计算任何偏移量，也就不会有累积误差**。
改成 `adelay` 之类按 offset 摆放的写法，镜头一多必然漂。

### 3. 时长余量不能省

`--voice-margin`（默认 0.45s）看起来像保守值，其实是必需的：
画面做到「刚好等于念白」时，帧率舍入会让画面略短于音频，每句最后半个字被吃掉。
来源见 `docs/video-compositing-notes.md` 第 4 条。

## 加模板

往 `templates/` 丢一个 HTML，占位符 `{{title}}` `{{subtitle}}` `{{footer}}`
`{{caption}}` `{{image}}` `{{bg}}` `{{accent1}}` `{{accent2}}`。**不用改代码。**

画布尺寸写死在 CSS 里（body 的 width/height），与 `--size` 保持一致。
图片会被转成 data URI 注入，所以模板必须自包含，不要引外部资源。

## 配音后端

| 后端 | 何时用 |
|---|---|
| `voxcraft`（默认） | 批量。本地 Qwen3-TTS，一次性 4.2GB 模型，之后免费 |
| `museav` | 临时试验，或本地模型未就绪时兜底。按 token 计费 |

**voxcraft 的 `voice` 命令在它自己的 venv 里，默认不在 PATH。**
`src/voxcraft-locate.mjs` 负责定位并按完整度打分（能跑 > 有模型 > 有音色），
挑最完整的一份。**不要改成只判断 `which voice`** —— 那会误判成没装，
进而重复下载 4.2GB 模型（这个错真的犯过）。

**voxcraft 必须走 web 服务模式**（`src/voxcraft-server.mjs`）。
它的 CLI 每次调用都重载模型，逐句起进程做 8 句就是加载 8 次，
实测第 5 句崩在 `libc++abi: recursive_mutex lock failed`。

## AI 视频生成后端选择顺序(2026-09-24 起)

reel-kit 接入 AI 视频生成时,**按以下顺序自动选**(用户拍板,不要反过来):

| 顺序 | 后端 | 何时用 |
|---|---|---|
| ① 优先 | **`MiniMax Hailuo 2.3`** | Ultra 覆盖,**5 段/天** quota。走 `connector__matrix__submit_video_generation` + `query_video_generation`,走 mcode-tools shared-broker 鉴权(无需在 kyvault 单独存 API key)。768P(6/10s)/1080P(6s),**输出无声 → 仍要叠 voxcraft 配音** |
| ② 降级 | `museav`(用户自建业务中台) | ① quota 用完 / 调用失败时自动降级。按 token 计费,接口路径以 museav 实际为准 |

**不要默认接 H3 / H3-Max** — Token Plan 不覆盖,按账户余额付费;只有用户主动要"原生音轨+愿意花钱"时才考虑,不在自动选择链里。

### Quota 管理(必做)

每日 quota 用本地计数文件,`~/.reel-kit/quotas.json`:

```json
{
  "minimax-hailuo-2.3": { "date": "2026-09-24", "count": 3 }
}
```

- 提交任务前 `+1`,超 5 段直接拒绝 + 报"quota 用完,自动降级 museav"
- 跨日自动 reset
- **失败/被取消的任务不消耗 quota**(Mavis H3 任务会 succeeded / failed / cancelled,Hailuo 同样语义)

## 验证改动

```bash
node --check bin/reel.mjs src/*.mjs        # 语法
pnpm test                                  # check + lint + 模板枚举（CI 跑的就是这条）
scorecard webkubor/reel-kit --min 5        # 仓库质检闸门
```

改了合成逻辑后，**验证数据必须来自被验证的那次运行** ——
拿上一轮的时长去切新生成的视频，会得出完全错误的结论（这个坑踩过）。

## 不做什么

- **不做抠图**：那是 `museav remove-bg` 与 `wechat-sticker-submit` skill 的职责。
- **不做通用剪辑**：调色、多轨、转场请用 DaVinci Resolve。
- **不自动装大件**：缺 voxcraft 或模型时明确报出并给命令，
  但绝不自动下载 4.2GB 的东西。
