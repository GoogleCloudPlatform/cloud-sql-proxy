setx GOPATH "C:\Go"
call RefreshEnv.cmd
"bash.exe" github/cloud-sql-proxy/.kokoro/tests/run_tests_windows.sh
