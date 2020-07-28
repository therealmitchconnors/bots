package cmd

import (
	"testing"
)

func Test_buildPartitionedJobs(t *testing.T) {
	type args struct {
		start int
		batch int
		count int64
	}
	tests := []struct {
		name string
		args args
		want []interface{}
	}{
		{
			name: "first",
			args: args{
				start: 100,
				batch: 800,
				count: 90000,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildPartitionedJobs(tt.args.start, tt.args.batch, tt.args.count)
			if len(got) != 30 {
				t.Errorf("buildPartitionedJobs should always have len 30, got %v", got)
			}
			//if !reflect.DeepEqual(got, tt.want) {
			//	t.Errorf("buildPartitionedJobs() = %v, want %v", got, tt.want)
			//}
		})
	}
}