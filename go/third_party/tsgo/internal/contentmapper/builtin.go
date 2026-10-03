// Reactogenic: a content mapper that runs in this process.
// Not part of upstream: added by go/patches/0004-rtsx-mapper.patch.

package contentmapper

import (
	"sync/atomic"

	"github.com/microsoft/TypeScript/tsc/internal/locale"
	"github.com/microsoft/TypeScript/tsc/internal/vfs"
)

// BuiltIn is a content mapper compiled into the binary. Once registered it
// serves its extensions in every project — configured, referenced and
// inferred — without a tsconfig entry and without runExternalCode: nothing
// external runs. Configured mappers are ignored while one is registered.
type BuiltIn struct {
	Name, Version string
	Extensions    []string
	// Transform never fails: user errors yield a usable result.
	Transform func(BuiltInRequest) Result
	// FileIdentity is what a file's transform depends on besides its own
	// text (the names of its siblings); it becomes part of the cache key.
	FileIdentity func(BuiltInRequest) string

	// mappers is the registration as config-level mappers: made once, since
	// a project compares its mappers with a snapshot's by identity — a new
	// one for every snapshot would rebuild the inferred project each time.
	mappers []*Mapper
}

// BuiltInRequest is a file to transform and the file system to read its
// surroundings from: in the language server, the snapshot's, with open
// buffers.
type BuiltInRequest struct {
	FileName string
	Content  string
	FS       vfs.FS
}

var builtIn atomic.Pointer[BuiltIn]

// RegisterBuiltIn installs b for the process; call it before any program is
// built. nil removes it.
func RegisterBuiltIn(b *BuiltIn) {
	if b != nil {
		b.mappers = []*Mapper{{
			Definition: Definition{Package: b.Name, Extensions: b.Extensions},
			Manifest:   Manifest{Name: b.Name, Version: b.Version},
		}}
	}
	builtIn.Store(b)
}

// BuiltInMappers returns the registered mapper as config-level mappers, with
// its extensions; nil when none is registered. The same slice every time: it
// is not to be changed.
func BuiltInMappers() ([]*Mapper, []string) {
	b := builtIn.Load()
	if b == nil {
		return nil, nil
	}
	return b.mappers, b.Extensions
}

// FileIdentifier is implemented by a Project whose transforms depend on
// more than the file's text.
type FileIdentifier interface {
	FileIdentity(fileName, content string) string
}

type builtInProject struct{ fs vfs.FS }

// NewBuiltInProject is the Project of the registered built-in mapper,
// reading through fs.
func NewBuiltInProject(fs vfs.FS) Project { return &builtInProject{fs: fs} }

func (p *builtInProject) identity() string {
	if b := builtIn.Load(); b != nil {
		return b.Name + "@" + b.Version
	}
	return ""
}

func (p *builtInProject) Refresh() error                   { return nil }
func (p *builtInProject) Identities() ([]string, error)    { return []string{p.identity()}, nil }
func (p *builtInProject) Identity(*Mapper) (string, error) { return p.identity(), nil }
func (p *builtInProject) WatchedFiles() ([]string, error)  { return nil, nil }
func (p *builtInProject) Diagnostics() []OptionDiagnostic  { return nil }
func (p *builtInProject) Close() error                     { return nil }
func (p *builtInProject) Transform(_ *Mapper, request Request) (Result, error) {
	b := builtIn.Load()
	if b == nil {
		return Result{}, ErrProjectUnavailable
	}
	return b.Transform(BuiltInRequest{FileName: request.FileName, Content: request.Content, FS: p.fs}), nil
}

func (p *builtInProject) FileIdentity(fileName, content string) string {
	if b := builtIn.Load(); b != nil && b.FileIdentity != nil {
		return b.FileIdentity(BuiltInRequest{FileName: fileName, Content: content, FS: p.fs})
	}
	return ""
}

type builtInHost struct{}

// NewBuiltInHost is the Host of the registered built-in mapper: it spawns
// nothing.
func NewBuiltInHost() Host { return builtInHost{} }

func (builtInHost) Timings() Timings                   { return Timings{} }
func (builtInHost) Project(spec ProjectSpec) Project   { return NewBuiltInProject(spec.FS) }
func (builtInHost) Acquire([]*Mapper) (release func()) { return func() {} }
func (builtInHost) SetLocale(locale.Locale)            {}
func (builtInHost) Close() error                       { return nil }
func (builtInHost) Transform(_ *Mapper, request Request) (Result, error) {
	return NewBuiltInProject(nil).Transform(nil, request)
}
