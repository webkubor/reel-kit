# Magpie 网关通道

把本机跑着的 **Magpie** 当成一个统一模型网关接进工作台：图片走 Magpie 里已经配好的模型
（GPT Image、Gemini 图像、MAI 等），视频在 Magpie 里配好视频模型后才可用。

这条通道是**内置**的 —— 不需要在「模型通道」里手填 baseUrl 或密钥，也不需要写
`~/.reel-kit/providers.json`。它跟 MuseAV 中台、MiniMax 直连一样，是工作台自带的通道之一，
在「生成素材」的通道下拉里直接选。

## 前置条件

| 条件 | 不满足时（界面上的原话） |
|---|---|
| 本机装了 `magpie` CLI 且在 `PATH` 里 | `Magpie 未安装或不可用，请启动 Magpie 后刷新` |
| Magpie 网关在跑（应用开着，或 `magpie tui` / `magpie web`） | 图片：`未能从 Magpie 取到图片模型清单；请检查网关是否在运行`；视频：`Magpie 视频通道未验证` |
| Magpie 里配好了图片模型（Settings → Models → Images → Image generation） | 图片通道 ready=false，模型列表为空 |
| Magpie 里配好了视频模型（Settings → Models） | `Magpie 当前未配置视频模型（Settings → Models）：视频通道不可用` |

桌面版从 Finder 启动时 `PATH` 很短，`~/.local/bin` 里的 `magpie` 可能找不到；
工作台启动后台时会自己把 `~/.local/bin` 补进 `PATH`，所以装在默认位置就行。

## 三条硬规矩

### 1. 能力发现是只读的

图片模型清单**不是 reel-kit 内置的一张表**，而是向 Magpie 问出来的：

> 给 Magpie 一个**不存在的**模型名，它会把可用清单连同默认模型一起回给你（并拒绝这次调用）。

reel-kit 只在工具文档明确写了这条保证时才发这一次探测请求（探测的是不存在的模型，
不会真的生成、不会计费）。视频没有这条保证，所以**视频通道的可用性只从 Magpie 的工具文档
与设置里读**，绝不拿一次真实生成去试通道通不通。

因此清单永远以网关当时公布的为准 —— Magpie 加了模型，刷新连接就能看到；Magpie 下掉了，
下一次刷新就没了。清单里的模型 id 形如 `provider/model`（如 `codex/gpt-image-2.5`）。

### 2. 清单里出现 ≠ 你有权限用

清单是网关**公布的能力**。某个 provider 有没有额度、密钥通没通、是否已开通，
一律以实际调用返回为准。工作台不会替网关猜这些。

### 3. 提交即计费，下载失败不重复计费

上游一受理（返回任务 ID 或保留一小时的结果地址），reel-kit 就先把 `upstream.taskId` /
`upstream.url` 落到任务上，**再去**下载素材。

所以「素材已经生成、只是下载失败」这种情况，历史记录里会出现**「取回已生成的素材」**：
它走的是同一个任务 ID，**不会重新提交、不会重复扣费**。别看到失败就再点一次生成 ——
那才是真的重复计费。

## 请求模型与实际模型分开记

素材上的两个字段不是一回事：

| 字段 | 记什么 |
|---|---|
| `requestedModel` | 你填的模型（界面上选的那个） |
| `actualModel` | 上游回执里真写了模型名才填，否则记 `unknown` |

没有回执就不猜 —— 这一条在所有通道上都一样。

## 怎么用

界面：左栏「生成素材」→ 通道选「Magpie 网关」→ 选模型（图片清单来自网关；视频先看通道说明
是否就绪）→ 填提示词 → 生成。

命令行 / 脚本：起工作台（`node bin/reel.mjs studio`），然后调 HTTP 接口，
`backend` 传 `"magpie"`：

```bash
curl -s -X POST http://127.0.0.1:4390/api/jobs \
  -H 'X-Reel-Action: studio' -H 'Content-Type: application/json' \
  -d '{"kind":"image","projectId":"p1","backend":"magpie","model":"codex/gpt-image-2.5","prompt":"一只猫"}'
```

接口契约（鉴权头、`/api/jobs/resume` 取回、状态轮询）见
[本地 HTTP 接口](studio-api.md)。

## 和自定义通道的区别

| | Magpie 网关 | 自定义通道 |
|---|---|---|
| 怎么来的 | 内置，装好 Magpie 就有 | 自己填 baseUrl / 模型 / 密钥环境变量名 |
| 协议 | 固定走 `magpie` CLI | `openai` / `minimax-image` / `museav` / `ark-task` |
| 密钥 | 不经 reel-kit（在 Magpie 那侧配） | 只写环境变量名，密钥由你注入 |
| 清单 | 网关自己公布 | 你自己填死一个模型 |

两条通道都能用，怎么选看你自己的额度和信任边界。**不静默换通道、不静默换账户** ——
你选的那条没就绪，工作台会禁用生成并写明原因，不会悄悄改走另一条可能更贵的通道。
