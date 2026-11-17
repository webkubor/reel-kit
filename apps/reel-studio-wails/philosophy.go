package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

// 项目创作哲学的读取与编译。
//
// 为什么哲学要「编译」而不是「拼接」：项目哲学是结构化数据（画面哲学/
// 动作美学/镜头语言/交付红线），每个项目长出的维度不一样。直接 JSON.dump
// 进提示词既浪费 token 又让模型抓不到重点。编译成**按重要性排序**的一段话：
// 禁令在前（必须遵守），美学在后（尽量体现），数值规格最后。
//
// 哲学与角色卡是两层约束，缺一不可：
//   角色卡管「这个人长什么样」（五官/发型/体型），
//   项目哲学管「这类画面长什么样」（影调/运镜/动作物理）。
// 两层一起编译进提示词，才既有对的人，又有对的味。

// Philosophy 项目创作哲学（中台 workspaces.philosophy）。
//
// 字段按项目自定义，这里只约定已知维度的类型；未知键原样进编译，
// 不做白名单过滤 —— 哲学的维度是各项目自己长出来的。
type Philosophy map[string]any

// WorkspacePhilosophy 从工作区解析出的哲学 + 出片数值规格。
type WorkspacePhilosophy struct {
	Name       string
	Philosophy Philosophy
}

// PhilosophyFragment 编译出的提示词片段。
type PhilosophyFragment struct {
	Text     string
	Redlines []string
}

// GetWorkspacePhilosophy 读中台工作区的创作哲学。
//
// 走 HTTP 而不是 CLI：philosophy 是只读字段，museav CLI 还没暴露它。
// 中台约定「计费动作只在用户明确触发的那一步」——读哲学是纯读，不计费。
func (m *MuseavClient) GetWorkspacePhilosophy(ctx context.Context, workspace string) (*WorkspacePhilosophy, error) {
	token, base, err := museavCredentials()
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/workspaces", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("读取中台工作区失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("读取中台工作区失败：HTTP %d", resp.StatusCode)
	}
	var rows []workspaceRow
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return nil, err
	}
	return pickWorkspacePhilosophy(rows, workspace)
}

type workspaceRow struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Philosophy Philosophy `json:"philosophy"`
}

// pickWorkspacePhilosophy 按 id 或名字挑工作区。
//
// 不给工作区就报错，绝不退回「第一个」：账号下有多个项目时，第一个
// 往往是「默认项目」，武侠角色会被套上别的片子的口径 —— 宁可没哲学，
// 也不要错哲学。
func pickWorkspacePhilosophy(rows []workspaceRow, workspace string) (*WorkspacePhilosophy, error) {
	workspace = strings.TrimSpace(workspace)
	if workspace == "" {
		return nil, errors.New("未指定中台工作区，不读创作哲学")
	}
	for i := range rows {
		if rows[i].ID == workspace || rows[i].Name == workspace {
			return &WorkspacePhilosophy{Name: rows[i].Name, Philosophy: rows[i].Philosophy}, nil
		}
	}
	return nil, fmt.Errorf("中台没有工作区「%s」", workspace)
}

// Compile 把项目哲学编译成提示词片段。
//
// 顺序是故意的：禁令 → 美学 → 规格。模型对开头的约束服从度最高，
// 「不能做什么」比「尽量做什么」更需要置顶。
func (p Philosophy) Compile() PhilosophyFragment {
	if len(p) == 0 {
		return PhilosophyFragment{}
	}
	var b strings.Builder
	redlines := flattenStringList(p["交付红线"])
	if len(redlines) > 0 {
		b.WriteString("【交付红线】" + strings.Join(redlines, "；") + "。")
	}
	for _, key := range []string{"叙事基调", "画面哲学", "动作美学", "镜头语言", "环境音频"} {
		if v, ok := p[key]; ok {
			if s := flattenValue(v, 0); s != "" {
				b.WriteString("【" + key + "】" + s + "。")
			}
		}
	}
	// 数值规格放最后：它是「尽量体现」，不是「必须遵守」。
	if v, ok := p["出片规格"]; ok {
		if s := flattenValue(v, 0); s != "" {
			b.WriteString("【出片规格】" + s + "。")
		}
	}
	// 未列名的键按名字排序补上，不丢数据。
	known := map[string]bool{
		"叙事基调": true, "画面哲学": true, "动作美学": true,
		"镜头语言": true, "环境音频": true, "出片规格": true, "交付红线": true,
	}
	rest := []string{}
	for k := range p {
		if !known[k] {
			rest = append(rest, k)
		}
	}
	sort.Strings(rest)
	for _, k := range rest {
		if s := flattenValue(p[k], 0); s != "" {
			b.WriteString("【" + k + "】" + s + "。")
		}
	}
	return PhilosophyFragment{Text: b.String(), Redlines: redlines}
}

// shotKeys 单镜头出镜用得上的维度。世界观、配音、技能方法论、三视图 SOP
// 是写剧本/做角色的口径，塞进视频提示词只会稀释约束、让模型抓不住重点。
var shotKeys = []string{"交付红线", "叙事基调", "画面哲学", "动作美学", "镜头语言", "出片规格"}

// CompileShot 编译单镜头出镜的哲学片段。
//
// 神兵谱只留总纲和这个角色自己的兵器：把十四把神兵全写进去，
// 模型会把别人的枪画到这个人手里。weapon 是角色卡里的「武器」字段。
func (p Philosophy) CompileShot(weapon string) string {
	if len(p) == 0 {
		return ""
	}
	sub := Philosophy{}
	for _, k := range shotKeys {
		if v, ok := p[k]; ok {
			sub[k] = v
		}
	}
	text := sub.Compile().Text
	if armory, ok := p["神兵谱"].(map[string]any); ok {
		parts := []string{}
		if s := flattenValue(armory["总纲"], 0); s != "" {
			parts = append(parts, s)
		}
		names := make([]string, 0, len(armory))
		for k := range armory {
			names = append(names, k)
		}
		sort.Strings(names)
		for _, k := range names {
			if k != "总纲" && weapon != "" && strings.Contains(weapon, k) {
				if s := flattenValue(armory[k], 0); s != "" {
					parts = append(parts, k+"："+s)
				}
			}
		}
		if len(parts) > 0 {
			text += "【神兵】" + strings.Join(parts, "；") + "。"
		}
	}
	return text
}

// flattenValue 把嵌套的哲学结构压成一段可读文本。
// 深度限 4 层：哲学数据是人写的，正常三四层就到底了。
func flattenValue(v any, depth int) string {
	if depth > 4 {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case []any:
		parts := []string{}
		for _, item := range t {
			if s := flattenValue(item, depth+1); s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, "；")
	case map[string]any:
		parts := []string{}
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if s := flattenValue(t[k], depth+1); s != "" {
				parts = append(parts, k+"："+s)
			}
		}
		return strings.Join(parts, "；")
	case bool:
		if t {
			return "是"
		}
		return "否"
	case float64:
		return strings.TrimSuffix(strings.TrimSuffix(fmt.Sprintf("%f", t), "0"), ".")
	}
	return ""
}

func flattenStringList(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := []string{}
	for _, item := range arr {
		if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
			out = append(out, strings.TrimSpace(s))
		}
	}
	return out
}

// museavCredentials 读中台凭据（~/.museav.json）。
func museavCredentials() (token, baseURL string, err error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", err
	}
	data, err := os.ReadFile(home + "/.museav.json")
	if err != nil {
		return "", "", errors.New("找不到中台登录态 ~/.museav.json，请先 museav login")
	}
	var cfg struct {
		BaseURL string `json:"baseUrl"`
		Token   string `json:"token"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return "", "", err
	}
	if cfg.Token == "" || cfg.BaseURL == "" {
		return "", "", errors.New("中台登录态不完整，请重新 museav login")
	}
	return cfg.Token, strings.TrimRight(cfg.BaseURL, "/"), nil
}

// philosophyText 读指定工作区的哲学并编译成一段提示词。
//
// 取不到（没登录 / 没指定工作区 / 工作区没写哲学）就返回空串：
// 本地功能不该被中台可用性绑架。
func (e *Engine) philosophyText(ctx context.Context, workspace string) string {
	if strings.TrimSpace(workspace) == "" {
		return ""
	}
	ws, err := (&MuseavClient{}).GetWorkspacePhilosophy(ctx, workspace)
	if err != nil || ws == nil {
		return ""
	}
	return ws.Philosophy.Compile().Text
}

// shotPhilosophyText 读指定工作区的哲学，按单镜头口径编译。
func (e *Engine) shotPhilosophyText(ctx context.Context, workspace, weapon string) string {
	if strings.TrimSpace(workspace) == "" {
		return ""
	}
	ws, err := (&MuseavClient{}).GetWorkspacePhilosophy(ctx, workspace)
	if err != nil || ws == nil {
		return ""
	}
	return ws.Philosophy.CompileShot(weapon)
}

// systemInstruction 组装分镜生成的系统提示词。
//
// 工作区取自任务入参 workspace；没给就只用裸指令，不猜项目。
func (e *Engine) systemInstruction(ctx context.Context, job *Job) string {
	workspace := ""
	if job != nil && job.Input != nil {
		workspace, _ = job.Input["workspace"].(string)
	}
	return withPhilosophy(storyboardInstruction, e.philosophyText(ctx, workspace))
}

func withPhilosophy(base, philosophy string) string {
	if philosophy == "" {
		return base
	}
	return base + "\n\n本片的创作哲学（画面与动作的口径，分镜必须体现）：" + philosophy
}
