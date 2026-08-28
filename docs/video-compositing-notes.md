# 视频合成工程笔记

从 [MoneyPrinterTurbo](https://github.com/harry0703/MoneyPrinterTurbo) 的 `app/services/video.py`（1402 行）里提取的工程技巧。

**为什么记这个**：那个项目本身对我没用（靠 pexels 免费素材库拼片，而我有 museav 中台出图，路线更强也更独家），
2026-08-28 已删。但它踩过的合成坑是真的，删项目前先把知识捞出来 —— **留下的是踩坑结论，不是代码**。

以下每条都标注了「reel-kit 现状」，说明哪些已经做了、哪些是待补的。

---

## 1. 编码器三层降级：列表里有 ≠ 真的能用

想用硬件编码（NVENC / AMF / QSV / VideoToolbox）时，只做一层检查是不够的：

```python
# 第一层：配置白名单 —— 不开放任意 ffmpeg 参数
#   理由：开放任意参数后用户填错，任务会在很后面的阶段才失败，
#   而那时已经烧掉了出图、配音的时间
if configured_codec not in _SUPPORTED_VIDEO_CODECS:
    return _DEFAULT_VIDEO_CODEC

# 第二层：ffmpeg -encoders 列表探测
#   ⚠️ 这只证明 ffmpeg **编译时**包含该 encoder，
#      不能证明当前机器的硬件和驱动可用
if codec not in subprocess.run([ffmpeg, "-hide_banner", "-encoders"]).stdout:
    return _DEFAULT_VIDEO_CODEC

# 第三层：运行时失败后，在**本进程内**禁用该编码器
#   理由：一个任务有几十个片段，硬编在第 1 段失败，
#   后面 39 段没必要每段都再失败一次
_runtime_disabled_video_codecs.add(codec)
```

**第三层是关键**，也是最容易漏的。硬件编码器「列表里有但实际跑挂」很常见（驱动版本、
显存占满、并发限制），失败一次就该记住。

> reel-kit 现状：**未做**。目前写死 `libx264`（软编，最稳）。
> 素材量上来、单片渲染超过几分钟时再考虑加硬编 + 这套降级。

## 2. concat demuxer 的路径转义

```python
def _format_ffmpeg_concat_path(file_path):
    absolute_path = os.path.abspath(file_path)
    # Windows 的 C:\Users\... 里反斜杠会被当转义字符，统一转正斜杠
    return absolute_path.replace("\\", "/").replace("'", "'\\''")
```

两个坑：
- **反斜杠**：Windows 绝对路径进 concat list 会被解析成转义序列
- **单引号**：concat list 用单引号包裹路径，路径里的单引号要转成 `'\''`

中文路径本身没问题（UTF-8 写文件即可），但**空格 + 单引号**的组合会静默产生错误的文件列表 ——
ffmpeg 不会报「路径错」，而是报「找不到文件」，排查时容易怀疑到别处。

> reel-kit 现状：**已做单引号转义**（`compose.mjs` 的 `buildConcatList`）。
> 反斜杠转换未做 —— 只跑 macOS，暂不需要。

## 3. concat list 的最后一帧必须重复一次

```
file 'frame_0000.png'
duration 2.5
file 'frame_0001.png'
duration 2.5
file 'frame_0001.png'     ← 重复最后一帧
```

不重复的话，**最后一个 `duration` 会被 ffmpeg 忽略**，末镜时长变成 1 帧。
这是 ffmpeg concat demuxer 的已知行为，不是 bug，但文档里很不显眼。

> reel-kit 现状：**已做**。改 `compose.mjs` 时别删掉那行。

## 4. 视频时长要比音频长一点点

```python
_VIDEO_DURATION_SAFETY_MARGIN = 0.x
return audio_duration + _VIDEO_DURATION_SAFETY_MARGIN
```

做到「刚好等于音频时长」时，**帧率舍入会让成片略短于音频**，结果是旁白最后半个字被切掉。
统一加一点余量，代价是片尾多几帧静止画面 —— 比吞字好得多。

> reel-kit 现状：**不适用**（当前是「画面定时长，BGM 去适配」，方向相反）。
> 但将来若加配音驱动的模式，这条要照抄。

## 5. 素材去重：优先每个源文件只出现一次

线上素材常见「一个长视频被切成多个短片段」。直接对所有片段做 `shuffle`，
同源的几个切片会散落在片头和片中，观众感知就是**素材重复**。

正确做法：

```python
# 按源文件分组 → 每组取最长的那个片段作为「主片段」
# → 主片段之间 shuffle → 剩余片段作为兜底追加在后面
primary = [max(items, key=lambda i: i.duration) for items in grouped.values()]
random.shuffle(primary)
return primary + overflow
```

**取最长片段**而不是随机取一个：避免随机选中视频尾部的零碎短片，
导致明明素材够用却过早开始复用。

> reel-kit 现状：**不适用**（一图一镜，素材由人指定顺序）。
> 但如果将来做「素材池自动选片」，这个策略直接可用。

## 6. 字幕换行必须在创建 TextClip 之前算

```python
# 用 PIL 按当前字体和字号实测宽度，先把文本折好行，再交给渲染
font = ImageFont.truetype(font_path, fontsize)
```

否则渲染库只按原始文本计算区域，中文长句直接溢出画面。

**行高要用 `font.getmetrics()` 的 `ascent + descent`，不能用 `getbbox()` 的高度**：

- `getbbox()` 返回的是**当前字形的可见墨迹高度**
- 只含 `A m n` 这类无下伸部字符的英文会缺 descent
- 多行时误差**逐行累积**，最后一行被画布裁掉

`ascent + descent` 来自字体本身，不受具体字符组合影响。

> reel-kit 现状：**用 HTML/CSS 渲染，天然规避**。浏览器的排版引擎自己处理换行和行高，
> 这正是选浏览器渲染而非 `drawtext` 的原因之一。

## 7. 文字视觉居中 ≠ 几何居中

TextClip 会按字体行高和 baseline 创建透明画布，**很多字体的可见字形不在画布几何中心**。
直接 `position("center")` 会把整块透明画布居中，结果文字看起来偏上或偏下。

解法：读 TextClip 的透明 mask，只按**真实有像素的 bbox** 算偏移。

```python
mask_frame = text_clip.mask.get_frame(0)
ys, _ = np.where(mask_frame > 0.01)
visible_top = int(ys.min())   # 按可见像素而非画布边界对齐
```

> reel-kit 现状：**用 HTML/CSS 渲染，天然规避**。

## 8. moviepy 必须手动释放资源

```python
def close_clip(clip):
    clip.reader.close()                    # 主资源
    clip.audio.reader.close(); del clip.audio   # 音频
    clip.mask.reader.close();  del clip.mask    # 遮罩
    for child in clip.clips: close_clip(child)  # 递归子 clip（注意循环引用）
    clip.clips = []
    del clip; gc.collect()
```

moviepy 的 `VideoFileClip` 会持有 ffmpeg 子进程和文件句柄，不显式关闭就是**句柄泄漏**。
批量处理几十个片段时会撞到系统的 fd 上限，报错却指向别处（比如「无法打开文件」）。

递归关闭子 clip 时要判 `child is not clip`，`CompositeVideoClip` 可能有循环引用。

> reel-kit 现状：**不适用**（不用 moviepy，直接调 ffmpeg CLI，进程退出即回收）。
> 这也是当初选 ffmpeg CLI 而非 moviepy 的原因之一 —— 少一整类资源管理问题。

## 9. 素材分辨率门槛要留容差

```python
min_dimension = _MIN_MATERIAL_DIMENSION - _MIN_DIMENSION_TOLERANCE
```

标称最小 480×480，但实际要允许低几个像素 —— 编码器和消息应用会向下取整，
比如 WhatsApp 传出来的是 `478×850`。卡死在整数门槛会误杀正常素材。

---

## 这些技巧对 reel-kit 的净收益

| # | 技巧 | reel-kit |
|---|---|---|
| 2 | concat 单引号转义 | ✅ 已做 |
| 3 | 最后一帧重复 | ✅ 已做 |
| 1 | 编码器三层降级 | ⬜ 待定（现在软编够用） |
| 4 | 视频比音频长一点 | ⬜ 不适用（当前画面定时长） |
| 5 | 素材去重优先级 | ⬜ 不适用（人工指定顺序） |
| 6 7 | 字幕换行 / 视觉居中 | ✅ 浏览器渲染天然规避 |
| 8 | moviepy 资源释放 | ✅ 不用 moviepy，规避 |
| 9 | 分辨率门槛容差 | ⬜ 未做素材校验 |

**6 / 7 / 8 三条同时指向一个结论**：用「浏览器渲染版式 + ffmpeg CLI 合成」这条路，
直接绕开了 moviepy 生态里字体度量、视觉居中、句柄泄漏这一整类问题。
这不是运气 —— 排版引擎和进程隔离本来就是浏览器和 CLI 各自擅长的事。
