package cache

import (
	"context"
	"time"
)

func AsyncDel(ctx context.Context, rds *Redis, maxRetries int, waitDurations []time.Duration, keys ...string) {
	go func() {
		for i := 0; i < maxRetries; i++ {
			if i > 0 && i-1 < len(waitDurations) {
				select {
				case <-ctx.Done():
					return
				case <-time.After(waitDurations[i-1]):
				}
			} else if i > 0 {
				select {
				case <-ctx.Done():
					return
				case <-time.After(time.Second * time.Duration(i)):
				}
			}

			if err := rds.Del(ctx, keys...); err == nil {
				return
			}
		}
	}()
}
