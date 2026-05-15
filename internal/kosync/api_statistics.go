//
// File:        internal/kosync/api_statistics.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import (
	"git.obth.eu/atjontv/kosync/pkg/jmp"
	"github.com/gofiber/fiber/v3"
	"strconv"
)

var logApiStats = NewKlog("api/statistics")

func (app *Kosync) StatisticsRead(c fiber.Ctx) error {
	userId := c.Locals(CtxContextUserId).(string)
	daysStr := c.Query("days")
	days := 14
	if daysStr != "" {
		if d, err := strconv.Atoi(daysStr); err == nil && d >= 1 {
			days = d
		}
	}
	logApiStats.Debug("Fetching read statistics for user '%s' (%d days)", userId, days)
	stats, err := app.Db.GetReadStatistics(userId, days)
	if err != nil {
		logApiStats.Error("Failed to fetch read statistics: %v", err.Error())
		return err
	}
	return c.JSON(stats)
}

func (app *Kosync) RpcStatisticsRead(ctx *jmp.Context, payload *jmp.RpcRequestPayload) jmp.Result {
	userId := ctx.Data[CtxContextUserId].(string)
	days := 14
	if val, found := payload.Arguments["days"]; found {
		if d, ok := getRpcInt64(val); ok && d >= 1 {
			days = int(d)
		}
	}
	logApiStats.Debug("Fetching read statistics (RPC) for user '%s' (%d days)", userId, days)
	stats, err := app.Db.GetReadStatistics(userId, days)
	if err != nil {
		logApiStats.Error("Failed to fetch read statistics (RPC): %v", err.Error())
		return jmp.NewErrorResult([]string{err.Error()})
	}
	return jmp.NewOkResult("ReadStatistics", stats)
}
