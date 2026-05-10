module github.com/nixomose/zfstool

go 1.22

require golang.org/x/crypto v0.31.0

require github.com/webview/webview_go v0.0.0-20240831120633-6173450d4dd6

require golang.org/x/sys v0.28.0 // indirect

replace github.com/webview/webview_go => ./third_party/webview_go
