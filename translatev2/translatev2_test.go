package translatev2_test

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/microsoft/go-mssqldb"
)

func buildProxy(t *testing.T) string {
	tmpDir, err := os.MkdirTemp("", "proxy-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	binPath := filepath.Join(tmpDir, "cloud_sql_proxy")
	if runtime.GOOS == "windows" {
		binPath += ".exe"
	}

	cmd := exec.Command("go", "build", "-o", binPath, "../cmd/cloud_sql_proxy")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build proxy: %v\n%s", err, out)
	}

	return binPath
}

func TestVersionFlag(t *testing.T) {
	binPath := buildProxy(t)
	defer os.RemoveAll(filepath.Dir(binPath))

	// Run with -version (translated to v2)
	cmd := exec.Command(binPath, "-version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to run proxy -version: %v\n%s", err, out)
	}
	output := string(out)
	if !strings.Contains(output, "cloud-sql-proxy version") {
		t.Errorf("expected v2 version output, got: %s", output)
	}

	// Run with -legacy-v1-proxy -version (stay in v1)
	cmd = exec.Command(binPath, "-legacy-v1-proxy", "-version")
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to run proxy -legacy-v1-proxy -version: %v\n%s", err, out)
	}
	output = string(out)
	if !strings.Contains(output, "Cloud SQL Auth proxy:") {
		t.Errorf("expected v1 version output, got: %s", output)
	}
}

type safeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *safeBuffer) Write(p []byte) (n int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *safeBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

func TestModeComparison(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration tests")
	}
	binPath := buildProxy(t)
	defer os.RemoveAll(filepath.Dir(binPath))

	tests := []struct {
		name       string
		args       []string
		v2Expected bool
	}{
		{
			name:       "default mode is v2",
			args:       []string{"-instances=p:r:i=tcp:3308"},
			v2Expected: true,
		},
		{
			name:       "legacy mode is v1",
			args:       []string{"-legacy-v1-proxy", "-instances=p:r:i=tcp:3309"},
			v2Expected: false,
		},
		{
			name:       "verbose enables debug logs in v2",
			args:       []string{"-instances=p:r:i=tcp:3310", "-verbose"},
			v2Expected: true,
		},
		{
			name:       "ignored fd_rlimit stays in v2",
			args:       []string{"-instances=p:r:i=tcp:3311", "-fd_rlimit=10000"},
			v2Expected: true,
		},
		{
			name:       "ignored refresh_config_throttle stays in v2",
			args:       []string{"-instances=p:r:i=tcp:3312", "-refresh_config_throttle=1m"},
			v2Expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			cmd := exec.CommandContext(ctx, binPath, tt.args...)
			outBuf := &safeBuffer{}
			cmd.Stdout = outBuf
			cmd.Stderr = outBuf

			err := cmd.Start()
			if err != nil {
				t.Fatalf("failed to start proxy: %v", err)
			}

			found := false
			for {
				select {
				case <-ctx.Done():
					t.Errorf("timeout waiting for proxy mode identification. Current output: %s", outBuf.String())
					goto end
				default:
					output := outBuf.String()
					if tt.v2Expected {
						// V2 indicators
						if strings.Contains(output, "The proxy has started successfully") &&
							strings.Contains(output, "Ready for new connections") {
							found = true
							goto end
						}
					} else {
						// V1 indicators
						if strings.Contains(output, "Ready for new connections") ||
							strings.Contains(output, "error checking scopes") ||
							strings.Contains(output, "errors parsing config") {
							found = true
							goto end
						}
					}

					// Check if process exited early
					if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
						t.Errorf("proxy exited early without identifying mode. Output: %s", output)
						goto end
					}

					time.Sleep(200 * time.Millisecond)
				}
			}
		end:
			if cmd.Process != nil {
				cmd.Process.Kill()
			}
			if !found {
				t.Errorf("Mode identification failed. Output: %s", outBuf.String())
			}
		})
	}
}

func getFreePort(t *testing.T) int {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to get free port: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func runQuery(t *testing.T, driver, dsn string) {
	db, err := sql.Open(driver, dsn)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var now time.Time
	err = db.QueryRowContext(ctx, "SELECT NOW()").Scan(&now)
	if err != nil {
		t.Fatalf("failed to run query: %v", err)
	}
	t.Logf("Query successful, time: %v", now)
}

func runSQLServerQuery(t *testing.T, dsn string) {
	db, err := sql.Open("sqlserver", dsn)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var now time.Time
	err = db.QueryRowContext(ctx, "SELECT GETDATE()").Scan(&now)
	if err != nil {
		t.Fatalf("failed to run query: %v", err)
	}
	t.Logf("Query successful, time: %v", now)
}

func startProxy(t *testing.T, binPath string, args ...string) (*exec.Cmd, *safeBuffer) {
	ctx, cancel := context.WithCancel(context.Background())
	_ = cancel

	cmd := exec.CommandContext(ctx, binPath, args...)
	outBuf := &safeBuffer{}
	cmd.Stdout = outBuf
	cmd.Stderr = outBuf

	err := cmd.Start()
	if err != nil {
		t.Fatalf("failed to start proxy: %v", err)
	}

	// Wait for "Ready for new connections"
	found := false
	start := time.Now()
	for time.Since(start) < 20*time.Second {
		if strings.Contains(outBuf.String(), "Ready for new connections") {
			found = true
			break
		}
		if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
			t.Fatalf("proxy exited early: %s", outBuf.String())
		}
		time.Sleep(200 * time.Millisecond)
	}

	if !found {
		cmd.Process.Kill()
		t.Fatalf("timeout waiting for proxy to be ready. Output: %s", outBuf.String())
	}

	return cmd, outBuf
}

func testMySQL(t *testing.T, binPath, connName, dbName, user, pass, iamUser string, iam bool) {
	modes := []struct {
		name string
		args []string
	}{
		{name: "translated_v2", args: []string{}},
		{name: "legacy_v1", args: []string{"-legacy-v1-proxy"}},
	}

	for _, mode := range modes {
		t.Run(mode.name, func(t *testing.T) {
			baseArgs := append([]string{}, mode.args...)
			if iam {
				baseArgs = append(baseArgs, "-enable_iam_login")
			}

			// TCP
			t.Run("TCP", func(t *testing.T) {
				port := getFreePort(t)
				args := append([]string{}, baseArgs...)
				args = append(args, fmt.Sprintf("-instances=%s=tcp:%d", connName, port))

				cmd, _ := startProxy(t, binPath, args...)
				defer cmd.Process.Kill()

				u := user
				allowCleartext := ""
				if iamUser != "" {
					u = iamUser
					allowCleartext = "&allowCleartextPasswords=1"
				}
				dsn := fmt.Sprintf("%s:%s@tcp(127.0.0.1:%d)/%s?parseTime=true%s", u, pass, port, dbName, allowCleartext)
				runQuery(t, "mysql", dsn)
			})

			// Unix
			t.Run("Unix", func(t *testing.T) {
				// Use a shorter path for Unix sockets to avoid "bind: invalid argument"
				tmpDir, err := os.MkdirTemp("/tmp", "m-u-*")
				if err != nil {
					t.Fatalf("failed to create temp dir: %v", err)
				}
				defer os.RemoveAll(tmpDir)

				args := append([]string{}, baseArgs...)
				args = append(args, "-dir="+tmpDir, fmt.Sprintf("-instances=%s", connName))

				cmd, _ := startProxy(t, binPath, args...)
				defer cmd.Process.Kill()

				u := user
				allowCleartext := ""
				if iamUser != "" {
					u = iamUser
					allowCleartext = "&allowCleartextPasswords=1"
				}
				socketPath := filepath.Join(tmpDir, connName)
				dsn := fmt.Sprintf("%s:%s@unix(%s)/%s?parseTime=true%s", u, pass, socketPath, dbName, allowCleartext)
				runQuery(t, "mysql", dsn)
			})

			// FUSE
			if runtime.GOOS == "linux" {
				t.Run("FUSE", func(t *testing.T) {
					tmpDir, err := os.MkdirTemp("", "mysql-fuse-*")
					if err != nil {
						t.Fatalf("failed to create temp dir: %v", err)
					}
					defer os.RemoveAll(tmpDir)

					args := append([]string{}, mode.args...)
					args = append(args, "-dir="+tmpDir, "-fuse")

					cmd, _ := startProxy(t, binPath, args...)
					defer cmd.Process.Kill()

					u := user
					if iamUser != "" {
						u = iamUser
					}
					socketPath := filepath.Join(tmpDir, connName)
					dsn := fmt.Sprintf("%s:%s@unix(%s)/%s?parseTime=true", u, pass, socketPath, dbName)
					runQuery(t, "mysql", dsn)
				})
			}
		})
	}
}

func TestE2EMySQL(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E tests")
	}
	connName := os.Getenv("MYSQL_CONNECTION_NAME")
	if connName == "" {
		t.Skip("MYSQL_CONNECTION_NAME not set")
	}
	dbName := os.Getenv("MYSQL_DB")
	user := os.Getenv("MYSQL_USER")
	pass := os.Getenv("MYSQL_PASS")

	binPath := buildProxy(t)
	defer os.RemoveAll(filepath.Dir(binPath))

	t.Run("PasswordAuth", func(t *testing.T) {
		testMySQL(t, binPath, connName, dbName, user, pass, "", false)
	})

	iamUser := os.Getenv("MYSQL_IAM_USER")
	if iamUser != "" {
		t.Run("IAMAuth", func(t *testing.T) {
			testMySQL(t, binPath, connName, dbName, iamUser, "", iamUser, true)
		})
	}

	mcpConnName := os.Getenv("MYSQL_MCP_CONNECTION_NAME")
	if mcpConnName != "" {
		mcpPass := os.Getenv("MYSQL_MCP_PASS")
		t.Run("MCPInstance", func(t *testing.T) {
			testMySQL(t, binPath, mcpConnName, dbName, user, mcpPass, "", false)
		})
	}
}

func testPostgres(t *testing.T, binPath, connName, dbName, user, pass string, iam bool) {
	modes := []struct {
		name string
		args []string
	}{
		{name: "translated_v2", args: []string{}},
		{name: "legacy_v1", args: []string{"-legacy-v1-proxy"}},
	}

	if dbName == "" {
		dbName = "postgres"
	}

	for _, mode := range modes {
		t.Run(mode.name, func(t *testing.T) {
			baseArgs := append([]string{}, mode.args...)
			if iam {
				baseArgs = append(baseArgs, "-enable_iam_login")
			}

			// TCP
			t.Run("TCP", func(t *testing.T) {
				port := getFreePort(t)
				args := append([]string{}, baseArgs...)
				args = append(args, fmt.Sprintf("-instances=%s=tcp:%d", connName, port))

				cmd, _ := startProxy(t, binPath, args...)
				defer cmd.Process.Kill()

				// Use URL-style DSN to better handle special characters in user/pass
				dsn := fmt.Sprintf("postgres://%s:%s@127.0.0.1:%d/%s?sslmode=disable",
					url.QueryEscape(user), url.QueryEscape(pass), port, url.QueryEscape(dbName))
				runQuery(t, "postgres", dsn)
			})

			// Unix
			t.Run("Unix", func(t *testing.T) {
				// Use a shorter path for Unix sockets
				tmpDir, err := os.MkdirTemp("/tmp", "p-u-*")
				if err != nil {
					t.Fatalf("failed to create temp dir: %v", err)
				}
				defer os.RemoveAll(tmpDir)

				args := append([]string{}, baseArgs...)
				args = append(args, "-dir="+tmpDir, fmt.Sprintf("-instances=%s", connName))

				cmd, _ := startProxy(t, binPath, args...)
				defer cmd.Process.Kill()

				hostDir := filepath.Join(tmpDir, connName)
				dsn := fmt.Sprintf("postgres://%s:%s@/%s?host=%s&sslmode=disable",
					url.QueryEscape(user), url.QueryEscape(pass), url.QueryEscape(dbName), url.QueryEscape(hostDir))
				runQuery(t, "postgres", dsn)
			})

			// FUSE
			if runtime.GOOS == "linux" {
				t.Run("FUSE", func(t *testing.T) {
					tmpDir, err := os.MkdirTemp("", "postgres-fuse-*")
					if err != nil {
						t.Fatalf("failed to create temp dir: %v", err)
					}
					defer os.RemoveAll(tmpDir)

					args := append([]string{}, baseArgs...)
					args = append(args, "-dir="+tmpDir, "-fuse")

					cmd, _ := startProxy(t, binPath, args...)
					defer cmd.Process.Kill()

					dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s sslmode=disable", tmpDir, user, pass, dbName)
					runQuery(t, "postgres", dsn)
				})
			}
		})
	}
}

func TestE2EPostgres(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E tests")
	}
	connName := os.Getenv("POSTGRES_CONNECTION_NAME")
	if connName == "" {
		t.Skip("POSTGRES_CONNECTION_NAME not set")
	}
	dbName := os.Getenv("POSTGRES_DB")
	user := os.Getenv("POSTGRES_USER")
	pass := os.Getenv("POSTGRES_PASS")

	binPath := buildProxy(t)
	defer os.RemoveAll(filepath.Dir(binPath))

	t.Run("PasswordAuth", func(t *testing.T) {
		testPostgres(t, binPath, connName, dbName, user, pass, false)
	})

	iamUser := os.Getenv("POSTGRES_USER_IAM")
	if iamUser != "" {
		t.Run("IAMAuth", func(t *testing.T) {
			testPostgres(t, binPath, connName, dbName, iamUser, "", true)
		})
	}

	casConnName := os.Getenv("POSTGRES_CAS_CONNECTION_NAME")
	if casConnName != "" {
		casPass := os.Getenv("POSTGRES_CAS_PASS")
		t.Run("CASInstance", func(t *testing.T) {
			testPostgres(t, binPath, casConnName, dbName, user, casPass, false)
		})
	}
}

func testSQLServer(t *testing.T, binPath, connName, dbName, user, pass string) {
	modes := []struct {
		name string
		args []string
	}{
		{name: "translated_v2", args: []string{}},
		{name: "legacy_v1", args: []string{"-legacy-v1-proxy"}},
	}

	for _, mode := range modes {
		t.Run(mode.name, func(t *testing.T) {
			// TCP only
			t.Run("TCP", func(t *testing.T) {
				port := getFreePort(t)
				args := append([]string{}, mode.args...)
				args = append(args, fmt.Sprintf("-instances=%s=tcp:%d", connName, port))

				cmd, _ := startProxy(t, binPath, args...)
				defer cmd.Process.Kill()

				dsn := fmt.Sprintf("sqlserver://%s:%s@127.0.0.1:%d?database=%s", user, pass, port, dbName)
				runSQLServerQuery(t, dsn)
			})
		})
	}
}

func TestE2ESQLServer(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E tests")
	}
	connName := os.Getenv("SQLSERVER_CONNECTION_NAME")
	if connName == "" {
		t.Skip("SQLSERVER_CONNECTION_NAME not set")
	}
	dbName := os.Getenv("SQLSERVER_DB")
	user := os.Getenv("SQLSERVER_USER")
	pass := os.Getenv("SQLSERVER_PASS")

	binPath := buildProxy(t)
	defer os.RemoveAll(filepath.Dir(binPath))

	testSQLServer(t, binPath, connName, dbName, user, pass)
}
