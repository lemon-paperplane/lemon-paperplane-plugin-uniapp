// Package uniapp 是 Lemon Paperplane 的 uni-app 插件：识别 uni-app CLI 工程（Vue2 / Vue3）与 HBuilderX 工程，
// 给 nodejs 插件提供脚本偏好与显示名（H5 开发默认），HBuilderX 工程通过 HBuilderX 命令行运行，
// 以及「在微信开发者工具中打开」「在 HBuilderX 中打开」两个动作。
package uniapp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	sdk "github.com/lemon-paperplane/lemon-paperplane-main/lemon-paperplane-main-plugin-sdk"
)

// PluginID 是插件 id。
const PluginID = "uniapp"

// 工程形态。
const (
	ModeCLI       = "cli"
	ModeHBuilderX = "hbuilderx"
)

// Plugin 实现 uniapp 插件。
type Plugin struct{}

// New 返回插件实例。
func New() any { return &Plugin{} }

// Project 是 uni-app 工程的识别结果。
type Project struct {
	Mode    string // cli | hbuilderx
	Vue     int    // 2 | 3
	X       bool   // uni-app x
	Name    string // manifest.json 的 name
	SrcDir  string // 源码目录（CLI 为 src，HBuilderX 为工程根）
	Version string // @dcloudio/uni-app 版本（CLI）
}

// isCLIPkg 判断 package.json 是不是 uni-app CLI 工程。
func isCLIPkg(pkg *pkgJSON) bool {
	return pkg != nil && (pkg.has("@dcloudio/vite-plugin-uni") || pkg.has("@dcloudio/vue-cli-plugin-uni") || (pkg.has("@dcloudio/uni-app") && hasUniScript(pkg)))
}

var uniScriptRe = regexp.MustCompile(`\buni(?:\s|$)|vue-cli-service\s+uni-|uniapp-cli`)

func hasUniScript(pkg *pkgJSON) bool {
	for _, c := range pkg.Scripts {
		if uniScriptRe.MatchString(c) {
			return true
		}
	}
	return false
}

// DetectProject 识别目录：CLI 看 package.json 的 @dcloudio 依赖；HBuilderX 看根目录的 manifest.json + pages.json。
func DetectProject(dir string, files []string) *Project {
	pkg, _ := readPkg(dir)
	if isCLIPkg(pkg) {
		p := &Project{Mode: ModeCLI, Vue: 3, SrcDir: filepath.Join(dir, "src")}
		if pkg.has("@dcloudio/vue-cli-plugin-uni") || major(cleanRange(pkg.rangeOf("vue"))) == 2 {
			p.Vue = 2
		}
		p.Version = cleanRange(pkg.rangeOf("@dcloudio/uni-app"))
		if m := readManifest(filepath.Join(p.SrcDir, "manifest.json")); m != nil {
			p.Name = m.name()
		}
		return p
	}
	if !sdk.HasFile(files, "manifest.json") || !sdk.HasFile(files, "pages.json") {
		return nil
	}
	// CLI 工程的 src 目录不是独立工程
	if parent, err := readPkg(filepath.Dir(dir)); err == nil && isCLIPkg(parent) {
		return nil
	}
	m := readManifest(filepath.Join(dir, "manifest.json"))
	if m == nil || !m.isUni() || !pagesJSONValid(filepath.Join(dir, "pages.json")) {
		return nil
	}
	p := &Project{Mode: ModeHBuilderX, Vue: 2, SrcDir: dir, Name: m.name()}
	if _, ok := m["uni-app-x"]; ok || sdk.HasFile(files, "App.uvue") {
		p.X, p.Vue = true, 3
	} else if strings.TrimSpace(rawStr(m["vueVersion"])) == "3" {
		p.Vue = 3
	}
	return p
}

// manifest 是宽松解析的 manifest.json（允许注释与尾逗号）。
type manifest map[string]json.RawMessage

func readManifest(path string) manifest {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var m manifest
	if json.Unmarshal(stripJSONC(b), &m) != nil {
		return nil
	}
	return m
}

func (m manifest) name() string { return rawStr(m["name"]) }

// isUni：有 uni-app 特有的键才算（避免把浏览器扩展、PWA 的 manifest.json 当成 uni-app）。
func (m manifest) isUni() bool {
	for _, k := range []string{"uni-app-x", "vueVersion", "app-plus", "mp-weixin", "uniStatistics", "quickapp", "app-harmony"} {
		if _, ok := m[k]; ok {
			return true
		}
	}
	return strings.HasPrefix(rawStr(m["appid"]), "__UNI__")
}

func pagesJSONValid(path string) bool {
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var v struct {
		Pages []json.RawMessage `json:"pages"`
	}
	return json.Unmarshal(stripJSONC(b), &v) == nil && len(v.Pages) > 0
}

func rawStr(r json.RawMessage) string {
	var s string
	if json.Unmarshal(r, &s) == nil {
		return s
	}
	return strings.Trim(string(r), `"`)
}

// stripJSONC 去掉 // 与 /* */ 注释和对象、数组末尾多余的逗号（字符串内部保持不变）。
func stripJSONC(b []byte) []byte {
	b = []byte(strings.TrimPrefix(string(b), "\xef\xbb\xbf"))
	out := make([]byte, 0, len(b))
	inStr, esc := false, false
	for i := 0; i < len(b); i++ {
		c := b[i]
		if inStr {
			out = append(out, c)
			switch {
			case esc:
				esc = false
			case c == '\\':
				esc = true
			case c == '"':
				inStr = false
			}
			continue
		}
		switch {
		case c == '"':
			inStr = true
			out = append(out, c)
		case c == '/' && i+1 < len(b) && b[i+1] == '/':
			for i < len(b) && b[i] != '\n' {
				i++
			}
			out = append(out, '\n')
		case c == '/' && i+1 < len(b) && b[i+1] == '*':
			i += 2
			for i+1 < len(b) && !(b[i] == '*' && b[i+1] == '/') {
				i++
			}
			i++
		default:
			out = append(out, c)
		}
	}
	return regexp.MustCompile(`,(\s*[}\]])`).ReplaceAll(out, []byte("$1"))
}

// Detect 识别 uni-app 工程。CLI 工程给 nodejs 插件提供脚本偏好与显示名，并认领 src 目录。
func (pl *Plugin) Detect(ctx *sdk.Context, p *sdk.DetectParams) (*sdk.DetectResult, error) {
	proj := DetectProject(p.Dir, p.Files)
	if proj == nil {
		return &sdk.DetectResult{}, nil
	}
	data := map[string]any{"mode": proj.Mode, "vue": proj.Vue}
	res := &sdk.DetectResult{Name: proj.Name}
	if proj.Mode == ModeCLI {
		pkg, _ := readPkg(p.Dir)
		titles, hide := ScriptTitles(ctx.T, pkg)
		data[sdk.FacetDataPreferScripts] = []string{"dev:h5", "serve", "dev"}
		data[sdk.FacetDataScriptTitles] = titles
		data[sdk.FacetDataHideScripts] = hide
		res.Claims = []string{"src"}
		res.Name = "" // CLI 工程用 package.json / 目录名
	} else {
		data["x"] = proj.X
		res.Facets = append(res.Facets, sdk.Facet{ID: "hbuilderx"})
		res.Claims = []string{"**"}
	}
	res.Facets = append([]sdk.Facet{{ID: "uniapp", Data: data}}, res.Facets...)
	return res, nil
}

var scriptPlatformRe = regexp.MustCompile(`^(dev|build):(.+)$`)

// ScriptTitles 生成 uni-app 脚本的显示名（dev:mp-weixin → 微信小程序 开发），并给出要隐藏的脚本（custom 需要额外参数）。
func ScriptTitles(tr func(key string, args ...any) string, pkg *pkgJSON) (map[string]any, []string) {
	titles := map[string]any{}
	var hide []string
	if pkg == nil {
		return titles, hide
	}
	for name := range pkg.Scripts {
		m := scriptPlatformRe.FindStringSubmatch(name)
		if m == nil {
			continue
		}
		if m[2] == "custom" {
			hide = append(hide, name)
			continue
		}
		key := "platform." + m[2]
		pname := tr(key)
		if pname == key {
			continue
		}
		titles[name] = tr("script."+m[1], pname)
	}
	return titles, hide
}
