const form = document.querySelector('#intake-form');
const status = document.querySelector('#form-status');

form?.addEventListener('submit', async (event) => {
	event.preventDefault();
	status.textContent = 'Submitting your application...';
	const payload = {
		full_name: (form.full_name.value || '').trim(),
		email: (form.email.value || '').trim(),
		phone: (form.phone.value || '').trim(),
		date_of_birth: form.date_of_birth.value,
		course: (form.course.value || '').trim(),
		previous_school: (form.previous_school.value || '').trim(),
		message: (form.message.value || '').trim(),
	};

	if (!payload.full_name || !payload.email || !payload.phone || !payload.date_of_birth || !payload.course) {
		status.textContent = 'Please complete the required application details.';
		return;
	}

	try {
		const response = await fetch('/api/applications', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify(payload)
		});
		const responseText = await response.text();
		if (!response.ok) throw new Error(responseText || 'Application could not be submitted.');
		const result = responseText ? JSON.parse(responseText) : {};
		form.reset();
		status.textContent = `Application submitted successfully. Reference #${result.id}. Our admissions team will contact you soon.`;
	} catch (error) {
		status.textContent = error.message || 'Application could not be submitted.';
	}
});
