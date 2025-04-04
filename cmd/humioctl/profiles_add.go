package main

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/humio/cli/internal/api"
	"github.com/humio/cli/internal/viperkey"
	"github.com/humio/cli/prompt"
	"github.com/skratchdot/open-golang/open"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// usersCmd represents the users command
func newProfilesAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <profile>",
		Short: "Add a configuration profile",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			profileName := args[0]

			profile, err := collectProfileInfo(cmd)
			exitOnError(cmd, err, "Failed to collect profile info")

			addAccount(profileName, profile)

			err = saveConfig()
			exitOnError(cmd, err, "Error saving config")

			fmt.Fprintf(cmd.OutOrStdout(), "Successfully added profile with name %q\n", profileName)
		},
	}

	return cmd
}

func saveConfig() error {
	configFile := viper.ConfigFileUsed()

	if writeErr := viper.WriteConfig(); writeErr != nil {
		if os.IsNotExist(writeErr) {
			dirName := filepath.Dir(configFile)
			if dirErr := os.MkdirAll(dirName, 0700); dirErr != nil {
				return fmt.Errorf("error creating config directory %s: %s", dirName, dirErr)
			}
			if configFileErr := viper.WriteConfigAs(configFile); configFileErr != nil {
				return fmt.Errorf("error writing config file: %s", configFileErr)
			}
		}
	}

	return nil
}

func addAccount(newName string, profile *login) {
	profiles := viper.GetStringMap(viperkey.Profiles)

	if profiles == nil {
		profiles = map[string]interface{}{}
	}

	profiles[newName] = map[string]interface{}{
		viperkey.Address:       profile.address,
		viperkey.Token:         profile.token,
		viperkey.Username:      profile.username,
		viperkey.CACertificate: profile.caCertificate,
		viperkey.Insecure:      profile.insecure,
	}

	viper.Set(viperkey.Profiles, profiles)
}

func mapToLogin(data interface{}) *login {
	return &login{
		address:       getMapKeyString(data, viperkey.Address),
		username:      getMapKeyString(data, viperkey.Username),
		token:         getMapKeyString(data, viperkey.Token),
		caCertificate: getMapKeyString(data, viperkey.CACertificate),
		insecure:      getMapKeyBool(data, viperkey.Insecure),
	}
}

func getMapKeyString(data interface{}, key string) string {
	if m, ok := data.(map[string]interface{}); ok {
		if v, ok := m[key].(string); ok {
			return v
		}
	}

	return ""
}

func getMapKeyBool(data interface{}, key string) bool {
	if m, ok := data.(map[string]interface{}); ok {
		if v, ok := m[key].(bool); ok {
			return v
		}
	}

	return false
}

func collectProfileInfo(cmd *cobra.Command) (*login, error) {
	var addr, token, username, caCertificate string
	var insecure bool

	out := prompt.NewPrompt(cmd.OutOrStdout())
	out.Info("Which Humio instance should we talk to?")
	out.Description("If you are not using Humio Cloud enter the address of your Humio installation,")
	out.Description("e.g. http://localhost:8080/ or https://humio.example.com/")

	var parsedURL *url.URL
	for {
		var err error
		out.BlankLine()
		addr, err = out.Ask("Address (default: https://cloud.humio.com/ [Hit Enter])")
		exitOnError(cmd, err, "Error reading humio server address")

		if addr == "" {
			addr = "https://cloud.humio.com/"
		}

		// Make sure it is a valid URL and that
		// we always end in a slash.
		parsedURL, err = url.Parse(addr)

		if err != nil {
			out.Error("The value must be a valid URL.")
			continue
		}

		clientConfig := api.DefaultConfig()
		clientConfig.Address = parsedURL
		client := api.NewClient(clientConfig)

		cmd.Print("==> Testing Connection...")
		_, statusErr := client.Status()
		if statusErr != nil {
			if errors.Is(statusErr, x509.UnknownAuthorityError{}) {
				cmd.Println(prompt.Colorize("[[red]Failed[reset]] Certificate not signed by a trusted Certificate Authority."))
				out.Info("What is the absolute path to the CA certificate that should be used for TLS certificate validation?")
				out.Description("If you require a custom CA certificate for validating the TLS certificate of the Humio cluster,")
				out.Description("specify the path to the file containing the CA certificate in PEM format.")
				out.Description("If left empty it is not possible to validate TLS certificate chain.")

				caCertificateFilePath, err := out.Ask("Absolute path on local disk to CA certificate in PEM format")
				exitOnError(cmd, err, "Error reading Humio CA certificate file path")
				if caCertificateFilePath != "" {
					// Read the file
					// #nosec G304
					caCertContent, err := os.ReadFile(caCertificateFilePath)
					exitOnError(cmd, err, "Error reading Humio CA certificate file path")
					block, _ := pem.Decode(caCertContent)
					if block == nil {
						exitOnError(cmd, fmt.Errorf("expected PEM block"), "Expected PEM encoded CA certificate file")
					}
					caCertificate = string(caCertContent)
					clientConfig.CACertificatePEM = caCertificate
					client = api.NewClient(clientConfig)
				}
			}
		}

		out.Print("==> Testing Connection...")
		_, statusErr = client.Status()
		if statusErr != nil {
			if errors.Is(statusErr, x509.HostnameError{}) {
				out.Printf("%s: %s\n", prompt.Colorize("[[red]Failed[reset]] Certificate not valid for"), clientConfig.Address)
				out.Info("Disable hostname verification for TLS connections?")
				out.Description("By default all connections will verify the hostname, but this option allows you to disable this if required.")
				insecureString, err := out.Ask("Do you want to disable hostname verification? Type 'yes' to disable hostname verification")
				exitOnError(cmd, err, "Error reading humio ca certificate file path")
				if insecureString == "yes" {
					out.Print("Disabling hostname verification.")
					insecure = true
					clientConfig.Insecure = true
					client = api.NewClient(clientConfig)
				}
			}
		}

		out.Print("==> Testing Connection...")
		status, statusErr := client.Status()

		if statusErr != nil {
			cmd.Println(prompt.Colorize("[[red]Failed[reset]]"))
			out.Error(fmt.Sprintf("Could not connect to the Humio server: %s\nIs the address connect and reachable?", statusErr))
			continue
		}

		if status.IsDown() {
			cmd.Println(prompt.Colorize("[[red]Failed[reset]]"))
			cmd.Printf("The server reported that it is malfunctioning, status: %s\n", status.Status)
			os.Exit(1)
		} else {
			cmd.Println(prompt.Colorize("[[green]Ok[reset]]"))
		}
		break
	}

	out.Info("Paste in your Personal API Token")
	out.Description("To use Humio's CLI you will need to get a copy of your API Token.")
	out.Description("The API token can be found on the 'Account Settings' page of the UI.")
	out.Description("If you are running Humio without authorization just leave the API Token field empty.")

	if out.Confirm("Would you like us to open a browser on the account page?") {
		_ = open.Start(fmt.Sprintf("%ssettings", addr))

		out.Description("If the browser did not open, you can manually visit:")
		out.Description(fmt.Sprintf("%ssettings", addr))
	}

	for {
		var err error
		token, err = out.AskSecret("API Token")
		exitOnError(cmd, err, "Error reading token")

		// Create a new API client with the token
		config := api.DefaultConfig()
		config.Address = parsedURL
		config.Token = token
		config.CACertificatePEM = caCertificate
		config.Insecure = insecure

		client := api.NewClient(config)

		var apiErr error
		username, apiErr = client.Viewer().Username()

		if apiErr != nil {
			out.Error(fmt.Sprintf("Authentication failed, invalid token: %s", apiErr))

			if out.Confirm("Do you want to use another token?") {
				continue
			}
		}

		if username != "" {
			cmd.Println(prompt.Colorize(fmt.Sprintf("==> Logged in as: [purple]%s[reset]", username)))
		}
		break
	}

	return &login{address: addr, token: token, username: username, caCertificate: caCertificate, insecure: insecure}, nil
}

func isCurrentAccount(addr string, token string) bool {
	return viper.GetString(viperkey.Address) == addr && viper.GetString(viperkey.Token) == token
}
