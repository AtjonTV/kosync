//
// File:        internal/wikidocs/wikidocs.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

// Package wikidocs renders the repository's documentation as the files of a
// GitLab wiki.
//
// The documentation lives in this repository and is reviewed with the code that
// changes it; the wiki is a copy, pushed by CI, so that it can be read where
// people look for it. Nothing here talks to git: it turns a tree of files into
// another tree of files, which is the part worth testing.
//
// A wiki page is addressed by its slug and not by a file name, so every link
// between the documents has to be rewritten on the way in — and a link to
// something the wiki does not hold, a plan or the licence or a source file, has
// to become an address back into the repository rather than a dead page.
package wikidocs

import (
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"strings"
)

// Page is one document the wiki mirrors.
type Page struct {
	// Source is where the document lives in the repository.
	Source string
	// Slug is the name the wiki addresses it by.
	Slug string
	// Title is how the sidebar lists it.
	Title string
	// Group is the sidebar heading it appears under, empty for the first
	// entries, which stand on their own above the first heading.
	Group string
}

// Pages is what the wiki holds, in the order the sidebar lists it.
//
// The reader's pages come first because most of the people who open a wiki are
// looking for how the thing is used, not how it is built.
//
// The plans under docs/technical/plans are deliberately not here. They are the record of
// how something came to be built rather than documentation of what it does, one
// of them runs to a hundred kilobytes, and together they would outnumber the
// documentation in the page list. Links to them still work: they become
// addresses into the repository like any other file the wiki does not hold.
var Pages = []Page{
	{Source: "README.md", Slug: "home", Title: "Home"},

	{Source: "docs/user/index.md", Slug: "reading-with-kosync", Title: "KOsync for readers", Group: "For readers"},
	{Source: "docs/user/getting-started.md", Slug: "getting-started", Title: "Getting started", Group: "For readers"},
	{Source: "docs/user/library.md", Slug: "library", Title: "Your library", Group: "For readers"},
	{Source: "docs/user/reading.md", Slug: "reading", Title: "Your reading", Group: "For readers"},
	{Source: "docs/user/statistics.md", Slug: "statistics", Title: "The reading record", Group: "For readers"},
	{Source: "docs/user/account.md", Slug: "account", Title: "Your account", Group: "For readers"},

	{Source: "docs/technical/build.md", Slug: "build", Title: "Building and deploying", Group: "For operators and developers"},
	{Source: "docs/technical/config.md", Slug: "config", Title: "Configuration", Group: "For operators and developers"},
	{Source: "docs/technical/api.md", Slug: "api", Title: "API", Group: "For operators and developers"},
	{Source: "docs/technical/database.md", Slug: "database", Title: "Database", Group: "For operators and developers"},
	{Source: "docs/technical/analytics.md", Slug: "analytics", Title: "Analytics", Group: "For operators and developers"},
	{Source: "docs/technical/migration.md", Slug: "migration", Title: "Migrating from 1.x", Group: "For operators and developers"},

	{Source: "CHANGELOG.md", Slug: "changelog", Title: "Changelog", Group: "Project"},
	{Source: "CONTRIBUTING.md", Slug: "contributing", Title: "Contributing", Group: "Project"},
	{Source: "CODE_OF_CONDUCT.md", Slug: "code-of-conduct", Title: "Code of Conduct", Group: "Project"},
	{Source: "MACHINE_POLICY.md", Slug: "machine-policy", Title: "Machine Policy", Group: "Project"},
}

// Repo is where a link that leaves the wiki has to point.
type Repo struct {
	// URL is the project's address, without a trailing slash.
	URL string
	// Branch is the branch a link into the repository reads from.
	Branch string
}

// blob addresses a file as GitLab renders it, raw addresses its contents and
// tree addresses a directory. A screenshot has to be the second one, or the page
// shows a link where the picture should be.
func (r Repo) blob(name string) string { return r.URL + "/-/blob/" + r.Branch + "/" + name }
func (r Repo) raw(name string) string  { return r.URL + "/-/raw/" + r.Branch + "/" + name }
func (r Repo) tree(name string) string { return r.URL + "/-/tree/" + r.Branch + "/" + name }

// Build renders the wiki, keyed by the file name each page has in the wiki
// repository.
func Build(tree fs.FS, repo Repo) (map[string][]byte, error) {
	slugs := make(map[string]string, len(Pages))
	for _, page := range Pages {
		slugs[page.Source] = page.Slug
	}

	files := make(map[string][]byte, len(Pages)+1)
	for _, page := range Pages {
		body, err := fs.ReadFile(tree, page.Source)
		if err != nil {
			return nil, fmt.Errorf("wikidocs: %s: %w", page.Source, err)
		}

		files[page.Slug+".md"] = []byte(banner(page, repo) + rewrite(tree, string(body), page.Source, slugs, repo))
	}
	files["_sidebar.md"] = []byte(sidebar())

	return files, nil
}

// banner says where the page came from, on the page itself.
//
// A wiki invites editing, and an edit made here survives until the next push to
// the default branch and is then silently gone. Saying so is the only warning
// GitLab can be made to give.
func banner(page Page, repo Repo) string {
	return fmt.Sprintf("> Generated from [`%s`](%s) — edits made here are overwritten by the next\n"+
		"> sync. Change the file in the repository instead.\n\n", page.Source, repo.blob(page.Source))
}

// sidebar is the navigation GitLab shows beside every page.
func sidebar() string {
	var out strings.Builder
	out.WriteString("### KOsync\n\n")

	group := ""
	for _, page := range Pages {
		if page.Group != group {
			group = page.Group
			out.WriteString("\n**" + group + "**\n\n")
		}
		out.WriteString("- [" + page.Title + "](" + page.Slug + ")\n")
	}

	return out.String()
}

var (
	// fence opens or closes a code block. What is inside one is an example and
	// not a link, however much it looks like one.
	fence = regexp.MustCompile("^\\s*(```|~~~)")
	// mdLink matches a markdown link and, with the leading "!", an image.
	mdLink = regexp.MustCompile(`(!?)\[([^\]]*)\]\(([^)\s]+)\)`)
	// htmlImage matches the raw <img> tags the README uses for its screenshots,
	// which markdown cannot give a width to.
	htmlImage = regexp.MustCompile(`(<img[^>]+src=")([^"]+)(")`)
	// schemed is an address that already says where it points.
	schemed = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.-]*:`)
)

// rewrite points every link in one document at where it lives in the wiki.
func rewrite(tree fs.FS, body, from string, slugs map[string]string, repo Repo) string {
	lines := strings.Split(body, "\n")

	inFence := false
	for index, line := range lines {
		if fence.MatchString(line) {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}

		line = mdLink.ReplaceAllStringFunc(line, func(match string) string {
			parts := mdLink.FindStringSubmatch(match)
			asset := parts[1] == "!"

			return parts[1] + "[" + parts[2] + "](" + target(tree, parts[3], from, slugs, repo, asset) + ")"
		})
		lines[index] = htmlImage.ReplaceAllStringFunc(line, func(match string) string {
			parts := htmlImage.FindStringSubmatch(match)

			return parts[1] + target(tree, parts[2], from, slugs, repo, true) + parts[3]
		})
	}

	return strings.Join(lines, "\n")
}

// target rewrites one address so that it works from a wiki page.
//
// asset says the address is drawn rather than followed, which decides between
// the two ways GitLab serves a file it holds.
func target(tree fs.FS, href, from string, slugs map[string]string, repo Repo, asset bool) string {
	if strings.HasPrefix(href, "#") || strings.HasPrefix(href, "//") || schemed.MatchString(href) {
		return href
	}

	address, fragment, hasFragment := strings.Cut(href, "#")
	if address == "" {
		return href
	}

	// An address is relative to the document that wrote it, unless it starts at
	// the repository root.
	name := path.Clean(strings.TrimPrefix(address, "/"))
	if !strings.HasPrefix(address, "/") {
		name = path.Clean(path.Join(path.Dir(from), address))
	}

	rewritten := repo.blob(name)
	switch slug, held := slugs[name]; {
	case held:
		rewritten = slug
	case asset:
		rewritten = repo.raw(name)
	case isDir(tree, name):
		rewritten = repo.tree(name)
	}

	if hasFragment {
		rewritten += "#" + fragment
	}

	return rewritten
}

// isDir says whether the repository holds a directory under that name. A link
// to one is common in the documentation — "the migrations live here" — and
// GitLab addresses a directory differently from a file in it.
func isDir(tree fs.FS, name string) bool {
	info, err := fs.Stat(tree, name)

	return err == nil && info.IsDir()
}
