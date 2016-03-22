package main

import (
	"testing"
)

func TestCreateInstanceConfigs(t *testing.T) {
	for _, v := range []struct {
		desc string
		//inputs
		dir          string
		useFuse      bool
		instances    []string
		instancesSrc string

		// We don't need to check the []instancesConfig return value, we already
		// have a TestParseInstanceConfig.
		wantErr bool
	}{
		{
			"setting -fuse and -dir",
			"dir", true, nil, "", false,
		}, {
			"setting -fuse",
			"", true, nil, "", true,
		}, {
			"setting -fuse, -dir, and -instances",
			"dir", true, []string{"x"}, "", true,
		}, {
			"setting -fuse, -dir, and -instances_metadata",
			"dir", true, nil, "md", true,
		}, {
			"setting -dir and -instances (unix socket)",
			"dir", false, []string{"x"}, "", false,
		}, {
			"Seting -instance (unix socket)",
			"", false, []string{"x"}, "", true,
		}, {
			"setting -instance (tcp socket)",
			"", false, []string{"x=tcp:1234"}, "", false,
		}, {
			"setting -instance (tcp socket) and -instances_metadata",
			"", false, []string{"x=tcp:1234"}, "md", true,
		}, {
			"setting -dir, -instance (tcp socket), and -instances_metadata",
			"dir", false, []string{"x=tcp:1234"}, "md", false,
		}, {
			"setting -dir, -instance (unix socket), and -instances_metadata",
			"dir", false, []string{"x"}, "md", false,
		}, {
			"setting -dir and -instances_metadata",
			"dir", false, nil, "md", false,
		}, {
			"setting -instances_metadata",
			"", false, nil, "md", true,
		},
	} {
		_, err := CreateInstanceConfigs(v.dir, v.useFuse, v.instances, v.instancesSrc)
		if v.wantErr {
			if err == nil {
				t.Errorf("CreateInstanceConfigs passed when %s, wanted error", v.desc)
			}
			continue
		}
		if err != nil {
			t.Errorf("CreateInstanceConfigs gave error when %s: %v", v.desc, err)
		}
	}
}

func TestParseInstanceConfig(t *testing.T) {
	for _, v := range []struct {
		// inputs
		dir, instance string

		wantCfg instanceConfig
		wantErr bool
	}{
		{
			"/x", "my-instance",
			instanceConfig{"my-instance", "unix", "/x/my-instance"},
			false,
		}, {
			"/x", "my-instance=tcp:1234",
			instanceConfig{"my-instance", "tcp", "127.0.0.1:1234"},
			false,
		}, {
			"/x", "my-instance=tcp:my-host:1111",
			instanceConfig{"my-instance", "tcp", "my-host:1111"},
			false,
		}, {
			"/x", "my-instance=",
			instanceConfig{},
			true,
		}, {
			"/x", "my-instance=cool network",
			instanceConfig{},
			true,
		}, {
			"/x", "my-instance=cool network:1234",
			instanceConfig{},
			true,
		}, {
			"/x", "my-instance=oh:so:many:colons",
			instanceConfig{},
			true,
		},
	} {
		got, err := parseInstanceConfig(v.dir, v.instance)
		if v.wantErr {
			if err == nil {
				t.Errorf("parseInstanceConfig(%s, %s) = %+v, wanted error", got)
			}
			continue
		}
		if got != v.wantCfg {
			t.Errorf("parseInstanceConfig(%s, %s) = %+v, want %+v", v.dir, v.instance, got, v.wantCfg)
		}
	}
}
