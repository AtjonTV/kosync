//
// File:        internal/wikidocs/wikidocs_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package wikidocs_test

import (
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"git.obth.eu/atjontv/kosync/internal/wikidocs"
)

// repo is where a link that leaves the wiki points in these tests.
var repo = wikidocs.Repo{URL: "https://example.invalid/kosync", Branch: "main"}

// tree is a repository holding every document the wiki mirrors, with the given
// bodies written into the ones named and a placeholder in the rest. A page
// missing from the tree is an error, so all of them have to be there.
func tree(bodies map[string]string) fs.FS {
	files := fstest.MapFS{
		// One directory the documentation points at, and one file it does not.
		"server/internal/migrations/0001_init.go": &fstest.MapFile{Data: []byte("package migrations\n")},
		"LICENSE.txt": &fstest.MapFile{Data: []byte("EUPL\n")},
	}
	for _, page := range wikidocs.Pages {
		body, given := bodies[page.Source]
		if !given {
			body = "# " + page.Title + "\n"
		}
		files[page.Source] = &fstest.MapFile{Data: []byte(body)}
	}

	return files
}

// build renders the wiki and returns one page of it.
func build(t testing.TB, source, body string) string {
	t.Helper()

	files, err := wikidocs.Build(tree(map[string]string{source: body}), repo)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	slug := ""
	for _, page := range wikidocs.Pages {
		if page.Source == source {
			slug = page.Slug
		}
	}

	page, held := files[slug+".md"]
	if !held {
		t.Fatalf("the wiki has no page %q", slug)
	}

	return string(page)
}

func TestEveryDocumentBecomesAPage(t *testing.T) {
	files, err := wikidocs.Build(tree(nil), repo)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if len(files) != len(wikidocs.Pages)+1 {
		t.Errorf("the wiki has %d files for %d pages and a sidebar", len(files), len(wikidocs.Pages))
	}
	for _, page := range wikidocs.Pages {
		if _, held := files[page.Slug+".md"]; !held {
			t.Errorf("%s did not become %s.md", page.Source, page.Slug)
		}
	}
	if _, held := files["_sidebar.md"]; !held {
		t.Errorf("the wiki has no sidebar")
	}
}

// The wiki cannot be rendered from half a repository: a page silently missing
// would be a link in the sidebar leading nowhere.
func TestADocumentThatIsNotThereIsAnError(t *testing.T) {
	files := fstest.MapFS{"README.md": &fstest.MapFile{Data: []byte("# KOsync\n")}}

	if _, err := wikidocs.Build(files, repo); err == nil {
		t.Errorf("rendering a wiki without the documentation succeeded")
	}
}

// A wiki page is addressed by its slug, so a link between two documents has to
// stop being a file name.
func TestALinkToAnotherPageBecomesItsSlug(t *testing.T) {
	page := build(t, "docs/technical/api.md", "See [analytics.md](analytics.md) for the numbers.\n")

	if !strings.Contains(page, "[analytics.md](analytics)") {
		t.Errorf("the page reads %q", page)
	}
}

func TestALinkFromTheRootFindsADocument(t *testing.T) {
	page := build(t, "README.md", "See [docs/technical/config.md](docs/technical/config.md)\n")

	if !strings.Contains(page, "[docs/technical/config.md](config)") {
		t.Errorf("the page reads %q", page)
	}
}

func TestTheAnchorOfALinkSurvives(t *testing.T) {
	page := build(t, "CHANGELOG.md", "See [docs/technical/api.md](docs/technical/api.md#book-preview)\n")

	if !strings.Contains(page, "(api#book-preview)") {
		t.Errorf("the page reads %q", page)
	}
}

// The plans, the licence and every source file stay in the repository. A link
// to one has to leave the wiki rather than point at a page that is not there.
func TestALinkToSomethingTheWikiDoesNotHoldGoesBackToTheRepository(t *testing.T) {
	page := build(t, "README.md", "See [the plan](docs/technical/plans/rewrite-plan.md) and [the licence](/LICENSE.txt)\n")

	if !strings.Contains(page, "(https://example.invalid/kosync/-/blob/main/docs/technical/plans/rewrite-plan.md)") {
		t.Errorf("the plan is linked as %q", page)
	}
	if !strings.Contains(page, "(https://example.invalid/kosync/-/blob/main/LICENSE.txt)") {
		t.Errorf("the licence is linked as %q", page)
	}
}

// A picture has to be served as itself. Addressed the way a link is, GitLab
// answers with a page showing the file, and the page shows nothing.
func TestAPictureIsAddressedAsItsContents(t *testing.T) {
	page := build(t, "README.md", "![shelf](.gitlab/assets/webui-dark.png)\n"+
		`<img src=".gitlab/assets/webui-light.png" width="640px" alt="light"/>`+"\n")

	for _, want := range []string{
		"![shelf](https://example.invalid/kosync/-/raw/main/.gitlab/assets/webui-dark.png)",
		`<img src="https://example.invalid/kosync/-/raw/main/.gitlab/assets/webui-light.png" width="640px"`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the page does not contain %q:\n%s", want, page)
		}
	}
}

func TestAnAddressThatAlreadySaysWhereItPointsIsLeftAlone(t *testing.T) {
	body := "[KOReader](https://koreader.rocks), [mail](mailto:nobody@example.invalid), " +
		"[above](#heading), [host](//example.invalid/x)\n"
	page := build(t, "README.md", body)

	if !strings.Contains(page, body) {
		t.Errorf("the page reads %q", page)
	}
}

// What is inside a code block is an example of something, not a link to it.
func TestALinkInsideACodeBlockIsNotRewritten(t *testing.T) {
	page := build(t, "docs/technical/api.md", "```md\n[analytics.md](analytics.md)\n```\n[analytics.md](analytics.md)\n")

	if strings.Count(page, "[analytics.md](analytics.md)") != 1 {
		t.Errorf("the page reads %q", page)
	}
	if !strings.Contains(page, "[analytics.md](analytics)") {
		t.Errorf("the link outside the block was not rewritten: %q", page)
	}
}

// The page has to say where it came from, because a wiki invites an edit that
// the next sync would silently throw away.
func TestEveryPageSaysWhereItCameFrom(t *testing.T) {
	files, err := wikidocs.Build(tree(nil), repo)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	for _, page := range wikidocs.Pages {
		body := string(files[page.Slug+".md"])
		if !strings.HasPrefix(body, "> Generated from [`"+page.Source+"`]") {
			t.Errorf("%s.md begins %.60q", page.Slug, body)
		}
		if !strings.Contains(body, "overwritten") {
			t.Errorf("%s.md does not say an edit here is lost", page.Slug)
		}
	}
}

func TestTheSidebarListsEveryPageUnderItsHeading(t *testing.T) {
	files, err := wikidocs.Build(tree(nil), repo)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	sidebar := string(files["_sidebar.md"])
	for _, page := range wikidocs.Pages {
		if !strings.Contains(sidebar, "- ["+page.Title+"]("+page.Slug+")") {
			t.Errorf("the sidebar does not list %s:\n%s", page.Slug, sidebar)
		}
	}
	for _, heading := range []string{"**For readers**", "**For operators and developers**", "**Project**"} {
		if !strings.Contains(sidebar, heading) {
			t.Errorf("the sidebar has no %s heading:\n%s", heading, sidebar)
		}
	}
	// The page that stands above the first heading is the one the wiki opens on.
	if !strings.HasPrefix(sidebar, "### KOsync\n\n- [Home](home)") {
		t.Errorf("the sidebar begins %.40q", sidebar)
	}
}

// GitLab addresses a directory differently from a file, and the documentation
// points at directories often enough — "the migrations live here" — that a link
// to one landing on an error would be noticed.
func TestALinkToADirectoryIsAddressedAsATree(t *testing.T) {
	page := build(t, "docs/technical/database.md", "The [migrations](../../server/internal/migrations) run in order.\n")

	if !strings.Contains(page, "(https://example.invalid/kosync/-/tree/main/server/internal/migrations)") {
		t.Errorf("the page reads %q", page)
	}
}

// The documentation is written in two directories and the wiki is flat, so a
// link from one half to the other has to survive losing the path it climbed.
func TestALinkFromOneHalfOfTheDocumentationToTheOtherFindsItsPage(t *testing.T) {
	page := build(t, "docs/user/library.md", "The [API](../technical/api.md) serves it.\n")

	if !strings.Contains(page, "[API](api)") {
		t.Errorf("the page reads %q", page)
	}
}
