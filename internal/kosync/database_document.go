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
        WHERE id = ? and owner_id = ? AND deleted_at IS NULL;
    `
	rows, err := db.rawDb.Query(findOneDocument, documentId, ownerId)
	if err != nil {
		logDbDoc.Error("Failed to find document '%s' of user '%s': %v", documentId, ownerId, err.Error())
		return nil, false, err
	}
	defer func(rows *sql.Rows) {
		_ = rows.Close()
	}(rows)

	docs, err := scanDocumentsFromRows(rows)
	if err != nil {
		return nil, false, err
	}

	if len(*docs) == 0 {
		logDbDoc.Debug("No document found")
		return nil, false, nil
	} else if len(*docs) > 1 {
		logDbDoc.Error("Found more than one document when only one was expected")
		return nil, false, ErrFoundMoreThanOneResultWhenOnlyOneExpected
	}

	logDbDoc.Debug("Found document")
	return &(*docs)[0], true, err
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
        WHERE owner_id = ? AND deleted_at IS NULL;
    `
	rows, err := db.rawDb.Query(findAllDocuments, ownerId)
	if err != nil {
		logDbDoc.Error("Failed to find all documents of user '%s': %v", ownerId, err.Error())
		return nil, err
	}
	defer func(rows *sql.Rows) {
		_ = rows.Close()
	}(rows)

	docs, err := scanDocumentsFromRows(rows)
	if err != nil {
		logDbDoc.Error("Failed to scan document: %v", err.Error())
		return nil, err
	}

	logDbDoc.Debug("Found %d documents", len(*docs))
	return *docs, nil
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
        WHERE document_id = ? and owner_id = ? AND deleted_at IS NULL;
    `
	rows, err := db.rawDb.Query(findDocumentHistory, documentId, ownerId)
	if err != nil {
		logDbDoc.Error("Failed to find history for document '%s' of user '%s': %v", documentId, ownerId, err.Error())
		return nil, err
	}
	defer func(rows *sql.Rows) {
		_ = rows.Close()
	}(rows)

	docs, err := scanDocumentsFromRows(rows)
	if err != nil {
		logDbDoc.Error("Failed to scan document: %v", err.Error())
		return nil, err
	}

	logDbDoc.Debug("Found %d history entries", len(*docs))
	return *docs, nil
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
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, (unixepoch('subsec')*1000))
        ON CONFLICT (id, owner_id) DO
            UPDATE SET
                       title = if(length($3), $3, title),
                       current_location = $4,
                       progress = $5,
                       last_read_on_device = $6,
                       last_read_on_device_id = $7,
                       last_read_at = if($8 = unixepoch(), unixepoch('subsec')*1000, $8),
                       updated_at = (unixepoch('subsec')*1000),
                       deleted_at = null;
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

func (db *Database) DeleteDocumentHistoryItem(ownerId, documentId string, lastReadAt int64) error {
	logDbDoc.Debug("DeleteDocumentHistoryItem(ownerId='%s', documentId='%s', lastReadAt=%d)", ownerId, documentId, lastReadAt)
	var deleteHistoryItem = `
        UPDATE document_history
        SET deleted_at = (unixepoch('subsec')*1000)
        WHERE document_id = ? AND owner_id = ? AND last_read_at = ?;
    `
	_, err := db.rawDb.Exec(deleteHistoryItem, documentId, ownerId, lastReadAt)
	if err != nil {
		logDbDoc.Error("Failed to delete history item of document '%s' of user '%s': %v", documentId, ownerId, err.Error())
		return err
	}

	logDbDoc.Debug("Successfully deleted history item")
	return nil
}

func (db *Database) DeleteDocumentById(ownerId, documentId string) error {
	logDbDoc.Debug("DeleteDocumentById(ownerId='%s', documentId='%s')", ownerId, documentId)
	var deleteDocument = `
        UPDATE documents
        SET deleted_at = (unixepoch('subsec')*1000)
        WHERE id = ? AND owner_id = ?;
    `
	_, err := db.rawDb.Exec(deleteDocument, documentId, ownerId)
	if err != nil {
		logDbDoc.Error("Failed to delete document '%s' of user '%s': %v", documentId, ownerId, err.Error())
		return err
	}

	logDbDoc.Debug("Successfully deleted document")
	return nil
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

func (db *Database) AllDocumentsOfUserWithHistory(ownerId string) (result *[]DocumentWithHistory, err error) {
	docs, err := db.AllDocumentsOfUser(ownerId)
	if err != nil {
		logApiWeb.Error("Failed to get documents for user '%s': %v", ownerId, err.Error())
		return nil, err
	}
	result = new([]DocumentWithHistory)
	for i := range docs {
		history, err := db.GetDocumentHistory(ownerId, docs[i].Id)
		if err != nil {
			logApiWeb.Error("Failed to get history for document '%s': %v", docs[i].Id, err.Error())
			continue
		}

		*result = append(*result, DocumentWithHistory{Document: docs[i], History: history})
	}
	return
}

func scanDocumentsFromRows(rows *sql.Rows) (docs *[]Document, err error) {
	docs = new([]Document)
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
		*docs = append(*docs, doc)
	}
	return docs, nil
}
