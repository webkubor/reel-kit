/** 手写的极小 router —— 后端只有十来个端点，装 express 不值当 */

export function json(res, data, statusCode = 200) {
  const body = JSON.stringify(data);
  res.writeHead(statusCode, {
    'content-type': 'application/json; charset=utf-8',
    'content-length': Buffer.byteLength(body),
  });
  res.end(body);
}

export function fail(res, err) {
  const statusCode = err?.statusCode ?? 500;
  // 500 才打栈：403/404 是预期内的拒绝，刷屏没意义
  if (statusCode >= 500) console.error('[studio]', err);
  json(res, { error: err?.message ?? '未知错误' }, statusCode);
}

export async function readJsonBody(req, limitBytes = 5 * 1024 * 1024) {
  const chunks = [];
  let size = 0;
  for await (const chunk of req) {
    size += chunk.length;
    if (size > limitBytes) {
      throw Object.assign(new Error('请求体过大'), { statusCode: 413 });
    }
    chunks.push(chunk);
  }
  if (!chunks.length) return {};
  try {
    return JSON.parse(Buffer.concat(chunks).toString('utf8'));
  } catch {
    throw Object.assign(new Error('请求体不是合法 JSON'), { statusCode: 400 });
  }
}

/**
 * 路由表：{ 'GET /cast': handler }
 * 支持 ':param' 占位，handler 收到 (ctx) = { req, res, params, query, body }
 *
 * 用法选项：
 *   'GET /path'                       默认,GET 没有 body
 *   'POST /path'                      自动按 JSON 解析 body
 *   'POST /path raw'                  不解析 body,handler 自己从 req 读 stream
 *                                     （用于 binary upload / form-data 等）
 */
export function createRouter(routes) {
  const compiled = Object.entries(routes).map(([key, handler]) => {
    const [method, pattern, ...flags] = key.split(' ');
    const segments = pattern.split('/').filter(Boolean);
    return { method, segments, flags, handler };
  });

  return async function handle(req, res, pathname, query) {
    const parts = pathname.split('/').filter(Boolean);

    for (const route of compiled) {
      if (route.method !== req.method) continue;
      if (route.segments.length !== parts.length) continue;

      const params = {};
      const matched = route.segments.every((seg, i) => {
        if (seg.startsWith(':')) {
          params[seg.slice(1)] = decodeURIComponent(parts[i]);
          return true;
        }
        return seg === parts[i];
      });
      if (!matched) continue;

      const hasBody = req.method === 'POST' || req.method === 'PUT' || req.method === 'PATCH';
      const isRaw = route.flags.includes('raw');
      const body = hasBody && !isRaw ? await readJsonBody(req) : {};
      await route.handler({ req, res, params, query, body });
      return true;
    }
    return false;
  };
}
