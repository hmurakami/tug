package compose_test

import (
	"strings"
	"testing"

	"github.com/mickamy/tug/internal/compose"
	"github.com/mickamy/tug/internal/config"
)

func TestClassify_HTTP(t *testing.T) {
	t.Parallel()

	proj := compose.Project{
		Name: "myapp",
		Services: []compose.Service{
			{Name: "api", Image: "node:20", Ports: []compose.Port{{Host: 3000, Container: 3000}}},
		},
	}

	result := compose.Classify(proj, config.Config{})
	if len(result) != 1 {
		t.Fatalf("got %d services, want 1", len(result))
	}
	if result[0].Kind != compose.KindHTTP {
		t.Errorf("kind: got %d, want KindHTTP", result[0].Kind)
	}
}

func TestClassify_TCP_ByPort(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		containerPort uint16
	}{
		{"postgres", 5432},
		{"mysql", 3306},
		{"redis", 6379},
		{"mongo", 27017},
		{"memcached", 11211},
		{"rabbitmq", 5672},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			proj := compose.Project{
				Name: "myapp",
				Services: []compose.Service{
					{Name: "db", Image: "whatever", Ports: []compose.Port{{Host: tt.containerPort, Container: tt.containerPort}}},
				},
			}

			result := compose.Classify(proj, config.Config{})
			if result[0].Kind != compose.KindTCP {
				t.Errorf("port %d: got KindHTTP, want KindTCP", tt.containerPort)
			}
			if result[0].HostPort < 10000 || result[0].HostPort >= 60000 {
				t.Errorf("host port %d out of range", result[0].HostPort)
			}
		})
	}
}

func TestClassify_TCP_CustomImage(t *testing.T) {
	t.Parallel()

	proj := compose.Project{
		Name: "myapp",
		Services: []compose.Service{
			{Name: "db", Image: "ghcr.io/myorg/custom-db:v1", Ports: []compose.Port{{Host: 5432, Container: 5432}}},
		},
	}

	result := compose.Classify(proj, config.Config{})
	if result[0].Kind != compose.KindTCP {
		t.Error("custom image with well-known port should be KindTCP")
	}
}

func TestClassify_ConfigOverride_TCP(t *testing.T) {
	t.Parallel()

	proj := compose.Project{
		Name: "myapp",
		Services: []compose.Service{
			// Port 9090 is not a well-known TCP port, but config says TCP.
			{Name: "custom-db", Image: "myimage", Ports: []compose.Port{{Host: 9090, Container: 9090}}},
		},
	}

	cfg := config.Config{
		Services: map[string]config.ServiceConfig{
			"custom-db": {Kind: "tcp"},
		},
	}

	result := compose.Classify(proj, cfg)
	if result[0].Kind != compose.KindTCP {
		t.Error("config override to tcp should take effect")
	}
	if result[0].ContainerPort != 9090 {
		t.Errorf("container port: got %d, want 9090", result[0].ContainerPort)
	}
}

func TestClassify_ConfigOverride_HTTP(t *testing.T) {
	t.Parallel()

	proj := compose.Project{
		Name: "myapp",
		Services: []compose.Service{
			// Port 5432 would normally be TCP, but config says HTTP.
			{Name: "pg-proxy", Image: "myproxy", Ports: []compose.Port{{Host: 5432, Container: 5432}}},
		},
	}

	cfg := config.Config{
		Services: map[string]config.ServiceConfig{
			"pg-proxy": {Kind: "http"},
		},
	}

	result := compose.Classify(proj, cfg)
	if result[0].Kind != compose.KindHTTP {
		t.Error("config override to http should take effect")
	}
}

func TestClassify_DeterministicPort(t *testing.T) {
	t.Parallel()

	proj := compose.Project{
		Name: "myapp",
		Services: []compose.Service{
			{Name: "db", Image: "postgres:16", Ports: []compose.Port{{Host: 5432, Container: 5432}}},
		},
	}

	a := compose.Classify(proj, config.Config{})
	b := compose.Classify(proj, config.Config{})
	if a[0].HostPort != b[0].HostPort {
		t.Errorf("expected deterministic port, got %d and %d", a[0].HostPort, b[0].HostPort)
	}
}

func TestGenerateOverride_HTTP(t *testing.T) {
	t.Parallel()

	proj := compose.Project{
		Name: "myapp",
		Services: []compose.Service{
			{Name: "api", Image: "node:20", Ports: []compose.Port{{Host: 3000, Container: 3000}}},
		},
	}
	classified := compose.Classify(proj, config.Config{})

	data, err := compose.GenerateOverride(proj, classified)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := string(data)
	if !strings.Contains(out, "traefik.enable=true") {
		t.Error("missing traefik.enable label")
	}
	if !strings.Contains(out, "api.myapp.localhost") {
		t.Error("missing host rule")
	}
	if !strings.Contains(out, "tug") {
		t.Error("missing tug network")
	}
}

func TestGenerateOverride_TCP(t *testing.T) {
	t.Parallel()

	proj := compose.Project{
		Name: "myapp",
		Services: []compose.Service{
			{Name: "db", Image: "postgres:16", Ports: []compose.Port{{Host: 5432, Container: 5432}}},
		},
	}
	classified := compose.Classify(proj, config.Config{})

	data, err := compose.GenerateOverride(proj, classified)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := string(data)
	if !strings.Contains(out, "target: 5432") {
		t.Error("missing target port")
	}
	if !strings.Contains(out, "protocol: tcp") {
		t.Error("missing protocol")
	}
}
