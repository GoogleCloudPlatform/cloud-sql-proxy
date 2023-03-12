@echo off

setlocal

set SERVICE=cloud-sql-proxy

net stop "%SERVICE%"
sc.exe delete "%SERVICE%"

endlocal
