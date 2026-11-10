<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://cdn.jsdelivr.net/gh/webkubor/free-imgbed@main/reel-kit/mark-v3-dark.svg" />
    <img src="https://cdn.jsdelivr.net/gh/webkubor/free-imgbed@main/reel-kit/mark-v3.svg" width="96" alt="reel-kit" />
  </picture>
</p>

<p align="center">
  <img src="https://img.shields.io/github/license/webkubor/reel-kit?style=flat-square&color=92a8b3" alt="License" />
  <img src="https://img.shields.io/github/stars/webkubor/reel-kit?style=flat-square&color=cc584d" alt="Stars" />
  <img src="https://img.shields.io/badge/macOS-Apple%20Silicon-cc584d?style=flat-square" alt="macOS" />
  <img src="https://img.shields.io/badge/Windows-x64-cc584d?style=flat-square" alt="Windows" />
  <img src="https://img.shields.io/badge/voice-free%20%C2%B7%20offline-cc584d?style=flat-square" alt="free offline voice" />
</p>

<p align="center">
  <b>Reel Studio · turn a script and some assets into a short video you can publish</b>
  <br />
  A desktop app you download and run — macOS and Windows. Write the script, pick a voice, and it handles the rest: per-line voice-over, matched frames, one finished MP4.
  <br />
  <sub>The voice engine ships with the app and runs offline — no network, no per-minute cost, your audio never leaves your machine.</sub>
</p>

<p align="center">
  <a href="#download"><strong>Download</strong></a> ·
  <a href="#your-first-video-in-three-steps"><strong>Three steps</strong></a> ·
  <a href="#the-four-pipelines"><strong>What it makes</strong></a> ·
  <a href="#voice-where-it-runs-and-what-it-costs"><strong>Voice &amp; cost</strong></a> ·
  <a href="#when-something-goes-wrong"><strong>Troubleshooting</strong></a> ·
  <a href="#for-developers"><strong>For developers</strong></a>
</p>

<p align="center">
  <img src="https://cdn.jsdelivr.net/gh/webkubor/free-imgbed@main/reel-kit/demo.gif" width="240" alt="reel-kit demo" />
</p>

<p align="center">
  <sub>The clip above was made by reel-kit itself — assets, voice-over and compositing all local;
  <a href="examples/">examples/</a> contains the full reproducible input</sub>
</p>

<p align="center">
  <img src="https://cdn.jsdelivr.net/gh/webkubor/free-imgbed@main/reel-kit/preview.webp" width="720" alt="three-frame preview" />
</p>

---

## Download

Current version **v1.3.0** (released 2026-11-10). [Releases & checksums](https://github.com/webkubor/reel-kit/releases/tag/app-v1.3.0) · [Website](https://webkubor.github.io/reel-kit/)

| Platform | Download | Size | Where it lands |
|---|---|---|---|
| **macOS** (Apple Silicon) | [reel-studio-1.3.0-arm64.dmg](https://github.com/webkubor/reel-kit/releases/download/app-v1.3.0/reel-studio-1.3.0-arm64.dmg) | 237 MB | Drag into Applications |
| **macOS** (Apple Silicon) | [reel-studio-1.3.0-darwin-arm64.tar.gz](https://github.com/webkubor/reel-kit/releases/download/app-v1.3.0/reel-studio-1.3.0-darwin-arm64.tar.gz) | 289 MB | Unpack, then drag `Reel Studio.app` into Applications |
| **Windows** (x64) | [reel-studio-1.3.0-windows-amd64.tar.gz](https://github.com/webkubor/reel-kit/releases/download/app-v1.3.0/reel-studio-1.3.0-windows-amd64.tar.gz) | 347 MB | Unpack, then run `Reel Studio.exe` |

**Keep the folder layout on Windows**: `Reel Studio.exe` and the folders `backend/`, `bin/`, `chrome/`, `museav/`, `sherpa/` must sit side by side — the app won't start if one is missing.

The first launch is blocked by macOS Gatekeeper or Windows SmartScreen, because the build carries no signature or notarization. That's expected, and [the troubleshooting section](#when-something-goes-wrong) says how to get past it.

macOS can also install from the command line:

```bash
curl -fsSL https://github.com/webkubor/reel-kit/releases/download/app-v1.3.0/install.sh | sh
```

Linux users, or anyone who prefers the terminal: see [For developers](#for-developers).

## Your first video in three steps

The left column holds **four video pipelines**. Click one, the middle column is that pipeline's configuration, the right column is what it produced.

**Step 1 · Write the script**
Open "Knowledge Talk" and write the voice-over script — one line per line, and one line becomes one shot. The form shows the shot count as you type. Up to 8 shots, 80 characters per line.

```
If you were given ten thousand taels of gold, but had to do corvée labour for the Qin dynasty — would you take it?
```

**Step 2 · Pick a voice**
Choose one of the built-in local timbres under "Voice". No login, no extra install, no cost. Once a voice is picked, every line is spoken and each shot is timed to its own narration — slow lines get longer frames, fast lines get shorter ones.

**Step 3 · Export**
When the finished video shows up in the list on the right, play it or "save to local" as an MP4. The same configuration can generate another take, which makes A/B comparisons easy.

You don't need assets to start: the layout and palette follow the chosen visual style (44 available). Import your own images or clips under "Library" in the left column.

## The four pipelines

| Pipeline | What it makes |
|---|---|
| **Knowledge talk** | A line-per-shot script → per-line voice-over → each shot timed to the narration → rendered frames → one MP4. Vertical 1080×1920 |
| **Project spotlight** | Give it a GitHub repository URL → it reads the README and screenshots → writes the shot list → narrates it → a video about the project |
| **Assemble** | Put images and clips from the library in order, add voice-over and BGM, export the result |
| **Person / product** | An intro piece for a product or a person: full-frame vertical shots, or landscape with image left and text right |

## Voice: where it runs and what it costs

| What you want | Where it runs | Cost | What you need |
|---|---|---|---|
| **Standard voice-over** (default) | The offline engine bundled with the app | Free | Nothing — same engine on macOS and Windows |
| **Clone your own voice** / design a timbre | VoxFlow, on your machine | Free | VoxFlow on an Apple Silicon Mac, or running it on another machine |
| **A better cloud timbre** | MiniMax TTS | Billed by usage | A MiniMax key, once, under Settings → Cloud voice key |
| **Generate images / video** | MuseAV, MiniMax direct, or your own channel | Billed by that account | Log into MuseAV once in the app, add a vendor key, or add your own gateway |

Three rules worth knowing up front:

- **The channel you pick is the channel that runs.** When one isn't ready, the interface says what's missing and disables rendering — it never quietly switches to a different, possibly more expensive, provider.
- **A failed paid task isn't charged twice.** If the asset was generated but the download failed, the history offers "fetch the generated asset" — fetching doesn't re-bill.
- **Model, timbre and account are three separate things**, configured separately.

## Where your stuff lives

Assets, scripts and finished videos stay on your own machine (`~/.reel-kit/studio/`); by default the app uploads nothing. Generating AI assets calls the channel you chose, so those follow that provider's terms.

## When something goes wrong

Try these first:

| Symptom | Usually |
|---|---|
| Double-clicking does nothing | The downloaded file is still flagged by the OS — see the next section |
| No voice in the output | The channel isn't ready — the hint next to "Voice" says what's missing |
| A task failed | Open that failed task in the video list on the right: it shows which step failed and the exact error |
| Which version am I on | Settings → Software update shows the current version, and checks for new ones |

**macOS says "cannot be opened because the developer cannot be verified"**: the build isn't notarized (it's ad-hoc signed). Right-click the app → "Open" → "Open" again, or run:

```bash
xattr -dr com.apple.quarantine "/Applications/Reel Studio.app"
```

**Windows says "Windows protected your PC"**: click "More info" → "Run anyway". If double-clicking does nothing at all, select `Reel Studio.exe` in Explorer → right-click "Properties" → tick "Unblock" at the bottom → OK, then run it.

**Windows crashes on launch**: open Explorer, type `%TEMP%`, and look for `reel-studio-crash.log` — it holds the full crash stack. A few early startup failures don't produce this file.

**A task failed, or the UI shows an error — this is the part that lets us find it fast**:

1. Open "Settings & Feedback" → "Runtime logs" → "Export diagnostic report" to get a `reel-studio-diagnostics-*.txt`.
2. It contains the version and OS, channel readiness, **the error text of failed tasks and the step they stopped at**, plus recent backend and UI log lines. Common keys and tokens are masked automatically.
3. Check it for anything private, then attach it to a [new issue](https://github.com/webkubor/reel-kit/issues/new/choose).

There's also "copy diagnostics and recent logs", which puts the same text on your clipboard for pasting straight into an issue. The backend log itself lives at `%TEMP%\reel-studio-backend.log`; "Open backend log" opens it for you.

## Known limitations

Better to say it up front than to let you hit it:

- **No code signing, no notarization.** macOS and Windows both gate the first launch; the workarounds are above.
- **The Windows build is still in real-user testing.** The CLI and backend are covered by CI; the desktop UI has not yet been through a full hands-on acceptance pass. Reports from real users are exactly what this needs — see the reporting path above.
- **The offline voice is a bit flat.** It's good enough for narration; for a warmer delivery, switch to a cloud timbre.
- **Knowledge talk is fixed at 1080×1920 vertical.** Landscape layouts live in the person/product pipeline.
- **Not a general editor.** Color grading, multi-track and frame-accurate alignment belong to DaVinci Resolve.

---

# For developers

The same engine is available from the terminal, and the app is a thin native shell over it.

## Quick start

```bash
# no install needed
npx github:webkubor/reel-kit make \
  --template sticker-promo \
  --title "reel-kit" \
  --assets ./shots \
  --caps ./captions.txt \
  --out out.mp4
```

The repo ships full assets and captions, so a clone reproduces the demo clip:

```bash
git clone git@github.com:webkubor/reel-kit
cd reel-kit && pnpm install && npm link

reel make --template sticker-promo \
  --title "reel-kit" --subtitle "Vertical short-form video studio" \
  --assets ./examples/assets --caps ./examples/demo-caps.txt \
  --out demo.mp4
```

Dependencies: **ffmpeg** (compositing) and a **local Chrome** (renders the layout — it does not download an extra chromium; override the path with `CHROME_PATH`). Voice-over is optional: without `--voice` you don't need any TTS.

Full parameter reference: **[docs/cli.md](docs/cli.md)** (Chinese) — or the summary below.

## What problem it solves

Making emoji-sticker promos or product shorts usually means improvising an ffmpeg incantation every time. **The output stays, the workflow doesn't** — the folder keeps the mp4, the assets and the captions, while the compositing logic is thrown away, so next time you re-derive it from scratch.

reel-kit turns that workflow into a reusable command.

## vs other approaches

| | JianYing / CapCut | Raw FFmpeg | MoviePy | Remotion | **reel-kit** |
|---|:---:|:---:|:---:|:---:|:---:|
| Batch | ❌ manual | ✅ | ✅ | ✅ | ✅ |
| Cost of changing layout | drag the timeline | **write drawtext filter chain** | write Python | write React | **write CSS** |
| Chinese typography | ✅ | ⚠️ hand-typed font path/stroke/wrap | ⚠️ font metrics traps | ✅ | ✅ browser layout |
| Voice-over | manual import | DIY | DIY | DIY | ✅ built-in, **local TTS, zero cost** |
| Shot duration | manual drag | fixed | fixed | programmable | ✅ **shot length follows voice length** |
| Runtime deps | client app | ffmpeg | Python + moviepy | Node + React + Chromium | Node + ffmpeg + **local Chrome** |
| Scope | general editing | general | general | general | **templated vertical short-form** |

**reel-kit is not a general-purpose editor** — it only does "one image per shot + per-line captions" style templated shorts. Color grading, multi-track, frame-accurate alignment — that's DaVinci Resolve territory, a different problem.

### Three concrete differences

**① Adding a template = drop in an HTML, no code changes**

Layout is `templates/*.html`, placeholders `{{title}}` `{{caption}}` `{{image}}`. To change font size, add curved decoration, swap palette — edit a few lines of CSS, see the result directly in the browser.

Doing the same with ffmpeg's `drawtext`: hand-write Chinese font paths, stroke, wrap, centering; curved decoration isn't even drawable — and each change means re-tuning a filter chain without seeing the effect.

**② Shot length = voice length**

```
Fixed 2.5s/shot  →  20.0s   fast lines stall, slow lines get cut off
Voice-driven      →  16.7s   shots 1.7~2.7s, natural pacing
```

**③ Voice via local TTS, batch at zero cost**

The desktop app bundles an offline engine (sherpa-onnx) for the free path. The CLI's default engine is [voxcraft](https://github.com/webkubor/voxcraft) (local Qwen3-TTS) — one-time 4.2 GB model download, then every sentence is free, with voice cloning for IP-specific timbre. Alternatively `--voice-engine minimax` for the cloud (billed by usage).

> voxcraft's timbre library is empty by default. Until you register a timbre, `--voice` will fail.
> reel-kit auto-locates voxcraft on your machine (its `voice` command lives in a venv, not on PATH by default). When something is missing, it reports clearly and gives a copy-pasteable fix command — **it will never silently download a 4.2 GB model**.
> `museav speak` is unavailable in the current MuseAV CLI, so it is not offered as a voice backend.

## 🎙️ Voice

```bash
reel make ... --voice <timbre>                        # voxcraft local (default)
reel make ... --voice <timbre> --voice-engine minimax # MiniMax TTS, billed by usage
```

| | voxcraft (default) | minimax |
|---|---|---|
| Where | local Qwen3-TTS Base-1.7B | MiniMax cloud API |
| Cost | one-time 4.2 GB model, then free | billed by usage |
| Timbres | voice cloning / text-described timbres | vendor timbre IDs |
| Prerequisite | must register a timbre first | `MINIMAX_API_KEY` in the environment |

### How audio/video alignment is guaranteed

After each voice-over, pad silence to `(shot duration − voice duration)` via `apad=whole_dur=<shot duration>`, then concat head-to-tail. **No offset arithmetic, so no accumulated drift.**

Measured across 8 shots: shot head -13 to -19 dB (voice), shot tail -91 dB (padded silence), still exact at shot 8.

When BGM is added, it's auto-attenuated to 0.22 gain as a bed (adjustable via `--bgm-gain`). Measured: voice segments -17 to -21 dB, gaps pure BGM -25 to -33 dB, 6–13 dB gap — voice up front, BGM doesn't compete.

## 🎵 BGM library (optional)

Easiest usage is to point at a file directly:

```bash
reel make ... --bgm ./local.mp3
```

If you have a batch of regular BGM, build a manifest and look them up by **alias** without re-pathing every time:

```bash
export REEL_BGM_MANIFEST=https://your-domain/music-manifest.json
# or write to ~/.reel-kit/config.json: { "bgmManifest": "..." }

reel bgm                        # list available tracks (alias / duration / mood / license)
reel make ... --bgm upbeat      # fetch by alias, auto-download on first use, cache to ~/.reel-kit/bgm/
```

Manifest format (`REEL_BGM_MANIFEST` can be a URL or a local json path):

```json
{
  "cdn": "https://your-domain/",
  "bgm": [
    {
      "alias": "upbeat",
      "key": "bgm/upbeat.mp3",
      "duration": 113,
      "mood": ["upbeat", "daily"],
      "license": "CC0 / attribution-free commercial OK",
      "source": "where the track came from"
    }
  ]
}
```

`key` can be a path relative to `cdn`, a full URL, or a local absolute path.

> **reel-kit ships no music** — licensing is the user's responsibility; bundling a batch would only cause you trouble.

### When `license` is filled in, it prints at render time

```
[reel] BGM "upbeat" license: CC0 / attribution-free commercial OK
```

**Every** render, not just the first download. Videos ship externally, so "can this be used commercially, does it need attribution" has to be visible *at* the render moment. Remote manifests cache for a day, falling back to the cached version when offline; a failed fetch won't fail the render. Local manifests don't cache — edit, take effect immediately.

## ⚙️ Common parameters

```
--template <name>      Template, default sticker-promo (`reel templates` for all)
--style <name>         Knowledge-video visual style (44 in the library)
--assets <dir|file>    Asset images, directory sorted by filename
--caps <file>           Per-line captions, one per line, line count = shot count
--title/--subtitle/--footer   Template text
--bgm <alias|file>     Background music, auto-loop to fill + fade out at end. Aliases from `reel bgm`
--bgm-gain <0~1>       BGM gain, defaults to 0.22 when voice-over is on
--voice <timbre>       Enable voice-over, shot duration driven by voice length
--voice-engine <name>  voxcraft (default) | minimax
--voice-margin <sec>   Buffer after voice per shot, default 0.45
--per-shot <sec>       Fixed shot length without voice, default 2.5
--size <WxH>           Canvas, default 1080x1920
--transition <name>    none (default) | fade | slide-* | wipe-* | dissolve | zoom-in
--keep-frames          Keep intermediate frames, useful for tweaking layout
```

## 📦 Repo structure

```
reel-kit/                          # @kubor/reel-kit
├── bin/                           # CLI entry (the `reel` command)
├── src/                           # video compositing core (also exported as a lib: import { renderFrames, compose, ... })
├── templates/                     # 10 HTML templates
├── prompts/
│   └── handdrawn/                 # 20 hand-drawn style recipes
├── studio/                        # local web workbench
├── apps/reel-studio/              # desktop app (Go + mygo native UI)
├── docs/                          # design trade-offs, compositing notes, CLI reference
└── examples/                      # fully reproducible demo assets
```

## 🧩 Design trade-offs

**Why use a browser for layout rendering** — see "Three concrete differences" ① above. The cost is one screenshot per frame, but a piece has only 8–20 frames, just seconds. As a bonus, this dodges the entire class of MoviePy-ecosystem issues with font metrics, visual centering, and handle leaks (see [`docs/video-compositing-notes.md`](docs/video-compositing-notes.md)).

**Why concat demuxer instead of `-framerate 1/2.5`** — the latter requires every shot to have equal length, but "let the last shot linger a moment" is a natural requirement.

> ⚠️ ffmpeg's concat list **must not** duplicate the last frame: doing so on ffmpeg 7.x causes it to emit one extra shot's worth of duration (5 shots × 2.5 s should be 12.5 s, with duplication measured 15.0 s; without duplication measured 12.43 s). Don't add `which time` from older docs — see `AGENTS.md` rule 1.

**When asset count and caption count differ**, take the smaller side and **explicitly report which weren't used** — silent truncation makes you think the piece is complete when it isn't.

**voxcraft runs in web-server mode** — its CLI reloads the 4.2 GB model on every call, which means 8 sentences = 8 model loads, slow and unstable (measured crash at sentence 5 with `libc++ recursive_mutex`). Now: start one server, make multiple calls, tear down after use. Already-running servers get reused.

## 🖼 Asset requirements

Templates display assets as **subject images** centered. For sticker effect, assets must be **pre-cutout** (transparent PNG). This tool does not do cutout — that's an upstream concern; doing it here would just be a second implementation.

## ⚠️ Known limitations (engine)

- **One line, one shot**: long lines wrap naturally in the template, but don't split into two shots (`--caps-split` does it manually).
- **BGM shorter than the video loops**: filled with `aloop` and faded out; if the seam is obvious, use a longer track.
- **Not a general editor**: color grading, multi-track, frame-accurate alignment — DaVinci Resolve territory.

## 🖼 Thumbnail / storyboard-frame understanding (mlx-vlm-kit)

This tool has no image-recognition model built in. For thumbnail QA, storyboard-frame content understanding, copy matching, use the local-shared [mlx-vlm-kit](https://github.com/webkubor/mlx-vlm-kit) (free / offline):

```bash
vlm describe ./ep07-thumb.jpg                       # thumbnail content
vlm ask frame01.png --q "does this frame match script line 3"   # script alignment check
```

In an agent workflow, you can also call via [cs-mcp](https://github.com/webkubor/cs-mcp).

## 🌐 GitHub Pages site

`docs/index.html` is the official single-page site for reel-kit; once deployed to GitHub Pages it's accessible at `https://webkubor.github.io/reel-kit/`.

**Enable in 30 seconds:**

1. GitHub repo → **Settings** → **Pages**
2. **Source** pick `Deploy from a branch`
3. **Branch** `main` · **Folder** `/docs`
4. Save, a few minutes later it goes live

## Star History

<a href="https://www.star-history.com/#webkubor/reel-kit&Date">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/svg?repos=webkubor/reel-kit&amp;type=Date&amp;theme=dark" />
    <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/svg?repos=webkubor/reel-kit&amp;type=Date" />
    <img alt="Star History Chart for webkubor/reel-kit" src="https://api.star-history.com/svg?repos=webkubor/reel-kit&amp;type=Date" width="100%" />
  </picture>
</a>

## 📄 License

MIT
