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
	"github.com/humio/cli/api"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

func newAlertsExportCmd() *cobra.Command {
	var outputName string

	cmd := cobra.Command{
		Use:   "export [flags] <view> <alert>",
		Short: "Export an alert <alert> in <view> to a file.",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			view := args[0]
			alertName := args[1]
			client := NewApiClient(cmd)

			if outputName == "" {
				outputName = alertName
			}

			alert, err := client.Alerts().Get(view, alertName)
			exitOnError(cmd, err, "Error fetching alert")

			yamlData, err := yaml.Marshal(&alert)
			exitOnError(cmd, err, "Failed to serialize the alert")

			outFilePath := outputName + ".yaml"
			err = os.WriteFile(outFilePath, yamlData, 0600)
			exitOnError(cmd, err, "Error saving the alert file")
		},
	}

	cmd.Flags().StringVarP(&outputName, "output", "o", "", "The file path where the alert should be written. Defaults to ./<alert-name>.yaml")

	return &cmd
}

func newExportAllLegacyAlerts() *cobra.Command {
	cmd := cobra.Command{
		Use:   "export-all <view>",
		Short: "Export all legacy alerts",
		Long:  `Export all legacy alerts to yaml files with naming <legacy-alert-name>.yaml.`,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			view := args[0]
			client := NewApiClient(cmd)

			var legacyAlerts []api.Alert
			legacyAlerts, err := client.Alerts().List(view)
			exitOnError(cmd, err, "Error fetching legacy alerts")

			for _, alert := range legacyAlerts {
				yamlData, err := yaml.Marshal(&alert)
				exitOnError(cmd, err, "Failed to serialize the legacy alert")
				outFilePath := alert.Name + ".yaml"
				err = os.WriteFile(outFilePath, yamlData, 0600)
				exitOnError(cmd, err, "Error saving the legacy alert file")
			}
		},
	}
	return &cmd
}
