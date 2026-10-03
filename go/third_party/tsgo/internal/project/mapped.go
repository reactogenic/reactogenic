// Reactogenic: what the session does for the documents of the built-in
// content mapper. Not part of upstream: added by
// go/patches/0004-rtsx-mapper.patch.

package project

import (
	"github.com/microsoft/TypeScript/tsc/internal/contentmapper"
	"github.com/microsoft/TypeScript/tsc/internal/lsp/lsproto"
	"github.com/microsoft/TypeScript/tsc/internal/tsoptions"
	"github.com/microsoft/TypeScript/tsc/internal/tspath"
)

// mappedBufferDiffers reports whether uri is a content-mapped document whose
// buffer is not the file on disk: a new file not saved yet, a buffer with
// edits. Opening or closing such a document changes the program — a module
// appears or goes, or reads differently — and with it the diagnostics of
// other open documents, which no client pulls again by itself: it pulls the
// document it opened. DidOpenFile and DidCloseFile then ask for a refresh.
// (Upstream refreshes on a change to such a document, and on a watched
// file.)
//
// DidOpenFile asks with the snapshot updated; DidCloseFile, before it queues
// the close.
func (s *Session) mappedBufferDiffers(uri lsproto.DocumentUri) bool {
	if !s.isContentMapperFile(uri) {
		return false
	}
	overlay := s.fs.Overlays()[s.toPath(uri.FileName())]
	if overlay == nil {
		return true
	}
	// A change that is not in the overlay yet.
	s.pendingFileChangesMu.Lock()
	for _, change := range s.pendingFileChanges {
		if change.URI == uri {
			s.pendingFileChangesMu.Unlock()
			return true
		}
	}
	s.pendingFileChangesMu.Unlock()
	matches, _ := overlay.computeMatchesDiskText(s.fs.host)
	return !matches
}

// listsMappedFile reports whether a project that contains a file may be its
// default project: always, but for a file of the built-in mapper, which is
// its lister's — the project whose config names it, in `files` or through
// `include` — and not one that reaches it through an import only. The
// search for a default project goes on past such a project, which stays the
// fallback when no project lists the file.
//
// Upstream takes the first project that contains the file, so with
// `references` to a test project and then to the app it tests, a file of the
// app is checked under the tests' options. The host's command-line check
// reports each file by its lister (specs/phase01/diagnostics.md,
// *References*), and the editor shows what that prints.
func listsMappedFile(config *tsoptions.ParsedCommandLine, fileName string, path tspath.Path) bool {
	if _, extensions := contentmapper.BuiltInMappers(); !tspath.FileExtensionIsOneOf(fileName, extensions) {
		return true
	}
	_, listed := config.FileNamesByPath()[path]
	return listed
}
