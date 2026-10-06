# Academic Service

Handles academic structure (courses, classes, subjects), student term
enrollment, assessment/mark entry by teachers, and HOD approval workflow.

## Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/subjects` | List subjects |
| POST | `/api/subjects` | Create subject |
| GET | `/api/classes` | List class offerings (filter by term/teacher) |
| POST | `/api/classes` | Create class offering |
| GET | `/api/enrollments` | List term enrollments (filter by student) |
| POST | `/api/enrollments` | Upsert term enrollment |
| GET | `/api/assessments` | List assessments |
| POST | `/api/assessments` | Create assessment |
| GET | `/api/marks` | List marks (filter by status) |
| POST | `/api/marks` | Save draft mark |
| POST | `/api/marks/submit` | Submit mark for approval |
| POST | `/api/marks/approve` | HOD approves submitted mark |
| POST | `/api/marks/reject` | HOD rejects submitted mark |

## Environment

- `ACADEMIC_DATABASE_URL` / `DATABASE_URL` — Postgres DSN
- `JWT_SECRET` — required
- `ACADEMIC_PORT` — default `8082`
- `NATS_URL` — default `nats://127.0.0.1:4222`
- `INTAKE_URL` — default `http://intake-service:8080`

## Staff UI

Served at `/staff/` — teacher and HOD dashboards.