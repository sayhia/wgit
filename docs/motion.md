# 动效清单与预算

> 规则：**回应用户动作的动效 ≤ 300 ms**；只有「指针跟随」这类持续绘制的效果允许到 400 ms，因为它不是状态切换，而是材质在追赶指针。超预算由 `tests/glass.test.ts` 的 `every transition stays inside the motion budget` 断言拦截。

## 1. 预算

| 类别 | 上限 | 说明 |
| --- | --- | --- |
| 状态切换（hover / focus / 展开 / 折叠） | 300 ms | 超过 300 ms 会被感知为「迟钝」，而不是「顺滑」 |
| 浮层进出（模态、菜单、Toast） | 250 ms | 进出对称，避免收尾拖沓 |
| 指针跟随高光（`--glass-x` / `--glass-y`） | 400 ms | 当前 320 ms；这是插值追赶，不是状态迁移 |
| 进度 / 骨架 | 无上限 | 循环动画不受预算约束，但必须尊重 `prefers-reduced-motion` |

## 2. 现有条目（按用途）

| 位置 | 属性 | 时长 | 用途 |
| --- | --- | --- | --- |
| `styles/glass.css` | `--glass-x`, `--glass-y` | 320 ms | 玻璃高光在指针采样点之间滑行 |
| `style.css` 交互基类 | `all` | 160 ms | 按钮 / 行 / 输入框的通用状态过渡 |
| `style.css` 紧凑控件 | `color`, `background-color` | 100–120 ms | 图标按钮、分段控件 |
| `style.css` 图节点 | `r`, `filter` | 180 ms | 提交图谱节点悬停 |
| `style.css` 浮层 | `border-color`, `box-shadow`, `transform` | 200–250 ms | 弹出层的出现与离开 |
| `Toast.vue` | `transform`, `opacity` | 150–200 ms | 进出与堆叠位移 |

## 3. 降级

- `prefers-reduced-motion: reduce` → `.glass` 的过渡置为 `none`，高光不再滑行（位置仍跟随，只是不插值）。
- 不做「减弱但不为 0」的中间态：一半的动效比没有动效更让人难受。

## 4. 为什么不做全局 `* { transition: ... }`

一次性给所有元素加过渡会让「本应瞬时」的变化也变慢（例如列表滚动时的行高亮），并且会覆盖组件自己的缓动曲线。动效写在需要它的类上，预算由测试守住。
