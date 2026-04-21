'use strict';

const API = '/api/rewrites';

// ── Validation ────────────────────────────────────────────────────────────────

function isValidFQDN(v) {
  return /^([a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$/.test(v);
}

function isValidTarget(v) {
  return v.endsWith('.svc.cluster.local') && isValidFQDN(v);
}

// ── API helpers ───────────────────────────────────────────────────────────────

async function apiFetch(path, options = {}) {
  const res = await fetch(path, {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  });
  if (res.status === 204) return null;
  const data = await res.json();
  if (!res.ok) throw new Error(data.error || `HTTP ${res.status}`);
  return data;
}

const fetchAll   = ()         => apiFetch(API);
const createRule = (body)     => apiFetch(API, { method: 'POST',   body: JSON.stringify(body) });
const updateRule = (name, b)  => apiFetch(`${API}/${name}`, { method: 'PUT', body: JSON.stringify(b) });
const deleteRule = (name)     => apiFetch(`${API}/${name}`, { method: 'DELETE' });

// ── Toast ─────────────────────────────────────────────────────────────────────

let toastTimer;
function showToast(msg, isError = false) {
  const el = document.getElementById('toast');
  el.textContent = msg;
  el.classList.toggle('error', isError);
  el.classList.remove('hidden');
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => el.classList.add('hidden'), 3000);
}

// ── Polling ───────────────────────────────────────────────────────────────────

// After a write operation, poll until no rule is pending (or timeout).
// While pending rules exist, re-render the table each tick so the user
// sees the badge flip from "pending" → "applied" in real time.
const POLL_INTERVAL_MS = 1500;
const POLL_TIMEOUT_MS  = 30000;

let pollTimer = null;

function stopPolling() {
  if (pollTimer) { clearTimeout(pollTimer); pollTimer = null; }
}

function startPolling() {
  stopPolling();
  const deadline = Date.now() + POLL_TIMEOUT_MS;

  async function tick() {
    try {
      const rules = await fetchAll();
      renderTable(rules);
      const hasPending = rules.some(r => !r.applied && !r.error);
      if (hasPending && Date.now() < deadline) {
        pollTimer = setTimeout(tick, POLL_INTERVAL_MS);
      } else {
        pollTimer = null;
      }
    } catch {
      pollTimer = null;
    }
  }

  pollTimer = setTimeout(tick, POLL_INTERVAL_MS);
}

// ── Table rendering ───────────────────────────────────────────────────────────

function renderTable(rules) {
  const tbody = document.getElementById('rules-body');
  tbody.innerHTML = '';

  if (!rules || rules.length === 0) {
    tbody.innerHTML = '<tr><td colspan="5" class="empty">No rules yet. Add one above.</td></tr>';
    return;
  }

  rules.forEach(rule => {
    const tr = document.createElement('tr');

    const statusBadge = rule.enabled
      ? '<span class="badge badge-on">enabled</span>'
      : '<span class="badge badge-off">disabled</span>';

    let appliedBadge;
    if (rule.error) {
      appliedBadge = `<span class="badge badge-err" title="${escHtml(rule.error)}">error</span>`;
    } else if (rule.applied) {
      appliedBadge = '<span class="badge badge-ok">applied</span>';
    } else {
      appliedBadge = '<span class="badge badge-pending">⟳ pending</span>';
    }

    tr.innerHTML = `
      <td><code>${escHtml(rule.host)}</code></td>
      <td><code>${escHtml(rule.target)}</code></td>
      <td>${statusBadge}</td>
      <td>${appliedBadge}</td>
      <td>
        <div class="actions">
          <button class="btn btn-sm" data-action="toggle" data-name="${escHtml(rule.name)}"
                  data-enabled="${rule.enabled}" data-host="${escHtml(rule.host)}"
                  data-target="${escHtml(rule.target)}">
            ${rule.enabled ? 'Disable' : 'Enable'}
          </button>
          <button class="btn btn-sm" data-action="edit" data-name="${escHtml(rule.name)}"
                  data-host="${escHtml(rule.host)}" data-target="${escHtml(rule.target)}"
                  data-enabled="${rule.enabled}">Edit</button>
          <button class="btn btn-sm btn-danger" data-action="delete" data-name="${escHtml(rule.name)}">Delete</button>
        </div>
      </td>`;
    tbody.appendChild(tr);
  });
}

function escHtml(str) {
  return String(str)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}

// ── Load & refresh ────────────────────────────────────────────────────────────

async function loadRules() {
  try {
    const rules = await fetchAll();
    renderTable(rules);
  } catch (e) {
    showToast('Failed to load rules: ' + e.message, true);
  }
}

// ── Modal ─────────────────────────────────────────────────────────────────────

const overlay   = document.getElementById('modal-overlay');
const form      = document.getElementById('rule-form');
const fName     = document.getElementById('field-name');
const fNameOrig = document.getElementById('field-name-original');
const fHost     = document.getElementById('field-host');
const fTarget   = document.getElementById('field-target');
const fEnabled  = document.getElementById('field-enabled');
const modalTitle = document.getElementById('modal-title');

function openModal(rule = null) {
  form.reset();
  clearErrors();
  if (rule) {
    modalTitle.textContent = 'Edit rule';
    fName.value     = rule.name;
    fName.disabled  = true;
    fNameOrig.value = rule.name;
    fHost.value     = rule.host;
    fTarget.value   = rule.target;
    fEnabled.checked = rule.enabled;
  } else {
    modalTitle.textContent = 'Add rule';
    fName.disabled  = false;
    fNameOrig.value = '';
  }
  overlay.classList.remove('hidden');
  (rule ? fHost : fName).focus();
}

function closeModal() {
  overlay.classList.add('hidden');
}

function clearErrors() {
  ['name', 'host', 'target'].forEach(f => {
    document.getElementById(`err-${f}`).textContent = '';
    document.getElementById(`field-${f}`).classList.remove('invalid');
  });
}

function setError(field, msg) {
  document.getElementById(`err-${field}`).textContent = msg;
  document.getElementById(`field-${field}`).classList.add('invalid');
}

function validateForm() {
  clearErrors();
  let ok = true;
  const name   = fName.value.trim();
  const host   = fHost.value.trim();
  const target = fTarget.value.trim();

  if (!fName.disabled) {
    if (!name) { setError('name', 'Required'); ok = false; }
    else if (!/^[a-z0-9][a-z0-9\-]*$/.test(name)) { setError('name', 'Lowercase alphanumeric and hyphens only'); ok = false; }
  }
  if (!host) { setError('host', 'Required'); ok = false; }
  else if (!isValidFQDN(host)) { setError('host', 'Must be a valid FQDN'); ok = false; }

  if (!target) { setError('target', 'Required'); ok = false; }
  else if (!isValidTarget(target)) { setError('target', 'Must end with .svc.cluster.local'); ok = false; }

  return ok;
}

// ── Event wiring ──────────────────────────────────────────────────────────────

document.getElementById('btn-add').addEventListener('click', () => openModal());
document.getElementById('btn-cancel').addEventListener('click', closeModal);
overlay.addEventListener('click', e => { if (e.target === overlay) closeModal(); });

form.addEventListener('submit', async e => {
  e.preventDefault();
  if (!validateForm()) return;

  const isEdit = !!fNameOrig.value;
  const body = {
    name:    fName.value.trim(),
    host:    fHost.value.trim(),
    target:  fTarget.value.trim(),
    enabled: fEnabled.checked,
  };

  try {
    if (isEdit) {
      await updateRule(fNameOrig.value, body);
      showToast('Rule updated');
    } else {
      await createRule(body);
      showToast('Rule created');
    }
    closeModal();
    await loadRules();
    startPolling();
  } catch (err) {
    showToast(err.message, true);
  }
});

// Table action delegation
document.getElementById('rules-body').addEventListener('click', async e => {
  const btn = e.target.closest('[data-action]');
  if (!btn) return;

  const action = btn.dataset.action;
  const name   = btn.dataset.name;

  if (action === 'delete') {
    if (!confirm(`Delete rule "${name}"?`)) return;
    try {
      await deleteRule(name);
      showToast('Rule deleted');
      await loadRules();
      startPolling();
    } catch (err) {
      showToast(err.message, true);
    }
  }

  if (action === 'edit') {
    openModal({
      name:    btn.dataset.name,
      host:    btn.dataset.host,
      target:  btn.dataset.target,
      enabled: btn.dataset.enabled === 'true',
    });
  }

  if (action === 'toggle') {
    try {
      await updateRule(name, {
        host:    btn.dataset.host,
        target:  btn.dataset.target,
        enabled: btn.dataset.enabled !== 'true',
      });
      showToast(`Rule ${btn.dataset.enabled === 'true' ? 'disabled' : 'enabled'}`);
      await loadRules();
      startPolling();
    } catch (err) {
      showToast(err.message, true);
    }
  }
});

// ── Help modal ────────────────────────────────────────────────────────────────

const helpOverlay = document.getElementById('help-overlay');
document.getElementById('btn-help').addEventListener('click', () => helpOverlay.classList.remove('hidden'));
document.getElementById('btn-help-close').addEventListener('click', () => helpOverlay.classList.add('hidden'));
helpOverlay.addEventListener('click', e => { if (e.target === helpOverlay) helpOverlay.classList.add('hidden'); });

// ── Init ──────────────────────────────────────────────────────────────────────
loadRules();
