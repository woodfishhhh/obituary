//go:build darwin && desktop

package tray

/*
#cgo darwin CFLAGS: -x objective-c -fobjc-arc
#cgo darwin LDFLAGS: -framework Foundation -framework UserNotifications
#import <UserNotifications/UserNotifications.h>
#import <os/log.h>
#include <stdlib.h>

static void obitNotify(const char *title, const char *text) {
    if (@available(macOS 10.14, *)) {
        NSString *titleString = [NSString stringWithUTF8String:title];
        NSString *textString = [NSString stringWithUTF8String:text];
        UNMutableNotificationContent *content = [UNMutableNotificationContent new];
        content.title = titleString;
        content.body = textString;
        UNTimeIntervalNotificationTrigger *trigger = [UNTimeIntervalNotificationTrigger triggerWithTimeInterval:0.1 repeats:NO];
        NSString *identifier = [[NSUUID UUID] UUIDString];
        UNNotificationRequest *request = [UNNotificationRequest requestWithIdentifier:identifier content:content trigger:trigger];
        UNUserNotificationCenter *center = [UNUserNotificationCenter currentNotificationCenter];
        [center requestAuthorizationWithOptions:(UNAuthorizationOptionAlert | UNAuthorizationOptionSound) completionHandler:^(BOOL granted, NSError *error) {
            if (error) {
                os_log_error(OS_LOG_DEFAULT, "obituary notification authorization: %{public}@ %ld %{public}@", error.domain, (long)error.code, error.localizedDescription);
            } else if (granted) {
                [center addNotificationRequest:request withCompletionHandler:^(NSError *err) {
                    if (err) os_log_error(OS_LOG_DEFAULT, "obituary notification delivery: %{public}@ %ld %{public}@", err.domain, (long)err.code, err.localizedDescription);
                }];
            }
        }];
    }
}
*/
import "C"

import "unsafe"

func showBalloon(title, text string) error {
	titlePtr := C.CString(title)
	defer C.free(unsafe.Pointer(titlePtr))
	textPtr := C.CString(text)
	defer C.free(unsafe.Pointer(textPtr))
	C.obitNotify(titlePtr, textPtr)
	return nil
}
