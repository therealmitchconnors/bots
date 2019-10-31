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
	"fmt"
	"io/ioutil"

	"istio.io/bots/policybot/pkg/blobstorage"

	"github.com/sergi/go-diff/diffmatchpatch"
	"github.com/spf13/cobra"
)

// getSignaturesCmd represents the getSignatures command
var getSignaturesCmd = &cobra.Command{
	Use:   "getSignatures",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		e := ExperimentClients
		fmt.Println("getSignatures called")
		failedLoc := "pr-logs/pull/istio_istio/18332/pilot-multicluster-e2e_istio/2387/"
		passedLoc := "pr-logs/pull/istio_istio/18332/pilot-multicluster-e2e_istio/2260/"
		bucket := e.Blobstore.Bucket(e.Orgs[0].BucketName)
		failedLog, err := gcsLocToContents(failedLoc, bucket, e.Ctx)
		passedLog, err := gcsLocToContents(passedLoc, bucket, e.Ctx)
		d := diffmatchpatch.New()
		d.DifWITHfMain(failedLog, passedLog, false)
	},
}

func gcsLocToContents(location string, b blobstorage.Bucket, ctx context.Context) (string, error) {
	read, err := b.Reader(ctx, location)
	if err != nil {
		return "", err
	}
	bytes, err := ioutil.ReadAll(read)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func init() {
	experimentCmd.AddCommand(getSignaturesCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// getSignaturesCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// getSignaturesCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
