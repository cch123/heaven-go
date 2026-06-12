# Heaven Go 移植规范

本仓库是 Heaven Studio（Unity）游玩部分到 Go + Ebitengine 的移植。
以下三条是每个 minigame 移植的硬性验收标准：

## 1. 对照 Unity 原版，所有动画必须完整移植，不要有遗漏

每个游戏宣告"移植完成"前，必须做完整性审计：
- 游戏目录下全部 .anim 剪辑逐一核对是否被运行时驱动（含 idle/循环剪辑）；
- 每条曲线的 path 对场景树做解析验证（逐剪辑检查
  animatorRoot + "/" + path 是否存在于 scene.json）；
- 每个动画属性确认运行时支持：m_Color.*、m_Size.x/y、m_FlipX/Y、
  m_SortingOrder、m_IsActive/m_Enabled、sprite 换帧、循环标志；
- ParticleSystem、AnimatorController 默认状态、同名 .anim 文件
  （需命名空间 key，如 Girl/Bop）逐项过。

历史教训：somen 的 WaterFlow 绑错节点导致流水静止；trickClass 的
Girl/Player 同名 Bop 互相覆盖导致头身分家；光束因 m_Size 不支持整体缺失。

## 2. 原版靠 Unity 特性实现而 Ebitengine 不支持的，必须自己实现

不允许静默降级或跳过。已有先例（pattern 可复用）：
- ParticleSystem → 按 prefab 序列化参数手写粒子（games/somen 水花）；
- SpriteRenderer sliced/tiled 的 m_Size 拉伸 → 九宫格渲染（border 来自图集 meta）；
- 动画驱动的 sortingOrder → 绘制顺序每帧重排；
- NaughtyBezierCurves → kart.EvalBezier（按近似弧长加权分段）。

后续遇到 Timeline、材质 UV 滚动、后处理等同类问题，同样自行实现。

## 3. 交互和特效要和原版 100% 一致

- 判定窗口、输入冷却（如 playerCanDodge）、事件拍位语义一律以 C# 源码逐行对照为准；
- 音效细节（音量、多重音、随机变调、MultiSound 拍位偏移）以 C# 调用参数为准；
- 特效时序/层级/枢轴以 prefab 与图集 meta 的序列化数据为准
  （alignment 枚举按全表映射，不能默认中心）；
- 不做"视觉近似就行"的妥协；暂时做不到的必须在代码注释 + README
  的"已知简化"清单中显式标注，并尽快清掉。

## 参考路径

- Unity 源工程：/Users/xargin/Downloads/HeavenStudio-master/
- 官方关卡：/Users/xargin/Downloads/Heaven Studio.app/Contents/Resources/Data/StreamingAssets/Library Pack-In/Heaven Studio Pack-In Levels/
- 新游戏移植流程见 README.md；提取资产不入库，先跑 go run ./cmd/extract -game <id>。
