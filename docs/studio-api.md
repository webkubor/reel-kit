# 本地 HTTP 接口

> 桌面版原生界面只是这套接口的一个前端。想把 reel-kit 接进自己的工具、脚本或服务，
> 直接调这里 —— 不必依赖 CLI，也不用把界面扒一遍。

## 服务从哪来

| 场景 | 地址 | 怎么起 |
|---|---|---|
| 桌面版 | `http://127.0.0.1:4390` | 装完自带，开应用即起；端口可用 `REEL_STUDIO_PORT` 改 |
| 命令行 | 同上 | `node bin/reel.mjs studio` |
| 网页版 | 同上 | `node bin/reel.mjs web` |

数据、素材、成片都留在 `~/.reel-kit/studio/`（可用 `REEL_STUDIO_HOME` 指到别处）。

## 两条规矩

1. **只接受本机访问。** `Host` 头不是 `localhost:<port>` 或 `127.0.0.1:<port>` 一律 403；
   带 `Origin` 的请求还要求它和 `Host` 同源。跨站请求会被挡掉。
2. **写操作要带标记头。** 所有 `POST` 必须带 `X-Reel-Action: studio`，否则 403。
   这是为了让浏览器里随手发的一个请求改不动你本机的项目。

```bash
curl -s http://127.0.0.1:4390/api/state | head -c 400
```

## 读

| 路径 | 是什么 |
|---|---|
| `GET /api/state` | 全部项目、素材、任务、声音，以及可用模板 |
| `GET /api/providers` | 各通道当前状态：视频、图片、配音分别有哪些通道、就绪没有、说明 |
| `GET /api/styles` | 可用的画面风格 |
| `GET /api/voices` | 已注册的声音（本地音色、导入的录音） |
| `GET /api/bgm` | 配乐清单 |
| `GET /api/platform-limits` | 各通道的时长/分辨率限制 |
| `GET /api/voice/tasks` | 本机声音克隆任务 |
| `GET /api/providers` 里的 `customChannels` | 自定义通道清单：每条通道的 id、协议、地址、密钥来源，以及配置问题清单 |

## 写

| 路径 | 作用 |
|---|---|
| `POST /api/projects` | 新建项目：`{"name":"我的第一支短片"}` |
| `POST /api/assets?projectId=&name=` | 上传素材（原始字节，配 `Content-Type`） |
| `POST /api/jobs` | 提交生成任务，见下 |
| `POST /api/voice/import` | 注册一段自己的声音（录音或音频文件 + 逐字稿） |
| `POST /api/voice/record` | 录一段自己的声音（仅本机） |
| `POST /api/bgm/pull` | 把外部配乐收进项目 |
| `POST /api/recommendation/draft` | 从 GitHub 仓库读 README 与截图，生成推荐视频草稿 |
| `POST /api/workflow/select-sop` | 给一条视频选定制作方法版本 |
| `POST /api/workflow/quality` | 记录人工审片结果 |
| `POST /api/workflow/remake` | 照着上次的配置再做一条 |

### 自定义通道

用户自己的中转站或厂商直连，也能从接口配（桌面版设置页走的就是这几个接口）。

| 路径 | 作用 |
|---|---|
| `POST /api/channels` | 新增或改一条通道。正文：`{"id","label","kind","protocol","baseUrl","model","env"}`，`kind` 为 `image`/`video`，`env` 是**环境变量名** |
| `POST /api/channels/check` | 检查一条通道。正文 `{"id":"..."}`，返回三态检查项 |
| `DELETE /api/channels/:id` | 删掉一条通道 |

两条规矩跟接口本身有关，说清楚：

- **接口不收密钥。** `POST /api/channels` 的正文里没有密钥字段 —— 密钥要么在环境变量里，
  要么由桌面版设置页写进本机的 `~/.reel-kit/config.json`。密钥不经过 HTTP，也不进通道配置文件。
- **检查是只读的。** `POST /api/channels/check` 只发只读请求，不会生成任何东西，也不产生费用。
  返回的每项是三态：`true` 通过、`false` 失败、`null` **未验证**（比如上游没报「模型不存在」，
  但模型能不能真跑还得真跑一次任务才算数）。把未验证说成通过就是骗人。

## 出一支视频

`POST /api/jobs` 的正文（字段名就是界面上的配置项）：

```json
{
  "kind": "talk",
  "projectId": "项目 ID",
  "title": "标题",
  "captions": "第一句\n第二句\n第三句",
  "ratio": "9:16",
  "voiceBackend": "sherpa",
  "persona": "zh-fanchen-c",
  "pause": 0.3
}
```

- `kind`：`talk`（知识口播）、`recommendation`（项目推荐）、`product`（人物商品介绍）、
  `export`（素材合成）、`speech`（只要配音）、`image`、`video`、`clone`、`voice-design`。
- `voiceBackend`：`sherpa`（随应用自带的本机离线引擎）、`voxflow`（本机克隆音色，仅 macOS）、
  `minimax`（云端，按量计费）。
- `captions` 一行一镜，最多 8 行，每行不超过 80 字。

提交后拿 `job.id`，轮询 `GET /api/state` 看 `jobs[].status`；成功后 `assetId` 就是成片。

```bash
curl -s -X POST http://127.0.0.1:4390/api/jobs \
  -H 'X-Reel-Action: studio' -H 'Content-Type: application/json' \
  -d '{"kind":"talk","projectId":"p1","title":"试一条","captions":"你好，这是本机离线引擎配的旁白。","voiceBackend":"sherpa","persona":"zh-fanchen-c"}'
```

## 为什么把这层公开

界面、CLI、网页版共用这一套接口，是同一个工作台的三种皮：

- 想做自己的批量出片工具 → 调 `/api/jobs`；
- 想在别的产品里嵌一个「生成视频」按钮 → 起一个 reel-kit 工作台，调同一个接口；
- 想给别的模型接一条新通道 → 写 `~/.reel-kit/providers.json`（见 README 的「自定义通道」）。

这样第三方接的是**工作台**，不是某一条命令。
