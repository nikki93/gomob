## ios

in this directory,

```
go run github.com/nikki93/gomob -target ios -o ios/TestApp.framework .
```

then `open ios/gomob-test-app-.xcodeproj` and build and run. (objective-)c(++)
code can be added either in the go package itself using cgo (like with 'ios.m')
or added in xcode (like with 'AppController.[hm]' and 'ViewController.[hm]'
etc.). project settings can be edited in xcode.
