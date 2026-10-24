// Every call to the server. The X-MM header is what the server asks of a
// request that changes something: a page of another origin cannot send it.
export class ApiError extends Error {}

async function call(method, path, body, init = {}) {
  const headers = { 'X-MM': '1', ...(init.headers || {}) };
  let payload = body;
  if (body !== undefined && !(body instanceof FormData) && !(body instanceof Blob)) {
    headers['Content-Type'] = 'application/json';
    payload = JSON.stringify(body);
  }
  let res;
  try {
    res = await fetch(path, { method, headers, body: payload, signal: init.signal });
  } catch (err) {
    if (err.name === 'AbortError') throw err;
    throw new ApiError('Cannot reach mm. Is it still running?');
  }
  const type = res.headers.get('Content-Type') || '';
  const data = type.includes('json') ? await res.json() : null;
  if (!res.ok) throw new ApiError((data && data.error) || `${res.status} ${res.statusText}`);
  return data;
}

export const get = (path, init) => call('GET', path, undefined, init);
export const post = (path, body, init) => call('POST', path, body === undefined ? {} : body, init);
export const patch = (path, body, init) => call('PATCH', path, body, init);
export const put = (path, body, init) => call('PUT', path, body, init);
export const del = (path, body, init) => call('DELETE', path, body, init);

export const query = (params) => {
  const q = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) if (v !== '' && v != null && v !== false) q.set(k, v === true ? '1' : v);
  const s = q.toString();
  return s ? `?${s}` : '';
};
