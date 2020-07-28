// Copyright © 2020 NAME HERE <EMAIL ADDRESS>
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
	"cloud.google.com/go/spanner"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/hashicorp/go-multierror"
	"github.com/spf13/cobra"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"istio.io/bots/policybot/pkg/pipeline"
	"istio.io/pkg/env"
	"regexp"
	"time"
)

const limitFmt = " LIMIT %d OFFSET %d"
var sql string
var batch, start int

// migrateCmd represents the migrate command
var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// First determine size of source table
		srcRx := regexp.MustCompile("FROM (\\w*)")
		srcTable := srcRx.FindString(sql)
		if len(srcTable) < 1 {
			return errors.New("sql statement appears to contain no FROM clause")

		}

		creds64 := env.RegisterStringVar("GCP_CREDS", "", "gcpCreds").Get()
		creds, err := base64.StdEncoding.DecodeString(creds64)
		if err != nil {
			return fmt.Errorf("unable to decode GCP credentials: %v", err)
		}
		client, err := spanner.NewClient(context.TODO(),
			"projects/istio-testing/instances/istio-policy-bot/databases/main", option.WithCredentialsJSON(creds))
		if err != nil {
			return fmt.Errorf("unable to decode connect to spanner: %v", err)
		}
		rows := client.Single().Query(context.TODO(), spanner.Statement{
			SQL:    "SELECT COUNT(*) " + srcTable,
		})
		countresult, err := rows.Next()
		if err != nil {
			return fmt.Errorf("unable to query table size: %v", err)
		}
		var count int64
		err = countresult.Column(0, &count)
		if err != nil {
			return fmt.Errorf("unable to read spanner row: %v", err)
		}

		fmt.Printf("Target table has %d rows.  Calculating batches.\n", count)

		jobs := buildPartitionedJobs(start, batch, count)

		jobChan := pipeline.BuildProducer(context.TODO(), jobs)
		errorChan := pipeline.FromChan(jobChan).WithParallelism(30).To(func(i interface{}) error {
			offsets := i.([]interface{})
			var merr error
			for _, x := range offsets {
				offset := x.(int64)
				fmt.Printf("attempting to run batch starting at %d\n", offset)
				merr = multierror.Append(merr, doInsert(client, offset, batch, true))
			}
			return merr
		}).Go()
		var result *multierror.Error
		for err := range errorChan {
			result = multierror.Append(err.Err())
		}
		if result != nil {
			return result
		}
		return nil
	},
}

// returns [][]int64 cast to []interface{}
func buildPartitionedJobs(offset, batch int, count int64) []interface{} {
	// metaBatch refers to the partition of jobs that will be handled by a single worker
	metaBatchSize := int((count-int64(offset))/int64(30*batch))
	remainder := ((count-int64(offset)) % int64(30*batch))/int64(batch)
	if metaBatchSize < 1 {
		metaBatchSize = 1
	}
	flatjobs := []interface{}{}
	for i := int64(offset); i < count; i += int64(batch) {
		flatjobs = append(flatjobs, i)
	}

	jobs := make([]interface{}, 30)

	start := 0
	end := start + metaBatchSize
	i:=0
	for end < len(flatjobs) {
		if int64(i) < remainder {
			end++
		}
		jobs[i] = flatjobs[start:end]
		start = end
		end = start + metaBatchSize
		i++
	}
	return jobs
}

func doInsert(client *spanner.Client, offset int64, limit int, retry bool) error {
	jobSQL := fmt.Sprintf("%s LIMIT %d OFFSET %d", sql, limit, offset)
	_, err := client.ReadWriteTransaction(context.TODO(), func(ctx2 context.Context, txn *spanner.ReadWriteTransaction) error {
		_, iErr := txn.Update(ctx2, spanner.Statement{SQL: jobSQL})
		return iErr
	})
	if err != nil {
		if retry {
			code := spanner.ErrCode(err)
			if code == codes.InvalidArgument {
				fmt.Printf("failed writing batch %d, subdividing\n", offset)
				// retry smaller
				newlimit := limit/4
				var merr error
				for i:=offset; i<offset+int64(limit); i += int64(newlimit) {
					merr = multierror.Append(doInsert(client, i, newlimit, false), merr)
				}
				return merr
			} else if code == codes.Aborted {
				fmt.Printf("failed writing batch %d, retrying\n", offset)
				time.Sleep(10*time.Millisecond)
				return doInsert(client, offset, limit, false)
			}
		} else {
			fmt.Printf("permanent failure writing batch %d limit %d\n", offset, limit)
			return err
		}
	}
	return nil
}

func init() {

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// migrateCmd.PersistentFlags().String("foo", "", "A help for foo")
	migrateCmd.PersistentFlags().IntVar(&batch, "batch", 1000, "the size of initial batches for attempting")
	migrateCmd.PersistentFlags().IntVar(&start, "start", 0, "starting point (offset) for the beginning of processing")
    migrateCmd.PersistentFlags().StringVar(&sql, "sql", "", "the insert statement to use")
	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// migrateCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
