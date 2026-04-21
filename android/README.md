# Pilotty Android

Jetpack Compose + Material3 客户端。底部 5 tab(首页 / 对话 / 节点 / 规则 / 设置),通过 gomobile 绑定调用 `mobile/netpilot.go` 提供的 Go 控制面。

## 首次构建

```bash
# 1. 生成 netpilot.aar(gomobile 产物,已被 .gitignore,必须本机构建)
#    注意:修改 mobile/*.go 后必须重跑本脚本,否则 Kotlin 侧会报 Unresolved reference
./scripts/build-aar.sh

# 2. 需要 JDK 17 —— 当前 Gradle 8.7 / AGP 8.5.2 不兼容 JDK 22+
export JAVA_HOME=/opt/homebrew/opt/openjdk@17/libexec/openjdk.jdk/Contents/Home
export PATH="$JAVA_HOME/bin:$PATH"

# 3. 构建 debug APK
cd android
./gradlew :app:assembleDebug
```

产出:`android/app/build/outputs/apk/debug/app-debug.apk`

## 依赖版本

- Kotlin 2.0.0 / AGP 8.5.2 / Gradle 8.7 / Compose BoM 2024.08.00
- minSdk 21 / compileSdk 36 / targetSdk 36
