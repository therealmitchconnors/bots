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
	"github.com/pkg/errors"
	"github.com/sergi/go-diff/diffmatchpatch"
	"io/ioutil"
	"istio.io/bots/policybot/pkg/blobstorage"
	"os"
	"regexp"
	"sort"

	"github.com/spf13/cobra"
)

// compareLogsCmd represents the compareLogs command
var compareLogsCmd = &cobra.Command{
	Use:   "compareLogs",
	Short: "Compare logs and find the most meaningful diff",
	Long: `Compare a passing test log and a failing test log, ignoring 
trivial differences such as dates, durations, ephemeral ports, and random 
strings.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("compareLogs called")
		e := ExperimentClients
		bucket := e.Blobstore.Bucket(e.Orgs[0].BucketName)
		failedLog, err := gcsLocToContents(failedLoc, bucket, e.Ctx)
		if err != nil {
			return err
		}
		passedLog, err := gcsLocToContents(passedLoc, bucket, e.Ctx)
		if err != nil {
			return err
		}
		d := diffmatchpatch.New()
		d.MatchDistance = 1000
		x := d.DiffMain(clean(failedLog), clean(passedLog), true)
		type levdiff struct{
			levScore int
			diffs     []diffmatchpatch.Diff
		}
		var all []levdiff
		var curr levdiff
		for _, y := range x {
			curr.diffs = append(curr.diffs, y)
			if y.Type == diffmatchpatch.DiffEqual {
				if len(curr.diffs) > 1 {
					curr.levScore = d.DiffLevenshtein(curr.diffs)
					all = append(all, curr)
					curr = levdiff{}
				}
			}
		}

		sort.Slice(all, func(i, j int) bool {
			return all[i].levScore>all[j].levScore
		})
		i := 0
		for _, big := range all {
			if i > 5 {
				break
			}
			if len(big.diffs) < 3 {
				continue
			}
			fmt.Printf("Big diff: %v\n", big.levScore)
			fmt.Println(d.DiffPrettyText(big.diffs))
			i++
		}
		fmt.Printf("found a total of %d diffs\n", len(all))
		fmt.Printf("with a total lev score of %d out of %d.\n", d.DiffLevenshtein(x), max(len(passedLog), len(failedLog)))
		return ioutil.WriteFile("out.html", []byte(d.DiffPrettyHtml(x)), os.ModeAppend)
	},
}

type rx struct {
	regex string
	replace string
}

func max(x, y int) int {
	if x > y {
		return x
	}
	return y
}

func clean(log string) string {
	dirts := []rx{
		rx{"(\\d{4}-[01]\\d-[0-3]\\dT[0-2]\\d:[0-5]\\d:[0-5]\\d\\.\\d+([+-][0-2]\\d:[0-5]\\d|Z))|(\\d{4}-[01]\\d-[0-3]\\dT[0-2]\\d:[0-5]\\d:[0-5]\\d([+-][0-2]\\d:[0-5]\\d|Z))|(\\d{4}-[01]\\d-[0-3]\\dT[0-2]\\d:[0-5]\\d([+-][0-2]\\d:[0-5]\\d|Z))", "SEDDATE",},
		rx{"\\d{2}:\\d{2}:\\d{2}(\\.\\d*)?", "SEDTIME"},
		rx{"-[\\da-f]{8,10}-[\\da-z]{5}","-SEDPOD"},
		rx{"-[\\da-f]{6,}", "-SEDHEX"},
		rx{"\\(\\d+\\.\\d+s\\)", "SEDDUR"},
		rx{"/tmp\\.[a-zA-Z]{10}(\\W)", "/SEDTMP$1"},
		rx{"201\\d-\\d{2}-\\d{2}", "SEDDATE2"},
		rx{"\\[\\d{6}\\]", "[SEDTHREAD]"},
		rx{"(\\dm)?\\d+(\\.\\d+)?[mµn]?s", "SEDDUR"},
		rx{"\\d{6,}", "SEDNUMBER"},
		rx{":\\d{5,6}(\\D)", "SEDPORT$1"},
		rx{"go: \\w+ing.*", "SEDGOMOD"},
		rx{"-[a-zA-Z0-9]{5}(\\s)", "SEDPOD$1"},
	}
	for _, i := range dirts {

		rx, err := regexp.Compile(i.regex)
		if err != nil {
			fmt.Printf("super bad error: %v", err)
			continue
		}
		log = rx.ReplaceAllString(log, i.replace)
	}
	return log
}

func gcsLocToContents(location string, b blobstorage.Bucket, ctx context.Context) (string, error) {
	read, err := b.Reader(ctx, location)
	if err != nil {
		return "", errors.Wrap(err, "Unable to create reader for " + location)
	}
	bytes, err := ioutil.ReadAll(read)
	if err != nil {
		return "", errors.Wrap(err, "unable to read " + location)
	}
	return string(bytes), nil
}

var (
	passedLoc, failedLoc string
)
func init() {
	experimentCmd.AddCommand(compareLogsCmd)
	compareLogsCmd.PersistentFlags().StringVar(&passedLoc, "passed-location",
		"pr-logs/pull/istio_istio/18332/pilot-multicluster-e2e_istio/2260/build-log.txt",
		"the GCS location of the passing log")
	compareLogsCmd.PersistentFlags().StringVar(&failedLoc, "failed-location",
		"pr-logs/pull/istio_istio/18332/pilot-multicluster-e2e_istio/2387/build-log.txt",
		"the GCS location of the failing log")

}
