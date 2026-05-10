# third_party

## webview_go (fork)

Upstream: [webview/webview_go](https://github.com/webview/webview_go) (MIT).

The Linux `pkg-config` line is set to **`webkit2gtk-4.1`**, which matches **Ubuntu 24.04+**
and current Debian. On **Ubuntu 22.04 (Jammy)**, dev packages expose **`webkit2gtk-4.0`**;
edit `third_party/webview_go/webview.go` and change `webkit2gtk-4.1` back to
`webkit2gtk-4.0`, then `make build`.

Regenerate from module cache after an upgrade:

```bash
ver=v0.0.0-20240831120633-6173450d4dd6
cp -a "$(go env GOMODCACHE)/github.com/webview/webview_go@$ver" third_party/webview_go
chmod -R u+w third_party/webview_go
sed -i 's/webkit2gtk-4\.0/webkit2gtk-4.1/g' third_party/webview_go/webview.go
# trim optional dirs if desired
```
