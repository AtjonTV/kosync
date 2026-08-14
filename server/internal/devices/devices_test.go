//
// File:        internal/devices/devices_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package devices_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"git.obth.eu/atjontv/kosync/internal/devices"
	"git.obth.eu/atjontv/kosync/internal/schema"
	"git.obth.eu/atjontv/kosync/internal/testutil"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

// The identifier from the reference database, and the name KOReader sends with
// it. The point of the whole package is that these are not the same thing.
const (
	go7Id   = "865F46C0C0F4401D9A05768B6B0BF3AC"
	go7Name = "go7"
)

var moment = time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)

// newApp returns a migrated app with the hooks registered and one user.
func newApp(t testing.TB) (*tests.TestApp, *core.Record) {
	t.Helper()

	app := testutil.NewApp(t)
	devices.Register(app)
	user := testutil.CreateUser(t, app, testutil.IdUserA, testutil.EmailUserA, testutil.PasswordUsers)

	return app, user
}

// push writes a progress record the way a device would.
func push(t testing.TB, app core.App, user *core.Record, hash, name, deviceId string, at time.Time) *core.Record {
	t.Helper()

	document := testutil.CreateDocument(t, app, user, "", hash, 0.4, at)
	document.Set(schema.FieldLastDevice, name)
	document.Set(schema.FieldLastDeviceId, deviceId)
	if err := app.Save(document); err != nil {
		t.Fatalf("failed to store the push: %v", err)
	}

	return document
}

// deviceOf loads the registered device with the given identifier.
func deviceOf(t testing.TB, app core.App, user *core.Record, deviceId string) *core.Record {
	t.Helper()

	record, err := devices.Find(app, user.Id, deviceId)
	if err != nil {
		t.Fatalf("failed to look the device up: %v", err)
	}

	return record
}

func TestADeviceIsRegisteredOnItsFirstPush(t *testing.T) {
	app, user := newApp(t)

	push(t, app, user, "hash-a", go7Name, go7Id, moment)

	record := deviceOf(t, app, user, go7Id)
	if record == nil {
		t.Fatalf("expected the device to be registered")
	}
	if got := record.GetString(schema.FieldReportedName); got != go7Name {
		t.Errorf("expected the reported name %q, got %q", go7Name, got)
	}
	// The name starts as whatever KOReader said, so a device is never nameless.
	if got := record.GetString(schema.FieldName); got != go7Name {
		t.Errorf("expected the display name to start from the reported one, got %q", got)
	}
	if record.GetDateTime(schema.FieldLastSeen).IsZero() {
		t.Errorf("expected last_seen to be set")
	}
}

func TestPushingAgainDoesNotDuplicateTheDevice(t *testing.T) {
	app, user := newApp(t)

	push(t, app, user, "hash-a", go7Name, go7Id, moment)
	push(t, app, user, "hash-b", go7Name, go7Id, moment.Add(time.Hour))

	all, err := app.FindAllRecords(schema.CollectionDevices)
	if err != nil {
		t.Fatalf("failed to list the devices: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("expected one device, got %d", len(all))
	}
	if got := all[0].GetDateTime(schema.FieldLastSeen).Time(); !got.Equal(moment.Add(time.Hour)) {
		t.Errorf("expected last_seen to follow the newer push, got %v", got)
	}
}

// This is the whole point: a name chosen here has to survive the next sync, or
// it was never worth typing.
func TestARenameSurvivesFurtherPushes(t *testing.T) {
	app, user := newApp(t)

	push(t, app, user, "hash-a", go7Name, go7Id, moment)

	record := deviceOf(t, app, user, go7Id)
	record.Set(schema.FieldName, "Boox Go 7")
	if err := app.Save(record); err != nil {
		t.Fatalf("failed to rename the device: %v", err)
	}

	push(t, app, user, "hash-b", go7Name, go7Id, moment.Add(time.Hour))

	renamed := deviceOf(t, app, user, go7Id)
	if got := renamed.GetString(schema.FieldName); got != "Boox Go 7" {
		t.Errorf("expected the chosen name to survive, got %q", got)
	}
	// The reported name is still tracked, so it is clear what the device calls
	// itself even after the override.
	if got := renamed.GetString(schema.FieldReportedName); got != go7Name {
		t.Errorf("expected the reported name to still be recorded, got %q", got)
	}
}

// Renaming a device in KOReader shows up for anyone who has not overridden it.
func TestTheReportedNameFollowsTheDevice(t *testing.T) {
	app, user := newApp(t)

	push(t, app, user, "hash-a", go7Name, go7Id, moment)
	push(t, app, user, "hash-b", "go7-kitchen", go7Id, moment.Add(time.Hour))

	record := deviceOf(t, app, user, go7Id)
	if got := record.GetString(schema.FieldReportedName); got != "go7-kitchen" {
		t.Errorf("expected the new reported name, got %q", got)
	}
}

// Old history is written after newer state during an import, and a device must
// not look unused because of it.
func TestLastSeenOnlyMovesForwards(t *testing.T) {
	app, user := newApp(t)

	document := push(t, app, user, "hash-a", go7Name, go7Id, moment)

	entry := testutil.CreateHistoryEntry(t, app, document, "", 0.1, moment.AddDate(0, 0, -30))
	entry.Set(schema.FieldLastDevice, go7Name)
	entry.Set(schema.FieldLastDeviceId, go7Id)
	if err := app.Save(entry); err != nil {
		t.Fatalf("failed to store the history entry: %v", err)
	}

	record := deviceOf(t, app, user, go7Id)
	if got := record.GetDateTime(schema.FieldLastSeen).Time(); !got.Equal(moment) {
		t.Errorf("expected last_seen to stay at the newest push, got %v", got)
	}
}

func TestAPushWithoutADeviceRegistersNothing(t *testing.T) {
	app, user := newApp(t)

	testutil.CreateDocument(t, app, user, "", "hash-a", 0.4, moment)

	all, err := app.FindAllRecords(schema.CollectionDevices)
	if err != nil {
		t.Fatalf("failed to list the devices: %v", err)
	}
	if len(all) != 0 {
		t.Errorf("expected no device from a push that named none, got %d", len(all))
	}
}

func TestDevicesAreOwnerScoped(t *testing.T) {
	app, alice := newApp(t)
	bob := testutil.CreateUser(t, app, testutil.IdUserB, testutil.EmailUserB, testutil.PasswordUsers)

	push(t, app, alice, "hash-a", go7Name, go7Id, moment)
	push(t, app, bob, "hash-b", go7Name, go7Id, moment)

	all, err := app.FindAllRecords(schema.CollectionDevices)
	if err != nil {
		t.Fatalf("failed to list the devices: %v", err)
	}
	// The same physical device shared between two accounts is two rows, because
	// each owner names it for themselves.
	if len(all) != 2 {
		t.Errorf("expected one row per owner, got %d", len(all))
	}
}

func TestDisplayNameFallsBackThroughTheThreeNames(t *testing.T) {
	app, user := newApp(t)

	push(t, app, user, "hash-a", go7Name, go7Id, moment)
	record := deviceOf(t, app, user, go7Id)

	record.Set(schema.FieldName, "Boox Go 7")
	if got := devices.DisplayName(record); got != "Boox Go 7" {
		t.Errorf("expected the chosen name, got %q", got)
	}

	record.Set(schema.FieldName, "")
	if got := devices.DisplayName(record); got != go7Name {
		t.Errorf("expected the reported name, got %q", got)
	}

	record.Set(schema.FieldReportedName, "")
	if got := devices.DisplayName(record); got != go7Id {
		t.Errorf("expected the identifier as the last resort, got %q", got)
	}

	if got := devices.DisplayName(nil); got != "" {
		t.Errorf("expected an empty name for no device, got %q", got)
	}
}

// The collection URL, and the id the fixture device is given so a request can
// name it without looking it up first.
const devicesURL = "/api/collections/devices/records"

var idDevice = testutil.PadId("deva")

// asOwner runs a scenario against a fresh app holding one registered device.
//
// Every scenario builds its own app on purpose: PocketBase registers its routes
// when a scenario starts, so two scenarios sharing one app collide on the very
// first route.
func asOwner(t *testing.T, scenario tests.ApiScenario) {
	t.Helper()

	headers := map[string]string{}
	for k, v := range scenario.Headers {
		headers[k] = v
	}
	scenario.Headers = headers

	scenario.TestAppFactory = func(t testing.TB) *tests.TestApp {
		app := testutil.NewApp(t)
		devices.Register(app)
		user := testutil.CreateUser(t, app, testutil.IdUserA, testutil.EmailUserA, testutil.PasswordUsers)

		// Registered by a push, the way a real one is, and then given a known id
		// so the request can address it.
		push(t, app, user, "hash-a", go7Name, go7Id, moment)
		registered := deviceOf(t, app, user, go7Id)
		if registered == nil {
			t.Fatalf("expected the push to register a device")
		}
		if err := app.Delete(registered); err != nil {
			t.Fatalf("failed to replace the device: %v", err)
		}

		collection, err := app.FindCollectionByNameOrId(schema.CollectionDevices)
		if err != nil {
			t.Fatalf("failed to find the devices collection: %v", err)
		}
		fixture := core.NewRecord(collection)
		fixture.Id = idDevice
		fixture.Set(schema.FieldOwner, user.Id)
		fixture.Set(schema.FieldDeviceId, go7Id)
		fixture.Set(schema.FieldReportedName, go7Name)
		fixture.Set(schema.FieldName, go7Name)
		if err := app.Save(fixture); err != nil {
			t.Fatalf("failed to store the fixture device: %v", err)
		}

		if _, set := headers["Authorization"]; !set {
			headers["Authorization"] = testutil.UserToken(t, user)
		}

		return app
	}

	scenario.Test(t)
}

// The name is the one thing here the owner gets to choose, and choosing it is
// the entire feature.
func TestTheDisplayNameIsEditable(t *testing.T) {
	asOwner(t, tests.ApiScenario{
		Name:   "the display name can be changed",
		Method: http.MethodPatch,
		URL:    devicesURL + "/" + idDevice,
		Body:   strings.NewReader(`{"name":"Boox Go 7"}`),
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		ExpectedStatus:  http.StatusOK,
		ExpectedContent: []string{`"name":"Boox Go 7"`, `"reported_name":"go7"`},
	})
}

func TestReportedFieldsCannotBeEdited(t *testing.T) {
	for _, field := range []string{schema.FieldDeviceId, schema.FieldReportedName, schema.FieldLastSeen} {
		asOwner(t, tests.ApiScenario{
			Name:   "editing " + field + " is refused",
			Method: http.MethodPatch,
			URL:    devicesURL + "/" + idDevice,
			Body:   strings.NewReader(`{"` + field + `":"tampered"}`),
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			ExpectedStatus:  http.StatusBadRequest,
			ExpectedContent: []string{"reported by the device"},
		})
	}
}

// A device exists because it synced. One created through the API would name
// nothing at all.
func TestDevicesCannotBeCreatedByHand(t *testing.T) {
	asOwner(t, tests.ApiScenario{
		Name:   "creating a device is refused",
		Method: http.MethodPost,
		URL:    devicesURL,
		Body: strings.NewReader(
			`{"owner":"` + testutil.IdUserA + `","device_id":"invented","name":"Nothing"}`),
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		ExpectedStatus:  http.StatusForbidden,
		ExpectedContent: []string{`"data":{}`},
	})
}

func TestDevicesAreInvisibleToGuests(t *testing.T) {
	asOwner(t, tests.ApiScenario{
		Name:            "a guest lists no devices",
		Method:          http.MethodGet,
		URL:             devicesURL,
		Headers:         map[string]string{"Authorization": ""},
		ExpectedStatus:  http.StatusOK,
		ExpectedContent: []string{`"totalItems":0`},
	})
}
