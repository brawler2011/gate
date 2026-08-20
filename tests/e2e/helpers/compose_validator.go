package helpers

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// ComposePort represents a parsed Docker port mapping.
type ComposePort struct {
	HostIP        string
	HostPort      int
	ContainerPort int
	Protocol      string
}

// ComposeService represents a service declaration in Docker Compose.
type ComposeService struct {
	Image       string                 `yaml:"image"`
	Ports       []interface{}          `yaml:"ports"`
	Environment interface{}            `yaml:"environment"`
	Networks    interface{}            `yaml:"networks"`
	Volumes     []interface{}          `yaml:"volumes"`
	Restart     string                 `yaml:"restart"`
	Deploy      map[string]interface{} `yaml:"deploy"`
	Labels      interface{}            `yaml:"labels"`
}

// ComposeConfig represents a parsed docker-compose.yml structure.
type ComposeConfig struct {
	Version  string                             `yaml:"version"`
	Services map[string]ComposeService          `yaml:"services"`
	Networks map[string]interface{}             `yaml:"networks"`
	Volumes  map[string]interface{}             `yaml:"volumes"`
}

// ParseComposeFile reads and parses a Docker Compose YAML file into ComposeConfig.
func ParseComposeFile(path string) (*ComposeConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read compose file %s: %w", path, err)
	}

	var cfg ComposeConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse compose file %s: %w", path, err)
	}

	return &cfg, nil
}

// ValidateComposeFile validates that the compose file exists and parses cleanly.
func ValidateComposeFile(t *testing.T, path string) *ComposeConfig {
	t.Helper()
	cfg, err := ParseComposeFile(path)
	require.NoError(t, err, "Failed to parse Docker Compose file: %s", path)
	require.NotEmpty(t, cfg.Services, "Docker Compose file %s has no services", path)
	return cfg
}

// ParsePortString parses various Docker Compose port formats:
// "8888:8888", "127.0.0.1:8888:8888", "4317:4317/tcp", 8889, or short/long syntax.
func ParsePortString(raw interface{}) (*ComposePort, error) {
	switch v := raw.(type) {
	case int:
		return &ComposePort{
			HostPort:      v,
			ContainerPort: v,
			Protocol:      "tcp",
		}, nil
	case string:
		// Check for protocol suffix (e.g., "/udp" or "/tcp")
		proto := "tcp"
		portStr := v
		if strings.Contains(portStr, "/") {
			parts := strings.Split(portStr, "/")
			portStr = parts[0]
			proto = parts[1]
		}

		parts := strings.Split(portStr, ":")
		switch len(parts) {
		case 1:
			p, err := strconv.Atoi(parts[0])
			if err != nil {
				return nil, fmt.Errorf("invalid port: %s", v)
			}
			return &ComposePort{HostPort: p, ContainerPort: p, Protocol: proto}, nil
		case 2:
			hp, err1 := strconv.Atoi(parts[0])
			cp, err2 := strconv.Atoi(parts[1])
			if err1 != nil || err2 != nil {
				return nil, fmt.Errorf("invalid port mapping: %s", v)
			}
			return &ComposePort{HostPort: hp, ContainerPort: cp, Protocol: proto}, nil
		case 3:
			ip := parts[0]
			hp, err1 := strconv.Atoi(parts[1])
			cp, err2 := strconv.Atoi(parts[2])
			if err1 != nil || err2 != nil {
				return nil, fmt.Errorf("invalid port mapping with IP: %s", v)
			}
			return &ComposePort{HostIP: ip, HostPort: hp, ContainerPort: cp, Protocol: proto}, nil
		default:
			return nil, fmt.Errorf("unrecognized port format: %s", v)
		}
	case map[string]interface{}:
		// Long syntax: target, published, protocol, host_ip
		cpVal, _ := v["target"].(int)
		hpVal, _ := v["published"].(int)
		protoVal, _ := v["protocol"].(string)
		if protoVal == "" {
			protoVal = "tcp"
		}
		ipVal, _ := v["host_ip"].(string)
		return &ComposePort{
			HostIP:        ipVal,
			HostPort:      hpVal,
			ContainerPort: cpVal,
			Protocol:      protoVal,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported port data type: %T", raw)
	}
}

// GetServicePorts returns all parsed ports for a given service.
func (c *ComposeConfig) GetServicePorts(serviceName string) ([]ComposePort, error) {
	svc, ok := c.Services[serviceName]
	if !ok {
		return nil, fmt.Errorf("service %s not found in compose", serviceName)
	}

	var ports []ComposePort
	for _, raw := range svc.Ports {
		p, err := ParsePortString(raw)
		if err != nil {
			return nil, fmt.Errorf("service %s: %w", serviceName, err)
		}
		ports = append(ports, *p)
	}
	return ports, nil
}

// AssertServicePresent asserts that the named service is defined in the compose configuration.
func AssertServicePresent(t *testing.T, cfg *ComposeConfig, serviceName string) {
	t.Helper()
	_, ok := cfg.Services[serviceName]
	require.True(t, ok, "Expected service '%s' to be present in docker-compose", serviceName)
}

// AssertPortAllocation verifies that a service exposes a specific host port.
func AssertPortAllocation(t *testing.T, cfg *ComposeConfig, serviceName string, expectedHostPort, expectedContainerPort int) {
	t.Helper()
	ports, err := cfg.GetServicePorts(serviceName)
	require.NoError(t, err, "Failed to get ports for service %s", serviceName)

	found := false
	for _, p := range ports {
		if p.HostPort == expectedHostPort && p.ContainerPort == expectedContainerPort {
			found = true
			break
		}
	}
	require.True(t, found, "Service '%s' does not bind host port %d to container port %d (got: %+v)",
		serviceName, expectedHostPort, expectedContainerPort, ports)
}

// AssertNoPortCollision checks that no two services bind to the same host port,
// and specifically asserts that targetPort1 does not collide with targetPort2.
func AssertNoPortCollision(t *testing.T, cfg *ComposeConfig, port1, port2 int) {
	t.Helper()
	require.NotEqual(t, port1, port2, "Port %d and %d are identical", port1, port2)

	allocatedHostPorts := make(map[int]string) // port -> serviceName
	for svcName, svc := range cfg.Services {
		for _, rawPort := range svc.Ports {
			p, err := ParsePortString(rawPort)
			if err != nil || p.HostPort == 0 {
				continue
			}

			if existingSvc, exists := allocatedHostPorts[p.HostPort]; exists {
				require.Fail(t, fmt.Sprintf("Port collision detected! Host port %d is bound by both '%s' and '%s'",
					p.HostPort, existingSvc, svcName))
			}
			allocatedHostPorts[p.HostPort] = svcName
		}
	}
}

// AssertNamedVolumePresent asserts that a named volume exists in the compose configuration.
func AssertNamedVolumePresent(t *testing.T, cfg *ComposeConfig, volumeName string) {
	t.Helper()
	_, ok := cfg.Volumes[volumeName]
	require.True(t, ok, "Expected named volume '%s' in docker-compose volumes", volumeName)
}
