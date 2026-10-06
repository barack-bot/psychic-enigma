INSERT INTO public.departments (name)
VALUES ('Admissions'), ('Academics'), ('Finance')
ON CONFLICT (name) DO NOTHING;

INSERT INTO public.staff_users (name, email, username, password_hash, role, department_id, status)
VALUES
  ('Registrar Admin', 'registrar@school.local', 'registrar', '$2a$10$80OuTUSq2ItFU7m919KKk.MJsZslgKIypS/kdF3r5tc7s22r3i2Ie', 'registrar', (SELECT id FROM public.departments WHERE name = 'Admissions'), 'ACTIVE'),
  ('Academic Admin', 'academic_admin@school.local', 'academic_admin', '$2a$10$uCPqFYTDxIjYRjcxNwRnnegxc24rgs5OXiZUO4ZTyPuw/.qNjismK', 'academic_admin', (SELECT id FROM public.departments WHERE name = 'Academics'), 'ACTIVE'),
  ('Finance Admin', 'finance_admin@school.local', 'finance_admin', '$2a$10$Nx5q1FK4Pj5Jxm2SEe2wUuK8I8H55xM3MUbFMalJ8guCpqaVFPnca', 'finance_admin', (SELECT id FROM public.departments WHERE name = 'Finance'), 'ACTIVE'),
  ('Head of Department', 'hod@school.local', 'hod', '$2a$10$amaBC2TFUgS65qzIAllJl.TJn7c9IwaQWM9.5ZQvTzHLSMvRTf5mq', 'hod', (SELECT id FROM public.departments WHERE name = 'Academics'), 'ACTIVE'),
  ('Teacher User', 'teacher@school.local', 'teacher', '$2a$10$rClkDtbZJvN/lA5cqqKku.T4TMTng0ILxgOKtX2Osg9uBVQ8KueUO', 'teacher', (SELECT id FROM public.departments WHERE name = 'Academics'), 'ACTIVE'),
  ('Super Admin', 'super_admin@school.local', 'super_admin', '$2a$10$pYZggROqyW0prgZMu11MOufoQktASmNDy64K1pLrbir0OqFf25sam', 'super_admin', (SELECT id FROM public.departments WHERE name = 'Academics'), 'ACTIVE')
ON CONFLICT (username) DO UPDATE
SET name = EXCLUDED.name,
    email = EXCLUDED.email,
    password_hash = EXCLUDED.password_hash,
    role = EXCLUDED.role,
    department_id = EXCLUDED.department_id,
    status = EXCLUDED.status;
