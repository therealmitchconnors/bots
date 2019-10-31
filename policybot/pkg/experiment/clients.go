package experiment

import (
	"cloud.google.com/go/bigquery"
	"context"
	"fmt"
	"google.golang.org/api/option"
	"istio.io/bots/policybot/pkg/blobstorage"
	"istio.io/bots/policybot/pkg/blobstorage/gcs"
	"istio.io/bots/policybot/pkg/config"
	"istio.io/bots/policybot/pkg/gh"
	"istio.io/bots/policybot/pkg/storage"
	"istio.io/bots/policybot/pkg/zh"
)

// Syncer is responsible for synchronizing state from GitHub and ZenHub into our local Store
type Clients struct {
	Bq        *bigquery.Client
	Gc        *gh.ThrottledClient
	Zc        *zh.ThrottledClient
	Store     storage.Store
	Orgs      []config.Org
	Blobstore blobstorage.Store
	Ctx       context.Context
}

func NewClient(ctx context.Context, gc *gh.ThrottledClient, gcpCreds []byte, gcpProject string,
	zc *zh.ThrottledClient, store storage.Store, orgs []config.Org) (*Clients, error) {
	bq, err := bigquery.NewClient(ctx, gcpProject, option.WithCredentialsJSON(gcpCreds))
	if err != nil {
		return nil, fmt.Errorf("unable to create BigQuery client: %v", err)
	}
	bs, err := gcs.NewStore(ctx, gcpCreds)
	if err != nil {
		return nil, fmt.Errorf("unable to create gcs client: %v", err)
	}

	return &Clients{
		Gc:        gc,
		Bq:        bq,
		Zc:        zc,
		Store:     store,
		Blobstore: bs,
		Orgs:      orgs,
		Ctx:       ctx,
	}, nil
}