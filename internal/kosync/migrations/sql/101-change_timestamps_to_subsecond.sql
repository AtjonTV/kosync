UPDATE documents SET last_read_at = (last_read_at * 10000) WHERE true;
UPDATE document_history SET last_read_at = (last_read_at * 10000) WHERE true;
