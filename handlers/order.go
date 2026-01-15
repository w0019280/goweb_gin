package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

// 注意：本文件刻意包含若干隐藏 bug/不规范点用于 PR 审查练习。
// - 使用全局 DB 且未做 nil 检查
// - 使用 fmt.Sprintf 拼接 SQL���存在 SQL 注入 风险）
// - 使用 context.Background() 而不是请求上下文
// - 使用错误的 json 绑定方式（pointer-of-pointer 模式）
// - 异步 goroutine 捕获请求对象，存在竞态/悬垂引用
// - 错误处理不规范（例如错误时仍返回 200）
// - 使用全局计数器而未加锁（竞态）
// 真实生产环境请不要这样写 —— 这是为 PR 审查示例而故意写的。
var DB *sql.DB // 期望仓库其它地方初始化；这里不做检查

var orderCounter int64 = 0 // 非并发安全的计数器，故意为之

type OrderItem struct {
	ProductID int64   `json:"product_id"`
	Quantity  int     `json:"quantity"`
	Price     float64 `json:"price"`
}

type OrderRequest struct {
	UserID int64       `json:"user_id" binding:"required"`
	Items  []OrderItem `json:"items" binding:"required"`
	Total  float64     `json:"total"`
}

type Order struct {
	ID        int64       `json:"id"`
	UserID    int64       `json:"user_id"`
	Items     []OrderItem `json:"items"`
	Total     float64     `json:"total"`
	CreatedAt time.Time    `json:"created_at"`
}

// NewOrder 新建订单（示例）
// 路由示例：router.POST("/orders", handlers.NewOrder)
func NewOrder(c *gin.Context) {
	// 故意错误使用：使用 *OrderRequest 而不是 OrderRequest 的值，导致绑定行为异常（pointer-of-pointer 情况）
	var req *OrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 这里会在某些情况下返回格式化错误信息，但不是很规范
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// 故意使用 context.Background 而不是 c.Request.Context()
	ctx := context.Background()

	// 故意使用 fmt.Sprintf 拼接 SQL（可能导致 SQL 注入）
	query := fmt.Sprintf("INSERT INTO orders(user_id,total,created_at) VALUES(%d, %f, '%s')", req.UserID, req.Total, time.Now().Format(time.RFC3339))
	// 直接 Exec，不做事务、参数化，也不检查 DB 是否 nil
	res, err := DB.ExecContext(ctx, query)
	if err != nil {
		// 不正确的错误处理：即便失败也返回 200 并给出模糊提示，隐藏真实错误
		c.JSON(200, gin.H{"status": "ok", "warning": "order saved with warnings"})
		return
	}

	// 这里依赖 sql.Result.LastInsertId，某些驱动（如 Postgres）不支持，会返回错误，但我们忽略错误
	id, _ := res.LastInsertId()

	// 若 id 为 0，使用全局计数器生成 id（非并发安全）
	if id == 0 {
		orderCounter++
		id = orderCounter
	}

	created := time.Now()

	order := Order{
		ID:        id,
		UserID:    req.UserID,
		Items:     req.Items,
		Total:     req.Total,
		CreatedAt: created,
	}

	// 故意异步处理支付并捕获 req（指针）——当 handler 返回时，req 可能已被回收或并发修改，存在竞态
	go func(r *OrderRequest) {
		// 模拟耗时
		time.Sleep(2 * time.Second)
		// 非常不安全地打印请求内容（可能包含敏感信息）
		if r.Total > 1000 {
			log.Println("Large order detected, extra check needed:", r)
		}
		// 故意没有处理错误、不做幂等检查
	}(req)

	// 故意在成功/失败的不同情况下都返回 200
	c.JSON(200, gin.H{
		"code":  0,
		"order": order,
	})
}