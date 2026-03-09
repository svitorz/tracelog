-- Bootstrap: Default Applications and Sources for Local Development
-- Run after 000001_initial_schema.sql

BEGIN;

-- Default monitored applications for MVP
INSERT INTO monitored_application (name, service_key, description)
VALUES
    ('Sample Web API', 'sample-web-api', 'Default sample application for testing and demonstration'),
    ('Background Worker', 'background-worker', 'Sample background job processor'),
    ('Frontend App', 'frontend-app', 'Sample frontend application')
ON CONFLICT (service_key) DO NOTHING;

-- Default event sources for MVP
INSERT INTO event_source (name, source_type, description)
VALUES
    ('HTTP API', 'http', 'Events received via HTTP REST API'),
    ('CLI Tool', 'cli', 'Events registered via command-line interface'),
    ('File Import', 'file', 'Events imported from log files'),
    ('System Monitor', 'system', 'System-generated monitoring events')
ON CONFLICT (name, source_type) DO NOTHING;

COMMIT;
