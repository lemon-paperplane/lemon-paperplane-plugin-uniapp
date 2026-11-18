package uniapp

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	sdk "github.com/lemon-paperplane/lemon-paperplane-main/lemon-paperplane-main-plugin-sdk"
)

// RunTypeHBuilderX 是通过 HBuilderX 命令行运行的运行配置类型。
const RunTypeHBuilderX = "hbuilderx"

// Analyze：CLI 工程给信息与 Vue2 旧版兼容提示；HBuilderX 工程给运行配置与安装检查。
func (pl *Plugin) Analyze(ctx *sdk.Context, p *sdk.AnalyzeParams) (*sdk.AnalyzeResult, error) {
	dir := p.Module.Dir
	files := listDir(dir)
	proj := DetectProject(dir, files)
	if proj == nil {
		return &sdk.AnalyzeResult{}, nil
	}
	res := &sdk.AnalyzeResult{}
	vueLabel := "Vue " + itoa(proj.Vue)
	if proj.Mode == ModeCLI {
		build := "Vite"
		if proj.Vue == 2 {
			build = "Vue CLI"
		}
		nm := loadNodeModel(&p.Module)
		val := vueLabel + " · " + build
		pkg, _ := readPkg(dir)
		if hx := compilerVersion(depVersion(dir, pkg, nm, "@dcloudio/uni-app")); hx != "" {
			val += " · " + hx
		}
		res.Info = append(res.Info, sdk.InfoItem{Label: "uni-app", Value: val, Icon: "icons/uniapp.svg"})
		if proj.Vue == 2 || nm.LegacyPackage == "@dcloudio/vue-cli-plugin-uni" {
			if nv, ok := legacyNeeded(ctx, p, nm); ok {
				res.Problems = append(res.Problems, legacyProblem(ctx, "uni-app Vue 2 (Vue CLI)", nv))
			}
		}
		return res, nil
	}

	// HBuilderX 工程
	if proj.X {
		vueLabel = "uni-app x"
	}
	res.Info = append(res.Info, sdk.InfoItem{Label: "uni-app", Value: vueLabel, Icon: "icons/uniapp.svg"})
	hx := FindHBuilderX(ctx)
	if hx == nil {
		res.Info = append(res.Info, sdk.InfoItem{Label: "HBuilderX", Value: ctx.T("info.notInstalled")})
		res.Problems = append(res.Problems, sdk.Diagnosis{
			RuleID: "uniapp.hbuilderx-missing", Severity: "error",
			Title: ctx.T("problem.hbuilderx.title"), Message: ctx.T("problem.hbuilderx.message"),
			Fixes: []sdk.Fix{{ID: "download", Title: ctx.T("fix.downloadHBuilderX"), Kind: sdk.FixOpenURL, Params: map[string]string{"url": "https://www.dcloud.io/hbuilderx.html"}, Recommended: true}},
		})
	} else {
		res.Info = append(res.Info, sdk.InfoItem{Label: "HBuilderX", Value: hx.Version})
	}
	if !sdk.FileExists(filepath.Join(dir, "package.json")) {
		res.Dependencies = &sdk.DependencyState{State: "none"}
	}
	res.RunConfigurations = []sdk.RunConfiguration{
		{
			ID: sdk.StableID(p.Module.RelDir, RunTypeHBuilderX, "web"), Name: ctx.T("run.hx.web"), Type: RunTypeHBuilderX,
			Options: map[string]any{"platform": "web", "browser": "Chrome"}, Default: true, Source: "auto", Priority: 100,
		},
		{
			ID: sdk.StableID(p.Module.RelDir, RunTypeHBuilderX, "mp-weixin"), Name: ctx.T("run.hx.mpWeixin"), Type: RunTypeHBuilderX,
			Options: map[string]any{"platform": "mp-weixin"}, Source: "auto", Priority: 80,
		},
	}
	return res, nil
}

func listDir(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() {
			n += "/"
		}
		out = append(out, n)
	}
	return out
}

func itoa(n int) string { return strconv.Itoa(n) }

var compilerVerRe = regexp.MustCompile(`^\d+\.\d+\.\d+-(\d)(\d{2})(\d{2})\d{8}`)

// compilerVersion 从 "3.0.0-4080420251103001" 取出对应的编译器（HBuilderX）版本 "4.84"：
// 预发布号开头是 主版本 1 位 + 次版本 2 位 + 修订 2 位，后面是日期。
func compilerVersion(v string) string {
	m := compilerVerRe.FindStringSubmatch(v)
	if m == nil {
		return ""
	}
	minor, _ := strconv.Atoi(m[2])
	patch, _ := strconv.Atoi(m[3])
	return m[1] + "." + strconv.Itoa(minor) + strconv.Itoa(patch)
}

// HBuilderX 是一个已安装的 HBuilderX。
type HBuilderX struct {
	App     string // 应用目录（mac 为 .app）
	CLI     string // 命令行程序
	Version string
}

// FindHBuilderX 查找 HBuilderX：插件设置 → 常见安装位置。
func FindHBuilderX(ctx *sdk.Context) *HBuilderX {
	var cands []string
	if s, ok := ctx.Setting("hbuilderxPath", "").(string); ok && s != "" {
		cands = append(cands, sdk.Expand(s))
	}
	home := sdk.HomeDir()
	switch runtime.GOOS {
	case "darwin":
		cands = append(cands, "/Applications/HBuilderX.app", "/Applications/HBuilderX-Alpha.app", filepath.Join(home, "Applications", "HBuilderX.app"))
	case "windows":
		for _, base := range []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)"), `C:\`, `D:\`, `E:\`, os.Getenv("LOCALAPPDATA"), home} {
			if base != "" {
				cands = append(cands, filepath.Join(base, "HBuilderX"))
			}
		}
	default:
		cands = append(cands, filepath.Join(home, "HBuilderX"), "/opt/HBuilderX")
	}
	for _, c := range cands {
		if hx := hbuilderxAt(c); hx != nil {
			return hx
		}
	}
	return nil
}

var plistVerRe = regexp.MustCompile(`<key>CFBundleShortVersionString</key>\s*<string>([^<]+)</string>`)

func hbuilderxAt(path string) *HBuilderX {
	if st, err := os.Stat(path); err == nil && !st.IsDir() {
		// 设置里直接给了 cli 或主程序
		path = filepath.Dir(path)
		if strings.HasSuffix(path, filepath.Join("Contents", "MacOS")) {
			path = filepath.Dir(filepath.Dir(path))
		}
	}
	var cli string
	switch {
	case strings.HasSuffix(path, ".app"):
		cli = filepath.Join(path, "Contents", "MacOS", "cli")
	case runtime.GOOS == "windows":
		cli = filepath.Join(path, "cli.exe")
	default:
		cli = filepath.Join(path, "cli")
	}
	if !sdk.FileExists(cli) {
		return nil
	}
	hx := &HBuilderX{App: path, CLI: cli}
	if m := plistVerRe.FindStringSubmatch(sdk.ReadText(filepath.Join(path, "Contents", "Info.plist"), 64<<10)); m != nil {
		hx.Version = shortHXVersion(m[1])
	}
	return hx
}

// shortHXVersion："5.07.2026041006" → "5.07"。
func shortHXVersion(v string) string {
	parts := strings.Split(v, ".")
	if len(parts) >= 2 {
		return parts[0] + "." + parts[1]
	}
	return v
}

// PrepareRun 通过 HBuilderX 命令行运行：先确保 HBuilderX 已启动（cli open），再 cli launch。
func (pl *Plugin) PrepareRun(ctx *sdk.Context, p *sdk.PrepareRunParams) (*sdk.LaunchSpec, error) {
	if p.Config.Type != RunTypeHBuilderX {
		return nil, sdk.ErrNotSupported
	}
	hx := FindHBuilderX(ctx)
	if hx == nil {
		return nil, sdk.UserError("%s", ctx.T("problem.hbuilderx.message"))
	}
	platform := sdk.OptString(p.Config.Options, "platform", "web")
	r, err := ctx.Host.Exec(ctx, &sdk.ExecParams{Command: hx.CLI, Args: []string{"open"}, Console: p.RunID, Title: ctx.T("run.hx.starting"), TimeoutMs: 60000})
	if err != nil {
		return nil, err
	}
	if r.ExitCode != 0 {
		return nil, sdk.UserError("%s", ctx.T("run.hx.openFailed", r.ExitCode))
	}
	args := HXLaunchArgs(platform, p.Module.Dir, sdk.OptString(p.Config.Options, "browser", ""))
	return &sdk.LaunchSpec{
		Command: hx.CLI,
		Args:    args,
		Cwd:     p.Module.Dir,
		Display: "cli " + strings.Join(args, " "),
	}, nil
}

// HXLaunchArgs 生成 HBuilderX `cli launch` 参数（--project 接受工程名或绝对路径）。
func HXLaunchArgs(platform, dir, browser string) []string {
	args := []string{"launch", platform, "--project", dir}
	if platform == "web" && browser != "" {
		args = append(args, "--browser", browser)
	}
	return args
}

// ---------------------------------------------------------------------------
// 动作

// 动作 id。
const (
	ActionOpenWechat    = "open-wechat-devtools"
	ActionOpenHBuilderX = "open-hbuilderx"
)

// WechatOutputDirs 返回微信小程序编译产物的候选目录（开发版优先）。
func WechatOutputDirs(moduleDir string, proj *Project) []string {
	if proj != nil && proj.Mode == ModeHBuilderX {
		return []string{
			filepath.Join(moduleDir, "unpackage", "dist", "dev", "mp-weixin"),
			filepath.Join(moduleDir, "unpackage", "dist", "build", "mp-weixin"),
		}
	}
	out := []string{
		filepath.Join(moduleDir, "dist", "dev", "mp-weixin"),
		filepath.Join(moduleDir, "dist", "build", "mp-weixin"),
	}
	// 也有人在 CLI 工程里用 HBuilderX 编译
	return append(out, filepath.Join(moduleDir, "unpackage", "dist", "dev", "mp-weixin"))
}

// FindWechatOutput 找到第一个已经编译出来的小程序目录（含 app.json 或 project.config.json）。
// preferBuild：从「构建」运行的工具栏触发时先找发行产物。
func FindWechatOutput(moduleDir string, proj *Project, preferBuild ...bool) string {
	dirs := WechatOutputDirs(moduleDir, proj)
	if len(preferBuild) > 0 && preferBuild[0] && len(dirs) >= 2 {
		dirs[0], dirs[1] = dirs[1], dirs[0]
	}
	for _, d := range dirs {
		if sdk.FileExists(filepath.Join(d, "app.json")) || sdk.FileExists(filepath.Join(d, "project.config.json")) {
			return d
		}
	}
	return ""
}

// FindWechatCLI 找微信开发者工具的命令行：插件设置 → 常见安装位置。
func FindWechatCLI(ctx *sdk.Context) string {
	var cands []string
	if s, ok := ctx.Setting("wechatDevtoolsPath", "").(string); ok && s != "" {
		s = sdk.Expand(s)
		if strings.HasSuffix(s, ".app") {
			s = filepath.Join(s, "Contents", "MacOS", "cli")
		} else if st, err := os.Stat(s); err == nil && st.IsDir() {
			s = filepath.Join(s, "cli.bat")
		}
		cands = append(cands, s)
	}
	switch runtime.GOOS {
	case "darwin":
		cands = append(cands, "/Applications/wechatwebdevtools.app/Contents/MacOS/cli", filepath.Join(sdk.HomeDir(), "Applications", "wechatwebdevtools.app", "Contents", "MacOS", "cli"))
	case "windows":
		for _, base := range []string{os.Getenv("ProgramFiles(x86)"), os.Getenv("ProgramFiles"), `C:\Program Files (x86)`, `C:\Program Files`, `D:\Program Files (x86)`, `D:\Program Files`} {
			if base != "" {
				cands = append(cands, filepath.Join(base, "Tencent", "微信web开发者工具", "cli.bat"), filepath.Join(base, "Tencent", "微信开发者工具", "cli.bat"))
			}
		}
	}
	for _, c := range cands {
		if sdk.FileExists(c) {
			return c
		}
	}
	return ""
}

// 微信开发者工具命令行在服务端口关闭时的输出（中英文）。
var wechatPortClosedRe = regexp.MustCompile(`(?i)service port disabled|服务端口已关闭|IDE service port`)

// InvokeAction 处理两个打开动作。
func (pl *Plugin) InvokeAction(ctx *sdk.Context, p *sdk.InvokeActionParams) (*sdk.InvokeActionResult, error) {
	dir := p.Module.Dir
	proj := DetectProject(dir, listDir(dir))
	switch p.Action {
	case ActionOpenWechat:
		// 从运行工具栏触发时按那次运行的脚本决定先找开发产物还是发行产物
		preferBuild := false
		if p.Run != nil && p.Run.Config != nil {
			preferBuild = strings.HasPrefix(sdk.OptString(p.Run.Config.Options, "script", ""), "build:")
		}
		out := FindWechatOutput(dir, proj, preferBuild)
		if out == "" {
			rel := WechatOutputDirs(dir, proj)[0]
			if r, err := filepath.Rel(dir, rel); err == nil {
				rel = r
			}
			return nil, sdk.UserError("%s", ctx.T("wechat.noOutput", filepath.ToSlash(rel)))
		}
		cli := FindWechatCLI(ctx)
		if cli == "" {
			return nil, sdk.UserError("%s", ctx.T("wechat.notInstalled"))
		}
		r, err := ctx.Host.Exec(ctx, &sdk.ExecParams{
			Command: cli, Args: []string{"open", "--project", out},
			Console: p.TaskID, Capture: true, TimeoutMs: 90000, Title: ctx.T("action.openWechat"),
		})
		if err != nil {
			return nil, err
		}
		if wechatPortClosedRe.MatchString(r.Stdout + r.Stderr) {
			return nil, sdk.UserError("%s", ctx.T("wechat.portClosed"))
		}
		if r.TimedOut || r.ExitCode != 0 {
			return nil, sdk.UserError("%s", ctx.T("wechat.failed", r.ExitCode))
		}
		return &sdk.InvokeActionResult{Message: ctx.T("wechat.opened", out)}, nil
	case ActionOpenHBuilderX:
		hx := FindHBuilderX(ctx)
		if hx == nil {
			return nil, sdk.UserError("%s", ctx.T("problem.hbuilderx.message"))
		}
		if r, err := ctx.Host.Exec(ctx, &sdk.ExecParams{Command: hx.CLI, Args: []string{"open"}, Console: p.TaskID, TimeoutMs: 60000}); err != nil || r.ExitCode != 0 {
			return nil, sdk.UserError("%s", ctx.T("run.hx.openFailed", exitCode(r)))
		}
		r, err := ctx.Host.Exec(ctx, &sdk.ExecParams{Command: hx.CLI, Args: []string{"project", "open", "--path", dir}, Console: p.TaskID, TimeoutMs: 60000})
		if (err != nil || r.ExitCode != 0) && runtime.GOOS == "darwin" {
			// 命令行导入失败时退回系统「用 HBuilderX 打开」
			if _, lerr := exec.LookPath("open"); lerr == nil {
				r, err = ctx.Host.Exec(ctx, &sdk.ExecParams{Command: "open", Args: []string{"-a", hx.App, dir}, Console: p.TaskID, TimeoutMs: 30000})
			}
		}
		if err != nil {
			return nil, err
		}
		if r.ExitCode != 0 {
			return nil, sdk.UserError("%s", ctx.T("run.hx.openFailed", r.ExitCode))
		}
		return &sdk.InvokeActionResult{Message: ctx.T("hx.opened")}, nil
	}
	return nil, sdk.ErrNotSupported
}

func exitCode(r *sdk.ExecResult) int {
	if r == nil {
		return -1
	}
	return r.ExitCode
}
