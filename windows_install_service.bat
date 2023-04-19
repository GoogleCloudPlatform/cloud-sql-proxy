@echo off

setlocal

set SERVICE=cloud-sql-proxy
set DISPLAYNAME=Google Cloud SQL Auth Proxy
set CREDENTIALSFILE=%~dp0key.json
set CONNECTIONNAME=project-id:region:db-instance

sc.exe create "%SERVICE%" binPath= "\"%~dp0cloud-sql-proxy.exe\" --credentials-file \"%CREDENTIALSFILE%\" %CONNECTIONNAME%" obj= "NT AUTHORITY\Network Service" start= auto displayName= "%DISPLAYNAME%"
sc.exe failure "%SERVICE%" reset= 0 actions= restart/0/restart/0/restart/0
net start "%SERVICE%"

endlocal
