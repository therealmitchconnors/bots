// Copyright © 2019 NAME HERE <EMAIL ADDRESS>
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

package cmd

import (
	"context"
	"encoding/base64"
	"fmt"

	"istio.io/bots/policybot/pkg/config"
	"istio.io/bots/policybot/pkg/experiment"
	"istio.io/bots/policybot/pkg/gh"
	"istio.io/bots/policybot/pkg/storage/spanner"
	"istio.io/bots/policybot/pkg/zh"
	"istio.io/pkg/env"
	"istio.io/pkg/log"

	"github.com/spf13/cobra"
)

// experimentCmd represents the experiment command
var experimentCmd = &cobra.Command{
	Use:   "experiment",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {

		// load the config file
		if err := ca.Fetch(); err != nil {
			return fmt.Errorf("unable to load configuration file: %v", err)
		}

		creds, err := base64.StdEncoding.DecodeString(ca.StartupOptions.GCPCredentials)
		if err != nil {
			return fmt.Errorf("unable to decode GCP credentials: %v", err)
		}
		ctx := context.Background()

		gc := gh.NewThrottledClient(ctx, ca.StartupOptions.GitHubToken)
		zc := zh.NewThrottledClient(ca.StartupOptions.ZenHubToken)

		store, err := spanner.NewStore(ctx, ca.SpannerDatabase, creds)
		if err != nil {
			return fmt.Errorf("unable to create storage layer: %v", err)
		}
		defer store.Close()

		ExperimentClients, err = experiment.NewClient(ctx, gc, creds, ca.GCPProject, zc, store, ca.Orgs)
		if err != nil {
			return fmt.Errorf("unable to create syncer: %v", err)
		}

		return nil
	},
}

var ca *config.Args
var ExperimentClients *experiment.Clients

func init() {
	ca = config.DefaultArgs()

	ca.StartupOptions.GitHubToken = env.RegisterStringVar("GITHUB_TOKEN", ca.StartupOptions.GitHubToken, githubToken).Get()
	ca.StartupOptions.ZenHubToken = env.RegisterStringVar("ZENHUB_TOKEN", ca.StartupOptions.ZenHubToken, zenhubToken).Get()
	ca.StartupOptions.GCPCredentials = env.RegisterStringVar("GCP_CREDS", ca.StartupOptions.GCPCredentials, gcpCreds).Get()
	ca.StartupOptions.ConfigRepo = env.RegisterStringVar("CONFIG_REPO", ca.StartupOptions.ConfigRepo, configRepo).Get()
	ca.StartupOptions.ConfigFile = env.RegisterStringVar("CONFIG_FILE", ca.StartupOptions.ConfigFile, configFile).Get()

	loggingOptions := log.DefaultOptions()
	var filters string

	experimentCmd.PersistentFlags().StringVarP(&ca.StartupOptions.ConfigRepo, "config_repo", "", ca.StartupOptions.ConfigRepo, configRepo)
	experimentCmd.PersistentFlags().StringVarP(&ca.StartupOptions.ConfigFile, "config_file", "", ca.StartupOptions.ConfigFile, configFile)
	experimentCmd.PersistentFlags().StringVarP(&ca.StartupOptions.GitHubToken, "github_token", "", ca.StartupOptions.GitHubToken, githubToken)
	experimentCmd.PersistentFlags().StringVarP(&ca.StartupOptions.ZenHubToken, "zenhub_token", "", ca.StartupOptions.ZenHubToken, zenhubToken)
	experimentCmd.PersistentFlags().StringVarP(&ca.StartupOptions.GCPCredentials, "gcp_creds", "", ca.StartupOptions.GCPCredentials, gcpCreds)

	experimentCmd.PersistentFlags().StringVarP(&filters,
		"filter", "", "", "Comma-separated filters to limit what is synced, one or more of "+
			"[issues, prs, labels, maintainers, members, zenhub, repocomments, events, testresults]")

	loggingOptions.AttachCobraFlags(experimentCmd)
}
