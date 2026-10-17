CREATE TABLE user_preferences (
  user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  language TEXT NOT NULL DEFAULT 'ru' CHECK (language IN ('ru','en')),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
