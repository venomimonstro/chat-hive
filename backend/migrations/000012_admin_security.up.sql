CREATE TABLE admin_users (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE RESTRICT,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','suspended')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE admin_user_roles (
    user_id UUID NOT NULL REFERENCES admin_users(user_id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK (role IN ('support','moderator','senior_moderator','security','legal','system_admin','owner')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, role)
);

CREATE TABLE admin_audit_events (
    id BIGSERIAL PRIMARY KEY,
    actor_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    actor_role TEXT NOT NULL,
    action TEXT NOT NULL,
    target_type TEXT NOT NULL DEFAULT '',
    target_id TEXT NOT NULL DEFAULT '',
    reason TEXT NOT NULL DEFAULT '',
    request_id TEXT NOT NULL DEFAULT '',
    source_ip INET,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX admin_audit_events_actor_created_idx ON admin_audit_events(actor_user_id, created_at DESC);
CREATE INDEX admin_audit_events_action_created_idx ON admin_audit_events(action, created_at DESC);

-- Prevent UPDATE/DELETE at the database permission model layer in production as well.
-- Application code only receives INSERT/SELECT paths for this table.
