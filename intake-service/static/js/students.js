const tokenKey = 'staff_token';
const readToken = () => localStorage.getItem(tokenKey) || sessionStorage.getItem(tokenKey) || sessionStorage.getItem('intake_admin_token');
let token = readToken();
let editingID = null;
let editingVersion = null;
const app = document.querySelector('#app');
const form = document.querySelector('#student-form');
const status = document.querySelector('#form-status');

async function request(path, options = {}) {
  const headers = { ...options.headers };
  if (token) headers.Authorization = `Bearer ${token}`;
  if (!headers['Content-Type'] && typeof options.body === 'string') {
    headers['Content-Type'] = 'application/json';
  }
  const response = await fetch(path, { ...options, headers });
  if (response.status === 401 || response.status === 403) {
    localStorage.removeItem(tokenKey);
    sessionStorage.removeItem(tokenKey);
    sessionStorage.removeItem('intake_admin_token');
    localStorage.removeItem('staff_role');
    sessionStorage.removeItem('staff_role');
    location.href = '/staff/';
    return null;
  }
  if (!response.ok) throw new Error(await response.text() || 'Request failed');
  return response.status === 204 ? null : response.json();
}

function showApp() { app.hidden = false; loadStudents(); loadApplications(); }
function resetForm() { editingID = null; form.reset(); document.querySelector('[name="status"]').value = 'ACTIVE'; document.querySelector('#form-title').textContent = 'Add student'; document.querySelector('#save-button').textContent = 'Create student'; document.querySelector('#cancel-button').hidden = true; }
function editStudent(student) { editingID = student.student_id; editingVersion = student.version; for (const [key, value] of Object.entries({ name: student.name, id_number: student.id_number, date_of_birth: student.date_of_birth, email: student.email, phone_number: student.phone_number, kcse_index_number: student.kcse_index_number, kcse_year: student.kcse_year, kcpe_index_number: student.kcpe_index_number, kcpe_year: student.kcpe_year, parent_name: student.parent_name, parent_phone_number: student.parent_phone_number, county: student.county, ward: student.ward, location: student.location, village: student.village, admission_number: student.admission_number, status: student.status })) { if (form.elements[key]) form.elements[key].value = value ?? ''; } document.querySelector('#form-title').textContent = `Edit student #${editingID}`; document.querySelector('#save-button').textContent = 'Save changes'; document.querySelector('#cancel-button').hidden = false; form.scrollIntoView({ behavior: 'smooth' }); }
function cell(row, value) { const item = document.createElement('td'); item.textContent = value || '-'; row.append(item); }

async function loadStudents() {
  document.querySelector('#list-status').textContent = 'Loading...';
  try { const students = await request('/api/students'); const body = document.querySelector('#students'); body.replaceChildren(); for (const student of students) { const row = document.createElement('tr'); for (const value of [student.student_id, student.name, student.id_number, student.date_of_birth, student.email, student.phone_number, student.admission_number, student.status]) cell(row, value); const actions = document.createElement('td'); const edit = document.createElement('button'); edit.textContent = 'Edit'; edit.addEventListener('click', () => editStudent(student)); const remove = document.createElement('button'); remove.textContent = 'Delete'; remove.className = 'danger'; remove.addEventListener('click', () => deleteStudent(student.student_id)); actions.append(edit, document.createTextNode(' '), remove); row.append(actions); body.append(row); } document.querySelector('#list-status').textContent = students.length ? '' : 'No students found.'; } catch (error) { document.querySelector('#list-status').textContent = error.message; }
}
async function loadApplications() {
  document.querySelector('#applications-status').textContent = 'Loading...';
  try {
    const applications = await request('/api/applications');
    const body = document.querySelector('#applications');
    body.replaceChildren();
    for (const app of applications) {
      const row = document.createElement('tr');
      for (const value of [app.id, app.full_name, app.course, app.phone, app.status]) cell(row, value);
      const actions = document.createElement('td');
      const review = document.createElement('button'); review.textContent = 'Review'; review.addEventListener('click', () => reviewApplication(app.id, 'REVIEWING'));
      const accept = document.createElement('button'); accept.textContent = 'Accept'; accept.addEventListener('click', () => reviewApplication(app.id, 'ACCEPTED'));
      const decline = document.createElement('button'); decline.textContent = 'Decline'; decline.addEventListener('click', () => reviewApplication(app.id, 'DECLINED'));
      const admit = document.createElement('button'); admit.textContent = 'Admit'; admit.addEventListener('click', () => admitApplication(app.id));
      actions.append(review, document.createTextNode(' '), accept, document.createTextNode(' '), decline, document.createTextNode(' '), admit);
      row.append(actions); body.append(row);
    }
    document.querySelector('#applications-status').textContent = applications.length ? '' : 'No applications found.';
  } catch (error) {
    document.querySelector('#applications-status').textContent = error.message;
  }
}
async function reviewApplication(id, status) {
  try {
    await request(`/api/applications/${id}`, { method: 'PUT', body: JSON.stringify({ status }) });
    await loadApplications();
  } catch (error) { document.querySelector('#applications-status').textContent = error.message; }
}
async function admitApplication(id) {
  try {
    const result = await request(`/api/applications/${id}/admit`, { method: 'POST' });
    alert(`Student admitted.\nAdmission no: ${result.admission_number}\n\nNext steps:\n1. Academic admin: enroll into a class for the current term.\n2. Finance admin: issue a portal PIN.\n3. Student logs into the student portal and sets a password.`);
    await loadApplications();
    await loadStudents();
  } catch (error) { document.querySelector('#applications-status').textContent = error.message; }
}
async function deleteStudent(id) { if (!confirm(`Delete student #${id}?`)) return; try { await request(`/api/students/${id}`, { method: 'DELETE' }); if (editingID === id) resetForm(); await loadStudents(); } catch (error) { document.querySelector('#list-status').textContent = error.message; } }
document.querySelector('#import-form').addEventListener('submit', async event => { event.preventDefault(); const form = event.target; const fileInput = form.querySelector('input[name="file"]'); if (!fileInput.files[0]) { document.querySelector('#import-status').textContent = 'Choose a CSV file first.'; return; } const formData = new FormData(); formData.append('file', fileInput.files[0]); document.querySelector('#import-status').textContent = 'Importing...'; try { const response = await fetch('/api/admin/students/import', { method: 'POST', headers: { Authorization: `Bearer ${token}` }, body: formData }); if (!response.ok) throw new Error(await response.text()); document.querySelector('#import-status').textContent = 'Import complete.'; form.reset(); await loadStudents(); } catch (error) { document.querySelector('#import-status').textContent = error.message; } });
form.addEventListener('submit', async event => { event.preventDefault(); status.textContent = 'Saving...'; const data = Object.fromEntries(new FormData(form)); for (const key of Object.keys(data)) { if (data[key] === '') delete data[key]; } if (data.kcse_year) data.kcse_year = Number(data.kcse_year); if (data.kcpe_year) data.kcpe_year = Number(data.kcpe_year); if (editingID) data.expected_version = editingVersion; try { await request(editingID ? `/api/students/${editingID}` : '/api/students', { method: editingID ? 'PUT' : 'POST', body: JSON.stringify(data) }); resetForm(); status.textContent = 'Saved.'; await loadStudents(); } catch (error) { status.textContent = error.message; } });
document.querySelector('#cancel-button').addEventListener('click', resetForm); document.querySelector('#refresh-button').addEventListener('click', loadStudents); document.querySelector('#refresh-applications').addEventListener('click', loadApplications); document.querySelector('#sign-out').addEventListener('click', async () => {
  await fetch('/api/auth/logout', { headers: { Authorization: `Bearer ${token}` } }).catch(() => {});
  sessionStorage.removeItem(tokenKey);
  sessionStorage.removeItem('intake_admin_token');
  sessionStorage.removeItem('staff_role');
  location.href = '/staff/';
});
if (!token) {
  location.href = '/staff/';
} else {
  showApp();
}
