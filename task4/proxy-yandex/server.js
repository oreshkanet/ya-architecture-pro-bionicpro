const express = require('express');
const axios = require('axios');
const querystring = require('querystring');
const url = require('url');
const morgan = require('morgan');
const { exportJWK, generateKeyPair, SignJWT } = require('jose');

const app = express();
const PORT = process.env.PORT || 3000;

const ISSUER = process.env.ISSUER_BASE_URL || 'http://proxy-yandex:8002';

let idTokenPrivateKey;
let idTokenPublicJwk;
const JWK_KID = 'proxy-yandex-rs256';

// Nonce для id_token: Keycloak шлёт nonce в /authorize, но не в /token.
// Сохраняем по state и используем при следующем /token.
const NONCE_TTL_MS = 10 * 60 * 1000; // 10 мин
const nonceByState = new Map();
let lastNonce = null; // fallback при одном потоке входа

function storeNonce(state, nonce) {
  if (!state || !nonce) return;
  nonceByState.set(state, { nonce, expires: Date.now() + NONCE_TTL_MS });
  lastNonce = nonce;
  for (const [s, v] of nonceByState.entries()) {
    if (v.expires < Date.now()) nonceByState.delete(s);
  }
}

function takeNonce(stateOrFromBody) {
  if (stateOrFromBody) {
    const entry = nonceByState.get(stateOrFromBody);
    if (entry && entry.expires >= Date.now()) {
      nonceByState.delete(stateOrFromBody);
      return entry.nonce;
    }
  }
  const n = lastNonce;
  lastNonce = null;
  return n || null;
}

app.use(morgan('combined'));
app.use(express.urlencoded({ extended: true }));

app.use((req, res, next) => {
  const start = Date.now();
  const ts = new Date().toISOString();
  const query = Object.keys(req.query).length ? '?' + new URLSearchParams(req.query).toString() : '';
  let logLine = `[PROXY] ${ts} ${req.method} ${req.path}${query}`;
  if (req.method !== 'GET' && req.method !== 'HEAD' && req.body && Object.keys(req.body).length) {
    const bodyKeys = Object.keys(req.body).join(', ');
    logLine += ` | body keys: [${bodyKeys}]`;
  }
  console.log(logLine);

  res.on('finish', () => {
    const duration = Date.now() - start;
    console.log(`[PROXY] ${req.method} ${req.path} -> ${res.statusCode} ${res.getHeader('content-length') || 0}B ${duration}ms`);
  });
  next();
});

app.get('/authorize', async (req, res) => {
  try {
    const parsedUrl = url.parse(req.url, true);
    const params = { ...parsedUrl.query };

    // Сохраняем nonce по state
    if (params.state && params.nonce) {
      storeNonce(params.state, params.nonce);
    }

    // Удаляем "openid" из scope
    if (params.scope) {
      const scopes = params.scope.split(' ').filter(s => s !== 'openid' && s !== 'openid+');
      params.scope = scopes.join(' ');
      console.log(`[PROXY] Modified scope: "${params.scope}"`);
    }

    const targetUrl = `https://oauth.yandex.ru/authorize?${querystring.stringify(params)}`;
    console.log(`[PROXY] Redirecting to: ${targetUrl}`);

    // Перенаправляем пользователя напрямую к Яндексу
    res.redirect(targetUrl);
  } catch (error) {
    console.error('[PROXY] Error in /authorize:', error);
    res.status(500).json({ error: 'Proxy authorization error', details: error.message });
  }
});

app.post('/token', async (req, res) => {
  try {
    const bodyParams = { ...req.body };
    if (!bodyParams.code) {
      console.error('[PROXY] /token: no code in body, keys:', Object.keys(req.body || {}));
      return res.status(400).json({ error: 'missing_code', message: 'Authorization code is required' });
    }

    // Приводим redirect_uri к виду, который видел браузер (localhost), иначе Yandex вернёт invalid_grant
    if (bodyParams.redirect_uri && bodyParams.redirect_uri.includes('keycloak:8080')) {
      bodyParams.redirect_uri = bodyParams.redirect_uri.replace('keycloak:8080', 'localhost:8080');
      console.log('[PROXY] /token: normalized redirect_uri to localhost:8080');
    }

    bodyParams.grant_type = bodyParams.grant_type || 'authorization_code';
    const body = querystring.stringify(bodyParams);

    const response = await axios.post('https://oauth.yandex.ru/token', body, {
      headers: {
        'Content-Type': 'application/x-www-form-urlencoded',
        'User-Agent': req.headers['user-agent'] || 'proxy-yandex'
      },
      maxRedirects: 0,
      validateStatus: () => true,
      responseType: 'text'
    });

    if (response.status < 200 || response.status >= 300) {
      console.error('[PROXY] /token: Yandex error', response.status, response.data);
      try {
        const errBody = typeof response.data === 'string' ? querystring.parse(response.data) : response.data;
        return res.status(response.status).set('Content-Type', 'application/json').json(errBody);
      } catch (_) {
        return res.status(response.status).type('application/json').send(JSON.stringify({ error: String(response.data) }));
      }
    }

    // Нормализация ответа: Keycloak ожидает JSON-объект с access_token (не строку)
    let tokenData = response.data;
    if (typeof tokenData === 'string') {
      try {
        tokenData = JSON.parse(tokenData);
      } catch (_) {
        tokenData = querystring.parse(tokenData);
      }
    }
    const accessToken = tokenData.access_token;
    if (!accessToken) {
      console.error('[PROXY] /token: Yandex response missing access_token');
      return res.status(502).set('Content-Type', 'application/json').json(tokenData);
    }
    console.log('[PROXY] /token 200 access_token received');

    const keycloakTokenResponse = {
      access_token: accessToken,
      token_type: tokenData.token_type || 'bearer',
      expires_in: tokenData.expires_in != null ? Number(tokenData.expires_in) : 31536000
    };
    if (tokenData.refresh_token) keycloakTokenResponse.refresh_token = tokenData.refresh_token;
    if (tokenData.scope) keycloakTokenResponse.scope = tokenData.scope;

    try {
      const userInfoRes = await axios.get('https://login.yandex.ru/info?format=json', {
        headers: {
          'Authorization': `OAuth ${accessToken}`,
          'User-Agent': req.headers['user-agent'] || 'proxy-yandex'
        }
      });
      const claims = userInfoRes.data || {};
      const sub = String(claims.id || claims.client_id || '');
      const nonce = bodyParams.nonce || takeNonce(bodyParams.state);
      const oidcClaims = {
        sub,
        email: claims.default_email || claims.email,
        email_verified: !!(claims.default_email || claims.email),
        name: claims.display_name || claims.real_name || claims.login,
        preferred_username: claims.login || claims.display_name || sub,
        given_name: claims.first_name,
        family_name: claims.last_name,
        default_email: claims.default_email,
        email: claims.default_email,
        first_name: claims.first_name,
        last_name: claims.last_name,
        id: claims.id,
        login: claims.login,
        display_name: claims.display_name,
        real_name: claims.real_name
      };
      if (nonce) oidcClaims.nonce = nonce;
      const clientId = bodyParams.client_id || '';
      const idToken = await new SignJWT(oidcClaims)
        .setProtectedHeader({ alg: 'RS256', kid: JWK_KID, typ: 'JWT' })
        .setIssuer(ISSUER)
        .setAudience(clientId)
        .setExpirationTime('1h')
        .setIssuedAt()
        .sign(idTokenPrivateKey);
      keycloakTokenResponse.id_token = idToken;
      console.log('[PROXY] /token id_token issued for oidcClaims', oidcClaims);
    } catch (idErr) {
      console.error('[PROXY] /token id_token build failed:', idErr.message);
    }
    
    res.status(200).set('Content-Type', 'application/json').json(keycloakTokenResponse);
  } catch (error) {
    const errData = error.response?.data || { error: error.message };
    console.error('[PROXY] /token exception:', errData);
    res.status(error.response?.status || 500).json(errData);
  }
});

app.get('/info', async (req, res) => {
  try {
    const accessToken = req.headers.authorization?.replace('Bearer ', '') || 
                        req.query.access_token;

    if (!accessToken) {
      console.error('[PROXY] Access token required');
      return res.status(401).json({ error: 'Access token required' });
    }

    const response = await axios.get('https://login.yandex.ru/info?format=json', {
      headers: {
        'Authorization': `OAuth ${accessToken}`,
        'User-Agent': req.headers['user-agent'] || 'proxy-yandex'
      }
    });

    const userInfo = { ...response.data, 
      email: response.data.default_email,
      sub: String(response.data.id || response.data.client_id || '') 
    };

    console.log('[PROXY] /info 200 user info fetched:', userInfo);
    res.json(userInfo);
  } catch (error) {
    console.error('[PROXY] /info error:', error.response?.data || error.message);
    res.status(error.response?.status || 500).json(error.response?.data || { error: 'Failed to fetch user info' });
  }
});

app.get('/.well-known/jwks.json', (req, res) => {
  if (!idTokenPublicJwk) {
    return res.status(503).set('Content-Type', 'application/json').json({ error: 'JWKS not ready' });
  }
  res.set('Content-Type', 'application/json').json({ keys: [idTokenPublicJwk] });
});

// Health check
app.get('/health', (req, res) => {
  res.json({ status: 'ok', service: 'proxy-yandex' });
});

app.all('*', (req, res) => {
  console.log(`[PROXY] Unhandled: ${req.method} ${req.originalUrl}`);
  res.status(404).json({ error: 'Not Found', path: req.path, method: req.method });
});

async function start() {
  const { publicKey, privateKey } = await generateKeyPair('RS256');
  idTokenPrivateKey = privateKey;
  const jwk = await exportJWK(publicKey);
  idTokenPublicJwk = {
    ...jwk,
    alg: 'RS256',
    kid: JWK_KID,
    use: 'sig'
  };
  app.listen(PORT, () => {
    console.log(`[PROXY] Yandex OAuth Proxy running on port ${PORT}`);
  });
}

start().catch((err) => {
  console.error('[PROXY] Startup failed:', err);
  process.exit(1);
});