import { spawn } from 'node:child_process';

/**
 * museav CLI 封装。
 *
 * 工作台自己不写任何模型调用代码 —— 出图、出视频、超分、抠图全部 shell out
 * 到 museav（https://github.com/webkubor/museav-cli），密钥、路由、记账都在中台那边。
 *
 * ⚠️ 解析只认 stdout。museav 把人类可读的进度写 stderr、把 JSON 写 stdout；
 *    照着终端里看到的样子去解析会抓空。
 */

const BIN = process.env.MUSEAV_BIN || 'museav';

export class MuseavError extends Error {
  constructor(message, { code, stderr } = {}) {
    super(message);
    this.name = 'MuseavError';
    this.statusCode = 502;
    this.exitCode = code;
    this.stderr = stderr;
  }
}

/**
 * 跑一条 museav 命令。
 * @param {string[]} args
 * @param {(line: string) => void} [onProgress] 逐行拿 stderr，用来往前端推进度
 */
export function runMuseav(args, onProgress) {
  return new Promise((resolve, reject) => {
    let child;
    try {
      child = spawn(BIN, args, { stdio: ['ignore', 'pipe', 'pipe'] });
    } catch (err) {
      reject(new MuseavError(`无法启动 ${BIN}：${err.message}`));
      return;
    }

    let stdout = '';
    let stderr = '';
    let pending = '';

    child.stdout.setEncoding('utf8');
    child.stdout.on('data', (chunk) => {
      stdout += chunk;
    });

    child.stderr.setEncoding('utf8');
    child.stderr.on('data', (chunk) => {
      stderr += chunk;
      if (!onProgress) return;
      pending += chunk;
      const lines = pending.split('\n');
      pending = lines.pop() ?? '';
      for (const line of lines) {
        const text = line.trim();
        if (text) onProgress(text);
      }
    });

    child.on('error', (err) => {
      reject(
        new MuseavError(
          err.code === 'ENOENT'
            ? `找不到 ${BIN} 命令。装一个：npm i -g museav-cli`
            : `${BIN} 启动失败：${err.message}`,
          { stderr },
        ),
      );
    });

    child.on('close', (code) => {
      if (pending.trim() && onProgress) onProgress(pending.trim());
      if (code === 0) resolve({ stdout, stderr });
      else reject(new MuseavError(firstMeaningfulLine(stderr) || `museav 退出码 ${code}`, { code, stderr }));
    });
  });
}

/**
 * 从 stderr 里挑一行像样的错误信息给前端，不把整坨日志糊上去。
 *
 * museav 把进度和错误都写 stderr，且**错误在最后**。取第一行只会拿到
 * 「上传垫图 [图片1] …」这种进度文案，把真正的原因（❌ ENOENT: …）吞掉。
 * 所以先找带错误标记的行，没有才退回最后一条非空行。
 */
function firstMeaningfulLine(stderr) {
  const lines = stderr
    .split('\n')
    .map((l) => l.trim())
    .filter((l) => l && !l.startsWith('at '));

  const marked = lines.filter((l) => /❌|✗|error|failed|失败|错误/i.test(l));
  return marked.at(-1) ?? lines.at(-1);
}

function parseJson(stdout, what) {
  const text = stdout.trim();
  if (!text) throw new MuseavError(`museav ${what} 没有输出`);
  try {
    return JSON.parse(text);
  } catch {
    // 少数命令会在 JSON 前后带杂物，兜底抓最外层的数组/对象
    const match = text.match(/[[{][\s\S]*[\]}]/);
    if (match) {
      try {
        return JSON.parse(match[0]);
      } catch {
        /* 落到下面统一报错 */
      }
    }
    throw new MuseavError(`museav ${what} 的输出不是合法 JSON`);
  }
}

/** 出图工作流列表。服务端固定返回最近 50 条。 */
export async function listJobs({ limit, status } = {}) {
  const args = ['jobs'];
  if (limit) args.push('--limit', String(limit));
  if (status) args.push('--status', status);
  const { stdout } = await runMuseav(args);
  const jobs = parseJson(stdout, 'jobs');
  return Array.isArray(jobs) ? jobs : [];
}

/**
 * 从 museav 输出里挑出**生成结果**的 URL。
 *
 * 坑：用 --ref 垫图时，stderr 会回显一条「图片1 就绪: …/refs/<uuid>.jpg」——
 * 那是垫图上传后的地址，不是出图结果。早先的实现把 stdout+stderr 拼起来取最后一个
 * http 链接，结果稳定拿到垫图，用户看到的「出图结果」其实是自己传进去的参考图。
 *
 * 所以：先看 stdout（机器可读那一路），再退回 stderr，两边都排除 /refs/。
 * 一个结果 URL 都找不到时**报错**，绝不退而求其次返回垫图。
 */
function extractResultUrl(...texts) {
  for (const text of texts) {
    const url = (text.match(/https?:\/\/\S+/g) ?? [])
      .map((u) => u.replace(/[),.\s]+$/, ''))
      .filter((u) => !u.includes('/refs/'))
      .at(-1);
    if (url) return url;
  }
  return null;
}

/**
 * 出图 / 出视频。这是**花钱**的操作，调用方必须是用户的显式动作。
 *
 * museav gen 自带轮询，会一直跑到出结果，所以这个 Promise 可能几十秒才 resolve。
 */
export async function generate(options, onProgress) {
  const {
    prompt,
    skill,
    template,
    input,
    ratio,
    model,
    quality,
    refs = [],
    video = false,
    duration,
    firstFrame,
    transparent = false,
    project,
  } = options;

  if (!prompt && !skill && !template) {
    throw Object.assign(new Error('prompt / skill / template 至少给一个'), { statusCode: 400 });
  }
  if (refs.length > 5) {
    throw Object.assign(new Error('垫图最多 5 张'), { statusCode: 400 });
  }

  const args = ['gen'];
  if (prompt) args.push('--prompt', prompt);
  if (skill) args.push('--skill', skill);
  if (template) args.push('--template', template);
  if (input) args.push('--input', input);
  if (ratio) args.push('--ratio', ratio);
  if (model) args.push('--model', model);
  if (quality) args.push('--quality', quality);
  if (transparent) args.push('--transparent');
  if (project) args.push('--project', project);
  for (const ref of refs) args.push('--ref', ref);
  if (video) {
    args.push('--video');
    if (duration) args.push('--duration', String(duration));
    if (firstFrame) args.push('--image', firstFrame);
  }

  const { stdout, stderr } = await runMuseav(args, onProgress);

  const url = extractResultUrl(stdout, stderr);
  if (!url) throw new MuseavError('museav 没有返回结果 URL', { stderr });

  return { url, raw: stdout.trim() };
}

/** 本地后处理，免登录、不花钱 */
export async function postProcess(action, file, extraArgs = []) {
  const allowed = new Set(['upscale', 'remove-bg', 'remove-watermark', 'compress']);
  if (!allowed.has(action)) {
    throw Object.assign(new Error(`不支持的后处理: ${action}`), { statusCode: 400 });
  }
  const { stdout, stderr } = await runMuseav([action, ...extraArgs, file]);
  const out = `${stdout}\n${stderr}`.match(/\S+\.(png|jpg|jpeg|webp)/gi)?.at(-1);
  return { output: out ?? null, raw: stdout.trim() };
}

/** museav 在不在、登没登录 —— 工作台启动时探一次，别等用户点了出图才报错 */
export async function health() {
  try {
    const { stdout } = await runMuseav(['--version']);
    const version = stdout.trim();
    try {
      await listJobs({ limit: 1 });
      return { ok: true, version, loggedIn: true };
    } catch (err) {
      return { ok: true, version, loggedIn: false, reason: err.message };
    }
  } catch (err) {
    return { ok: false, reason: err.message };
  }
}
