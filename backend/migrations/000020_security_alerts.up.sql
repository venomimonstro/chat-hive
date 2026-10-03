CREATE TABLE security_alerts (
    event_id BIGINT PRIMARY KEY REFERENCES security_events(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open','acknowledged','resolved')),
    acknowledged_by UUID REFERENCES admin_users(user_id) ON DELETE SET NULL,
    acknowledged_at TIMESTAMPTZ,
    note TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX security_alerts_status_created_idx
ON security_alerts(status, created_at DESC);

CREATE OR REPLACE FUNCTION route_security_alert()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF NEW.severity IN ('high','critical') THEN
        INSERT INTO security_alerts(event_id, status, created_at, updated_at)
        VALUES(NEW.id, 'open', NEW.created_at, NEW.created_at)
        ON CONFLICT (event_id) DO NOTHING;
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER security_events_route_alert
AFTER INSERT ON security_events
FOR EACH ROW EXECUTE FUNCTION route_security_alert();
