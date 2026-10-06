const tokenKey = 'staff_token';
const readStaffStorage = key => localStorage.getItem(key) || sessionStorage.getItem(key);
const writeStaffStorage = (key, value) => {
  if (value) {
    localStorage.setItem(key, value);
    sessionStorage.setItem(key, value);
  } else {
    localStorage.removeItem(key);
    sessionStorage.removeItem(key);
  }
};
let token = readStaffStorage(tokenKey);
const adminBase = '/academic-api/api/admin';
const academicBase = '/academic-api';
const $ = selector => document.querySelector(selector);
const message = $('#message');

async function request(base, path, options = {}) {
  const response = await fetch(`${base}/${path}`, {
    ...options,
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}`, ...options.headers }
  });
  if (response.status === 401 || response.status === 403) {
    writeStaffStorage(tokenKey, null);
    writeStaffStorage('staff_role', null);
    location.href = '../index.html';
    return null;
  }
  if (!response.ok) throw new Error(await response.text() || 'Request failed');
  if (response.status === 204) return null;
  const text = await response.text();
  return text ? JSON.parse(text) : null;
}

function values(form) {
  return Object.fromEntries([...new FormData(form)]
    .filter(([, value]) => value !== '')
    .map(([key, value]) => [key, /^\d+(\.\d+)?$/.test(value) ? Number(value) : value]));
}

function addOption(select, value, label) {
  const option = document.createElement('option');
  option.value = value ?? '';
  option.textContent = label;
  select.append(option);
}

let studentSearchTimer;
$('#student-search')?.addEventListener('input', async event => {
  clearTimeout(studentSearchTimer);
  const query = event.target.value.trim();
  const results = $('#student-search-results');
  if (query.length < 2) { results.replaceChildren(); return; }
  studentSearchTimer = setTimeout(async () => {
    try {
      const students = await request(academicBase, `api/intake-students?search=${encodeURIComponent(query)}&page_size=50`);
      const matches = (students || []).filter(student => {
        const name = (student.name || '').toLowerCase();
        const admission = (student.admission_number || '').toLowerCase();
        const needle = query.toLowerCase();
        return name.includes(needle) || admission.includes(needle);
      });
      results.replaceChildren();
      for (const student of matches.slice(0, 12)) {
        const button = document.createElement('button');
        button.type = 'button';
        button.style.cssText = 'margin:4px 4px 0 0;padding:8px 12px;font-size:12px';
        button.textContent = `${student.name} (${student.admission_number || 'no adm no'})`;
        button.onclick = () => {
          document.querySelector('#enroll-form [name="student_id"]').value = student.student_id;
          results.replaceChildren();
        };
        results.append(button);
      }
    } catch (error) {
      results.textContent = error.message;
    }
  }, 250);
});

function cell(row, value) {
  const item = document.createElement('td');
  item.textContent = value ?? '-';
  row.append(item);
  return item;
}

async function loadStaff() {
  const staff = await request(adminBase, 'staff');
  const body = $('#staff-table tbody');
  body.replaceChildren();
  for (const person of staff) {
    const row = document.createElement('tr');
    for (const value of [person.id, person.name, person.email, person.role, person.department_id, person.status]) cell(row, value);
    const actions = document.createElement('td');
    const edit = document.createElement('button');
    edit.textContent = 'Edit';
    edit.onclick = async () => {
      const name = prompt('Staff name:', person.name);
      if (name === null) return;
      const email = prompt('Email:', person.email);
      if (email === null) return;
      const role = prompt('Role (teacher, hod, registrar, academic_admin, finance_admin):', person.role);
      if (role === null) return;
      const department = prompt('Department ID (blank to clear):', person.department_id ?? '');
      if (department === null) return;
      try {
        await request(adminBase, `staff/${person.id}`, {
          method: 'PUT',
          body: JSON.stringify({ name, email, role, department_id: department ? Number(department) : null, status: person.status })
        });
        await loadAdminData();
      } catch (error) { message.textContent = error.message; }
    };
    const deactivate = document.createElement('button');
    deactivate.textContent = 'Deactivate';
    deactivate.disabled = person.status === 'SUSPENDED';
    deactivate.onclick = async () => {
      if (!confirm(`Deactivate ${person.name}?`)) return;
      try {
        await request(adminBase, `staff/${person.id}`, { method: 'DELETE' });
        await loadAdminData();
      } catch (error) { message.textContent = error.message; }
    };
    const reset = document.createElement('button');
    reset.textContent = 'Reset password';
    reset.onclick = async () => {
      const password = prompt(`New password for ${person.name} (at least 8 characters):`);
      if (password === null) return;
      try {
        await request(adminBase, `staff/${person.id}/password`, { method: 'POST', body: JSON.stringify({ password }) });
        message.textContent = 'Password reset.';
      } catch (error) { message.textContent = error.message; }
    };
    actions.append(edit, ' ', deactivate, ' ', reset);
    row.append(actions);
    body.append(row);
  }
}

async function loadClasses() {
  const classes = await request(academicBase, 'api/classes');
  const teacherStaff = await request(adminBase, 'staff');
  const teachers = teacherStaff.filter(person => person.role === 'teacher' && person.status !== 'SUSPENDED');
  const classSelect = $('#class-subject-class');
  const selectedClass = classSelect.value;
  classSelect.replaceChildren();
  addOption(classSelect, '', 'Select class');
  for (const item of classes) addOption(classSelect, item.id, `${item.name} (term ${item.academic_term_id})`);
  if (classes.some(item => String(item.id) === selectedClass)) classSelect.value = selectedClass;

  const courseSelect = $('#class-course');
  const termSelect = $('#class-term');
  const [courses, terms] = await Promise.all([request(adminBase, 'courses'), request(adminBase, 'terms')]);
  courseSelect.replaceChildren(); addOption(courseSelect, '', 'Select course');
  for (const item of courses) addOption(courseSelect, item.id, item.name);
  termSelect.replaceChildren(); addOption(termSelect, '', 'Select term');
  for (const item of terms) addOption(termSelect, item.id, item.name);

  const enrollTerm = $('#enroll-term');
  enrollTerm.replaceChildren(); addOption(enrollTerm, '', 'Select term (required)');
  for (const item of terms) addOption(enrollTerm, item.id, item.name);
  const enrollClass = $('#enroll-class');
  const rebuildEnrollClassOptions = () => {
    const termID = enrollTerm.value;
    enrollClass.replaceChildren();
    addOption(enrollClass, '', 'No class yet (optional)');
    for (const item of classes.filter(classItem => !termID || String(classItem.academic_term_id) === String(termID))) {
      addOption(enrollClass, item.id, `${item.name} (term ${item.academic_term_id})`);
    }
  };
  enrollTerm.onchange = rebuildEnrollClassOptions;
  rebuildEnrollClassOptions();

  const subjectSelect = $('#class-subject-subject');
  const subjects = await request(academicBase, 'api/subjects');
  subjectSelect.replaceChildren(); addOption(subjectSelect, '', 'Select subject');
  for (const item of subjects) addOption(subjectSelect, item.id, `${item.name}${item.code ? ` (${item.code})` : ''}`);

  const teacherSelect = $('#class-subject-teacher');
  teacherSelect.replaceChildren(); addOption(teacherSelect, '', 'Unassigned');
  for (const teacher of teachers) addOption(teacherSelect, teacher.id, teacher.name);

  const body = $('#classes-table tbody');
  body.replaceChildren();
  for (const item of classes) {
    const row = document.createElement('tr');
    for (const value of [item.id, item.name, item.course_id, item.academic_term_id, item.capacity]) cell(row, value);
    const subjectsCell = document.createElement('td');
    const view = document.createElement('button');
    view.textContent = 'View subjects';
    view.onclick = () => loadClassSubjects(item.id, teachers);
    subjectsCell.append(view);
    row.append(subjectsCell);
    body.append(row);
  }
}

async function loadClassSubjects(classId, teachers) {
  $('#class-subject-class').value = String(classId);
  const subjects = await request(adminBase, `class-subjects?class_offering_id=${classId}`);
  const body = $('#class-subjects-table tbody');
  body.replaceChildren();
  for (const item of subjects) {
    const row = document.createElement('tr');
    cell(row, item.id);
    cell(row, item.subject_id);
    const teacherCell = document.createElement('td');
    const select = document.createElement('select');
    addOption(select, '', 'Unassigned');
    for (const teacher of teachers) addOption(select, teacher.id, teacher.name);
    select.value = item.teacher_id == null ? '' : String(item.teacher_id);
    select.addEventListener('change', async () => {
      if (!select.value) return;
      try {
        await request(adminBase, `class-subjects/${item.id}/teacher`, {
          method: 'PUT', body: JSON.stringify({ teacher_id: Number(select.value) })
        });
        message.textContent = 'Teacher assigned.';
      } catch (error) { message.textContent = error.message; }
    });
    teacherCell.append(select);
    row.append(teacherCell);
    body.append(row);
  }
}

async function loadAdminData() {
  try {
    await Promise.all([loadStaff(), loadClasses()]);
  } catch (error) { message.textContent = error.message; }
}

document.querySelectorAll('form[data-path]').forEach(form => form.addEventListener('submit', async event => {
  event.preventDefault();
  try {
    await request(adminBase, form.dataset.path, { method: 'POST', body: JSON.stringify(values(form)) });
    message.textContent = 'Saved.';
    form.reset();
    await loadAdminData();
  } catch (error) { message.textContent = error.message; }
}));

$('#subject-form').addEventListener('submit', async event => {
  event.preventDefault();
  try {
    await request(academicBase, 'api/subjects', { method: 'POST', body: JSON.stringify(values(event.target)) });
    message.textContent = 'Subject created.';
    event.target.reset();
    await loadClasses();
  } catch (error) { message.textContent = error.message; }
});

$('#class-subject-form').addEventListener('submit', async event => {
  event.preventDefault();
  const data = values(event.target);
  try {
    await request(adminBase, 'class-subjects', { method: 'POST', body: JSON.stringify(data) });
    message.textContent = 'Subject added to class.';
    const staff = await request(adminBase, 'staff');
    await loadClassSubjects(data.class_offering_id, staff.filter(person => person.role === 'teacher'));
  } catch (error) { message.textContent = error.message; }
});

$('#enroll-form').addEventListener('submit', async event => {
  event.preventDefault();
  const data = values(event.target);
  try {
    await request(academicBase, 'api/enrollments', { method: 'POST', body: JSON.stringify(data) });
    message.textContent = `Student ${data.student_id} enrolled.`;
    event.target.reset();
  } catch (error) { message.textContent = error.message; }
});

$('#logout').addEventListener('click', async () => {
  await fetch('/api/auth/logout', { headers: { Authorization: `Bearer ${token}` } }).catch(() => {});
  writeStaffStorage(tokenKey, null);
  writeStaffStorage('staff_role', null);
  location.href = '../index.html';
});
if (!token) {
  location.href = '../index.html';
} else {
  $('#workspace').hidden = false;
  loadAdminData();
}
