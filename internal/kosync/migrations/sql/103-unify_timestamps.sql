UPDATE users 
SET 
    created_at = CASE WHEN created_at IS NOT NULL AND created_at < 1000000000000 THEN created_at * 10000 WHEN created_at IS NOT NULL AND created_at < 10000000000000 THEN created_at * 10 ELSE created_at END,
    updated_at = CASE WHEN updated_at IS NOT NULL AND updated_at < 1000000000000 THEN updated_at * 10000 WHEN updated_at IS NOT NULL AND updated_at < 10000000000000 THEN updated_at * 10 ELSE updated_at END,
    deleted_at = CASE WHEN deleted_at IS NOT NULL AND deleted_at < 1000000000000 THEN deleted_at * 10000 WHEN deleted_at IS NOT NULL AND deleted_at < 10000000000000 THEN deleted_at * 10 ELSE deleted_at END;

UPDATE documents 
SET 
    last_read_at = CASE WHEN last_read_at IS NOT NULL AND last_read_at < 1000000000000 THEN last_read_at * 10000 WHEN last_read_at IS NOT NULL AND last_read_at < 10000000000000 THEN last_read_at * 10 ELSE last_read_at END,
    created_at = CASE WHEN created_at IS NOT NULL AND created_at < 1000000000000 THEN created_at * 10000 WHEN created_at IS NOT NULL AND created_at < 10000000000000 THEN created_at * 10 ELSE created_at END,
    updated_at = CASE WHEN updated_at IS NOT NULL AND updated_at < 1000000000000 THEN updated_at * 10000 WHEN updated_at IS NOT NULL AND updated_at < 10000000000000 THEN updated_at * 10 ELSE updated_at END,
    deleted_at = CASE WHEN deleted_at IS NOT NULL AND deleted_at < 1000000000000 THEN deleted_at * 10000 WHEN deleted_at IS NOT NULL AND deleted_at < 10000000000000 THEN deleted_at * 10 ELSE deleted_at END;

UPDATE document_history 
SET 
    last_read_at = CASE WHEN last_read_at IS NOT NULL AND last_read_at < 1000000000000 THEN last_read_at * 10000 WHEN last_read_at IS NOT NULL AND last_read_at < 10000000000000 THEN last_read_at * 10 ELSE last_read_at END,
    created_at = CASE WHEN created_at IS NOT NULL AND created_at < 1000000000000 THEN created_at * 10000 WHEN created_at IS NOT NULL AND created_at < 10000000000000 THEN created_at * 10 ELSE created_at END,
    updated_at = CASE WHEN updated_at IS NOT NULL AND updated_at < 1000000000000 THEN updated_at * 10000 WHEN updated_at IS NOT NULL AND updated_at < 10000000000000 THEN updated_at * 10 ELSE updated_at END,
    deleted_at = CASE WHEN deleted_at IS NOT NULL AND deleted_at < 1000000000000 THEN deleted_at * 10000 WHEN deleted_at IS NOT NULL AND deleted_at < 10000000000000 THEN deleted_at * 10 ELSE deleted_at END;

UPDATE schema_versions 
SET 
    installed_at = CASE WHEN installed_at IS NOT NULL AND installed_at < 1000000000000 THEN installed_at * 10000 WHEN installed_at IS NOT NULL AND installed_at < 10000000000000 THEN installed_at * 10 ELSE installed_at END;
