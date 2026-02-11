//
// File:        internal/kosync/database_document.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import (
	"database/sql"
	"errors"
)

var logDbDoc = NewKlog("db/document")

func (db *Database) FindDocumentById(ownerId, documentId string) (*Document, bool, error) {
	logDbDoc.Debug("FindDocumentById(userId='%s', documentId='%s')", ownerId, documentId)
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
		logDbDoc.Error("Failed to find document '%s' of user '%s': %v", documentId, ownerId, err.Error())
		return nil, false, err
	}
	defer func(rows *sql.Rows) {
		_ = rows.Close()
	}(rows)

	if !rows.Next() {
		logDbDoc.Debug("No document found")
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

	logDbDoc.Debug("Found document")
	return &doc, true, err
}

func (db *Database) AllDocumentsOfUser(ownerId string) ([]Document, error) {
	logDbDoc.Debug("AllDocumentsOfUser(ownerId='%s')", ownerId)
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
		logDbDoc.Error("Failed to find all documents of user '%s': %v", ownerId, err.Error())
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
			logDbDoc.Error("Failed to scan document: %v", err.Error())
			return nil, err
		}
		docs = append(docs, doc)
	}

	logDbDoc.Debug("Found %d documents", len(docs))
	return docs, nil
}

func (db *Database) GetDocumentHistory(ownerId, documentId string) ([]Document, error) {
	logDbDoc.Debug("GetDocumentHistory(ownerId='%s', documentId='%s')", ownerId, documentId)
	var findDocumentHistory = `
        SELECT
            document_id,
            owner_id,
            title,
            current_location,
            progress,
            last_read_on_device,
            last_read_on_device_id,
            last_read_at
        FROM document_history
        WHERE document_id = ? and owner_id = ?;
    `
	rows, err := db.rawDb.Query(findDocumentHistory, documentId, ownerId)
	if err != nil {
		logDbDoc.Error("Failed to find history for document '%s' of user '%s': %v", documentId, ownerId, err.Error())
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
			logDbDoc.Error("Failed to scan document: %v", err.Error())
			return nil, err
		}
		docs = append(docs, doc)
	}
	logDbDoc.Debug("Found %d history entries", len(docs))
	return docs, nil
}

func (db *Database) CreateOrUpdateDocument(doc *Document) error {
	logDbDoc.Debug("CreateOrUpdateDocument(document={Id: '%s'})", doc.Id)
	t, err := db.rawDb.Begin()
	if err != nil {
		logDbDoc.Error("Failed to start transaction: %v", err.Error())
		return err
	}
	err = db.prepareHistoryCreationInTransaction(t, doc)
	if err != nil {
		logDbDoc.Error("Failed to prepare history creation: %v", err.Error())
		logDbDoc.Debug("Rolling back transaction")
		rbErr := t.Rollback()
		if rbErr != nil {
			logDbDoc.Error("Failed to rollback transaction: %v", rbErr.Error())
			return errors.Join(err, rbErr)
		}
		return err
	}
	var updateDocument = `
        INSERT INTO documents (id, owner_id, title, current_location, progress, last_read_on_device, last_read_on_device_id, last_read_at, created_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, (unixepoch('subsec')*1000))
        ON CONFLICT (id, owner_id) DO
            UPDATE SET
                       title = if(length(?), ?, title),
                       current_location = ?,
                       progress = ?,
                       last_read_on_device = ?,
                       last_read_on_device_id = ?,
                       last_read_at = if(? = unixepoch(), unixepoch('subsec')*1000, ?),
                       updated_at = (unixepoch('subsec')*1000);
    `
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
		doc.Title,
		doc.Title,
		doc.CurrentLocation,
		doc.Progress,
		doc.LastReadOnDevice,
		doc.LastReadOnDeviceId,
		doc.LastReadAt,
		doc.LastReadAt,
		doc.LastReadAt,
		doc.LastReadAt,
	)
	if err != nil {
		logDbDoc.Error("Failed to update document: %v", err.Error())
		logDbDoc.Debug("Rolling back transaction")
		rbErr := t.Rollback()
		if rbErr != nil {
			logDbDoc.Error("Failed to rollback transaction: %v", rbErr.Error())
			return errors.Join(err, rbErr)
		}
		return err
	}
	logDbDoc.Debug("Successfully updated document")
	return t.Commit()
}

func (db *Database) prepareHistoryCreationInTransaction(tx *sql.Tx, doc *Document) error {
	logDbDoc.Debug("prepareHistoryCreationInTransaction(Tx, document={Id: '%s'})", doc.Id)
	_, found, err := db.FindDocumentById(doc.OwnerId, doc.Id)
	if err != nil {
		logDbDoc.Error("Failed to find document '%s' of user '%s': %v", doc.Id, doc.OwnerId, err.Error())
		return err
	}
	if !found {
		return nil
	}

	var copyToHistory = `
        INSERT INTO document_history (document_id, owner_id, last_read_at, title, current_location, progress, last_read_on_device, last_read_on_device_id, created_at)
        SELECT
            id,
            owner_id,
            last_read_at,
            title,
            current_location,
            progress,
            last_read_on_device,
            last_read_on_device_id,
            (unixepoch('subsec')*1000) as created_at
        FROM documents
        WHERE id = ? AND owner_id = ?;
    `
	_, err = tx.Exec(copyToHistory, doc.Id, doc.OwnerId)
	if err != nil {
		logDbDoc.Error("Failed to copy document to history: %v", err.Error())
	}
	return err
}

func (db *Database) AllDocumentsOfUserWithHistory(ownerId string) (*[]DocumentWithHistory, error) {
	docs, err := db.AllDocumentsOfUser(ownerId)
	if err != nil {
		logApiWeb.Error("Failed to get documents for user '%s': %v", ownerId, err.Error())
		return nil, err
	}
	result := make([]DocumentWithHistory, 0, len(docs))
	for i := range docs {
		history, err := db.GetDocumentHistory(ownerId, docs[i].Id)
		if err != nil {
			logApiWeb.Error("Failed to get history for document '%s': %v", docs[i].Id, err.Error())
			continue
		}

		result = append(result, DocumentWithHistory{Document: docs[i], History: history})
	}
	return &result, nil
}
