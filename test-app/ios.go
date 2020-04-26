// +build darwin,ios

package main

import "fmt"

/*
#cgo CFLAGS: -DGLES_SILENCE_DEPRECATION -Werror -Wno-deprecated-declarations -fmodules -fobjc-arc -x objective-c

#include "ios.h"

*/
import "C"

//export test_app_go
func test_app_go() {
	fmt.Println("ios.go: test_app_go")
}
