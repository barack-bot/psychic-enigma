const loginForm = document.getElementById('staff-login-form');
const loginStatus = document.getElementById('login-status');
const readStaffStorage = key => {
  try {
    const stored = localStorage.getItem(key);
    return stored ?? sessionStorage.getItem(key);
  } catch (error) {
    return sessionStorage.getItem(key);
  }
};
const writeStaffStorage = (key, value) => {
  try {
    localStorage.setItem(key, value);
  } catch (error) {}
  sessionStorage.setItem(key, value);
};

loginForm?.addEventListener('submit', async event => {
  event.preventDefault();
  loginStatus.textContent = 'Signing in...';
  const payload = Object.fromEntries(new FormData(loginForm));

  try {
    const response = await fetch('/api/auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    });
    const text = await response.text();
    if (!response.ok) throw new Error(text || 'Unable to sign in.');

    const result = JSON.parse(text);
    const routeByRole = {
      teacher: '/staff/teacher/dashboard.html',
      hod: '/staff/hod/dashboard.html',
      academic_admin: '/staff/admin/index.html',
      registrar: '/students.html',
      super_admin: '/staff/admin/index.html',
      finance_admin: '/admin/'
    };
    writeStaffStorage('staff_token', result.token);
    writeStaffStorage('staff_role', result.role);
    window.location.assign(routeByRole[result.role] || '/');
  } catch (error) {
    loginStatus.textContent = error.message;
  }
});
