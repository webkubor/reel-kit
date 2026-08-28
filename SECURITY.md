# 安全策略

## 报告漏洞

发现安全问题请通过 GitHub Security Advisory 私下报告，
或发邮件到 webkubor@163.com，**不要开公开 issue**。

## 这个工具会碰什么

reel-kit 是本地 CLI，**不上传任何素材、音频或成片**。需要留意的只有三处：

| 面 | 说明 |
|---|---|
| 本机 Chrome | 用无头模式渲染模板 HTML。模板由你自己提供，**不要渲染不受信任的 HTML** |
| ffmpeg | 处理你指定的本地文件 |
| 配音后端 | `voxcraft` 全程本地；`museav` 会把**文案文本**发给 API（音频在本地落盘） |

模板是自包含的（图片转 data URI），CI 里有检查禁止模板引用外部资源。

## 密钥

`museav` 后端通过 `cs kyvault run` 注入 API key，明文不落盘、不进 argv、不进 shell history。
仓库里不含任何凭据。
