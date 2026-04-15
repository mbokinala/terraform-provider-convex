package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestValidateEnvironmentVariableWriteModel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		model         environmentVariableResourceModel
		wantValue     string
		wantSensitive bool
		wantErrors    int
	}{
		{
			name: "plain value defaults to non-sensitive",
			model: environmentVariableResourceModel{
				DeploymentID: types.Int64Value(1),
				Name:         types.StringValue("EXAMPLE"),
				Value:        types.StringValue("plain"),
			},
			wantValue:     "plain",
			wantSensitive: false,
		},
		{
			name: "sensitive value is allowed when sensitive is true",
			model: environmentVariableResourceModel{
				DeploymentID:   types.Int64Value(1),
				Name:           types.StringValue("SECRET"),
				Sensitive:      types.BoolValue(true),
				SensitiveValue: types.StringValue("secret"),
			},
			wantValue:     "secret",
			wantSensitive: true,
		},
		{
			name: "plain value is rejected when sensitive is true",
			model: environmentVariableResourceModel{
				DeploymentID: types.Int64Value(1),
				Name:         types.StringValue("SECRET"),
				Value:        types.StringValue("plain"),
				Sensitive:    types.BoolValue(true),
			},
			wantErrors: 1,
		},
		{
			name: "sensitive value is rejected when sensitive is false",
			model: environmentVariableResourceModel{
				DeploymentID:   types.Int64Value(1),
				Name:           types.StringValue("EXAMPLE"),
				SensitiveValue: types.StringValue("secret"),
			},
			wantErrors: 1,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var diagnostics diag.Diagnostics
			_, _, gotValue, gotSensitive, ok := validateEnvironmentVariableWriteModel(tt.model, &diagnostics, "test")

			if len(diagnostics) != tt.wantErrors {
				t.Fatalf("validateEnvironmentVariableWriteModel() diagnostics = %d, want %d", len(diagnostics), tt.wantErrors)
			}

			if tt.wantErrors > 0 {
				if ok {
					t.Fatalf("validateEnvironmentVariableWriteModel() ok = true, want false")
				}
				return
			}

			if !ok {
				t.Fatalf("validateEnvironmentVariableWriteModel() ok = false, want true")
			}
			if gotValue != tt.wantValue {
				t.Fatalf("validateEnvironmentVariableWriteModel() value = %q, want %q", gotValue, tt.wantValue)
			}
			if gotSensitive != tt.wantSensitive {
				t.Fatalf("validateEnvironmentVariableWriteModel() sensitive = %t, want %t", gotSensitive, tt.wantSensitive)
			}
		})
	}
}

func TestSetEnvironmentVariableStateValue(t *testing.T) {
	t.Parallel()

	t.Run("plain value", func(t *testing.T) {
		t.Parallel()

		model := environmentVariableResourceModel{}
		setEnvironmentVariableStateValue(&model, "plain", false)

		if model.Sensitive.ValueBool() {
			t.Fatalf("setEnvironmentVariableStateValue() sensitive = true, want false")
		}
		if model.Value.ValueString() != "plain" {
			t.Fatalf("setEnvironmentVariableStateValue() value = %q, want %q", model.Value.ValueString(), "plain")
		}
		if !model.SensitiveValue.IsNull() {
			t.Fatalf("setEnvironmentVariableStateValue() sensitive_value = %q, want null", model.SensitiveValue.ValueString())
		}
	})

	t.Run("sensitive value", func(t *testing.T) {
		t.Parallel()

		model := environmentVariableResourceModel{}
		setEnvironmentVariableStateValue(&model, "secret", true)

		if !model.Sensitive.ValueBool() {
			t.Fatalf("setEnvironmentVariableStateValue() sensitive = false, want true")
		}
		if model.SensitiveValue.ValueString() != "secret" {
			t.Fatalf("setEnvironmentVariableStateValue() sensitive_value = %q, want %q", model.SensitiveValue.ValueString(), "secret")
		}
		if !model.Value.IsNull() {
			t.Fatalf("setEnvironmentVariableStateValue() value = %q, want null", model.Value.ValueString())
		}
	})
}
