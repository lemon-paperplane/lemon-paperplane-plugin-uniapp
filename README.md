# Lemon Paperplane uni-app 插件

- CLI 工程（依赖 `@dcloudio/vite-plugin-uni` 为 Vue 3，`@dcloudio/vue-cli-plugin-uni` 为 Vue 2）：通过 nodejs 插件运行，默认「H5 开发」（`dev:h5`），各平台脚本显示为「微信小程序 开发」等，`dev:custom` 不生成运行配置。认领 `src` 目录。
- Vue 2 CLI 工程偏好 Node 16；在 Node 17+ 上预先提示 OpenSSL 旧版兼容。
- HBuilderX 工程（根目录 `manifest.json` + `pages.json`，含 uni-app x）：facet `hbuilderx`，运行配置「运行到浏览器」「运行到微信小程序」，通过 HBuilderX 命令行 `cli open` 后 `cli launch web|mp-weixin --project <目录>`。未安装 HBuilderX 时给出问题与下载链接。
- 动作「在微信开发者工具中打开」：打开 `dist/dev/mp-weixin`（HBuilderX 工程为 `unpackage/dist/dev/mp-weixin`），需要开发者工具开启服务端口。
- 动作「在 HBuilderX 中打开」：`cli project open --path <目录>`，macOS 上失败时退回 `open -a`。
- 插件设置可以指定 HBuilderX 与微信开发者工具的位置。

```sh
./build.sh
go test ./...
```
