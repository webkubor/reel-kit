#!/usr/bin/env bash
# 端到端实测：用「打包版布局」的工作台，配「随应用自带」的离线引擎，出一条真视频。
#
# 跑的是 apps/reel-studio/resources/darwin-arm64 里那套真实产物：
# 自带的 node、自带的 chrome headless、自带的 ffmpeg、自带的本机语音引擎。
set -euo pipefail

RES="$PWD/apps/reel-studio/resources/darwin-arm64"
HOME_DIR="/tmp/rs-e2e-home"
PORT=4397
rm -rf "$HOME_DIR"; mkdir -p "$HOME_DIR"

cd "$RES/backend"
PATH="$RES/bin:$PATH" \
CHROME_PATH="$RES/chrome/chrome-headless-shell" \
REEL_SHERPA_HOME="$RES/sherpa" \
REEL_STUDIO_HOME="$HOME_DIR" \
REEL_STUDIO_PORT="$PORT" \
"$RES/bin/node" bin/reel.mjs studio >/tmp/rs-e2e-backend.log 2>&1 &
SERVER_PID=$!
trap 'kill $SERVER_PID 2>/dev/null || true' EXIT

for _ in $(seq 1 60); do
  if curl -s --max-time 2 "http://127.0.0.1:$PORT/api/state" >/dev/null 2>&1; then break; fi
  sleep 0.5
done

curl -s "http://127.0.0.1:$PORT/api/providers" | python3 -c "
import json,sys
d=json.load(sys.stdin)
print('语音总状态:', d['voice']['note'])
for c in d['voice']['channels']:
    print('  通道', c['id'], '| ready =', c['ready'], '|', c['note'])
"

PROJECT=$(curl -s -X POST "http://127.0.0.1:$PORT/api/projects" \
  -H 'X-Reel-Action: studio' -H 'Content-Type: application/json' \
  -d '{"name":"离线配音实测"}' | python3 -c "import json,sys;print(json.load(sys.stdin)['id'])")
echo "项目：$PROJECT"

JOB_RESP=$(curl -s -w "\n%{http_code}" -X POST "http://127.0.0.1:$PORT/api/jobs" \
  -H 'X-Reel-Action: studio' -H 'Content-Type: application/json' \
  -d "{\"kind\":\"talk\",\"projectId\":\"$PROJECT\",\"title\":\"下载即用实测\",\"captions\":\"这一条视频的旁白，由随应用自带的本机离线引擎合成。\\n没有安装任何东西，也没有登录任何账号。\",\"voiceBackend\":\"sherpa\",\"persona\":\"zh-fanchen-c\",\"ratio\":\"9:16\"}")
HTTP_CODE=$(echo "$JOB_RESP" | tail -1)
JOB_BODY=$(echo "$JOB_RESP" | sed '$d')
echo "提交任务 HTTP $HTTP_CODE: $JOB_BODY"
if [ "$HTTP_CODE" != "202" ]; then
  echo "任务提交失败" >&2
  exit 1
fi

for _ in $(seq 1 240); do
  STATUS=$(curl -s "http://127.0.0.1:$PORT/api/state" | python3 -c "
import json,sys
d=json.load(sys.stdin)
job=d['jobs'][-1] if d.get('jobs') else None
print(job['status'] if job else 'none')
")
  case "$STATUS" in
    succeeded|failed|interrupted) break ;;
  esac
  sleep 1
done
echo "任务状态：$STATUS"

curl -s "http://127.0.0.1:$PORT/api/state" | python3 -c "
import json,sys
d=json.load(sys.stdin)
job=d['jobs'][-1]
print('任务错误：', job.get('error'))
for a in d['assets']:
    print('素材：', a.get('name'), '|', a.get('mime'), '|', a.get('engine',''))
" | tail -8

VIDEO_ID=$(curl -s "http://127.0.0.1:$PORT/api/state" | python3 -c "
import json,sys
d=json.load(sys.stdin)
for a in d['assets']:
    if a.get('mime')=='video/mp4':
        print(a.get('id'))
        break
")
VIDEO_FILE="$HOME_DIR/media/$VIDEO_ID.mp4"
echo "成片文件：$VIDEO_FILE"
if [ -f "$VIDEO_FILE" ]; then
  "$RES/bin/ffprobe" -v error -show_entries format=duration,size:stream=codec_name,width,height,channels,sample_rate -of default=nw=1 "$VIDEO_FILE"
  ls -lh "$VIDEO_FILE"
  echo "✅ 端到端实测成功：自带 Node + Chrome Headless + FFmpeg + 离线语音引擎，出片完整"
else
  echo "没有产出成片" >&2
  tail -20 /tmp/rs-e2e-backend.log
  exit 1
fi
