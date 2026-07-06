-- WARNING: This schema is for context only and is not meant to be run.
-- Table order and constraints may not be valid for execution.

CREATE TABLE public.roles (
  id uuid NOT NULL DEFAULT gen_random_uuid(),
  role_name text NOT NULL UNIQUE CHECK (role_name = ANY (ARRAY['STUDENT'::text, 'FACULTY'::text, 'COORDINATOR'::text, 'ADMIN'::text])),
  description text NOT NULL,
  created_at timestamp with time zone NOT NULL DEFAULT now(),
  CONSTRAINT roles_pkey PRIMARY KEY (id)
);
CREATE TABLE public.users (
  id uuid NOT NULL DEFAULT gen_random_uuid(),
  email text NOT NULL UNIQUE CHECK (email = lower(email)),
  google_sub text NOT NULL UNIQUE,
  role_id uuid NOT NULL,
  display_name text NOT NULL,
  created_at timestamp with time zone NOT NULL DEFAULT now(),
  last_login_at timestamp with time zone,
  CONSTRAINT users_pkey PRIMARY KEY (id),
  CONSTRAINT users_role_id_fkey FOREIGN KEY (role_id) REFERENCES public.roles(id)
);
CREATE TABLE public.college_students (
  roll_no text NOT NULL,
  name text NOT NULL,
  branch text NOT NULL,
  year integer NOT NULL CHECK (year >= 1 AND year <= 4),
  semester integer NOT NULL CHECK (semester >= 1 AND semester <= 8),
  email text NOT NULL UNIQUE CHECK (email = lower(email)),
  dept USER-DEFINED NOT NULL,
  CONSTRAINT college_students_pkey PRIMARY KEY (roll_no)
);
CREATE TABLE public.college_faculty (
  emp_id text NOT NULL,
  name text NOT NULL,
  dept USER-DEFINED NOT NULL,
  email text NOT NULL UNIQUE CHECK (email = lower(email)),
  CONSTRAINT college_faculty_pkey PRIMARY KEY (emp_id)
);
CREATE TABLE public.internships (
  id uuid NOT NULL DEFAULT gen_random_uuid(),
  student_user_id uuid NOT NULL,
  faculty_user_id uuid,
  company_name text NOT NULL,
  company_address text NOT NULL,
  role_title text NOT NULL,
  status USER-DEFINED NOT NULL DEFAULT 'DRAFT'::internship_status,
  enrolled_at timestamp with time zone NOT NULL DEFAULT now(),
  approved_at timestamp with time zone,
  total_submitted_reports integer NOT NULL DEFAULT 0 CHECK (total_submitted_reports >= 0),
  total_approved_reports integer NOT NULL DEFAULT 0 CHECK (total_approved_reports >= 0),
  CONSTRAINT internships_pkey PRIMARY KEY (id),
  CONSTRAINT internships_student_user_id_fkey FOREIGN KEY (student_user_id) REFERENCES public.users(id),
  CONSTRAINT internships_faculty_user_id_fkey FOREIGN KEY (faculty_user_id) REFERENCES public.users(id)
);
CREATE TABLE public.reports (
  id uuid NOT NULL DEFAULT gen_random_uuid(),
  internship_id uuid NOT NULL,
  week_number integer NOT NULL CHECK (week_number >= 1 AND week_number <= 16),
  report_type USER-DEFINED NOT NULL,
  content jsonb NOT NULL DEFAULT '{}'::jsonb,
  status USER-DEFINED NOT NULL DEFAULT 'DRAFT'::report_status,
  submitted_at timestamp with time zone,
  approved_at timestamp with time zone,
  approved_by uuid,
  reminder_sent_at timestamp with time zone,
  CONSTRAINT reports_pkey PRIMARY KEY (id),
  CONSTRAINT reports_internship_id_fkey FOREIGN KEY (internship_id) REFERENCES public.internships(id),
  CONSTRAINT reports_approved_by_fkey FOREIGN KEY (approved_by) REFERENCES public.users(id)
);
CREATE TABLE public.mentor_tokens (
  id uuid NOT NULL DEFAULT gen_random_uuid(),
  internship_id uuid NOT NULL,
  token_hash text NOT NULL UNIQUE,
  expires_at timestamp with time zone NOT NULL,
  used_at timestamp with time zone,
  created_at timestamp with time zone NOT NULL DEFAULT now(),
  CONSTRAINT mentor_tokens_pkey PRIMARY KEY (id),
  CONSTRAINT mentor_tokens_internship_id_fkey FOREIGN KEY (internship_id) REFERENCES public.internships(id)
);
CREATE TABLE public.feedback (
  id uuid NOT NULL DEFAULT gen_random_uuid(),
  internship_id uuid NOT NULL,
  source USER-DEFINED NOT NULL,
  given_by uuid,
  comments text NOT NULL,
  rating integer CHECK (rating IS NULL OR rating >= 1 AND rating <= 5),
  submitted_at timestamp with time zone NOT NULL DEFAULT now(),
  CONSTRAINT feedback_pkey PRIMARY KEY (id),
  CONSTRAINT feedback_internship_id_fkey FOREIGN KEY (internship_id) REFERENCES public.internships(id),
  CONSTRAINT feedback_given_by_fkey FOREIGN KEY (given_by) REFERENCES public.users(id)
);
CREATE TABLE public.evaluations (
  id uuid NOT NULL DEFAULT gen_random_uuid(),
  internship_id uuid NOT NULL,
  evaluator_user_id uuid NOT NULL,
  scheduled_date timestamp with time zone NOT NULL,
  marks numeric CHECK (marks IS NULL OR marks >= 0::numeric),
  venue text NOT NULL,
  locked_at timestamp with time zone,
  created_at timestamp with time zone NOT NULL DEFAULT now(),
  reminder_sent_at timestamp with time zone,
  CONSTRAINT evaluations_pkey PRIMARY KEY (id),
  CONSTRAINT evaluations_internship_id_fkey FOREIGN KEY (internship_id) REFERENCES public.internships(id),
  CONSTRAINT evaluations_evaluator_user_id_fkey FOREIGN KEY (evaluator_user_id) REFERENCES public.users(id)
);
CREATE TABLE public.evaluation_marks (
  mark_id uuid NOT NULL DEFAULT gen_random_uuid(),
  evaluation_id uuid NOT NULL,
  criteria_name text NOT NULL,
  max_marks numeric NOT NULL CHECK (max_marks > 0::numeric),
  marks_obtained numeric NOT NULL CHECK (marks_obtained >= 0::numeric),
  CONSTRAINT evaluation_marks_pkey PRIMARY KEY (mark_id),
  CONSTRAINT evaluation_marks_evaluation_id_fkey FOREIGN KEY (evaluation_id) REFERENCES public.evaluations(id)
);
CREATE TABLE public.marksheets (
  id uuid NOT NULL DEFAULT gen_random_uuid(),
  evaluation_id uuid NOT NULL,
  object_key text NOT NULL UNIQUE,
  download_url text NOT NULL,
  generated_at timestamp with time zone NOT NULL DEFAULT now(),
  url_expires_at timestamp with time zone NOT NULL,
  CONSTRAINT marksheets_pkey PRIMARY KEY (id),
  CONSTRAINT marksheets_evaluation_id_fkey FOREIGN KEY (evaluation_id) REFERENCES public.evaluations(id)
);
CREATE TABLE public.email_outbox (
  id uuid NOT NULL DEFAULT gen_random_uuid(),
  to_address text NOT NULL CHECK (to_address = lower(to_address)),
  subject text NOT NULL,
  body_html text NOT NULL,
  template_name text NOT NULL,
  template_data jsonb NOT NULL DEFAULT '{}'::jsonb,
  status USER-DEFINED NOT NULL DEFAULT 'PENDING'::email_status,
  attempt_count integer NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
  scheduled_at timestamp with time zone NOT NULL DEFAULT now(),
  sent_at timestamp with time zone,
  created_at timestamp with time zone NOT NULL DEFAULT now(),
  CONSTRAINT email_outbox_pkey PRIMARY KEY (id)
);
CREATE TABLE public.event_outbox (
  id uuid NOT NULL DEFAULT gen_random_uuid(),
  topic text NOT NULL CHECK (length(TRIM(BOTH FROM topic)) > 0),
  payload jsonb NOT NULL,
  published_at timestamp with time zone,
  created_at timestamp with time zone NOT NULL DEFAULT now(),
  CONSTRAINT event_outbox_pkey PRIMARY KEY (id)
);
CREATE TABLE public.audit_logs (
  id uuid NOT NULL DEFAULT gen_random_uuid(),
  actor_user_id uuid,
  action text NOT NULL CHECK (length(TRIM(BOTH FROM action)) > 0),
  resource_type text NOT NULL CHECK (length(TRIM(BOTH FROM resource_type)) > 0),
  resource_id text NOT NULL CHECK (length(TRIM(BOTH FROM resource_id)) > 0),
  diff jsonb NOT NULL DEFAULT '{}'::jsonb,
  ip_address inet,
  trace_id text,
  created_at timestamp with time zone NOT NULL DEFAULT now(),
  CONSTRAINT audit_logs_pkey PRIMARY KEY (id),
  CONSTRAINT audit_logs_actor_user_id_fkey FOREIGN KEY (actor_user_id) REFERENCES public.users(id)
);
