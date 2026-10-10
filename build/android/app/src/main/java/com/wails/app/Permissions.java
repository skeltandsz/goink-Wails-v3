package com.wails.app;

import android.Manifest;
import android.app.Activity;
import android.content.Context;
import android.content.Intent;
import android.content.UriPermission;
import android.content.pm.PackageManager;
import android.net.Uri;
import android.os.Build;
import android.os.PowerManager;
import android.provider.Settings;
import android.util.Log;

import androidx.core.app.ActivityCompat;
import androidx.core.content.ContextCompat;

import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;

/**
 * Single gate for the runtime permissions the mobile capabilities need.
 *
 * Why this exists instead of another inline checkSelfPermission(): the scaffold
 * used to request POST_NOTIFICATIONS in two places with codes 1001/1003 and no
 * result handling at all, so a dismissed dialog silently dropped the
 * notification and nobody could tell "not asked yet" from "user said no
 * forever". Every permission in here therefore has (a) an API-level aware
 * mapping, (b) a stable request code, (c) permanent-denial detection so the UI
 * can point at Settings instead of re-asking in a loop, and (d) one result event
 * name for the frontend to listen to.
 *
 * Request-code layout, to stay collision free with the codes owned elsewhere:
 *   7001-7010  MainActivity (file picker, photo/video capture, CAMERA prompt)
 *   1002       WailsBridge's location prompt, still inline (it has its own
 *              retry flow through emitLocation, so it was left alone)
 *   7100+      this class, one fixed slot per capability (see REQUEST_CODES)
 */
final class Permissions {
    private static final String TAG = "WailsPermissions";
    private static final boolean DEBUG = BuildConfig.DEBUG;

    // Capability keys are the stable contract with the frontend; never rename
    // them to a permission name, because one capability can map to several
    // permissions and that mapping changes with the API level.
    static final String CAP_MEDIA = "media";
    static final String CAP_STORAGE_WRITE = "storageWrite";
    static final String CAP_CALENDAR = "calendar";
    static final String CAP_NOTIFICATIONS = "notifications";

    /** Event name the frontend listens to: {"capability","granted","permanentlyDenied","reason"} */
    static final String EVENT = "wails:permission";

    private static final int CODE_BASE = 7100;
    private static final Map<String, Integer> REQUEST_CODES = new HashMap<>();
    static {
        REQUEST_CODES.put(CAP_MEDIA, CODE_BASE + 1);
        REQUEST_CODES.put(CAP_STORAGE_WRITE, CODE_BASE + 2);
        REQUEST_CODES.put(CAP_CALENDAR, CODE_BASE + 3);
        REQUEST_CODES.put(CAP_NOTIFICATIONS, CODE_BASE + 4);
    }

    /** @param reason "scoped-storage" when the platform has no grantable permission for it. */
    interface Callback {
        void onResult(String capability, boolean granted, boolean permanentlyDenied, String reason);
    }

    private final Activity activity;
    private final WailsBridge bridge;
    // Only one in-flight request per capability: a second ensure() for the same
    // capability while the dialog is open just replaces the callback, because the
    // system delivers one result per request code.
    private final Map<String, Callback> pending = new HashMap<>();

    Permissions(Activity activity, WailsBridge bridge) {
        this.activity = activity;
        this.bridge = bridge;
    }

    /**
     * Permissions that a grant prompt can actually produce on this API level.
     * Empty means the capability needs no runtime permission here (either it is
     * auto-granted or the platform has no equivalent).
     */
    static String[] permissionsFor(String capability) {
        switch (capability) {
            case CAP_MEDIA:
                if (Build.VERSION.SDK_INT >= 33) {
                    // Images only: the manifest deliberately does not declare
                    // READ_MEDIA_VIDEO/_AUDIO (Play wants "declared == used"), so
                    // adding a new media type means touching both places at once.
                    return new String[]{Manifest.permission.READ_MEDIA_IMAGES};
                }
                return new String[]{Manifest.permission.READ_EXTERNAL_STORAGE};
            case CAP_STORAGE_WRITE:
                // From API 29 shared-storage writes go through SAF/MediaStore; there is
                // no permission to ask for, so callers must not treat this as denied.
                if (Build.VERSION.SDK_INT >= 29) {
                    return new String[0];
                }
                return new String[]{
                        Manifest.permission.WRITE_EXTERNAL_STORAGE,
                        Manifest.permission.READ_EXTERNAL_STORAGE};
            case CAP_CALENDAR:
                return new String[]{
                        Manifest.permission.READ_CALENDAR,
                        Manifest.permission.WRITE_CALENDAR};
            case CAP_NOTIFICATIONS:
                if (Build.VERSION.SDK_INT >= 33) {
                    return new String[]{Manifest.permission.POST_NOTIFICATIONS};
                }
                return new String[0];
            default:
                return null;
        }
    }

    static boolean isGranted(Context context, String capability) {
        String[] perms = permissionsFor(capability);
        if (perms == null) {
            // Unknown capability: fail closed so a typo cannot silently "grant".
            Log.w(TAG, "isGranted: unknown capability " + capability);
            return false;
        }
        if (perms.length == 0 && CAP_STORAGE_WRITE.equals(capability)) {
            // On API 29+ "may write shared files" is not a permission at all: it is a
            // persisted SAF tree grant. Answer with the real state so callers do not
            // send the user to a picker they already completed.
            return hasPersistedTreeAccess(context);
        }
        for (String p : perms) {
            if (ContextCompat.checkSelfPermission(context, p)
                    != PackageManager.PERMISSION_GRANTED) {
                return false;
            }
        }
        return true;
    }

    /**
     * Run {@code cb} with whether the capability is usable. Asks the user only if
     * the platform can still grant it; never re-asks after a permanent denial (the
     * caller gets permanentlyDenied=true and should route the user to Settings).
     */
    void ensure(String capability, Callback cb) {
        String[] perms = permissionsFor(capability);
        if (perms == null) {
            deliver(capability, false, false, "unknown-capability", cb);
            return;
        }
        if (perms.length == 0) {
            // Nothing the dialog could grant. Let isGranted decide usability: for
            // notifications below API 33 that is trivially true, for storageWrite on
            // API 29+ it means "a SAF tree is already persisted" — and the caller
            // then opens the picker instead of showing a "denied" toast.
            boolean usable = isGranted(activity, capability);
            deliver(capability, usable, false,
                    usable ? null : "scoped-storage", cb);
            return;
        }
        if (isGranted(activity, capability)) {
            deliver(capability, true, false, null, cb);
            return;
        }
        Integer code = REQUEST_CODES.get(capability);
        if (code == null) {
            deliver(capability, false, false, "no-request-code", cb);
            return;
        }
        pending.put(capability, cb);
        ActivityCompat.requestPermissions(activity, perms, code);
    }

    /**
     * Route a system permission result back to the waiting capability.
     *
     * @return true when this class owned {@code requestCode}; the activity must
     *         then skip its own handling.
     */
    boolean handleResult(int requestCode, String[] permissions, int[] grantResults) {
        String capability = capabilityForCode(requestCode);
        if (capability == null) {
            return false;
        }
        Callback cb = pending.remove(capability);
        boolean granted = isGranted(activity, capability);
        // "Permanently denied" == denied without the system offering to ask again.
        // Only meaningful when the user actually answered, hence the !granted guard:
        // after a grant, shouldShowRequestPermissionRationale() is false as well.
        boolean permanent = !granted && !shouldShowRationale(permissions);
        deliver(capability, granted, permanent,
                granted ? null : (permanent ? "denied-permanently" : "denied"), cb);
        return true;
    }

    private boolean shouldShowRationale(String[] permissions) {
        if (permissions == null) {
            return false;
        }
        for (String p : permissions) {
            if (ActivityCompat.shouldShowRequestPermissionRationale(activity, p)) {
                return true;
            }
        }
        return false;
    }

    private static String capabilityForCode(int requestCode) {
        for (Map.Entry<String, Integer> e : REQUEST_CODES.entrySet()) {
            if (e.getValue() == requestCode) {
                return e.getKey();
            }
        }
        return null;
    }

    private void deliver(String capability, boolean granted, boolean permanent,
                         String reason, Callback cb) {
        if (DEBUG) {
            Log.d(TAG, "ensure(" + capability + ") -> granted=" + granted
                    + " permanent=" + permanent + " reason=" + reason);
        }
        StringBuilder json = new StringBuilder(96);
        json.append("{\"capability\":\"").append(capability)
                .append("\",\"granted\":").append(granted)
                .append(",\"permanentlyDenied\":").append(permanent);
        if (reason != null) {
            json.append(",\"reason\":\"").append(reason).append("\"");
        }
        json.append("}");
        // One event for every resolution so the frontend can react to a grant that
        // arrived while it was not even looking (dialog finished, callback gone).
        bridge.emitEvent(EVENT, json.toString());
        if (cb != null) {
            cb.onResult(capability, granted, permanent, reason);
        }
    }

    // --------------------------------------------------- gates without a dialog
    // Battery exemption and exact-alarm scheduling are NOT runtime permissions:
    // the user can only change them on a Settings screen, so they get their own
    // "ask the platform, then hand the user over" helpers instead of pretending
    // to be part of requestPermissions().

    /**
     * Flags for the SAF tree pick (ACTION_OPEN_DOCUMENT_TREE) that backs file access
     * on API 29+: READ/WRITE are the runtime grant for the picked subtree, PERSISTABLE
     * lets it survive the process, PREFIX lets child documents be read without
     * re-picking. Without PERSISTABLE the user is asked for the same folder on
     * every app start.
     */
    static final int SAF_TREE_FLAGS = Intent.FLAG_GRANT_READ_URI_PERMISSION
            | Intent.FLAG_GRANT_WRITE_URI_PERMISSION
            | Intent.FLAG_GRANT_PERSISTABLE_URI_PERMISSION
            | Intent.FLAG_GRANT_PREFIX_URI_PERMISSION;

    /**
     * Turn a freshly picked tree URI into a lasting grant.
     *
     * @param wantWrite false for read-only access (chapter browsing without autosave)
     * @return false if the system refused to persist it; caller should then keep the
     *         URI only for the current process.
     */
    static boolean persistTreeGrant(Activity activity, Uri treeUri, boolean wantWrite) {
        if (treeUri == null) {
            return false;
        }
        try {
            int flags = wantWrite
                    ? (Intent.FLAG_GRANT_READ_URI_PERMISSION | Intent.FLAG_GRANT_WRITE_URI_PERMISSION)
                    : Intent.FLAG_GRANT_READ_URI_PERMISSION;
            activity.getContentResolver().takePersistableUriPermission(treeUri, flags);
            return true;
        } catch (SecurityException e) {
            // The URI was not picked with FLAG_GRANT_PERSISTABLE_URI_PERMISSION.
            Log.w(TAG, "cannot persist tree grant for " + treeUri, e);
            return false;
        }
    }

    /**
     * Whether the app still holds a persisted SAF tree grant. On 29+ this — not a
     * permission — is what "storage access" means, so capabilities should check it
     * before sending the user to a picker.
     */
    static boolean hasPersistedTreeAccess(Context context) {
        return !persistedTreeUris(context, false).isEmpty();
    }

    /** Persisted SAF tree URIs; {@code writableOnly} filters by the write grant. */
    static List<Uri> persistedTreeUris(Context context, boolean writableOnly) {
        List<Uri> out = new ArrayList<>();
        for (UriPermission p : context.getContentResolver().getPersistedUriPermissions()) {
            if (!p.isReadPermission()) {
                continue;
            }
            if (writableOnly && !p.isWritePermission()) {
                continue;
            }
            out.add(p.getUri());
        }
        return out;
    }

    boolean isBatteryExempt() {
        if (Build.VERSION.SDK_INT < 23) {
            return true;
        }
        PowerManager pm = (PowerManager) activity.getSystemService(Context.POWER_SERVICE);
        return pm != null && pm.isIgnoringBatteryOptimizations(activity.getPackageName());
    }

    /**
     * Open the battery-optimisation prompt for this app. Returns false when the
     * device cannot show it (no such settings screen), so the caller can fall back
     * to pointing at Settings#ACTION_APPLICATION_DETAILS_SETTINGS.
     */
    boolean requestBatteryExemption() {
        if (Build.VERSION.SDK_INT < 23 || isBatteryExempt()) {
            return true;
        }
        try {
            Intent i = new Intent(Settings.ACTION_REQUEST_IGNORE_BATTERY_OPTIMIZATIONS);
            i.setData(Uri.parse("package:" + activity.getPackageName()));
            activity.startActivity(i);
            return true;
        } catch (Exception e) {
            Log.w(TAG, "battery exemption prompt unavailable", e);
            return false;
        }
    }

    /** API 31+ made exact alarms revocable, so state must be read, not assumed. */
    boolean canScheduleExactAlarms() {
        if (Build.VERSION.SDK_INT < 31) {
            return true;
        }
        android.app.AlarmManager am =
                (android.app.AlarmManager) activity.getSystemService(Context.ALARM_SERVICE);
        return am == null || am.canScheduleExactAlarms();
    }

    /** Open the system's exact-alarm toggle for this app (API 31+ only). */
    boolean requestExactAlarmAccess() {
        if (Build.VERSION.SDK_INT < 31) {
            return true;
        }
        if (canScheduleExactAlarms()) {
            return true;
        }
        try {
            Intent i = new Intent(Settings.ACTION_REQUEST_SCHEDULE_EXACT_ALARM);
            i.setData(Uri.parse("package:" + activity.getPackageName()));
            activity.startActivity(i);
            return true;
        } catch (Exception e) {
            Log.w(TAG, "exact-alarm prompt unavailable", e);
            return false;
        }
    }

    /** Last resort for permanently denied capabilities: the app's Settings page. */
    void openAppSettings() {
        try {
            Intent i = new Intent(Settings.ACTION_APPLICATION_DETAILS_SETTINGS);
            i.setData(Uri.parse("package:" + activity.getPackageName()));
            activity.startActivity(i);
        } catch (Exception e) {
            Log.w(TAG, "cannot open app settings", e);
        }
    }
}
