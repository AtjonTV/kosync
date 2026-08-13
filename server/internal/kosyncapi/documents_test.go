//
// File:        internal/kosyncapi/documents_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosyncapi_test

import (
	"net/http"
	"testing"

	"git.obth.eu/atjontv/kosync/internal/schema"
	"git.obth.eu/atjontv/kosync/internal/testutil"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/tests"
)

// restoreUrl addresses the restore endpoint of a document and history entry.
func restoreUrl(documentId, historyId string) string {
	return "/api/kosync/documents/" + documentId + "/restore/" + historyId
}

func TestRestoreHistory(t *testing.T) {
	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:            "an earlier state becomes the current one",
		Method:          http.MethodPost,
		URL:             restoreUrl(testutil.IdDocumentA, testutil.IdHistoryA),
		ExpectedStatus:  http.StatusOK,
		ExpectedContent: []string{"restored"},
		AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
			document, err := app.FindRecordById(schema.CollectionDocuments, testutil.IdDocumentA)
			if err != nil {
				t.Fatalf("failed to reload the document: %v", err)
			}
			if got := document.GetFloat(schema.FieldProgress); got != 0.1 {
				t.Errorf("expected the restored progress of 0.1, got %v", got)
			}

			// The state that was replaced has to be recoverable in turn.
			history, err := app.FindAllRecords(schema.CollectionDocumentHistory,
				dbx.HashExp{"document_ref": testutil.IdDocumentA})
			if err != nil {
				t.Fatalf("failed to load the history: %v", err)
			}
			if len(history) != 1 {
				t.Fatalf("expected exactly one history entry after the restore, got %d", len(history))
			}
			if got := history[0].GetFloat(schema.FieldProgress); got != 0.25 {
				t.Errorf("expected the replaced progress of 0.25 to be archived, got %v", got)
			}
			if history[0].Id == testutil.IdHistoryA {
				t.Errorf("expected the restored entry to leave the history, it is the current state now")
			}
		},
	})
}

func TestRestoreHistoryIsScopedToTheOwner(t *testing.T) {
	asUser(t, testutil.IdUserB, tests.ApiScenario{
		Name:            "a user cannot restore someone else's document",
		Method:          http.MethodPost,
		URL:             restoreUrl(testutil.IdDocumentA, testutil.IdHistoryA),
		ExpectedStatus:  http.StatusNotFound,
		ExpectedContent: []string{`"data":{}`},
		AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
			document, err := app.FindRecordById(schema.CollectionDocuments, testutil.IdDocumentA)
			if err != nil {
				t.Fatalf("failed to reload the document: %v", err)
			}
			if got := document.GetFloat(schema.FieldProgress); got != 0.25 {
				t.Errorf("the foreign document was modified, progress is now %v", got)
			}
		},
	})
}

func TestRestoreHistoryRejectsAMismatchedPair(t *testing.T) {
	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:            "an unknown history entry",
		Method:          http.MethodPost,
		URL:             restoreUrl(testutil.IdDocumentA, testutil.PadId("missing")),
		ExpectedStatus:  http.StatusNotFound,
		ExpectedContent: []string{`"data":{}`},
	})

	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:            "an unknown document",
		Method:          http.MethodPost,
		URL:             restoreUrl(testutil.PadId("missing"), testutil.IdHistoryA),
		ExpectedStatus:  http.StatusNotFound,
		ExpectedContent: []string{`"data":{}`},
	})
}

func TestRestoreHistoryRequiresAuthentication(t *testing.T) {
	asUser(t, "", tests.ApiScenario{
		Name:            "guests cannot restore",
		Method:          http.MethodPost,
		URL:             restoreUrl(testutil.IdDocumentA, testutil.IdHistoryA),
		ExpectedStatus:  http.StatusUnauthorized,
		ExpectedContent: []string{`"data":{}`},
	})
}
