package uniapp

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	sdk "github.com/lemon-paperplane/lemon-paperplane-main/lemon-paperplane-main-plugin-sdk"
)

const cliVue3 = `{"name":"uni-preset-vue","scripts":{"dev:custom":"uni -p","dev:h5":"uni","dev:mp-weixin":"uni -p mp-weixin","build:mp-weixin":"uni build -p mp-weixin","type-check":"vue-tsc --noEmit"},
"dependencies":{"@dcloudio/uni-app":"3.0.0-4080520251106001","vue":"^3.4.21"},"devDependencies":{"@dcloudio/vite-plugin-uni":"3.0.0-4080520251106001","vite":"5.2.8"}}`
const cliVue2 = `{"name":"sudoku","scripts":{"serve":"npm run dev:h5","dev:h5":"cross-env NODE_ENV=development UNI_PLATFORM=h5 vue-cli-service uni-serve","dev:mp-weixin":"cross-env UNI_PLATFORM=mp-weixin vue-cli-service uni-build --watch"},
"dependencies":{"@dcloudio/uni-h5":"^2.0.0-26120200226001","vue":"^2.6.11"},"devDependencies":{"@dcloudio/vue-cli-plugin-uni":"^2.0.0-26120200226001","@vue/cli-service":"^4.2.0"}}`

const hxManifest = `{
	"name": "vrtalk-app-x",
	"appid": "__UNI__F0B13EC",
	"uni-app-x": {},
	/* 小程序特有相关 */
	"mp-weixin": {
		"appid": "", // 留空
		"setting": {"urlCheck": false},
	},
	"vueVersion": "3",
}`
const pagesJSON = `{
	// 页面路由
	"pages": [{"path": "pages/index/index", "style": {"navigationBarTitleText": "首页 // 不是注释"}}],
	"globalStyle": {}
}`

func TestStripJSONC(t *testing.T) {
	var v map[string]any
	if err := json.Unmarshal(stripJSONC([]byte(pagesJSON)), &v); err != nil {
		t.Fatal(err)
	}
	pages := v["pages"].([]any)
	style := pages[0].(map[string]any)["style"].(map[string]any)
	if style["navigationBarTitleText"] != "首页 // 不是注释" {
		t.Errorf("字符串里的 // 不能被当注释：%v", style)
	}
}

func TestDetectCLI(t *testing.T) {
	dir := writeFiles(t, map[string]string{"package.json": cliVue3, "src/manifest.json": hxManifest, "src/pages.json": pagesJSON})
	p := DetectProject(dir, []string{"package.json", "src/"})
	if p == nil || p.Mode != ModeCLI || p.Vue != 3 || compilerVersion(p.Version) != "4.85" || compilerVersion("3.0.0-4060620250520001") != "4.66" || compilerVersion("3.0.0-3090820231124001") != "3.98" {
		t.Fatalf("cli vue3: %+v", p)
	}
	r, _ := (&Plugin{}).Detect(testCtx(), &sdk.DetectParams{Dir: dir, Files: []string{"package.json", "src/"}})
	d := r.Facets[0].Data
	if r.Facets[0].ID != "uniapp" || strings.Join(r.Claims, ",") != "src" || d["mode"] != ModeCLI {
		t.Fatalf("detect: %+v", r)
	}
	if strings.Join(d["hideScripts"].([]string), ",") != "dev:custom" || d["preferScripts"].([]string)[0] != "dev:h5" {
		t.Errorf("hints: %+v", d)
	}
	titles := d["scriptTitles"].(map[string]any)
	// 测试里没有加载文案，ctx.T 返回 key 本身，平台名未知时不生成标题
	if len(titles) != 0 {
		t.Errorf("没有文案时不应有标题：%v", titles)
	}
	// src 目录不是独立的 HBuilderX 工程
	if p := DetectProject(filepath.Join(dir, "src"), []string{"manifest.json", "pages.json"}); p != nil {
		t.Errorf("src 被误认：%+v", p)
	}
	dir2 := writeFiles(t, map[string]string{"package.json": cliVue2})
	if p := DetectProject(dir2, []string{"package.json"}); p == nil || p.Vue != 2 {
		t.Errorf("cli vue2: %+v", p)
	}
}

func TestScriptTitles(t *testing.T) {
	b, err := sdk.LoadBundle("..", "zh-Hans")
	if err != nil {
		t.Fatal(err)
	}
	pkg := &pkgJSON{Scripts: map[string]string{"dev:h5": "uni", "dev:mp-weixin": "uni -p mp-weixin", "build:mp-alipay": "uni build -p mp-alipay", "dev:custom": "uni -p", "dev:unknown-x": "x"}}
	titles, hide := ScriptTitles(b.T, pkg)
	if titles["dev:h5"] != "H5 开发" || titles["dev:mp-weixin"] != "微信小程序 开发" || titles["build:mp-alipay"] != "支付宝小程序 构建" || titles["dev:unknown-x"] != nil || len(hide) != 1 {
		t.Errorf("titles %v hide %v", titles, hide)
	}
}

func TestDetectHBuilderX(t *testing.T) {
	dir := writeFiles(t, map[string]string{"manifest.json": hxManifest, "pages.json": pagesJSON, "App.uvue": "<template></template>"})
	files := []string{"manifest.json", "pages.json", "App.uvue", "uni_modules/"}
	p := DetectProject(dir, files)
	if p == nil || p.Mode != ModeHBuilderX || !p.X || p.Vue != 3 || p.Name != "vrtalk-app-x" {
		t.Fatalf("hx: %+v", p)
	}
	r, _ := (&Plugin{}).Detect(testCtx(), &sdk.DetectParams{Dir: dir, Files: files})
	if len(r.Facets) != 2 || r.Facets[1].ID != "hbuilderx" || strings.Join(r.Claims, ",") != "**" || r.Name != "vrtalk-app-x" {
		t.Errorf("detect: %+v", r)
	}
	// 浏览器扩展的 manifest.json 不是 uni-app
	ext := writeFiles(t, map[string]string{"manifest.json": `{"manifest_version":3,"name":"ext"}`, "pages.json": pagesJSON})
	if p := DetectProject(ext, []string{"manifest.json", "pages.json"}); p != nil {
		t.Errorf("误认浏览器扩展：%+v", p)
	}
}

func TestAnalyzeHBuilderX(t *testing.T) {
	dir := writeFiles(t, map[string]string{"manifest.json": hxManifest, "pages.json": pagesJSON})
	r, err := (&Plugin{}).Analyze(testCtx(), &sdk.AnalyzeParams{Module: sdk.ModuleInfo{Dir: dir, RelDir: ""}})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.RunConfigurations) != 2 || !r.RunConfigurations[0].Default || r.RunConfigurations[0].Options["platform"] != "web" || r.Dependencies == nil || r.Dependencies.State != "none" {
		t.Errorf("hx analyze: %+v", r)
	}
	if FindHBuilderX(testCtx()) == nil {
		// 没装 HBuilderX 的机器上要给出问题与下载修复
		if len(r.Problems) != 1 || r.Problems[0].Fixes[0].Kind != sdk.FixOpenURL {
			t.Errorf("problems %+v", r.Problems)
		}
	}
	if got := strings.Join(HXLaunchArgs("web", "/p", "Chrome"), " "); got != "launch web --project /p --browser Chrome" {
		t.Errorf("launch args %q", got)
	}
	if got := strings.Join(HXLaunchArgs("mp-weixin", "/p", "Chrome"), " "); got != "launch mp-weixin --project /p" {
		t.Errorf("launch args %q", got)
	}
}

func TestAnalyzeCLIVue2Legacy(t *testing.T) {
	dir := writeFiles(t, map[string]string{"package.json": cliVue2})
	model := nodeModelJSON(map[string]string{"vue": "2.7.16"}, ">=14", "16", true, "@vue/cli-service")
	p := &sdk.AnalyzeParams{Module: sdk.ModuleInfo{Dir: dir, Models: map[string]json.RawMessage{"nodejs": model}}, Toolchains: map[string]sdk.Toolchain{"node": {Kind: "node", Version: "22.20.0"}}}
	r, _ := (&Plugin{}).Analyze(testCtx(), p)
	if len(r.Info) != 1 || r.Info[0].Value != "Vue 2 · Vue CLI" || len(r.Problems) != 1 || r.Problems[0].Fixes[1].Params["constraint"] != "16" {
		t.Errorf("vue2 / node 22: %+v", r)
	}
	p.Env = map[string]string{"NODE_OPTIONS": "--openssl-legacy-provider"}
	if r, _ = (&Plugin{}).Analyze(testCtx(), p); len(r.Problems) != 0 {
		t.Errorf("已开启旧版兼容仍提示：%+v", r.Problems)
	}
	p.Env = nil
	p.Toolchains["node"] = sdk.Toolchain{Kind: "node", Version: "16.17.1"}
	if r, _ = (&Plugin{}).Analyze(testCtx(), p); len(r.Problems) != 0 {
		t.Errorf("node 16: %+v", r.Problems)
	}
}

func TestWechatOutput(t *testing.T) {
	dir := writeFiles(t, map[string]string{"package.json": cliVue2, "dist/dev/mp-weixin/app.json": "{}", "dist/build/mp-weixin/app.json": "{}"})
	proj := DetectProject(dir, []string{"package.json"})
	if got := FindWechatOutput(dir, proj); got != filepath.Join(dir, "dist", "dev", "mp-weixin") {
		t.Errorf("cli output %q", got)
	}
	if got := FindWechatOutput(dir, proj, true); got != filepath.Join(dir, "dist", "build", "mp-weixin") {
		t.Errorf("build 运行触发时应先找发行产物：%q", got)
	}
	hx := writeFiles(t, map[string]string{"manifest.json": hxManifest, "pages.json": pagesJSON, "unpackage/dist/dev/mp-weixin/project.config.json": "{}"})
	proj = DetectProject(hx, []string{"manifest.json", "pages.json"})
	if got := FindWechatOutput(hx, proj); got != filepath.Join(hx, "unpackage", "dist", "dev", "mp-weixin") {
		t.Errorf("hx output %q", got)
	}
	empty := writeFiles(t, map[string]string{"package.json": cliVue3})
	if got := FindWechatOutput(empty, DetectProject(empty, []string{"package.json"})); got != "" {
		t.Errorf("没编译时应为空：%q", got)
	}
	if !wechatPortClosedRe.MatchString("[error] IDE service port disabled. To use CLI Call") || !wechatPortClosedRe.MatchString("工具的服务端口已关闭") {
		t.Error("服务端口关闭提示")
	}
}
