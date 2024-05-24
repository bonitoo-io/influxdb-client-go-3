package influxdb3

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateBucket(t *testing.T) {
	correctPath := "/api/v2/buckets"

	tests := []struct {
		name     string
		bucket   Bucket
		wantBody map[string]any
	}{
		{
			name: "Apply bucket orgID and name",
			bucket: Bucket{
				OrgID: "my-organization",
				Name:  "my-bucket",
				RetentionRules: []BucketRetentionRule{
					{
						Type:         "expire",
						EverySeconds: 86400,
					},
				},
			},
			wantBody: map[string]any{
				"orgID": "my-organization",
				"name":  "my-bucket",
				"retentionRules": []any{
					map[string]any{
						"type":         "expire",
						"everySeconds": float64(86400),
					},
				},
			},
		},
		{
			name: "fallback to client config orgID and database name",
			bucket: Bucket{
				RetentionRules: []BucketRetentionRule{
					{
						Type:         "expire",
						EverySeconds: 86400,
					},
				},
			},
			wantBody: map[string]any{
				"orgID": "default-organization",
				"name":  "default-database",
				"retentionRules": []any{
					map[string]any{
						"type":         "expire",
						"everySeconds": float64(86400),
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// initialization of query client
				if r.Method == "PRI" {
					return
				}

				assert.EqualValues(t, correctPath, r.URL.String())
				bodyBytes, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				var body map[string]any
				err = json.Unmarshal(bodyBytes, &body)
				require.NoError(t, err)
				assert.Equal(t, tt.wantBody, body)
				w.WriteHeader(201)
			}))

			c, err := New(ClientConfig{
				Host:         ts.URL,
				Token:        "my-token",
				Organization: "default-organization",
				Database:     "default-database",
			})
			require.NoError(t, err)

			err = c.CreateBucket(context.Background(), &tt.bucket)
			require.NoError(t, err)
		})
	}
}
