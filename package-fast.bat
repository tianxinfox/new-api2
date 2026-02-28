@echo off
setlocal EnableExtensions EnableDelayedExpansion

set "IMAGE_NAME=new-api"
set "TAG=local"
if not "%~1"=="" set "TAG=%~1"
set "IMAGE=%IMAGE_NAME%:%TAG%"
set "TAR_NAME=%IMAGE_NAME%-%TAG%.tar"

echo [1/4] Building image: %IMAGE%
echo [MODE] fast \(use docker cache\)
docker build -t %IMAGE% .
if errorlevel 1 (
  echo [ERROR] Docker build failed.
  exit /b 1
)

echo [2/4] Cleaning old tar if exists: %TAR_NAME%
if exist %TAR_NAME% del /f /q %TAR_NAME%

echo [3/4] Saving image to tar: %TAR_NAME%
docker save -o %TAR_NAME% %IMAGE%
if errorlevel 1 (
  echo [ERROR] Docker save failed.
  exit /b 1
)

if /I "%TAG%"=="local" (
  copy /Y %TAR_NAME% new-api-local.tar >nul
)

echo [4/4] Calculating SHA256 for %TAR_NAME%
certutil -hashfile %TAR_NAME% SHA256

echo.
echo [DONE] Package ready: %TAR_NAME%
echo Deploy on server with:
echo   docker load -i %TAR_NAME%
echo   docker compose down
echo   docker compose up -d --force-recreate

endlocal
