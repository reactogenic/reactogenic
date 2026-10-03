package mapper

import "github.com/microsoft/TypeScript/tsc/rtsx"

// Transform is the transform for a host that is not the fork's program: the
// stock content mapper (ide.md, *Stock TypeScript 7.1*). The virtual text
// and the File are the built-in mapper's; File.Map is nil when the source
// stands in as its own virtual text.
func Transform(fileName, content string, readFile func(path string) (string, bool)) (string, *File) {
	result := transform(rtsx.MapperRequest{FileName: fileName, Content: content, ReadFile: readFile})
	return result.Text, result.Extra.(*File)
}
