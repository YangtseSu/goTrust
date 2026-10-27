// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package session

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/YangtseSu/goTrust/core/atrust/client"
	"github.com/YangtseSu/goTrust/core/atrust/client/atrust/auth"

	"github.com/YangtseSu/goTrust/core/config"
)

// AuthMethod is one authentication method the gateway offers before login.
// aTrust matches the type and the domain as a pair, so both are part of the
// identity of a method and not two independent settings.
type AuthMethod struct {
	// Name is the gateway's label. It falls back to the type when the gateway
	// sends none, so a chooser never shows an empty button.
	Name string
	// Type is the authType the login sends, for example auth/psw.
	Type string
	// Domain is the loginDomain that belongs to the type.
	Domain string
	// URL is where an external login (CAS or OAuth2) happens. It is empty for
	// the methods completed inside the app.
	URL string
}

// String renders the pair the way the settings window and the error messages
// show it.
func (m AuthMethod) String() string {
	if m.Name != "" && m.Name != m.Type {
		return fmt.Sprintf("%s (%s · %s)", m.Name, m.Type, m.Domain)
	}
	return fmt.Sprintf("%s · %s", m.Type, m.Domain)
}

// Matches reports whether the method is the saved pair. An empty saved type
// means "pick automatically" and matches nothing.
func (m AuthMethod) Matches(authType, domain string) bool {
	return authType != "" && m.Type == authType && m.Domain == domain
}

// ErrAuthMethodChoice reports that the gateway offers several methods and the
// user has not stored one. The caller asks the user which one to use.
var ErrAuthMethodChoice = errors.New("the gateway offers several authentication methods")

// DiscoverAuthMethods asks the gateway for its pre-login method list. It is
// the same authConfig request the client makes while logging in, so a gateway
// that answers the login also answers this.
//
// An empty list is a valid answer: a gateway that considers the client already
// logged in offers nothing, and the client then restores the stored session.
// The caller must not read it as an error.
//
// The call dials the gateway directly, like a login does: it must not be
// captured by a tunnel that is already up. The context stops the request.
func DiscoverAuthMethods(ctx context.Context, profile config.Profile) ([]AuthMethod, error) {
	address := profile.ServerAddress()
	if address == "" {
		return nil, errors.New("server address is empty")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return discoverAuthMethods(ctx, authHost(address, profile.Server.Port), (&net.Dialer{}).DialContext)
}

// authHost is the host a login sends to the gateway. The client omits the port
// at 443, and the certificate is checked against exactly this name, so
// discovery has to use the same form.
func authHost(address string, port int) string {
	if port != 443 {
		return net.JoinHostPort(address, strconv.Itoa(port))
	}
	if ip := net.ParseIP(address); ip != nil && ip.To4() == nil {
		return "[" + address + "]"
	}
	return address
}

// discoverAuthMethods is the part that can be pointed at a test server.
func discoverAuthMethods(ctx context.Context, host string, dial client.DialContextFunc) ([]AuthMethod, error) {
	session := auth.NewSessionContext(ctx, host, nil, dial)
	infos, err := session.GetAuthInfoList()
	if err != nil {
		return nil, err
	}

	methods := make([]AuthMethod, 0, len(infos))
	seen := make(map[string]bool, len(infos))
	for _, info := range infos {
		if strings.TrimSpace(info.AuthType) == "" {
			continue
		}
		key := info.AuthType + "\x00" + info.LoginDomain
		if seen[key] {
			continue
		}
		seen[key] = true

		name := strings.TrimSpace(info.AuthName)
		if name == "" {
			name = info.AuthType
		}
		methods = append(methods, AuthMethod{
			Name:   name,
			Type:   info.AuthType,
			Domain: info.LoginDomain,
			URL:    info.LoginURL,
		})
	}
	return methods, nil
}

// ResolveAuthMethod picks the pair a login sends.
//
// A saved pair wins, but it has to be offered: a gateway that dropped the
// method must fail with the list instead of silently logging in another way.
// Without a saved pair the only offered method is unambiguous; several are a
// question for the user, reported as ErrAuthMethodChoice.
func ResolveAuthMethod(methods []AuthMethod, authType, domain string) (AuthMethod, error) {
	if len(methods) == 0 {
		return AuthMethod{}, errors.New("the gateway did not offer an authentication method")
	}

	if authType != "" {
		for _, method := range methods {
			if method.Matches(authType, domain) {
				return method, nil
			}
		}
		return AuthMethod{}, fmt.Errorf("the gateway no longer offers %s · %s; it offers %s",
			authType, domain, methodList(methods))
	}

	if len(methods) == 1 {
		return methods[0], nil
	}
	return AuthMethod{}, fmt.Errorf("%w: %s", ErrAuthMethodChoice, methodList(methods))
}

// methodList joins the offered pairs for an error message or a status line.
func methodList(methods []AuthMethod) string {
	parts := make([]string, len(methods))
	for index, method := range methods {
		parts[index] = method.Type + " · " + method.Domain
	}
	return strings.Join(parts, ", ")
}
