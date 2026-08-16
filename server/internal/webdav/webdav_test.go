//
// File:        internal/webdav/webdav_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package webdav_test

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"git.obth.eu/atjontv/kosync/internal/config"
	"git.obth.eu/atjontv/kosync/internal/koreader"
	"git.obth.eu/atjontv/kosync/internal/testutil"
	"git.obth.eu/atjontv/kosync/internal/webdav"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
	"github.com/pocketbase/pocketbase/tools/hook"

	_ "modernc.org/sqlite"
)

const syncURL = "/webdav/" + webdav.FileName

// basicAuth returns the header a WebDAV client sends.
func basicAuth(username, password string) map[string]string {
	return map[string]string{
		"Authorization": "Basic " + base64.StdEncoding.EncodeToString([]byte(username+":"+password)),
	}
}

// newFactory builds a fresh app per scenario, since routes are registered when
// one starts.
func newFactory() func(testing.TB) *tests.TestApp {
	return func(t testing.TB) *tests.TestApp {
		app := testutil.NewApp(t)

		conf := &config.Config{KoreaderAuthCacheTtl: 300, EnableWebdav: true}
		conf.Normalize()

		sync := koreader.Register(app, conf)
		webdav.Register(app, conf, sync)

		testutil.Seed(t, app)

		return app
	}
}

// statisticsBytes builds a database with the shape KOReader's statistics plugin
// produces.
//
// Built rather than checked in: the reference database is somebody's reading
// history, which is not a thing to keep in a repository, and what is being
// tested is the shape rather than the contents.
func statisticsBytes(t testing.TB) []byte {
	t.Helper()

	path := filepath.Join(t.TempDir(), "built.sqlite3")

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	schema := []string{
		`CREATE TABLE book (id integer PRIMARY KEY autoincrement, title text, authors text,
			notes integer, last_open integer, highlights integer, pages integer,
			series text, language text, md5 text, total_read_time integer, total_read_pages integer)`,
		`CREATE TABLE page_stat_data (id_book integer, page integer NOT NULL DEFAULT 0,
			start_time integer NOT NULL DEFAULT 0, duration integer NOT NULL DEFAULT 0,
			total_pages integer NOT NULL DEFAULT 0, UNIQUE (id_book, page, start_time))`,
		`INSERT INTO book (title, md5, pages, total_read_time, total_read_pages)
			VALUES ('Zeit des Sturms', '043f11771ef9d191364ac0ba08198d36', 668, 52920, 668)`,
		`INSERT INTO page_stat_data (id_book, page, start_time, duration, total_pages)
			VALUES (1, 12, 1767181448, 74, 668)`,
	}
	for _, statement := range schema {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("build the statistics database: %v", err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	content, err := os.ReadFile(path) // #nosec G304 -- built above
	if err != nil {
		t.Fatalf("read back: %v", err)
	}

	return content
}

// stored returns what the server kept for an account.
func stored(t testing.TB, app *tests.TestApp, owner string) ([]byte, bool) {
	t.Helper()

	path := filepath.Join(app.DataDir(), "webdav", owner, webdav.FileName)
	content, err := os.ReadFile(path) // #nosec G304 -- built above
	if os.IsNotExist(err) {
		return nil, false
	}
	if err != nil {
		t.Fatalf("read the stored file: %v", err)
	}

	return content, true
}

func TestADeviceCanUploadItsStatistics(t *testing.T) {
	content := statisticsBytes(t)

	scenario := tests.ApiScenario{
		Name:            "the statistics database arrives over WebDAV",
		Method:          http.MethodPut,
		URL:             syncURL,
		Body:            bytes.NewReader(content),
		Headers:         basicAuth(testutil.KoUsernameA, testutil.KoPasswordA),
		TestAppFactory:  newFactory(),
		ExpectedStatus:  http.StatusCreated,
		ExpectedContent: []string{"Created"},
		AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
			kept, ok := stored(t, app, testutil.IdUserA)
			if !ok {
				t.Fatal("expected the upload to be kept")
			}
			// Byte for byte: KOReader downloads this copy and merges its own
			// into it, so anything the server rewrote would be a merge against
			// something the device never wrote.
			if !bytes.Equal(kept, content) {
				t.Errorf("the stored file is %d bytes, the upload was %d", len(kept), len(content))
			}
		},
	}
	scenario.Test(t)
}

// The round trip is the whole point: the plugin fetches the remote copy, merges
// its own history into it and puts the result back.
func TestTheStoredFileCanBeFetchedBack(t *testing.T) {
	content := statisticsBytes(t)
	factory := newFactory()

	scenario := tests.ApiScenario{
		Name:           "what was uploaded can be downloaded",
		Method:         http.MethodGet,
		URL:            syncURL,
		Headers:        basicAuth(testutil.KoUsernameA, testutil.KoPasswordA),
		TestAppFactory: factory,
		BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
			put(t, app, testutil.IdUserA, content)
		},
		ExpectedStatus:  http.StatusOK,
		ExpectedContent: []string{"SQLite format 3"},
	}
	scenario.Test(t)
}

func TestNothingButAStatisticsDatabaseIsKept(t *testing.T) {
	scenario := tests.ApiScenario{
		Name:           "a file that is not a statistics database is refused",
		Method:         http.MethodPut,
		URL:            syncURL,
		Body:           bytes.NewReader([]byte("this is not a database at all, it is a note")),
		Headers:        basicAuth(testutil.KoUsernameA, testutil.KoPasswordA),
		TestAppFactory: newFactory(),
		// The library answers a failed write with 405. What matters is that
		// nothing was kept.
		ExpectedStatus:  http.StatusMethodNotAllowed,
		ExpectedContent: []string{"Method Not Allowed"},
		AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
			if _, ok := stored(t, app, testutil.IdUserA); ok {
				t.Error("a file that is not a statistics database was kept")
			}
		},
	}
	scenario.Test(t)
}

// A real SQLite file with somebody else's schema is the case the header check
// alone would let through.
func TestADatabaseOfTheWrongShapeIsRefused(t *testing.T) {
	path := filepath.Join(t.TempDir(), "other.sqlite3")

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE notes (id integer, body text)`); err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	content, err := os.ReadFile(path) // #nosec G304 -- built above
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	scenario := tests.ApiScenario{
		Name:            "another application's database is refused",
		Method:          http.MethodPut,
		URL:             syncURL,
		Body:            bytes.NewReader(content),
		Headers:         basicAuth(testutil.KoUsernameA, testutil.KoPasswordA),
		TestAppFactory:  newFactory(),
		ExpectedStatus:  http.StatusMethodNotAllowed,
		ExpectedContent: []string{"Method Not Allowed"},
		AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
			if _, ok := stored(t, app, testutil.IdUserA); ok {
				t.Error("a database of the wrong shape was kept")
			}
		},
	}
	scenario.Test(t)
}

// The name is the other half of not being a file host.
func TestAnyOtherNameIsRefused(t *testing.T) {
	for _, name := range []string{"holiday.jpg", "statistics.sqlite3.bak", "notes/statistics.sqlite3"} {
		scenario := tests.ApiScenario{
			Name:            "uploading " + name + " is refused",
			Method:          http.MethodPut,
			URL:             "/webdav/" + name,
			Body:            bytes.NewReader(statisticsBytes(t)),
			Headers:         basicAuth(testutil.KoUsernameA, testutil.KoPasswordA),
			TestAppFactory:  newFactory(),
			ExpectedStatus:  http.StatusNotFound,
			ExpectedContent: []string{"Not Found"},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				if _, ok := stored(t, app, testutil.IdUserA); ok {
					t.Error("a refused name was kept under the allowed one")
				}
			},
		}
		scenario.Test(t)
	}
}

// The path is cleaned before it is compared, so climbing out lands on a name
// that is not the one allowed rather than on a directory above the root.
//
// The traversal is percent encoded on purpose. A plain "../.." never reaches
// this package at all — the router normalises it and answers with a redirect —
// so a test written that way would be checking net/http rather than the store.
func TestClimbingOutOfTheDirectoryIsRefused(t *testing.T) {
	scenario := tests.ApiScenario{
		Name:            "a traversal is refused",
		Method:          http.MethodPut,
		URL:             "/webdav/..%2f..%2fescaped.sqlite3",
		Body:            bytes.NewReader(statisticsBytes(t)),
		Headers:         basicAuth(testutil.KoUsernameA, testutil.KoPasswordA),
		TestAppFactory:  newFactory(),
		ExpectedStatus:  http.StatusNotFound,
		ExpectedContent: []string{"Not Found"},
		AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
			for _, where := range []string{
				filepath.Join(app.DataDir(), "escaped.sqlite3"),
				filepath.Join(app.DataDir(), "webdav", "escaped.sqlite3"),
			} {
				if _, err := os.Stat(where); err == nil {
					t.Fatalf("a file was written to %s", where)
				}
			}
		},
	}
	scenario.Test(t)
}

func TestTheEndpointNeedsADeviceCredential(t *testing.T) {
	scenarios := []tests.ApiScenario{
		{
			Name:            "no credential at all",
			Method:          http.MethodGet,
			URL:             syncURL,
			TestAppFactory:  newFactory(),
			ExpectedStatus:  http.StatusUnauthorized,
			ExpectedContent: []string{`"status":401`},
		},
		{
			Name:            "the wrong password",
			Method:          http.MethodGet,
			URL:             syncURL,
			Headers:         basicAuth(testutil.KoUsernameA, "not-the-password"),
			TestAppFactory:  newFactory(),
			ExpectedStatus:  http.StatusUnauthorized,
			ExpectedContent: []string{`"status":401`},
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}

// One directory per account, and no way to name another one: the path a device
// sends never says whose file it means.
func TestOneAccountCannotReachAnothersFile(t *testing.T) {
	content := statisticsBytes(t)

	scenario := tests.ApiScenario{
		Name:           "the other account's upload is not there",
		Method:         http.MethodGet,
		URL:            syncURL,
		Headers:        basicAuth(testutil.KoUsernameB, testutil.KoPasswordB),
		TestAppFactory: newFactory(),
		BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
			put(t, app, testutil.IdUserA, content)
		},
		ExpectedStatus:  http.StatusNotFound,
		ExpectedContent: []string{"Not Found"},
	}
	scenario.Test(t)
}

func TestAnUploadPastTheLimitIsRefused(t *testing.T) {
	scenario := tests.ApiScenario{
		Name:   "a database larger than the limit is refused",
		Method: http.MethodPut,
		URL:    syncURL,
		Body:   bytes.NewReader(bytes.Repeat([]byte("x"), 3*1024*1024)),
		TestAppFactory: func(t testing.TB) *tests.TestApp {
			app := testutil.NewApp(t)

			conf := &config.Config{KoreaderAuthCacheTtl: 300, EnableWebdav: true, WebdavMaxMegabytes: 1}
			conf.Normalize()

			sync := koreader.Register(app, conf)
			webdav.Register(app, conf, sync)
			testutil.Seed(t, app)

			return app
		},
		Headers:         basicAuth(testutil.KoUsernameA, testutil.KoPasswordA),
		ExpectedStatus:  http.StatusMethodNotAllowed,
		ExpectedContent: []string{"Method Not Allowed"},
		AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
			if _, ok := stored(t, app, testutil.IdUserA); ok {
				t.Error("an oversized upload was kept")
			}
		},
	}
	scenario.Test(t)
}

// The endpoint can be turned off entirely.
func TestTheEndpointCanBeDisabled(t *testing.T) {
	scenario := tests.ApiScenario{
		Name:   "nothing is mounted when it is off",
		Method: http.MethodGet,
		URL:    syncURL,
		TestAppFactory: func(t testing.TB) *tests.TestApp {
			app := testutil.NewApp(t)

			conf := &config.Config{KoreaderAuthCacheTtl: 300}
			conf.Normalize()

			sync := koreader.Register(app, conf)
			webdav.Register(app, conf, sync)
			testutil.Seed(t, app)

			return app
		},
		Headers:         basicAuth(testutil.KoUsernameA, testutil.KoPasswordA),
		ExpectedStatus:  http.StatusNotFound,
		ExpectedContent: []string{`"status":404`},
	}
	scenario.Test(t)
}

// put writes a stored file directly, for the tests that need one to already be
// there.
func put(t testing.TB, app *tests.TestApp, owner string, content []byte) {
	t.Helper()

	dir := filepath.Join(app.DataDir(), "webdav", owner)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("prepare the directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, webdav.FileName), content, 0o600); err != nil {
		t.Fatalf("write the file: %v", err)
	}
}

// PROPFIND is what a client asks first: is there a remote copy, how big, and
// how old. KOReader decides whether to download and merge from the answer, so
// this is the request that has to work before any of the others matter.
func TestTheDirectoryListsTheOneFile(t *testing.T) {
	content := statisticsBytes(t)

	scenario := tests.ApiScenario{
		Name:   "a listing shows the stored database",
		Method: "PROPFIND",
		URL:    "/webdav/",
		Headers: withHeader(basicAuth(testutil.KoUsernameA, testutil.KoPasswordA),
			"Depth", "1"),
		TestAppFactory: newFactory(),
		BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
			put(t, app, testutil.IdUserA, content)
		},
		ExpectedStatus: http.StatusMultiStatus,
		ExpectedContent: []string{
			"<D:multistatus",
			webdav.FileName,
			"<D:getcontentlength>" + strconv.Itoa(len(content)),
		},
	}
	scenario.Test(t)
}

// An empty directory is a valid answer and not an error: it is what a device
// syncing for the first time sees, and it has to mean "nothing here yet"
// rather than "this is not a WebDAV server".
func TestAnEmptyDirectoryStillLists(t *testing.T) {
	scenario := tests.ApiScenario{
		Name:   "the first sync finds an empty directory",
		Method: "PROPFIND",
		URL:    "/webdav/",
		Headers: withHeader(basicAuth(testutil.KoUsernameA, testutil.KoPasswordA),
			"Depth", "1"),
		TestAppFactory:     newFactory(),
		ExpectedStatus:     http.StatusMultiStatus,
		ExpectedContent:    []string{"<D:multistatus"},
		NotExpectedContent: []string{webdav.FileName},
	}
	scenario.Test(t)
}

// A client checks what it is talking to before it trusts it.
func TestTheEndpointAnnouncesItselfAsWebdav(t *testing.T) {
	scenario := tests.ApiScenario{
		Name:           "OPTIONS advertises the DAV class",
		Method:         http.MethodOptions,
		URL:            "/webdav/",
		Headers:        basicAuth(testutil.KoUsernameA, testutil.KoPasswordA),
		TestAppFactory: newFactory(),
		ExpectedStatus: http.StatusOK,
		AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
			if dav := res.Header.Get("DAV"); !strings.Contains(dav, "1") {
				t.Errorf("DAV header is %q, expected it to claim class 1", dav)
			}
			if allow := res.Header.Get("Allow"); !strings.Contains(allow, "PROPFIND") {
				t.Errorf("Allow header is %q", allow)
			}
		},
	}
	scenario.Test(t)
}

// withHeader adds one header to a set.
func withHeader(headers map[string]string, key, value string) map[string]string {
	out := map[string]string{key: value}
	for name, existing := range headers {
		out[name] = existing
	}

	return out
}

// A client that asks for the collection without the trailing slash is asking a
// different pattern of the router, and gets the same answer.
func TestTheCollectionAnswersWithoutATrailingSlash(t *testing.T) {
	scenario := tests.ApiScenario{
		Name:   "PROPFIND on /webdav",
		Method: "PROPFIND",
		URL:    "/webdav",
		Headers: withHeader(basicAuth(testutil.KoUsernameA, testutil.KoPasswordA),
			"Depth", "0"),
		TestAppFactory:  newFactory(),
		ExpectedStatus:  http.StatusMultiStatus,
		ExpectedContent: []string{"<D:multistatus"},
	}
	scenario.Test(t)
}

// The web interface serves everything it does not recognise from one catch-all
// GET route, and this endpoint has to be able to live beside it.
//
// This is here because it did not use to. A route bound to every method under
// "/webdav/" and a "GET /{path...}" are two patterns where neither is more
// specific than the other — one matches more methods, the other a more general
// path — which net/http calls a conflict and answers with a panic while it is
// building the mux. Nothing in this package's own tests mounted a web interface,
// so every one of them passed against a server that could not start.
func TestTheSyncTargetCoexistsWithTheWebInterface(t *testing.T) {
	content := statisticsBytes(t)

	scenario := tests.ApiScenario{
		Name:   "both the catch-all and the sync target are mounted",
		Method: http.MethodPut,
		URL:    syncURL,
		Body:   bytes.NewReader(content),
		TestAppFactory: func(t testing.TB) *tests.TestApp {
			app := testutil.NewApp(t)

			conf := &config.Config{KoreaderAuthCacheTtl: 300, EnableWebdav: true}
			conf.Normalize()

			sync := koreader.Register(app, conf)
			webdav.Register(app, conf, sync)

			// What registerWebUi binds in main, at the priority it binds it.
			app.OnServe().Bind(&hook.Handler[*core.ServeEvent]{
				Func: func(se *core.ServeEvent) error {
					se.Router.GET("/{path...}", func(e *core.RequestEvent) error {
						return e.String(http.StatusOK, "the web interface")
					})

					return se.Next()
				},
				Priority: 999,
			})

			testutil.Seed(t, app)

			return app
		},
		Headers:         basicAuth(testutil.KoUsernameA, testutil.KoPasswordA),
		ExpectedStatus:  http.StatusCreated,
		ExpectedContent: []string{"Created"},
		AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
			if _, ok := stored(t, app, testutil.IdUserA); !ok {
				t.Error("the upload did not reach the sync target")
			}
		},
	}
	scenario.Test(t)
}

// And the catch-all still answers for everything that is not the sync target.
func TestTheWebInterfaceStillAnswersEverythingElse(t *testing.T) {
	scenario := tests.ApiScenario{
		Name:   "an ordinary page is served by the catch-all",
		Method: http.MethodGet,
		URL:    "/library",
		TestAppFactory: func(t testing.TB) *tests.TestApp {
			app := testutil.NewApp(t)

			conf := &config.Config{KoreaderAuthCacheTtl: 300, EnableWebdav: true}
			conf.Normalize()

			sync := koreader.Register(app, conf)
			webdav.Register(app, conf, sync)

			app.OnServe().Bind(&hook.Handler[*core.ServeEvent]{
				Func: func(se *core.ServeEvent) error {
					se.Router.GET("/{path...}", func(e *core.RequestEvent) error {
						return e.String(http.StatusOK, "the web interface")
					})

					return se.Next()
				},
				Priority: 999,
			})

			testutil.Seed(t, app)

			return app
		},
		ExpectedStatus:  http.StatusOK,
		ExpectedContent: []string{"the web interface"},
	}
	scenario.Test(t)
}
