// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package guards

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode"
)

// TestViewsSpeakThroughTheCatalog keeps a view from growing a literal again.
// The windows get their words from package ui and internal/gui, which say them
// through locales; a Chinese string in those packages is a message that only
// exists in one language.
func TestViewsSpeakThroughTheCatalog(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("locate the module root: %v", err)
	}
	var offenders []string
	for _, dir := range []string{"ui", "internal/gui"} {
		entries, err := os.ReadDir(filepath.Join(root, dir))
		if err != nil {
			t.Fatalf("read %s: %v", dir, err)
		}
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			path := filepath.Join(dir, name)
			file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, path), nil, 0)
			if err != nil {
				t.Fatalf("parse %s: %v", path, err)
			}
			ast.Inspect(file, func(node ast.Node) bool {
				lit, ok := node.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					return true
				}
				value, err := strconv.Unquote(lit.Value)
				if err != nil {
					value = lit.Value
				}
				if strings.IndexFunc(value, isHan) >= 0 {
					offenders = append(offenders, fmt.Sprintf("%s:%d %s", path, lit.Pos(), lit.Value))
				}
				return true
			})
		}
	}
	if len(offenders) > 0 {
		t.Errorf("views still carry literals that only exist in one language:\n%s",
			strings.Join(offenders, "\n"))
	}
}

// isHan reports whether r is a character the Chinese catalog writes.
func isHan(r rune) bool {
	return unicode.Is(unicode.Han, r)
}

// TestCatalogsCarryTheSameKeys is the other half: a language that lacks a
// message silently falls back to the source, so the gap has to fail here.
func TestCatalogsCarryTheSameKeys(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("locate the module root: %v", err)
	}
	keys := map[string]map[string]bool{}
	for _, lang := range []string{"zh", "en"} {
		body, err := os.ReadFile(filepath.Join(root, "locales", lang+".json"))
		if err != nil {
			t.Fatalf("read the %s catalog: %v", lang, err)
		}
		var messages map[string]any
		if err := json.Unmarshal(body, &messages); err != nil {
			t.Fatalf("parse the %s catalog: %v", lang, err)
		}
		set := make(map[string]bool, len(messages))
		for key := range messages {
			if strings.HasPrefix(key, "@") {
				continue
			}
			set[key] = true
		}
		if len(set) == 0 {
			t.Fatalf("the %s catalog has no messages", lang)
		}
		keys[lang] = set
	}

	for _, pair := range [][2]string{{"zh", "en"}, {"en", "zh"}} {
		source, other := pair[0], pair[1]
		var missing []string
		for key := range keys[source] {
			if !keys[other][key] {
				missing = append(missing, key)
			}
		}
		slices.Sort(missing)
		if len(missing) > 0 {
			t.Errorf("%s lacks %d message(s) of %s: %s",
				other, len(missing), source, strings.Join(missing, ", "))
		}
	}
}
