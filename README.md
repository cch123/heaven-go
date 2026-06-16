# Heaven Go

Heaven Studio 游玩部分（play-only，无编辑器）的 Go + Ebitengine 移植。引擎层（判定/调度/切游戏/HUD）与游戏模块解耦，资产经导出管线从 Heaven Studio Unity 工程提取，支持加载任意用户 `.riq` 谱面。

**已注册可玩模块**以 `go run ./cmd/officialgames` 输出为准；当前包含 Air Rally、Basketball Girls、Blue Bear、Board Meeting、Bouncy Road、Built to Scale DS、Catchy Tune、Chameleon、Cheer Readers、Clap Trap、Coin Toss、Crop Stomp、Dog Ninja、Drumming Practice、Fireworks、Fork Lifter、Frog Princess、Glee Club、Karate Man、Kitties!、Lockstep、Marching Orders、Meat Grinder、Mr. Upbeat、Munchy Monk、Rhythm Sōmen、See-Saw、Sneaky Spirits、Spaceball、Space Dance、Tambourine、Tap Trial、Tap Troupe、The Clappy Trio、Totem Climb、Tram & Pauline、Trick on the Class、Tunnel、Wizard's Waltz。
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
go run ./cmd/extract -game cropStomp
go run ./cmd/extract -game dogNinja
go run ./cmd/extract -game drummingPractice
go run ./cmd/extract -game fireworks
go run ./cmd/extract -game flipperFlop
go run ./cmd/extract -game forkLifter
go run ./cmd/extract -game frogPrincess
go run ./cmd/extract -game gleeClub
go run ./cmd/extract -game mrUpbeat
go run ./cmd/extract -game spaceball
go run ./cmd/extract -game splashdown
go run ./cmd/extract -game tapTroupe
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
| `cmd/extract` | 资产导出管线，两种模式：单骨架（karateman）与整场景（`-game rhythmSomen`：全 prefab 树 + 多图集 + 全部剪辑 + 脚本字段→节点绑定 roles.json；MeshFilter/MeshRenderer/SkinnedMeshRenderer、材质贴图属性、可用贴图文件和 imported FBX Geometry 顶点/UV/三角导出到 meshes.json） | —（即移植方案中"必须先做的资产管线"） |
| `kmdata` | 导出物的中间格式（JSON schema） | — |
| `kart` | 运行时：图集子图、仿射骨架/场景合成、曲线采样（Hermite + 阶跃换帧 + FlipX + m_IsActive 层级传播 + m_Color + CellAnime `_Color/_AddColor`）；`SceneInst` 支持多 Animator 并行与同根多层剪辑，剪辑时间 = 拍数 × timeScale；AnimatorController 状态机（状态名→剪辑映射、退出转换 + bool 条件，meatGrinder 的 tackMeated 满脸肉循环）；DoNormalizedAnimation（按归一化时间采样）；TMP 世界文本（动态字体 glyph 表为空 → 用源 OTF 排版为动态切片，meatGrinder 的 GRINDER 铭牌与 changeText）；加载 `meshes.json` 的 MeshRenderer/材质绑定数据并绘制 Unity 内置 Plane/Cube 类 mesh footprint 与单 Geometry imported FBX MeshRenderer（含 `_MainTex` UV 采样，源贴图可用时）；模块注入的动态绘制项（模板实例/手写粒子）与场景节点统一 (layer, order, z) 排序 | Animator(Controller) / TextMeshPro / SpriteRenderer / MeshRenderer |
| `riq` | `.riq` 加载（v1 `remix.json` 与 v2 `Charts/chart0.json` 双布局）、tempo map、关卡元数据 | Jukebox（RiqFileHandler / RiqBeatmap） |
| `conductor` | 采样时钟：以单调时钟平滑推进，`player.Position()` 只做粗同步锚，避免音频缓冲块台阶传到动画 | Conductor.cs（`dspTime` + `absTime` 平滑） |
| `synth` | 程序化 PCM 合成（karateman demo 音轨鼓点） | —（替代版权音乐） |
| `cmd/genriq` | 生成 v2 布局测试谱面 | —（替代关卡编辑器） |
| `somen.go` | Rhythm Sōmen 完整玩法：吊臂时序、判定（ace ±10ms / just ±50ms / ng ±100ms）、bop 区间、slurp 打断逻辑、技能星、flash、结算 | RhythmSomen.cs + GameManager 事件调度 |
| `main.go` | 启动参数、注册模块、创建 engine.App | GameManager |

## 设计要点

- **资产管线**：`cmd/extract` 演示了移植方案的核心环节——把 Unity 序列化资产（图集 `.meta` 切片、prefab 骨架、`.anim` 关键帧曲线）转成引擎无关的 JSON。Joe 的出拳（Jab）/律动（Beat）动画直接来自原工程的曲线数据，含 Hermite 切线与阶跃帧语义。
- **时钟**：复刻 Conductor.cs 的策略——歌曲时间以单调时钟平滑推进，音频播放位置只作为粗同步锚；偏差在死区内不校正，超过死区才半步收敛，避免 `audio.Player.Position()` 的缓冲块台阶造成所有动画抖动。
- **输入采样**：`ebiten.SetTPS(240)`，把逻辑帧对输入的量化误差从 60Hz 的 ±8ms 压到约 ±2ms。
- **tempo map**：分段线性的节拍↔时间双向映射，支持谱面中途变速（demo 谱面第 48 拍 120→140 BPM）。
- **轨迹**：复刻原版 `KarateManPot.ProgressToFlyPosition` 的飞行公式——参数（判定点、地面、起点偏移、`HitPositionOffset`、`ItemSlipRt`）由提取器从 prefab 序列化字段导出为 `stage.json`。事件 beat 为抛出拍，判定在 beat+1，全程 2 拍；y 走归一化抛物线（判定时刻恰过拳头），z 跨度 ±8 配合透视近似（相机距离 10，缩放 `s = 10/(10+z)`），罐子从近景右下角飞入、判定后缩小着飞向远处；入场自转 125°/拍。
- **音效**：抛出 `objectOut.ogg`、命中 `potHit.ogg`/`punchKickHit1.ogg`、空挥 `swingNoHit.wav`、漏拍 `karate_through.wav`，均为原版资产，启动时解码为裸 PCM 实现零延迟触发。
- **测试**：`go test ./kart/` 在无窗口环境下验证骨架采样（变换有限性、Jab 换帧、包围盒合理性）。

## 已知简化（demo 范围外）

engine 路径（rhythmSomen / trickClass / meatGrinder / totemClimb / airRally 等）：
- karateman：Pack-In 使用路径已接入 engine（hit/bop/prepare/warnings/background/
  set object colors/particle effects/force facial expression）。普通 pot 与 rock 的
  轨迹、判定、throw/hit/through/voice 音效按 C# 时序移植；背景颜色、物体颜色、
  Joe MappingMaterial 换色和 wig 开关已接入；Word prefab 已接入原版字形图集与
  `word/Word00..06` 动画；`Head.FaceXX` 表情 clip 已按 Unity 子树播放语义
  叠加在 Joe 身体动作之上；背景纹理与 Sunburst/Rings 已按官方单图贴图和
  `bg/Sunburst`、`bg/Rings` 动画 clip 播放；Snow/Fire/Rain weather
  ParticleSystem 已从 `karateman.prefab` 导出，并按 `SetParticleEffect` 的
  rateOverTime/instant/wind 语义驱动；成功命中分支的 `HitParticles[]` 已按
  prefab 索引 root、HitPosition[1..5] 与 ItemCurves[6] 采样点参数级发射。
  仍缺完整项：bomb/kick 在飞行超时与 NG 清理阶段的延迟爆点。
- builtToScaleDS：`spawn blocks` 的生成、windup、判定、hit/NG/miss/Sink、
  Piano 音高、颜色/灯光/相机参数已按 C# 时序接入；官方资产是 mesh-only，
  提取器已导出、`assets/builtToScaleDS/meshes.json` 已入库且运行时已加载并
  可绘制 Unity 内置 mesh footprint（内置 mesh fileID、MeshRenderer 材质、
  纹理槽、float/color 材质参数），当前游戏模块仍用 Ebitengine 几何体替代 3D
  SkinnedMeshRenderer/材质渲染；Piano 的 `SetLoopParams(beat+length, 0.1f)`
  持续尾音和淡出已接入。
- rockers：`intervalStart`/`riff`/`passTurn`/`prepare`/`unPrepare`/`count`/
  `cmon`/`lastOne`/together riff 的事件流、JJ/Soshi 动画、循环和弦/逐弦音色、
  premade sample enum、mute 收尾、摄像机 pass-turn 与 C# `BendUp/BendDown(0.05f)`
  的连续滑音、barely 命中时 Soshi 闪电黄/蓝随机换色已接入。
- superSamuraiSlice：`bop`/小恶魔/大恶魔/平台滚动/环境开关的事件流、输入、
  主要动画、平台 guard、爆炸音效与鸟叫已接入；`Explode1/2/3`、`lightning`、
  `waterL/waterR` 已用官方 `sliceparticles` 图集切片做 burst，并按 C# 时序触发；
  Unity ParticleSystem 序列化参数已通用导出到 `particles.json`，爆炸/水花/闪电
  burst 运行时改为按该数据驱动。
- chargingChicken：`input`/`journeyLength`/倒计时泡泡/文本编辑/音乐淡入淡出/
  背景与视差 appearance/车身与前景色/强制 look/explode 的 action surface 已接入，
  主 charge、release、blastoff、鼓点 loop、岛实例入场和主要动画可运行；岛屿
  Collapse/StonePlatform/ChickenSplash/GrassFall 等 ParticleSystem 已按 Unity
  `particles.json` burst 参数接入；逐石块实例、精确落水/复位物理仍待按 Unity
  序列化参数补齐。
- airboarder：`bop`/`duck`/`crouch`/`jump`/`forceCharge`/`letsGo`/背景地板
  颜色/相机事件、arch/wall 判定窗口、CPU/玩家编舞、ready/yeah/miss/barely
  音效序列已按 C# 时序接入；提取器已导出 `Models` 下的 airboy、arch、
  wall、dog、floor controller 和所有 `.anim`，并把 Animator 曲线访问的 FBX
  内部骨架 path 合成为 scene 节点；MeshRenderer/材质/纹理槽与 sky imported
  FBX Geometry 顶点/UV 已导出，`assets/airboarder/meshes.json` 已入库并由运行时
  加载/绘制单 Geometry MeshRenderer；当前源树缺 sky `_MainTex` GUID 对应图片，
  因而该材质仍回退到 `_Color`。原版主体是 MeshRenderer/材质贴图的 3D 场景，
  当前仍暂用手写 2D billboard 渲染角色/障碍；运行时仍待补多 Geometry
  FBX 映射、缺失贴图恢复、CameraPivot/FOV 与
  ScrollingFloor 材质滚动的完整运行时支持。
- animalAcrobat：动物队列、障碍旋转/hold 判定、起跳/落地、背景颜色、
  Spotlight/Confetti、BGTileManager 双 tile 回收，以及 AnimalAcrobat.CameraUpdate
  的逐动物 hold/release/长颈鹿 zoom 相机流程已接入；PlayerAcrobat 的
  SuperCurveObject 跳跃曲线、RotateJump/ArcRotate、ShadowCo/LandingShadowCo
  已按 C# 公式接入；hold/release/sweat/SparkleTrail 已按 PlayerMonkey.prefab
  的 UV sprite、burst/rate、lifetime、velocity、size/color 与 Renderer
  sortingOrder 参数接入运行时；PartyPoppers confetti 已按 ConfettiL/R
  prefab 的 PopIntro 延迟、stream transform、burst、lifetime、velocity、
  gravity、startColor、size/color over lifetime 与 Renderer sortingOrder
  参数接入运行时。
- fillbots：`bop`/small/medium/large/custom/blackout/background appearance/
  object appearance 已接入，机器人落体、堆叠、传送带哨兵、hold/release 判定、
  release whiff、explosion、fillErUp 和 OK/miss/arm/water/beep 音效按 C# 时序
  移植；controller、模板 prefab、FullBody/limb/filler/meter/conveyer 动画已
  接入。燃料 Fill 已按 prefab 的 Unity Square、Fill 动画和 SpriteMask
  `VisibleInsideMask` 合成路径绘制。
- 启动页 Library 选择流程已接入原版背景、unplayed 关卡边框、`.riq` 自带
  `LibraryLevelIcon` 和关卡元数据；原版排序/搜索/收藏、已游玩评级边框与
  勋章状态尚未接入，当前固定按 `levels/*.riq` 文件名排序。
- AnimatorController 转换的 duration（交叉淡入）按立即切换处理；当前唯一非零用例
  BossCall→BossCallIdle 已逐曲线验证源末帧与目标姿态一致，视觉无差。
- 缓动函数全表实现（engine/ease.go，HS Ease 枚举 0..43 含 Expo/Circ/Bounce/
  Back/Elastic/OutIn/InstantOut）。
- TMP 文本用源 OTF 排版（原版为 SDF 渲染），字体/字号/颜色与 TMP 水平/垂直
  对齐枚举已接入，Justified/Flush 会按 TMP 行宽拉伸词距；字形边缘抗锯齿
  方式仍不同。
- vfx/display textbox 已按原版 TextboxAnchor、TextboxPrefab 尺寸、文本矩形、
  富文本 align 与自动换行绘制；框体按官方 `textboxSDF.png` 的四角 sliced
  SpriteRenderer 与 TextboxSDFMaterial 阈值生成，文本仍用 OTF 位图排版而非 TMP
  SDF 字形渲染。
- 多游戏 remix 中，未激活游戏的 interval 调度音效仍会播放（与 C# MultiSound 全局
  播放行为一致）；其动画动作也会执行但不可见。
- C# 的 `BossAnim.SetBool("bossAnnoyed")` 在原版 controller 中无任何转换引用（死调
  用），未移植——bop 的不悦表现走 `bossAnnoyed ? BossMiss : Bop` 分支，与原版一致。
- countIn 计数音实现 Normal/Alt/Cowbell/GBA/DS Male/DS Female 音色；公共音效
  以 `count-ins` 子目录相对路径加载，匹配 SoundEffects.cs 的 folder/type 语义。
- agbSamuraiSlice：`slowDown` 命中已接入 slow 版 slice 音效、Flash 动画，以及
  原版 `conductor.SetMinigamePitch(0.5)` 一拍全曲实时变调。
- airRally：rally、ba-bum-bum-bum、catch、enter、set distance、forward、
  4beat/8beat/count voice、rainbow、spawnBird、day、cloud、snowflake、tree、
  islandSpeed 均已接入运行时；动态天气/鸟群/树/彩虹为按 AirRally.cs 与 prefab
  序列化参数手写的非场景节点实例。
- Judgement 结算页已接入 Heaven Studio 的评分阈值/分类评价消息、rank 标志图、
  默认 epilogue 图与结算音效/jingle/循环音乐；`JudgementOpen.playable` 的
  Message0/1/2、BarStart 信号时间，以及 `JudgementManager` 的 barDuration/
  barRankWait/rankMusWait 已按原版参数推进。
- firstContact：`FirstContact.cs` 的 trailing mistranslation 分支引用
  `firstContact/slightlyFail`，但 HeavenStudio-master 的
  `Assets/Bundled/Games/FirstContact/Sounds` 未包含该音频文件；当前不播放
  替代音效，只保留 translator_eh 动画与红色 `..?` 文本，并用审计测试锁定
  该资源缺口，后续补到官方音频后移除此项。
- djSchool：hold 期间的 `sound FX` 原版切到 `DJSchool_Hold` AudioMixer
  snapshot（唱片摩擦滤波）；当前运行时先按同拍位做音乐 ducking，recordStop/
  recordSwipe/voice/cheer/boo 与动画状态已按 C# 时序接入，后续需补 live music
  filter 才能清掉此项。
- cheerReaders：toggleCaption 已接原 TMP caption/underlay 节点、StickyCanvas
  跟随相机语义和旧谱面 CheckCaptions 自动禁用路径；yay 纸花已按
  WhiteParticle/BlackParticle 的 sprite、emission、shape、lifetime、size/
  rotation/color over lifetime 与 sortingOrder 参数接入运行时。
- ninjaBodyguard：HitParticle 已按 prefab 中 ArrowSliceA/B 两个 ParticleSystem
  的 lifetime、simulationSpeed、startSpeed、shape arc/radius/rotation、burst、
  ForceModule 与 ParticleSystemRenderer sortingOrder/lengthScale 做运行时发射。
- lockstep：人群渲染将原版"3 台正交相机 → RenderTexture → 平铺 quad"等价
  实现为同尺度无限棋盘格直绘（几何/相位/缩放一致）。
- ppe 后处理（engine/postfx.go）：colorGrading/vignette/cabb/lensD/pixelQuad 与
  colorReplace/scanJitter/screenJump/retroTv/edgeDetect/sobelNeon/gaussBlur/
  grainBlur/dirBlur/analogNoise/liquidScreen/aurora 已接入；colorGrading 的
  technicolor AfterStack、GrainyBlur、AnalogNoise、Drunk、AuroraVignette 已按
  X-PostProcessing/Custom shader 公式接入；bloom 用 1/4 分辨率两轮高斯近似
  PPv2 的 mip 金字塔，anamorphicRatio 按 Unity 参数语义偏置水平/垂直模糊轴；
  retroTv 下的 HSonVHS 已接入 VHS noise/smear/downsample/upsample/composite/
  grain 多 RT 链，VHS 纹理优先从 `assets/common/vhs` 读取，缺失时回退本机
  HeavenStudio-master 资源，再缺失则记录日志并生成确定性 fallback；grain/retroTv
  非 VHS 噪声用 hash 近似烘焙噪声纹理；flash/HUD 不参与后处理（对应编辑器叠层）。
- totemClimb 柱子网格按"可见窗口直算"替代 Unity 的 12×3 环形回收（视觉等价）；
  原版 pillar (2) 不带下延段的细节按统一模板绘制（重叠区域同贴图，视觉等价）。
- totemClimb 高跳保持期的提前松手惩罚（UnHold + ScoreMiss + 重按回握）按
  HoldCo 轮询语义实现；空抬起（无判定窗）不计 whiff，与 C# 一致。

- 资产版权：图集与音效来自 Heaven Studio 工程，仅限本地验证使用。
