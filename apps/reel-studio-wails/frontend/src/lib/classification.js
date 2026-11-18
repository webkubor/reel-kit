// 素材分类规则（与后端 character_cache.go 的同名函数一一对应）。
//
// 单独成文件是为了能被测试直接读到：前端角色详情按这套规则分组，
// 后端角色库列表按 Go 里的同名函数算。两边不一致时，同一张图会在
// 列表和详情里落到不同分区。

// 兵器与道具。
//
// 带三视图/定妆/设定的先归定妆，否则「顾栖月-三视图-顾栖月_v1_玉笛」
// 会被当成兵器。
export const isWeapon = (name) =>
  /武器|兵器|面具|枪|剑|刀|弓|笛|锤|鞭|匕|枷|面甲|道具/.test(name || '') &&
  !/三视图|定妆|设定|turnaround/.test(name || '')

// 真人参考：现实照片质感或来源标注「真人/脸部参考」，永远不进角色形象区。
// 发型/妆造参考也是真人照片（陆听潮-纯黑发参考），同样要排除。
export const isRealFace = (name) =>
  /真人参考|脸部参考|真人|脸参考|发参考|发型参考|妆造参考|妆参考|face|photo/i.test(name || '')

// 定妆图（三视图/定妆/设定）：角色形象本身。
export const isDesignSheet = (name) => /三视图|定妆|设定|turnaround/i.test(name || '')

// 角色头像。
export const isAvatar = (name) => /头像|avatar/i.test(name || '') && !isRealFace(name)
