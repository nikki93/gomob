// +build darwin,ios

#include <stdio.h>

#include "ios.h"
#include "_cgo_export.h"

void test_app_one() {
  printf("ios.m: one\n");
  test_app_two();
}

