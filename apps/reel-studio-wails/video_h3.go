package main

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// 出镜视频先只适配 MiniMax H3（用户有套餐）。
//
// 各家视频模型参数和能力不一样，不做「通用」抽象去迁就最小公约数：
// 出镜走 H3 的「多模态参考生视频」，规格按 H3 官方约束收口（中台
// _video-adapters.js 里的 minimax-v2 形状与这里对齐）：
//
//	· 时长 4–15 秒，分辨率 768P
//	· 参考图 ≤ 9，且与首帧/尾帧互斥 —— 出镜只用参考图
//	· 有参考图时画幅可选，给了就按给的出；可用 16:9/4:3/1:1/3:4/9:16/21:9
//	· 提示词 ≤ 7000 字符
//	· 走 MiniMax M Plan 套餐额度（Explore/Build 档含 H3，消耗按档位计），出镜不自动重试，免得一次点击跑多条
//	· 成片自带音轨 —— 配音/BGM 合成时以 reel-kit 的音轨为准
type videoProfile struct {
	Model       string
	MinDuration int
	MaxDuration int
	Ratios      []string
	MaxPrompt   int
}

var h3Profile = videoProfile{
	Model:       "MiniMax H3",
	MinDuration: 4,
	MaxDuration: 15,
	Ratios:      []string{"16:9", "4:3", "1:1", "3:4", "9:16", "21:9"},
	MaxPrompt:   7000,
}

// shotSpec 这一条镜头最终用的规格。
type shotSpec struct {
	Ratio    string
	Duration int
}

// resolve 定规格：用户填的优先，其次项目「出片规格」，最后兜底 16:9 / 上限时长。
// 时长夹到模型支持的区间内，不让中台收一个必然被拒的参数。
func (p videoProfile) resolve(ratio string, duration int, philosophy Philosophy) shotSpec {
	projRatio, projDur := projectOutputSpec(philosophy)
	spec := shotSpec{Ratio: strings.TrimSpace(ratio), Duration: duration}
	if !p.ratioOK(spec.Ratio) {
		spec.Ratio = ""
	}
	if spec.Ratio == "" && p.ratioOK(projRatio) {
		spec.Ratio = projRatio
	}
	if spec.Ratio == "" {
		spec.Ratio = "16:9"
	}
	if spec.Duration <= 0 {
		spec.Duration = projDur
	}
	if spec.Duration <= 0 {
		spec.Duration = p.MaxDuration
	}
	if spec.Duration < p.MinDuration {
		spec.Duration = p.MinDuration
	}
	if spec.Duration > p.MaxDuration {
		spec.Duration = p.MaxDuration
	}
	return spec
}

func (p videoProfile) ratioOK(r string) bool {
	for _, v := range p.Ratios {
		if v == r {
			return true
		}
	}
	return false
}

// projectOutputSpec 读项目哲学「出片规格」里的画幅与时长（如「16:9」「15 秒」）。
func projectOutputSpec(p Philosophy) (string, int) {
	spec, ok := p["出片规格"].(map[string]any)
	if !ok {
		return "", 0
	}
	ratio, _ := spec["画幅"].(string)
	dur := 0
	if s, ok := spec["时长"].(string); ok {
		digits := strings.TrimFunc(s, func(r rune) bool { return r < '0' || r > '9' })
		if i := strings.IndexFunc(digits, func(r rune) bool { return r < '0' || r > '9' }); i >= 0 {
			digits = digits[:i]
		}
		dur, _ = strconv.Atoi(digits)
	} else if f, ok := spec["时长"].(float64); ok {
		dur = int(f)
	}
	return strings.TrimSpace(ratio), dur
}

// h3ShotPrompt 按 H3 官方推荐的结构组织出镜提示词：
//
//	① 规格（时长 + 画幅）→ ② 参考图分工（图1、图2…）→ ③ 动作 → ④ 人物与装扮 → ⑤ 画面口径
//
// 人只写第 ③ 条时 H3 出片会散；参考图不点名分工，它不知道哪张锁脸、哪张锁兵器。
// 超过 7000 字时从末尾的画面口径开始截，动作与锁脸约束永远保留。
func h3ShotPrompt(c *Character, look, action string, spec shotSpec, refs []shotRef, philosophy string) string {
	head := []string{fmt.Sprintf("【规格】%d 秒，%s，单镜头，写实电影质感。", spec.Duration, spec.Ratio)}
	if len(refs) > 0 {
		lines := []string{}
		for i, r := range refs {
			label := r.Label
			if label == "" {
				label = c.Name + "的参考"
			}
			lines = append(lines, fmt.Sprintf("图%d：%s", i+1, label))
		}
		head = append(head, "【参考图分工】"+strings.Join(lines, "；")+"。参考图只用于锁定外观，画面里不得出现白底设定图、拼图或文字标注。")
	}
	head = append(head, "【动作】"+strings.TrimSpace(action))
	person := []string{}
	if look != "" && look != DefaultLook {
		person = append(person, "装扮："+look+"，只穿参考图中这一套，不得混入其他服装")
	}
	for _, t := range c.Traits {
		person = append(person, t.Key+"："+t.Value)
	}
	if len(person) > 0 {
		line := "【人物】" + c.Name + "。" + strings.Join(person, "；") + "。五官、发型、肤色、体型全程不得改变。"
		// 角色卡的外貌/服装字段写的是默认那一套（如「黑金交织战袍」）。选了别的装扮时，
		// 文字和参考图会打架 —— 明确告诉模型衣服以参考图为准，文字里的衣着描述作废。
		// 不改中台数据：同一张角色卡还要服务默认装扮。
		if look != "" && look != DefaultLook {
			line += "以上文字中的服装、披风、甲胄等衣着描述仅适用于默认装扮，本镜一律以「" + look + "」定妆参考图为准。"
		}
		head = append(head, line)
	} else {
		head = append(head, "【人物】"+c.Name+"，外貌以参考图为准，全程不得改变。")
	}
	out := strings.Join(head, "\n")
	if philosophy = strings.TrimSpace(philosophy); philosophy != "" {
		room := h3Profile.MaxPrompt - utf8.RuneCountInString(out) - 1
		if room > 0 {
			r := []rune(philosophy)
			if len(r) > room {
				r = r[:room]
			}
			out += "\n" + string(r)
		}
	}
	return out
}
