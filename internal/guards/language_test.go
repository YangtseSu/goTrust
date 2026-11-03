// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package guards

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestChineseCatalogNamesItsScript keeps the Chinese catalog's tag
// script-qualified. The tag a window shows reaches the text engine as the
// language of its text (mygo's text.Style.Lang), and Windows' DirectWrite picks
// the fallback font for Han characters from that tag: zh-Hans and zh-Hant have
// entries, a bare zh has none and takes the system's default, which is a
// Japanese font. Renaming the file back to zh.json would silently undo the
// choice, and no test of the views can see the glyphs.
func TestChineseCatalogNamesItsScript(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("locate the module root: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(root, "mygo.json"))
	if err != nil {
		t.Fatalf("read mygo.json: %v", err)
	}
	var config struct {
		I18n struct {
			Dir    string `json:"dir"`
			Source string `json:"source"`
		} `json:"i18n"`
	}
	if err := json.Unmarshal(body, &config); err != nil {
		t.Fatalf("parse mygo.json: %v", err)
	}
	if config.I18n.Source != "zh-Hans" {
		t.Errorf("the source language is %q, want zh-Hans: a bare zh has no Han "+
			"fallback font on Windows", config.I18n.Source)
	}
	entries, err := os.ReadDir(filepath.Join(root, config.I18n.Dir))
	if err != nil {
		t.Fatalf("read %s: %v", config.I18n.Dir, err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		if tag := strings.TrimSuffix(name, ".json"); tag == "zh" {
			t.Errorf("%s is a bare zh tag: name it zh-Hans.json", name)
		}
	}
}
