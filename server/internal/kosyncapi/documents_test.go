//
// File:        internal/kosyncapi/documents_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosyncapi_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"git.obth.eu/atjontv/kosync/internal/schema"
	"git.obth.eu/atjontv/kosync/internal/testutil"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
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

// mergeUrl addresses the merge endpoint.
const mergeUrl = "/api/kosync/documents/merge"

// idDocumentC is a second document of user A, the other half of a split reading.
var idDocumentC = testutil.PadId("docvc")

// withSecondDocument seeds user A a second document, read more recently than the
// one the fixture already gives them.
func withSecondDocument(t testing.TB, app *tests.TestApp, _ *core.ServeEvent) {
	user, err := app.FindRecordById(schema.CollectionUsers, testutil.IdUserA)
	if err != nil {
		t.Fatalf("failed to load the fixture user: %v", err)
	}

	testutil.CreateDocument(t, app, user, idDocumentC, "cccc3333cccc3333cccc3333cccc3333",
		0.75, time.Date(2026, 3, 5, 9, 0, 0, 0, time.UTC))
}

func TestMergeDocuments(t *testing.T) {
	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:            "two documents become one",
		Method:          http.MethodPost,
		URL:             mergeUrl,
		Body:            strings.NewReader(`{"into":"` + testutil.IdDocumentA + `","from":["` + idDocumentC + `"]}`),
		BeforeTestFunc:  withSecondDocument,
		ExpectedStatus:  http.StatusOK,
		ExpectedContent: []string{"2 documents merged into one."},
		AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
			survivor, err := app.FindRecordById(schema.CollectionDocuments, testutil.IdDocumentA)
			if err != nil {
				t.Fatalf("failed to reload the surviving document: %v", err)
			}
			if got := survivor.GetFloat(schema.FieldProgress); got != 0.75 {
				t.Errorf("expected the more recent progress of 0.75, got %v", got)
			}

			if _, err := app.FindRecordById(schema.CollectionDocuments, idDocumentC); err == nil {
				t.Errorf("expected the merged document to be gone")
			}

			// The reading of both, under one document: the fixture's own history
			// entry, the position the survivor was superseded from, and the state
			// of the document that was folded in.
			history, err := app.FindAllRecords(schema.CollectionDocumentHistory,
				dbx.HashExp{"document_ref": testutil.IdDocumentA})
			if err != nil {
				t.Fatalf("failed to load the history: %v", err)
			}
			if len(history) != 2 {
				t.Fatalf("expected two archived states after the merge, got %d", len(history))
			}
		},
	})
}

func TestMergeDocumentsIsScopedToTheOwner(t *testing.T) {
	asUser(t, testutil.IdUserB, tests.ApiScenario{
		Name:            "a user cannot fold someone else's document into their own",
		Method:          http.MethodPost,
		URL:             mergeUrl,
		Body:            strings.NewReader(`{"into":"` + testutil.IdDocumentB + `","from":["` + testutil.IdDocumentA + `"]}`),
		ExpectedStatus:  http.StatusNotFound,
		ExpectedContent: []string{`"data":{}`},
		AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
			document, err := app.FindRecordById(schema.CollectionDocuments, testutil.IdDocumentA)
			if err != nil {
				t.Fatalf("the foreign document was merged away: %v", err)
			}
			if got := document.GetFloat(schema.FieldProgress); got != 0.25 {
				t.Errorf("the foreign document was modified, progress is now %v", got)
			}
		},
	})

	asUser(t, testutil.IdUserB, tests.ApiScenario{
		Name:            "nor merge into it",
		Method:          http.MethodPost,
		URL:             mergeUrl,
		Body:            strings.NewReader(`{"into":"` + testutil.IdDocumentA + `","from":["` + testutil.IdDocumentB + `"]}`),
		ExpectedStatus:  http.StatusNotFound,
		ExpectedContent: []string{`"data":{}`},
	})
}

func TestMergeDocumentsRejectsAnIncompleteRequest(t *testing.T) {
	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:            "nothing to merge into",
		Method:          http.MethodPost,
		URL:             mergeUrl,
		Body:            strings.NewReader(`{"from":["` + idDocumentC + `"]}`),
		ExpectedStatus:  http.StatusBadRequest,
		ExpectedContent: []string{"name the document to keep"},
	})

	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:            "nothing to merge",
		Method:          http.MethodPost,
		URL:             mergeUrl,
		Body:            strings.NewReader(`{"into":"` + testutil.IdDocumentA + `","from":[]}`),
		ExpectedStatus:  http.StatusBadRequest,
		ExpectedContent: []string{"at least one document"},
	})

	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:            "a document into itself",
		Method:          http.MethodPost,
		URL:             mergeUrl,
		Body:            strings.NewReader(`{"into":"` + testutil.IdDocumentA + `","from":["` + testutil.IdDocumentA + `"]}`),
		ExpectedStatus:  http.StatusBadRequest,
		ExpectedContent: []string{"cannot be merged into itself"},
	})
}

func TestMergeDocumentsRequiresAuthentication(t *testing.T) {
	asUser(t, "", tests.ApiScenario{
		Name:            "guests cannot merge",
		Method:          http.MethodPost,
		URL:             mergeUrl,
		Body:            strings.NewReader(`{"into":"` + testutil.IdDocumentA + `","from":["` + idDocumentC + `"]}`),
		ExpectedStatus:  http.StatusUnauthorized,
		ExpectedContent: []string{`"data":{}`},
	})
}
