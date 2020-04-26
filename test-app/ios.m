// +build darwin,ios

#include <stdio.h>

#include "ios.h"
#include "_cgo_export.h"

void test_app_objc() {
  printf("ios.m: test_app_objc\n");
  test_app_go();
}

