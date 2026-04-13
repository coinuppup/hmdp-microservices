@echo off
echo ========================================
echo   黑马点评前端启动脚本
echo ========================================

cd /d "%~dp0"

echo.
echo [1/3] 检查并安装依赖...
if not exist "node_modules" (
    call npm install
) else (
    echo 依赖已安装，跳过
)

echo.
echo [2/3] 启动开发服务器...
echo 访问地址: http://localhost:5173
echo.

call npm run dev

pause