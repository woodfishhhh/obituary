//go:build darwin && desktop

package tray

/*
#cgo darwin CFLAGS: -x objective-c -fobjc-arc
#cgo darwin LDFLAGS: -framework Foundation
#import <UserNotifications/UserNotifications.h>
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
            if (granted && error == nil) {
                [center addNotificationRequest:request withCompletionHandler:nil];
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
