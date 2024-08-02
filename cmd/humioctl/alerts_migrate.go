// Copyright © 2020 Humio Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"errors"
	"fmt"
	"github.com/humio/cli/api"
	"github.com/spf13/cobra"
	"regexp"
	"strconv"
	"time"
)

func newMigrateAllLegacyAlerts() *cobra.Command {
	cmd := cobra.Command{
		Use:   "migrate-all [flags] <view>",
		Short: "Attempt to migrate all legacy alerts to aggregate alerts",
		Long:  `Attempt to migrate all legacy alert to aggregate alerts. If the a legacy migration is successful, the legacy alert will be deleted.`,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			client := NewApiClient(cmd)
			viewName := args[0]

			allLegacyAlerts, err := client.Alerts().List(viewName)
			if err != nil {
				exitOnError(cmd, err, "could not list legacy alerts")
			}
			cmd.Printf("[INFO] found %d legacy alerts to migrate...\n", len(allLegacyAlerts))

			for i, legacyAlert := range allLegacyAlerts {
				migrateLegacyAlert(legacyAlert, cmd, client, viewName, i+1, len(allLegacyAlerts))
			}
		},
	}
	return &cmd
}

func newMigrateLegacyAlert() *cobra.Command {
	cmd := cobra.Command{
		Use:   "migrate [flags] <view> <alert-name>",
		Short: "Attempt to migrate a single legacy alert to aggregate alert",
		Long:  `Attempt to migrate a single legacy alert to aggregate alert. If the legacy alert migration is successful, the legacy alert will be deleted.`,
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			client := NewApiClient(cmd)
			viewName := args[0]
			legacyAlertName := args[1]

			var legacyAlert *api.Alert
			legacyAlert, err := client.Alerts().Get(viewName, legacyAlertName)
			if err != nil {
				msg := fmt.Sprintf("Could not get legacy alert from view `%s` with name `%s`", viewName, legacyAlertName)
				exitOnError(cmd, err, msg)
			}

			migrateLegacyAlert(*legacyAlert, cmd, client, viewName, 1, 1)
		},
	}
	return &cmd
}

func migrateLegacyAlert(
	legacyAlert api.Alert,
	cmd *cobra.Command,
	client *api.Client,
	viewName string,
	i int,
	size int,
) {
	var shortName, progress string
	progress = fmt.Sprintf("%d/%d", i, size)

	if len(legacyAlert.Name) >= 20 {
		shortName = fmt.Sprintf("%s...", legacyAlert.Name[:17])
	} else {
		shortName = legacyAlert.Name
	}
	cmd.Printf(
		"[%s] [%s] [INFO] migrating legacy alert %+v %+v\n",
		progress,
		shortName,
		legacyAlert.Name,
		legacyAlert.ID,
	)

	queryStartSeconds, err := getSecondsFromQueryStart(legacyAlert.QueryStart)
	if err != nil {
		cmd.Printf(
			"[%s] [%s] [FAILED] error getting seconds from query start `%s` err=%s\n",
			progress,
			shortName,
			legacyAlert.QueryStart,
			err,
		)
		return
	}
	searchIntervalSeconds := getClosest(queryStartSeconds, getValidSearchIntervalSeconds())

	var throttleTimeSeconds int
	if legacyAlert.ThrottleTimeMillis != 0 {
		if legacyAlert.ThrottleTimeMillis > 24*60*60*1000 {
			throttleTimeSeconds = 24 * 60 * 60
		} else if legacyAlert.ThrottleTimeMillis < 60*1000 {
			throttleTimeSeconds = 60
		} else {
			throttleTimeSeconds = legacyAlert.ThrottleTimeMillis / 1000
		}
	}

	defaultQueryTimestampType := "IngestTimestamp"

	tempName := fmt.Sprintf("%s-%d", legacyAlert.Name, time.Now().UnixMilli())

	intervalModified := queryStartSeconds != searchIntervalSeconds

	if intervalModified {
		if queryStartSeconds == legacyAlert.ThrottleTimeMillis/1000 {
			throttleTimeSeconds = searchIntervalSeconds
		} else {
			cmd.Printf(
				"[%s] [%s] [FAILED] search interval was changed from `%s` to `%d seconds` but was not equal to throttle time `%d millis` and could not be migrated. Correct query start and throttle time manually and try again.",
				progress,
				shortName,
				legacyAlert.QueryStart,
				searchIntervalSeconds,
				legacyAlert.ThrottleTimeMillis,
			)
			return
		}
	}

	aggregateAlert := &api.AggregateAlert{
		Name:                  tempName,
		Description:           legacyAlert.Description,
		QueryString:           legacyAlert.QueryString,
		SearchIntervalSeconds: searchIntervalSeconds,
		ActionNames:           legacyAlert.Actions,
		Labels:                legacyAlert.Labels,
		Enabled:               legacyAlert.Enabled,
		ThrottleField:         legacyAlert.ThrottleField,
		ThrottleTimeSeconds:   throttleTimeSeconds,
		QueryOwnershipType:    legacyAlert.QueryOwnershipType,
		QueryTimestampType:    defaultQueryTimestampType,
		RunAsUserID:           legacyAlert.RunAsUserID,
	}
	create, err := client.AggregateAlerts().Create(viewName, aggregateAlert)
	if err != nil {
		cmd.Printf(
			"[%s] [%s] [FAILED] creating new aggregate alert with input `%+v`, err=%s\n",
			progress,
			shortName,
			aggregateAlert,
			err,
		)
		return
	}
	cmd.Printf(
		"[%s] [%s] [INFO] created new aggregate alert with name '%s'\n",
		progress,
		shortName,
		aggregateAlert.Name,
	)

	err = client.Alerts().Delete(viewName, legacyAlert.Name)
	if err != nil {
		cmd.PrintErrf(
			"[%s] [%s] [FAILED] deleting legacy alert `%s`\n",
			progress,
			shortName,
			legacyAlert.Name,
		)
		return
	}
	cmd.Printf(
		"[%s] [%s] [INFO] deleted legacy alert `%s`\n",
		progress,
		shortName,
		legacyAlert.Name,
	)

	update, err := client.AggregateAlerts().Update(viewName, &api.AggregateAlert{
		ID:                    create.ID,
		Name:                  legacyAlert.Name,
		Description:           create.Description,
		QueryString:           create.QueryString,
		SearchIntervalSeconds: create.SearchIntervalSeconds,
		ActionNames:           create.ActionNames,
		Labels:                create.Labels,
		Enabled:               create.Enabled,
		ThrottleField:         create.ThrottleField,
		ThrottleTimeSeconds:   create.ThrottleTimeSeconds,
		QueryOwnershipType:    create.QueryOwnershipType,
		TriggerMode:           create.TriggerMode,
		QueryTimestampType:    create.QueryTimestampType,
		RunAsUserID:           create.RunAsUserID,
	})
	if err != nil {
		cmd.PrintErrf(
			"[%s] [%s] [FAILED] renaming new aggregate alert from `%s` to `%s`\n",
			progress,
			shortName,
			create.Name,
			legacyAlert.Name,
		)
		return
	}
	cmd.Printf(
		"[%s] [%s] [INFO] renamed aggregate alert from `%s` to `%s`\n",
		progress,
		shortName,
		create.Name,
		update.Name,
	)
	return
}

func getValidSearchIntervalSeconds() []int {
	var result []int

	for i := 1; i <= 80; i++ {
		result = append(result, i*60)
	}

	for j := 82; j <= 180; j += 2 {
		result = append(result, j*60)
	}

	for k := 4; k <= 24; k++ {
		result = append(result, k*60*60)
	}

	return result
}

func getClosest(n int, input []int) int {
	curr := 0
	for i := 0; i < len(input); i++ {
		if absDiff(n, input[i]) < absDiff(n, input[curr]) {
			curr = i
		}
	}
	return input[curr]
}

func absDiff(x, y int) int {
	if x < y {
		return y - x
	}
	return x - y
}

func getSecondsFromQueryStart(queryStart string) (int, error) {
	relativeTimeStringPattern := regexp.MustCompile(`^(\d+) ?(years?|y|yrs?|quarters?|q|qtrs?|months?|mon|weeks?|w|days?|d|hours?|hr?|hrs|minutes?|m|min|seconds?|s|secs?|milliseconds?|milli|ms)$`)
	match := relativeTimeStringPattern.FindStringSubmatch(queryStart)

	if len(match) != 3 {
		return 0, errors.New("cannot parse query start")
	}

	n, err := strconv.Atoi(match[1])
	if err != nil {
		return 0, err
	}

	unit := match[2]
	if containsString([]string{"milliseconds", "millisecond", "milli", "ms"}, unit) {
		if n < 1000 {
			return 0, errors.New("queryStart must be larger than 1000 milliseconds")
		}
		return n / 1000, nil // remainders are ignored
	}

	if containsString([]string{"seconds", "second", "secs", "sec", "s"}, unit) {
		return n, nil
	}

	if containsString([]string{"minutes", "minute", "min", "m"}, unit) {
		return n * 60, nil
	}

	if containsString([]string{"hours", "hour", "hr", "h"}, unit) {
		return n * 60 * 60, nil
	}

	if containsString([]string{"days", "day", "d"}, unit) {
		return n * 60 * 60 * 24, nil
	}

	if containsString([]string{"weeks", "week", "w"}, unit) {
		return n * 60 * 60 * 24 * 7, nil
	}

	if containsString([]string{"months", "month", "mon"}, unit) {
		return n * 60 * 60 * 24 * 30, nil
	}

	if containsString([]string{"quarters", "quarter", "qtrs", "qtr", "q"}, unit) {
		return n * 60 * 60 * 24 * 90, nil
	}

	if containsString([]string{"years", "year", "yr", "yrs", "y"}, unit) {
		return n * 60 * 60 * 24 * 365, nil
	}

	return 0, errors.New("unexpected matching")
}

func containsString(strings []string, s string) bool {
	for i, _ := range strings {
		if strings[i] == s {
			return true
		}
	}
	return false
}
