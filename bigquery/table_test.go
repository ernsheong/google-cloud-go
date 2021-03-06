// Copyright 2017 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package bigquery

import (
	"testing"
	"time"

	"cloud.google.com/go/internal/testutil"
	bq "google.golang.org/api/bigquery/v2"
)

func TestBQTableToMetadata(t *testing.T) {
	aTime := time.Date(2017, 1, 26, 0, 0, 0, 0, time.Local)
	aTimeMillis := aTime.UnixNano() / 1e6
	for _, test := range []struct {
		in   *bq.Table
		want *TableMetadata
	}{
		{&bq.Table{}, &TableMetadata{}}, // test minimal case
		{
			&bq.Table{
				CreationTime:     aTimeMillis,
				Description:      "desc",
				Etag:             "etag",
				ExpirationTime:   aTimeMillis,
				FriendlyName:     "fname",
				Id:               "id",
				LastModifiedTime: uint64(aTimeMillis),
				Location:         "loc",
				NumBytes:         123,
				NumLongTermBytes: 23,
				NumRows:          7,
				StreamingBuffer: &bq.Streamingbuffer{
					EstimatedBytes:  11,
					EstimatedRows:   3,
					OldestEntryTime: uint64(aTimeMillis),
				},
				TimePartitioning: &bq.TimePartitioning{
					ExpirationMs: 7890,
					Type:         "DAY",
				},
				Type: "EXTERNAL",
				View: &bq.ViewDefinition{Query: "view-query"},
			},
			&TableMetadata{
				Description:      "desc",
				Name:             "fname",
				ViewQuery:        "view-query",
				FullID:           "id",
				Type:             ExternalTable,
				ExpirationTime:   aTime.Truncate(time.Millisecond),
				CreationTime:     aTime.Truncate(time.Millisecond),
				LastModifiedTime: aTime.Truncate(time.Millisecond),
				NumBytes:         123,
				NumRows:          7,
				TimePartitioning: &TimePartitioning{Expiration: 7890 * time.Millisecond},
				StreamingBuffer: &StreamingBuffer{
					EstimatedBytes:  11,
					EstimatedRows:   3,
					OldestEntryTime: aTime,
				},
				ETag: "etag",
			},
		},
	} {
		got := bqTableToMetadata(test.in)
		if diff := testutil.Diff(got, test.want); diff != "" {
			t.Errorf("%+v:\n, -got, +want:\n%s", test.in, diff)
		}
	}
}

func TestBQTableFromMetadata(t *testing.T) {
	aTime := time.Date(2017, 1, 26, 0, 0, 0, 0, time.Local)
	aTimeMillis := aTime.UnixNano() / 1e6
	sc := Schema{fieldSchema("desc", "name", "STRING", false, true)}

	for _, test := range []struct {
		in   *TableMetadata
		want *bq.Table
	}{
		{nil, &bq.Table{}},
		{&TableMetadata{}, &bq.Table{}},
		{
			&TableMetadata{
				Name:           "n",
				Description:    "d",
				Schema:         sc,
				ExpirationTime: aTime,
			},
			&bq.Table{
				FriendlyName: "n",
				Description:  "d",
				Schema: &bq.TableSchema{
					Fields: []*bq.TableFieldSchema{
						bqTableFieldSchema("desc", "name", "STRING", "REQUIRED"),
					},
				},
				ExpirationTime: aTimeMillis,
			},
		},
		{
			&TableMetadata{ViewQuery: "q"},
			&bq.Table{
				View: &bq.ViewDefinition{
					Query:           "q",
					UseLegacySql:    false,
					ForceSendFields: []string{"UseLegacySql"},
				},
			},
		},
		{
			&TableMetadata{
				ViewQuery:        "q",
				UseLegacySQL:     true,
				TimePartitioning: &TimePartitioning{},
			},
			&bq.Table{
				View: &bq.ViewDefinition{
					Query:        "q",
					UseLegacySql: true,
				},
				TimePartitioning: &bq.TimePartitioning{
					Type:         "DAY",
					ExpirationMs: 0,
				},
			},
		},
		{
			&TableMetadata{
				ViewQuery:        "q",
				UseStandardSQL:   true,
				TimePartitioning: &TimePartitioning{time.Second},
			},
			&bq.Table{
				View: &bq.ViewDefinition{
					Query:           "q",
					UseLegacySql:    false,
					ForceSendFields: []string{"UseLegacySql"},
				},
				TimePartitioning: &bq.TimePartitioning{
					Type:         "DAY",
					ExpirationMs: 1000,
				},
			},
		},
	} {
		got, err := bqTableFromMetadata(test.in)
		if err != nil {
			t.Fatalf("%+v: %v", test.in, err)
		}
		if diff := testutil.Diff(got, test.want); diff != "" {
			t.Errorf("%+v:\n-got, +want:\n%s", test.in, diff)
		}
	}

	// Errors
	for _, in := range []*TableMetadata{
		{Schema: sc, ViewQuery: "q"}, // can't have both schema and query
		{UseLegacySQL: true},         // UseLegacySQL without query
		{UseStandardSQL: true},       // UseStandardSQL without query
		// read-only fields
		{FullID: "x"},
		{Type: "x"},
		{CreationTime: aTime},
		{LastModifiedTime: aTime},
		{NumBytes: 1},
		{NumRows: 1},
		{StreamingBuffer: &StreamingBuffer{}},
		{ETag: "x"},
	} {
		_, err := bqTableFromMetadata(in)
		if err == nil {
			t.Errorf("%+v: got nil, want error", in)
		}
	}
}

func TestBQTableFromMetadataToUpdate(t *testing.T) {
	aTime := time.Date(2017, 1, 26, 0, 0, 0, 0, time.Local)
	for _, test := range []struct {
		tm   TableMetadataToUpdate
		want *bq.Table
	}{
		{
			tm:   TableMetadataToUpdate{},
			want: &bq.Table{},
		},
		{
			tm: TableMetadataToUpdate{
				Description: "d",
				Name:        "n",
			},
			want: &bq.Table{
				Description:     "d",
				FriendlyName:    "n",
				ForceSendFields: []string{"Description", "FriendlyName"},
			},
		},
		{
			tm: TableMetadataToUpdate{
				Schema:         Schema{fieldSchema("desc", "name", "STRING", false, true)},
				ExpirationTime: aTime,
			},
			want: &bq.Table{
				Schema: &bq.TableSchema{
					Fields: []*bq.TableFieldSchema{
						bqTableFieldSchema("desc", "name", "STRING", "REQUIRED"),
					},
				},
				ExpirationTime:  aTime.UnixNano() / 1e6,
				ForceSendFields: []string{"Schema", "ExpirationTime"},
			},
		},
	} {
		got := bqTableFromMetadataToUpdate(test.tm)
		if !testutil.Equal(got, test.want) {
			t.Errorf("%v:\ngot  %v\nwant %v", test.tm, got, test.want)
		}
	}
}
