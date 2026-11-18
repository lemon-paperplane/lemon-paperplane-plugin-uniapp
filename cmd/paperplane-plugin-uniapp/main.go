// paperplane-plugin-uniapp 是 uniapp 插件的独立可执行入口。
package main

import (
	sdk "github.com/lemon-paperplane/lemon-paperplane-main/lemon-paperplane-main-plugin-sdk"
	uniapp "github.com/lemon-paperplane/lemon-paperplane-plugin-uniapp/plugin"
)

func main() { sdk.Serve(uniapp.New()) }
