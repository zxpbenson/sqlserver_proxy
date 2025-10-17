package util

import "context"

func IsContextDone(ctx context.Context) bool { // non-blocking implementation
	select {
	case <-ctx.Done():
		return true // 已经结束（被取消或超时）
	default:
		return false // 还没结束
	}
}
