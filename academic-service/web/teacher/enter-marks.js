const classSelect = document.querySelector('#class-select');
const assessmentSelect = document.querySelector('#assessment-select');
const roster = document.querySelector('#roster');
const status = document.querySelector('#status');
let classes = [];
let assessments = [];

function classAssessments() {
  return assessments.filter(assessment => String(assessment.class_offering_id) === classSelect.value);
}

function populateAssessments() {
  assessmentSelect.replaceChildren();
  for (const assessment of classAssessments()) {
    const option = document.createElement('option');
    option.value = assessment.id;
    option.textContent = `${assessment.name} (${assessment.max_score})`;
    assessmentSelect.append(option);
  }
}

async function saveDraft(input) {
  const assessment = classAssessments().find(item => String(item.id) === assessmentSelect.value);
  if (!assessment || input.value === '') throw new Error('Select an assessment and enter a score first.');
  const result = await staffApi('/api/marks', {
    method: 'POST',
    body: JSON.stringify({
      assessment_id: assessment.id,
      student_id: Number(input.dataset.studentId),
      score: Number(input.value)
    })
  });
  input.dataset.markId = result.id;
  return result;
}

async function loadSheet() {
  if (!classSelect.value) return;
  try {
    const subjectSelect = document.querySelector('#assessment-subject');
    subjectSelect.replaceChildren();
    const subjects = await staffApi(`/api/teacher/class-subjects?class_offering_id=${classSelect.value}`);
    if (!subjects.length) {
      subjectSelect.replaceChildren(new Option('No subjects assigned to you in this class', ''));
    } else {
      for (const subject of subjects) subjectSelect.append(new Option(subject.subject_name, subject.class_subject_id));
    }
    populateAssessments();
    roster.replaceChildren();
    if (!assessmentSelect.value) {
      roster.textContent = 'No assessments are available for this class.';
      return;
    }
    const students = await staffApi(`/api/teacher/mark-sheet?class_offering_id=${classSelect.value}&assessment_id=${assessmentSelect.value}`);
    if (!students.length) {
      roster.textContent = 'No enrolled students.';
      return;
    }

    const table = document.createElement('table');
    const head = document.createElement('thead');
    const headRow = document.createElement('tr');
    for (const label of ['Student', 'Score', 'Actions']) {
      const cell = document.createElement('th');
      cell.textContent = label;
      headRow.append(cell);
    }
    head.append(headRow);
    const body = document.createElement('tbody');

    for (const student of students) {
      const row = document.createElement('tr');
      const name = document.createElement('td');
      name.textContent = student.name;
      const input = document.createElement('input');
      input.type = 'number';
      input.min = '0';
      input.step = '0.01';
      input.dataset.studentId = student.student_id;
      input.placeholder = 'Score';
      if (student.score !== null && student.score !== undefined) input.value = String(student.score);
      if (student.mark_id) input.dataset.markId = student.mark_id;
      if (student.status === 'SUBMITTED' || student.status === 'APPROVED') {
        input.disabled = true;
      }
      const scoreCell = document.createElement('td');
      scoreCell.append(input);
      const actions = document.createElement('td');
      const saveButton = document.createElement('button');
      saveButton.textContent = 'Save draft';
      saveButton.disabled = student.status === 'SUBMITTED' || student.status === 'APPROVED';
      saveButton.onclick = async () => {
        try {
          await saveDraft(input);
          status.textContent = `Draft saved for ${student.name}.`;
        } catch (error) {
          status.textContent = error.message;
        }
      };
      const submitButton = document.createElement('button');
      submitButton.textContent = 'Submit';
      submitButton.disabled = student.status === 'SUBMITTED' || student.status === 'APPROVED' || input.value === '';
      submitButton.onclick = async () => {
        try {
          await saveDraft(input);
          await staffApi('/api/teacher/submit', {
            method: 'POST',
            body: JSON.stringify({ mark_id: Number(input.dataset.markId) })
          });
          input.disabled = true;
          saveButton.disabled = true;
          submitButton.disabled = true;
          status.textContent = `Mark submitted for ${student.name}.`;
        } catch (error) {
          status.textContent = error.message;
        }
      };
      input.addEventListener('input', () => { submitButton.disabled = input.disabled || input.value === ''; });
      actions.append(saveButton, ' ', submitButton);
      row.append(name, scoreCell, actions);
      body.append(row);
    }
    table.append(head, body);
    roster.append(table);
    status.textContent = '';
  } catch (error) {
    status.textContent = error.message;
  }
}

async function initMarkSheet() {
  try {
    [classes, assessments] = await Promise.all([
      staffApi('/api/classes'),
      staffApi('/api/assessments')
    ]);
    for (const item of classes) {
      const option = document.createElement('option');
      option.value = item.id;
      option.textContent = `${item.name} (term ${item.academic_term_id})`;
      classSelect.append(option);
    }
    classSelect.addEventListener('change', loadSheet);
    assessmentSelect.addEventListener('change', loadSheet);
    document.querySelector('#assessment-form').addEventListener('submit', async event => {
      event.preventDefault();
      const data = Object.fromEntries(new FormData(event.target));
      try {
        await staffApi('/api/assessments', { method: 'POST', body: JSON.stringify({
          class_subject_id: Number(document.querySelector('#assessment-subject').value),
          name: data.name,
          assessment_type: data.assessment_type,
          max_score: Number(data.max_score)
        }) });
        status.textContent = 'Assessment created.';
        event.target.reset();
        assessments = await staffApi('/api/assessments');
        await loadSheet();
      } catch (error) { status.textContent = error.message; }
    });
    await loadSheet();
  } catch (error) {
    status.textContent = error.message;
  }
}

initMarkSheet();
