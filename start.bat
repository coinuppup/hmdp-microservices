name@echo off
chcp 65001 >nul
echo === 启动 HMDP 微服务 ===

echo [1/3] 启动Docker中间件...
docker-compose up -d
timeout /t 5 /nobreak >nul

echo.
echo [2/3] 中间件状态:
docker ps --format "table {{.Names}}\t{{.Status}}" | findstr hmdp-

echo.
echo [3/3] 启动Go服务...

start "User-Service" cmd /k "cd /d %~dp0user-service && go run main.go"
start "Shop-Service" cmd /k "cd /d %~dp0shop-service && go run main.go"
start "Content-Service" cmd /k "cd /d %~dp0content-service && go run main.go"

echo.
echo === 全部服务启动完成 ===
echo.
echo 服务地址:
echo   - 用户服务:   http://localhost:8081
echo   - 商铺服务:   http://localhost:8082
echo   - 内容服务:   http://localhost:8083
echo.
echo 按任意键打开浏览器访问用户服务...
pause >nul
start http://localhost:8081