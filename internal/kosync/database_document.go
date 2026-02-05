//
// File:        internal/kosync/database_document.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import (
	"database/sql"
	"errors"
	"time"
)

func (db *Database) FindDocumentById(ownerId, documentId string) (*Document, bool, error) {
	var findOneDocument = `
        SELECT
            id,
            owner_id,
            title,
            current_location,
            progress,
            last_read_on_device,
            last_read_on_device_id,
            last_read_at
        FROM documents
        WHERE id = ? and owner_id = ?;
    `
	rows, err := db.rawDb.Query(findOneDocument, documentId, ownerId)
	if err != nil {
		return nil, false, err
	}
	defer func(rows *sql.Rows) {
		_ = rows.Close()
	}(rows)

	if !rows.Next() {
		return nil, false, nil
	}

	var doc Document
	err = rows.Scan(
		&doc.Id,
		&doc.OwnerId,
		&doc.Title,
		&doc.CurrentLocation,
		&doc.Progress,
		&doc.LastReadOnDevice,
		&doc.LastReadOnDeviceId,
		&doc.LastReadAt,
	)

	return &doc, true, err
}

func (db *Database) AllDocumentsOfUser(ownerId string) ([]Document, error) {
	var findAllDocuments = `
        SELECT
            id,
            owner_id,
            title,
            current_location,
            progress,
            last_read_on_device,
            last_read_on_device_id,
            last_read_at
        FROM documents
        WHERE owner_id = ?;
    `
	rows, err := db.rawDb.Query(findAllDocuments, ownerId)
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) {
		_ = rows.Close()
	}(rows)

	var docs []Document
	for rows.Next() {
		var doc Document
		err = rows.Scan(
			&doc.Id,
			&doc.OwnerId,
			&doc.Title,
			&doc.CurrentLocation,
			&doc.Progress,
			&doc.LastReadOnDevice,
			&doc.LastReadOnDeviceId,
			&doc.LastReadAt,
		)
		if err != nil {
			return nil, err
		}
		docs = append(docs, doc)
	}

	return docs, nil
}

func (db *Database) GetDocumentHistory(ownerId, documentId string) ([]Document, error) {
	var findDocumentHistory = `
        SELECT
            id,
            owner_id,
            title,
            current_location,
            progress,
            last_read_on_device,
            last_read_on_device_id,
            last_read_at
        FROM document_history
        WHERE id = ? and owner_id = ?;
    `
	rows, err := db.rawDb.Query(findDocumentHistory, documentId, ownerId)
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) {
		_ = rows.Close()
	}(rows)

	var docs []Document
	for rows.Next() {
		var doc Document
		err = rows.Scan(
			&doc.Id,
			&doc.OwnerId,
			&doc.Title,
			&doc.CurrentLocation,
			&doc.Progress,
			&doc.LastReadOnDevice,
			&doc.LastReadOnDeviceId,
			&doc.LastReadAt,
		)
		if err != nil {
			return nil, err
		}
		docs = append(docs, doc)
	}
	return docs, nil
}

func (db *Database) CreateOrUpdateDocument(doc *Document) error {
	t, err := db.rawDb.Begin()
	if err != nil {
		return err
	}
	err = db.prepareHistoryCreationInTransaction(t, doc)
	if err != nil {
		rbErr := t.Rollback()
		if rbErr != nil {
			return errors.Join(err, rbErr)
		}
		return err
	}
	var updateDocument = `
        INSERT INTO documents (id, owner_id, title, current_location, progress, last_read_on_device, last_read_on_device_id, last_read_at, created_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT (id, owner_id) DO
            UPDATE SET
                       title = if(length(?), ?, title),
                       current_location = ?,
                       progress = ?,
                       last_read_on_device = ?,
                       last_read_on_device_id = ?,
                       last_read_at = ?,
                       updated_at = ?;
    `
	now := time.Now().Unix()
	_, err = t.Exec(
		updateDocument,
		doc.Id,
		doc.OwnerId,
		doc.Title,
		doc.CurrentLocation,
		doc.Progress,
		doc.LastReadOnDevice,
		doc.LastReadOnDeviceId,
		doc.LastReadAt,
		now,
		doc.Title,
		doc.Title,
		doc.CurrentLocation,
		doc.Progress,
		doc.LastReadOnDevice,
		doc.LastReadOnDeviceId,
		doc.LastReadAt,
		now,
	)
	if err != nil {
		rbErr := t.Rollback()
		if rbErr != nil {
			return errors.Join(err, rbErr)
		}
		return err
	}
	return t.Commit()
}

func (db *Database) prepareHistoryCreationInTransaction(tx *sql.Tx, doc *Document) error {
	_, found, err := db.FindDocumentById(doc.OwnerId, doc.Id)
	if !found || err != nil {
		return err
	}

	var copyToHistory = `
        INSERT INTO document_history (id, owner_id, last_read_at, title, current_location, progress, last_read_on_device, last_read_on_device_id, created_at)
        SELECT
            id,
            owner_id,
            last_read_at,
            title,
            current_location,
            progress,
            last_read_on_device,
            last_read_on_device_id,
            ? as created_at
        FROM documents
        WHERE id = ? AND owner_id = ?;
    `
	now := time.Now().Unix()
	_, err = tx.Exec(copyToHistory, now, doc.Id, doc.OwnerId)
	return err
}
