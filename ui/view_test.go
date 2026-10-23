package ui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/YangtseSu/goTrust/config"
	"github.com/YangtseSu/goTrust/session"
)

// The view is a pure function of state, so it renders without a window: this
// catches the mistakes that would otherwise only appear when a user opens the
// app, such as a panic in a nil callback or a label that never changes.
func TestViewRendersEveryPhase(t *testing.T) {
	phases := []session.Phase{
		session.PhaseIdle,
		session.PhaseConnecting,
		session.PhaseOnline,
		session.PhaseFailed,
	}

	for _, phase := range phases {
		t.Run(string(phase), func(t *testing.T) {
			state := State{
				Session: session.State{
					Phase:    phase,
					Detail:   "detail for " + string(phase),
					ClientIP: "192.0.2.10",
				},
				Config:   config.Default(),
				Server:   "vpn.example.com",
				Port:     "443",
				Username: "alice",
			}

			tester := ui.NewTester(View(&state, Actions{}), 520, 560)
			tester.Frame()

			if texts := tester.Texts(); len(texts) == 0 {
				t.Fatal("the view rendered no text")
			}
			if !tester.HasText("goTrust") {
				t.Fatal("the title is missing")
			}
			if !tester.HasText(phaseLabel(state.Session)) {
				t.Fatalf("the phase label %q is missing", phaseLabel(state.Session))
			}
		})
	}
}

// The password must not be painted as readable text.
func TestPasswordIsNotShownAsText(t *testing.T) {
	state := State{
		Session:  session.State{Phase: session.PhaseIdle, Detail: "x"},
		Config:   config.Default(),
		Server:   "vpn.example.com",
		Port:     "443",
		Username: "alice",
		Password: "hunter2",
		Domain:   "local",
	}

	tester := ui.NewTester(View(&state, Actions{}), 520, 560)
	tester.Frame()

	if tester.HasText("hunter2") {
		t.Fatal("the password appears as readable text")
	}
	if !tester.HasText("alice") && !tester.HasText("用户名") {
		t.Fatal("the password test rendered an unexpected view")
	}
	if !tester.HasText("登录域") {
		t.Fatal("the login domain field is missing")
	}
}

// Clicking 连接 must reach the action, and clicking 断开 in the online phase
// must reach Logout instead. A swapped wiring would lock the user out.
func TestButtonsCallTheRightAction(t *testing.T) {
	cases := []struct {
		phase      session.Phase
		button     string
		wantLogin  bool
		wantLogout bool
	}{
		{phase: session.PhaseIdle, button: "连接", wantLogin: true},
		{phase: session.PhaseFailed, button: "连接", wantLogin: true},
		{phase: session.PhaseOnline, button: "断开", wantLogout: true},
	}

	for _, testCase := range cases {
		t.Run(string(testCase.phase), func(t *testing.T) {
			var login, logout int
			state := State{
				Session:  session.State{Phase: testCase.phase, Detail: "x"},
				Config:   config.Default(),
				Server:   "vpn.example.com",
				Port:     "443",
				Username: "alice",
			}
			actions := Actions{
				Login:  func() { login++ },
				Logout: func() { logout++ },
			}

			tester := ui.NewTester(View(&state, actions), 520, 560)
			if err := tester.Click(testCase.button); err != nil {
				t.Fatalf("click %q: %v", testCase.button, err)
			}

			if testCase.wantLogin && login != 1 {
				t.Fatalf("login calls = %d, want 1", login)
			}
			if testCase.wantLogout && logout != 1 {
				t.Fatalf("logout calls = %d, want 1", logout)
			}
		})
	}
}

// The listener addresses are what the user copies into another application, so
// the view has to show every one of them.
func TestAddressesAreListed(t *testing.T) {
	state := State{
		Session:   session.State{Phase: session.PhaseOnline, Detail: "vpn.example.com:443"},
		Config:    config.Default(),
		Server:    "vpn.example.com",
		Port:      "443",
		Addresses: []string{"127.0.0.1:1080", "127.0.0.1:1081"},
	}

	tester := ui.NewTester(View(&state, Actions{}), 520, 560)
	tester.Frame()

	for _, address := range state.Addresses {
		if !tester.HasText("代理监听 " + address) {
			t.Fatalf("address %q is missing from the view", address)
		}
	}
}
