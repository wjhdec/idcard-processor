.PHONY: build dev clean

# Windows 的 PowerShell/cmd 下，GNU Make（Windows32 版）启动时探测 Unix shell 失败
# （Scoop 的 sh shim 探测不过），且不认环境变量/Makefile 的 SHELL 赋值，recipe 会
# 被直接交给 CreateProcess 执行，导致 mkdir/cp 等 shell 命令报"系统找不到指定的文件"。
# 因此 Windows 分支用 cmd /c 包裹（cmd 模式与 sh 模式下均可执行），Unix 分支用 sh 语法。
ifeq ($(OS),Windows_NT)
MKDIR_P    := cmd /c if not exist build mkdir build
CP         := cmd /c copy /Y
RM_BUILD   := cmd /c if exist build rd /s /q build
RM_DIST    := cmd /c if exist frontend\dist rd /s /q frontend\dist
RM_MODULES := cmd /c if exist frontend\node_modules rd /s /q frontend\node_modules
else
MKDIR_P    := mkdir -p build
CP         := cp
RM_BUILD   := rm -rf build
RM_DIST    := rm -rf frontend/dist
RM_MODULES := rm -rf frontend/node_modules
endif

build: build/appicon.png
	wails build -tags webkit2_41

# 开发模式（热重载）
dev:
	wails dev -tags webkit2_41

build/appicon.png: appicon.png
	@$(MKDIR_P)
	$(CP) appicon.png $@

# 清理构建产物
clean:
	$(RM_BUILD)
	$(RM_DIST)
	$(RM_MODULES)
