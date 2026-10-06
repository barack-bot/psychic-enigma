window.staffReady = async () => {
  const target = document.querySelector('#marks');
  target.replaceChildren();
  const marks = await staffApi('/api/marks?status=SUBMITTED');
  if (!marks.length) {
    target.textContent = 'No submitted marks.';
    return;
  }
  const approveAll = document.createElement('button');
  approveAll.type = 'button';
  approveAll.textContent = 'Approve all visible';
  approveAll.onclick = async () => {
    try {
      await Promise.all(marks.map(mark => staffApi(`/api/marks/approve`, {
        method: 'POST',
        body: JSON.stringify({ mark_id: mark.id })
      })));
      await window.staffReady();
    } catch (error) {
      alert(error.message);
    }
  };
  target.append(approveAll, document.createElement('br'));
  for (const mark of marks) {
    const row = document.createElement('article');
    row.className = 'row';
    const text = document.createElement('span');
    text.textContent = `#${mark.id} ${mark.student_name} (${mark.student_id}) | ${mark.class_name || 'Class'} / ${mark.subject_name || 'Subject'} / ${mark.assessment_name}: ${mark.score}`;
    const actions = document.createElement('span');
    const approve = document.createElement('button');
    approve.textContent = 'Approve';
    approve.onclick = () => transition(mark.id, 'approve');
    const reject = document.createElement('button');
    reject.className = 'danger';
    reject.textContent = 'Reject';
    reject.onclick = () => transition(mark.id, 'reject');
    actions.append(approve, ' ', reject);
    row.append(text, actions);
    target.append(row);
  }
};

async function transition(id, action) {
  const reason = action === 'reject' ? prompt('Reason for rejection:') : '';
  if (action === 'reject' && !reason?.trim()) return;
  try {
    await staffApi(`/api/marks/${action}`, {
      method: 'POST',
      body: JSON.stringify({ mark_id: id, rejection_reason: reason || '' })
    });
    await staffReady();
  } catch (error) {
    alert(error.message);
  }
}
