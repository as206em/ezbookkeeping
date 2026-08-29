package middlewares

import (
	"crypto/subtle"
	"strings"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/settings"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
)

const BankMessageAPIKeyHeaderName = "X-API-Key"

// BankMessageAPIKeyAuthorization verifies the dedicated bank-message ingestion key.
func BankMessageAPIKeyAuthorization(config *settings.Config) core.MiddlewareHandlerFunc {
	return func(c *core.WebContext) {
		expected := config.BankMessageAPIKey
		actual := strings.TrimSpace(c.GetHeader(BankMessageAPIKeyHeaderName))

		if expected == "" || actual == "" || len(expected) != len(actual) || subtle.ConstantTimeCompare([]byte(expected), []byte(actual)) != 1 {
			utils.PrintJsonErrorResult(c, errs.ErrUnauthorizedAccess)
			return
		}

		c.Next()
	}
}
