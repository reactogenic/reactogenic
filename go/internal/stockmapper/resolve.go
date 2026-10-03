package stockmapper

import (
	"encoding/json"
	"path"
	"sort"
	"strings"
)

// Stock TypeScript does not find a mapped file through an extensionless
// import (`./button` → `button.rtsx`): "content mappers don't participate in
// module resolution" (microsoft/TypeScript#64546). In our own hosts the
// fork's resolver does (go/patches/0005-rtsx-resolver.patch). Here the
// mapper writes the extension into the virtual text of the files it owns,
// by the same rule: an .rtsx file is found after every built-in extension,
// as a file before a directory's index, relative or through `paths`. Not
// through node_modules: a package's .rtsx files are imported by their full
// names.

const mappedExtension = ".rtsx"

// builtInExtensions are what TypeScript appends to an extensionless import;
// a file that one of them finds wins over `.rtsx`.
var builtInExtensions = []string{".ts", ".tsx", ".d.ts", ".js", ".jsx"}

// project is what a transform needs of its tsconfig: the `paths` aliases.
type project struct {
	base     string // what `paths` substitutions are relative to
	exact    map[string][]string
	patterns []pathPattern // with one `*`; the longest prefix first
}

type pathPattern struct {
	prefix, suffix string
	substitutions  []string
}

func newProject(p openProjectParams) *project {
	// The host sends its whole effective CompilerOptions; only these matter.
	var options struct {
		Paths          map[string][]string `json:"paths"`
		PathsBasePath  string              `json:"pathsBasePath"`
		ConfigFilePath string              `json:"configFilePath"`
	}
	_ = json.Unmarshal(p.CompilerOptions, &options) // unreadable options: no aliases
	proj := &project{base: options.PathsBasePath, exact: map[string][]string{}}
	if proj.base == "" {
		config := options.ConfigFilePath
		if config == "" {
			config = p.ConfigFileName
		}
		if config != "" {
			proj.base = path.Dir(config)
		}
	}
	for pattern, substitutions := range options.Paths {
		prefix, suffix, star := strings.Cut(pattern, "*")
		if !star {
			proj.exact[pattern] = substitutions
			continue
		}
		proj.patterns = append(proj.patterns, pathPattern{prefix, suffix, substitutions})
	}
	sort.Slice(proj.patterns, func(i, j int) bool {
		a, b := proj.patterns[i], proj.patterns[j]
		if len(a.prefix) != len(b.prefix) {
			return len(a.prefix) > len(b.prefix)
		}
		return a.prefix+"*"+a.suffix < b.prefix+"*"+b.suffix
	})
	return proj
}

// candidates returns the paths a `paths` alias gives a non-relative
// specifier: TypeScript's match — an exact pattern, else the one with the
// longest prefix — with the `*` filled in.
func (p *project) candidates(specifier string) []string {
	if p == nil || p.base == "" {
		return nil
	}
	substitutions, star, matched := p.exact[specifier], "", false
	if substitutions != nil {
		matched = true
	}
	for _, pattern := range p.patterns {
		if matched {
			break
		}
		if len(specifier) >= len(pattern.prefix)+len(pattern.suffix) &&
			strings.HasPrefix(specifier, pattern.prefix) && strings.HasSuffix(specifier, pattern.suffix) {
			substitutions, star, matched = pattern.substitutions, specifier[len(pattern.prefix):len(specifier)-len(pattern.suffix)], true
		}
	}
	var paths []string
	for _, substitution := range substitutions {
		target := strings.Replace(substitution, "*", star, 1)
		if !path.IsAbs(target) && !isDriveAbsolute(target) {
			target = path.Join(p.base, target)
		}
		paths = append(paths, path.Clean(target))
	}
	return paths
}

// isDriveAbsolute: `C:/…`, as the host writes Windows paths.
func isDriveAbsolute(p string) bool {
	return len(p) >= 3 && p[1] == ':' && p[2] == '/'
}

// explicitImport returns how to rewrite the specifier of an import written
// in the file fileName so that it names the .rtsx file our own resolver
// would find for it: a suffix to append, or — for an alias that no longer
// matches its `paths` pattern with the suffix on (an exact alias, a pattern
// with text after its `*`) — a relative path to that file, in place of the
// whole specifier. Both are "" when the import is fine as written: it finds
// a built-in file, names its extension already, or finds nothing.
func (s *server) explicitImport(fileName, specifier string, proj *project) (suffix, whole string) {
	if specifier == "" {
		return "", ""
	}
	directoryOnly := strings.HasSuffix(specifier, "/")
	if isRelative(specifier) {
		last := specifier[strings.LastIndex(specifier, "/")+1:]
		suffix, _ := s.mappedSuffix(path.Join(path.Dir(fileName), specifier), directoryOnly || last == "." || last == "..")
		return joinSuffix(specifier, suffix), ""
	}
	for _, candidate := range proj.candidates(specifier) {
		if s.opts.FileExists(candidate) {
			return "", "" // a substitution that names its file
		}
		suffix, found := s.mappedSuffix(candidate, directoryOnly)
		if !found {
			continue
		}
		if suffix == "" {
			return "", ""
		}
		target := candidate + suffix
		if suffix = joinSuffix(specifier, suffix); s.aliasFinds(proj, specifier+suffix, target) {
			return suffix, ""
		}
		return "", relativeSpecifier(path.Dir(fileName), target)
	}
	return "", ""
}

// aliasFinds: TypeScript, resolving specifier through `paths`, reaches the
// file target before anything else.
func (s *server) aliasFinds(proj *project, specifier, target string) bool {
	for _, candidate := range proj.candidates(specifier) {
		if candidate == target {
			return true
		}
		if s.opts.FileExists(candidate) {
			return false
		}
		if _, found := s.mappedSuffix(candidate, false); found {
			return false
		}
	}
	return false
}

// relativeSpecifier names the file target from a file in fromDir: `./x`,
// `../x`; the absolute path when they share no root (two Windows drives).
func relativeSpecifier(fromDir, target string) string {
	from, to := strings.Split(strings.TrimSuffix(fromDir, "/"), "/"), strings.Split(target, "/")
	common := 0
	for common < len(from) && common < len(to)-1 && from[common] == to[common] {
		common++
	}
	if common == 0 {
		return target
	}
	if common == len(from) {
		return "./" + strings.Join(to[common:], "/")
	}
	return strings.Repeat("../", len(from)-common) + strings.Join(to[common:], "/")
}

func isRelative(specifier string) bool {
	return specifier == "." || specifier == ".." || strings.HasPrefix(specifier, "./") || strings.HasPrefix(specifier, "../")
}

func joinSuffix(specifier, suffix string) string {
	if strings.HasSuffix(specifier, "/") {
		return strings.TrimPrefix(suffix, "/")
	}
	return suffix
}

// mappedSuffix looks for the module at candidate, a path without an
// extension, in the resolver's order. found: something is there; suffix:
// it is an .rtsx file, named by candidate + suffix.
func (s *server) mappedSuffix(candidate string, directoryOnly bool) (suffix string, found bool) {
	exists := s.opts.FileExists
	if !directoryOnly {
		for _, ext := range builtInExtensions {
			if exists(candidate + ext) {
				return "", true
			}
		}
		if exists(candidate + mappedExtension) {
			return mappedExtension, true
		}
	}
	// A directory: the entry its package.json names is TypeScript's
	// business; else its index.
	if s.namesEntry(candidate + "/package.json") {
		return "", true
	}
	for _, ext := range builtInExtensions {
		if exists(candidate + "/index" + ext) {
			return "", true
		}
	}
	if exists(candidate + "/index" + mappedExtension) {
		return "/index" + mappedExtension, true
	}
	return "", false
}

// namesEntry: the package.json has one of the fields TypeScript reads a
// directory's entry from when the directory is imported by its path. One
// without them — or one that is not JSON — leaves the directory to its
// index.
func (s *server) namesEntry(packageJSON string) bool {
	if !s.opts.FileExists(packageJSON) {
		return false
	}
	text, ok := s.opts.ReadFile(packageJSON)
	if !ok {
		return false
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal([]byte(text), &fields) != nil {
		return false
	}
	for _, field := range []string{"types", "typings", "main", "typesVersions"} {
		if _, named := fields[field]; named {
			return true
		}
	}
	return false
}

// extensionless returns the specifier of a segment's import without its
// `.tsx` / `.ts`, when TypeScript finds the same file that way: no sibling
// with an extension it tries first.
func (s *server) extensionless(fileName, specifier string) (string, bool) {
	ext := path.Ext(specifier)
	bare := strings.TrimSuffix(specifier, ext)
	base := path.Join(path.Dir(fileName), bare)
	for _, tried := range builtInExtensions {
		if s.opts.FileExists(base + tried) {
			return bare, tried == ext
		}
	}
	return "", false
}
