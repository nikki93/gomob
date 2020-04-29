## ios

just `open ios/gomob-test-app.xcodeproj` and run. the xcode project has a 'run
script' build phase that builds the go code. (objective-)c(++) code can be
added either in the go package itself using cgo (like with 'ios.m') or added in
xcode (like with 'AppController.[hm]' and 'ViewController.[hm]' etc.). project
settings can be edited in xcode.
