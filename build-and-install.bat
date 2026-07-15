@echo off
REM OpenLink one-click build & install (launches build.ps1)
REM Double-click this file to run. Pass "full" as argument to also rebuild the Chrome extension.
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0build.ps1" %*
