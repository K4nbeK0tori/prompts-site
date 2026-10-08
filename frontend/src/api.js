const BASE = ''

async function request(path, options = {}) {
  const res = await fetch(BASE + path, {
    credentials: 'same-origin',
    headers: { 'Content-Type': 'application/json' },
    ...options,
  })
  let payload = null
  try {
    payload = await res.json()
  } catch {
    payload = null
  }
  if (!res.ok || !payload || payload.ok === false) {
    const err = new Error((payload && payload.error) || `请求失败（HTTP ${res.status}）`)
    err.status = res.status
    throw err
  }
  return payload.data
}

function qs(params) {
  const sp = new URLSearchParams()
  Object.entries(params).forEach(([key, value]) => {
    if (value === undefined || value === null || value === '') return
    if (Array.isArray(value)) {
      value.forEach((v) => v !== '' && v !== null && sp.append(key, v))
      return
    }
    sp.append(key, String(value))
  })
  return sp.toString()
}

export const api = {
  me: () => request('/api/me'),
  stats: () => request('/api/stats'),
  tags: () => request('/api/tags'),
  list: (params) => request(`/api/prompts?${qs(params)}`),
  get: (id) => request(`/api/prompts/${id}`),
  login: (password) =>
    request('/api/login', { method: 'POST', body: JSON.stringify({ password }) }),
  logout: () => request('/api/logout', { method: 'POST', body: '{}' }),
  create: (body) => request('/api/prompts', { method: 'POST', body: JSON.stringify(body) }),
  update: (id, body) =>
    request(`/api/prompts/${id}`, { method: 'PUT', body: JSON.stringify(body) }),
  remove: (id) => request(`/api/prompts/${id}`, { method: 'DELETE' }),
  favorite: (id, favorite) =>
    request(`/api/prompts/${id}/favorite`, {
      method: 'POST',
      body: JSON.stringify({ favorite }),
    }),
}
