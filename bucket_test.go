package quartz

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestTask(t *testing.T) {
	tk := newTask(context.Background(), time.Millisecond*10, nowNano(), func(ctx context.Context) {

	})

	tk.Stop()
	tk.Pause()
	tk.Continue()
	fmt.Println(tk.state)
}
