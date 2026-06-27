-- Model radar daily benchmark results and settings.

CREATE TABLE IF NOT EXISTS model_radar_runs (
  id BIGSERIAL PRIMARY KEY,
  run_date DATE NOT NULL,
  trigger_type VARCHAR(16) NOT NULL DEFAULT 'scheduled',
  status VARCHAR(16) NOT NULL DEFAULT 'pending',
  published BOOLEAN NOT NULL DEFAULT FALSE,
  started_at TIMESTAMPTZ,
  finished_at TIMESTAMPTZ,
  total_combinations INTEGER NOT NULL DEFAULT 0,
  success_combinations INTEGER NOT NULL DEFAULT 0,
  error_message TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_model_radar_runs_scheduled_day
  ON model_radar_runs (run_date)
  WHERE trigger_type = 'scheduled';

CREATE INDEX IF NOT EXISTS idx_model_radar_runs_published_date
  ON model_radar_runs (published, run_date DESC, id DESC);

CREATE TABLE IF NOT EXISTS model_radar_results (
  id BIGSERIAL PRIMARY KEY,
  run_id BIGINT NOT NULL REFERENCES model_radar_runs(id) ON DELETE CASCADE,
  model VARCHAR(128) NOT NULL,
  reasoning_effort VARCHAR(32) NOT NULL,
  score INTEGER NOT NULL DEFAULT 0,
  pass_count INTEGER NOT NULL DEFAULT 0,
  total_count INTEGER NOT NULL DEFAULT 0,
  avg_latency_ms INTEGER,
  error_count INTEGER NOT NULL DEFAULT 0,
  status VARCHAR(16) NOT NULL DEFAULT 'pending',
  rank INTEGER NOT NULL DEFAULT 0,
  error_message TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (run_id, model, reasoning_effort)
);

CREATE INDEX IF NOT EXISTS idx_model_radar_results_run_rank
  ON model_radar_results (run_id, rank ASC, score DESC);

CREATE TABLE IF NOT EXISTS model_radar_task_results (
  id BIGSERIAL PRIMARY KEY,
  result_id BIGINT NOT NULL REFERENCES model_radar_results(id) ON DELETE CASCADE,
  task_id VARCHAR(64) NOT NULL,
  task_version INTEGER NOT NULL DEFAULT 1,
  passed BOOLEAN NOT NULL DEFAULT FALSE,
  expected_answer TEXT NOT NULL DEFAULT '',
  actual_answer TEXT NOT NULL DEFAULT '',
  latency_ms INTEGER,
  error_message TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (result_id, task_id)
);

INSERT INTO settings (key, value, updated_at)
VALUES
  ('model_radar_enabled', 'false', NOW()),
  ('model_radar_config', '', NOW())
ON CONFLICT (key) DO NOTHING;
