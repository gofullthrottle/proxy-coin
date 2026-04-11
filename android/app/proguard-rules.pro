# ProxyCoin ProGuard / R8 rules
# Applied in release builds (minifyEnabled = true, shrinkResources = true)

# ── Kotlin ────────────────────────────────────────────────────────────────────

-keepattributes Signature
-keepattributes *Annotation*
-keepattributes EnclosingMethod
-keepattributes InnerClasses
-keepattributes SourceFile,LineNumberTable

# Keep Kotlin data classes (needed for reflection-based serialisation).
-keepclassmembers class * extends java.io.Serializable {
    static final long serialVersionUID;
    private static final java.io.ObjectStreamField[] serialPersistentFields;
    private void writeObject(java.io.ObjectOutputStream);
    private void readObject(java.io.ObjectInputStream);
    java.lang.Object writeReplace();
    java.lang.Object readResolve();
}

# Kotlin Coroutines internal classes.
-keepclassmembernames class kotlinx.** {
    volatile <fields>;
}
-dontwarn kotlinx.coroutines.**

# ── Hilt / Dagger ─────────────────────────────────────────────────────────────

-keep class dagger.hilt.** { *; }
-keep class javax.inject.** { *; }
-keep class dagger.** { *; }
-keep @dagger.hilt.android.lifecycle.HiltViewModel class * { *; }

# Generated Hilt components and modules (names contain $$ separator).
-keep class **_HiltModules { *; }
-keep class **_HiltComponents { *; }
-keep class **_Factory { *; }
-keep class **_MembersInjector { *; }

-dontwarn dagger.hilt.**

# ── Retrofit + OkHttp ─────────────────────────────────────────────────────────

-keep class retrofit2.** { *; }
-keepattributes Exceptions
-dontwarn retrofit2.**

-keep class okhttp3.** { *; }
-keep interface okhttp3.** { *; }
-keep class okio.** { *; }
-dontwarn okhttp3.**
-dontwarn okio.**

# OkHttp's internal platform detection.
-dontwarn org.conscrypt.**
-dontwarn org.bouncycastle.**
-dontwarn org.openjsse.**

# ── Moshi ─────────────────────────────────────────────────────────────────────

-keepclassmembers class * {
    @com.squareup.moshi.FromJson <methods>;
    @com.squareup.moshi.ToJson <methods>;
}

# Keep Moshi-generated adapters (JsonClass(generateAdapter = true)).
-keep @com.squareup.moshi.JsonClass class * { *; }
-keepclassmembers @com.squareup.moshi.JsonClass class * {
    <fields>;
    <init>(...);
}
-keep class **JsonAdapter { *; }
-dontwarn com.squareup.moshi.**

# ── Room ──────────────────────────────────────────────────────────────────────

# Keep all Room entity data classes.
-keep class com.proxycoin.app.data.local.entity.** { *; }

# Keep Room DAOs (Room generates implementations via KSP).
-keep @androidx.room.Dao class * { *; }
-keep @androidx.room.Entity class * { *; }
-keep @androidx.room.Database class * { *; }

-dontwarn androidx.room.**

# ── Data Transfer Objects (Moshi serialisation) ───────────────────────────────

-keep class com.proxycoin.app.data.remote.dto.** { *; }

# ── Web3j ─────────────────────────────────────────────────────────────────────

-keep class org.web3j.** { *; }
-keep interface org.web3j.** { *; }
-keep class org.bouncycastle.** { *; }
-keep interface org.bouncycastle.** { *; }

# Web3j uses reflection to instantiate ABI types.
-keepclassmembers class org.web3j.abi.datatypes.** { *; }

-dontwarn org.web3j.**
-dontwarn org.bouncycastle.**
-dontwarn org.spongycastle.**

# ── Protobuf ──────────────────────────────────────────────────────────────────

-keep class com.google.protobuf.** { *; }
-keep class * extends com.google.protobuf.GeneratedMessageLite { *; }
-dontwarn com.google.protobuf.**

# ── DataStore (Preferences) ───────────────────────────────────────────────────

-keep class androidx.datastore.** { *; }
-dontwarn androidx.datastore.**

# ── WorkManager ───────────────────────────────────────────────────────────────

-keep class androidx.work.** { *; }
-keep class * extends androidx.work.Worker { *; }
-keep class * extends androidx.work.CoroutineWorker { *; }
-keep class * extends androidx.work.ListenableWorker {
    public <init>(android.content.Context, androidx.work.WorkerParameters);
}
-dontwarn androidx.work.**

# ── Play Integrity ────────────────────────────────────────────────────────────

-keep class com.google.android.play.core.integrity.** { *; }
-dontwarn com.google.android.play.core.**

# ── Compose ───────────────────────────────────────────────────────────────────

# Compose relies on reflection for Composable function lookup.
-keep @androidx.compose.runtime.Composable class * { *; }
-keepclassmembers class * {
    @androidx.compose.runtime.Composable <methods>;
}

-dontwarn androidx.compose.**

# ── Hilt Navigation Compose ───────────────────────────────────────────────────

-keep class androidx.hilt.navigation.compose.** { *; }
-dontwarn androidx.hilt.navigation.compose.**

# ── Lifecycle ─────────────────────────────────────────────────────────────────

-keep class androidx.lifecycle.** { *; }
-keep class * extends androidx.lifecycle.ViewModel { *; }
-keepclassmembers class * extends androidx.lifecycle.ViewModel {
    <init>(...);
}
-dontwarn androidx.lifecycle.**

# ── Strip debug logs in release ───────────────────────────────────────────────
# R8 / ProGuard can inline-remove Log.d and Log.v calls.

-assumenosideeffects class android.util.Log {
    public static boolean isLoggable(java.lang.String, int);
    public static int d(...);
    public static int v(...);
}

# ── Miscellaneous ─────────────────────────────────────────────────────────────

# Prevent R8 from removing enum constructors (needed by Moshi adapter generation).
-keepclassmembers enum * {
    public static **[] values();
    public static ** valueOf(java.lang.String);
}

# Preserve Parcelable implementations (used by Android IPC).
-keep class * implements android.os.Parcelable {
    public static final android.os.Parcelable$Creator *;
}

# Prevent stripping of custom exception subclasses (stack traces in crash reports).
-keep public class * extends java.lang.Exception

-dontwarn java.lang.instrument.**
-dontwarn sun.misc.**
-dontwarn java.awt.**
