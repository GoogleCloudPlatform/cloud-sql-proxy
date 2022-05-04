go version
where go
where bash
setx GOPATH "C:\Program Files\Go"
call RefreshEnv.cmd
"C:\cygwin64\bin\bash.exe" github/cloud-sql-proxy/.kokoro/tests/run_tests_windows.sh
