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
	"fmt"
	"istio.io/bots/policybot/pkg/config"
	"istio.io/bots/policybot/pkg/experiment"
	"istio.io/bots/policybot/pkg/pipeline"
	"istio.io/bots/policybot/pkg/storage"

	"github.com/spf13/cobra"
)

// examineFlakesCmd represents the examineFlakes command
var examineFlakesCmd = &cobra.Command{
	Use:   "examineFlakes",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		testName = args[0]
		fmt.Println("examineFlakes called")
		e := ExperimentClients
		startPipe := make(chan pipeline.OutResult, 1000)
		fmt.Printf("Begin querying spanner.  This might take a minute...")
		go func() {
			defer close(startPipe)
			err := e.Store.QueryTestFlakes(e.Ctx, "istio", "istio", 15000, testName, func(flake *storage.TestFlake) error {
				startPipe <- pipeline.NewOutResult(nil, flake)
				return nil
			})
			if err != nil {
				startPipe <- pipeline.NewOutResult(err, nil)
			}
		}()
		org, _ := getOrgAndRepoFromClients(e, "istio", "istio")
		bucket := e.Blobstore.Bucket(org.BucketName)
		x := pipeline.FromChan(startPipe).OnError(func(e error) {
			fmt.Printf("WARNING: encountered unexpected error reading from spanner: %v", e)
		}).Transform(func(iFlake interface{}) (i interface{}, err error) {
			flake := iFlake.(*storage.TestFlake)
			passLog, err := gcsLocToContents(flake.PassedRunPath+"build-log.txt", bucket, e.Ctx)
			if err != nil {
				return
			}
			failLog, err := gcsLocToContents(flake.FailedRunPath+"build-log.txt", bucket, e.Ctx)
			if err != nil {
				return
			}
			// Do some querying here about the flake
			// return something interesting, and maybe transform it again or store it in DB.
			// these next two lines are just so the example can build
			passLog = failLog
			failLog = passLog
			return
		}).Go()
		for y := range x {
			if y.Err() != nil {
				fmt.Printf("Got an error! %v", y.Err())
				continue
			}
			fmt.Printf("Got some result: %v", y.Output())
		}
		return nil
	},
}
var (
	testName string
)

func getOrgAndRepoFromClients(e *experiment.Clients, orgName, repoName string) (config.Org, config.Repo) {
	for _, o := range e.Orgs {
		if o.Name == orgName {
			for _, r := range o.Repos {
				if r.Name == repoName {
					return o, r
				}
			}
			return o, config.Repo{}
		}
	}
	return config.Org{}, config.Repo{}
}
func init() {
	experimentCmd.AddCommand(examineFlakesCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// examineFlakesCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// examineFlakesCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
