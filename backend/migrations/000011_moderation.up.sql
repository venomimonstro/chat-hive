CREATE TABLE reports (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    reporter_id UUID REFERENCES users(id) ON DELETE SET NULL,
    target_type TEXT NOT NULL CHECK (target_type IN ('user','message','post','group')),
    target_id UUID NOT NULL,
    category TEXT NOT NULL CHECK (category IN ('spam','scam','harassment','threats','illegal_content','sexual_content','impersonation','malware','other')),
    note TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open','triaged','resolved','dismissed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX reports_status_created_idx ON reports(status, created_at ASC);
CREATE INDEX reports_target_idx ON reports(target_type, target_id, created_at DESC);
CREATE INDEX reports_reporter_idx ON reports(reporter_id, created_at DESC);

CREATE TABLE moderation_cases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    target_type TEXT NOT NULL CHECK (target_type IN ('user','message','post','group')),
    target_id UUID NOT NULL,
    severity TEXT NOT NULL DEFAULT 'medium' CHECK (severity IN ('low','medium','high','critical')),
    status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open','reviewing','resolved','dismissed')),
    reason TEXT NOT NULL DEFAULT '',
    assigned_to UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at TIMESTAMPTZ
);

CREATE INDEX moderation_cases_queue_idx ON moderation_cases(status, severity, created_at ASC);
CREATE INDEX moderation_cases_target_idx ON moderation_cases(target_type, target_id, created_at DESC);

CREATE TABLE moderation_case_reports (
    case_id UUID NOT NULL REFERENCES moderation_cases(id) ON DELETE CASCADE,
    report_id UUID NOT NULL REFERENCES reports(id) ON DELETE CASCADE,
    PRIMARY KEY (case_id, report_id)
);

CREATE TABLE account_restrictions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    restriction TEXT NOT NULL CHECK (restriction IN ('message_new_users','publish','create_groups','login')),
    reason TEXT NOT NULL,
    source_case_id UUID REFERENCES moderation_cases(id) ON DELETE SET NULL,
    expires_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX account_restrictions_active_idx ON account_restrictions(user_id, restriction)
    WHERE revoked_at IS NULL;

CREATE TABLE moderation_actions (
    id BIGSERIAL PRIMARY KEY,
    case_id UUID REFERENCES moderation_cases(id) ON DELETE SET NULL,
    actor_id UUID,
    action TEXT NOT NULL CHECK (action IN ('case_created','assigned','content_hidden','content_restored','restriction_added','restriction_revoked','account_suspended','account_restored','case_resolved','case_dismissed')),
    target_type TEXT NOT NULL,
    target_id UUID NOT NULL,
    reason TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX moderation_actions_case_idx ON moderation_actions(case_id, created_at DESC);
CREATE INDEX moderation_actions_target_idx ON moderation_actions(target_type, target_id, created_at DESC);
