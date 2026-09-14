# 埋点集成(可选,reel-kit 不默认启用)

reel-kit 本身是 CLI + 本地工具,**不在 GitHub.com 上产生流量**。但如果将来发布到 GitHub Pages 文档站,推荐用以下方式集成埋点。

## 选型对比

| 平台 | 隐私 | 部署 | 费用 |
|---|---|---|---|
| **Plausible**(推荐) | 无 cookie、GDPR 合规 | 一行 `<script>` | $9/mo 起(自托管免费) |
| **Umami** | 自托管 or 官方云 | 一行 `<script>` | 免费(自托管)/ 起步价(云) |
| **GoatCounter** | 无 cookie | 一行 `<script>` | 免费层(够个人项目) |
| Google Analytics 4 | 有 cookie,需 consent banner | gtag 复杂 | 免费 |

**默认推荐 Plausible**——最简单、隐私友好、文档清晰。

## Plausible 集成步骤

1. 去 [plausible.io](https://plausible.io) 注册账号 + 添加站点
   - 站点域名:`webkubor.github.io` (假设用 GitHub Pages)
2. 拿到一个 domain key(比如 `webkubor.github.io`)
3. 在文档站 `index.html` 的 `<head>` 里嵌入:

```html
<script defer data-domain="webkubor.github.io" src="https://plausible.io/js/script.js"></script>
```

4. 完成。`webkubor.github.io` 整个域名都会上报访问。

## Umami 集成步骤(自托管更隐私)

1. 部署 Umami Cloud 或自托管(参考 [umami.is](https://umami.is/docs))
2. 添加 website,拿到 `website-id`(UUID)
3. 在 `index.html` 加:

```html
<script async defer data-website-id="YOUR-UUID" src="https://umami.your-domain.com/script.js"></script>
```

## 触发自定义事件(可选)

如果想统计"用户跑了 reel make"或"用户跑了哪种模板",在 CLI 端加 telemetry:

```js
// src/analytics.mjs —— 不默认启用,需显式开
import { track } from './analytics.js'

track('reel_make', {
  template: 'sticker-promo',
  duration: 12.4,
  voice: 'demo',
})
```

> **注意:CLI 工具默认不要开 telemetry**。要开就先在 README / 第一次跑时告知用户,
> 给 opt-out(`REEL_NO_TRACKING=1`)。

## 推荐姿势(reel-kit 现在不做)

- 文档站启用 Plausible(只统计页面浏览)
- CLI 不开 telemetry(避免任何隐私争议)
- npm 包下载量靠 [npmjs.com](https://npmjs.com/package/@kubor/reel-kit) 自带统计

## 当前状态

reel-kit 仓库**未集成任何埋点**。要启用,按上面步骤走,纯前端一行 script,无需改仓内代码。
