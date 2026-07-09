package models

import "testing"

func int64Ptr(v int64) *int64 { return &v }

func TestUpdateResourcesRequestValidate(t *testing.T) {
	tests := []struct {
		name    string
		req     UpdateResourcesRequest
		wantErr bool
	}{
		{name: "empty request", req: UpdateResourcesRequest{}},
		{name: "valid memory", req: UpdateResourcesRequest{MemoryBytes: int64Ptr(512 * 1024 * 1024)}},
		{name: "zero memory removes limit", req: UpdateResourcesRequest{MemoryBytes: int64Ptr(0)}},
		{name: "negative memory", req: UpdateResourcesRequest{MemoryBytes: int64Ptr(-1)}, wantErr: true},
		{name: "valid nano cpus", req: UpdateResourcesRequest{NanoCPUs: int64Ptr(1500000000)}},
		{name: "negative nano cpus", req: UpdateResourcesRequest{NanoCPUs: int64Ptr(-5)}, wantErr: true},
		{name: "valid restart policy no", req: UpdateResourcesRequest{RestartPolicy: &RestartPolicySpec{Name: "no"}}},
		{name: "valid restart policy always", req: UpdateResourcesRequest{RestartPolicy: &RestartPolicySpec{Name: "always"}}},
		{name: "valid restart policy unless-stopped", req: UpdateResourcesRequest{RestartPolicy: &RestartPolicySpec{Name: "unless-stopped"}}},
		{name: "on-failure with retries", req: UpdateResourcesRequest{RestartPolicy: &RestartPolicySpec{Name: "on-failure", MaximumRetryCount: 3}}},
		{name: "invalid restart policy name", req: UpdateResourcesRequest{RestartPolicy: &RestartPolicySpec{Name: "sometimes"}}, wantErr: true},
		{name: "empty restart policy name", req: UpdateResourcesRequest{RestartPolicy: &RestartPolicySpec{Name: ""}}, wantErr: true},
		{name: "retries without on-failure", req: UpdateResourcesRequest{RestartPolicy: &RestartPolicySpec{Name: "always", MaximumRetryCount: 3}}, wantErr: true},
		{name: "negative retries", req: UpdateResourcesRequest{RestartPolicy: &RestartPolicySpec{Name: "on-failure", MaximumRetryCount: -1}}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
