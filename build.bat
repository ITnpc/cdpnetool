@echo off
chcp 65001 >nul
setlocal
echo ========================================
echo   cdpnetool Release Build Script
echo ========================================

REM 1. 环境检查
where wails >nul 2>nul
if %errorlevel% neq 0 (
    echo ERROR: 未找到 Wails CLI。请先安装:
    echo go install github.com/wailsapp/wails/v2/cmd/wails@latest
    exit /b 1
)

REM 2. 执行构建
echo INFO: 正在清理旧构建并生成安装包 (Windows AMD64)...
echo INFO: 这将包含前端构建和 Go 编译，请稍候...

REM 使用 -nsis 参数生成安装程序 (Setup.exe)
REM 使用 -clean 参数确保重新构建前端
wails build -platform windows/amd64 -clean -nsis

if %errorlevel% neq 0 (
    echo.
    echo ERROR: 构建失败！请检查上述错误日志。
    exit /b %errorlevel%
)

echo.
echo ========================================
echo SUCCESS: 构建成功！
echo ========================================
pause
