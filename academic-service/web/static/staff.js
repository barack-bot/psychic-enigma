const staffTokenKey = 'staff_token';
const readStaffToken = () => localStorage.getItem(staffTokenKey) || sessionStorage.getItem(staffTokenKey);
const writeStaffToken = value => {
  if (value) {
    localStorage.setItem(staffTokenKey, value);
    sessionStorage.setItem(staffTokenKey, value);
  } else {
    localStorage.removeItem(staffTokenKey);
    sessionStorage.removeItem(staffTokenKey);
  }
};
let staffToken = readStaffToken();
const academicApiBase = '/academic-api';

const escapeHTML = value => String(value ?? '-').replace(/[&<>"']/g, character => ({
  '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;'
}[character]));

const staffApi = async (path, options = {}) => {
  const response = await fetch(`${academicApiBase}${path}`, {
    ...options,
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${staffToken}`, ...options.headers }
  });
  if (!response.ok) throw new Error(await response.text() || 'Request failed');
  if (response.status === 204) return null;
  const text = await response.text();
  return text ? JSON.parse(text) : null;
};

const renderList = (id, rows, fields) => {
  const element = document.getElementById(id);
  if (!element) return;
  element.innerHTML = rows?.length
    ? `<ul>${rows.map(row => `<li>${fields.map(field => escapeHTML(row[field])).join(' | ')}</li>`).join('')}</ul>`
    : '<p>No records found.</p>';
};

async function staffReady() {
  const [subjects, classes, assessments, draftMarks, rejectedMarks] = await Promise.all([
    staffApi('/api/subjects'),
    staffApi('/api/classes'),
    staffApi('/api/assessments'),
    staffApi('/api/marks?status=DRAFT'),
    staffApi('/api/marks?status=REJECTED')
  ]);
  renderList('subjects', subjects, ['name', 'code']);
  renderList('classes', classes, ['name', 'academic_term_id']);
  renderList('assessments', assessments, ['name', 'assessment_type', 'max_score']);
  renderList('marks', [...(draftMarks || []), ...(rejectedMarks || [])], ['id', 'student_id', 'score', 'status', 'rejection_reason']);
}

let staffLoaded = false;
async function loadStaff() {
  if (!staffToken || staffLoaded) return;
  staffLoaded = true;
  try {
    document.getElementById('login').hidden = true;
    document.getElementById('workspace').hidden = false;
    await staffReady();
  } catch (error) {
    staffLoaded = false;
    writeStaffToken(null);
    localStorage.removeItem('staff_role');
    sessionStorage.removeItem('staff_role');
    location.href = '../index.html';
  }
}

document.getElementById('logout')?.addEventListener('click', async () => {
  await fetch('/api/auth/logout', { headers: { Authorization: `Bearer ${staffToken}` } }).catch(() => {});
  writeStaffToken(null);
  localStorage.removeItem('staff_role');
  sessionStorage.removeItem('staff_role');
  location.href = '../index.html';
});

(function injectPortalNav() {
  const role = localStorage.getItem('staff_role') || sessionStorage.getItem('staff_role');
  const links = [
    ['Mark sheet', 'enter-marks.html', ['teacher']],
    ['Teacher dashboard', '../teacher/dashboard.html', ['teacher', 'hod', 'academic_admin', 'super_admin']],
    ['HOD dashboard', '../hod/dashboard.html', ['hod', 'academic_admin', 'super_admin']],
    ['Academic admin', '../admin/index.html', ['academic_admin', 'super_admin']],
    ['Staff portal', '../index.html', null]
  ];
  const nav = document.createElement('nav');
  nav.style.cssText = 'display:flex;gap:14px;align-items:center;font:800 11px Manrope;letter-spacing:.06em;text-transform:uppercase;margin-bottom:18px;flex-wrap:wrap';
  for (const [label, href, roles] of links) {
    if (roles && role && !roles.includes(role)) continue;
    const a = document.createElement('a');
    a.href = href;
    a.textContent = label;
    a.style.cssText = 'color:#176b53;text-decoration:none;padding:6px 10px;border:1px solid #d9e1dc;background:#fff';
    nav.append(a);
  }
  const target = document.querySelector('.app main, .app');
  target?.prepend(nav);
})();

if (staffToken) queueMicrotask(loadStaff);
