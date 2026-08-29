-- Courses are tracked by lesson and never come from a catalog.
INSERT INTO type_settings (user_id, kind, depth, step) VALUES (1, 'course', 'units', 1)
ON CONFLICT (user_id, kind) DO NOTHING;
