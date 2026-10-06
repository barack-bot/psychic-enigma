-- ONE-OFF MANUAL SCRIPT. Run only after event publishing is verified.
-- Rebuilds pending update events from the current intake student state.
-- Review the generated rows before starting the publisher.
INSERT INTO student_events_outbox (student_id, event_type, version, payload)
SELECT s.student_id, 'StudentUpdated', s.version,
       jsonb_build_object('student_id', s.student_id,
                          'admission_number', s.admission_number,
                          'name', s.name,
                          'status', s.status,
                          'version', s.version)
FROM students s
WHERE s.deleted_at IS NULL;