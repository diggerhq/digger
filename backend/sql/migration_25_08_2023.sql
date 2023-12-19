INSERT INTO public.repos
SELECT id, created_at, updated_at, deleted_at, name, organisation_id
FROM public.namespaces;

UPDATE public.projects
SET repo_id = namespace_id;

UPDATE public.policies
SET repo_id = namespace_id;
