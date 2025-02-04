// Copyright © 2024 CrowdStrike
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
	"strings"

	"github.com/humio/cli/internal/format"
	"github.com/spf13/cobra"
)

func newAggregateAlertsListCmd() *cobra.Command {
	cmd := cobra.Command{
		Use:   "list <view>",
		Short: "List all aggregate alerts in a view.",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			view := args[0]
			client := NewApiClient(cmd)

			aggregateAlerts, err := client.AggregateAlerts().List(view)
			exitOnError(cmd, err, "Error fetching aggregate alerts")

			var rows = make([][]format.Value, len(aggregateAlerts))
			for i := range aggregateAlerts {
				aggregateAlert := aggregateAlerts[i]
				rows[i] = []format.Value{
					format.String(aggregateAlert.ID),
					format.String(aggregateAlert.Name),
					format.StringPtr(aggregateAlert.Description),
					format.String(strings.Join(aggregateAlert.ActionNames, ", ")),
					format.String(strings.Join(aggregateAlert.Labels, ", ")),
					format.Bool(aggregateAlert.Enabled),
					format.StringPtr(aggregateAlert.ThrottleField),
					format.Int(aggregateAlert.ThrottleTimeSeconds),
					format.Int(aggregateAlert.SearchIntervalSeconds),
					format.String(aggregateAlert.QueryTimestampType),
					format.String(aggregateAlert.TriggerMode),
					format.String(aggregateAlert.OwnershipRunAsID),
					format.String(aggregateAlert.QueryOwnershipType),
				}
			}

			printOverviewTable(cmd, []string{"ID", "Name", "Description", "Action Names", "Labels", "Enabled", "Throttle Field", "Throttle Time Seconds", "Search Interval Seconds", "Query Timestamp Type", "Trigger Mode", "Run As UserID", "Query Ownership Type"}, rows)
		},
	}

	return &cmd
}
