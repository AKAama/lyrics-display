//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa
#include <stdlib.h>
#import <Cocoa/Cocoa.h>

void LockStatusItemWidthForText(const char *utf8) {
  void (^apply)(void) = ^{
    if (utf8 == NULL) {
      return;
    }
    NSString *text = [NSString stringWithUTF8String:utf8];
    if (text == nil) {
      return;
    }
    NSFont *font = [NSFont menuBarFontOfSize:0];
    if (font == nil) {
      font = [NSFont systemFontOfSize:13.0];
    }
    CGFloat width = ceil([text sizeWithAttributes:@{NSFontAttributeName: font}].width) + 12.0;
    if (width < 24.0) {
      width = 24.0;
    }
    NSArray *items = nil;
    @try {
      items = [[NSStatusBar systemStatusBar] valueForKey:@"items"];
    } @catch (NSException *ex) {
      return;
    }
    for (NSStatusItem *item in items) {
      item.length = width;
      item.button.lineBreakMode = NSLineBreakByClipping;
      item.button.alignment = NSTextAlignmentLeft;
    }
  };

  if ([NSThread isMainThread]) {
    apply();
  } else {
    dispatch_sync(dispatch_get_main_queue(), apply);
  }
}
*/
import "C"

import (
	"strings"
	"unsafe"
)

func lockStatusItemToSlot(prefix string, slotWidth int) {
	slotWidth = normalizeSlotCells(slotWidth)
	canonical := prefix + strings.Repeat(string(menuBarPadFull), slotWidth)
	cstr := C.CString(canonical)
	defer C.free(unsafe.Pointer(cstr))
	C.LockStatusItemWidthForText(cstr)
}
