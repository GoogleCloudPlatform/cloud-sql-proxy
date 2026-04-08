package main

import (
	"os"
	"reflect"
	"testing"
)

func TestTranslateV2Args(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		wantArgs     []string
		wantLogDebug bool
		wantVerbose  bool
		wantOk       bool
	}{
		{
			name:        "legacy flag",
			args:        []string{"cloud_sql_proxy", "-legacy-v1-proxy"},
			wantVerbose: false,
			wantOk:      false,
		},
		{
			name:        "simple instance",
			args:        []string{"cloud_sql_proxy", "-instances=proj:reg:inst"},
			wantArgs:    []string{"--auto-ip", "proj:reg:inst"},
			wantVerbose: true,
			wantOk:      true,
		},
		{
			name:        "instance with tcp port",
			args:        []string{"cloud_sql_proxy", "-instances=proj:reg:inst=tcp:3306"},
			wantArgs:    []string{"--auto-ip", "proj:reg:inst?port=3306"},
			wantVerbose: true,
			wantOk:      true,
		},
		{
			name:        "instance with tcp host:port",
			args:        []string{"cloud_sql_proxy", "-instances=proj:reg:inst=tcp:0.0.0.0:3306"},
			wantArgs:    []string{"--auto-ip", "proj:reg:inst?address=0.0.0.0&port=3306"},
			wantVerbose: true,
			wantOk:      true,
		},
		{
			name:        "instance with custom unix socket",
			args:        []string{"cloud_sql_proxy", "-dir=/tmp", "-instances=proj:reg:inst=unix:custom"},
			wantArgs:    []string{"--unix-socket", "/tmp", "--auto-ip", "proj:reg:inst?unix-socket=custom"},
			wantVerbose: true,
			wantOk:      true,
		},
		{
			name:        "instance with absolute unix socket",
			args:        []string{"cloud_sql_proxy", "-instances=proj:reg:inst=unix:/tmp/socket"},
			wantArgs:    []string{"--auto-ip", "proj:reg:inst?unix-socket-path=/tmp/socket"},
			wantVerbose: true,
			wantOk:      true,
		},
		{
			name:        "multiple instances",
			args:        []string{"cloud_sql_proxy", "-instances=i1=tcp:3306,i2=tcp:3307"},
			wantArgs:    []string{"--auto-ip", "i1?port=3306", "i2?port=3307"},
			wantVerbose: true,
			wantOk:      true,
		},
		{
			name:        "credentials and other flags",
			args:        []string{"cloud_sql_proxy", "-credential_file=key.json", "-instances=i1", "-quiet"},
			wantArgs:    []string{"--credentials-file", "key.json", "--quiet", "--auto-ip", "i1"},
			wantVerbose: true,
			wantOk:      true,
		},
		{
			name:         "log_debug_stdout",
			args:         []string{"cloud_sql_proxy", "-instances=i1", "-log_debug_stdout"},
			wantArgs:     []string{"--auto-ip", "i1"},
			wantLogDebug: true,
			wantVerbose:  true,
			wantOk:       true,
		},
		{
			name:         "verbose_true",
			args:         []string{"cloud_sql_proxy", "-instances=i1", "-verbose=true"},
			wantArgs:     []string{"--auto-ip", "i1"},
			wantLogDebug: false,
			wantVerbose:  true,
			wantOk:       true,
		},
		{
			name:         "verbose_false",
			args:         []string{"cloud_sql_proxy", "-instances=i1", "-verbose=false"},
			wantArgs:     []string{"--auto-ip", "i1"},
			wantLogDebug: false,
			wantVerbose:  false,
			wantOk:       true,
		},
		{
			name:        "ip_address_types private",
			args:        []string{"cloud_sql_proxy", "-ip_address_types=PRIVATE", "-instances=i1"},
			wantArgs:    []string{"--private-ip", "i1"},
			wantVerbose: true,
			wantOk:      true,
		},
		{
			name:        "ip_address_types both",
			args:        []string{"cloud_sql_proxy", "-ip_address_types=PUBLIC,PRIVATE", "-instances=i1"},
			wantArgs:    []string{"--auto-ip", "i1"},
			wantVerbose: true,
			wantOk:      true,
		},
		{
			name:        "ignored flag fd_rlimit",
			args:        []string{"cloud_sql_proxy", "-instances=i1", "-fd_rlimit=10000"},
			wantArgs:    []string{"--auto-ip", "i1"},
			wantVerbose: true,
			wantOk:      true,
		},
		{
			name:        "ignored flag refresh_config_throttle",
			args:        []string{"cloud_sql_proxy", "-instances=i1", "-refresh_config_throttle=1m"},
			wantArgs:    []string{"--auto-ip", "i1"},
			wantVerbose: true,
			wantOk:      true,
		},
		{
			name:   "unsupported flag projects",
			args:   []string{"cloud_sql_proxy", "-projects=p1", "-instances=i1"},
			wantOk: false,
		},
		{
			name:     "fuse mode",
			args:     []string{"cloud_sql_proxy", "-dir=/tmp", "-fuse"},
			wantArgs: []string{"--unix-socket", "/tmp", "--fuse", "/tmp", "--auto-ip"},
			wantVerbose: true,
			wantOk:      true,
		},
		{
			name:     "v2 flag",
			args:     []string{"cloud_sql_proxy", "-v2", "--debug-logs", "p:r:i"},
			wantArgs: []string{"--debug-logs", "p:r:i"},
			wantOk:   true,
		},
		{
			name:        "v2-compat flag",
			args:        []string{"cloud_sql_proxy", "-v2-compat", "-instances=i1"},
			wantArgs:    []string{"--auto-ip", "i1"},
			wantVerbose: true,
			wantOk:      true,
		},
		{
			name:        "instances_metadata flag",
			args:        []string{"cloud_sql_proxy", "-instances_metadata=path/to/attr"},
			wantArgs:    []string{"--instances-metadata", "path/to/attr", "--auto-ip"},
			wantVerbose: true,
			wantOk:      true,
		},
		}


	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldArgs := os.Args
			defer func() { os.Args = oldArgs }()
			os.Args = tt.args

			gotArgs, gotLogDebug, gotVerbose, _, _, _, gotV2Mode, gotOk := translateV2Args()
			if gotOk != tt.wantOk {
				t.Errorf("translateV2Args() ok = %v, want %v", gotOk, tt.wantOk)
			}
			if gotOk {
				if !reflect.DeepEqual(gotArgs, tt.wantArgs) {
					t.Errorf("translateV2Args() args = %v, want %v", gotArgs, tt.wantArgs)
				}
				if gotLogDebug != tt.wantLogDebug {
					t.Errorf("translateV2Args() logDebug = %v, want %v", gotLogDebug, tt.wantLogDebug)
				}
				if gotVerbose != tt.wantVerbose {
					t.Errorf("translateV2Args() verbose = %v, want %v", gotVerbose, tt.wantVerbose)
				}
				if gotV2Mode != (tt.name == "v2 flag") {
					t.Errorf("translateV2Args() v2Mode = %v, want %v", gotV2Mode, tt.name == "v2 flag")
				}
			}
		})
	}
}
