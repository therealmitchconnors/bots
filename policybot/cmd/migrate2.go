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
	"cloud.google.com/go/civil"
	"cloud.google.com/go/spanner"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/hashicorp/go-multierror"
	"github.com/spf13/cobra"
	"google.golang.org/api/option"
	spanner2 "google.golang.org/genproto/googleapis/spanner/v1"
	"google.golang.org/grpc/codes"
	"istio.io/bots/policybot/pkg/pipeline"
	"istio.io/pkg/env"
	"time"
)

var srcTable string
// migrate2Cmd represents the migrate2 command
var migrate2Cmd = &cobra.Command{
	Use:   "migrate2",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("migrate2 called")
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
			SQL:    "SELECT * FROM " + srcTable,
		})
		fmt.Print("Finished sending query, beginning write operations")
		errorChan := pipeline.FromIter(pipeline.IterProducer{
			Setup:    func() error { return nil},
			Iterator: func() (i interface{}, e error) {
				row, err := rows.Next()
				if err != nil {
					return nil, err
				}
				out := make(map[string]interface{})
				for _, colname := range row.ColumnNames() {
					var val2 spanner.GenericColumnValue
					err := row.ColumnByName(colname, &val2)
					if err != nil {
						fmt.Printf("Unable to retrieve column value: %v", err)
						return nil, err
					}
					val, err := decode(val2)
					if err != nil {
						fmt.Printf("Unable to decode column value: %v", err)
						return nil, err
					}
					out[colname] = val
				}
				return spanner.InsertOrUpdateMap(srcTable + "_TMP", out), nil
			},
		}).Batch(batch).WithParallelism(2).To(func(i interface{}) error {
			gendata := i.([]interface{})
			var data []*spanner.Mutation
			for _, g := range gendata {
				data = append(data, g.(*spanner.Mutation))
			}

			fmt.Printf("beginning transaction of len %d\n", len(data))
			err := doInsertM(client, data, true)
			if err == nil {
				fmt.Printf("completed transaction of len %d\n", len(data))
			} else {
				fmt.Printf("failed transaction of len %d: %v\n", len(data), err)
			}
			return err
		}).Go()
		var result *multierror.Error
		for err := range errorChan {
			result = multierror.Append(err.Err())
		}
		fmt.Printf("%v", result)
		return result
	},
}

func decode(val spanner.GenericColumnValue) (interface{}, error) {
	switch val.Type.Code{
	case spanner2.TypeCode_BOOL:
		var out bool
		err := val.Decode(&out)
		return out, err
	case spanner2.TypeCode_INT64:
		var out int64
		err := val.Decode(&out)
		return out, err
	case spanner2.TypeCode_FLOAT64:
		var out float64
		err := val.Decode(&out)
		return out, err
	case spanner2.TypeCode_TIMESTAMP:
		var out time.Time
		err := val.Decode(&out)
		return out, err
	case spanner2.TypeCode_DATE:
		var out civil.Date
		err := val.Decode(&out)
		return out, err
	case spanner2.TypeCode_STRING:
		var out string
		err := val.Decode(&out)
		return out, err
	case spanner2.TypeCode_BYTES:
		var out []byte
		err := val.Decode(&out)
		return out, err
	case spanner2.TypeCode_ARRAY:
		switch val.Type.ArrayElementType.Code {
		case spanner2.TypeCode_STRING:
			var out []string
			err := val.Decode(&out)
			return out, err
		default:
			return nil, errors.New("decoding of non-string array fields not permittedΩ")
		}
	case spanner2.TypeCode_STRUCT:
		return nil, errors.New("decoding of struct fields not permittedΩ")
	default:
		return nil, fmt.Errorf("unknown data type: %s", val.Type)
	}
}

func doInsertM(client *spanner.Client, data []*spanner.Mutation, retry bool) error {
	_, err := client.ReadWriteTransaction(context.TODO(), func(i context.Context, transaction *spanner.ReadWriteTransaction) error {
		return transaction.BufferWrite(data)
	})
	if err != nil {
		if retry {
			code := spanner.ErrCode(err)
			if code == codes.InvalidArgument {
				fmt.Print("failed writing batch, subdividing\n")
				// retry smaller
				newsize := len(data)/4
				var merr error
				for i:=0; i<len(data); i += newsize {
					end := i + newsize
					if end >= len(data) {
						end = len(data) -1
					}
					err = doInsertM(client, data[i:end], false)
					if err != nil {
						merr = multierror.Append(err)
					}
				}
				return merr
			} else if code == codes.Aborted {
				fmt.Print("failed writing batch, retrying\n")
				time.Sleep(10*time.Millisecond)
				return doInsertM(client, data, false)
			}
		} else {
			fmt.Print("permanent failure writing batch\n")
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
	migrate2Cmd.PersistentFlags().IntVar(&batch, "batch", 1000, "the size of initial batches for attempting")
	migrate2Cmd.PersistentFlags().StringVar(&srcTable, "table", "", "the table to select from")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// migrateCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
