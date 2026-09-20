//go:build android

package ui

/*
#include <jni.h>
#include <stdint.h>
#include <stdlib.h>

// startViewIntent fires ACTION_VIEW for url from the application context.
// Returns 0 on success.
static int startViewIntent(uintptr_t javaVM, uintptr_t appCtx, const char *url) {
	JavaVM *vm = (JavaVM *)javaVM;
	JNIEnv *env = NULL;
	int attached = 0;
	int rc = -1;

	if ((*vm)->GetEnv(vm, (void **)&env, JNI_VERSION_1_6) == JNI_EDETACHED) {
		if ((*vm)->AttachCurrentThread(vm, &env, NULL) != 0) {
			return -1;
		}
		attached = 1;
	}

	jobject ctx = (jobject)appCtx;
	jclass uriCls = (*env)->FindClass(env, "android/net/Uri");
	jclass intentCls = (*env)->FindClass(env, "android/content/Intent");
	if (uriCls == NULL || intentCls == NULL) goto done;

	jmethodID parse = (*env)->GetStaticMethodID(env, uriCls, "parse", "(Ljava/lang/String;)Landroid/net/Uri;");
	jmethodID ctor = (*env)->GetMethodID(env, intentCls, "<init>", "(Ljava/lang/String;Landroid/net/Uri;)V");
	jmethodID addFlags = (*env)->GetMethodID(env, intentCls, "addFlags", "(I)Landroid/content/Intent;");
	jclass ctxCls = (*env)->GetObjectClass(env, ctx);
	jmethodID start = (*env)->GetMethodID(env, ctxCls, "startActivity", "(Landroid/content/Intent;)V");
	if (parse == NULL || ctor == NULL || addFlags == NULL || start == NULL) goto done;

	jstring jurl = (*env)->NewStringUTF(env, url);
	jstring action = (*env)->NewStringUTF(env, "android.intent.action.VIEW");
	jobject uri = (*env)->CallStaticObjectMethod(env, uriCls, parse, jurl);
	jobject intent = (*env)->NewObject(env, intentCls, ctor, action, uri);
	// FLAG_ACTIVITY_NEW_TASK is required when starting from a non-Activity context.
	(*env)->CallObjectMethod(env, intent, addFlags, 0x10000000);
	(*env)->CallVoidMethod(env, ctx, start, intent);

	if ((*env)->ExceptionCheck(env)) {
		(*env)->ExceptionClear(env);
	} else {
		rc = 0;
	}

done:
	if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env);
	if (attached) (*vm)->DetachCurrentThread(vm);
	return rc;
}
*/
import "C"

import (
	"errors"
	"runtime"
	"unsafe"

	"gioui.org/app"
)

// openURLAndroid opens rawURL in the user's browser with a real
// ACTION_VIEW intent (the old `am start` shell-out isn't allowed for app
// processes on modern Android).
func openURLAndroid(rawURL string) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	curl := C.CString(rawURL)
	defer C.free(unsafe.Pointer(curl))
	if C.startViewIntent(C.uintptr_t(app.JavaVM()), C.uintptr_t(app.AppContext()), curl) != 0 {
		return errors.New("could not start browser intent")
	}
	return nil
}
