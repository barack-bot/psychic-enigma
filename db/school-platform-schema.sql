-- ============================================================
-- SCHOOL PLATFORM — CONSOLIDATED SINGLE-DATABASE SCHEMA
-- Replaces the three separate databases (intake/academic/fees).
-- No projection tables, no outbox tables, no reconciliation
-- tables — real foreign keys instead. Fee is a flat rate per
-- academic term, same amount regardless of course.
-- ============================================================

BEGIN;

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE OR REPLACE FUNCTION public.set_updated_at()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$;

-- ============================================================
-- 1. IDENTITY & ACCESS
-- ============================================================

CREATE TABLE IF NOT EXISTS public.departments (
    id serial PRIMARY KEY,
    name varchar(100) NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS public.staff_users (
    id bigserial PRIMARY KEY,
    employee_no varchar(20) UNIQUE,
    name varchar(100) NOT NULL,
    email varchar(255) UNIQUE,
    phone varchar(15),
    username varchar(100) NOT NULL UNIQUE,
    password_hash text NOT NULL,
    -- registrar: intake/admissions. academic_admin: curriculum & terms.
    -- finance_admin: fees & payments. teacher: enters marks for their
    -- own classes. hod: approves marks for their department.
    -- super_admin: everything.
    role varchar(30) NOT NULL,
    department_id integer REFERENCES public.departments(id),
    status varchar(20) NOT NULL DEFAULT 'ACTIVE',
    created_at timestamp without time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT staff_users_role_check CHECK (role IN
        ('registrar', 'academic_admin', 'finance_admin', 'teacher', 'hod', 'super_admin')),
    CONSTRAINT staff_users_status_check CHECK (status IN ('ACTIVE', 'SUSPENDED', 'LEFT'))
);
DROP TRIGGER IF EXISTS staff_users_set_updated_at ON public.staff_users;
CREATE TRIGGER staff_users_set_updated_at BEFORE UPDATE ON public.staff_users
FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();

CREATE TABLE IF NOT EXISTS public.students (
    student_id serial PRIMARY KEY,
    name varchar(100) NOT NULL,
    id_number varchar(20) NOT NULL UNIQUE,
    email varchar(255) UNIQUE,
    date_of_birth date NOT NULL,
    phone_number varchar(15),
    kcse_index_number varchar(20),
    kcse_year integer,
    kcpe_index_number varchar(20),
    kcpe_year integer,
    parent_name varchar(100),
    parent_phone_number varchar(15),
    county varchar(50),
    ward varchar(50),
    location varchar(50),
    village varchar(50),
    admission_number varchar(20),
    portal_pin_hash text,
    password_hash text,
    must_change_password boolean NOT NULL DEFAULT false,
    is_continuing boolean NOT NULL DEFAULT false,
    status varchar(20) NOT NULL DEFAULT 'ACTIVE',
    version integer NOT NULL DEFAULT 1,
    deleted_at timestamp without time zone,
    created_at timestamp without time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT students_status_check CHECK (status IN ('ACTIVE', 'WITHDRAWN', 'GRADUATED', 'SUSPENDED'))
);
CREATE UNIQUE INDEX IF NOT EXISTS students_admission_number_key
    ON public.students (admission_number) WHERE admission_number IS NOT NULL;
CREATE TABLE IF NOT EXISTS public.student_events_outbox (
    event_id varchar(64) PRIMARY KEY,
    event_type varchar(30) NOT NULL,
    student_id integer NOT NULL REFERENCES public.students(student_id),
    version integer NOT NULL,
    payload jsonb NOT NULL,
    created_at timestamp without time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
    published_at timestamp without time zone
);
CREATE INDEX IF NOT EXISTS outbox_unpublished_idx
    ON public.student_events_outbox (created_at) WHERE published_at IS NULL;
DROP TRIGGER IF EXISTS students_set_updated_at ON public.students;
CREATE TRIGGER students_set_updated_at BEFORE UPDATE ON public.students
FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();

CREATE TABLE IF NOT EXISTS public.applications (
    id bigserial PRIMARY KEY,
    full_name varchar(150) NOT NULL,
    email varchar(255) NOT NULL,
    phone varchar(30) NOT NULL,
    date_of_birth date,
    course varchar(150) NOT NULL,
    previous_school varchar(200),
    message text,
    status varchar(20) NOT NULL DEFAULT 'RECEIVED',
    created_at timestamp without time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT applications_status_check CHECK (status IN ('RECEIVED', 'REVIEWING', 'ACCEPTED', 'ADMITTED', 'DECLINED'))
);
CREATE INDEX IF NOT EXISTS applications_created_at_idx ON public.applications (created_at DESC);

CREATE TABLE IF NOT EXISTS public.auth_sessions (
    id bigserial PRIMARY KEY,
    token_hash char(64) NOT NULL UNIQUE,
    principal_type varchar(20) NOT NULL,
    student_id integer REFERENCES public.students(student_id) ON DELETE CASCADE,
    staff_user_id bigint REFERENCES public.staff_users(id) ON DELETE CASCADE,
    login_origin varchar(40) NOT NULL DEFAULT 'unknown',
    user_agent text,
    client_ip text,
    expires_at timestamp without time zone NOT NULL,
    created_at timestamp without time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT auth_sessions_principal_check CHECK (
        (principal_type = 'student' AND student_id IS NOT NULL AND staff_user_id IS NULL)
        OR (principal_type = 'staff' AND staff_user_id IS NOT NULL AND student_id IS NULL)
    )
);
CREATE INDEX IF NOT EXISTS auth_sessions_expires_at_idx ON public.auth_sessions (expires_at);
CREATE INDEX IF NOT EXISTS auth_sessions_student_id_idx ON public.auth_sessions (student_id) WHERE student_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS auth_sessions_staff_user_id_idx ON public.auth_sessions (staff_user_id) WHERE staff_user_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS public.audit_logs (
    id bigserial PRIMARY KEY,
    actor_type varchar(40) NOT NULL,
    actor_id bigint,
    actor_name text,
    action varchar(100) NOT NULL,
    target_type varchar(40),
    target_id bigint,
    details jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamp without time zone NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS audit_logs_target_idx ON public.audit_logs (target_type, target_id);
CREATE INDEX IF NOT EXISTS audit_logs_created_at_idx ON public.audit_logs (created_at DESC);

-- ============================================================
-- 2. ACADEMIC STRUCTURE (courses, calendar)
-- ============================================================

CREATE TABLE IF NOT EXISTS public.course_levels (
    id serial PRIMARY KEY,
    name varchar(50) NOT NULL UNIQUE,
    total_terms integer NOT NULL CHECK (total_terms > 0)
);

CREATE TABLE IF NOT EXISTS public.courses (
    id serial PRIMARY KEY,
    name varchar(100) NOT NULL,
    level_id integer NOT NULL REFERENCES public.course_levels(id),
    department_id integer REFERENCES public.departments(id),
    UNIQUE (name, level_id)
);

CREATE TABLE IF NOT EXISTS public.academic_years (
    id bigserial PRIMARY KEY,
    year_label varchar(20) NOT NULL UNIQUE,
    start_date date NOT NULL,
    end_date date NOT NULL,
    CONSTRAINT academic_year_dates_check CHECK (end_date > start_date)
);

CREATE TABLE IF NOT EXISTS public.academic_terms (
    id bigserial PRIMARY KEY,
    academic_year_id bigint NOT NULL REFERENCES public.academic_years(id) ON DELETE RESTRICT,
    term_number integer NOT NULL CHECK (term_number BETWEEN 1 AND 3),
    term_type varchar(20) NOT NULL DEFAULT 'NORMAL',
    name varchar(80) NOT NULL UNIQUE,
    starts_on date NOT NULL,
    ends_on date NOT NULL,
    version integer NOT NULL DEFAULT 1,
    created_at timestamp without time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT academic_terms_dates_check CHECK (ends_on >= starts_on),
    CONSTRAINT academic_terms_type_check CHECK (term_type IN ('NORMAL', 'ATTACHMENT')),
    CONSTRAINT academic_terms_dates_key UNIQUE (starts_on, ends_on),
    CONSTRAINT academic_terms_year_term_key UNIQUE (academic_year_id, term_number)
);

CREATE TABLE IF NOT EXISTS public.student_course_placement (
    id bigserial PRIMARY KEY,
    student_id integer NOT NULL REFERENCES public.students(student_id),
    course_id integer NOT NULL REFERENCES public.courses(id) ON DELETE RESTRICT,
    effective_from date NOT NULL,
    effective_to date,
    created_at timestamp without time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT placement_dates_check CHECK (effective_to IS NULL OR effective_to >= effective_from)
);
CREATE UNIQUE INDEX IF NOT EXISTS student_one_current_placement
    ON public.student_course_placement(student_id) WHERE effective_to IS NULL;

-- ============================================================
-- 3. CLASSES, SUBJECTS & TERM ENROLLMENT
-- This is the piece that was entirely missing before: which
-- specific class/stream a student sits in for a given term, and
-- which subjects are taught in that class by which teacher.
-- ============================================================

CREATE TABLE IF NOT EXISTS public.subjects (
    id serial PRIMARY KEY,
    name varchar(100) NOT NULL,
    code varchar(20) UNIQUE,
    department_id integer REFERENCES public.departments(id)
);

-- A specific offering of a course, for one term, as a class/stream
-- (e.g. "Electrical Eng Year 1A" for Term 1 2026).
CREATE TABLE IF NOT EXISTS public.class_offerings (
    id bigserial PRIMARY KEY,
    course_id integer NOT NULL REFERENCES public.courses(id) ON DELETE RESTRICT,
    academic_term_id bigint NOT NULL REFERENCES public.academic_terms(id) ON DELETE RESTRICT,
    name varchar(80) NOT NULL,
    class_teacher_id bigint REFERENCES public.staff_users(id),
    capacity integer CHECK (capacity IS NULL OR capacity > 0),
    created_at timestamp without time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (course_id, academic_term_id, name)
);
CREATE INDEX IF NOT EXISTS class_offerings_term_idx ON public.class_offerings (academic_term_id);

-- Which subjects are taught in a class offering, and by whom.
CREATE TABLE IF NOT EXISTS public.class_subjects (
    id bigserial PRIMARY KEY,
    class_offering_id bigint NOT NULL REFERENCES public.class_offerings(id) ON DELETE CASCADE,
    subject_id integer NOT NULL REFERENCES public.subjects(id) ON DELETE RESTRICT,
    teacher_id bigint REFERENCES public.staff_users(id),
    UNIQUE (class_offering_id, subject_id)
);
CREATE INDEX IF NOT EXISTS class_subjects_teacher_idx ON public.class_subjects (teacher_id);

-- One row per student per term: is the student active this term,
-- and (once assigned) which class offering are they in. This is
-- what the fee scheduler and academic reporting both key off.
CREATE TABLE IF NOT EXISTS public.student_term_enrollment (
    id bigserial PRIMARY KEY,
    student_id integer NOT NULL REFERENCES public.students(student_id),
    academic_term_id bigint NOT NULL REFERENCES public.academic_terms(id) ON DELETE RESTRICT,
    class_offering_id bigint REFERENCES public.class_offerings(id),
    status varchar(20) NOT NULL DEFAULT 'ENROLLED',
    reported_at timestamp without time zone,
    deferred_reason text,
    version integer NOT NULL DEFAULT 1,
    created_at timestamp without time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT term_enrollment_status_check CHECK (status IN
        ('ENROLLED', 'ACTIVE', 'COMPLETED', 'DEFERRED', 'WITHDRAWN')),
    CONSTRAINT term_enrollment_term_key UNIQUE (student_id, academic_term_id)
);
CREATE INDEX IF NOT EXISTS term_enrollment_student_idx ON public.student_term_enrollment (student_id);
CREATE INDEX IF NOT EXISTS term_enrollment_class_idx ON public.student_term_enrollment (class_offering_id);
DROP TRIGGER IF EXISTS term_enrollment_set_updated_at ON public.student_term_enrollment;
CREATE TRIGGER term_enrollment_set_updated_at BEFORE UPDATE ON public.student_term_enrollment
FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();

-- ============================================================
-- 4. ASSESSMENTS & MARKS (teacher entry -> HOD approval workflow)
-- ============================================================

CREATE TABLE IF NOT EXISTS public.assessments (
    id bigserial PRIMARY KEY,
    class_subject_id bigint NOT NULL REFERENCES public.class_subjects(id) ON DELETE CASCADE,
    name varchar(100) NOT NULL,
    assessment_type varchar(20) NOT NULL,
    max_score numeric(6, 2) NOT NULL CHECK (max_score > 0),
    weight_percent numeric(5, 2) CHECK (weight_percent IS NULL OR (weight_percent > 0 AND weight_percent <= 100)),
    created_by bigint REFERENCES public.staff_users(id),
    created_at timestamp without time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT assessments_type_check CHECK (assessment_type IN ('CAT', 'EXAM', 'ASSIGNMENT', 'PRACTICAL'))
);

CREATE TABLE IF NOT EXISTS public.marks (
    id bigserial PRIMARY KEY,
    assessment_id bigint NOT NULL REFERENCES public.assessments(id) ON DELETE CASCADE,
    student_id integer NOT NULL REFERENCES public.students(student_id),
    score numeric(6, 2) NOT NULL CHECK (score >= 0),
    status varchar(20) NOT NULL DEFAULT 'DRAFT',
    submitted_by bigint REFERENCES public.staff_users(id),
    submitted_at timestamp without time zone,
    approved_by bigint REFERENCES public.staff_users(id),
    approved_at timestamp without time zone,
    rejection_reason text,
    version integer NOT NULL DEFAULT 1,
    created_at timestamp without time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT marks_status_check CHECK (status IN ('DRAFT', 'SUBMITTED', 'APPROVED', 'REJECTED')),
    CONSTRAINT marks_assessment_student_key UNIQUE (assessment_id, student_id)
);
CREATE INDEX IF NOT EXISTS marks_student_idx ON public.marks (student_id);
CREATE INDEX IF NOT EXISTS marks_status_idx ON public.marks (status);
DROP TRIGGER IF EXISTS marks_set_updated_at ON public.marks;
CREATE TRIGGER marks_set_updated_at BEFORE UPDATE ON public.marks
FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();

-- ============================================================
-- 5. FINANCE — flat rate per term, same amount for every course.
-- ============================================================

CREATE TABLE IF NOT EXISTS public.tuition_rates (
    id bigserial PRIMARY KEY,
    academic_term_id bigint NOT NULL UNIQUE REFERENCES public.academic_terms(id),
    amount numeric(12, 2) NOT NULL CHECK (amount >= 0),
    created_at timestamp without time zone NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Waivers/scholarships/bursaries — a reduction applied against a
-- student's charge for a term (or open-ended if academic_term_id
-- is NULL, e.g. a standing sibling discount).
CREATE TABLE IF NOT EXISTS public.student_fee_waivers (
    id bigserial PRIMARY KEY,
    student_id integer NOT NULL REFERENCES public.students(student_id),
    academic_term_id bigint REFERENCES public.academic_terms(id),
    sponsor varchar(30) NOT NULL,
    amount numeric(12, 2) NOT NULL CHECK (amount > 0),
    description varchar(255),
    approved_by bigint REFERENCES public.staff_users(id),
    created_at timestamp without time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT waiver_sponsor_check CHECK (sponsor IN ('HELB', 'CDF', 'BURSARY', 'SCHOLARSHIP', 'SIBLING_DISCOUNT', 'OTHER'))
);
CREATE INDEX IF NOT EXISTS waivers_student_idx ON public.student_fee_waivers (student_id);

CREATE TABLE IF NOT EXISTS public.student_tuition_charges (
    id bigserial PRIMARY KEY,
    student_id integer NOT NULL REFERENCES public.students(student_id),
    student_term_enrollment_id bigint REFERENCES public.student_term_enrollment(id),
    academic_term_id bigint NOT NULL REFERENCES public.academic_terms(id),
    tuition_rate_id bigint REFERENCES public.tuition_rates(id) ON DELETE RESTRICT,
    amount numeric(12, 2) NOT NULL CHECK (amount > 0),
    description varchar(255),
    created_at timestamp without time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT charge_enrollment_unique UNIQUE (student_id, student_term_enrollment_id)
);
CREATE INDEX IF NOT EXISTS charges_student_created_idx ON public.student_tuition_charges (student_id, created_at DESC);

CREATE TABLE IF NOT EXISTS public.student_term_charge_status (
    id bigserial PRIMARY KEY,
    student_id integer NOT NULL REFERENCES public.students(student_id),
    academic_term_id bigint NOT NULL REFERENCES public.academic_terms(id),
    charge_status varchar(20) NOT NULL DEFAULT 'PENDING',
    charge_attempts integer NOT NULL DEFAULT 0,
    charge_last_error text,
    created_at timestamp without time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT charge_status_check CHECK (charge_status IN ('PENDING', 'COMPLETED', 'FAILED', 'NOT_REQUIRED')),
    CONSTRAINT charge_status_term_key UNIQUE (student_id, academic_term_id)
);
DROP TRIGGER IF EXISTS charge_status_set_updated_at ON public.student_term_charge_status;
CREATE TRIGGER charge_status_set_updated_at BEFORE UPDATE ON public.student_term_charge_status
FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();

CREATE TABLE IF NOT EXISTS public.tuition_payments (
    id bigserial PRIMARY KEY,
    student_id integer NOT NULL REFERENCES public.students(student_id),
    student_term_enrollment_id bigint REFERENCES public.student_term_enrollment(id),
    academic_term_id bigint REFERENCES public.academic_terms(id),
    source varchar(20) NOT NULL,
    method varchar(20) NOT NULL,
    amount numeric(12, 2) NOT NULL CHECK (amount > 0),
    transaction_reference varchar(100) NOT NULL UNIQUE,
    status varchar(20) NOT NULL DEFAULT 'COMPLETED',
    received_by bigint REFERENCES public.staff_users(id),
    paid_at timestamp without time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at timestamp without time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT payment_source_check CHECK (source IN ('STUDENT', 'HELB', 'SCHOLARSHIP', 'OTHER')),
    CONSTRAINT payment_method_check CHECK (method IN ('MPESA', 'BANK', 'CASH', 'BURSARY', 'GOVERNMENT', 'LEGACY')),
    CONSTRAINT payment_status_check CHECK (status IN ('PENDING', 'COMPLETED', 'FAILED', 'REVERSED'))
);
CREATE INDEX IF NOT EXISTS payments_student_paid_idx ON public.tuition_payments (student_id, paid_at DESC);
CREATE INDEX IF NOT EXISTS payments_paid_at_idx ON public.tuition_payments (paid_at DESC);
CREATE INDEX IF NOT EXISTS payments_method_idx ON public.tuition_payments (method);

CREATE TABLE IF NOT EXISTS public.mpesa_stk_requests (
    checkout_request_id varchar(100) PRIMARY KEY,
    student_id integer NOT NULL REFERENCES public.students(student_id),
    amount numeric(12, 2) NOT NULL CHECK (amount > 0),
    phone_number varchar(15) NOT NULL,
    status varchar(20) NOT NULL DEFAULT 'PENDING',
    receipt_number varchar(100),
    created_at timestamp without time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at timestamp without time zone
);

-- ============================================================
-- 6. VIEWS — what the finance admin and academic admin pages read
-- ============================================================

-- Every student's running balance, net of waivers, across all terms.
CREATE OR REPLACE VIEW public.student_balances AS
SELECT s.student_id,
       s.name,
       s.admission_number,
       COALESCE(c.total_charges, 0::numeric) AS total_charges,
       COALESCE(w.total_waived, 0::numeric) AS total_waived,
       COALESCE(p.total_paid, 0::numeric) AS total_paid,
       COALESCE(c.total_charges, 0::numeric)
           - COALESCE(w.total_waived, 0::numeric)
           - COALESCE(p.total_paid, 0::numeric) AS balance
FROM public.students s
LEFT JOIN (SELECT student_id, SUM(amount) total_charges FROM public.student_tuition_charges GROUP BY student_id) c
    ON c.student_id = s.student_id
LEFT JOIN (SELECT student_id, SUM(amount) total_waived FROM public.student_fee_waivers GROUP BY student_id) w
    ON w.student_id = s.student_id
LEFT JOIN (SELECT student_id, SUM(amount) total_paid FROM public.tuition_payments WHERE status = 'COMPLETED' GROUP BY student_id) p
    ON p.student_id = s.student_id
WHERE s.status <> 'WITHDRAWN';

-- Finance admin "who hasn't paid" view — just filter balance > 0.
CREATE OR REPLACE VIEW public.unpaid_students AS
SELECT * FROM public.student_balances WHERE balance > 0 ORDER BY balance DESC;

-- Finance admin "who paid today" / daily reconciliation view.
CREATE OR REPLACE VIEW public.payments_today AS
SELECT p.id, p.student_id, s.name AS student_name, s.admission_number,
       p.amount, p.method, p.source, p.transaction_reference,
       p.received_by, u.name AS received_by_name, p.paid_at
FROM public.tuition_payments p
JOIN public.students s ON s.student_id = p.student_id
LEFT JOIN public.staff_users u ON u.id = p.received_by
WHERE p.status = 'COMPLETED' AND p.paid_at::date = CURRENT_DATE
ORDER BY p.paid_at DESC;

-- Same thing, summarized by method, for the finance dashboard header.
CREATE OR REPLACE VIEW public.daily_collections_summary AS
SELECT p.paid_at::date AS collection_date, p.method,
       COUNT(*) AS payment_count, SUM(p.amount) AS total_amount
FROM public.tuition_payments p
WHERE p.status = 'COMPLETED'
GROUP BY p.paid_at::date, p.method;

-- Per-student, per-term statement: charges, waivers, payments, balance.
CREATE OR REPLACE VIEW public.student_term_statement AS
SELECT
    s.student_id,
    t.id AS academic_term_id,
    y.year_label,
    t.name AS term_name,
    t.term_number,
    COALESCE(e.status, 'NOT_ENROLLED') AS enrollment_status,
    co.name AS class_name,
    COALESCE(cs.charge_status, 'PENDING') AS charge_status,
    COALESCE(charges.charged, 0::numeric) AS charged,
    COALESCE(waivers.waived, 0::numeric) AS waived,
    COALESCE(payments.paid, 0::numeric) AS paid,
    COALESCE(charges.charged, 0::numeric) - COALESCE(waivers.waived, 0::numeric) - COALESCE(payments.paid, 0::numeric) AS balance
FROM public.students s
CROSS JOIN public.academic_terms t
JOIN public.academic_years y ON y.id = t.academic_year_id
LEFT JOIN public.student_term_enrollment e ON e.student_id = s.student_id AND e.academic_term_id = t.id
LEFT JOIN public.class_offerings co ON co.id = e.class_offering_id
LEFT JOIN public.student_term_charge_status cs ON cs.student_id = s.student_id AND cs.academic_term_id = t.id
LEFT JOIN (SELECT student_id, academic_term_id, SUM(amount) charged FROM public.student_tuition_charges GROUP BY student_id, academic_term_id) charges
    ON charges.student_id = s.student_id AND charges.academic_term_id = t.id
LEFT JOIN (SELECT student_id, academic_term_id, SUM(amount) waived FROM public.student_fee_waivers WHERE academic_term_id IS NOT NULL GROUP BY student_id, academic_term_id) waivers
    ON waivers.student_id = s.student_id AND waivers.academic_term_id = t.id
LEFT JOIN (SELECT student_id, academic_term_id, SUM(amount) paid FROM public.tuition_payments WHERE status = 'COMPLETED' AND academic_term_id IS NOT NULL GROUP BY student_id, academic_term_id) payments
    ON payments.student_id = s.student_id AND payments.academic_term_id = t.id
WHERE e.student_id IS NOT NULL OR charges.student_id IS NOT NULL OR payments.student_id IS NOT NULL;

-- Default staff accounts for each section and a full super-admin.
INSERT INTO public.departments (name)
VALUES ('Admissions'), ('Academics'), ('Finance')
ON CONFLICT (name) DO NOTHING;

INSERT INTO public.staff_users (name, email, username, password_hash, role, department_id, status)
VALUES
    ('Registrar Admin', 'registrar@school.local', 'registrar', '$2a$10$8nENJzFCasAZ6gyBqvbVXuLnfNldxQhnn7u3AgrtzQS2pb9Dlxs.u', 'registrar', (SELECT id FROM public.departments WHERE name = 'Admissions'), 'ACTIVE'),
    ('Academic Admin', 'academic_admin@school.local', 'academic_admin', '$2a$10$8nENJzFCasAZ6gyBqvbVXuLnfNldxQhnn7u3AgrtzQS2pb9Dlxs.u', 'academic_admin', (SELECT id FROM public.departments WHERE name = 'Academics'), 'ACTIVE'),
    ('Finance Admin', 'finance_admin@school.local', 'finance_admin', '$2a$10$8nENJzFCasAZ6gyBqvbVXuLnfNldxQhnn7u3AgrtzQS2pb9Dlxs.u', 'finance_admin', (SELECT id FROM public.departments WHERE name = 'Finance'), 'ACTIVE'),
    ('Head of Department', 'hod@school.local', 'hod', '$2a$10$8nENJzFCasAZ6gyBqvbVXuLnfNldxQhnn7u3AgrtzQS2pb9Dlxs.u', 'hod', (SELECT id FROM public.departments WHERE name = 'Academics'), 'ACTIVE'),
    ('Teacher User', 'teacher@school.local', 'teacher', '$2a$10$8nENJzFCasAZ6gyBqvbVXuLnfNldxQhnn7u3AgrtzQS2pb9Dlxs.u', 'teacher', (SELECT id FROM public.departments WHERE name = 'Academics'), 'ACTIVE'),
    ('Super Admin', 'super_admin@school.local', 'super_admin', '$2a$10$8nENJzFCasAZ6gyBqvbVXuLnfNldxQhnn7u3AgrtzQS2pb9Dlxs.u', 'super_admin', (SELECT id FROM public.departments WHERE name = 'Academics'), 'ACTIVE')
ON CONFLICT (username) DO UPDATE
SET name = EXCLUDED.name,
    email = EXCLUDED.email,
    password_hash = EXCLUDED.password_hash,
    role = EXCLUDED.role,
    department_id = EXCLUDED.department_id,
    status = EXCLUDED.status;

COMMIT;
