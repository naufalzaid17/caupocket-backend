/**
 * DAST probe suite untuk Caupocket (dijalankan terhadap API + web yang hidup).
 *
 *   DAST_JWT_SECRET=... node scripts/dast-probes.mjs [apiBase] [webBase]
 *
 * Default: http://localhost:8080/api/v1 dan http://localhost:4321.
 * Setiap probe mencetak [PASS]/[FAIL]/[INFO]; exit code 1 bila ada FAIL.
 *
 * Catatan kuota: membuat 3 user (register limit 5/menit) dan melakukan ≤9
 * login sebelum probe rate-limit (login limit 12/menit) agar suite tidak
 * mengunci dirinya sendiri.
 */
import crypto from 'node:crypto';

const API = process.argv[2] ?? 'http://localhost:8080/api/v1';
const WEB = process.argv[3] ?? 'http://localhost:4321';
const JWT_SECRET = process.env.DAST_JWT_SECRET ?? '';
if (!JWT_SECRET) {
  console.error('DAST_JWT_SECRET wajib di-set (untuk memforging token kadaluarsa/tanda tangan salah).');
  process.exit(2);
}

let pass = 0, fail = 0;
function report(ok, name, detail = '') {
  const tag = ok ? '[PASS]' : '[FAIL]';
  if (ok) pass++; else fail++;
  console.log(`${tag} ${name}${detail ? ' — ' + detail : ''}`);
}
function info(name, detail = '') {
  console.log(`[INFO] ${name}${detail ? ' — ' + detail : ''}`);
}

async function call(path, { method = 'GET', token, body, headers = {}, base = API, raw } = {}) {
  const h = { ...headers };
  if (body !== undefined && !raw) h['Content-Type'] = 'application/json';
  if (token) h.Authorization = `Bearer ${token}`;
  const res = await fetch(base + path, {
    method,
    headers: h,
    body: raw ?? (body !== undefined ? JSON.stringify(body) : undefined),
  });
  const text = await res.text();
  let json = null;
  try { json = JSON.parse(text); } catch { /* non-JSON */ }
  return { status: res.status, headers: res.headers, text, json };
}

function signJWT(payload, secret) {
  const b64 = (o) => Buffer.from(JSON.stringify(o)).toString('base64url');
  const head = b64({ alg: 'HS256', typ: 'JWT' });
  const body = b64(payload);
  const sig = crypto.createHmac('sha256', secret).update(`${head}.${body}`).digest('base64url');
  return `${head}.${body}.${sig}`;
}

const stamp = Date.now();
async function makeUser(label) {
  const email = `dast-${label}-${stamp}@example.com`;
  await call('/auth/register', { method: 'POST', body: { name: 'Dast', email, password: 'rahasia123' } });
  const login = await call('/auth/login', { method: 'POST', body: { email, password: 'rahasia123' } });
  return { email, token: login.json?.data?.token, userID: login.json?.data?.user?.id };
}

// ---------------------------------------------------------------------------
console.log(`\n== DAST Caupocket — ${new Date().toISOString()} ==\n`);

const main = await makeUser('main');
const idora = await makeUser('idora');
const idorb = await makeUser('idorb');

// A01: Akses tanpa autentikasi.
{
  const paths = ['/me', '/wallets', '/transactions', '/goals', '/habits/settings', '/habits/logs'];
  let all401 = true;
  for (const p of paths) {
    const r = await call(p);
    if (r.status !== 401 || r.json?.error_code !== 'UNAUTHORIZED') all401 = false;
  }
  report(all401, 'A01 endpoint terproteksi tanpa token → 401 semua', paths.length + ' endpoint');
}

// A02: Manipulasi JWT.
{
  const noneToken = `${Buffer.from(JSON.stringify({ alg: 'none', typ: 'JWT' })).toString('base64url')}.${Buffer.from(JSON.stringify({ sub: main.userID, exp: 9999999999 })).toString('base64url')}.`;
  const r1 = await call('/me', { token: noneToken });
  report(r1.status === 401, 'A02a JWT alg=none ditolak', `status ${r1.status}`);

  const badSig = signJWT({ sub: main.userID, exp: 9999999999 }, 'secret-salah');
  const r2 = await call('/me', { token: badSig });
  report(r2.status === 401, 'A02b JWT tanda tangan salah ditolak', `status ${r2.status}`);

  const expired = signJWT({ sub: main.userID, iat: 1, exp: 1 }, JWT_SECRET);
  const r3 = await call('/me', { token: expired });
  report(r3.status === 401, 'A02c JWT kadaluarsa ditolak', `status ${r3.status}`);

  const subBukanUUID = signJWT({ sub: 'admin', exp: 9999999999 }, JWT_SECRET);
  const r4 = await call('/me', { token: subBukanUUID });
  report(r4.status === 401, 'A02d JWT sub bukan UUID ditolak', `status ${r4.status}`);
}

// A03: SQL Injection.
{
  const payloads = ["' OR '1'='1' --", "admin@example.com'--", "x'; DROP TABLE users; --"];
  let safe = true;
  for (const p of payloads) {
    const r = await call('/auth/login', { method: 'POST', body: { email: p, password: p } });
    if (r.status !== 401 || r.text.includes('DROP') || r.status >= 500) safe = false;
  }
  report(safe, 'A03a SQLi pada login aman (parameterized)', '3 payload → 401 tanpa error DB');

  const r = await call("/goals/' OR '1'='1' --/fund", { method: 'POST', token: main.token, body: { wallet_id: 'x', amount: 1 } });
  report(r.status === 400 || r.status === 404, 'A03b SQLi pada path param goal ditolak', `status ${r.status}`);
}

// A04: IDOR — user B tidak boleh menyentuh goal milik user A.
{
  const goalA = await call('/goals', { method: 'POST', token: idora.token, body: { title: 'Goal A', target_amount: 1000000 } });
  const goalID = goalA.json?.data?.id;

  const wB = (await call('/wallets', { token: idorb.token })).json?.data?.[0]?.id;
  await call('/transactions', { method: 'POST', token: idorb.token, body: { wallet_id: wB, type: 'INCOME', amount: 100000, description: 'topup' } });
  const r = await call(`/goals/${goalID}/fund`, { method: 'POST', token: idorb.token, body: { wallet_id: wB, amount: 10000 } });
  const goalAfter = (await call('/goals', { token: idora.token })).json?.data?.find((g) => g.id === goalID);
  report(
    r.status === 404 && r.json?.error_code === 'GOAL_NOT_FOUND' && goalAfter?.current_amount === 0,
    'A04 IDOR fund goal milik user lain → 404 & saldo goal tak tersentuh',
    `status ${r.status}, current=${goalAfter?.current_amount}`,
  );
}

// A07: XSS tersimpan — payload harus ter-escape di HTML dashboard.
{
  const wallet = (await call('/wallets', { token: main.token })).json?.data?.[0]?.id;
  const payload = '<script>alert("xss")</script>';
  await call('/transactions', { method: 'POST', token: main.token, body: { wallet_id: wallet, type: 'INCOME', amount: 1000, description: payload } });

  const login = await fetch(`${WEB}/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/x-www-form-urlencoded', Origin: WEB },
    body: `email=${encodeURIComponent(main.email)}&password=rahasia123`,
    redirect: 'manual',
  });
  const cookie = login.headers.get('set-cookie')?.split(';')[0] ?? '';
  const html = await (await fetch(`${WEB}/dashboard`, { headers: { cookie } })).text();
  const escaped = html.includes('&lt;script&gt;') || html.includes('&lt;/script&gt;');
  const rawExec = html.includes('<script>alert("xss")</script>');
  report(escaped && !rawExec, 'A07 XSS tersimpan ter-escape di SSR HTML', escaped ? 'payload di-encode' : 'payload tak tampil');
}

// A05: CSRF Astro checkOrigin.
{
  const r = await fetch(`${WEB}/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/x-www-form-urlencoded', Origin: 'https://evil.example' },
    body: 'email=x@x.id&password=xxx',
  });
  report(r.status === 403, 'A05 CSRF POST cross-origin ditolak Astro (403)', `status ${r.status}`);
}

// Cookie flags (login valid lewat proxy web — bukan menghabiskan kuota API
// tambahan karena proxy memakai kredensial yang sama).
{
  const sc = await fetch(`${WEB}/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/x-www-form-urlencoded', Origin: WEB },
    body: `email=${encodeURIComponent(main.email)}&password=rahasia123`,
    redirect: 'manual',
  }).then((r) => r.headers.get('set-cookie') ?? '');
  report(
    /httponly/i.test(sc) && /secure/i.test(sc) && /samesite=lax/i.test(sc),
    'Cookie sesi httpOnly+secure+sameSite=lax', sc.split(';').slice(1, 4).join(';').trim());
}

// Validasi input uang.
{
  const wallet = (await call('/wallets', { token: main.token })).json?.data?.[0]?.id;
  const rNeg = await call('/transactions', { method: 'POST', token: main.token, body: { wallet_id: wallet, type: 'EXPENSE', amount: -500 } });
  const rZero = await call('/transactions', { method: 'POST', token: main.token, body: { wallet_id: wallet, type: 'EXPENSE', amount: 0 } });
  report(rNeg.status === 400 && rZero.status === 400, 'Validasi nominal negatif/nol → 400', `neg=${rNeg.status} zero=${rZero.status}`);
}

// Upload: oversize + file palsu.
{
  const big = new Uint8Array(6 * 1024 * 1024);
  const fd1 = new FormData();
  fd1.append('file', new File([big], 'big.png', { type: 'image/png' }));
  const r1 = await fetch(`${API}/ocr/receipt`, { method: 'POST', headers: { Authorization: `Bearer ${main.token}` }, body: fd1 });
  report(r1.status === 413, 'Upload OCR > 5MB → 413', `status ${r1.status}`);

  const fd2 = new FormData();
  fd2.append('file', new File([new TextEncoder().encode('bukan gambar')], 'fake.png', { type: 'image/png' }));
  const r2 = await fetch(`${API}/ocr/receipt`, { method: 'POST', headers: { Authorization: `Bearer ${main.token}` }, body: fd2 });
  report(r2.status === 400, 'Upload OCR file non-gambar (magic bytes) → 400', `status ${r2.status}`);
}

// Enumerasi user pada login: pesan harus seragam.
{
  const rUnknown = await call('/auth/login', { method: 'POST', body: { email: `ghost-${stamp}@x.id`, password: 'rahasia123' } });
  const rWrong = await call('/auth/login', { method: 'POST', body: { email: main.email, password: 'password-salah' } });
  report(
    rUnknown.json?.message === rWrong.json?.message && rUnknown.json?.error_code === rWrong.json?.error_code,
    'Enumerasi user login — pesan email-salah vs password-salah identik');
}

// Security headers.
{
  const r = await fetch(`${API}/auth/login`, { method: 'OPTIONS' });
  const h = r.headers;
  report(
    h.get('x-content-type-options') === 'nosniff' && h.get('x-frame-options') === 'DENY' && !!h.get('referrer-policy'),
    'Security headers API (nosniff/DENY/referrer)', `${h.get('x-content-type-options')}/${h.get('x-frame-options')}/${h.get('referrer-policy')}`);

  const page = await fetch(`${WEB}/login`);
  report(
    page.headers.get('x-content-type-options') === 'nosniff' && page.headers.get('x-frame-options') === 'DENY',
    'Security headers web (middleware Astro)', `${page.headers.get('x-content-type-options')}/${page.headers.get('x-frame-options')}`);
}

// Rate limit login (default 12/menit per IP) — diletakkan TERAKHIR karena
// sengaja menghabiskan kuota.
{
  let got429 = false, first429At = 0;
  for (let i = 1; i <= 12; i++) {
    const r = await call('/auth/login', { method: 'POST', body: { email: `brute-${stamp}@x.id`, password: 'salah' } });
    if (r.status === 429) { got429 = true; first429At = i; break; }
  }
  report(got429, 'Rate limit brute-force login → 429 setelah kuota', got429 ? `429 pada percobaan #${first429At}` : 'tidak pernah 429');
}

// Swagger exposure (informatif — dikendalikan SWAGGER_ENABLED).
{
  const r = await fetch(new URL('/swagger/index.html', API.replace('/api/v1', '')).href);
  info('Swagger UI', `status ${r.status} (set SWAGGER_ENABLED=false di production)`);
}

console.log(`\n== Hasil: ${pass} PASS, ${fail} FAIL ==\n`);
process.exit(fail > 0 ? 1 : 0);
