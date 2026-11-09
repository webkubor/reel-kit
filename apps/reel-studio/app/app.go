// Package app 是原生工作台的应用状态与界面。
//
// 界面分三栏（与网页版同一套信息架构）：
//   - 左：项目列表与连接状态
//   - 中：生成器（通道 / 模型 / 音色 / 提示词）
//   - 右：预览与素材库
//
// 底部是任务历史，含「取回已生成素材」。
package app

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/webkubor/reel-kit/apps/reel-studio/design"
	"github.com/webkubor/reel-kit/apps/reel-studio/studioapi"
	kit "github.com/webkubor/reel-kit/apps/reel-studio/ui"
)

// Mode 是生成器的模式。
type Mode int

// Version 是应用版本号。
//
// 写死在这里而不在界面上：界面上的版本号必须跟实际发布的版本一致，
// 写死在两处就会出现「显示 0.1.0、实际发的是 0.2.0」。
// 打包时可以用 -ldflags "-X ...app.Version=x.y.z" 覆盖。
var Version = "0.2.0"

// 模式。顺序与网页版的标签一致：项目推荐排在最前，它就是本产品的主线。
const (
	ModeRecommend Mode = iota
	ModeTalk
	ModeVideo
	ModeImage
	ModeSpeech
	ModeExport
	ModeSettings
)

// Options 是启动参数。
type Options struct {
	BaseURL string
	// BackendRoot 是 reel-kit 的运行目录；留空则自动找。
	BackendRoot string
}

// App 是应用状态。
type App struct {
	opts Options
	kit  kit.Comps
	win  *mygo.Window
	// mu 只保护 pending 这一个队列。
	//
	// 界面状态本身**不加锁**：所有写入都经 mutate 排到渲染线程执行，
	// 所有读取都发生在渲染线程（View），所以两者天然串行。
	// 之前给整个状态加锁，结果 View 持锁渲染、按钮回调里再取锁 → 死锁。
	mu      sync.Mutex
	pending []func()
	ready   bool

	state     *studioapi.State
	providers *studioapi.Providers
	bgm       *studioapi.BGMLibrary

	// backend 状态
	backendProc  *Backend
	backendState BackendStatus

	projectID string
	mode      Mode

	// 生成器的表单状态
	prompt     string
	backend    string
	model      string
	voiceID    string
	persona    string
	profile    string
	ratio      string
	duration   string
	resolution string
	assetIDs   []string
	previewID  string

	// 项目推荐
	repoURL     string
	recTitle    string
	recCaptions string
	recPlatform string
	recRatio    string
	recBGM      string
	recImages   []string
	recSelected map[string]bool

	// 知识口播
	talkTitle    string
	talkCaptions string
	talkSubtitle string
	talkStyle    string
	talkPersona  string
	talkImages   []string
	talkSelected map[string]bool
	talkStyles   []studioapi.Style
	// talkStylesLoading 防止每帧重复拉风格表
	talkStylesLoading bool

	// 制作闭环
	reviewChecks     map[string]bool
	pubPlatform      string
	pubURL           string
	perfPlays        string
	perfLikes        string
	perfSaves        string
	reviewFindings   string
	reviewHypothesis string
	reviewChanges    string

	// 提示信息
	notice    string
	noticeErr bool
	// busy 是「有请求在飞」。
	//
	// 用原子量而不是普通 bool：后台线程要能随时读到自己刚设的值。
	// 普通 bool 配 mutate 会出现这种错 —— mutate 在有窗口时是「排进主线程」，
	// 不是同步执行，poll 读到的是上一帧的旧值，于是「忙」这个状态根本传不出去。
	busy atomic.Bool

	// 更新与日志
	updates updates
	log     *LogWriter

	// 问题与需求反馈
	feedback feedbackState

	// stopping 用来叫停后台轮询。
	stopping chan struct{}

	media mediaCache
}

// New 返回一个应用。
func New(opts Options) *App {
	if opts.BaseURL == "" {
		opts.BaseURL = "http://127.0.0.1:4390"
	}
	root := opts.BackendRoot
	if root == "" {
		root = FindRoot()
	}
	backend := NewBackend(opts.BaseURL, root)
	backend.Bundled = FindBundled()
	return &App{
		opts:         opts,
		kit:          kit.New(design.Light()),
		ratio:        "9:16",
		duration:     "5",
		recPlatform:  "channels",
		recRatio:     "9:16",
		reviewChecks: map[string]bool{},
		recSelected:  map[string]bool{},
		talkSelected: map[string]bool{},
		talkStyle:    "paper",
		backendProc:  backend,
		backendState: BackendStatus{State: "starting", Note: "正在连接本地工作台…"},
		log:          NewLogWriter(),
		feedback: feedbackState{
			kind:    FeedbackBug,
			include: true,
		},
		stopping: make(chan struct{}),
	}
}

// SetWindow 绑定窗口。
func (a *App) SetWindow(w *mygo.Window) { a.win = w }

// mutate 把一次状态修改排到渲染线程上执行。
//
// **所有状态写入都必须走这里**，而且必须发生在渲染线程上：
// View 在渲染线程读状态，后台 goroutine 写状态，两边撞上就是数据竞态
// （界面可能渲染出写了一半的值，竞态检测器也会报）。
//
// 具体做法是排队：后台线程把要改的东西塞进 pending，请求下一帧；
// View 在渲染前把 pending 里的动作一个个执行掉。
//
// 为什么不直接用 win.Update 改：那样修改发生在主线程的某个时刻，
// 而 View 可能已经在渲染了 —— 读和写还是会撞。排队 + 每帧开头统一执行，
// 才能保证「渲染看到的永远是完整的上一帧状态」。
//
// 无窗口（单测）时没有帧循环，但 Tester's Frame() 会调 View，
// 排队的动作照样会被执行。
func (a *App) mutate(fn func()) {
	a.mu.Lock()
	a.pending = append(a.pending, fn)
	a.mu.Unlock()
	a.redraw()
}

// drainPending 执行排队的状态修改。只能在渲染线程调用。
func (a *App) drainPending() {
	a.mu.Lock()
	pending := a.pending
	a.pending = nil
	a.mu.Unlock()
	for _, fn := range pending {
		fn()
	}
}

// Start 准备本地工作台、拉初始数据、开始轮询。
//
// 全程在后台线程做：开窗时主线程不能被网络请求卡住。
func (a *App) Start() {
	a.logf("Reel Studio 启动（版本 %s）", mygo.App.Version())
	go func() {
		if a.backendProc != nil {
			status := a.backendProc.Ensure()
			a.mutate(func() {
				a.backendState = status
			})
			a.logf("本地工作台：%s（%s）", status.State, status.Note)
		}
		a.refresh()
		a.poll()
	}()
	// 后台查一次更新：有新版就在界面上给个提示，而不是等人手动点。
	go a.checkUpdate()
}

// Stop 收尾：停掉自己起的工作台。
func (a *App) Stop() {
	select {
	case <-a.stopping:
		// 已经停过了
	default:
		close(a.stopping)
	}
	if a.backendProc != nil {
		a.backendProc.Stop()
	}
	if a.log != nil {
		a.log.Close()
	}
}

// refresh 重新读取后端状态。
func (a *App) refresh() {
	client := studioapi.New(a.opts.BaseURL)
	state, err := client.State()
	if err != nil {
		a.mutate(func() {
			a.notice = "连不上本地工作台：" + err.Error()
			a.noticeErr = true
		})
		return
	}
	providers, _ := client.Providers()
	bgm, _ := client.BGM()
	a.mutate(func() {
		a.state = state
		if providers != nil {
			a.providers = providers
		}
		if bgm != nil {
			a.bgm = bgm
		}
		if a.projectID == "" && len(state.Projects) > 0 {
			a.projectID = state.Projects[0].ID
		}
		// 通道就绪状态可能变（比如用户刚配好密钥），要在可用通道里挑一个
		a.ensureBackend()
	})
}

// ensureTalkStyles 拉一次口播可用的视觉风格。
//
// 风格表是固定的（服务端 /api/styles），拉一次就够；
// 拉失败时留空，界面会只显示标题与脚本，不影响出片。
func (a *App) ensureTalkStyles() {
	if len(a.talkStyles) > 0 || a.talkStylesLoading {
		return
	}
	a.talkStylesLoading = true
	go func() {
		client := studioapi.New(a.opts.BaseURL)
		styles, err := client.Styles()
		a.mutate(func() {
			a.talkStylesLoading = false
			if err != nil {
				a.logf("读取口播视觉风格失败：%s", err.Error())
				return
			}
			a.talkStyles = styles
			if a.talkStyle == "" && len(styles) > 0 {
				a.talkStyle = styles[0].Key
			}
		})
		a.redraw()
	}()
}

// poll 每两秒拉一次状态。原生界面没有网页的 setInterval，只能自己轮询。
//
// 忙的时候不轮询：请求刚提交时界面正在显示「已提交」，此时拉回来的
// 中间态会把提示冲掉，用户以为没提交。
func (a *App) poll() {
	for {
		select {
		case <-a.stopping:
			return
		case <-time.After(2 * time.Second):
		}
		if a.busy.Load() {
			continue
		}
		client := studioapi.New(a.opts.BaseURL)
		state, err := client.State()
		if err != nil {
			continue
		}
		a.mutate(func() {
			changed := a.state == nil || len(state.Jobs) != len(a.state.Jobs) ||
				len(state.Assets) != len(a.state.Assets) || jobSummary(state.Jobs) != jobSummary(a.state.Jobs)
			a.state = state
			if changed && a.win != nil {
				a.redraw()
			}
		})
	}
}

// jobSummary 把任务的关键状态压成一行，用于判断有没有变化。
func jobSummary(jobs []studioapi.Job) string {
	var b strings.Builder
	for _, j := range jobs {
		b.WriteString(j.ID)
		b.WriteString(j.Status)
		b.WriteString(j.Message)
		b.WriteString(j.Error)
		b.WriteString(j.AssetID)
	}
	return b.String()
}

// redraw 请求重绘。
func (a *App) redraw() {
	if a.win == nil {
		return
	}
	a.win.Update(func() {})
}

// ensureBackend 在通道列表变化后挑一条能用的通道。
//
// 只在"当前通道不见了"时才换 —— 用户选的通道即使暂时未就绪也不替换，
// 否则等于替他换了供应商。
func (a *App) ensureBackend() {
	group := a.group()
	if a.providers == nil {
		return
	}
	channels := a.providers.Channels(group)
	if len(channels) == 0 {
		return
	}
	for _, c := range channels {
		if c.ID == a.backend {
			return
		}
	}
	for _, c := range channels {
		if c.Ready {
			a.backend = c.ID
			break
		}
	}
}

// group 返回当前模式对应的能力组。
func (a *App) group() string {
	switch a.mode {
	case ModeVideo:
		return "video"
	case ModeImage:
		return "image"
	case ModeRecommend, ModeSpeech:
		// 项目推荐要配音，也要混 BGM：用的是语音这一组通道。
		return "voice"
	default:
		return "video"
	}
}

// Channels 返回当前模式的通道列表。
func (a *App) Channels() []studioapi.Channel {
	if a.providers == nil {
		return nil
	}
	switch a.group() {
	case "video":
		return a.providers.Video.Channels
	case "image":
		return a.providers.Image.Channels
	case "voice":
		return a.providers.Voice.Channels
	}
	return nil
}

// CurrentChannel 返回当前选中的通道。
func (a *App) CurrentChannel() *studioapi.Channel {
	for _, c := range a.Channels() {
		if c.ID == a.backend {
			cc := c
			return &cc
		}
	}
	return nil
}

// CurrentProject 返回当前项目。
func (a *App) CurrentProject() *studioapi.Project {
	if a.state == nil {
		return nil
	}
	for _, p := range a.projects() {
		if p.ID == a.projectID {
			pp := p
			return &pp
		}
	}
	return nil
}

// ProjectAssets 返回当前项目的素材。
func (a *App) ProjectAssets() []studioapi.Asset {
	if a.state == nil {
		return nil
	}
	var out []studioapi.Asset
	for _, asset := range a.state.Assets {
		if asset.ProjectID == a.projectID {
			out = append(out, asset)
		}
	}
	return out
}

// ProjectJobs 返回当前项目的任务，按新到旧。
func (a *App) ProjectJobs() []studioapi.Job {
	if a.state == nil {
		return nil
	}
	var out []studioapi.Job
	for _, j := range a.state.Jobs {
		if j.ProjectID == a.projectID {
			out = append(out, j)
		}
	}
	sort.SliceStable(out, func(i, k int) bool { return out[i].CreatedAt > out[k].CreatedAt })
	return out
}

// View 画出整个窗口。
func (a *App) View(c *ui.Context) {
	// 先把后台线程排队的状态修改执行掉，再渲染。
	// 这样渲染看到的一定是完整状态，不会撞上写到一半的字段。
	a.drainPending()
	a.kit = kit.New(design.Light())
	c.SetTheme(a.kit.T.Theme())
	ui.Row(c).Fill().Gap(0).Children(func() {
		a.viewSidebar(c)
		a.viewMain(c)
	})
}

// viewSidebar 画左栏：品牌、项目列表、连接状态。
func (a *App) viewSidebar(c *ui.Context) {
	ui.Column(c).Width(232).FillHeight().
		Background(a.kit.T.Surface).Border(1, a.kit.T.Line).Children(func() {
		ui.Column(c).FillWidth().Gap(design.SpaceLG).Padding(design.SpaceXL).Children(func() {
			ui.Text(c, "Reel Studio").FontSize(design.FontBrand).Bold().TextColor(a.kit.T.Ink)
			ui.Text(c, "本地视频工作台").FontSize(design.FontTiny).TextColor(a.kit.T.Muted)

			ui.Row(c).Gap(design.SpaceSM).Children(func() {
				if kitButton(a.kit, c, kit.ButtonPrimary, "新建项目", false).Clicked() {
					a.newProject()
				}
			})

			ui.Divider(c)

			ui.Text(c, "项目").FontSize(design.FontTiny).TextColor(a.kit.T.Muted)
			if a.state == nil || len(a.state.Projects) == 0 {
				ui.Text(c, "还没有项目").FontSize(design.FontSmall).TextColor(a.kit.T.Muted)
			}
		})
		// 项目列表会变多，可滚；连接状态固定在底部。
		ui.Scroll(c).FillWidth().Grow(1).MinHeight(0).Padding(design.SpaceLG, design.SpaceXL).Children(func() {
			ui.Column(c).FillWidth().Gap(design.SpaceXS).Children(func() {
				for _, p := range a.projects() {
					project := p
					selected := project.ID == a.projectID
					item := ui.Button(c, project.Name)
					item = item.TextColor(a.kit.T.Ink).FillWidth().Justify(ui.Start).Radius(design.RadiusSM)
					item = item.Padding(design.SpaceSM, design.SpaceMD)
					if selected {
						item = item.Background(a.kit.T.AccentSoft).TextColor(a.kit.T.Accent)
					}
					if item.Clicked() {
						a.projectID = project.ID
						a.redraw()
					}
				}
			})
		})
		ui.Column(c).FillWidth().Gap(design.SpaceSM).
			Background(a.kit.T.Surface).Border(1, a.kit.T.Line).
			Padding(design.SpaceLG, design.SpaceXL).Children(func() {
			a.viewBackendStatus(c)
			a.viewConnections(c)
			a.viewSidebarFooter(c)
		})
	})
}

// viewSidebarFooter 画左栏底部的设置与反馈入口。
func (a *App) viewSidebarFooter(c *ui.Context) {
	info := a.updates.info
	if info.State == UpdateReady {
		b := ui.Button(c, "★ 发现新版 "+info.Version)
		b = b.FillWidth().Radius(design.RadiusSM).Padding(design.SpaceSM, design.SpaceMD).
			Background(a.kit.T.AccentSoft).TextColor(a.kit.T.Accent)
		if b.Clicked() {
			a.mode = ModeSettings
			a.redraw()
		}
		return
	}
	b := ui.Button(c, "⚙ 设置与反馈")
	b = b.FillWidth().Radius(design.RadiusSM).Padding(design.SpaceSM, design.SpaceMD).
		TextColor(a.kit.T.Muted)
	if a.mode == ModeSettings {
		b = b.Background(a.kit.T.AccentSoft).TextColor(a.kit.T.Accent)
	}
	if b.Clicked() {
		a.mode = ModeSettings
		a.redraw()
	}
}

// viewSettingsAndFeedback 画软件设置、更新、日志与反馈面板。
func (a *App) viewSettingsAndFeedback(c *ui.Context) {
	ui.Scroll(c).Fill().Padding(design.SpaceXL).Children(func() {
		ui.Column(c).FillWidth().Gap(design.SpaceXL).Children(func() {
			// 1. 软件更新
			a.kit.SectionTitle(c, "软件更新", "检查最新版本与自动安装更新（基于 GitHub Release 签名校验）", nil)
			a.kit.Card(c, func() {
				info := a.updates.info
				ui.Row(c).Gap(design.SpaceLG).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "当前版本：v"+Version).FontSize(design.FontBody).Bold().TextColor(a.kit.T.Ink)
					switch info.State {
					case UpdateChecking:
						ui.Text(c, "正在检查更新…").FontSize(design.FontSmall).TextColor(a.kit.T.Muted)
					case UpdateReady:
						ui.Text(c, "发现新版本："+info.Version).FontSize(design.FontSmall).TextColor(a.kit.T.Accent)
					case UpdateLatest:
						ui.Text(c, "已是最新版本").FontSize(design.FontSmall).TextColor(a.kit.T.Green)
					case UpdateInstalling:
						note := "正在下载安装更新…"
						if info.Error != "" {
							note = info.Error
						}
						ui.Text(c, note).FontSize(design.FontSmall).TextColor(a.kit.T.Accent)
					case UpdateInstalled:
						ui.Text(c, "新版本已安装完成，重启后生效").FontSize(design.FontSmall).TextColor(a.kit.T.Green)
					case UpdateFailed:
						ui.Text(c, "检查失败").FontSize(design.FontSmall).TextColor(a.kit.T.Accent)
					}
				})
				if info.Notes != "" {
					ui.Text(c, "更新内容：\n"+info.Notes).FontSize(design.FontSmall).TextColor(a.kit.T.Muted)
				}
				if info.State == UpdateFailed && info.Error != "" {
					a.kit.Notice(c, "更新检查未完成："+info.Error, "")
				}
				ui.Row(c).Gap(design.SpaceSM).Children(func() {
					check := kitButton(a.kit, c, kit.ButtonSecondary, "检查更新", info.State == UpdateChecking || info.State == UpdateInstalling)
					if check.Clicked() {
						go a.checkUpdate()
					}
					if info.State == UpdateReady {
						install := kitButton(a.kit, c, kit.ButtonPrimary, "立即安装更新", false)
						if install.Clicked() {
							go a.installUpdate()
						}
					}
				})
			})

			// 2. 运行日志
			a.kit.SectionTitle(c, "运行日志", "查看本地排障日志与文件位置", nil)
			a.kit.Card(c, func() {
				if path := a.LogFilePath(); path != "" {
					ui.Text(c, "日志文件路径："+path).FontSize(design.FontSmall).TextColor(a.kit.T.Muted)
				}
				ui.Row(c).Gap(design.SpaceSM).Children(func() {
					openBtn := kitButton(a.kit, c, kit.ButtonSecondary, "在文件管理器中打开", false)
					if openBtn.Clicked() {
						revealPath(a.LogFilePath())
					}
				})
				if tail := a.logTail(10); tail != "" {
					a.kit.Field(c, "近期记录（最后 10 行）", "", func() {
						ui.Text(c, tail).FontSize(design.FontTiny).TextColor(a.kit.T.Muted)
					})
				}
			})

			// 3. 问题与需求反馈
			a.viewFeedback(c)
		})
	})
}

// viewBackendStatus 画本地工作台的连接状态。
//
// 桌面版要能双击就用：工作台没起来时，界面会自己起一个并把过程写在这里。
func (a *App) viewBackendStatus(c *ui.Context) {
	status := a.backendState
	label := "本地工作台"
	switch status.State {
	case "starting":
	case "failed":
	}
	ui.Row(c).Gap(design.SpaceSM).AlignItems(ui.Center).Children(func() {
		dot := ui.Text(c, "●").FontSize(design.FontTiny)
		if status.State == "ready" {
			dot = dot.TextColor(a.kit.T.Green)
		} else if status.State == "failed" {
			dot = dot.TextColor(a.kit.T.Accent)
		} else {
			dot = dot.TextColor(a.kit.T.Muted)
		}
		ui.Text(c, label).FontSize(design.FontTiny).TextColor(a.kit.T.Muted)
	})
	if status.Note != "" {
		ui.Text(c, status.Note).FontSize(design.FontTiny).TextColor(a.kit.T.Muted)
	}
}

// viewConnections 画左栏底部的连接状态。
func (a *App) viewConnections(c *ui.Context) {
	ui.Column(c).FillWidth().Gap(design.SpaceSM).Children(func() {
		ui.Text(c, "连接状态").FontSize(design.FontTiny).TextColor(a.kit.T.Muted)
		if a.providers == nil {
			ui.Text(c, "正在连接本地工作台…").FontSize(design.FontSmall).TextColor(a.kit.T.Muted)
			return
		}
		for _, row := range []struct {
			name  string
			ready bool
			note  string
		}{
			{"视频", a.providers.Video.Ready, a.providers.Video.Note},
			{"图片", a.providers.Image.Ready, a.providers.Image.Note},
			{"配音", a.providers.Voice.Ready, a.providers.Voice.Note},
		} {
			ui.Row(c).Gap(design.SpaceSM).AlignItems(ui.Center).Children(func() {
				dot := ui.Text(c, "●").FontSize(design.FontTiny)
				if row.ready {
					dot = dot.TextColor(a.kit.T.Green)
				} else {
					dot = dot.TextColor(a.kit.T.Muted)
				}
				ui.Text(c, row.name).FontSize(design.FontSmall).TextColor(a.kit.T.Ink)
			})
		}
		if problems := a.providers.CustomChannels.Problems; len(problems) > 0 {
			ui.Text(c, "自定义通道配置有问题").FontSize(design.FontTiny).TextColor(a.kit.T.Accent)
			for _, p := range problems {
				ui.Text(c, "· "+p).FontSize(design.FontTiny).TextColor(a.kit.T.Muted)
			}
		}
	})
}

// viewMain 画右边的两个区域：生成器 + 预览/素材/历史。
func (a *App) viewMain(c *ui.Context) {
	ui.Column(c).Fill().Gap(0).Children(func() {
		a.viewToolbar(c)
		if a.mode == ModeSettings {
			a.viewSettingsAndFeedback(c)
			return
		}
		// Grow(1)：这一行拿走工具条以外的全部高度，
		// 不然生成器会按内容高度往下长，底部的「生成」按钮被挤出窗口。
		ui.Row(c).Fill().Grow(1).MinHeight(0).Gap(0).Children(func() {
			a.viewComposer(c)
			a.viewRight(c)
		})
	})
}

// viewToolbar 画顶部：模式切换 + 项目名。
func (a *App) viewToolbar(c *ui.Context) {
	ui.Row(c).FillWidth().Gap(design.SpaceLG).AlignItems(ui.Center).
		Background(a.kit.T.Surface).Border(1, a.kit.T.Line).Padding(design.SpaceLG, design.SpaceXL).Children(func() {
		labels := []string{"项目推荐", "口播", "视频", "图片", "配音", "导出", "设置与反馈"}
		for i, label := range labels {
			index := i
			active := int(a.mode) == index
			b := ui.Button(c, label)
			b = b.Radius(design.RadiusSM).Padding(design.SpaceXS, design.SpaceLG)
			if active {
				b = b.Background(a.kit.T.AccentSoft).TextColor(a.kit.T.Accent)
			} else {
				b = b.TextColor(a.kit.T.Muted)
			}
			if b.Clicked() {
				a.mode = Mode(index)
				a.ensureBackend()
				a.redraw()
			}
		}
		ui.Spacer(c)
		if p := a.CurrentProject(); p != nil {
			ui.Text(c, p.Name).FontSize(design.FontMedium).TextColor(a.kit.T.Ink)
		}
	})
}

// viewComposer 画中间的生成器。
//
// 表单项会随项目变多（分镜画面可能几十张），所以内容区要能滚，
// 否则「生成」按钮会被挤出窗口，用户根本点不到。
func (a *App) viewComposer(c *ui.Context) {
	ui.Column(c).Width(380).FillHeight().
		Background(a.kit.T.Paper).Border(1, a.kit.T.Line).Children(func() {
		ui.Scroll(c).FillWidth().Grow(1).MinHeight(0).Padding(design.SpaceXL).Children(func() {
			ui.Column(c).FillWidth().Gap(design.SpaceLG).Children(func() {
				if a.state == nil {
					ui.Text(c, "正在连接本地工作台…").FontSize(design.FontBody).TextColor(a.kit.T.Muted)
					return
				}
				if len(a.state.Projects) == 0 {
					ui.Text(c, "先新建一个项目").FontSize(design.FontBody).TextColor(a.kit.T.Muted)
					return
				}
				if a.mode == ModeRecommend {
					a.viewRecommendationComposer(c)
				} else if a.mode == ModeTalk {
					a.ensureTalkStyles()
					a.viewTalkComposer(c)
				} else {
					a.viewChannelPicker(c)
					a.viewModelPicker(c)
					a.viewPrompt(c)
				}
			})
		})
		// 提交条固定在底部：表单再长，「生成」也一直在眼前。
		ui.Column(c).FillWidth().Gap(design.SpaceSM).
			Background(a.kit.T.Surface).Border(1, a.kit.T.Line).
			Padding(design.SpaceLG, design.SpaceXL).Children(func() {
			a.viewSubmit(c)
		})
	})
}

// viewPrompt 画提示词与参数。
func (a *App) viewPrompt(c *ui.Context) {
	if a.mode == ModeExport {
		a.kit.Field(c, "合成说明", "按素材顺序合成，镜头时长由旁白决定", func() {
			a.kit.TextArea(c, &a.prompt, "选好素材后点下面的合成按钮", 70)
		})
		return
	}
	a.kit.Field(c, "提示词", "写清楚画面内容、镜头与主体", func() {
		a.kit.TextArea(c, &a.prompt, "例如：橘猫在窗台上看雨，晨光斜照，镜头缓慢推近", 80)
	})
	if a.mode == ModeVideo {
		ratios := []string{"9:16", "16:9", "1:1"}
		selected := a.ratio
		a.kit.Field(c, "画幅", "", func() {
			el := a.kit.Select(c, &selected, ratios, "")
			if el.Changed() {
				a.ratio = selected
				a.redraw()
			}
		})
	}
}

// viewSubmit 画提交按钮与提示。
func (a *App) viewSubmit(c *ui.Context) {
	ui.Spacer(c)
	if a.notice != "" {
		kind := "info"
		if a.noticeErr {
			kind = "error"
		}
		a.kit.Notice(c, a.notice, kind)
	}
	channel := a.CurrentChannel()
	disabled := a.busy.Load() || channel == nil || !channel.Ready || a.prompt == ""
	label := "生成"
	switch a.mode {
	case ModeRecommend:
		label = "生成推荐视频"
		disabled = a.busy.Load() || a.recTitle == "" || a.recCaptions == "" || a.repoURL == "" || len(a.recImages) == 0
	case ModeTalk:
		label = "生成口播视频"
		disabled = a.busy.Load() || a.talkTitle == "" || a.talkCaptions == "" || a.talkPersona == ""
	case ModeImage:
		label = "生成图片"
	case ModeSpeech:
		label = "生成配音"
	case ModeExport:
		label = "合成并导出"
	}
	if a.mode == ModeRecommend && channel != nil && !channel.Ready {
		a.kit.Hint(c, "配音通道未就绪："+channel.Note)
	}
	if a.mode == ModeTalk {
		a.kit.Hint(c, "逐句用本机 VoxFlow 配音，画面与合成都在本机执行，不产生模型费用。")
	}
	if channel != nil && !channel.Ready && a.mode != ModeRecommend && a.mode != ModeTalk {
		a.kit.Hint(c, "所选通道未就绪："+channel.Note)
	}
	button := kitButton(a.kit, c, kit.ButtonPrimary, label, disabled)
	if button.Clicked() {
		a.submit()
	}
}

// viewRight 画右侧：预览、素材库、制作闭环、任务历史。
func (a *App) viewRight(c *ui.Context) {
	ui.Column(c).Fill().Gap(design.SpaceLG).Padding(design.SpaceXL).Children(func() {
		ui.Scroll(c).FillWidth().Grow(1).MinHeight(0).Children(func() {
			ui.Column(c).FillWidth().Gap(design.SpaceLG).Children(func() {
				a.viewPreview(c)
				a.viewAssets(c)
				a.viewWorkflow(c)
				a.viewJobs(c)
			})
		})
	})
}

// viewJobs 画任务历史。
func (a *App) viewJobs(c *ui.Context) {
	jobs := a.ProjectJobs()
	a.kit.SectionTitle(c, "任务", fmt.Sprintf("%d 条", len(jobs)), nil)
	if len(jobs) == 0 {
		a.kit.Hint(c, "还没有生成记录。")
		return
	}
	ui.Column(c).FillWidth().Gap(design.SpaceSM).Children(func() {
		for _, job := range jobs {
			j := job
			status := j.Status
			kind := ""
			switch j.Status {
			case "succeeded":
				kind = "success"
			case "failed":
				kind = "brand"
			}
			ui.Column(c).FillWidth().Gap(design.SpaceXS).Background(a.kit.T.Surface).
				Border(1, a.kit.T.Line).Radius(design.RadiusSM).Padding(design.SpaceMD).Children(func() {
				ui.Row(c).Gap(design.SpaceMD).AlignItems(ui.Center).Children(func() {
					ui.Text(c, kindLabel(j.Input["kind"])).FontSize(design.FontBody).TextColor(a.kit.T.Ink)
					a.kit.Tag(c, statusLabel(status), kind)
					ui.Spacer(c)
					// 上游已经计费、只是没取回素材的，给一条取回路径
					if j.Upstream != nil && j.Upstream["taskId"] != nil && (status == "failed" || status == "interrupted") {
						b := ui.Button(c, "取回已生成的素材")
						b = b.TextColor(a.kit.T.Accent).FontSize(design.FontSmall)
						if b.Clicked() {
							a.resume(j.ID)
						}
					}
				})
				detail := j.Message
				if j.Error != "" {
					detail = j.Error
				}
				if detail != "" {
					ui.Text(c, detail).FontSize(design.FontTiny).TextColor(a.kit.T.Muted)
				}
			})
		}
	})
}

// newProject 新建项目。
func (a *App) newProject() {
	go func() {
		client := studioapi.New(a.opts.BaseURL)
		p, err := client.NewProject("新项目")
		if err != nil {
			a.fail(err.Error())
			return
		}
		a.mutate(func() {
			a.projectID = p.ID
		})
		a.refresh()
	}()
}

// submit 提交生成任务。
func (a *App) submit() {
	channel := a.CurrentChannel()
	if a.mode == ModeRecommend {
		a.submitRecommendation(channel)
		return
	}
	if a.mode == ModeTalk {
		a.submitTalk()
		return
	}
	if channel == nil {
		a.fail("没有可用的通道")
		return
	}
	if !channel.Ready {
		a.fail("所选通道未就绪")
		return
	}
	input := map[string]any{
		"projectId": a.projectID,
		"kind":      a.kind(),
		"prompt":    a.prompt,
		"backend":   a.backend,
		"model":     a.model,
		"profile":   a.profile,
	}
	if a.mode == ModeVideo {
		input["ratio"] = a.ratio
		input["duration"] = atoi(a.duration)
		if a.resolution != "" {
			input["resolution"] = a.resolution
		}
	}
	if a.mode == ModeSpeech {
		if channel.ID == "voxflow" {
			input["persona"] = a.voiceID
		} else {
			input["voiceId"] = a.voiceID
		}
	}
	if len(a.assetIDs) > 0 {
		input["assetIds"] = a.assetIDs
	}
	// busy 是原子量，直接设，不用排队 —— 排队要等下一帧，
	// 那一帧里按钮还是可点的，用户能连点两次提交。
	a.busy.Store(true)
	a.mutate(func() {
		a.notice = ""
	})

	go func() {
		client := studioapi.New(a.opts.BaseURL)
		job, err := client.SubmitJob(input)
		a.busy.Store(false)
		a.mutate(func() {
			if err != nil {
				a.notice = err.Error()
				a.noticeErr = true
				a.logf("提交生成任务失败（通道 %s，模型 %s）：%s", a.backend, a.model, err.Error())
			} else {
				a.notice = "已提交：" + job.ID
				a.noticeErr = false
				a.logf("已提交生成任务 %s（通道 %s，模型 %s）", job.ID, a.backend, a.model)
			}
		})
		a.refresh()
	}()
}

// submitRecommendation 提交一条项目推荐视频。
//
// 配乐有两种来源：默认曲库（选别名时先复制进项目）与项目里已有的音频。
// 复制这一步在后端做，来源与授权会一起记进素材，出片时可查。
func (a *App) submitRecommendation(channel *studioapi.Channel) {
	if a.recTitle == "" || a.recCaptions == "" {
		a.fail("标题和分镜口播都要填")
		return
	}
	if a.repoURL == "" {
		a.fail("先填 GitHub 仓库地址")
		return
	}
	if len(a.recImages) == 0 {
		a.fail("先「读取项目」或勾选至少一张分镜画面")
		return
	}
	if channel == nil {
		a.fail("没有可用的配音通道")
		return
	}
	if !channel.Ready {
		a.fail("所选配音通道未就绪：" + channel.Note)
		return
	}
	input := map[string]any{
		"projectId": a.projectID,
		"kind":      "recommendation",
		"prompt":    a.recCaptions,
		"title":     a.recTitle,
		"captions":  a.recCaptions,
		"repoUrl":   a.repoURL,
		"platform":  a.recPlatform,
		"ratio":     a.recRatio,
		"backend":   a.backend,
		"model":     a.model,
		"profile":   a.profile,
		"voiceId":   a.voiceID,
	}
	if channel.ID == "voxflow" {
		input["persona"] = a.voiceID
		delete(input, "voiceId")
	}
	// 项目里的图片按勾选顺序进片；后端要求至少一张
	var images []string
	if len(a.recImages) > 0 {
		images = a.recImages
	} else {
		for _, asset := range a.ProjectAssets() {
			if strings.HasPrefix(asset.Mime, "image/") {
				images = append(images, asset.ID)
			}
		}
	}
	if len(images) > 0 {
		input["assetIds"] = images
	}

	bgmChoice := a.recBGM
	projectID := a.projectID

	a.busy.Store(true)
	a.mutate(func() {
		a.notice = ""
	})

	go func() {
		client := studioapi.New(a.opts.BaseURL)
		if bgmChoice != "" {
			switch {
			case strings.HasPrefix(bgmChoice, "r2:"):
				alias := strings.TrimPrefix(bgmChoice, "r2:")
				asset, err := client.PullBGM(projectID, alias)
				if err != nil {
					a.busy.Store(false)
					a.mutate(func() {
						a.notice = "取配乐失败：" + err.Error()
						a.noticeErr = true
					})
					a.refresh()
					return
				}
				input["bgmId"] = asset.ID
			case strings.HasPrefix(bgmChoice, "asset:"):
				input["bgmId"] = strings.TrimPrefix(bgmChoice, "asset:")
			case strings.HasPrefix(bgmChoice, "bgm:"):
				input["bgmId"] = strings.TrimPrefix(bgmChoice, "bgm:")
			default:
				// 裸 alias：只能当库里的别名解析。
				// 解析失败就报错 —— 悄悄当成素材 ID 传下去，
				// 后端会拿一个不存在的 ID 去合成，用户只看到「合成失败」，
				// 根本不知道是自己选的配乐没拉到。
				asset, err := client.PullBGM(projectID, bgmChoice)
				if err != nil {
					a.busy.Store(false)
					a.mutate(func() {
						a.notice = "配乐「" + bgmChoice + "」拉取失败：" + err.Error()
						a.noticeErr = true
					})
					a.refresh()
					return
				}
				input["bgmId"] = asset.ID
			}
		}
		job, err := client.SubmitJob(input)
		a.busy.Store(false)
		a.mutate(func() {
			if err != nil {
				a.notice = err.Error()
				a.noticeErr = true
			} else {
				a.notice = "已提交：" + job.ID
				a.noticeErr = false
			}
		})
		a.refresh()
	}()
}

// resume 取回已在上游生成的产物。
func (a *App) resume(jobID string) {
	a.busy.Store(true)
	a.mutate(func() {
		a.notice = "正在向后端取回产物…"
		a.noticeErr = false
	})
	go func() {
		client := studioapi.New(a.opts.BaseURL)
		_, err := client.ResumeJob(jobID)
		a.busy.Store(false)
		a.mutate(func() {
			if err != nil {
				a.notice = err.Error()
				a.noticeErr = true
			} else {
				a.notice = "已取回素材，未重新提交生成请求。"
				a.noticeErr = false
			}
		})
		a.refresh()
	}()
}

// fail 记一条错误提示。
func (a *App) fail(message string) {
	a.busy.Store(false)
	a.mutate(func() {
		a.notice = message
		a.noticeErr = true
	})
}

// kind 返回任务类型。
func (a *App) kind() string {
	switch a.mode {
	case ModeRecommend:
		return "recommendation"
	case ModeTalk:
		return "talk"
	case ModeImage:
		return "image"
	case ModeSpeech:
		return "speech"
	case ModeExport:
		return "export"
	default:
		return "video"
	}
}

func kindLabel(kind any) string {
	switch kind {
	case "video":
		return "视频"
	case "image":
		return "图片"
	case "speech":
		return "配音"
	case "export":
		return "导出"
	case "recommendation":
		return "项目推荐视频"
	}
	return "生成"
}

func statusLabel(status string) string {
	switch status {
	case "succeeded":
		return "已完成"
	case "failed":
		return "失败"
	case "running":
		return "进行中"
	case "queued":
		return "排队中"
	case "interrupted":
		return "已中断"
	}
	return status
}

func atoi(s string) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return n
		}
		n = n*10 + int(r-'0')
	}
	return n
}
