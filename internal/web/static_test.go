package web

import (
	"io/fs"
	"path"
	"regexp"
	"strings"
	"testing"
)

// There is no JavaScript toolchain to catch a module that imports a file that
// is not there, or a name another module does not export, so this does.
func TestEveryModuleImportsWhatExists(t *testing.T) {
	js, err := fs.Sub(staticFiles, "static/js")
	if err != nil {
		t.Fatal(err)
	}
	files, err := fs.Glob(js, "*.js")
	if err != nil || len(files) == 0 {
		t.Fatalf("no modules: %v", err)
	}
	src := map[string]string{}
	for _, f := range files {
		b, err := fs.ReadFile(js, f)
		if err != nil {
			t.Fatal(err)
		}
		src[f] = string(b)
	}
	imports := regexp.MustCompile(`(?m)^import .*$`)
	starNamed := regexp.MustCompile(`import\s*\*\s*as\s+(\w+)\s+from\s*'\./([\w-]+\.js)'`)
	named := regexp.MustCompile(`import\s*\{([^}]*)\}\s*from\s*'\./([\w-]+\.js)'`)
	star := regexp.MustCompile(`import\s*\*\s*as\s+\w+\s+from\s*'\./([\w-]+\.js)'`)
	for f, s := range src {
		// ns.name must be something the module exports
		for _, m := range starNamed.FindAllStringSubmatch(s, -1) {
			from, ok := src[m[2]]
			if !ok {
				continue
			}
			use := regexp.MustCompile(`(?:^|[^\w.])` + m[1] + `\.(\w+)`)
			body := imports.ReplaceAllString(s, "")
			for _, u := range use.FindAllStringSubmatch(body, -1) {
				if !exports(from, u[1]) {
					t.Errorf("%s uses %s.%s, but %s does not export %s", f, m[1], u[1], m[2], u[1])
				}
			}
		}
		for _, m := range star.FindAllStringSubmatch(s, -1) {
			if _, ok := src[m[1]]; !ok {
				t.Errorf("%s imports %s, which is not there", f, m[1])
			}
		}
		for _, m := range named.FindAllStringSubmatch(s, -1) {
			from, ok := src[m[2]]
			if !ok {
				t.Errorf("%s imports %s, which is not there", f, m[2])
				continue
			}
			for _, name := range strings.Split(m[1], ",") {
				name = strings.TrimSpace(strings.SplitN(name, " as ", 2)[0])
				if name == "" {
					continue
				}
				if !exports(from, name) {
					t.Errorf("%s imports %s from %s, which does not export it", f, name, m[2])
				}
			}
		}
	}
	for _, f := range []string{"index.html", "app.css", "js/main.js"} {
		if _, err := fs.Stat(staticFiles, path.Join("static", f)); err != nil {
			t.Errorf("the page needs %s: %v", f, err)
		}
	}
}

// exports reports whether the module source exports the name.
func exports(source, name string) bool {
	return regexp.MustCompile(`export\s+(async\s+)?(function|const|let|class)\s+`+regexp.QuoteMeta(name)+`\b`).MatchString(source) ||
		regexp.MustCompile(`export\s*\{[^}]*\b`+regexp.QuoteMeta(name)+`\b[^}]*\}`).MatchString(source)
}
