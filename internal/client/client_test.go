package client

import (
	"encoding/json"
	"reflect"
	"testing"
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
