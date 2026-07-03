# Eagle代码规范

## Functional Options 模式

### 适用场景

当一个对象在初始化时存在：

- 少量必填参数
- 多个可选参数
- 后续仍可能继续扩展的配置项

优先使用 Functional Options 模式，而不是把所有参数都堆到构造函数签名中。

参考实现：

- `type Option func(*Config)`
- `WithEnv(name string) Option`
- `WithFileType(fileType string) Option`
- `New(cfgDir string, opts ...Option) *Config`

### 设计收益

1. 降低构造函数复杂度

必填参数直接放在构造函数中，可选参数通过 `WithXxx(...)` 传入，避免出现超长参数列表和位置参数误传问题。

2. 提升调用可读性

相比 `New(cfgDir, env, fileType, enableWatch, timeout)`，`New(cfgDir, WithEnv("dev"), WithFileType("yaml"))` 更容易理解，每个配置项的含义在调用点就是显式的。

3. 便于设置默认值

构造函数先建立稳定默认值，再由 `Option` 按需覆盖，避免调用方每次都传完整配置，也能保证默认行为一致。

4. 提高扩展性和兼容性

新增配置时优先增加新的 `WithXxx(...)`，而不是修改 `New(...)` 签名。这样老调用方不需要跟着改，API 更稳定。

5. 将“对象创建”与“对象定制”解耦

`New(...)` 只负责创建合法对象，`WithXxx(...)` 只负责描述如何修改对象，职责更清晰，代码更容易维护和测试。

### 使用准则

- 必填参数保留在 `New(...)` 中，不要全部塞进 `Option`
- 可选参数使用 `WithXxx(...)` 表达
- 先设置默认值，再依次应用 `opts ...Option`
- `WithXxx(...)` 应只做单一职责的属性设置，不夹带复杂副作用
- 当配置项已经形成受控集合时，优先使用强类型或常量，而不是裸字符串

### 不建议的情况

- 只有 1 到 2 个简单参数时，不必为了模式而模式
- 如果每个 `Option` 都包含大量业务逻辑、副作用或跨模块耦合，这个模式会变得难以追踪
- 不要把本应强约束的必填依赖伪装成可选项

### 推荐表述

在需要“少量必填参数 + 多个可选参数 + 未来持续扩展”的初始化场景下，优先采用 Functional Options 模式，以获得更清晰的调用语义、更稳定的构造接口和更好的默认值管理能力。

## Go Functional Options 与 Rust Builder / with_xxx 对照

### 共通设计思想

Go 中的 Functional Options，与 Rust 中常见的 `new + with_xxx` 或 Builder 模式，在设计目标上是高度一致的：

- `new(...)` / `New(...)` 负责创建一个合法对象
- 默认值先在构造阶段建立
- 可选配置通过后续定制逐步覆盖
- 避免把所有配置都塞进一个很长的构造函数参数列表
- 让调用点具备更好的可读性和可维护性

### 主要区别

Rust 常见写法是通过对象方法链来修改对象，例如：

- `Config::new(...).with_env("dev").with_file_type("yaml")`

Go 的 Functional Options 则不是链式调用对象方法，而是：

- 先定义 `type Option func(*Config)`
- 再由 `WithEnv(...)`、`WithFileType(...)` 返回配置函数
- 最后在 `New(..., opts...)` 内部统一应用这些配置函数

### 理解映射

可以用下面这组关系帮助记忆：

- Rust `new(...)` 对应 Go `New(...)`
- Rust `with_xxx(...)` / Builder 步骤 对应 Go `WithXxx(...) Option`
- Rust 在方法链中逐步构造对象 对应 Go 在 `New(...)` 中统一执行 `opts ...Option`

### 记忆结论

可以把 Go 的 Functional Options 理解为：不用链式方法表达的 Builder 思想。它与 Rust 的 `new + with_xxx` 在目标上相同，只是在语言层面的组织方式不同。
