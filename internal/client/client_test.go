package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestDecodeEnvironmentVariablesResponse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		payload string
		want    []EnvironmentVariable
	}{
		{
			name:    "top level array",
			payload: `[{"name":"TF_TEST_VAR","value":"hello"}]`,
			want: []EnvironmentVariable{
				{Name: "TF_TEST_VAR", Value: "hello"},
			},
		},
		{
			name:    "wrapped map",
			payload: `{"environmentVariables":{"BETA":"two","ALPHA":"one"}}`,
			want: []EnvironmentVariable{
				{Name: "ALPHA", Value: "one"},
				{Name: "BETA", Value: "two"},
			},
		},
		{
			name:    "wrapped array",
			payload: `{"environmentVariables":[{"name":"TF_TEST_VAR","value":"hello"}]}`,
			want: []EnvironmentVariable{
				{Name: "TF_TEST_VAR", Value: "hello"},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := decodeEnvironmentVariablesResponse(json.RawMessage(tt.payload))
			if err != nil {
				t.Fatalf("decodeEnvironmentVariablesResponse() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("decodeEnvironmentVariablesResponse() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestSelectDeploymentByName(t *testing.T) {
	t.Parallel()

	deployments := []Deployment{
		{ID: 1, Name: "alpha", ProjectID: 10, DeploymentType: "prod"},
		{ID: 2, Name: "beta", ProjectID: 11, DeploymentType: "dev"},
	}

	got, err := selectDeploymentByName(deployments, "beta")
	if err != nil {
		t.Fatalf("selectDeploymentByName() error = %v", err)
	}

	want := deployments[1]
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("selectDeploymentByName() = %#v, want %#v", got, want)
	}
}

func TestSelectDeploymentByID(t *testing.T) {
	t.Parallel()

	deployments := []Deployment{
		{ID: 1, Name: "alpha", ProjectID: 10, DeploymentType: "prod"},
		{ID: 2, Name: "beta", ProjectID: 11, DeploymentType: "dev"},
	}

	got, err := selectDeploymentByID(deployments, 1)
	if err != nil {
		t.Fatalf("selectDeploymentByID() error = %v", err)
	}

	want := deployments[0]
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("selectDeploymentByID() = %#v, want %#v", got, want)
	}
}

func TestSetEnvironmentVariablesContextRetriesOptimisticConcurrencyControlFailure(t *testing.T) {
	t.Parallel()

	attempts := 0
	sleeps := make([]time.Duration, 0, 2)
	client := &ConvexClient{
		token: "test-token",
		httpClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				attempts++
				if !strings.HasSuffix(req.URL.Path, updateEnvironmentVariablesPath) {
					t.Fatalf("request path = %q, want suffix %q", req.URL.Path, updateEnvironmentVariablesPath)
				}

				if attempts < 3 {
					return jsonResponse(http.StatusServiceUnavailable, `{"code":"OptimisticConcurrencyControlFailure","message":"try again"}`), nil
				}

				return jsonResponse(http.StatusOK, `{}`), nil
			}),
		},
		sleep: func(_ context.Context, duration time.Duration) error {
			sleeps = append(sleeps, duration)
			return nil
		},
	}

	value := "hello"
	err := client.SetEnvironmentVariablesContext(context.Background(), "demo", []EnvVarChange{{
		Name:  "EXAMPLE",
		Value: &value,
	}})
	if err != nil {
		t.Fatalf("SetEnvironmentVariablesContext() error = %v", err)
	}
	if attempts != 3 {
		t.Fatalf("SetEnvironmentVariablesContext() attempts = %d, want 3", attempts)
	}

	wantSleeps := []time.Duration{environmentVariableWriteBackoff, 2 * environmentVariableWriteBackoff}
	if !reflect.DeepEqual(sleeps, wantSleeps) {
		t.Fatalf("SetEnvironmentVariablesContext() sleeps = %#v, want %#v", sleeps, wantSleeps)
	}
}

func TestSetEnvironmentVariablesContextDoesNotRetryNonRetryableErrors(t *testing.T) {
	t.Parallel()

	attempts := 0
	client := &ConvexClient{
		token: "test-token",
		httpClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				attempts++
				return jsonResponse(http.StatusBadRequest, `{"code":"BadRequest","message":"nope"}`), nil
			}),
		},
		sleep: func(_ context.Context, _ time.Duration) error {
			t.Fatal("sleep should not be called for non-retryable errors")
			return nil
		},
	}

	value := "hello"
	err := client.SetEnvironmentVariablesContext(context.Background(), "demo", []EnvVarChange{{
		Name:  "EXAMPLE",
		Value: &value,
	}})
	if err == nil {
		t.Fatal("SetEnvironmentVariablesContext() error = nil, want non-nil")
	}
	if attempts != 1 {
		t.Fatalf("SetEnvironmentVariablesContext() attempts = %d, want 1", attempts)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func jsonResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Status:     http.StatusText(statusCode),
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
