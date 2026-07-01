ALTER TABLE content_items ADD COLUMN ai_summary_markdown TEXT NOT NULL DEFAULT '';
ALTER TABLE content_items ADD COLUMN ai_summary_status TEXT NOT NULL DEFAULT 'none' CHECK (ai_summary_status IN ('none', 'pending', 'success', 'failed', 'insufficient_text'));
ALTER TABLE content_items ADD COLUMN ai_summary_generated_at DATETIME NULL;
ALTER TABLE content_items ADD COLUMN ai_summary_error TEXT NOT NULL DEFAULT '';
