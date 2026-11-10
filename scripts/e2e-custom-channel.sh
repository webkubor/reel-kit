#!/usr/bin/env bash
# 端到端实测：自定义模型通道能不能真的出一张图，并接进合成流程。
#
# 跑的是源码树里的真实后端（node bin/reel.mjs studio），不是打桩：
#   配置通道 → 探活（不花钱）→ 真生成一张图 → 落进素材库 → 合成出成片
#
# 用法：
#   MINIMAX_API_KEY=xxx ./scripts/e2e-custom-channel.sh minimax-image
#   MUSEAV_API_KEY=xxx  ./scripts/e2e-custom-channel.sh museav-http
#
# 密钥也可以不在这儿给：设置页里存过的密钥会自动复用（同厂商不用填两遍）。
#
# 退出码：0 = 闭环成功；非 0 = 断在哪一步，脚本会说明。
set -euo pipefail

CHANNEL="${1:-minimax-image}"
PORT="${REEL_STUDIO_PORT:-4398}"
# 每次跑都用全新的空目录，别去清固定的路径（清别人的东西是事故）
HOME_DIR="$(mktemp -d "${TMPDIR:-/tmp}/rs-channel-e2e.XXXXXX")"
BACKEND_LOG="$HOME_DIR/backend.log"
# 样片留在仓库外，别跟着临时目录一起被删 —— 跑完要能自己看一眼
OUT_DIR="${REEL_E2E_OUT:-$HOME_DIR/out}"
mkdir -p "$OUT_DIR"

cd "$(dirname "$0")/.."

echo "通道：$CHANNEL"
echo "数据目录：$HOME_DIR"
echo "样片目录：$OUT_DIR"

# 端口先占位检查：端口被别的东西占了还接着跑，会把别人的服务当成自己的
if curl -s --max-time 2 "http://127.0.0.1:$PORT/api/state" >/dev/null 2>&1; then
  echo "❌ 端口 $PORT 上已经有服务在跑。先停掉它，或设 REEL_STUDIO_PORT 换个端口。" >&2
  exit 1
fi

cleanup() {
  [ -n "${SERVER_PID:-}" ] && kill "$SERVER_PID" 2>/dev/null || true
  rm -rf "$HOME_DIR"
}
trap cleanup EXIT

REEL_STUDIO_HOME="$HOME_DIR" \
REEL_STUDIO_PORT="$PORT" \
node bin/reel.mjs studio >"$BACKEND_LOG" 2>&1 &
SERVER_PID=$!

for _ in $(seq 1 60); do
  if curl -s --max-time 2 "http://127.0.0.1:$PORT/api/state" >/dev/null 2>&1; then break; fi
  sleep 0.5
done
if ! curl -s --max-time 2 "http://127.0.0.1:$PORT/api/state" >/dev/null 2>&1; then
  echo "❌ 工作台没起来" >&2
  tail -20 "$BACKEND_LOG"
  exit 1
fi

api() { curl -s --max-time 20 "$@"; }

# 提交任务并回读任务 id —— 不能靠 jobs[-1] 猜，那会把别的任务当自己的
submit_job() {
  local payload="$1"
  local response
  response="$(api -X POST "http://127.0.0.1:$PORT/api/jobs" \
    -H 'X-Reel-Action: studio' -H 'Content-Type: application/json' \
    -d "$payload")"
  printf '%s' "$response" | python3 -I -c "import json,sys;d=json.load(sys.stdin);print(d.get('id',''))"
}

# 等某个任务跑完，返回它的状态
wait_job() {
  local job_id="$1" status=none i
  for i in $(seq 1 300); do
    status="$(api "http://127.0.0.1:$PORT/api/state" | JOB_ID="$job_id" python3 -I -c "
import json,os,sys
d=json.load(sys.stdin)
job=next((j for j in d.get('jobs',[]) if j.get('id')==os.environ['JOB_ID']),None)
print(job['status'] if job else 'none')
")"
    case "$status" in succeeded|failed|interrupted) break ;; esac
    sleep 1
  done
  printf '%s' "$status"
}

# ── 第一步：探活（不生成任何东西，不花钱）──────────────────────────────
echo
echo "── 探活 ──"
if ! node bin/reel.mjs channels check "$CHANNEL"; then
  echo "探活没过，先按上面几行改配置" >&2
  exit 1
fi

# ── 第二步：真生成一张图（这一步花钱，只跑一次）────────────────────────
PROJECT="$(api -X POST "http://127.0.0.1:$PORT/api/projects" \
  -H 'X-Reel-Action: studio' -H 'Content-Type: application/json' \
  -d '{"name":"自定义通道实测"}' | python3 -I -c "import json,sys;print(json.load(sys.stdin)['id'])")"
echo "项目：$PROJECT"

IMAGE_JOB="$(submit_job "{\"kind\":\"image\",\"projectId\":\"$PROJECT\",\"backend\":\"$CHANNEL\",\"prompt\":\"一只橘猫坐在窗台上看雨，暖色调\",\"ratio\":\"9:16\"}")"
if [ -z "$IMAGE_JOB" ]; then
  echo "生成任务提交失败" >&2
  exit 1
fi
echo "生成任务：$IMAGE_JOB"

STATUS="$(wait_job "$IMAGE_JOB")"
echo "生成任务状态：$STATUS"

IMAGE_ID="$(api "http://127.0.0.1:$PORT/api/state" | PROJECT="$PROJECT" python3 -I -c "
import json,os,sys
d=json.load(sys.stdin)
for a in d.get('assets',[]):
    if str(a.get('mime','')).startswith('image/'):
        print(a.get('id'))
        break
")"
if [ "$STATUS" != "succeeded" ] || [ -z "$IMAGE_ID" ]; then
  echo "生成没成功，任务错误见下" >&2
  api "http://127.0.0.1:$PORT/api/state" | python3 -I -c "
import json,sys
d=json.load(sys.stdin)
for j in d.get('jobs',[]):
    if j.get('error'): print(j.get('error'))
"
  tail -20 "$BACKEND_LOG"
  exit 1
fi
echo "生成图片素材：$IMAGE_ID"

# ── 第三步：把这张图接进合成，出成片 ─────────────────────────────────
EXPORT_JOB="$(submit_job "{\"kind\":\"export\",\"projectId\":\"$PROJECT\",\"title\":\"通道闭环实测\",\"captions\":\"这张图由自定义模型通道生成。\",\"assetIds\":[\"$IMAGE_ID\"],\"template\":\"knowledge-paper\",\"ratio\":\"9:16\",\"duration\":3}")"
if [ -z "$EXPORT_JOB" ]; then
  echo "合成任务提交失败" >&2
  exit 1
fi
echo "合成任务：$EXPORT_JOB"

STATUS="$(wait_job "$EXPORT_JOB")"
echo "合成任务状态：$STATUS"

VIDEO_ID="$(api "http://127.0.0.1:$PORT/api/state" | python3 -I -c "
import json,sys
d=json.load(sys.stdin)
for a in d.get('assets',[]):
    if a.get('mime')=='video/mp4':
        print(a.get('id'))
        break
")"
VIDEO_FILE="$HOME_DIR/media/$VIDEO_ID.mp4"
if [ "$STATUS" != "succeeded" ] || [ ! -f "$VIDEO_FILE" ]; then
  echo "没有产出成片" >&2
  tail -20 "$BACKEND_LOG"
  exit 1
fi

# ffprobe 不一定在 PATH 上：先用系统里的，没有就用随包分发的那个
FFPROBE="$(command -v ffprobe || true)"
if [ -z "$FFPROBE" ]; then
  for candidate in "$PWD/apps/reel-studio/resources/darwin-arm64/bin/ffprobe" \
                   "$PWD/apps/reel-studio/resources/darwin-amd64/bin/ffprobe"; do
    [ -x "$candidate" ] && FFPROBE="$candidate" && break
  done
fi
if [ -z "$FFPROBE" ]; then
  # 探测不了成片就不能说闭环成功 —— 片子在，但没人证明它是好的
  echo "❌ 没找到 ffprobe，无法验证成片是否正常" >&2
  exit 1
fi
"$FFPROBE" -v error -show_entries format=duration,size:stream=codec_name,width,height -of default=nw=1 "$VIDEO_FILE"

cp "$VIDEO_FILE" "$OUT_DIR/channel-e2e.mp4"
ls -lh "$OUT_DIR/channel-e2e.mp4"
echo "✅ 端到端闭环成功：自定义通道出图 → 落素材库 → 合成出成片"
echo "   成片：$OUT_DIR/channel-e2e.mp4"
