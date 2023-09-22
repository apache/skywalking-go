package frameworks

import (
	"github.com/apache/skywalking-go/tools/go-agent/instrument/plugins/rewrite"
	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
)

type Buildin struct {
}

func NewBuildin() *Buildin {
	return &Buildin{}
}

func (l *Buildin) Name() string {
	return "buildin"
}

func (l *Buildin) PackagePaths() map[string]*PackageConfiguration {
	return map[string]*PackageConfiguration{"log": {NeedsHelpers: true, NeedsVariables: true, NeedsChangeLoggerFunc: true}}
}

func (l *Buildin) AutomaticBindFunctions(fun *dst.FuncDecl) string {
	return ""
}

func (l *Buildin) GenerateExtraFiles(pkgPath, debugDir string) (info []*rewrite.FileInfo, err error) {
	return
}

func (l *Buildin) CustomizedEnhance(path string, curFile *dst.File, cursor *dstutil.Cursor, allFiles []*dst.File) (cache map[string]string, ok bool) {
	return
}

func (l *Buildin) InitFunctions() []*dst.FuncDecl {
	return nil
}

func (l *Buildin) InitImports() []*dst.ImportSpec {
	return nil
}
