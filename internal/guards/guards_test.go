// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package guards holds the dependency invariants of the two front-ends. It is
// a test-only package: nothing imports it, and it imports nothing but the
// standard library, so it cannot itself break the rules it checks.
package guards

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const module = "github.com/YangtseSu/goTrust"

// dependencies lists the build graph of one package pattern, with the module
// root as the working directory. Test-only imports are not part of the graph,
// which is what the binaries are actually built from.
func dependencies(t *testing.T, pattern string) []string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("locate the module root: %v", err)
	}
	command := exec.Command("go", "list", "-deps", "-f", "{{.ImportPath}}", pattern)
	command.Dir = root
	output, err := command.Output()
	if err != nil {
		t.Fatalf("go list %s: %v", pattern, err)
	}
	deps := strings.Fields(string(output))
	if len(deps) == 0 {
		t.Fatalf("go list %s listed nothing", pattern)
	}
	return deps
}

// hasPrefix reports whether any dependency sits under path. Subpackages count:
// importing github.com/egoist/mygo/ui brings github.com/egoist/mygo too, and a
// guard on the root alone would miss a direct import of a subpackage.
func hasPrefix(deps []string, path string) bool {
	for _, dep := range deps {
		if dep == path || strings.HasPrefix(dep, path+"/") {
			return true
		}
	}
	return false
}

// require reports every forbidden import the graph carries.
func require(t *testing.T, pattern string, deps []string, forbidden ...string) {
	t.Helper()
	for _, path := range forbidden {
		if hasPrefix(deps, path) {
			t.Errorf("the build graph of %s reaches %s, which the front-end split forbids", pattern, path)
		}
	}
}

// TestCoreCarriesNoToolkit is what makes core/ a reusable layer: a second
// front-end must be able to build against it without GTK, mygo or purego.
func TestCoreCarriesNoToolkit(t *testing.T) {
	deps := dependencies(t, "./core/...")
	require(t, "./core/...", deps,
		"github.com/egoist/mygo",
		"github.com/ebitengine/purego",
		module+"/ui",
		module+"/internal/cli",
	)
	// The protocol layer is the largest part of core; a guard that listed an
	// empty graph would pass vacuously.
	if !hasPrefix(deps, module+"/core/session") || !hasPrefix(deps, module+"/core/atrust") {
		t.Fatal("the core dependency graph is missing the session or the protocol layer")
	}
}

// TestCLIImportsCoreOnly keeps the headless binary buildable on a host with no
// GUI toolkit and no display.
func TestCLIImportsCoreOnly(t *testing.T) {
	deps := dependencies(t, "./cmd/gotrust-cli")
	require(t, "./cmd/gotrust-cli", deps,
		"github.com/egoist/mygo",
		"github.com/ebitengine/purego",
		module+"/ui",
		module+"/internal/gui",
		module+"/cmd/gotrust",
	)
	if !hasPrefix(deps, module+"/internal/cli") {
		t.Fatal("the CLI does not use internal/cli")
	}
}

// TestGUIImportsNoHeadlessFrontEnd is the other direction: the tray app keeps
// one headless implementation, and it is the CLI.
func TestGUIImportsNoHeadlessFrontEnd(t *testing.T) {
	deps := dependencies(t, "./cmd/gotrust")
	require(t, "./cmd/gotrust", deps,
		module+"/internal/cli",
		module+"/cmd/gotrust-cli",
	)
	if !hasPrefix(deps, module+"/ui") {
		t.Fatal("the GUI does not reach its own views")
	}
	if !hasPrefix(deps, module+"/internal/gui") {
		t.Fatal("the tray binary does not reach internal/gui")
	}
}

// directImports lists the packages one package names in its own files, test
// files included. A transitive check cannot express "may use the session layer
// but not the protocol layer": core/session itself is built on the protocol
// layer, so every front-end reaches it through the session's graph.
func directImports(t *testing.T, pattern string) []string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("locate the module root: %v", err)
	}
	command := exec.Command("go", "list", "-f", `{{.ImportPath}}: {{join .Imports " "}} {{join .TestImports " "}}`, pattern)
	command.Dir = root
	output, err := command.Output()
	if err != nil {
		t.Fatalf("go list %s: %v", pattern, err)
	}

	var imports []string
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		_, list, found := strings.Cut(line, ":")
		if !found {
			t.Fatalf("go list %s: unexpected line %q", pattern, line)
		}
		imports = append(imports, strings.Fields(list)...)
	}
	if len(imports) == 0 {
		t.Fatalf("go list %s listed no imports", pattern)
	}
	return imports
}

// TestFrontEndsStayOffTheProtocolLayer is what "every business call goes
// through core/session" means mechanically: the two front-ends import the
// session, the config, the proxy and the logging packages, never a package of
// the protocol layer. core/prompt/web is part of core, not a front-end, and
// adapts the protocol's own challenge types for the CLI.
func TestFrontEndsStayOffTheProtocolLayer(t *testing.T) {
	protocol := module + "/core/atrust"
	for _, pattern := range []string{"./internal/gui", "./internal/cli"} {
		imports := directImports(t, pattern)
		require(t, pattern, imports, protocol)
		if !hasPrefix(imports, module+"/core/session") {
			t.Fatalf("%s does not import core/session", pattern)
		}
	}
}
