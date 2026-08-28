# 更新日志

## [0.1.1] - 2026-08-28

### 修复：配乐库不再指向作者的私有 CDN

0.1.0 里 `src/bgm-library.mjs` 把清单地址写死成了
`https://music.webkubor.online/manifest.json` —— 那是作者自己的 R2。
装了 0.1.0 的人跑 `reel bgm` 或用 `--bgm <别名>`，会去拉那份跟自己无关的清单、
从别人的存储下载文件。这条能力本来就该由使用者自己指清单。

现在清单位置由你指定：

```bash
export REEL_BGM_MANIFEST=https://你的域名/music-manifest.json   # 或本地 json 路径
# 或写进 ~/.reel-kit/config.json： { "bgmManifest": "..." }
```

没配置也完全能用 —— `--bgm ./某文件.mp3` 直接给路径，不经过清单。
未配置时用别名会明确告诉你这两种办法，而不是静默失败或去拉某个陌生域名。

**reel-kit 不自带任何配乐**，音乐授权是使用者自己的事。

其他：
- 清单缓存带上来源地址，换了 `REEL_BGM_MANIFEST` 不会还用旧缓存
- 本地清单不缓存，改完立刻生效（调清单时省事）
- 清单条目的 `key` 支持相对 `cdn` 的路径、完整 URL、或本地绝对路径
- `pnpm check` 补上了 `src/bgm-library.mjs`（新文件此前没进语法检查）

## [0.1.0] - 2026-08-28

首个版本。

### 能力

- **模板化合成**：素材 + 逐句文案 + BGM → 1080×1920 竖版 mp4
- **版式用 HTML/CSS**：加模板 = 往 `templates/` 丢一个 HTML，不改代码
- **念白驱动镜头时长**：开配音后，每镜时长 = 该句念白 + 余量，
  而非固定值。实测同组文案固定 2.5s 出 20.0s、念白驱动出 16.7s
- **两个 TTS 后端**：voxcraft（本地 Qwen3-TTS，批量零成本）/ museav（API）
- **音画零累积偏移**：apad 补齐后 concat，不算 offset。实测 8 镜全对齐

### 已知局限

- 无转场（硬切）
- 一句一镜，长句不拆镜
- 只有 `sticker-promo` 一个模板，横版模板未做
