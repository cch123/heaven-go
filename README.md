# Heaven Go

Heaven Studio 游玩部分（play-only，无编辑器）的 Go + Ebitengine 移植。引擎层（判定/调度/切游戏/HUD）与游戏模块解耦，资产经导出管线从 Heaven Studio Unity 工程提取，支持加载任意用户 `.riq` 谱面。

**已注册可玩模块**以 `go run ./cmd/officialgames` 输出为准；当前包含 Air Rally、Basketball Girls、Blue Bear、Board Meeting、Bouncy Road、Catchy Tune、Chameleon、Cheer Readers、Clap Trap、Coin Toss、Dog Ninja、Drumming Practice、Fork Lifter、Frog Princess、Glee Club、Kitties!、Lockstep、Marching Orders、Meat Grinder、Mr. Upbeat、Munchy Monk、Rhythm Sōmen、See-Saw、Sneaky Spirits、Space Dance、Tambourine、Tap Trial、The Clappy Trio、Totem Climb、Tram & Pauline、Trick on the Class、Tunnel、Wizard's Waltz（karateman 仍走旧 demo 路径）。
谱面中未移植的 minigame 显示占位画面，乐曲与其余游戏照常进行。

## 运行

```sh
go run ./cmd/extract -game rhythmSomen   # 各游戏资产提取（一次性）
go run ./cmd/extract -game trickClass
go run ./cmd/extract -game meatGrinder
go run ./cmd/extract -game totemClimb
go run ./cmd/extract -game airRally
go run ./cmd/extract -game basketballGirls
go run ./cmd/extract -game boardMeeting
go run ./cmd/extract -game bouncyRoad
go run ./cmd/extract -game catchyTune
go run ./cmd/extract -game chameleon
go run ./cmd/extract -game clapTrap
go run ./cmd/extract -game clappyTrio
go run ./cmd/extract -game coinToss
go run ./cmd/extract -game dogNinja
go run ./cmd/extract -game drummingPractice
go run ./cmd/extract -game forkLifter
go run ./cmd/extract -game frogPrincess
go run ./cmd/extract -game gleeClub
go run ./cmd/extract -game mrUpbeat
go run ./cmd/extract -game tramAndPauline
go run ./cmd/extract -game tunnel
go run ./cmd/extract -game wizardsWaltz
go run ./cmd/extract -game common      # 公共音效（countIn 计数音、miss/nearMiss）
go run ./cmd/officialgames             # 官方游戏移植矩阵：loader / Pack-In / 注册 / 提取状态
go run . -riq "levels/Rhythm Somen.riq"        # 内置已移植关卡
go run . -riq "levels/Trick on the Class.riq"
go run . -riq "levels/Meat Grinder.riq"
go run . -riq "levels/Totem Climb.riq"
go run .                                 # 启动关卡选择 UI；也可把任意 .riq 拖进窗口
go run . -fullscreen                     # 启动即全屏
go run . -riq "levels/Meat Grinder.riq" -autoplay   # 完美自动打击（调试）
go run ./cmd/verify -riq "levels/Meat Grinder.riq" -beats "1,36.6" -out /tmp/mg  # 录制验证：抓帧 + 判定计数
```

`levels/` 收录已完成移植验证的官方 Pack-In 关卡。

操作：`Space` / `J` / 鼠标左键（totemClimb 高跳需按住 2 拍后松开）；`F11` / `Alt+Enter`（macOS 也可 `⌘+Enter` / `⌃⌘F`）切换全屏；`Tab` 调试叠层；结算 epilogue 后 `Enter` / 点击回选关，`R` 重开当前关；`Esc` 退出。可随时把新的 `.riq` 拖入窗口切换关卡。

## 移植一个新 minigame 的流程

1. 在 `cmd/extract/scene.go` 的 `sceneSpecs` 里登记游戏（prefab 名、角色字段、可选的引用数组/字符串表/Bezier 曲线/对象模板/音效序列），运行提取。
2. 在 `games/<id>/` 实现 `engine.Module` 接口：`OnEvent` 把谱面实体翻译成 `ctx.At`（时间轴动作）、`ctx.ScheduleInput`（判定）、`ctx.Play`（动画）、`ctx.PlaySeq`（音效序列）。
3. `main.go` 中 `engine.Register` 注册。判定窗口、时机条、技能星、flash、切游戏、结算均由 engine 处理。

`cmd/extract/scene_specs_official.go` 给官方 bundled prefab 提供了基础提取入口
（scene/anims/controllers）。这些条目只代表资产可提取，不代表玩法已完成；注册到
`engine.Register` 前仍必须按 `AGENTS.md` 对照 Unity C#、prefab、`.anim`、图集 meta
完成交互、音效、特效和动画审计。

## 架构

```
Unity 工程 (HeavenStudio-master)              运行时
─────────────────────────────────            ─────────────────────────────
karateman_main.png(.meta) ─┐
karateman.prefab (Joe子树) ─┼─ cmd/extract ─> assets/karateman/
anime/karateman/*.anim    ─┤   (Unity YAML     ├─ atlas.png + sprites.json ─┐
Sounds/*.ogg|wav          ─┘    解析)          ├─ rig.json / anims.json     ├─> kart（骨架渲染+动画采样）
                                               └─ sounds/*                 ─┘        │
demo.riq (ZIP)                                                                       │
 ├─ Charts/chart0.json ──> riq.Load ──> Beatmap{tempo map, entities}                 │
 └─ Music/song0.wav ──解码──> audio.Player ──> conductor（采样时钟+平滑）──> 判定/渲染
```

| 包 | 职责 | 对应 Heaven Studio 模块 |
|---|---|---|
| `unityyaml` | Unity 多文档 YAML 解析（`!u!` 标记、stripped 文档、Infinity 斜率、UTF-8 BOM） | —（Unity 序列化层） |
| `cmd/extract` | 资产导出管线，两种模式：单骨架（karateman）与整场景（`-game rhythmSomen`：全 prefab 树 + 多图集 + 全部剪辑 + 脚本字段→节点绑定 roles.json） | —（即移植方案中"必须先做的资产管线"） |
| `kmdata` | 导出物的中间格式（JSON schema） | — |
| `kart` | 运行时：图集子图、仿射骨架/场景合成、曲线采样（Hermite + 阶跃换帧 + FlipX + m_IsActive 层级传播 + m_Color + CellAnime `_Color/_AddColor`）；`SceneInst` 支持多 Animator 并行与同根多层剪辑，剪辑时间 = 拍数 × timeScale；AnimatorController 状态机（状态名→剪辑映射、退出转换 + bool 条件，meatGrinder 的 tackMeated 满脸肉循环）；DoNormalizedAnimation（按归一化时间采样）；TMP 世界文本（动态字体 glyph 表为空 → 用源 OTF 排版为动态切片，meatGrinder 的 GRINDER 铭牌与 changeText）；模块注入的动态绘制项（模板实例/手写粒子）与场景节点统一 (layer, order, z) 排序 | Animator(Controller) / TextMeshPro / SpriteRenderer |
| `riq` | `.riq` 加载（v1 `remix.json` 与 v2 `Charts/chart0.json` 双布局）、tempo map、关卡元数据 | Jukebox（RiqFileHandler / RiqBeatmap） |
| `conductor` | 采样时钟：以 `player.Position()` 为权威，单调时钟外推 + 漂移校正 | Conductor.cs（`dspTime` + `absTime` 平滑） |
| `synth` | 程序化 PCM 合成（karateman demo 音轨鼓点） | —（替代版权音乐） |
| `cmd/genriq` | 生成 v2 布局测试谱面 | —（替代关卡编辑器） |
| `somen.go` | Rhythm Sōmen 完整玩法：吊臂时序、判定（ace ±10ms / just ±50ms / ng ±100ms）、bop 区间、slurp 打断逻辑、技能星、flash、结算 | RhythmSomen.cs + GameManager 事件调度 |
| `main.go` | 谱面路由（按实体推断 minigame）+ Karate Man 旧 demo | GameManager |

## 设计要点

- **资产管线**：`cmd/extract` 演示了移植方案的核心环节——把 Unity 序列化资产（图集 `.meta` 切片、prefab 骨架、`.anim` 关键帧曲线）转成引擎无关的 JSON。Joe 的出拳（Jab）/律动（Beat）动画直接来自原工程的曲线数据，含 Hermite 切线与阶跃帧语义。
- **时钟**：复刻 Conductor.cs 的策略——歌曲时间以音频播放位置为锚，每帧用单调时钟外推，偏差小于 50ms 时按 8%/帧 缓收敛，大于则直接重锚。
- **输入采样**：`ebiten.SetTPS(240)`，把逻辑帧对输入的量化误差从 60Hz 的 ±8ms 压到约 ±2ms。
- **tempo map**：分段线性的节拍↔时间双向映射，支持谱面中途变速（demo 谱面第 48 拍 120→140 BPM）。
- **轨迹**：复刻原版 `KarateManPot.ProgressToFlyPosition` 的飞行公式——参数（判定点、地面、起点偏移、`HitPositionOffset`、`ItemSlipRt`）由提取器从 prefab 序列化字段导出为 `stage.json`。事件 beat 为抛出拍，判定在 beat+1，全程 2 拍；y 走归一化抛物线（判定时刻恰过拳头），z 跨度 ±8 配合透视近似（相机距离 10，缩放 `s = 10/(10+z)`），罐子从近景右下角飞入、判定后缩小着飞向远处；入场自转 125°/拍。
- **音效**：抛出 `objectOut.ogg`、命中 `potHit.ogg`/`punchKickHit1.ogg`、空挥 `swingNoHit.wav`、漏拍 `karate_through.wav`，均为原版资产，启动时解码为裸 PCM 实现零延迟触发。
- **测试**：`go test ./kart/` 在无窗口环境下验证骨架采样（变换有限性、Jab 换帧、包围盒合理性）。

## 已知简化（demo 范围外）

karateman 旧 demo 路径：
- 未实现：swing、`karateman/hit` 的 `type` 参数（石头/灯泡等变体）、表情/特殊镜头。
- 背景为纯色 + 节拍脉冲（原版背景色 `#fbca3e`），未接入背景贴图层与后处理。

engine 路径（rhythmSomen / trickClass / meatGrinder / totemClimb / airRally 等）：
- 启动页 Library 选择流程已接入原版背景、unplayed 关卡边框、`.riq` 自带
  `LibraryLevelIcon` 和关卡元数据；原版排序/搜索/收藏、已游玩评级边框与
  勋章状态尚未接入，当前固定按 `levels/*.riq` 文件名排序。
- AnimatorController 转换的 duration（交叉淡入）按立即切换处理；当前唯一非零用例
  BossCall→BossCallIdle 已逐曲线验证源末帧与目标姿态一致，视觉无差。
- 缓动函数全表实现（engine/ease.go，HS Ease 枚举 0..43 含 Expo/Circ/Bounce/
  Back/Elastic/OutIn/InstantOut）。
- TMP 文本用源 OTF 排版（原版为 SDF 渲染），字体/字号/颜色/对齐一致，字形边缘
  抗锯齿方式不同；只实现 Center/Middle 对齐（其他对齐出现时显式报错）。
- vfx/display textbox 已按原版 TextboxAnchor、TextboxPrefab 尺寸、文本矩形、
  富文本 align 与自动换行绘制；框体暂用等价白底黑边圆角面板替代 TextboxSDF
  shader 的四角 sliced 渲染，边缘抗锯齿细节不同。
- 多游戏 remix 中，未激活游戏的 interval 调度音效仍会播放（与 C# MultiSound 全局
  播放行为一致）；其动画动作也会执行但不可见。
- C# 的 `BossAnim.SetBool("bossAnnoyed")` 在原版 controller 中无任何转换引用（死调
  用），未移植——bop 的不悦表现走 `bossAnnoyed ? BossMiss : Bop` 分支，与原版一致。
- countIn 计数音实现 Normal/Alt/Cowbell 音色；GBA/DS 变体音色目录未提取（出现时
  回退 Normal 并打日志）。
- airRally：rally、ba-bum-bum-bum、catch、enter、set distance、forward、
  4beat/8beat/count voice、rainbow、spawnBird、day、cloud、snowflake、tree、
  islandSpeed 均已接入运行时；动态天气/鸟群/树/彩虹为按 AirRally.cs 与 prefab
  序列化参数手写的非场景节点实例。
- Judgement 结算页已接入 Heaven Studio 的评分阈值/分类评价消息、rank 标志图、
  默认 epilogue 图与结算音效/jingle/循环音乐；原版 `JudgementOpen.playable`
  的逐信号精确定时尚未完整移植，当前用等价状态推进。
- kitties：roll 成功后的 spinnya 循环音未实现随机变调（±5%，循环重采样
  不支持）；音量 0.85 与起止时序一致。
- cheerReaders：字幕（toggleCaption 启用路径）未实现——官方非 PRACTICE 关
  均为 version=0 旧谱面且无 toggleCaption 块，按 CheckCaptions 语义自动禁
  用，行为一致；若自制谱启用会打日志提示。yay 纸花粒子为等价手写实现
  （白/黑方片爆散），未逐参数复刻 ParticleSystem。
- lockstep：人群渲染将原版"3 台正交相机 → RenderTexture → 平铺 quad"等价
  实现为同尺度无限棋盘格直绘（几何/相位/缩放一致）。
- ppe 后处理（engine/postfx.go）：colorGrading/vignette/cabb/lensD/pixelQuad 按
  PPv2 / X-PostProcessing 公式逐式复刻；bloom 用 1/4 分辨率两轮高斯近似 PPv2 的
  mip 金字塔；grain 用 hash 噪声近似烘焙噪声纹理；anamorphicRatio、technicolor
  未实现（全部官方关卡未使用）；flash/HUD 不参与后处理（对应编辑器叠层）。
- totemClimb 柱子网格按"可见窗口直算"替代 Unity 的 12×3 环形回收（视觉等价）；
  原版 pillar (2) 不带下延段的细节按统一模板绘制（重叠区域同贴图，视觉等价）。
- totemClimb 高跳保持期的提前松手惩罚（UnHold + ScoreMiss + 重按回握）按
  HoldCo 轮询语义实现；空抬起（无判定窗）不计 whiff，与 C# 一致。

- 资产版权：图集与音效来自 Heaven Studio 工程，仅限本地验证使用。
