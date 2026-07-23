import assert from 'node:assert/strict'
import { test } from 'node:test'

const FRONTEND_URL = cleanBaseUrl(process.env.SYNCBOX_FRONTEND_URL ?? 'http://localhost:8081')
const API_URL = cleanBaseUrl(process.env.SYNCBOX_API_URL ?? `${FRONTEND_URL}/api/v1`)
const ADMIN_EMAIL = process.env.SYNCBOX_ADMIN_EMAIL ?? 'admin@syncbox.co'
const ADMIN_PASSWORD = process.env.SYNCBOX_ADMIN_PASSWORD ?? 'Cambiar.123!'

const REQUEST_TIMEOUT_MS = Number(process.env.SYNCBOX_TEST_TIMEOUT_MS ?? 15_000)

let adminAuthPromise

function cleanBaseUrl(value) {
  return String(value).replace(/\/+$/, '')
}

function frontendUrl(path) {
  return `${FRONTEND_URL}${path.startsWith('/') ? path : `/${path}`}`
}

function apiUrl(path) {
  return `${API_URL}${path.startsWith('/') ? path : `/${path}`}`
}

async function fetchWithTimeout(url, options = {}) {
  const controller = new AbortController()
  const timeout = setTimeout(() => controller.abort(), REQUEST_TIMEOUT_MS)
  try {
    return await fetch(url, { ...options, signal: controller.signal })
  } finally {
    clearTimeout(timeout)
  }
}

async function readResponse(res) {
  const contentType = res.headers.get('content-type') ?? ''
  if (contentType.includes('application/json')) {
    return res.json()
  }
  return res.text()
}

async function apiRequest(path, { token, method = 'GET', body, expectedStatus } = {}) {
  const headers = { Accept: 'application/json' }
  if (token) headers.Authorization = `Bearer ${token}`
  if (body !== undefined) headers['Content-Type'] = 'application/json'

  const res = await fetchWithTimeout(apiUrl(path), {
    method,
    headers,
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  const payload = await readResponse(res)

  if (expectedStatus !== undefined) {
    assert.equal(
      res.status,
      expectedStatus,
      `${method} ${path} should return ${expectedStatus}; got ${res.status}: ${JSON.stringify(payload)}`,
    )
  } else {
    assert.ok(
      res.ok,
      `${method} ${path} should be ok; got ${res.status}: ${JSON.stringify(payload)}`,
    )
  }

  return { res, payload }
}

async function loginAsAdmin() {
  if (!adminAuthPromise) {
    adminAuthPromise = apiRequest('/auth/login', {
      method: 'POST',
      body: { correo: ADMIN_EMAIL, password: ADMIN_PASSWORD },
    }).then(({ payload }) => {
      assert.equal(typeof payload.access_token, 'string')
      assert.ok(payload.access_token.length > 40, 'access token should look like a JWT')
      assert.equal(typeof payload.refresh_token, 'string')
      assert.equal(payload.user?.correo, ADMIN_EMAIL)
      assert.equal(payload.user?.rol, 'ADMINISTRADOR')
      assert.equal(payload.user?.activo, true)
      return payload
    })
  }
  return adminAuthPromise
}

async function discoverServedLogoAsset(entryScripts) {
  const pending = [...entryScripts]
  const visited = new Set()

  while (pending.length > 0) {
    const src = pending.shift()
    if (!src || visited.has(src)) continue
    visited.add(src)

    const res = await fetchWithTimeout(frontendUrl(src))
    assert.equal(res.status, 200, `script ${src} should be served`)
    assert.match(res.headers.get('content-type') ?? '', /(javascript|ecmascript)/)

    const js = await res.text()
    const pngMatch = js.match(/\/assets\/logo-syncbox-[^"']+\.png/)
    if (pngMatch) return pngMatch[0]

    const imports = [...js.matchAll(/(?:import\(|from)\s*["']\.\/([^"']+\.js)["']/g)]
      .map((match) => `/assets/${match[1]}`)
    pending.push(...imports)
  }

  assert.fail(`expected a served logo asset reference in JS chunks: ${[...visited].join(', ')}`)
}

test('frontend sirve la SPA, sus assets principales y el logo construido', async () => {
  const loginRes = await fetchWithTimeout(frontendUrl('/login?redirect=/'), {
    headers: { Accept: 'text/html' },
  })
  assert.equal(loginRes.status, 200)
  assert.match(loginRes.headers.get('content-type') ?? '', /text\/html/)

  const html = await loginRes.text()
  assert.match(html, /<div id="app"><\/div>/)

  const scripts = [...html.matchAll(/<script[^>]+src="([^"]+)"/g)].map((match) => match[1])
  assert.ok(scripts.length > 0, 'expected at least one bundled JS script')

  const logoPath = await discoverServedLogoAsset(scripts)
  const logoRes = await fetchWithTimeout(frontendUrl(logoPath))
  assert.equal(logoRes.status, 200)
  assert.match(logoRes.headers.get('content-type') ?? '', /image\/png/)
  assert.ok(Number(logoRes.headers.get('content-length') ?? 0) > 10_000, 'logo asset should not be empty')
})

test('autenticacion: protege /me, autentica admin y persiste idioma de usuario', async () => {
  await apiRequest('/me', { expectedStatus: 401 })

  const auth = await loginAsAdmin()
  const token = auth.access_token

  const { payload: me } = await apiRequest('/me', { token })
  assert.equal(me.id, auth.user.id)
  assert.equal(me.correo, ADMIN_EMAIL)
  assert.equal(me.rol, 'ADMINISTRADOR')
  assert.equal(me.password_hash, undefined, 'public user payload must not expose password hash')

  const originalLocale = me.idioma_preferido === 'en' ? 'en' : 'es'
  const alternateLocale = originalLocale === 'es' ? 'en' : 'es'

  try {
    const { payload: changed } = await apiRequest('/me/preferencias', {
      method: 'PATCH',
      token,
      body: { idioma: alternateLocale },
    })
    assert.equal(changed.idioma_preferido, alternateLocale)

    const { payload: reloaded } = await apiRequest('/me', { token })
    assert.equal(reloaded.idioma_preferido, alternateLocale)
  } finally {
    await apiRequest('/me/preferencias', {
      method: 'PATCH',
      token,
      body: { idioma: originalLocale },
    })
  }
})

test('reglas del arbol: ubicacion contiene laboratorio, laboratorio queda como hoja', async () => {
  const { access_token: token } = await loginAsAdmin()
  const suffix = `${Date.now()}-${Math.random().toString(16).slice(2)}`
  const createdNodeIds = []

  try {
    const { payload: root } = await apiRequest('/nodos', {
      method: 'POST',
      token,
      body: {
        tipo: 'UBICACION',
        nombre: `Test ubicacion ${suffix}`,
        slug: `test_ubicacion_${suffix}`,
      },
      expectedStatus: 201,
    })
    createdNodeIds.push(root.id)
    assert.equal(root.tipo, 'UBICACION')
    assert.equal(root.parent_id, undefined)

    const { payload: lab } = await apiRequest('/nodos', {
      method: 'POST',
      token,
      body: {
        tipo: 'LABORATORIO',
        parent_id: root.id,
        nombre: `Test laboratorio ${suffix}`,
        slug: `test_laboratorio_${suffix}`,
      },
      expectedStatus: 201,
    })
    createdNodeIds.push(lab.id)
    assert.equal(lab.tipo, 'LABORATORIO')
    assert.equal(lab.parent_id, root.id)

    await apiRequest('/nodos', {
      method: 'POST',
      token,
      body: {
        tipo: 'UBICACION',
        parent_id: lab.id,
        nombre: `Sububicacion invalida ${suffix}`,
        slug: `sububicacion_invalida_${suffix}`,
      },
      expectedStatus: 400,
    })

    await apiRequest('/nodos', {
      method: 'POST',
      token,
      body: {
        tipo: 'EQUIPO',
        parent_id: root.id,
        nombre: `Equipo nodo invalido ${suffix}`,
        slug: `equipo_nodo_invalido_${suffix}`,
      },
      expectedStatus: 400,
    })
  } finally {
    for (const id of createdNodeIds.reverse()) {
      await apiRequest(`/nodos/${id}`, {
        method: 'DELETE',
        token,
        body: { confirm: 'entiendo' },
      }).catch(() => {})
    }
  }
})
