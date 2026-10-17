package cron

import (
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/api"
	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/services"
)

// RemoveExpiredTokensJob represents the cron job which periodically remove expired user tokens from the database
var RemoveExpiredTokensJob = &CronJob{
	Name:        "RemoveExpiredTokens",
	Description: "Periodically remove expired user tokens from the database.",
	Period: CronJobFixedHourPeriod{
		Hour: 0,
	},
	Run: func(c *core.CronContext) error {
		return services.Tokens.DeleteAllExpiredTokens(c)
	},
}

// CreateScheduledTransactionJob represents the cron job which periodically create transaction by scheduled transaction template
var CreateScheduledTransactionJob = &CronJob{
	Name:        "CreateScheduledTransaction",
	Description: "Periodically create transaction by scheduled transaction template.",
	Period: CronJobEvery15MinutesPeriod{
		Second: 0,
	},
	Run: func(c *core.CronContext) error {
		return services.Transactions.CreateScheduledTransactions(c, time.Now().Unix(), c.GetInterval())
	},
}

// ProcessBankMessageOutboxJob processes bank messages after the ingest request returns.
var ProcessBankMessageOutboxJob = &CronJob{
	Name:        "ProcessBankMessageOutbox",
	Description: "Process durable bank message outbox items.",
	Period: CronJobIntervalPeriod{
		Interval: time.Second,
	},
	Run: func(c *core.CronContext) error {
		return api.BankMessages.ProcessOutbox(c)
	},
}

// RefreshAssetPricesMidSessionJob represents the cron job which refreshes asset prices during the UAE trading day (06:00 server time, 10:00 UAE time when the server runs in UTC)
var RefreshAssetPricesMidSessionJob = &CronJob{
	Name:        "RefreshAssetPricesMidSession",
	Description: "Refresh the prices of held assets during the UAE trading day.",
	Period: CronJobFixedHourPeriod{
		Hour: 6,
	},
	Run: func(c *core.CronContext) error {
		return services.Assets.RefreshPricesOfAllUsers(c)
	},
}

// RefreshAssetPricesAfterCloseJob represents the cron job which refreshes asset prices after the UAE markets close (11:00 server time, 15:00 UAE time when the server runs in UTC)
var RefreshAssetPricesAfterCloseJob = &CronJob{
	Name:        "RefreshAssetPricesAfterClose",
	Description: "Refresh the prices of held assets after the UAE markets close.",
	Period: CronJobFixedHourPeriod{
		Hour: 11,
	},
	Run: func(c *core.CronContext) error {
		return services.Assets.RefreshPricesOfAllUsers(c)
	},
}

// SyncBinanceTradesJob represents the cron job which pulls new Binance spot trades for users who connected Binance (03:00 server time)
var SyncBinanceTradesJob = &CronJob{
	Name:        "SyncBinanceTrades",
	Description: "Pull new Binance spot trades for users who connected a read-only API key.",
	Period: CronJobFixedHourPeriod{
		Hour: 3,
	},
	Run: func(c *core.CronContext) error {
		return services.Assets.SyncBinanceTradesOfAllUsers(c)
	},
}
