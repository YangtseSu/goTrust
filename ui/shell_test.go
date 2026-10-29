// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// newShell renders the one window with the settings draft on page, at the
// window's default size. The returned state is the one the view reads, so a
// test can assert the page the sidebar chose.
func newShell(t *testing.T, settings *SettingsState, actions SettingsActions, page Page) (*State, *ui.Tester) {
	t.Helper()
	state := &State{Settings: settings, Page: page}
	if settings != nil {
		state.Config = settings.Draft
	}
	stateActions := Actions{
		Settings: actions,
		ShowPage: func(chosen Page) { state.Page = chosen },
	}
	tester := ui.NewTester(View(state, stateActions), 900, 1600)
	tester.Frame()
	return state, tester
}

// openPage clicks a page in the sidebar, then draws what the choice changed.
func openPage(t *testing.T, tester *ui.Tester, label string) {
	t.Helper()
	if err := tester.Click(label); err != nil {
		t.Fatalf("click the %s page: %v", label, err)
	}
	tester.Frame()
}
