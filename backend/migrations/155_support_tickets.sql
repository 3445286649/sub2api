CREATE TABLE IF NOT EXISTS support_tickets (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title VARCHAR(120) NOT NULL,
    category VARCHAR(30) NOT NULL DEFAULT 'general',
    status VARCHAR(30) NOT NULL DEFAULT 'pending_admin',
    priority VARCHAR(20) NOT NULL DEFAULT 'normal',
    last_message_at TIMESTAMPTZ,
    last_user_message_at TIMESTAMPTZ,
    last_admin_message_at TIMESTAMPTZ,
    user_last_read_at TIMESTAMPTZ,
    admin_last_read_at TIMESTAMPTZ,
    assigned_admin_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    closed_at TIMESTAMPTZ,
    closed_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS support_ticket_messages (
    id BIGSERIAL PRIMARY KEY,
    ticket_id BIGINT NOT NULL REFERENCES support_tickets(id) ON DELETE CASCADE,
    sender_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    sender_role VARCHAR(20) NOT NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_support_tickets_user_id ON support_tickets(user_id);
CREATE INDEX IF NOT EXISTS idx_support_tickets_status ON support_tickets(status);
CREATE INDEX IF NOT EXISTS idx_support_tickets_category ON support_tickets(category);
CREATE INDEX IF NOT EXISTS idx_support_tickets_priority ON support_tickets(priority);
CREATE INDEX IF NOT EXISTS idx_support_tickets_assigned_admin_id ON support_tickets(assigned_admin_id);
CREATE INDEX IF NOT EXISTS idx_support_tickets_last_message_at ON support_tickets(last_message_at DESC);
CREATE INDEX IF NOT EXISTS idx_support_tickets_last_user_message_at ON support_tickets(last_user_message_at DESC);
CREATE INDEX IF NOT EXISTS idx_support_tickets_last_admin_message_at ON support_tickets(last_admin_message_at DESC);
CREATE INDEX IF NOT EXISTS idx_support_ticket_messages_ticket_id ON support_ticket_messages(ticket_id);
CREATE INDEX IF NOT EXISTS idx_support_ticket_messages_ticket_id_id ON support_ticket_messages(ticket_id, id DESC);
CREATE INDEX IF NOT EXISTS idx_support_ticket_messages_ticket_id_created_at ON support_ticket_messages(ticket_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_support_ticket_messages_sender_id ON support_ticket_messages(sender_id);
