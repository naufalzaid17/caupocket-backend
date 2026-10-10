/**
 * Seed data demo untuk Caupocket (dijalankan terhadap API yang sedang aktif).
 *
 *   node scripts/seed-demo.mjs [baseUrl] [email] [password]
 *
 * Default: http://localhost:8080/api/v1 demo@caupocket.id demo1234
 */
const BASE = process.argv[2] ?? 'http://localhost:8080/api/v1';
const EMAIL = process.argv[3] ?? 'demo@caupocket.id';
const PASSWORD = process.argv[4] ?? 'demo1234';

async function call(path, { token, method = 'POST', body } = {}) {
  const res = await fetch(`${BASE}${path}`, {
    method,
    headers: {
      'Content-Type': 'application/json',
      ...(token && { Authorization: `Bearer ${token}` }),
    },
    body: body ? JSON.stringify(body) : undefined,
  });
  const json = await res.json();
  if (!json.success) throw new Error(`${path}: ${res.status} ${json.message}`);
  return json.data;
}

async function register(email) {
  return call('/auth/register', {
    body: { name: 'Demo Caupocket', email, password: PASSWORD },
  });
}

let finalEmail = EMAIL;
try {
  await register(finalEmail);
} catch {
  // Email sudah terdaftar → pakai varian unik.
  finalEmail = EMAIL.replace('@', `+${Date.now()}@`);
  await register(finalEmail);
}
const login = await call('/auth/login', { body: { email: finalEmail, password: PASSWORD } });
const token = login.token;
const wallets = await call('/wallets', { token, method: 'GET' });
const wallet = wallets[0];
const categories = await call('/categories', { token, method: 'GET' });
const cat = (name) => categories.find((c) => c.name === name)?.id;

await call('/transactions', {
  token,
  body: {
    wallet_id: wallet.id,
    category_id: cat('Gaji'),
    type: 'INCOME',
    amount: 8500000,
    description: 'Gaji bulan Oktober',
    trx_date: '2026-10-01T09:00:00Z',
  },
});
await call('/transactions', {
  token,
  body: {
    wallet_id: wallet.id,
    category_id: cat('Makanan & Minuman'),
    type: 'EXPENSE',
    amount: 87500,
    description: 'Nasi goreng + es teh',
    trx_date: '2026-10-01T12:30:00Z',
  },
});
await call('/transactions', {
  token,
  body: {
    wallet_id: wallet.id,
    category_id: cat('Transportasi'),
    type: 'EXPENSE',
    amount: 150000,
    description: 'Ojek online ke kantor',
    trx_date: '2026-10-01T08:15:00Z',
  },
});

const bali = await call('/goals', {
  token,
  body: { title: 'Liburan ke Bali', target_amount: 5000000, target_date: '2026-12-20' },
});
await call(`/goals/${bali.id}/fund`, {
  token,
  body: { wallet_id: wallet.id, amount: 2000000 },
});

const lombok = await call('/goals', {
  token,
  body: { title: 'Weekend Lombok', target_amount: 3500000, target_date: '2027-03-14' },
});
await call(`/goals/${lombok.id}/fund`, {
  token,
  body: { wallet_id: wallet.id, amount: 500000 },
});

console.log('Seed selesai ✔');
console.log(`  Login: ${finalEmail} / ${PASSWORD}`);
console.log(`  Saldo dompet: ${wallet ? 'muat ulang /wallets' : '-'}`);
