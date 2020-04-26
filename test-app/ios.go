// +build darwin,ios

package main

import "fmt"

/*
#cgo CFLAGS: -DGLES_SILENCE_DEPRECATION -Werror -Wno-deprecated-declarations -fmodules -fobjc-arc -x objective-c

#include "ios.h"

*/
import "C"

//export test_app_two
func test_app_two() {
	fmt.Println("ios.go: two")
}
