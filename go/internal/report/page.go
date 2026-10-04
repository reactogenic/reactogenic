package report

// Page is a report about a built page as a whole: at the page's file, named
// with its pathname, and without a position — what the builder finds on the
// rendered HTML and in the page's record cannot be traced to a line of the
// source (specs/phase02/builder.md, *Checks on the page*: the renderer
// carries no provenance of an attribute). `check.Print` writes it as
// `pages/guide/index.rtsx: error idref-not-found: Page /guide/: …`.
func Page(file, pathname, code, message string) Report {
	return Report{File: file, Code: code, Message: "Page " + pathname + ": " + message}
}
