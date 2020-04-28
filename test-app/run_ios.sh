#!/bin/sh

set -e

go run github.com/nikki93/gomob -x -target ios -o ios/TestApp.framework .

pushd ios
xcodebuild | xcpretty
popd

ios-deploy -L -b ios/build/Release-iphoneos/*.app

