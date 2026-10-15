const fs = require('fs');
const path = require('path');
const https = require('https');
const { execSync } = require('child_process');

const token = execSync('gcloud auth print-access-token', { encoding: 'utf8' }).trim();
const projectId = 'whisperledger-94715';
const outDir = path.resolve(__dirname, '../firebase-export');

if (!fs.existsSync(outDir)) {
  fs.mkdirSync(outDir, { recursive: true });
}

function request(method, pathStr, body) {
  return new Promise((resolve, reject) => {
    const postData = body ? JSON.stringify(body) : null;
    const req = https.request({
      hostname: 'firestore.googleapis.com',
      path: `/v1/projects/${projectId}/databases/(default)/documents/${pathStr}`,
      method,
      headers: {
        Authorization: `Bearer ${token}`,
        'Content-Type': 'application/json',
        ...(postData ? { 'Content-Length': Buffer.byteLength(postData) } : {})
      }
    }, (res) => {
      let data = '';
      res.on('data', chunk => data += chunk);
      res.on('end', () => {
        try {
          resolve(JSON.parse(data));
        } catch (e) {
          resolve({ error: data });
        }
      });
    });
    req.on('error', reject);
    if (postData) req.write(postData);
    req.end();
  });
}

const get = (p) => request('GET', p);
const post = (p, b) => request('POST', p, b);

function parseFirestoreValue(val) {
  if (!val) return null;
  if ('stringValue' in val) return val.stringValue;
  if ('integerValue' in val) return parseInt(val.integerValue, 10);
  if ('doubleValue' in val) return parseFloat(val.doubleValue);
  if ('booleanValue' in val) return val.booleanValue;
  if ('timestampValue' in val) return val.timestampValue;
  if ('nullValue' in val) return null;
  if ('mapValue' in val) {
    const obj = {};
    for (const [k, v] of Object.entries(val.mapValue.fields || {})) {
      obj[k] = parseFirestoreValue(v);
    }
    return obj;
  }
  if ('arrayValue' in val) {
    return (val.arrayValue.values || []).map(parseFirestoreValue);
  }
  return val;
}

function parseDoc(doc) {
  const parts = doc.name.split('/databases/(default)/documents/')[1].split('/');
  const docId = parts[parts.length - 1];
  const obj = { id: docId, _docPath: doc.name };
  for (const [k, v] of Object.entries(doc.fields || {})) {
    obj[k] = parseFirestoreValue(v);
  }
  return obj;
}

async function exportAll() {
  console.log(`Starting Firestore export for project: ${projectId}`);

  // 1. Export Users
  console.log('Fetching users...');
  const usersRes = await get('users');
  const rawUsers = (usersRes.documents || []).map(parseDoc);
  const users = rawUsers.map(u => ({
    id: u.uid || u.id,
    email: u.email || '',
    displayName: u.name || u.displayName || u.username || (u.email ? u.email.split('@')[0] : 'User'),
    photoURL: u.photoURL || u.avatar_url || '',
    role: u.role === 'platform_admin' ? 'platform_admin' : 'user',
    createdAt: u.createdAt || new Date().toISOString()
  })).filter(u => u.email && u.email.includes('@'));

  fs.writeFileSync(path.join(outDir, 'users.json'), JSON.stringify(users, null, 2));
  console.log(`✓ Exported ${users.length} users to users.json`);

  // 2. Export Households
  console.log('Fetching households...');
  const hhRes = await get('households');
  const rawHouseholds = (hhRes.documents || []).map(parseDoc);
  const households = rawHouseholds.map(h => ({
    id: h.id,
    name: h.name || 'Personal Space',
    currency: h.currency || 'INR',
    inviteCode: h.inviteCode || h.id,
    createdBy: h.createdBy || '',
    members: Array.isArray(h.memberUids) ? h.memberUids : (h.members ? Object.keys(h.members) : []),
    createdAt: h.createdAt || new Date().toISOString()
  }));

  fs.writeFileSync(path.join(outDir, 'households.json'), JSON.stringify(households, null, 2));
  console.log(`✓ Exported ${households.length} households to households.json`);

  // 3. Export Expenses (from all users and households)
  console.log('Fetching expenses across user and household subcollections...');
  const allExpenses = [];

  for (const u of rawUsers) {
    const uid = u.uid || u.id;
    const expRes = await get(`users/${uid}/expenses`);
    if (expRes.documents) {
      for (const d of expRes.documents) {
        const parsed = parseDoc(d);
        allExpenses.push({
          id: parsed.id,
          userId: uid,
          householdId: null,
          amount: typeof parsed.amount === 'number' ? parsed.amount : parseFloat(parsed.amount || '0'),
          category: parsed.categoryId || parsed.category || 'other',
          merchant: parsed.merchant || parsed.note || 'Expense',
          description: parsed.note || parsed.description || '',
          date: parsed.date || new Date().toISOString().split('T')[0],
          source: parsed.source || 'manual',
          isSplit: false,
          createdAt: parsed.createdAt || new Date().toISOString()
        });
      }
    }
  }

  for (const h of rawHouseholds) {
    const expRes = await get(`households/${h.id}/expenses`);
    if (expRes.documents) {
      for (const d of expRes.documents) {
        const parsed = parseDoc(d);
        allExpenses.push({
          id: parsed.id,
          userId: parsed.userId || parsed.paidBy || h.createdBy || '',
          householdId: h.id,
          amount: typeof parsed.amount === 'number' ? parsed.amount : parseFloat(parsed.amount || '0'),
          category: parsed.categoryId || parsed.category || 'other',
          merchant: parsed.merchant || parsed.title || 'Household Expense',
          description: parsed.description || parsed.note || '',
          date: parsed.date || new Date().toISOString().split('T')[0],
          source: 'household',
          isSplit: true,
          createdAt: parsed.createdAt || new Date().toISOString()
        });
      }
    }
  }

  fs.writeFileSync(path.join(outDir, 'expenses.json'), JSON.stringify(allExpenses, null, 2));
  console.log(`✓ Exported ${allExpenses.length} expenses to expenses.json`);

  // 4. Receivables (legacy Firebase did not store receivables; init as empty)
  fs.writeFileSync(path.join(outDir, 'receivables.json'), JSON.stringify([], null, 2));
  console.log(`✓ Exported 0 receivables to receivables.json`);

  // 5. Export Support Cases
  console.log('Fetching support cases...');
  const casesRes = await get('support_cases');
  const rawCases = (casesRes.documents || []).map(parseDoc);
  const supportCases = rawCases.map(c => ({
    id: c.id,
    userId: c.userId || '',
    subject: c.subject || 'Support Request',
    message: c.message || c.description || `Source: ${c.source || 'app'} - Email: ${c.userEmail || ''}`,
    status: c.status || 'received',
    createdAt: c.createdAt || new Date().toISOString()
  }));

  fs.writeFileSync(path.join(outDir, 'support_cases.json'), JSON.stringify(supportCases, null, 2));
  console.log(`✓ Exported ${supportCases.length} support cases to support_cases.json`);

  console.log('\n🎉 Firebase export completed successfully into:', outDir);
}

exportAll().catch(err => {
  console.error('Export failed:', err);
  process.exit(1);
});
