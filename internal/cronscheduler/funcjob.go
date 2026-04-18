package cronscheduler

import "context"

// FuncJob 把 "裸函数 + 名字 + spec" 直接包成 Job, 适合业务方只需要一小段
// 逻辑, 不值得单独开一个 struct 的场景 (e.g. sqlite cache 定期清理, bundle
// 周期同步)。语义上等价于手写的 Job 实现, 走同一套 adapter: panic recover /
// 耗时日志 / SkipIfStillRunning 全都免费拿到, 不用每个业务方重造轮子。
//
// 为什么不直接把 Job 做成函数类型: 因为 Job 需要承载 Name/Spec/Run 三块
// 元信息, 而 Name/Spec 是"注册期静态数据", 跟 Run 的"运行期逻辑"生命周期
// 不同, 拆成 struct 字段表达更顺。
type FuncJob struct {
	name string
	spec string
	run  func(ctx context.Context) error
}

// NewFuncJob 构造一个 FuncJob。调用方保证:
//   - name 非空, 且在同一 Scheduler 内唯一 (duplicate 在 Register 时被拒);
//   - spec 符合 robfig/cron 语法 (e.g. "0 3 * * *" / "@every 30s"), 非法 spec
//     同样会在 Register 时被拒;
//   - run 非 nil, 函数内要尊重 ctx 取消信号以便 Stop 能按超时收敛。
//
// name/spec 的合法性校验统一延后到 Register 时由 Scheduler 一处报错, 避免两
// 边重复校验; 但 run == nil 属于"调用方写错代码"的编译期/静态期问题, 留到
// 运行期只会让 job 每次 tick 都悄悄空跑 + 日志显示成功, 排障成本极高, 所以
// 这里直接 panic 让问题在注册第一刻暴露。这是 fail-fast 选择, 已有调用方全
// 部传静态闭包, 不存在运行时才决定 run 是否为 nil 的合理场景。
func NewFuncJob(name, spec string, run func(ctx context.Context) error) *FuncJob {
	if run == nil {
		panic("cronscheduler: NewFuncJob run must not be nil")
	}
	return &FuncJob{name: name, spec: spec, run: run}
}

func (j *FuncJob) Name() string { return j.name }

func (j *FuncJob) Spec() string { return j.spec }

func (j *FuncJob) Run(ctx context.Context) error {
	return j.run(ctx)
}
