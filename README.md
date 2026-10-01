# BehaviorLens 有序漏斗匹配库

Go 实现的有序漏斗匹配：从只进事件流中为每个用户选出唯一最优转化路径，并统计每一步的累计到达用户数。

## API

- `Build(steps []string, window int64) Query`：构造查询，包含有序步骤（事件名序列）和转换窗口（毫秒，首尾时间差等于窗口仍有效，从实际所选第一步起算）。
- `Scan(query, next, emit) []int`：通过 `next() (Event, bool)` 逐条前向读取事件，通过 `emit(userID, path)` 逐用户交付所选路径，返回每一步的累计到达用户数。

输入要求：已去重，按 `(UserID, Ts, ID)` 升序，同一用户事件连续。

选路规则：步骤数最多者优先，其次首尾时间差最小，再其次按 `(Ts, ID)` 二元组序列字典序最小；结果唯一。未命中第一步的用户不输出、不计数。

## 运行测试

```sh
go test ./...
```

覆盖内容：

- `TestWindowSkipsEarlyStart`：窗口 10ms，A@0、A@9、B@10、C@18，漏斗 A→B→C，选中从 9 开始的完整路径。
- `TestWindowBoundaryInclusive`：首尾时间差恰好等于窗口有效，超出 1ms 则回退到更短路径。
- `TestNoFirstStepNoOutput`：未命中第一步的用户不输出。
- `TestRandomAgainstExhaustive`：3000 组小规模随机数据（含重复步骤、同一事件匹配多步、相同时间戳、多条最长路径耗时相同），与穷举全部有效路径的独立参照逐用户比较路径与计数，并验证参照结果唯一。
