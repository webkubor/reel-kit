# 参与贡献

## 改之前先读 [AGENTS.md](AGENTS.md)

那里写了**改代码前必须知道的三条**（concat 末帧必须重复、对齐靠 apad 不要改成算
offset、时长余量不能省）。这三条都是踩过坑写下来的，不知道就改很容易把它们改坏。

## 本地跑起来

```bash
pnpm install
node bin/reel.mjs make --template sticker-promo \
  --assets ./examples/assets --caps ./examples/demo-caps.txt \
  --out /tmp/test.mp4
```

配音是可选的，不加 `--voice` 就不需要任何 TTS。

## 提交前

```bash
pnpm test          # 语法 + lint + 模板枚举
```

改了合成逻辑的话，**验证数据必须来自被验证的那次运行** ——
拿上一轮的时长去切新生成的视频会得出完全错误的结论（这个坑踩过）。

## 加模板

往 `templates/` 丢一个 HTML 就行，不用改代码。两条硬要求（CI 会检查）：

- **必须自包含**，不引外部资源。引了外链的模板在离线/CI 环境会静默渲染成空白，
  比直接报错更难查。图片由渲染器转成 data URI 注入。
- 必须含 `{{caption}}` 和 `{{image}}` 占位符。

## 不接受的改动

- 把抠图搬进来 —— 那是 `museav remove-bg` 与 `wechat-sticker-submit` 的职责
- 把 voxcraft 定位改成只判断 `which voice` —— 它的命令在 venv 里，
  那样会误判成没装，进而重复下 4.2GB 模型
- 通用剪辑能力（调色、多轨、转场关键帧）—— 那是 DaVinci Resolve 的活
