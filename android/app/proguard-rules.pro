# Pilotty ProGuard / R8 规则
# 参考: docs/play-submission-checklist.md §5 + ~/References/nekobox/app/proguard-rules.pro
# 策略: -dontobfuscate 保留符号名防 libbox 反射踩坑, 靠 shrink + inline 瘦身

# === gomobile 桥接 (libbox + mobile 包) ===
-keep class go.** { *; }
-keep class mobile.** { *; }
-keep class libbox.** { *; }
-keepclassmembers class libbox.** { *; }

# === kotlinx.serialization ===
-keepattributes *Annotation*, InnerClasses
-dontnote kotlinx.serialization.AnnotationsKt
-keep,includedescriptorclasses class com.pilotty.app.data.**$$serializer { *; }
-keepclassmembers class com.pilotty.app.data.** {
    *** Companion;
}
-keepclasseswithmembers class com.pilotty.app.data.** {
    kotlinx.serialization.KSerializer serializer(...);
}

# === 保留行号便于 crash 排查 ===
-keepattributes SourceFile,LineNumberTable
-renamesourcefileattribute SourceFile

# === 不混淆 (同 NekoBox, 避免 libbox JNI 反射踩坑) ===
-dontobfuscate

# === Compose runtime / Material3 ===
-dontwarn androidx.compose.**

# === sing-box / Go runtime 常见误报 ===
-dontwarn sun.misc.**
-dontwarn java.beans.**

# === Clean Kotlin Intrinsics (照抄 NekoBox) ===
-assumenosideeffects class kotlin.jvm.internal.Intrinsics {
    static void checkParameterIsNotNull(java.lang.Object, java.lang.String);
    static void checkExpressionValueIsNotNull(java.lang.Object, java.lang.String);
    static void checkNotNullExpressionValue(java.lang.Object, java.lang.String);
    static void checkReturnedValueIsNotNull(java.lang.Object, java.lang.String, java.lang.String);
    static void checkReturnedValueIsNotNull(java.lang.Object, java.lang.String);
    static void checkFieldIsNotNull(java.lang.Object, java.lang.String, java.lang.String);
    static void checkFieldIsNotNull(java.lang.Object, java.lang.String);
    static void checkNotNull(java.lang.Object);
    static void checkNotNull(java.lang.Object, java.lang.String);
    static void checkNotNullParameter(java.lang.Object, java.lang.String);
    static void throwUninitializedPropertyAccessException(java.lang.String);
}
