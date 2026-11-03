package atrust

import (
	"io"

	"github.com/YangtseSu/goTrust/core/atrust/client/atrust/auth"
)

// testUserAgent is the identity a test client presents. The protocol layer has
// no UA of its own, so every session a test builds has to name one.
const testUserAgent = "goTrust test client"

func newAuthTestSession(host string, writer io.Writer) *auth.Session {
	session := auth.NewSession(host, writer)
	session.SetUserAgent(testUserAgent)
	session.SetPlatform("Linux")
	return session
}
