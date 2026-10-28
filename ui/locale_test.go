// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/YangtseSu/goTrust/core/config"
	"github.com/YangtseSu/goTrust/core/session"
	"github.com/YangtseSu/goTrust/locales"
)

// The same state renders in the language mygo shows: MYGO_LOCALE is how a
// test asks for one without starting an app.
func TestViewsFollowTheAppLanguage(t *testing.T) {
	t.Setenv("MYGO_LOCALE", "en")

	state := State{
		Session: session.State{Phase: session.PhaseIdle},
		Config:  config.Default(),
	}
	tester := ui.NewTester(View(&state, Actions{}), 520, 560)
	tester.Frame()

	if !tester.HasText(locales.In("en").Connect()) {
		t.Fatalf("the English window has no %q; texts: %v",
			locales.In("en").Connect(), tester.Texts())
	}
	if tester.HasText(locales.In("zh").Connect()) {
		t.Fatalf("the English window still says %q", locales.In("zh").Connect())
	}
}

// The language picker reports the click it stores, not a value recomputed on
// the next frame: choosing English must call SetLocale("en") once.
func TestSettingsLanguagePickerCallsSetLocale(t *testing.T) {
	var chosen []string
	state := NewSettingsState(config.Default(), "/tmp/logs")
	tester := ui.NewTester(SettingsView(state, SettingsActions{
		SetLocale: func(tag string) { chosen = append(chosen, tag) },
	}), 600, 2200)
	tester.Frame()
	openSettingsPage(t, tester, "启动")

	if err := tester.Click("English"); err != nil {
		t.Fatalf("click English: %v; texts: %v", err, tester.Texts())
	}
	if len(chosen) != 1 || chosen[0] != "en" {
		t.Fatalf("SetLocale calls = %q, want [en]", chosen)
	}
}
