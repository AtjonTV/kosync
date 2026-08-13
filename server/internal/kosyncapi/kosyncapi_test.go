//
// File:        internal/kosyncapi/kosyncapi_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosyncapi_test

import (
	"net/http"
	"strings"
	"testing"

	"git.obth.eu/atjontv/kosync/internal/config"
	"git.obth.eu/atjontv/kosync/internal/koreader"
	"git.obth.eu/atjontv/kosync/internal/kosyncapi"
	"git.obth.eu/atjontv/kosync/internal/schema"
	"git.obth.eu/atjontv/kosync/internal/testutil"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

// newApp returns a seeded app with the WebUI and device APIs mounted.
//
// Each scenario gets its own app: PocketBase registers its routes when the
// serve event fires, and firing that twice on one app collides.
func newApp(t testing.TB) *tests.TestApp {
	t.Helper()

	app := testutil.SeededApp(t)
	conf := &config.Config{}
	conf.Normalize()
	kosyncapi.Register(app, conf)
	koreader.Register(app, conf)

	return app
}

// newAppWithoutRegistration returns an app that refuses new WebUI accounts.
func newAppWithoutRegistration(t testing.TB) *tests.TestApp {
	t.Helper()

	app := testutil.SeededApp(t)
	conf := &config.Config{DisableRegistration: true}
	conf.Normalize()
	kosyncapi.Register(app, conf)

	return app
}

// asUser runs a scenario authenticated as the given fixture user.
//
// The token is minted in BeforeTestFunc because its signing key only exists
// once the app has been created.
func asUser(t *testing.T, userId string, scenario tests.ApiScenario) {
	t.Helper()

	if scenario.TestAppFactory == nil {
		scenario.TestAppFactory = newApp
	}

	headers := map[string]string{}
	for k, v := range scenario.Headers {
		headers[k] = v
	}
	scenario.Headers = headers

	if userId != "" {
		before := scenario.BeforeTestFunc
		scenario.BeforeTestFunc = func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
			user, err := app.FindRecordById(schema.CollectionUsers, userId)
			if err != nil {
				t.Fatalf("failed to load the fixture user %q: %v", userId, err)
			}
			headers["Authorization"] = testutil.UserToken(t, user)

			if before != nil {
				before(t, app, e)
			}
		}
	}

	scenario.Test(t)
}

func TestCreateKoreaderAccount(t *testing.T) {
	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:            "the server hashes the password the way KOReader will",
		Method:          http.MethodPost,
		URL:             "/api/kosync/koreader-accounts",
		Body:            strings.NewReader(`{"username":"alice-boox","password":"my-device-password","label":"Boox Page"}`),
		ExpectedStatus:  http.StatusCreated,
		ExpectedContent: []string{`"username":"alice-boox"`, `"label":"Boox Page"`},
		NotExpectedContent: []string{
			"my-device-password",
			testutil.Md5Hex("my-device-password"),
		},
		AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
			account, err := app.FindFirstRecordByData(schema.CollectionKoreaderAccounts, schema.FieldUsername, "alice-boox")
			if err != nil {
				t.Fatalf("expected the credential to be stored: %v", err)
			}
			if account.GetString(schema.FieldOwner) != testutil.IdUserA {
				t.Errorf("expected the credential to belong to the caller")
			}
			// This is the whole point of the endpoint: what a device sends must
			// validate against what was stored.
			if !account.ValidatePassword(testutil.Md5Hex("my-device-password")) {
				t.Errorf("expected the stored credential to match the digest KOReader would send")
			}
		},
	})

	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:            "a taken username is refused",
		Method:          http.MethodPost,
		URL:             "/api/kosync/koreader-accounts",
		Body:            strings.NewReader(`{"username":"` + testutil.KoUsernameB + `","password":"my-device-password"}`),
		ExpectedStatus:  http.StatusBadRequest,
		ExpectedContent: []string{"already taken"},
	})

	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:            "a short password is refused",
		Method:          http.MethodPost,
		URL:             "/api/kosync/koreader-accounts",
		Body:            strings.NewReader(`{"username":"alice-boox","password":"short"}`),
		ExpectedStatus:  http.StatusBadRequest,
		ExpectedContent: []string{"at least"},
	})

	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:            "a username is required",
		Method:          http.MethodPost,
		URL:             "/api/kosync/koreader-accounts",
		Body:            strings.NewReader(`{"username":"   ","password":"my-device-password"}`),
		ExpectedStatus:  http.StatusBadRequest,
		ExpectedContent: []string{"username is required"},
	})

	asUser(t, "", tests.ApiScenario{
		Name:            "guests cannot create credentials",
		Method:          http.MethodPost,
		URL:             "/api/kosync/koreader-accounts",
		Body:            strings.NewReader(`{"username":"anon","password":"my-device-password"}`),
		ExpectedStatus:  http.StatusUnauthorized,
		ExpectedContent: []string{`"data":{}`},
	})
}

func TestACredentialCreatedInTheWebUiWorksOnADevice(t *testing.T) {
	// The end to end promise of the whole design: what the WebUI stores is what
	// a KOReader device can authenticate with.
	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:           "create a credential",
		Method:         http.MethodPost,
		URL:            "/api/kosync/koreader-accounts",
		Body:           strings.NewReader(`{"username":"alice-boox","password":"my-device-password"}`),
		ExpectedStatus: http.StatusCreated,
		ExpectedContent: []string{
			`"username":"alice-boox"`,
		},
		AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
			account, err := app.FindFirstRecordByData(schema.CollectionKoreaderAccounts, schema.FieldUsername, "alice-boox")
			if err != nil {
				t.Fatalf("expected the credential to be stored: %v", err)
			}
			if !account.ValidatePassword(testutil.Md5Hex("my-device-password")) {
				t.Fatalf("the stored digest does not match what KOReader would send")
			}
			if account.GetBool(schema.FieldDisabled) {
				t.Errorf("expected a new credential to be usable right away")
			}
		},
	})
}

func TestRotateKoreaderPassword(t *testing.T) {
	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:            "the owner can set a new password",
		Method:          http.MethodPost,
		URL:             "/api/kosync/koreader-accounts/" + testutil.IdAccountA + "/password",
		Body:            strings.NewReader(`{"password":"a-brand-new-password"}`),
		ExpectedStatus:  http.StatusOK,
		ExpectedContent: []string{"changed"},
		AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
			account, err := app.FindRecordById(schema.CollectionKoreaderAccounts, testutil.IdAccountA)
			if err != nil {
				t.Fatalf("failed to reload the credential: %v", err)
			}
			if !account.ValidatePassword(testutil.Md5Hex("a-brand-new-password")) {
				t.Errorf("expected the new password digest to be stored")
			}
			if account.ValidatePassword(testutil.Md5Hex(testutil.KoPasswordA)) {
				t.Errorf("expected the old password to stop working")
			}
		},
	})

	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:            "a user cannot rotate someone else's credential",
		Method:          http.MethodPost,
		URL:             "/api/kosync/koreader-accounts/" + testutil.IdAccountB + "/password",
		Body:            strings.NewReader(`{"password":"a-brand-new-password"}`),
		ExpectedStatus:  http.StatusNotFound,
		ExpectedContent: []string{`"data":{}`},
		AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
			account, err := app.FindRecordById(schema.CollectionKoreaderAccounts, testutil.IdAccountB)
			if err != nil {
				t.Fatalf("failed to reload the credential: %v", err)
			}
			if !account.ValidatePassword(testutil.Md5Hex(testutil.KoPasswordB)) {
				t.Errorf("the foreign credential was changed")
			}
		},
	})

	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:            "rotating a credential that does not exist",
		Method:          http.MethodPost,
		URL:             "/api/kosync/koreader-accounts/" + testutil.PadId("missing") + "/password",
		Body:            strings.NewReader(`{"password":"a-brand-new-password"}`),
		ExpectedStatus:  http.StatusNotFound,
		ExpectedContent: []string{`"data":{}`},
	})

	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:            "a short replacement password is refused",
		Method:          http.MethodPost,
		URL:             "/api/kosync/koreader-accounts/" + testutil.IdAccountA + "/password",
		Body:            strings.NewReader(`{"password":"short"}`),
		ExpectedStatus:  http.StatusBadRequest,
		ExpectedContent: []string{"at least"},
	})
}

func TestCredentialPasswordCannotBeChangedThroughTheCollectionApi(t *testing.T) {
	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:            "the password is rejected",
		Method:          http.MethodPatch,
		URL:             "/api/collections/koreader_accounts/records/" + testutil.IdAccountA,
		Body:            strings.NewReader(`{"password":"not-a-digest","passwordConfirm":"not-a-digest"}`),
		ExpectedStatus:  http.StatusBadRequest,
		ExpectedContent: []string{"cannot be changed here"},
		AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
			account, err := app.FindRecordById(schema.CollectionKoreaderAccounts, testutil.IdAccountA)
			if err != nil {
				t.Fatalf("failed to reload the credential: %v", err)
			}
			if !account.ValidatePassword(testutil.Md5Hex(testutil.KoPasswordA)) {
				t.Errorf("expected the device password to survive the attempt")
			}
		},
	})

	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:            "the username is rejected",
		Method:          http.MethodPatch,
		URL:             "/api/collections/koreader_accounts/records/" + testutil.IdAccountA,
		Body:            strings.NewReader(`{"username":"renamed"}`),
		ExpectedStatus:  http.StatusBadRequest,
		ExpectedContent: []string{"cannot be changed here"},
	})

	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:            "the label and the disabled flag are allowed",
		Method:          http.MethodPatch,
		URL:             "/api/collections/koreader_accounts/records/" + testutil.IdAccountA,
		Body:            strings.NewReader(`{"label":"Old Kobo","disabled":true}`),
		ExpectedStatus:  http.StatusOK,
		ExpectedContent: []string{`"label":"Old Kobo"`, `"disabled":true`},
	})
}

func TestRegistrationCanBeDisabled(t *testing.T) {
	scenario := tests.ApiScenario{
		Name:            "registration is refused when disabled",
		Method:          http.MethodPost,
		URL:             "/api/collections/users/records",
		Body:            strings.NewReader(`{"email":"newcomer@example.com","password":"a-long-enough-password","passwordConfirm":"a-long-enough-password"}`),
		TestAppFactory:  newAppWithoutRegistration,
		ExpectedStatus:  http.StatusForbidden,
		ExpectedContent: []string{"Registration is disabled"},
	}
	scenario.Test(t)

	allowed := tests.ApiScenario{
		Name:           "registration works by default",
		Method:         http.MethodPost,
		URL:            "/api/collections/users/records",
		Body:           strings.NewReader(`{"email":"newcomer@example.com","password":"a-long-enough-password","passwordConfirm":"a-long-enough-password"}`),
		TestAppFactory: newApp,
		ExpectedStatus: http.StatusOK,
		// PocketBase hides the address unless the account made it visible, so
		// the created record is checked directly.
		ExpectedContent: []string{`"collectionName":"users"`},
		AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
			if _, err := app.FindAuthRecordByEmail(schema.CollectionUsers, "newcomer@example.com"); err != nil {
				t.Errorf("expected the account to be created: %v", err)
			}
		},
	}
	allowed.Test(t)
}
