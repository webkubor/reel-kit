# 项目介绍视频工作流（可复用）

一次实测跑通的完整链路 —— 用来给任意 GitHub 项目做 20~30 秒的演示片（横版 16:9 或竖版 9:16）。
全流程本地完成，零外部服务、零 API 花费（配音走本地 TTS）。

## 组成（都是本机现成工具）

| 环节 | 工具 | 位置 |
|---|---|---|
| 逐句文案 | LLM（每行一句，一句一镜） | `captions.txt` |
| 版式模板 | reel-kit（HTML/CSS，5 套） | `~/dev/video/reel-kit` |
| 素材截图 | Playwright（HTML → PNG） | `shoot.mjs` |
| 配音 | voxcraft（本地 Qwen3-TTS） | `~/dev/github/app/voxcraft` |
| 合成 | reel-kit `reel make` | 同上 |
| GIF 导出 | ffmpeg（palettegen/paletteuse） | 系统 |

## 步骤

### 1. 写文案（一句一镜）
```
你的 API key，正在明文流进 AI 的上下文
kyvault set，密钥加密入库，只存本机
工具凭别名取用，模型上下文里只有一个名字
...
```
句数 = 镜头数。6~8 句 ≈ 25 秒。**每句都要是能单独成立的画面**。

### 2. 做素材（HTML → PNG）
复制 `frames/terminal-frame.html` 改内容，用 `shoot.mjs` 截图：
```bash
# shoot.mjs 里改两处：frames 数组（顺序与文案一一对应）、输出路径
bun run shoot.mjs
```
素材图 **1000×1000 方图**、背景与画布同色（白底 `#ffffff`），模板会放在画布左半边。

### 3. 准备配音音色（只需一次）
```bash
cd ~/dev/github/app/voxcraft
.venv/bin/voice design demo_narrator "你的 API key，正在明文流进 AI 的上下文" \
  --tone "沉稳、清晰、专业" --commit
# design 不写 ref_text，手动补一句（就是上面那个朗读文本）：
bun -e "const p='/Users/webkubor/.voxflow/configs/personas.json'; const j=await Bun.file(p).json(); j.demo_narrator.ref_text='你的 API key，正在明文流进 AI 的上下文'; await Bun.write(p, JSON.stringify(j,null,2))"
```

### 4. 合成
```bash
cd ~/dev/video/reel-kit
npx github:webkubor/reel-kit make --template landscape-product \
  --title "kyvault" --subtitle "AI 时代的密钥管家" --footer "github.com/webkubor/kyvault" \
  --assets ~/dev/video/project-demo-kit/shots \
  --caps ~/dev/video/project-demo-kit/captions.txt \
  --size 1920x1080 --voice demo_narrator \
  --out out.mp4
```
- 竖版把 `--size` 改 `1080x1920`、`--template` 改 `sticker-square`
- 模板文字色是给**白底**设计的：深色背景会让文字读不清 —— 别用 `--bg` 改深色
- 传了 `--voice` 就是**念白驱动**（每镜时长=这句的音频时长）；不传则固定 `--per-shot`

### 5. 导出 GIF
```bash
ffmpeg -y -i out.mp4 -vf "fps=12,scale=640:-1:flags=lanczos,split[s0][s1];[s0]palettegen=stats_mode=diff[p];[s1][p]paletteuse" out.gif
```

### 6. 分发
放 `webkubor/free-imgbed` 的 `blog/projects/<项目名>/`，README 里用 jsDelivr CDN 引用：
```
https://cdn.jsdelivr.net/gh/webkubor/free-imgbed@main/blog/projects/<项目>/demo.gif
```
README 里放 GIF（自动播放），MP4 供社媒/嵌入。jsDelivr 有缓存延迟，提交后等一两分钟再验。

## 踩过的坑

1. **模板文字色硬编码深色** —— 深色背景 = 文字看不清。用白底，让素材自己带深色（终端卡那种）。
2. **素材图背景要跟画布同色** —— 否则白底画布上出现一个生硬的深色方块。
3. **`voice design` 不写 `ref_text`** —— reel-kit 会拒绝合成，手动补进 `~/.voxflow/configs/personas.json`。
4. **design 的第二个参数是「被朗读的文本」** —— 想要 ref_text 就把这句话显式传进去。
5. **素材数必须 ≥ 文案句数**，少了会按较少的那边取镜（后面几句白写）。
6. **Playwright 的 chromium 版本要对上** —— 直接用 `~/Library/Caches/ms-playwright/chromium-1243/` 里的可执行文件，别让 playwright 自己找。

## 质检

出片后抽帧目检（`ffmpeg -vf "select='eq(n,60)'"`），重点看：文字对比度、字幕是否跟句子对位、素材顺序对不对。
