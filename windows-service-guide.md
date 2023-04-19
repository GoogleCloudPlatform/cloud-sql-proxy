# Cloud SQL Auth Proxy Windows Service Guide

This document covers running the *Cloud SQL Auth Proxy* as service
on the Windows operating system.

It was originally built and tested using Go 1.20.2 on Windows Server 2019.

## Install the Windows Service

Prerequisites: A built binary for Windows of the Cloud SQL Auth Proxy is required. Either build it from source or [download a release](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/releases) of a Windows pre-built version, e.g. `cloud-sql-proxy.x64.exe`. 

First, install the binary by:

1. Create a new empty folder, e.g. `C:\Program Files\cloud-sql-proxy`
2. Copy the binary and helper batch files 
3. Modify the batch files as needed:
    - `SERVICE` is the Windows internal service name (as shown in the Task Manager)
    - `DISPLAYNAME` is the service name (as shown in the Windows Administration Console (MMC))
    - `CREDENTIALSFILE` is the *full* path to the credentials file, where `%~dp0` points to the full path of the script file folder.
    - `CONNECTIONNAME` is the Google SQL connection name in the format of `project-id:region:db-instance`
    - Please note that the `--credentials-file \"%CREDENTIALSFILE%\"` argument is optional and is not needed if the local machine runs within the Google Cloud Compute Engine and "defaults" to the VM instance service account.
4. Grant *read & execute* access to the `Network Service` user
5. Create a `logs` sub-folder, e.g. `C:\Program Files\cloud-sql-proxy\logs`
6. Grant *modify* access to the `Network Service` user
7. Run the `windows_install_service.bat` batch file within an *elevated* command line prompt (read: *Run as Administrator*).

After that, perform the setup:

1. Copy the JSON credentials file, if required
2. Modify the `windows_install_service.bat` file to your needs
3. Run the `windows_install_service.bat` file from the commandline

Please see the FAQ below for common error messages.

## Uninstall the Windows Service

To uninstall the Windows Service, perform the following steps:

1. Modify the `windows_remove_service.bat` file to your needs
2. Run the `windows_remove_service.bat` file from the commandline

## FAQ

### Error Message: *Access is denied*

The error message `Access is denied.` (or `System error 5 has occurred.`) occurs when
trying to start the installed service but the service account does not have access
to the service's file directory.

Usually this is the *Network Service* built-in user.

Please note that write access is also required for creating and managing the log files, e.g.:

- `cloud-sql-proxy.log`
- `cloud-sql-proxy-2016-11-04T18-30-00.000.log`

### Error Message: *The specified service has been marked for deletion.*

The error message `The specified service has been marked for deletion.` occurs when 
reinstalling the service and the previous deletion request could not be completed
(e.g. because the service was still running or opened in the service manager).

In this case, the local machine needs to be restarted.

### Why not running as the *System* user?

Since the Cloud Proxy does not require and file system access, besides the log files,
extensive operating system access is not required.

The *Network Service* accounts allow binding ports while not granting 
access to file system resources.

### Why not using *Automatic (Delayed Start)* startup type?

The service is installed in the *Automatic* startup type, by default.

The alternative *Automatic (Delayed Start)* startup type was introduced
by Microsoft for services that are not required for operating system operations
like Windows Update and similar services.

However, if the primary purpose of the local machine is to provide services
which require access to the cloud database, then the start of the service
should not be delayed. 

Delayed services might be started even minutes after operating system startup.
